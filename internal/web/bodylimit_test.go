package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func postSettings(t *testing.T, s *Server, body string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/api/settings", strings.NewReader(body))
	req.Header.Set(requestHeader, "1")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec.Code
}

// A request can't be used to fill the computer's memory: past a megabyte the
// rest is never read, and the request is refused like any other broken one.
func TestRequestBodiesAreCutOff(t *testing.T) {
	s := newServer(t, testDirs(t), sampleDrive(t))

	if code := postSettings(t, s, `{}`); code != http.StatusOK {
		t.Fatalf("a request that changes nothing: %d, want 200", code)
	}
	// Close to the limit is still fine.
	if code := postSettings(t, s, `{"unused":"`+strings.Repeat("a", maxBody-64)+`"}`); code != http.StatusOK {
		t.Errorf("just under the limit: %d, want 200", code)
	}
	// Over it: a field nobody reads, two megabytes long. Without the limit
	// this is read into memory whole and then accepted.
	if code := postSettings(t, s, `{"unused":"`+strings.Repeat("a", 2*maxBody)+`"}`); code != http.StatusBadRequest {
		t.Errorf("over the limit: %d, want 400", code)
	}
}

// An upload is a whole image: it is the one request the limit leaves alone.
func TestUploadIsNotCutOff(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	body := strings.Repeat("x", maxBody+4096)
	if rec := send(t, s, "bigger-than-the-limit.iso", body, ""); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	info, err := os.Stat(filepath.Join(dir, "bigger-than-the-limit.iso"))
	if err != nil || info.Size() != int64(len(body)) {
		t.Fatalf("the whole file should be in the folder: %v %v", info, err)
	}
	waitIdle(t, s)
}
