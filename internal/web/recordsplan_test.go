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

// Changing where a folder's records live asks first, and asking must change
// nothing: not the records, not the answer in the settings file. The
// maintainer had to ask what the dropdown did after it had already done it.
func TestAskingBeforeMovingRecordsMovesNothing(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	elsewhere := t.TempDir()
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	rec := request(t, s, http.MethodPost, "/api/records/plan", map[string]any{
		"location": settings.Elsewhere, "dir": elsewhere,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("asking: %d %s", rec.Code, rec.Body)
	}
	plan := decode[movedJSON](t, rec)
	here := filepath.Join(target, state.DirName, "state.json")
	if plan.What != "move" || plan.From != here || !strings.HasPrefix(plan.To, elsewhere) {
		t.Errorf("plan = %+v, want a move from %s to somewhere in %s", plan, here, elsewhere)
	}
	if plan.FromSaved.IsZero() {
		t.Error("the plan doesn't say when the records being moved were saved")
	}
	if _, err := os.Stat(here); err != nil {
		t.Errorf("asking moved the records: %v", err)
	}
	if _, err := os.Stat(plan.To); err == nil {
		t.Error("asking wrote the records where they would go")
	}
	if got := settings.Load(dirs.Config).RecordsFor(target); got.Location != settings.InFolder {
		t.Errorf("asking saved the new answer: %+v", got)
	}

	// Saying yes does what was described, and says so.
	rec = request(t, s, http.MethodPost, "/api/records", map[string]any{
		"location": settings.Elsewhere, "dir": elsewhere,
	})
	moved := decode[stateJSON](t, rec).Records.Moved
	if moved == nil || moved.What != "move" || moved.From != plan.From || moved.To != plan.To {
		t.Errorf("the move said %+v, want the move the plan described: %+v", moved, plan)
	}
	// And the answer to the next question about the page doesn't repeat it:
	// it was news once.
	if again := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)); again.Records.Moved != nil {
		t.Errorf("the state still reports the move: %+v", again.Records.Moved)
	}
}

// The case that used to happen in silence: records already waiting where
// they are going. The question says so before, with how old each set is, and
// the answer says so after. Nothing is deleted either way.
func TestRecordsAlreadyThereAreSaidBeforeAndAfter(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	elsewhere := t.TempDir()
	s := newServer(t, dirs, target)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)
	if err := state.Home(elsewhere).Save(state.New("folder"), target); err != nil {
		t.Fatal(err)
	}

	change := map[string]any{"location": settings.Elsewhere, "dir": elsewhere}
	plan := decode[movedJSON](t, request(t, s, http.MethodPost, "/api/records/plan", change))
	if plan.What != "keep" || plan.FromSaved.IsZero() || plan.ToSaved.IsZero() {
		t.Errorf("plan = %+v, want it to say both have records, and when each was saved", plan)
	}
	if plan.ToDir != filepath.Join(elsewhere, "records") {
		t.Errorf("plan names %q as where they are going, want %q", plan.ToDir, filepath.Join(elsewhere, "records"))
	}

	moved := decode[stateJSON](t, request(t, s, http.MethodPost, "/api/records", change)).Records.Moved
	if moved == nil || moved.What != "keep" {
		t.Fatalf("the move said %+v, want it to say it kept the records already there", moved)
	}
	if _, err := os.Stat(moved.From); err != nil {
		t.Errorf("the records that stayed behind were deleted: %v", err)
	}
}

// A folder nothing has been saved about yet has nothing to ask about.
func TestNothingToMoveIsSaidAsNothing(t *testing.T) {
	dirs, target := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, target)
	plan := decode[movedJSON](t, request(t, s, http.MethodPost, "/api/records/plan",
		map[string]any{"location": settings.WithApp}))
	if plan.What != "none" {
		t.Errorf("plan = %+v, want nothing to move", plan)
	}
}
