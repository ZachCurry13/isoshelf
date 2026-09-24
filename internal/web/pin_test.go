package web

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// A folder whose images had answers of their own before v0.7.0 gets pins in
// their place when it is opened, and the page says so - once. Opened again,
// the pins are still there and there is nothing more to say.
func TestOldAnswersBecomePinsAndThePageSaysSoOnce(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	path := filepath.Join(target, "mint.iso")
	if err := os.WriteFile(path, []byte("an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st := state.New(scan.Folder)
	st.Files["mint.iso"] = state.FileRecord{Size: info.Size(), ModTime: info.ModTime(), Entry: "linuxmint"}
	st.Tracks["linuxmint"] = state.Track{KeepOld: true}
	if err := st.Save(target); err != nil {
		t.Fatal(err)
	}

	first := decode[stateJSON](t, request(t, newServer(t, dirs, target), http.MethodGet, "/api/state", nil))
	if !slices.Contains(first.Pinned, "mint.iso") {
		t.Errorf("pinned %v, want mint.iso: it was set to keep both copies", first.Pinned)
	}
	if said := strings.Join(first.Warnings, " "); !strings.Contains(said, "1 image set to keep both copies now has pinned files") {
		t.Errorf("the page said %q, want it to say what became of the old answer", said)
	}

	again := decode[stateJSON](t, request(t, newServer(t, dirs, target), http.MethodGet, "/api/state", nil))
	if !slices.Contains(again.Pinned, "mint.iso") {
		t.Errorf("opened again, pinned %v: the pin should have been saved", again.Pinned)
	}
	if said := strings.Join(again.Warnings, " "); strings.Contains(said, "pins") {
		t.Errorf("opened again, the page still says %q", said)
	}
}
