package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Moving a folder's records takes what isoshelf has learned with them. A
// folder that came back forgotten would be worse than not offering this at
// all.
func TestMovingRecordsKeepsWhatWasLearned(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	elsewhere := t.TempDir()
	s := newServer(t, dirs, target)

	// Two scans, then a star: a history, a usual set (which takes two scans
	// to have anything in it) and a choice, all of which have to survive.
	for range 2 {
		request(t, s, http.MethodPost, "/api/scan", nil)
		waitIdle(t, s)
	}
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "starred": true}); rec.Code != http.StatusOK {
		t.Fatalf("starring: %d %s", rec.Code, rec.Body)
	}

	before := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
	if before.Records.Location != settings.InFolder {
		t.Fatalf("records start at %q, want in the folder", before.Records.Location)
	}
	if len(before.UsualSet) == 0 {
		t.Fatal("the scan left no usual set, so this test can't tell whether it survived")
	}

	rec := request(t, s, http.MethodPost, "/api/records", map[string]any{
		"location": settings.Elsewhere, "dir": elsewhere,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("moving the records: %d %s", rec.Code, rec.Body)
	}
	after := decode[stateJSON](t, rec)
	if after.Records.Location != settings.Elsewhere || after.Records.Dir != elsewhere {
		t.Errorf("records are at %+v, want %q", after.Records, elsewhere)
	}
	if !strings.HasPrefix(after.Records.File, elsewhere) {
		t.Errorf("the records file is %q, want it under %q", after.Records.File, elsewhere)
	}
	if _, err := os.Stat(after.Records.File); err != nil {
		t.Errorf("no records file where it says they are: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, state.DirName, "state.json")); err == nil {
		t.Error("the records were copied, not moved: the folder still has its own")
	}
	if len(after.UsualSet) != len(before.UsualSet) || !after.Tracks["netbootxyz"].Starred {
		t.Errorf("the move lost what was learned: %d in the usual set, starred=%v",
			len(after.UsualSet), after.Tracks["netbootxyz"].Starred)
	}

	// And a new scan writes to the new place, not the old one.
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	if _, err := os.Stat(filepath.Join(target, state.DirName, "state.json")); err == nil {
		t.Error("a scan after the move wrote records back into the folder")
	}
	if !decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Tracks["netbootxyz"].Starred {
		t.Error("a scan after the move lost the star")
	}
}

// Back again: the answer is a choice, not a one-way door.
func TestMovingRecordsBackIntoTheFolder(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "starred": true}); rec.Code != http.StatusOK {
		t.Fatalf("starring: %d %s", rec.Code, rec.Body)
	}

	request(t, s, http.MethodPost, "/api/records", map[string]any{
		"location": settings.Elsewhere, "dir": t.TempDir(),
	})
	rec := request(t, s, http.MethodPost, "/api/records", map[string]any{"location": settings.InFolder})
	if rec.Code != http.StatusOK {
		t.Fatalf("moving them back: %d %s", rec.Code, rec.Body)
	}
	got := decode[stateJSON](t, rec)
	if got.Records.Location != settings.InFolder {
		t.Errorf("records are at %q, want in the folder", got.Records.Location)
	}
	if !got.Tracks["netbootxyz"].Starred {
		t.Error("moving back lost the star")
	}
	if _, err := os.Stat(filepath.Join(target, state.DirName, "state.json")); err != nil {
		t.Errorf("the folder has no records after moving them back: %v", err)
	}
}

// isoshelf's own folder is one of the three answers, and needs no path typed.
func TestRecordsCanGoInIsoshelfsOwnFolder(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, target)

	rec := request(t, s, http.MethodPost, "/api/records", map[string]any{"location": settings.WithApp})
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := decode[stateJSON](t, rec)
	if !strings.HasPrefix(got.Records.File, dirs.Config) {
		t.Errorf("records file is %q, want it under %q", got.Records.File, dirs.Config)
	}
}

// Each folder answers for itself, which is the point: one drive isoshelf
// shouldn't write to doesn't change where a NAS share keeps its records.
func TestEachFolderAnswersForItself(t *testing.T) {
	dirs, one, two := testDirs(t), sampleDrive(t), sampleDrive(t)
	s := newServer(t, dirs, one)
	elsewhere := t.TempDir()

	request(t, s, http.MethodPost, "/api/records", map[string]any{
		"location": settings.Elsewhere, "dir": elsewhere,
	})
	rec := request(t, s, http.MethodPost, "/api/target", map[string]any{"path": two})
	if rec.Code != http.StatusOK {
		t.Fatalf("opening the second folder: %d %s", rec.Code, rec.Body)
	}
	if got := decode[stateJSON](t, rec).Records; got.Location != settings.InFolder {
		t.Errorf("the second folder was given the first one's answer: %+v", got)
	}
	// And the first folder still has its own.
	rec = request(t, s, http.MethodPost, "/api/target", map[string]any{"path": one})
	if got := decode[stateJSON](t, rec).Records; got.Location != settings.Elsewhere || got.Dir != elsewhere {
		t.Errorf("the first folder forgot its answer: %+v", got)
	}
}

