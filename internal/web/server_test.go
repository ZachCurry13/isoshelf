package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

const testToken = "TESTTOKEN234567ABCDEFGHIJKL"

func newServer(t *testing.T, dirs appdir.Dirs, target string) *Server {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	return New(Config{
		Dirs:    dirs,
		Catalog: cat,
		HTTP:    &http.Client{Transport: remotetest.Recorded()},
		Version: "dev",
		Token:   testToken,
		Target:  target,
	})
}

func testDirs(t *testing.T) appdir.Dirs {
	return appdir.Dirs{App: t.TempDir(), Config: t.TempDir(), Temp: t.TempDir()}
}

func sampleDrive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range sampledrive.Files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte("stand-in"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// request sends a request the way the page does: to localhost, with the
// token cookie and, for changes, the isoshelf header.
func request(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "http://127.0.0.1:8765"+path, reader)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	if method != http.MethodGet {
		req.Header.Set(requestHeader, "1")
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("response is not JSON (%d): %v\n%s", rec.Code, err, rec.Body)
	}
	return v
}

// waitIdle polls the state until no scan is running.
func waitIdle(t *testing.T, s *Server) stateJSON {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		st := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
		if st.Run == nil {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the scan didn't finish")
	return stateJSON{}
}

func TestGuard(t *testing.T) {
	s := newServer(t, testDirs(t), "")
	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)
		return rec
	}

	// The link isoshelf prints sets the cookie and redirects.
	rec := serve(httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/?token="+testToken, nil))
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Set-Cookie"), cookieName+"="+testToken) {
		t.Errorf("token link: %d, Set-Cookie %q", rec.Code, rec.Header().Get("Set-Cookie"))
	}

	withCookie := func(method, url string) *http.Request {
		req := httptest.NewRequest(method, url, nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
		return req
	}
	tests := []struct {
		name string
		req  *http.Request
		want int
	}{
		{"no cookie", httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/api/state", nil), http.StatusForbidden},
		{"wrong token link", httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8765/?token=WRONG", nil), http.StatusForbidden},
		{"rebinding host", withCookie(http.MethodGet, "http://evil.example:8765/api/state"), http.StatusForbidden},
		{"page with cookie", withCookie(http.MethodGet, "http://localhost:8765/"), http.StatusOK},
		{"change without header", withCookie(http.MethodPost, "http://127.0.0.1:8765/api/scan"), http.StatusForbidden},
	}
	crossSite := withCookie(http.MethodPost, "http://127.0.0.1:8765/api/cancel")
	crossSite.Header.Set(requestHeader, "1")
	crossSite.Header.Set("Origin", "https://evil.example")
	tests = append(tests, struct {
		name string
		req  *http.Request
		want int
	}{"cross-site change", crossSite, http.StatusForbidden})

	for _, tt := range tests {
		if rec := serve(tt.req); rec.Code != tt.want {
			t.Errorf("%s: got %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
}

func TestPageAndStaticFiles(t *testing.T) {
	s := newServer(t, testDirs(t), "")
	if rec := request(t, s, http.MethodGet, "/", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>isoshelf</title>") {
		t.Errorf("page: %d", rec.Code)
	}
	for _, path := range []string{"/static/app.js", "/static/app.css"} {
		if rec := request(t, s, http.MethodGet, path, nil); rec.Code != http.StatusOK {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
	if rec := request(t, s, http.MethodGet, "/", nil); !strings.Contains(rec.Header().Get("Content-Security-Policy"), "default-src 'self'") {
		t.Error("no Content-Security-Policy header")
	}
}

func TestScanCheckAndSettings(t *testing.T) {
	dirs := testDirs(t)
	drive := sampleDrive(t)
	s := newServer(t, dirs, "")

	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("scan without a folder: %d", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/target", map[string]string{"path": filepath.Join(drive, "missing")}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing folder: %d", rec.Code)
	}
	rec := request(t, s, http.MethodPost, "/api/target", map[string]string{"path": drive, "profile": "ventoy"})
	if st := decode[stateJSON](t, rec); rec.Code != http.StatusOK || st.Target != drive || st.Report != nil {
		t.Fatalf("set target: %d %+v", rec.Code, st)
	}

	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	st := waitIdle(t, s)
	if st.Report == nil || st.Report.Checked || len(st.Report.Items) != len(sampledrive.Files)-1 || st.Error != "" {
		t.Fatalf("after scan: %+v", st)
	}

	if rec := request(t, s, http.MethodPost, "/api/check", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	st = waitIdle(t, s)
	if st.Report == nil || !st.Report.Checked {
		t.Fatalf("after check: %+v", st)
	}
	var pop *struct{ status, latest, updates string }
	for _, it := range st.Report.Items {
		if it.Path == "pop-os_22.04_amd64_intel_56.iso" {
			pop = &struct{ status, latest, updates string }{it.Status, it.Latest, it.Updates}
		}
	}
	if pop == nil || pop.status != "update available" || pop.latest != "58" || pop.updates != "download" {
		t.Errorf("Pop!_OS item: %+v", pop)
	}

	// The replace-old checkbox and the star are saved in the folder's state.
	rec = request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "popos-2204-intel", "keep_old": true, "starred": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("track: %d %s", rec.Code, rec.Body)
	}
	saved, err := state.Load(drive)
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Track("popos-2204-intel"); !got.KeepOld || !got.Starred {
		t.Errorf("saved track = %+v", got)
	}
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "nope", "starred": true}); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown entry: %d", rec.Code)
	}

	// The catalog says which entries are in the folder.
	cat := decode[struct {
		Entries []catalogEntryJSON `json:"entries"`
	}](t, request(t, s, http.MethodGet, "/api/catalog", nil))
	onTarget := map[string]bool{}
	for _, e := range cat.Entries {
		onTarget[e.ID] = e.OnTarget
	}
	if !onTarget["popos-2204-intel"] || onTarget["ubuntu-desktop-lts"] {
		t.Errorf("on_target: popos %v, ubuntu %v", onTarget["popos-2204-intel"], onTarget["ubuntu-desktop-lts"])
	}

	// A new server remembers the folder.
	again := newServer(t, dirs, "")
	if st := decode[stateJSON](t, request(t, again, http.MethodGet, "/api/state", nil)); st.Target != drive {
		t.Errorf("remembered target = %q, want %q", st.Target, drive)
	}
	if st := decode[stateJSON](t, request(t, again, http.MethodGet, "/api/state", nil)); len(st.Recent) == 0 || st.Recent[0] != drive {
		t.Errorf("recent targets = %v", st.Recent)
	}
}

func TestBrowse(t *testing.T) {
	root := filepath.Join(t.TempDir(), "template", "iso")
	for _, dir := range []string{"archive", "Beta", ".isoshelf", "$RECYCLE.BIN"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "netboot.xyz.iso"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, testDirs(t), "")

	got := decode[browseJSON](t, request(t, s, http.MethodGet, "/api/browse?path="+root, nil))
	var names []string
	for _, f := range got.Folders {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "archive,Beta" {
		t.Errorf("folders = %v, want archive and Beta only", names)
	}
	if got.Path != root || got.Parent != filepath.Dir(root) || got.SuggestedProfile != "proxmox" || len(got.Roots) == 0 {
		t.Errorf("browse = %+v", got)
	}

	missing := decode[browseJSON](t, request(t, s, http.MethodGet, "/api/browse?path="+filepath.Join(root, "nope"), nil))
	if missing.Error == "" {
		t.Error("browsing a missing folder: want an error message")
	}
}
