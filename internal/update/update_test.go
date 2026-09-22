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

func TestRunNothingToDownload(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))
	manual := &catalog.Entry{ID: "manual", Source: catalog.Source{Type: catalog.SourceManual}}

	_, err := Run(context.Background(), Options{Target: dir, Entry: manual, Client: rc, Fetcher: fc, State: st})
	if !errors.Is(err, ErrNothingToDownload) {
		t.Errorf("got %v, want ErrNothingToDownload", err)
	}
}

// fixedSite serves an image whose filename never changes.
func fixedSite(t *testing.T) http.RoundTripper {
	t.Helper()
	sum := sha256.Sum256([]byte(newImage))
	files := map[string]string{
		"/isos/fixed.iso":  newImage,
		"/isos/SHA256SUMS": hex.EncodeToString(sum[:]) + "  fixed.iso\n",
		"/api/latest":      `{"version": "2"}`,
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

func fixedEntry(t *testing.T) *catalog.Entry {
	t.Helper()
	text := `schema = 1
[[entry]]
id = "fixed"
name = "Fixed Name Image"
arch = "x86_64"
match = 'fixed\.iso'
fixed_name = true
samples = ["fixed.iso"]
[entry.source]
type = "listing"
url = "https://example.org/api/latest"
regex = '"version": "(?P<version>\d+)"'
[entry.artifact]
base = "https://example.org/isos/"
file = 'fixed\.iso'
manifest = "SHA256SUMS"
`
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("fixed")
}

// An image whose filename never changes lands on top of the old file, so the
// old one has to be moved aside first, and only after the new one is verified.
func TestRunSameFilename(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixed.iso"), []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "fixed.iso", state.FileRecord{Entry: "fixed"}); err != nil {
		t.Fatal(err)
	}
	rc, fc := clients(t, fixedSite(t))
	opts := Options{
		Target: dir, Entry: fixedEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"fixed.iso"}, Removal: MoveAside, Now: func() time.Time { return now },
	}

	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "fixed.iso")); err != nil || string(got) != newImage {
		t.Fatalf("new file: %q, %v", got, err)
	}
	aside := filepath.Join(dir, state.DirName, RemovedDir, "fixed.iso")
	if got, err := os.ReadFile(aside); err != nil || string(got) != "the old image" {
		t.Errorf("the old file wasn't kept safe: %q, %v", got, err)
	}
	if !slices.Equal(res.Removed, []string{"fixed.iso"}) {
		t.Errorf("removed = %v", res.Removed)
	}

	// With no answer at all it still refuses, because that is the page asking
	// rather than someone having chosen.
	opts.Removal = ""
	if _, err := Run(context.Background(), opts); err == nil || !strings.Contains(err.Error(), "same filename") {
		t.Errorf("no answer: got %v, want an explanation", err)
	}
}

// Keeping both copies of an image whose filename never changes: the new
// download carries its version in its name, and the file already on the drive
// is not touched at all.
func TestRunKeepsBothWhenTheNameNeverChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixed.iso"), []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "fixed.iso", state.FileRecord{Entry: "fixed", Version: "1.2.3"}); err != nil {
		t.Fatal(err)
	}
	rc, fc := clients(t, fixedSite(t))
	res, err := Run(context.Background(), Options{
		Target: dir, Entry: fixedEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"fixed.iso"}, Removal: Keep, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	// The file that was already there is exactly as it was. Nothing on the
	// drive was renamed, so nothing pointing at it by name can have broken.
	if got, err := os.ReadFile(filepath.Join(dir, "fixed.iso")); err != nil || string(got) != "the old image" {
		t.Fatalf("the old file changed: %q, %v", got, err)
	}
	if !slices.Contains(res.Kept, "fixed.iso") || len(res.Removed) != 0 {
		t.Errorf("kept %v, removed %v", res.Kept, res.Removed)
	}

	// The new one says which version it is, in its name.
	if !strings.HasPrefix(res.File, "fixed-") || !strings.HasSuffix(res.File, ".iso") || res.File == "fixed.iso" {
		t.Fatalf("the new file is called %q; it should carry its version", res.File)
	}
	if got, err := os.ReadFile(filepath.Join(dir, res.File)); err != nil || string(got) != newImage {
		t.Errorf("the new file: %q, %v", got, err)
	}

	// And isoshelf knows what it is, though its name no longer matches the
	// catalog's - otherwise the next scan would call it an unknown file.
	rec, ok := st.Files[res.File]
	if !ok || rec.Entry != "fixed" || !rec.Assigned {
		t.Errorf("the new file's record: %+v (present %v)", rec, ok)
	}
}

