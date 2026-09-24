package web

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// restarts records what isoshelf asked the command line to start.
type restarts struct {
	mu       sync.Mutex
	programs []string
}

func (r *restarts) restart(program, stage string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.programs = append(r.programs, program)
	return nil
}

func (r *restarts) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.programs...)
}

// updatingServer is a server that can update itself from a fake, signed
// release of v9.0.1 served on this machine.
func updatingServer(t *testing.T, version string, container bool) (*Server, *restarts, string) {
	t.Helper()
	suffix := appupdate.Suffix(runtime.GOOS, runtime.GOARCH)
	if suffix == "" {
		t.Skip("isoshelf publishes no program for this system")
	}
	pub, priv, _ := ed25519.GenerateKey(nil)
	name := "isoshelf-v9.0.1-" + suffix
	program := []byte("the new program")
	sum := sha256.Sum256(program)
	sums := []byte(fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name))
	files := map[string][]byte{name: program, appupdate.SumsFile: sums,
		appupdate.SumsFile + appupdate.SignatureSuffix: []byte(appupdate.Sign(priv, sums))}

	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/tags/", func(w http.ResponseWriter, r *http.Request) {
		var assets []map[string]any
		for n, b := range files {
			assets = append(assets, map[string]any{"name": n, "browser_download_url": srv.URL + "/dl/" + n, "size": len(b)})
		}
		json.NewEncoder(w).Encode(map[string]any{"assets": assets})
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		w.Write(files[strings.TrimPrefix(r.URL.Path, "/dl/")])
	})
	srv = httptest.NewTLSServer(mux)
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	exe := filepath.Join(dir, "isoshelf-v9.0.0-"+suffix)
	os.WriteFile(exe, []byte("the old program"), 0o755)
	fc := fetch.New("test")
	fc.HTTP, fc.Retries = srv.Client(), 0
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	r := &restarts{}
	s := New(Config{
		Dirs: testDirs(t), Catalog: cat, Version: version, Token: testToken,
		SelfUpdate: SelfUpdateConfig{
			Exe: exe, Container: container, Restart: r.restart,
			Source: &appupdate.Source{API: srv.URL + "/tags/", Client: &remote.Client{HTTP: srv.Client()}, Fetch: fc, Key: pub},
		},
	})
	s.mu.Lock()
	s.notice = &appupdate.Notice{Current: version, Latest: "v9.0.1"}
	s.mu.Unlock()
	return s, r, exe
}

// Update now downloads the release, checks its signature, puts it in place
// under the plain name, and asks to be restarted into it.
func TestUpdateNowPutsTheNewProgramInPlace(t *testing.T) {
	s, r, exe := updatingServer(t, "v9.0.0", false)
	if st := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)); !st.SelfUpdate.Can {
		t.Fatalf("Update now isn't offered: %q", st.SelfUpdate.Why)
	}
	if rec := request(t, s, http.MethodPost, "/api/selfupdate", nil); rec.Code != http.StatusOK {
		t.Fatalf("Update now: %d %s", rec.Code, rec.Body)
	}
	waitUntil(t, func() bool { return len(r.got()) > 0 })
	plain := filepath.Join(filepath.Dir(exe), "isoshelf-"+appupdate.Suffix(runtime.GOOS, runtime.GOARCH))
	if got := r.got()[0]; got != plain {
		t.Errorf("restarted into %s, want %s", got, plain)
	}
	if data, _ := os.ReadFile(plain); string(data) != "the new program" {
		t.Errorf("the program in place is %q", data)
	}
}

// Image downloads finish first: a restart would throw away half of one.
func TestUpdateNowWaitsForDownloads(t *testing.T) {
	s, r, _ := updatingServer(t, "v9.0.0", false)
	s.mu.Lock()
	s.downloading = &run{kind: "update", job: &job{id: 1, name: "Ubuntu"}}
	s.mu.Unlock()
	request(t, s, http.MethodPost, "/api/selfupdate", nil)
	waitUntil(t, func() bool {
		return decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).SelfUpdate.Stage == "waiting"
	})
	if len(r.got()) > 0 {
		t.Fatal("it restarted with a download running")
	}
	s.mu.Lock()
	s.downloading = nil
	s.mu.Unlock()
	waitUntil(t, func() bool { return len(r.got()) > 0 })
}

// Where isoshelf can't update itself, it says why rather than offering a
// button that fails.
func TestUpdateNowSaysWhyNot(t *testing.T) {
	for _, c := range []struct {
		version   string
		container bool
		want      string
	}{
		{"dev", false, "development build"},
		{"v9.0.0", true, "pulling the new image"},
	} {
		s, r, _ := updatingServer(t, c.version, c.container)
		st := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
		if st.SelfUpdate.Can || !strings.Contains(st.SelfUpdate.Why, c.want) {
			t.Errorf("%s: can=%v why=%q, want it to say %q", c.version, st.SelfUpdate.Can, st.SelfUpdate.Why, c.want)
		}
		if rec := request(t, s, http.MethodPost, "/api/selfupdate", nil); rec.Code != http.StatusConflict {
			t.Errorf("%s: Update now answered %d, want a refusal", c.version, rec.Code)
		}
		time.Sleep(50 * time.Millisecond)
		if len(r.got()) > 0 {
			t.Errorf("%s: it restarted anyway", c.version)
		}
	}
}

func waitUntil(t *testing.T, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatal("timed out")
}
