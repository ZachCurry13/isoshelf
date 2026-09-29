package web

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// The usage page counts from the folder's own records, and says where
// isoshelf is running.
func TestUsageCountsTheWeekAndDescribesTheSystem(t *testing.T) {
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
	st.Files["mint.iso"] = state.FileRecord{
		Size: info.Size(), ModTime: info.ModTime(), Entry: "linuxmint",
		Origin: state.Origin{How: state.OriginDownload, At: time.Now().Add(-time.Minute)},
	}
	if err := st.Save(target); err != nil {
		t.Fatal(err)
	}

	got := decode[usageJSON](t, request(t, newServer(t, dirs, target), http.MethodGet, "/api/usage", nil))
	if len(got.Weeks) != usageWeeks {
		t.Fatalf("%d weeks, want %d", len(got.Weeks), usageWeeks)
	}
	if d := got.Weeks[0].Downloaded; d.Files != 1 || d.Bytes != info.Size() {
		t.Errorf("downloaded this week %+v, want the one file", d)
	}
	sys := got.System
	if sys.Files != 1 || sys.Bytes != info.Size() {
		t.Errorf("the folder holds %d files of %d bytes, want 1 of %d", sys.Files, sys.Bytes, info.Size())
	}
	if sys.Mode != "desktop" || sys.OS != runtime.GOOS || sys.Folder != target {
		t.Errorf("system %+v: want a desktop isoshelf on %s looking at %s", sys, runtime.GOOS, target)
	}
	if sys.Started.IsZero() {
		t.Error("no start time")
	}
}

// With no folder open there is nothing to count, and the page still gets an
// answer it can draw.
func TestUsageWithNoFolder(t *testing.T) {
	got := decode[usageJSON](t, request(t, newServer(t, testDirs(t), ""), http.MethodGet, "/api/usage", nil))
	if got.Weeks == nil || len(got.Weeks) != 0 {
		t.Errorf("weeks %v, want an empty list", got.Weeks)
	}
	if got.System.Mode == "" {
		t.Error("no mode")
	}
}