// Without a version to go on, the new file is named by the day it arrived,
// and a second copy the same day doesn't land on the first.
func TestKeepBothNameFallsBackToTheDate(t *testing.T) {
	dir := t.TempDir()
	first := KeepBothName(dir, "fixed.iso", "", now)
	want := "fixed-" + now.UTC().Format("2006-01-02") + ".iso"
	if first != want {
		t.Errorf("first = %q, want %q", first, want)
	}
	if err := os.WriteFile(filepath.Join(dir, first), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if second := KeepBothName(dir, "fixed.iso", "", now); second == first {
		t.Errorf("the second copy took the first one's name: %q", second)
	}
	// A version that couldn't be part of a filename doesn't become one.
	if got := KeepBothName(dir, "fixed.iso", "../../etc", now); strings.ContainsAny(got, `/\`) {
		t.Errorf("a version turned into a path: %q", got)
	}
}

// A download with no published checksum never replaces anything on its own,
// whatever was asked: the old file stays until the user has looked.
func TestRunUnverifiedKeepsOldFile(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))
	entry := testEntry(t)
	entry.Artifact.Manifest = ""

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: entry, Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: DeleteNow, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Verified || len(res.Removed) != 0 || !slices.Equal(res.Kept, []string{"example-1.iso"}) {
		t.Errorf("verified %v, removed %v, kept %v", res.Verified, res.Removed, res.Kept)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-1.iso")); err != nil {
		t.Error("an unverified download replaced the old file")
	}
	if _, err := os.Stat(filepath.Join(dir, "example-2.iso")); err != nil {
		t.Errorf("the new file isn't there: %v", err)
	}
}

// With the same filename the old file can't stay where it is, so an
// unverified download archives it, even when deleting was asked for.
func TestRunUnverifiedSameFilenameArchives(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixed.iso"), []byte("the old image"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "fixed.iso", state.FileRecord{Entry: "fixed"}); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"/isos/":          `<a href="fixed.iso">fixed.iso</a>`,
		"/isos/fixed.iso": newImage,
		"/api/latest":     `{"version": "2"}`,
	}
	rc, fc := clients(t, remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, filepath.Base(r.URL.Path), time.Unix(0, 0), strings.NewReader(body))
	})))
	entry := fixedEntry(t)
	entry.Artifact.Manifest = ""

	if _, err := Run(context.Background(), Options{
		Target: dir, Entry: entry, Client: rc, Fetcher: fc, State: st,
		Old: []string{"fixed.iso"}, Removal: DeleteNow, Now: func() time.Time { return now },
	}); err != nil {
		t.Fatal(err)
	}
	aside := filepath.Join(dir, state.DirName, RemovedDir, "fixed.iso")
	if got, err := os.ReadFile(aside); err != nil || string(got) != "the old image" {
		t.Errorf("the old file was deleted instead of archived: %q, %v", got, err)
	}
}

// The old file can be removed by hand while its replacement downloads. The
// update still finishes, with nothing left to replace.
func TestRunOldFileAlreadyGone(t *testing.T) {
	dir, st := target(t)
	rc, fc := clients(t, site(t, newImageSHA256()))
	if err := os.Remove(filepath.Join(dir, "example-1.iso")); err != nil {
		t.Fatal(err)
	}

	res, err := Run(context.Background(), Options{
		Target: dir, Entry: testEntry(t), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-1.iso"}, Removal: DeleteNow, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("the update failed over a file that was already gone: %v", err)
	}
	if len(res.Removed) != 0 || len(res.Kept) != 0 {
		t.Errorf("removed %v, kept %v", res.Removed, res.Kept)
	}
}
