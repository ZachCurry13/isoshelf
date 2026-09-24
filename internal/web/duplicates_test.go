package web

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Make sure (#57) reads files that look like copies of one image - same
// entry, version and size - so their hashes can say whether they are. Until
// somebody asks, neither is read.
func TestMakeSureReadsBothCopies(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	const name = "linuxmint-22.3-cinnamon-64bit.iso"
	body := []byte("the same image twice")
	if err := os.MkdirAll(filepath.Join(target, "copies"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{name, filepath.Join("copies", name)} {
		if err := os.WriteFile(filepath.Join(target, p), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	st := waitIdle(t, s)
	one, two := itemAt(t, st, name), itemAt(t, st, "copies/"+name)
	if one.SHA256 != "" || two.SHA256 != "" {
		t.Fatalf("hashed before anybody asked: %q, %q", one.SHA256, two.SHA256)
	}

	rec := request(t, s, http.MethodPost, "/api/hash", map[string][]string{"paths": {name, "copies/" + name}})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("make sure: %d %s", rec.Code, rec.Body)
	}
	st = waitIdle(t, s)
	one, two = itemAt(t, st, name), itemAt(t, st, "copies/"+name)
	if one.SHA256 == "" || one.SHA256 != two.SHA256 {
		t.Errorf("after make sure: %q and %q, want the same hash for the same bytes", one.SHA256, two.SHA256)
	}
	if rec := request(t, s, http.MethodPost, "/api/hash", map[string][]string{"paths": {"not-here.iso"}}); rec.Code != http.StatusBadRequest {
		t.Errorf("hashing a file that isn't there answered %d, want a refusal", rec.Code)
	}
}
