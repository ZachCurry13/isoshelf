package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// send posts a file the way the page does: the body is the file itself, with
// its name and the answer about any clash in the query.
func send(t *testing.T, s *Server, name, body, replace string) *httptest.ResponseRecorder {
	t.Helper()
	url := "http://127.0.0.1:8765/api/upload?name=" + name
	if replace != "" {
		url += "&replace=" + replace
	}
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set(requestHeader, "1")
	req.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	return rec
}

func TestUploadAddsAFile(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	rec := send(t, s, "ubuntu-24.04.1-desktop-amd64.iso", "a new image", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	got, err := os.ReadFile(filepath.Join(dir, "ubuntu-24.04.1-desktop-amd64.iso"))
	if err != nil || string(got) != "a new image" {
		t.Fatalf("the file isn't in the folder: %q %v", got, err)
	}

	// It is written down, and the scan that follows works out what it is, the
	// same as it would for a file copied in with Explorer.
	st := waitIdle(t, s)
	disk, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec2, ok := disk.Files["ubuntu-24.04.1-desktop-amd64.iso"]
	if !ok {
		t.Fatalf("no record of the file: %+v", disk.Files)
	}
	if rec2.PlacedAt.IsZero() {
		t.Error("the record doesn't say when it arrived")
	}
	if rec2.Entry == "" {
		t.Errorf("the scan didn't work out what it is: %+v", rec2)
	}
	if st.UpdatedAt == nil {
		t.Error("the folder wasn't looked at again after the upload")
	}
}

func TestUploadRefusesWhatIsntAnImage(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	rec := send(t, s, "reading-list.txt", "not an image", "")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "isn't an image file") {
		t.Errorf("a text file: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "reading-list.txt")); err == nil {
		t.Error("it was written anyway")
	}
	// The sample drive has a notes.txt of its own; refusing one must not
	// disturb it.
	if got, err := os.ReadFile(filepath.Join(dir, "notes.txt")); err != nil || string(got) != "stand-in" {
		t.Errorf("the folder's own notes.txt changed: %q %v", got, err)
	}
}

// A clash is a question, not a failure: the page is told so it can offer the
// two answers, and nothing in the folder has changed meanwhile.
func TestUploadAsksBeforeOverwriting(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	name := "netboot.xyz.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("the one they have"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := send(t, s, name, "the new one", "")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"conflict":true`) {
		t.Fatalf("clash: %d %s", rec.Code, rec.Body)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "the one they have" {
		t.Errorf("the old file was touched: %q", got)
	}

	// Answering archives the old one, which can be brought back.
	if rec := send(t, s, name, "the new one", string(update.MoveAside)); rec.Code != http.StatusOK {
		t.Fatalf("after answering: %d %s", rec.Code, rec.Body)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != "the new one" {
		t.Errorf("the new file isn't in place: %q", got)
	}
	aside := filepath.Join(dir, state.DirName, update.RemovedDir, name)
	if got, _ := os.ReadFile(aside); string(got) != "the one they have" {
		t.Errorf("the old file isn't in the archive: %q", got)
	}
	waitIdle(t, s) // the scan the upload starts, before the folder is cleaned up
}

// The guard covers this the way it covers every other change.
func TestUploadNeedsTheHeaderAndTheToken(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)

	noHeader := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/api/upload?name=x.iso", bytes.NewReader([]byte("x")))
	noHeader.AddCookie(&http.Cookie{Name: cookieName, Value: testToken})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, noHeader)
	if rec.Code != http.StatusForbidden {
		t.Errorf("without the isoshelf header: %d, want 403", rec.Code)
	}

	noToken := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/api/upload?name=x.iso", bytes.NewReader([]byte("x")))
	noToken.Header.Set(requestHeader, "1")
	rec = httptest.NewRecorder()
	s.ServeHTTP(rec, noToken)
	if rec.Code != http.StatusForbidden {
		t.Errorf("without the token: %d, want 403", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "x.iso")); err == nil {
		t.Error("a file was written by a request that should have been refused")
	}
}

// An upload keeps whatever else reached the records while it was arriving.
func TestUploadKeepsOtherChanges(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "starred": true}); rec.Code != http.StatusOK {
		t.Fatalf("star: %d %s", rec.Code, rec.Body)
	}
	if rec := send(t, s, "debian-13.1.0-amd64-netinst.iso", "an image", ""); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s)

	disk, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !disk.Track("netbootxyz").Starred {
		t.Error("the upload saved over the star")
	}
	if _, ok := disk.Files["debian-13.1.0-amd64-netinst.iso"]; !ok {
		t.Error("the uploaded file has no record")
	}
}

// A file replaced by one of the same name still shows in the archive. Its
// note goes as soon as the next scan sees that path filled again, but the old
// file is on the disk using room, and being able to put it back is the whole
// point of archiving rather than deleting.
func TestArchiveShowsAReplacedFileAfterTheNextScan(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	name := "netboot.xyz.iso"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("the one they have"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := send(t, s, name, "the new one", string(update.MoveAside)); rec.Code != http.StatusOK {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	waitIdle(t, s) // the scan that follows, which forgets the note

	var got struct {
		Items []archiveItemJSON `json:"items"`
	}
	rec := request(t, s, http.MethodGet, "/api/archive", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range got.Items {
		if item.Path == name {
			found = true
			// It is in the archive, not the history: the file is on the
			// drive using room. But it can't go back yet, because the name
			// is taken by the file that replaced it.
			if !item.OnDisk {
				t.Errorf("%s is on the drive but counted as gone for good: %+v", name, item)
			}
			if item.Restorable {
				t.Errorf("%s offers to go back on top of the file holding its name: %+v", name, item)
			}
			if item.Size == 0 {
				t.Errorf("%s doesn't say how much room it uses: %+v", name, item)
			}
		}
	}
	if !found {
		t.Errorf("the replaced file isn't in the archive: %+v", got.Items)
	}

	// Once the name is free again it can go back, and really does.
	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		t.Fatal(err)
	}
	rec = request(t, s, http.MethodGet, "/api/archive", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(got.Items, func(i archiveItemJSON) bool { return i.Path == name && i.Restorable }) {
		t.Errorf("with the name free again it still can't be put back: %+v", got.Items)
	}
	if rec := request(t, s, http.MethodPost, "/api/restore", map[string]any{"name": name}); rec.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if body, _ := os.ReadFile(filepath.Join(dir, name)); string(body) != "the one they have" {
		t.Errorf("the restored file holds %q", body)
	}
	waitIdle(t, s)
}
