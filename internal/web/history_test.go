package web

import (
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// History lists what happened in the folder, named the way the catalog names
// the image, and only as far back as it is asked.
func TestHistoryListsWhatHappenedAsFarBackAsAsked(t *testing.T) {
	dirs, target := testDirs(t), t.TempDir()
	now := time.Now()
	st := state.New(scan.Folder)
	st.Past = []state.ArchiveEntry{
		{Path: "mint-old.iso", Entry: "linuxmint-cinnamon", Version: "21", Size: 5, Gone: state.GoneRemoved, GoneAt: now.Add(-48 * time.Hour)},
		{Path: "ancient.iso", Size: 6, Gone: state.GoneVanished, GoneAt: now.AddDate(0, 0, -200)},
	}
	if err := st.Save(target); err != nil {
		t.Fatal(err)
	}
	srv := newServer(t, dirs, target)

	type reply struct {
		Events []eventJSON `json:"events"`
	}
	recent := decode[reply](t, request(t, srv, http.MethodGet, "/api/history?days=30", nil))
	if len(recent.Events) != 1 {
		t.Fatalf("last 30 days: %d events, want the one from two days ago", len(recent.Events))
	}
	e := recent.Events[0]
	if e.Kind != state.EventDeleted || e.Name == e.Path || e.Name == "" || e.Version != "21" {
		t.Errorf("%+v: want a deletion of a catalog image, named by the catalog", e)
	}
	if all := decode[reply](t, request(t, srv, http.MethodGet, "/api/history", nil)); len(all.Events) != 2 {
		t.Errorf("everything: %d events, want 2", len(all.Events))
	}
}

func TestHistoryWithNoFolderIsAnEmptyList(t *testing.T) {
	got := decode[map[string][]eventJSON](t, request(t, newServer(t, testDirs(t), ""), http.MethodGet, "/api/history?days=30", nil))
	if got["events"] == nil || len(got["events"]) != 0 {
		t.Errorf("events %v, want an empty list the page can draw", got["events"])
	}
}

// The facts about where isoshelf runs ride on the page's state, for the
// Version row in Settings.
func TestStateSaysWhereIsoshelfRuns(t *testing.T) {
	got := decode[stateJSON](t, request(t, newServer(t, testDirs(t), ""), http.MethodGet, "/api/state", nil))
	sys := got.System
	if sys.OS != runtime.GOOS || sys.Arch != runtime.GOARCH || sys.Mode != "desktop" {
		t.Errorf("system %+v: want a desktop isoshelf on %s/%s", sys, runtime.GOOS, runtime.GOARCH)
	}
	if sys.Started.IsZero() || sys.CatalogRevision == 0 {
		t.Errorf("system %+v: want a start time and a catalog revision", sys)
	}
}
