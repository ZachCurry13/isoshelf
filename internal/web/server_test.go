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
