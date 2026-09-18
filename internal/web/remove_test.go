package web

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveAndEmpty(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)

	// Text files are not images, so they can't be removed.
	if rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"notes.txt"}, "how": "delete"}); rec.Code != http.StatusBadRequest {
		t.Errorf("removing notes.txt: %d, want 400", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("notes.txt was removed")
	}

	// An image can be moved aside, and then emptied for good.
	rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"Windows.iso"}, "how": "move-aside"})
	st := decode[stateJSON](t, rec)
	if rec.Code != http.StatusOK || st.Removed.Files != 1 || st.Removed.Bytes == 0 {
		t.Fatalf("move aside: %d, removed %+v", rec.Code, st.Removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "Windows.iso")); !os.IsNotExist(err) {
		t.Error("the file is still in the folder")
	}
	for _, it := range st.Report.Items {
		if it.Path == "Windows.iso" {
			t.Error("the removed file is still in the report")
		}
	}

	st = decode[stateJSON](t, request(t, s, http.MethodPost, "/api/removed/empty", nil))
	if st.Removed.Files != 0 {
		t.Errorf("after emptying: %+v", st.Removed)
	}
}

func TestUpdateRefusals(t *testing.T) {
	s := newServer(t, testDirs(t), sampleDrive(t))
	tests := []struct {
		name string
		body map[string]any
		want int
	}{
		{"unknown entry", map[string]any{"entry": "nope", "removal": "move-aside"}, http.StatusBadRequest},
		{"no removal choice", map[string]any{"entry": "netbootxyz"}, http.StatusBadRequest},
	}
	for _, tt := range tests {
		if rec := request(t, s, http.MethodPost, "/api/update", tt.body); rec.Code != tt.want {
			t.Errorf("%s: %d, want %d", tt.name, rec.Code, tt.want)
		}
	}
}

func TestArchiveAndRestore(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)

	type archive struct {
		Items []struct {
			Path         string `json:"path"`
			Name         string `json:"name"`
			Gone         string `json:"gone"`
			Restorable   bool   `json:"restorable"`
			Downloadable bool   `json:"downloadable"`
		} `json:"items"`
	}
	if got := decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil)); len(got.Items) != 0 {
		t.Fatalf("a fresh folder remembers %d images", len(got.Items))
	}

	// Move one aside, and one is deleted outright.
	for _, how := range []struct{ file, how string }{
		{"netboot.xyz.iso", "move-aside"},
		{"Windows.iso", "delete"},
	} {
		if rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{how.file}, "how": how.how}); rec.Code != http.StatusOK {
			t.Fatalf("remove %s: %d %s", how.file, rec.Code, rec.Body)
		}
	}
	got := decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	byPath := map[string]int{}
	for i, item := range got.Items {
		byPath[item.Path] = i
	}
	netboot, ok := byPath["netboot.xyz.iso"]
	if !ok || got.Items[netboot].Gone != "moved-aside" || !got.Items[netboot].Restorable || !got.Items[netboot].Downloadable {
		t.Errorf("moved-aside image: %+v", got.Items)
	}
	if windows, ok := byPath["Windows.iso"]; !ok || got.Items[windows].Gone != "removed" || got.Items[windows].Restorable {
		t.Errorf("deleted image: %+v", got.Items)
	}

	// Putting it back works once, and the archive forgets it after a scan.
	if rec := request(t, s, http.MethodPost, "/api/restore", map[string]any{"name": "netboot.xyz.iso"}); rec.Code != http.StatusOK {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(dir, "netboot.xyz.iso")); err != nil {
		t.Fatalf("the file didn't come back: %v", err)
	}
	if rec := request(t, s, http.MethodPost, "/api/restore", map[string]any{"name": "netboot.xyz.iso"}); rec.Code != http.StatusBadRequest {
		t.Errorf("restoring twice: %d, want 400", rec.Code)
	}
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("rescan: %d", rec.Code)
	}
	waitIdle(t, s)
	got = decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	for _, item := range got.Items {
		if item.Path == "netboot.xyz.iso" {
			t.Error("an image that came back is still listed as gone")
		}
	}

	// A file that disappears on its own is remembered too.
	if err := os.Remove(filepath.Join(dir, "HBCD_PE_x64.iso")); err != nil {
		t.Fatal(err)
	}
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	got = decode[archive](t, request(t, s, http.MethodGet, "/api/archive", nil))
	found := false
	for _, item := range got.Items {
		if item.Path == "HBCD_PE_x64.iso" && item.Gone == "vanished" && item.Name == "Hiren's BootCD PE" {
			found = true
		}
	}
	if !found {
		t.Errorf("a vanished file isn't remembered: %+v", got.Items)
	}
}
