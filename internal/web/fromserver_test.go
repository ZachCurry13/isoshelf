package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/auth"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// The laptop lists what the server has that its folder hasn't, copies one,
// and records it as what it is: a copy of the server's file, with the
// server's account of it, and not checked against anything the project
// publishes. The copy never lands on a file that's already there.
func TestCopyingAFileFromTheServer(t *testing.T) {
	holder, name, sum := sharingServer(t, true)
	if err := auth.Set(holder.cfg.Dirs.Config, "nas", "a good long password"); err != nil {
		t.Fatal(err)
	}
	// What the server knows about its own copy, which the laptop should
	// keep as the server's account rather than as anything it checked.
	fetched := state.Origin{How: state.OriginDownload, From: "https://example.org/mint.iso", At: time.Date(2026, 3, 3, 12, 0, 0, 0, time.UTC)}
	holder.mu.Lock()
	held := holder.st.Files[name]
	held.Origin = fetched
	holder.st.Files[name] = held
	holder.mu.Unlock()
	shared := httptest.NewServer(holder)
	defer shared.Close()

	dirs, target := testDirs(t), t.TempDir()
	off := false
	if err := settings.Save(dirs.Config, settings.Settings{
		// Offline: the scan after the copy must not go looking for the
		// project's answer on the real internet.
		AutoCheck: &off,
		Peer:      settings.Peer{Address: strings.TrimPrefix(shared.URL, "http://"), User: "nas", Password: "a good long password"},
	}); err != nil {
		t.Fatal(err)
	}
	laptop := newServer(t, dirs, target)
	// The file comes from the test server on this machine, which the
	// recorded answers the tests use for everything else don't cover.
	laptop.cfg.HTTP = shared.Client()
	request(t, laptop, http.MethodPost, "/api/scan", nil)
	waitIdle(t, laptop)

	listed := decode[struct {
		Files []peerFileJSON `json:"files"`
		Error string         `json:"error"`
	}](t, request(t, laptop, http.MethodGet, "/api/peer/files", nil))
	if listed.Error != "" || len(listed.Files) != 1 || listed.Files[0].Name != name {
		t.Fatalf("the laptop sees %+v (%q), want just %s", listed.Files, listed.Error, name)
	}

	if rec := request(t, laptop, http.MethodPost, "/api/peer/copy", map[string]string{"name": name, "sha256": sum}); rec.Code != http.StatusAccepted {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, laptop)
	here, err := os.ReadFile(filepath.Join(target, name))
	if err != nil {
		t.Fatalf("nothing arrived: %v", err)
	}
	there, _ := os.ReadFile(filepath.Join(holder.target, name))
	if string(here) != string(there) {
		t.Error("what arrived isn't the server's file")
	}
	rec := laptop.st.Files[name]
	if rec.Origin.How != state.OriginCopy || rec.Origin.Checked != "" {
		t.Errorf("origin is %+v, want a copy that nothing has checked", rec.Origin)
	}
	if rec.Before != fetched {
		t.Errorf("before is %+v, want the server's own account %+v", rec.Before, fetched)
	}

	// It is here now, so it isn't offered again, and a second copy is
	// refused rather than landing on it.
	again := decode[struct {
		Files []peerFileJSON `json:"files"`
	}](t, request(t, laptop, http.MethodGet, "/api/peer/files", nil))
	if len(again.Files) != 0 {
		t.Errorf("after the copy the laptop is still offered %+v", again.Files)
	}
	if rec := request(t, laptop, http.MethodPost, "/api/peer/copy", map[string]string{"name": name, "sha256": sum}); rec.Code != http.StatusConflict {
		t.Errorf("a second copy answered %d, want a refusal", rec.Code)
	}
}
