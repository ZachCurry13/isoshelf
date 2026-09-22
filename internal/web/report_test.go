package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The report says what isoshelf is, and nothing about where the user keeps
// their files. A bug report is a public page.
func TestReportSaysNothingPrivate(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	rec := request(t, s, http.MethodGet, "/api/report", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("report: %d %s", rec.Code, rec.Body)
	}
	var got reportJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.System == "" || got.Template != "bug.yml" || !strings.HasPrefix(got.URL, "https://github.com/") {
		t.Errorf("report = %+v", got)
	}
	if got.Folder == "" {
		t.Error("the report doesn't say what kind of folder is open")
	}

	// The folder's path must not appear anywhere in it, under any field.
	body := rec.Body.String()
	if strings.Contains(body, dir) {
		t.Errorf("the report carries the folder's path: %s", body)
	}
	for _, name := range []string{"notes.txt", "Windows.iso", ".isoshelf"} {
		if strings.Contains(body, name) {
			t.Errorf("the report names a file (%s): %s", name, body)
		}
	}
}
