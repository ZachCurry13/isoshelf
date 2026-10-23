package web

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Download button hands over an image the folder holds, under its own
// name, and nothing that isn't one: a path outside the records finds nothing.
func TestDownloadButtonServesOnlyImagesInTheRecords(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	const name = "linuxmint-22.3-cinnamon-64bit.iso"
	body := []byte("an image")
	if err := os.WriteFile(filepath.Join(target, name), body, 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(target), "secret.txt")
	if err := os.WriteFile(outside, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	// Off until someone turns it on: an update must not hand every image to
	// everyone who can sign in.
	if rec := request(t, s, http.MethodGet, "/api/file?path="+url.QueryEscape(name), nil); rec.Code != http.StatusForbidden {
		t.Fatalf("before turning downloads on: %d, want 403", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/settings", map[string]bool{"allow_downloads": true}); rec.Code != http.StatusOK {
		t.Fatalf("turning downloads on: %d %s", rec.Code, rec.Body)
	}

	rec := request(t, s, http.MethodGet, "/api/file?path="+url.QueryEscape(name), nil)
	if rec.Code != http.StatusOK || rec.Body.String() != string(body) {
		t.Fatalf("download: %d %q, want 200 and the file", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, name) {
		t.Errorf("Content-Disposition %q, want an attachment named %s", got, name)
	}

	for _, path := range []string{"", "../secret.txt", "missing.iso"} {
		if rec := request(t, s, http.MethodGet, "/api/file?path="+url.QueryEscape(path), nil); rec.Code != http.StatusNotFound {
			t.Errorf("path %q: %d, want 404", path, rec.Code)
		}
	}

	// Allow downloads turned off in Settings shuts the door, not just the
	// button.
	if rec := request(t, s, http.MethodPost, "/api/settings", map[string]bool{"allow_downloads": false}); rec.Code != http.StatusOK {
		t.Fatalf("turning downloads off: %d %s", rec.Code, rec.Body)
	}
	if rec := request(t, s, http.MethodGet, "/api/file?path="+url.QueryEscape(name), nil); rec.Code != http.StatusForbidden {
		t.Errorf("with downloads off: %d, want 403", rec.Code)
	}
}
