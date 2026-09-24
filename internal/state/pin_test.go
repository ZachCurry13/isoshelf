package state

import (
	"slices"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// A pin belongs to the file: a scan that finds the same file keeps it, and a
// file that changed is a different file, so it goes.
func TestAPinGoesWhenTheFileChanges(t *testing.T) {
	st := New(scan.Ventoy)
	at := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	file := scan.File{Path: "debian.iso", Size: 100, ModTime: at}
	st.RecordScan(&scan.Result{Files: []scan.File{file}}, at)
	if err := st.Pin("debian.iso", true); err != nil {
		t.Fatal(err)
	}
	st.RecordScan(&scan.Result{Files: []scan.File{file}}, at.Add(time.Hour))
	if !slices.Equal(st.Pinned(), []string{"debian.iso"}) {
		t.Fatalf("pinned %v after a scan that found the same file", st.Pinned())
	}
	file.Size = 200
	st.RecordScan(&scan.Result{Files: []scan.File{file}}, at.Add(2*time.Hour))
	if len(st.Pinned()) != 0 {
		t.Errorf("pinned %v after the file changed", st.Pinned())
	}
	if err := st.Pin("gone.iso", true); err == nil {
		t.Error("pinned a file the scan never saw")
	}
}

// The per-image answers of before v0.7.0: keep both becomes a pin, and the
// rest follow Settings from then on, counted so the page can say so once.
func TestTheOldPerImageAnswersBecomePins(t *testing.T) {
	st := New(scan.Ventoy)
	st.Files["mint.iso"] = FileRecord{Entry: "mint"}
	st.Files["tails.iso"] = FileRecord{Entry: "tails"}
	st.Tracks["mint"] = Track{OldFiles: "keep", Starred: true}
	st.Tracks["tails"] = Track{OldFiles: "archive"}
	st.Tracks["debian"] = Track{OldFiles: "replace"}
	st.Tracks["kali"] = Track{KeepOld: true}

	got := st.MigrateChoices("replace")
	if got.Pinned != 2 || got.Following != 1 {
		t.Errorf("migrated %+v, want 2 pinned (mint, kali) and 1 following Settings (tails)", got)
	}
	if !st.Files["mint.iso"].Pinned || st.Files["tails.iso"].Pinned {
		t.Errorf("mint pinned %v, tails pinned %v", st.Files["mint.iso"].Pinned, st.Files["tails.iso"].Pinned)
	}
	if tr := st.Tracks["mint"]; tr.OldFiles != "" || !tr.Starred {
		t.Errorf("mint's track is %+v: the answer should be gone and the star kept", tr)
	}
	if _, ok := st.Tracks["debian"]; ok {
		t.Error("an image left with nothing to remember still has a track")
	}
	if again := st.MigrateChoices("replace"); again != (Migrated{}) {
		t.Errorf("a second run changed %+v", again)
	}
}
