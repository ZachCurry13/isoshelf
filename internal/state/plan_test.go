package state

import (
	"os"
	"testing"
	"time"
)

// Asking first is only worth anything if asking changes nothing: the page asks
// before every move, and somebody who says no must find everything as it was.
func TestPlanMovesNothing(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	if err := New("folder").Save(target); err != nil {
		t.Fatal(err)
	}

	plan, err := Plan("", home, target)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Moved || plan.Kept {
		t.Errorf("plan = %+v, want it to say the records would move", plan)
	}
	if plan.FromSaved.IsZero() || !plan.ToSaved.IsZero() {
		t.Errorf("plan = %+v, want a saved time for the records there are and none for the ones there aren't", plan)
	}
	if _, err := os.Stat(plan.From); err != nil {
		t.Errorf("planning a move moved the records: %v", err)
	}
	if _, err := os.Stat(plan.To); err == nil {
		t.Error("planning a move wrote the records to where they would go")
	}
}

// When both places have records, the question says which set wins and how old
// each is, so somebody can say no before the wrong set is the one in use.
func TestPlanSaysWhichRecordsWouldWin(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	if err := New("folder").Save(target); err != nil {
		t.Fatal(err)
	}
	if err := home.Save(New("folder"), target); err != nil {
		t.Fatal(err)
	}
	waiting, err := home.File(target)
	if err != nil {
		t.Fatal(err)
	}
	then := time.Now().Add(-72 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(waiting, then, then); err != nil {
		t.Fatal(err)
	}

	plan, err := Plan("", home, target)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Moved || !plan.Kept {
		t.Errorf("plan = %+v, want the records already there kept", plan)
	}
	if !plan.ToSaved.Equal(then) {
		t.Errorf("the records already there were saved %v, want %v", plan.ToSaved, then)
	}
	if !plan.FromSaved.After(then) {
		t.Errorf("the records being moved were saved %v, want after %v", plan.FromSaved, then)
	}
}

// Records only where they are going: nothing moves, and those are the ones
// read from now on. The page has to be able to tell this apart from a folder
// nothing is known about at all.
func TestPlanWithRecordsOnlyWhereTheyAreGoing(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	if err := home.Save(New("folder"), target); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan("", home, target)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Moved || plan.Kept || !plan.FromSaved.IsZero() || plan.ToSaved.IsZero() {
		t.Errorf("plan = %+v, want nothing to move and records waiting where they go", plan)
	}

	// And choosing the place they already are changes nothing at all.
	same, err := Plan(home, home, target)
	if err != nil {
		t.Fatal(err)
	}
	if same.From != same.To || same.Moved || same.Kept {
		t.Errorf("plan = %+v, want nothing to do", same)
	}
}
