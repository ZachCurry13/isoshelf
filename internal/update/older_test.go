package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// twoVersions serves a project that still publishes 2 beside the newest, 3.
func twoVersions(t *testing.T) http.RoundTripper {
	t.Helper()
	files := map[string]string{
		"/api/versions":       `{"version": "3"} {"version": "2"}`,
		"/isos/":              `<a href="example-2.iso">example-2.iso</a> <a href="example-3.iso">example-3.iso</a>`,
		"/isos/example-2.iso": "version two",
		"/isos/example-3.iso": "version three",
		"/isos/SHA256SUMS":    sum("version two") + "  example-2.iso\n" + sum("version three") + "  example-3.iso\n",
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

func olderEntry(t *testing.T, manifest bool) *catalog.Entry {
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
url = "https://example.org/api/versions"
regex = '"version": "(?P<version>\d+)"'
[entry.artifact]
base = "https://example.org/isos/"
file = 'example-{version}\.iso'
`
	if manifest {
		text += "manifest = \"SHA256SUMS\"\n"
	}
	cat, err := catalog.Load(fstest.MapFS{"c.toml": {Data: []byte(text)}}, "c.toml")
	if err != nil {
		t.Fatal(err)
	}
	return cat.Entry("example")
}

// TestRunInstallsAnOlderVersionAndHolds goes back from 3 to 2, which holds
// the track at 2, then forward to the newest by hand, which releases it.
func TestRunInstallsAnOlderVersionAndHolds(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "example-3.iso"), []byte("version three"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	if err := st.Placed(dir, "example-3.iso", state.FileRecord{Entry: "example", Version: "3"}); err != nil {
		t.Fatal(err)
	}
	rc, fc := clients(t, twoVersions(t))
	opts := Options{
		Target: dir, Entry: olderEntry(t, true), Client: rc, Fetcher: fc, State: st,
		Old: []string{"example-3.iso"}, Removal: MoveAside, Version: "2", Now: func() time.Time { return now },
	}
	res, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.File != "example-2.iso" || res.Version != "2" || !res.Verified {
		t.Errorf("result = %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "example-3.iso")); !os.IsNotExist(err) {
		t.Error("the broken newer file is still there; replacing follows the setting as usual")
	}
	if h := st.Track("example").Hold; h != "2" {
		t.Errorf("hold = %q, want 2", h)
	}

	opts.Version, opts.Old = "", []string{"example-2.iso"}
	if _, err := Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if h := st.Track("example").Hold; h != "" {
		t.Errorf("installing the newest by hand left the hold at %q", h)
	}
}

func TestRunRefusesAVersionNoLongerPublished(t *testing.T) {
	dir := t.TempDir()
	rc, fc := clients(t, twoVersions(t))
	_, err := Run(context.Background(), Options{
		Target: dir, Entry: olderEntry(t, true), Client: rc, Fetcher: fc, State: state.New(scan.Ventoy),
		Version: "1", Now: func() time.Time { return now },
	})
	if !errors.Is(err, ErrNotPublished) {
		t.Errorf("err = %v, want ErrNotPublished", err)
	}
}

// TestRunRefusesAnOlderVersionItCantCheck: an older version is never
// downloaded unchecked, unlike the newest.
func TestRunRefusesAnOlderVersionItCantCheck(t *testing.T) {
	dir := t.TempDir()
	rc, fc := clients(t, twoVersions(t))
	st := state.New(scan.Ventoy)
	_, err := Run(context.Background(), Options{
		Target: dir, Entry: olderEntry(t, false), Client: rc, Fetcher: fc, State: st,
		Version: "2", Now: func() time.Time { return now },
	})
	if !errors.Is(err, ErrOlderUnchecked) {
		t.Errorf("err = %v, want ErrOlderUnchecked", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("something was written: %v", entries)
	}
	if st.Track("example").Hold != "" {
		t.Error("a refused download put the track on hold")
	}
}
