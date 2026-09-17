package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

var now = time.Date(2026, 9, 17, 15, 0, 0, 0, time.UTC)

const newImage = "the new example image"

func newImageSHA256() string {
	sum := sha256.Sum256([]byte(newImage))
	return hex.EncodeToString(sum[:])
}

// site serves a small download site: an index, the image and its checksum.
func site(t *testing.T, checksum string) http.RoundTripper {
	t.Helper()
	files := map[string]string{
		"/isos/":              `<a href="example-2.iso">example-2.iso</a>`,
		"/isos/example-2.iso": newImage,
		"/isos/SHA256SUMS":    checksum + "  example-2.iso\n",
		"/api/latest":         `{"version": "2"}`,
	}
	return remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(r.URL.Path), time.Unix(0, 0), strings.NewReader(body))
	}))
}

// testEntry is a catalog entry pointing at the fake site.
func testEntry(t *testing.T) *catalog.Entry {
	t.Helper()
	text := `schema = 1
[[entry]]
id = "example"
name = "Example Linux"
arch = "x86_64"
match = 'example-(?P<version>\d+)\.iso'
samples = ["example-1.iso"]
[entry.source]
type = "listing"
url = "https://example.org/api/latest"
regex = '"version": "(?P<version>\d+)"'
[entry.artifact]
base = "https://example.org/isos/"
file = 'example-{version}\.iso'
manifest = "SHA256SUMS"
`
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("example")
}

func clients(t *testing.T, rt http.RoundTripper) (*remote.Client, *fetch.Client) {
	t.Helper()
	rc := remote.New("test")
	rc.HTTP = &http.Client{Transport: rt}
	rc.Backoff = time.Millisecond
	fc := fetch.New("test")
	fc.HTTP = &http.Client{Transport: rt}
	fc.Backoff = time.Millisecond
	fc.Retries = 1
	return rc, fc
}

// target sets up a folder holding an old copy of the image.
func target(t *testing.T) (string, *state.State) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example-1.iso"), []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "example-1.iso", state.FileRecord{Entry: "example", Version: "1"}); err != nil {
		t.Fatal(err)
	}
	return dir, st
}

func TestRunReplacesOldFile(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: MoveAside, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.File != "example-2.iso" || res.Version != "2" || !res.Verified || res.SHA256 != newImageSHA256() {
		t.Errorf("result = %+v", res)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "example-2.iso")); err != nil || string(got) != newImage {
		t.Fatalf("new file: %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-1.iso")); !os.IsNotExist(err) {
		t.Error("the old file is still in the folder")
	}
	// Moved aside, not deleted, and it can be got back.
	aside := filepath.Join(dir, state.DirName, RemovedDir, "example-1.iso")
	if got, err := os.ReadFile(aside); err != nil || string(got) != "the old image" {
		t.Errorf("moved-aside file: %q, %v", got, err)
	}
	if !slices.Equal(res.Removed, []string{"example-1.iso"}) {
		t.Errorf("removed = %v", res.Removed)
	}

	// The state knows the new file and forgot the old one.
	rec, ok := st.Files["example-2.iso"]
	if !ok || rec.Entry != "example" || rec.Version != "2" || rec.SHA256 != newImageSHA256() || rec.SourceURL == "" || rec.PlacedAt.IsZero() {
		t.Errorf("record = %+v", rec)
	}
	if _, ok := st.Files["example-1.iso"]; ok {
		t.Error("the old file still has a record")
	}

	files, bytes, err := Removed(dir)
	if err != nil || len(files) != 1 || bytes == 0 {
		t.Errorf("Removed() = %v, %d, %v", files, bytes, err)
	}
	if n, err := EmptyRemoved(dir); err != nil || n != 1 {
		t.Errorf("EmptyRemoved() = %d, %v", n, err)
	}
	if files, _, _ := Removed(dir); len(files) != 0 {
		t.Errorf("still %d files after emptying", len(files))
	}
}

func TestRunKeepsOldFile(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: Keep, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Removed) != 0 || !slices.Equal(res.Kept, []string{"example-1.iso"}) {
		t.Errorf("removed %v, kept %v", res.Removed, res.Kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-1.iso")); err != nil {
		t.Error("the old file was removed even though keeping was asked for")
	}
}

func TestRunRefusesBadChecksum(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, strings.Repeat("b", 64)))

	_, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: DeleteNow, Now: func() time.Time { return now },
	})
	var mismatch *verify.Mismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("got %v, want a checksum mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-2.iso")); !os.IsNotExist(err) {
		t.Error("a file that failed its checksum was placed")
	}
	if _, err := os.Stat(filepath.Join(dir, "example-1.iso")); err != nil {
		t.Error("the old file was removed even though the update failed")
	}
}

func TestRemoveRules(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	iso := make([]byte, 40000)
	copy(iso[32769:], "CD001")
	write("netboot.xyz.iso", "an image")
	write("notes.txt", "just notes")
	write("photos.zip", "PK\x03\x04 not an image")
	write("sub/Windows.iso", string(iso))
	write(filepath.Join(state.DirName, "state.json"), "{}")

	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	for _, rel := range []string{"netboot.xyz.iso", "notes.txt", "photos.zip", "sub/Windows.iso"} {
		if err := st.Placed(dir, rel, state.FileRecord{}); err != nil {
			t.Fatal(err)
		}
	}

	// Images may go; other files and isoshelf's own files may not.
	for rel, wantOK := range map[string]bool{
		"netboot.xyz.iso":             true, // recognized
		"sub/Windows.iso":             true, // unrecognized, but an image
		"notes.txt":                   false,
		"photos.zip":                  false,
		state.DirName + "/state.json": false,
		"../outside.iso":              false,
		"missing.iso":                 false,
	} {
		err := Removable(dir, rel, st, cat)
		if (err == nil) != wantOK {
			t.Errorf("Removable(%q) = %v, want ok=%v", rel, err, wantOK)
		}
	}

	removed, err := Remove(dir, []string{"netboot.xyz.iso", "sub/Windows.iso"}, DeleteNow, st, cat, now)
	if err != nil || len(removed) != 2 {
		t.Fatalf("Remove() = %v, %v", removed, err)
	}
	for _, rel := range removed {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s is still there", rel)
		}
		if _, ok := st.Files[rel]; ok {
			t.Errorf("%s still has a record", rel)
		}
	}

	if _, err := Remove(dir, []string{"notes.txt"}, DeleteNow, st, cat, now); err == nil {
		t.Error("removing a text file: want an error")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("notes.txt was deleted")
	}
	if _, err := Remove(dir, []string{"photos.zip"}, "shred", st, cat, now); err == nil {
		t.Error("unknown removal method: want an error")
	}
}

func TestRunNothingToDownload(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))
	manual := &catalog.Entry{ID: "manual", Source: catalog.Source{Type: catalog.SourceManual}}

	_, err := Run(context.Background(), Options{Target: dir, Entry: manual, Client: rc, Fetcher: fc, State: st})
	if !errors.Is(err, ErrNothingToDownload) {
		t.Errorf("got %v, want ErrNothingToDownload", err)
	}
}
