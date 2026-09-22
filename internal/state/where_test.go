package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The default puts a folder's records inside it, where they have always been.
func TestRecordsLiveInTheFolderByDefault(t *testing.T) {
	dir := t.TempDir()
	got, err := Home("").File(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, DirName, fileName); got != want {
		t.Errorf("File = %q, want %q", got, want)
	}
}

// Kept away from the folder, every folder's records live together, one file
// each, named for the folder so two folders never share a file.
func TestRecordsKeptAwayAreOneFilePerFolder(t *testing.T) {
	home := Home(t.TempDir())
	one, err := home.File(filepath.Join("/images", "ventoy"))
	if err != nil {
		t.Fatal(err)
	}
	two, err := home.File(filepath.Join("/images", "proxmox"))
	if err != nil {
		t.Fatal(err)
	}
	if one == two {
		t.Fatal("two folders were given the same records file")
	}
	for _, name := range []string{one, two} {
		if filepath.Dir(name) != filepath.Join(string(home), recordsDir) {
			t.Errorf("%q isn't in the records folder", name)
		}
		if strings.ContainsAny(filepath.Base(name), `/\:`) {
			t.Errorf("%q isn't a usable file name", filepath.Base(name))
		}
	}
	// And the same folder always gets the same file, however it is written.
	again, err := home.File(filepath.Join("/images", "ventoy", "..", "ventoy"))
	if err != nil {
		t.Fatal(err)
	}
	if again != one {
		t.Errorf("the same folder written another way got %q, want %q", again, one)
	}
}

// Records saved away from the folder are read back from there, and the folder
// is left untouched - which is the whole point for a drive isoshelf should
// not be writing to.
func TestRecordsRoundTripAwayFromTheFolder(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	st := New("folder")
	st.Tracks["debian"] = Track{Starred: true}
	if err := home.Save(st, target); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, DirName)); err == nil {
		t.Error("saving records elsewhere still wrote to the folder")
	}
	back, err := home.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Tracks["debian"].Starred || back.TargetID != st.TargetID {
		t.Errorf("read back %+v, want the same records", back)
	}
	// The default place knows nothing about this folder, as it should.
	fresh, err := Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.Tracks) != 0 {
		t.Errorf("the folder itself held records too: %+v", fresh.Tracks)
	}
}

// Changing where a folder's records live moves them. Anything else looks to
// the user like isoshelf forgot the folder.
func TestMoveTakesTheRecordsWithIt(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	st := New("folder")
	st.Tracks["debian"] = Track{Starred: true}
	if err := st.Save(target); err != nil {
		t.Fatal(err)
	}

	moved, err := Move("", home, target)
	if err != nil {
		t.Fatal(err)
	}
	if !moved.Moved || moved.Kept {
		t.Errorf("move = %+v, want it moved", moved)
	}
	if _, err := os.Stat(moved.From); err == nil {
		t.Error("the records were copied, not moved: the old file is still there")
	}
	back, err := home.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Tracks["debian"].Starred {
		t.Error("the moved records lost what was in them")
	}

	// And back again.
	if moved, err := Move(home, "", target); err != nil || !moved.Moved {
		t.Errorf("moving back: %+v, %v", moved, err)
	}
	if back, err := Load(target); err != nil || !back.Tracks["debian"].Starred {
		t.Errorf("moving back lost the records: %v", err)
	}
}

// A folder nothing is known about yet has nothing to move, which is not a
// failure - it is the ordinary case of choosing before the first scan.
func TestMoveWithNothingToMove(t *testing.T) {
	moved, err := Move("", Home(t.TempDir()), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if moved.Moved || moved.Kept {
		t.Errorf("move = %+v, want nothing done", moved)
	}
}

// Records already waiting where they are being moved to are the ones to use,
// and the old ones are left alone rather than thrown away. Nothing isoshelf
// wrote is deleted unless the user chose it.
func TestMoveKeepsWhatIsAlreadyThere(t *testing.T) {
	target, home := t.TempDir(), Home(t.TempDir())
	old := New("folder")
	old.Tracks["debian"] = Track{Starred: true}
	if err := old.Save(target); err != nil {
		t.Fatal(err)
	}
	waiting := New("folder")
	waiting.Tracks["fedora"] = Track{Starred: true}
	if err := home.Save(waiting, target); err != nil {
		t.Fatal(err)
	}

	moved, err := Move("", home, target)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Moved || !moved.Kept {
		t.Errorf("move = %+v, want the waiting records kept", moved)
	}
	if _, err := os.Stat(moved.From); err != nil {
		t.Error("the records that couldn't be moved were deleted instead")
	}
	back, err := home.Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if !back.Tracks["fedora"].Starred || back.Tracks["debian"].Starred {
		t.Errorf("the wrong records won: %+v", back.Tracks)
	}
}

func TestCleanHome(t *testing.T) {
	if _, err := CleanHome("   "); err == nil {
		t.Error("an empty folder was accepted")
	}
	if _, err := CleanHome("records"); err == nil {
		t.Error("a relative folder was accepted; it would mean somewhere different every run")
	}
	abs := t.TempDir()
	got, err := CleanHome(" " + abs + " ")
	if err != nil || string(got) != abs {
		t.Errorf("CleanHome(%q) = %q, %v", abs, got, err)
	}
}