// The complaints, in the words the page shows - from the question as well as
// the move, so nobody is asked about a move that would then be refused.
func TestRecordsRefusesWhatCannotWork(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, target)

	for _, c := range []struct {
		why  string
		body map[string]any
		want string
	}{
		{"a choice isoshelf doesn't know", map[string]any{"location": "the moon"}, "Choose where"},
		{"nowhere named", map[string]any{"location": settings.Elsewhere, "dir": "  "}, "Name a folder"},
		{"a relative path", map[string]any{"location": settings.Elsewhere, "dir": "records"}, "whole path"},
		{"inside the folder itself", map[string]any{"location": settings.Elsewhere, "dir": filepath.Join(target, "records")}, "same drive"},
	} {
		for _, path := range []string{"/api/records/plan", "/api/records"} {
			rec := request(t, s, http.MethodPost, path, c.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s %s: %d, want 400", path, c.why, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), c.want) {
				t.Errorf("%s %s: %s, want it to mention %q", path, c.why, rec.Body, c.want)
			}
		}
	}
}

// The list of folders isoshelf remembers, and taking one off it.
func TestForgettingAFolder(t *testing.T) {
	dirs, one, two := testDirs(t), sampleDrive(t), sampleDrive(t)
	s := newServer(t, dirs, one)

	// Two folders, each scanned once so each is remembered.
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	request(t, s, http.MethodPost, "/api/target", map[string]any{"path": two})
	request(t, s, http.MethodPost, "/api/scan", nil)
	got := waitIdle(t, s)
	if len(got.Recent) != 2 {
		t.Fatalf("remembered %d folders, want 2: %+v", len(got.Recent), got.Recent)
	}

	// The open one can't be forgotten out from under the page.
	var open, other rememberedJSON
	for _, f := range got.Recent {
		if f.Path == two {
			open = f
		} else {
			other = f
		}
	}
	rec := request(t, s, http.MethodPost, "/api/folders/forget", map[string]any{"path": open.Path, "id": open.ID})
	if rec.Code != http.StatusConflict {
		t.Errorf("forgetting the open folder: %d, want 409", rec.Code)
	}

	rec = request(t, s, http.MethodPost, "/api/folders/forget", map[string]any{"path": other.Path, "id": other.ID})
	if rec.Code != http.StatusOK {
		t.Fatalf("forgetting the other folder: %d %s", rec.Code, rec.Body)
	}
	after := decode[stateJSON](t, rec)
	if len(after.Recent) != 1 || after.Recent[0].Path != two {
		t.Errorf("after forgetting: %+v, want only the open folder", after.Recent)
	}

	// And nothing in the folder was touched: its own records are still there,
	// so opening it again finds everything.
	if _, err := os.Stat(filepath.Join(one, state.DirName, "state.json")); err != nil {
		t.Errorf("forgetting a folder deleted its own records: %v", err)
	}
	names, err := os.ReadDir(one)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) < 2 {
		t.Errorf("forgetting a folder emptied it: %d left", len(names))
	}
}

// Forgetting also drops the answer that folder was given about where its
// records live, so it doesn't linger in the settings file for a folder
// nobody has any more.
func TestForgettingDropsTheRecordsAnswerToo(t *testing.T) {
	dirs, one, two := testDirs(t), sampleDrive(t), sampleDrive(t)
	s := newServer(t, dirs, one)
	request(t, s, http.MethodPost, "/api/records", map[string]any{
		"location": settings.Elsewhere, "dir": t.TempDir(),
	})
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	request(t, s, http.MethodPost, "/api/target", map[string]any{"path": two})

	var gone rememberedJSON
	for _, f := range decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Recent {
		if f.Path == one {
			gone = f
		}
	}
	if gone.ID == "" {
		t.Fatal("the first folder isn't in the list, so there is nothing to forget")
	}
	request(t, s, http.MethodPost, "/api/folders/forget", map[string]any{"path": gone.Path, "id": gone.ID})

	if got := settings.Load(dirs.Config).RecordsFor(one); got.Location != settings.InFolder {
		t.Errorf("the forgotten folder kept its answer: %+v", got)
	}
}
