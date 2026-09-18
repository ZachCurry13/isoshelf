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
	"github.com/ZachCurry13/isoshelf/internal/check"
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

func TestRemoveAndEmpty(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)

	// Text files are not images, so they can't be removed.
	if rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"notes.txt"}, "how": "delete"}); rec.Code != http.StatusBadRequest {
		t.Errorf("removing notes.txt: %d, want 400", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("notes.txt was removed")
	}

	// An image can be moved aside, and then emptied for good.
	rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"Windows.iso"}, "how": "move-aside"})
	st := decode[stateJSON](t, rec)
	if rec.Code != http.StatusOK || st.Removed.Files != 1 || st.Removed.Bytes == 0 {
		t.Fatalf("move aside: %d, removed %+v", rec.Code, st.Removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "Windows.iso")); !os.IsNotExist(err) {
		t.Error("the file is still in the folder")
	}
	for _, it := range st.Report.Items {
		if it.Path == "Windows.iso" {
			t.Error("the removed file is still in the report")
		}
	}

	st = decode[stateJSON](t, request(t, s, http.MethodPost, "/api/removed/empty", nil))
	if st.Removed.Files != 0 {
		t.Errorf("after emptying: %+v", st.Removed)
	}
}

func TestUpdateRefusals(t *testing.T) {
	s := newServer(t, testDirs(t), sampleDrive(t))
	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown entry", map[string]any{"entry": "nope", "removal": "move-aside"}, http.StatusBadRequest},
		{"no removal choice", map[string]any{"entry": "netbootxyz"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		if rec := request(t, s, http.MethodPost, "/api/update", tt.body); rec.Code != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
}

func TestArchiveAndRestore(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)

	type archive struct {
		Items []struct {
			Path         string `json:"path"`
			Name         string `json:"name"`
			Gone         string `json:"gone"`
			Restorable   bool   `json:"restorable"`
			Downloadable bool   `json:"downloadable"`
		} `json:"items"`
	}
	if got := decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil)); len(got.Items) != 0 {
		t.Fatalf("a fresh folder remembers %d images", len(got.Items))
	}

	// Move one aside, and one is deleted outright.
	for _, how := range []struct{ file, how string }{
		{"netboot.xyz.iso", "move-aside"},
		{"Windows.iso", "delete"},
	} {
		if rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{how.file}, "how": how.how}); rec.Code != http.StatusOK {
			t.Fatalf("remove %s: %d %s", how.file, rec.Code, rec.Body)
		}
	}
	got := decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	byPath := map[string]int{}
	for i, item := range got.Items {
		byPath[item.Path] = i
	}
	netboot, ok := byPath["netboot.xyz.iso"]
	if !ok || got.Items[netboot].Gone != "moved-aside" || !got.Items[netboot].Restorable || !got.Items[netboot].Downloadable {
		t.Errorf("moved-aside image: %+v", got.Items)
	}
	if windows, ok := byPath["Windows.iso"]; !ok || got.Items[windows].Gone != "removed" || got.Items[windows].Restorable {
		t.Errorf("deleted image: %+v", got.Items)
	}

	// Putting it back works once, and the archive forgets it after a scan.
	if rec := request(t, s, http.MethodPost, "/api/restore", map[string]any{"name": "netboot.xyz.iso"}); rec.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "netboot.xyz.iso")); err != nil {
		t.Fatalf("the file didn't come back: %v", err)
	}
	if rec := request(t, s, http.MethodPost, "/api/restore", map[string]any{"name": "netboot.xyz.iso"}); rec.Code != http.StatusBadRequest {
		t.Errorf("restoring twice: %d, want 400", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("rescan: %d", rec.Code)
	}
	waitIdle(t, s)
	got = decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	for _, item := range got.Items {
		if item.Path == "netboot.xyz.iso" {
			t.Error("an image that came back is still listed as gone")
		}
	}

	// A file that disappears on its own is remembered too.
	if err := os.Remove(filepath.Join(dir, "HBCD_PE_x64.iso")); err != nil {
		t.Fatal(err)
	}
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	got = decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	found := false
	for _, item := range got.Items {
		if item.Path == "HBCD_PE_x64.iso" && item.Gone == "vanished" && item.Name == "Hiren's BootCD PE" {
			found = true
		}
	}
	if !found {
		t.Errorf("a vanished file isn't remembered: %+v", got.Items)
	}
}

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

// findItem returns the report row for a file.
func findItem(t *testing.T, st stateJSON, path string) check.ItemJSON {
	t.Helper()
	if st.Report == nil {
		t.Fatal("no report")
	}
	for _, it := range st.Report.Items {
		if it.Path == path {
			return it
		}
	}
	t.Fatalf("%s is not in the report", path)
	return check.ItemJSON{}
}
