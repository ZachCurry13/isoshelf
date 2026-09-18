package update

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func TestRemoveRules(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	iso := make([]byte, 40000)
	copy(iso[32769:], "CD001")
	write("netboot.xyz.iso", "an image")
	write("notes.txt", "just notes")
	write("photos.zip", "PK\x03\x04 not an image")
	write("sub/Windows.iso", string(iso))
	write(filepath.Join(state.DirName, "state.json"), "{}")

	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Ventoy)
	for _, rel := range []string{"netboot.xyz.iso", "notes.txt", "photos.zip", "sub/Windows.iso"} {
		if err := st.Placed(dir, rel, state.FileRecord{}); err != nil {
			t.Fatal(err)
		}
	}

	// Images may go; other files and isoshelf's own files may not.
	for rel, wantOK := range map[string]bool{
		"netboot.xyz.iso":             true, // recognized
		"sub/Windows.iso":             true, // unrecognized, but an image
		"notes.txt":                   false,
		"photos.zip":                  false,
		state.DirName + "/state.json": false,
		"../outside.iso":              false,
		"missing.iso":                 false,
	} {
		err := Removable(dir, rel, st, cat)
		if (err == nil) != wantOK {
			t.Errorf("Removable(%q) = %v, want ok=%v", rel, err, wantOK)
		}
	}

	removed, err := Remove(dir, []string{"netboot.xyz.iso", "sub/Windows.iso"}, DeleteNow, st, cat, now)
	if err != nil || len(removed) != 2 {
		t.Fatalf("Remove() = %v, %v", removed, err)
	}
	for _, rel := range removed {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s is still there", rel)
		}
		if _, ok := st.Files[rel]; ok {
			t.Errorf("%s still has a record", rel)
		}
	}

	if _, err := Remove(dir, []string{"notes.txt"}, DeleteNow, st, cat, now); err == nil {
		t.Error("removing a text file: want an error")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("notes.txt was deleted")
	}
	if _, err := Remove(dir, []string{"photos.zip"}, "shred", st, cat, now); err == nil {
		t.Error("unknown removal method: want an error")
	}
}
