package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// isoWithLabel builds a small file that carries a real ISO 9660 primary
// volume descriptor, so the scanner reads a label out of it.
func isoWithLabel(t *testing.T, dir, name, label string) {
	t.Helper()
	const pvd = 16 * 2048
	data := make([]byte, 40000)
	data[pvd] = 1
	copy(data[pvd+1:], "CD001")
	copy(data[pvd+40:], label)
	copy(data[pvd+813:], "2023080801190500\x00")
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIdentify(t *testing.T) {
	dir := t.TempDir()
	isoWithLabel(t, dir, "mystery.iso", "Ubuntu 22.04.3 LTS amd64")
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	type guessesResponse struct {
		Path     string      `json:"path"`
		Assigned string      `json:"assigned"`
		Guesses  []guessJSON `json:"guesses"`
	}
	got := decode[guessesResponse](t, request(t, s, http.MethodGet, "/api/guesses?path=mystery.iso", nil))
	if len(got.Guesses) == 0 {
		t.Fatal("no guesses for a disc labelled Ubuntu 22.04.3 LTS amd64")
	}
	best := got.Guesses[0]
	if best.Entry != "ubuntu-desktop-lts" {
		t.Fatalf("best guess %s (%s), want ubuntu-desktop-lts", best.Entry, best.Reason)
	}
	if best.Reason == "" || best.Score == 0 {
		t.Errorf("a guess needs a reason and a score: %+v", best)
	}

	// Confirming it records the answer without touching the file.
	rec := request(t, s, http.MethodPost, "/api/identify",
		map[string]string{"path": "mystery.iso", "entry": best.Entry, "version": best.Version})
	if rec.Code != http.StatusOK {
		t.Fatalf("identify: %d %s", rec.Code, rec.Body)
	}
	item := findItem(t, waitIdle(t, s), "mystery.iso")
	if item.Entry != "ubuntu-desktop-lts" || !item.Assigned {
		t.Errorf("after identifying: entry %q, assigned %v", item.Entry, item.Assigned)
	}
	if _, err := os.Stat(filepath.Join(dir, "mystery.iso")); err != nil {
		t.Errorf("the file itself should be untouched: %v", err)
	}

	// The answer is offered back, so the user can see what they chose.
	got = decode[guessesResponse](t, request(t, s, http.MethodGet, "/api/guesses?path=mystery.iso", nil))
	if got.Assigned != "ubuntu-desktop-lts" {
		t.Errorf("assigned = %q, want ubuntu-desktop-lts", got.Assigned)
	}

	// It survives another scan: the state remembers it, not the filename.
	request(t, s, http.MethodPost, "/api/scan", nil)
	if item := findItem(t, waitIdle(t, s), "mystery.iso"); item.Entry != "ubuntu-desktop-lts" {
		t.Errorf("after rescanning: entry %q", item.Entry)
	}

	// And it can be taken back.
	if rec := request(t, s, http.MethodPost, "/api/identify",
		map[string]string{"path": "mystery.iso", "entry": ""}); rec.Code != http.StatusOK {
		t.Fatalf("forget: %d %s", rec.Code, rec.Body)
	}
	if item := findItem(t, waitIdle(t, s), "mystery.iso"); item.Entry != "" || item.Status != "unrecognized" {
		t.Errorf("after forgetting: entry %q, status %q", item.Entry, item.Status)
	}
}

func TestIdentifyRefusals(t *testing.T) {
	dir := t.TempDir()
	isoWithLabel(t, dir, "mystery.iso", "Something Else 1.0")
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	tests := []struct {
		name string
		body map[string]string
		want int
	}{
		{"unknown entry", map[string]string{"path": "mystery.iso", "entry": "no-such-image"}, http.StatusBadRequest},
		{"file not in the scan", map[string]string{"path": "gone.iso", "entry": "qubes"}, http.StatusBadRequest},
		{"outside the folder", map[string]string{"path": "../elsewhere.iso", "entry": "qubes"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		if rec := request(t, s, http.MethodPost, "/api/identify", tt.body); rec.Code != tt.want {
			t.Errorf("%s: %d, want %d (%s)", tt.name, rec.Code, tt.want, strings.TrimSpace(rec.Body.String()))
		}
	}
}
