package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

const exampleCatalog = `schema = 1
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

// exampleSite is the project's own site for exampleCatalog: a listing that
// says version 2, the image and its checksum. Nothing here is the internet.
func exampleSite() http.RoundTripper {
	const image = "the new example image"
	sum := sha256.Sum256([]byte(image))
	files := map[string]string{
		"/api/latest":         `{"version": "2"}`,
		"/isos/":              `<a href="example-2.iso">example-2.iso</a>`,
		"/isos/example-2.iso": image,
		"/isos/SHA256SUMS":    hex.EncodeToString(sum[:]) + "  example-2.iso\n",
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

func runAgainst(t *testing.T, dirs appdir.Dirs, rt http.RoundTripper, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &env{
		stdout: &stdout, stderr: &stderr, dirs: &dirs,
		http:   &http.Client{Transport: rt},
		now:    func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) },
		getenv: func(string) string { return "" },
	})
	return result{code, stdout.String(), stderr.String()}
}

// isoshelf update (#1) says what happens to the old file or doesn't start,
// can say what it would do without doing it, and otherwise downloads the
// update, verified, and replaces the old file as it was told.
func TestUpdateOnTheCommandLine(t *testing.T) {
	dirs, folder := installed(t), t.TempDir()
	catalogFile := filepath.Join(t.TempDir(), "catalog.toml")
	if err := os.WriteFile(catalogFile, []byte(exampleCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(folder, "example-1.iso")
	if err := os.WriteFile(old, []byte("the old example image"), 0o644); err != nil {
		t.Fatal(err)
	}

	if res := runAgainst(t, dirs, exampleSite(), "update", "--catalog", catalogFile, folder); res.code == 0 || !strings.Contains(res.stderr, "--keep, --move-aside") {
		t.Errorf("no answer for the old file: exit %d, %q; want it refused", res.code, res.stderr)
	}

	res := runAgainst(t, dirs, exampleSite(), "update", "--catalog", catalogFile, "--dry-run", folder)
	if res.code != 0 || !strings.Contains(res.stdout, "Would update Example Linux: 1 -> 2") {
		t.Errorf("dry run: exit %d, %q %q", res.code, res.stdout, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(folder, "example-2.iso")); err == nil {
		t.Fatal("the dry run downloaded something")
	}

	res = runAgainst(t, dirs, exampleSite(), "update", "--catalog", catalogFile, "--delete", folder)
	if res.code != 0 || !strings.Contains(res.stdout, "Updated Example Linux to 2: example-2.iso") {
		t.Fatalf("update: exit %d, %q %q", res.code, res.stdout, res.stderr)
	}
	if got, err := os.ReadFile(filepath.Join(folder, "example-2.iso")); err != nil || string(got) != "the new example image" {
		t.Errorf("the new file is %q, %v", got, err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("--delete left the old file")
	}

	if res := runAgainst(t, dirs, exampleSite(), "update", "--catalog", catalogFile, "--keep", "--only", "nothing-like-it", folder); res.code == 0 || !strings.Contains(res.stderr, "nothing-like-it has no update") {
		t.Errorf("an entry with nothing to update: exit %d, %q", res.code, res.stderr)
	}
}
