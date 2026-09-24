package appupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return "(none)"
	}
	return string(b)
}

// A portable folder is updated as a whole, and undoing it puts every program
// back exactly as it was.
func TestSwapAndRestoreAPortableFolder(t *testing.T) {
	suffix := ownSuffix(t)
	dir := realTempDir(t)
	exe := filepath.Join(dir, "isoshelf-"+suffix)
	for _, p := range Platforms {
		write(t, filepath.Join(dir, "isoshelf-"+p), "old "+p)
	}
	targets, err := Targets(exe, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != len(Platforms) {
		t.Fatalf("%d targets, want every program in the folder: %+v", len(targets), targets)
	}
	staged := stageFake(t, dir, targets)

	swapped, err := staged.Swap("v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if swapped.Run != exe {
		t.Errorf("would start %s, want %s", swapped.Run, exe)
	}
	for _, p := range Platforms {
		if got := read(t, filepath.Join(dir, "isoshelf-"+p)); got != "new "+p {
			t.Errorf("%s is %q after the swap", p, got)
		}
	}
	if err := swapped.Restore(); err != nil {
		t.Fatal(err)
	}
	for _, p := range Platforms {
		if got := read(t, filepath.Join(dir, "isoshelf-"+p)); got != "old "+p {
			t.Errorf("%s is %q after undoing, want the old program back", p, got)
		}
	}
}

// A single download named for its version takes the plain name, and undoing
// that puts back both the old program and a file that already had the plain
// name - the case where the order of undoing matters.
func TestSwapToThePlainName(t *testing.T) {
	suffix := ownSuffix(t)
	dir := realTempDir(t)
	exe := filepath.Join(dir, "isoshelf-v0.5.3-"+suffix)
	plain := filepath.Join(dir, "isoshelf-"+suffix)
	write(t, exe, "old versioned")
	write(t, plain, "an older plain copy")
	targets, err := Targets(exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if targets[0].To != plain {
		t.Fatalf("goes to %s, want the plain name %s", targets[0].To, plain)
	}
	swapped, err := stageFake(t, dir, targets).Swap("v0.5.3")
	if err != nil {
		t.Fatal(err)
	}
	if got := read(t, plain); got != "new "+suffix {
		t.Errorf("the plain name holds %q", got)
	}
	if _, err := os.Stat(exe); err == nil {
		t.Error("the versioned name is still there after the swap")
	}
	if err := swapped.Restore(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, exe); got != "old versioned" {
		t.Errorf("the running program is %q after undoing", got)
	}
	if got := read(t, plain); got != "an older plain copy" {
		t.Errorf("the plain name holds %q after undoing, want what was there before", got)
	}
}

// Somebody who named the program themselves meant it.
func TestARenamedProgramKeepsItsName(t *testing.T) {
	suffix := ownSuffix(t)
	exe := filepath.Join(realTempDir(t), "my-isoshelf")
	write(t, exe, "old")
	targets, err := Targets(exe, false)
	if err != nil {
		t.Fatal(err)
	}
	if targets[0].To != exe || targets[0].Suffix != suffix {
		t.Errorf("target %+v, want it to stay %s", targets[0], exe)
	}
}

// If putting one program in place fails, nothing is left half done.
func TestAFailedSwapChangesNothing(t *testing.T) {
	suffix := ownSuffix(t)
	dir := realTempDir(t)
	for _, p := range Platforms {
		write(t, filepath.Join(dir, "isoshelf-"+p), "old "+p)
	}
	targets, _ := Targets(filepath.Join(dir, "isoshelf-"+suffix), true)
	staged := stageFake(t, dir, targets)
	os.Remove(staged.Files[len(staged.Files)-1].New) // the last one never arrived
	if _, err := staged.Swap("v1.0.0"); err == nil {
		t.Fatal("a swap with a program missing succeeded")
	}
	for _, p := range Platforms {
		if got := read(t, filepath.Join(dir, "isoshelf-"+p)); got != "old "+p {
			t.Errorf("%s is %q after a failed swap, want it untouched", p, got)
		}
	}
}

// stageFake puts "new <suffix>" programs in a staging folder, as Prepare would.
func stageFake(t *testing.T, dir string, targets []Target) *Staged {
	t.Helper()
	stage := filepath.Join(dir, StageDir)
	os.MkdirAll(stage, 0o755)
	s := &Staged{Version: "v9.0.0", Dir: stage}
	for _, target := range targets {
		n := filepath.Join(stage, "isoshelf-v9.0.0-"+target.Suffix)
		write(t, n, "new "+target.Suffix)
		s.Files = append(s.Files, StagedFile{Target: target, New: n})
	}
	return s
}

// realTempDir is t.TempDir by its full name. Windows may hand out a folder's
// short 8.3 name (RUNNER~1 for runneradmin, on GitHub's machines), and
// Targets resolves the program's real path, so the two only compare equal
// when both are spelled out in full.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
