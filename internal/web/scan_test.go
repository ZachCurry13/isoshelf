package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func TestScanCheckAndSettings(t *testing.T) {
	dirs := testDirs(t)
	drive := sampleDrive(t)
	s := newServer(t, dirs, "")

	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("scan without a folder: %d", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/target", map[string]string{"path": filepath.Join(drive, "missing")}); rec.Code != http.StatusBadRequest {
		t.Errorf("missing folder: %d", rec.Code)
	}
	rec := request(t, s, http.MethodPost, "/api/target", map[string]string{"path": drive, "profile": "ventoy"})
	if st := decode[stateJSON](t, rec); rec.Code != http.StatusOK || st.Target != drive || st.Report != nil {
		t.Fatalf("set target: %d %+v", rec.Code, st)
	}

	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d %s", rec.Code, rec.Body)
	}
	// A scan checks for updates too, unless Settings says not to (v0.3.2).
	st := waitIdle(t, s)
	if st.Report == nil || !st.Report.Checked || len(st.Report.Items) != len(sampledrive.Files)-1 || st.Error != "" {
		t.Fatalf("after scan: %+v", st)
	}

	if rec := request(t, s, http.MethodPost, "/api/check", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	st = waitIdle(t, s)
	if st.Report == nil || !st.Report.Checked {
		t.Fatalf("after check: %+v", st)
	}
	var pop *struct{ status, latest, updates string }
	for _, it := range st.Report.Items {
		if it.Path == "pop-os_22.04_amd64_intel_56.iso" {
			pop = &struct{ status, latest, updates string }{it.Status, it.Latest, it.Updates}
		}
	}
	if pop == nil || pop.status != "update available" || pop.latest != "58" || pop.updates != "download" {
		t.Errorf("Pop!_OS item: %+v", pop)
	}

	// The replace-old checkbox and the star are saved in the folder's state.
	rec = request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "popos-2204-intel", "keep_old": true, "starred": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("track: %d %s", rec.Code, rec.Body)
	}
	saved, err := state.Load(drive)
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Track("popos-2204-intel"); !got.KeepOld || !got.Starred {
		t.Errorf("saved track = %+v", got)
	}
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "nope", "starred": true}); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown entry: %d", rec.Code)
	}

	// The catalog says which entries are in the folder.
	cat := decode[struct {
		Entries []catalogEntryJSON `json:"entries"`
	}](t, request(t, s, http.MethodGet, "/api/catalog", nil))
	onTarget := map[string]bool{}
	for _, e := range cat.Entries {
		onTarget[e.ID] = e.OnTarget
	}
	if !onTarget["popos-2204-intel"] || onTarget["ubuntu-desktop-lts"] {
		t.Errorf("on_target: popos %v, ubuntu %v", onTarget["popos-2204-intel"], onTarget["ubuntu-desktop-lts"])
	}

	// A new server remembers the folder.
	again := newServer(t, dirs, "")
	if st := decode[stateJSON](t, request(t, again, http.MethodGet, "/api/state", nil)); st.Target != drive {
		t.Errorf("remembered target = %q, want %q", st.Target, drive)
	}
	remembered := decode[stateJSON](t, request(t, again, http.MethodGet, "/api/state", nil))
	if len(remembered.Recent) == 0 || remembered.Recent[0].Path != drive {
		t.Fatalf("recent targets = %v", remembered.Recent)
	}
	// The list says what each folder held, so it can be read without going
	// near a drive that may not be plugged in.
	if got := remembered.Recent[0]; got.Files == 0 || got.Bytes == 0 || got.LastUsed.IsZero() || got.ID == "" {
		t.Errorf("the remembered folder says nothing about itself: %+v", got)
	}
}

func TestBrowse(t *testing.T) {
	root := filepath.Join(t.TempDir(), "template", "iso")
	for _, dir := range []string{"archive", "Beta", ".isoshelf", "$RECYCLE.BIN"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "netboot.xyz.iso"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, testDirs(t), "")

	got := decode[browseJSON](t, request(t, s, http.MethodGet, "/api/browse?path="+root, nil))
	var names []string
	for _, f := range got.Folders {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "archive,Beta" {
		t.Errorf("folders = %v, want archive and Beta only", names)
	}
	if got.Path != root || got.Parent != filepath.Dir(root) || got.SuggestedProfile != "proxmox" || len(got.Roots) == 0 {
		t.Errorf("browse = %+v", got)
	}

	missing := decode[browseJSON](t, request(t, s, http.MethodGet, "/api/browse?path="+filepath.Join(root, "nope"), nil))
	if missing.Error == "" {
		t.Error("browsing a missing folder: want an error message")
	}
}
