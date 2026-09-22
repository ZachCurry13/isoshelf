package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// fakeDownloads stands in for the real download, so the queue can be driven
// one step at a time: each download waits until the test says how it ends.
type fakeDownloads struct {
	mu      sync.Mutex
	started []string
	finish  map[string]chan error
}

func useFakeDownloads(s *Server) *fakeDownloads {
	f := &fakeDownloads{finish: map[string]chan error{}}
	s.runJob = func(ctx context.Context, j *job) (string, error) {
		// The real download reports progress as it goes, and the dock draws
		// from it now that the scan's card is no longer shared.
		s.mu.Lock()
		if s.downloading != nil {
			s.downloading.progress = inventory.Progress{
				Stage: inventory.Stage("downloading"), File: j.name, Done: j.size / 4, Total: j.size,
			}
		}
		s.mu.Unlock()
		select {
		case err := <-f.channel(j.entry, true):
			return "", err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return f
}

func (f *fakeDownloads) channel(entry string, start bool) chan error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if start {
		f.started = append(f.started, entry)
	}
	if f.finish[entry] == nil {
		f.finish[entry] = make(chan error, 1)
	}
	return f.finish[entry]
}

func (f *fakeDownloads) end(entry string, err error) { f.channel(entry, false) <- err }

func (f *fakeDownloads) order() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.started, " ")
}

// waitDownloads polls the state until the queue looks the way want says.
func waitDownloads(t *testing.T, s *Server, want func(stateJSON) bool) stateJSON {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var st stateJSON
	for time.Now().Before(deadline) {
		st = decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
		if want(st) {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the queue never got there: %+v", st.Downloads)
	return st
}

func queued(d downloadsJSON) string {
	var names []string
	for _, j := range d.Queued {
		names = append(names, j.Entry)
	}
	return strings.Join(names, " ")
}

func current(d downloadsJSON) string {
	if d.Current == nil {
		return ""
	}
	return d.Current.Entry
}

func add(t *testing.T, s *Server, entry string) int {
	t.Helper()
	rec := request(t, s, http.MethodPost, "/api/update", map[string]any{"entry": entry, "removal": "keep"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("add %s: %d %s", entry, rec.Code, rec.Body)
	}
	return decode[struct {
		ID int `json:"id"`
	}](t, rec).ID
}

func TestDownloadQueue(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	f := useFakeDownloads(s)

	// The first one starts at once; the others wait, in the order added.
	add(t, s, "ubuntu-desktop-lts")
	arch := add(t, s, "archlinux")
	gparted := add(t, s, "gparted-live")
	st := waitDownloads(t, s, func(st stateJSON) bool {
		// Its progress too: the slot is filled a moment before the download
		// itself starts reporting.
		return current(st.Downloads) == "ubuntu-desktop-lts" && st.Downloads.Current.Stage != ""
	})
	if got := queued(st.Downloads); got != "archlinux gparted-live" {
		t.Fatalf("waiting: %q", got)
	}
	// The download's progress rides with the download; the scan's card stays
	// empty, so the page can show a scan beside it.
	if st.Run != nil {
		t.Errorf("a download filled the scan's card: %+v", st.Run)
	}
	if st.Downloads.Current.Total == 0 {
		t.Errorf("the dock has no progress to draw: %+v", st.Downloads.Current)
	}

	// Asking twice for the same image is refused, running or waiting.
	for _, entry := range []string{"ubuntu-desktop-lts", "archlinux"} {
		if rec := request(t, s, http.MethodPost, "/api/update", map[string]any{"entry": entry, "removal": "keep"}); rec.Code != http.StatusConflict {
			t.Errorf("adding %s twice: %d, want 409", entry, rec.Code)
		}
	}

	// The waiting ones can be reordered and dropped.
	d := decode[downloadsJSON](t, request(t, s, http.MethodPost, "/api/queue/move", map[string]any{"id": gparted, "position": 0}))
	if got := queued(d); got != "gparted-live archlinux" {
		t.Errorf("after moving to the top: %q", got)
	}
	d = decode[downloadsJSON](t, request(t, s, http.MethodPost, "/api/queue/move", map[string]any{"id": gparted, "position": 99}))
	if got := queued(d); got != "archlinux gparted-live" {
		t.Errorf("after moving past the end: %q", got)
	}
	d = decode[downloadsJSON](t, request(t, s, http.MethodPost, "/api/queue/drop", map[string]any{"id": arch}))
	if got := queued(d); got != "gparted-live" {
		t.Errorf("after dropping Arch: %q", got)
	}
	if rec := request(t, s, http.MethodPost, "/api/queue/move", map[string]any{"id": arch, "position": 0}); rec.Code != http.StatusBadRequest {
		t.Errorf("moving a dropped download: %d, want 400", rec.Code)
	}

	// Removing files and stars don't wait for downloads, and both reach the
	// folder's state on disk.
	if rec := request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"Windows.iso"}, "how": "move-aside"}); rec.Code != http.StatusOK {
		t.Errorf("remove during downloads: %d %s", rec.Code, rec.Body)
	}
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": "netbootxyz", "starred": true}); rec.Code != http.StatusOK {
		t.Errorf("star during downloads: %d %s", rec.Code, rec.Body)
	}
	disk, err := state.Load(dir)
	if err != nil || !disk.Track("netbootxyz").Starred {
		t.Errorf("the star didn't reach the folder's state: %v", err)
	}
	if _, ok := disk.Files["Windows.iso"]; ok || len(disk.Past) == 0 || disk.Past[0].Path != "Windows.iso" {
		t.Errorf("the removal didn't reach the folder's state: past %+v", disk.Past)
	}

	// A scan no longer waits for the downloads: it has its own slot, so the
	// page isn't locked for as long as a queue takes.
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan during downloads: %d %s", rec.Code, rec.Body)
	}
	st = waitDownloads(t, s, func(st stateJSON) bool { return st.Run == nil })
	if current(st.Downloads) != "ubuntu-desktop-lts" {
		t.Errorf("the download didn't survive the scan beside it: %q", current(st.Downloads))
	}
	// What the scan saved went onto the records as they were, so the star it
	// was never told about is still there.
	if disk, err := state.Load(dir); err != nil || !disk.Track("netbootxyz").Starred {
		t.Errorf("the scan saved over the star: %v", err)
	}

	// Switching folders and emptying the archive do still wait.
	rec := request(t, s, http.MethodPost, "/api/removed/empty", nil)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "downloads") {
		t.Errorf("emptying the archive during downloads: %d %s", rec.Code, rec.Body)
	}

	// Each one that ends makes way for the next; a failure is recorded and
	// the queue carries on.
	f.end("ubuntu-desktop-lts", nil)
	st = waitDownloads(t, s, func(st stateJSON) bool { return current(st.Downloads) == "gparted-live" })
	if len(st.Downloads.Finished) != 1 || st.Downloads.Finished[0].Outcome != "done" {
		t.Errorf("finished: %+v", st.Downloads.Finished)
	}
	f.end("gparted-live", errors.New("checksum mismatch"))

	// With the queue empty and something placed, the folder is scanned once.
	st = waitIdle(t, s)
	if got := f.order(); got != "ubuntu-desktop-lts gparted-live" {
		t.Errorf("downloaded in the order %q", got)
	}
	if len(st.Downloads.Finished) != 2 || st.Downloads.Finished[0].Outcome != "failed" || st.Downloads.Finished[0].Message != "checksum mismatch" {
		t.Errorf("finished: %+v", st.Downloads.Finished)
	}
	if !strings.Contains(st.Error, "GParted") {
		t.Errorf("the failure isn't shown: %q", st.Error)
	}
	s.mu.Lock()
	placed := s.placed
	s.mu.Unlock()
	if st.UpdatedAt == nil || placed {
		t.Error("the folder wasn't scanned after the downloads")
	}

	if d := decode[downloadsJSON](t, request(t, s, http.MethodPost, "/api/queue/clear", nil)); len(d.Finished) != 0 {
		t.Errorf("after clearing: %+v", d.Finished)
	}
}

func TestStoppingDownloads(t *testing.T) {
	s := newServer(t, testDirs(t), sampleDrive(t))
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	f := useFakeDownloads(s)

	// Dropping the one that is downloading stops it, and the next starts.
	ubuntu := add(t, s, "ubuntu-desktop-lts")
	add(t, s, "archlinux")
	add(t, s, "gparted-live")
	waitDownloads(t, s, func(st stateJSON) bool { return current(st.Downloads) == "ubuntu-desktop-lts" })
	request(t, s, http.MethodPost, "/api/queue/drop", map[string]any{"id": ubuntu})
	st := waitDownloads(t, s, func(st stateJSON) bool { return current(st.Downloads) == "archlinux" })
	if len(st.Downloads.Finished) != 1 || st.Downloads.Finished[0].Outcome != "stopped" {
		t.Errorf("finished: %+v", st.Downloads.Finished)
	}

	// Stop stops everything, and nothing was placed, so there is no scan.
	request(t, s, http.MethodPost, "/api/cancel", nil)
	st = waitIdle(t, s)
	if len(st.Downloads.Queued) != 0 || st.Downloads.Current != nil {
		t.Errorf("after stopping: %+v", st.Downloads)
	}
	if got := f.order(); got != "ubuntu-desktop-lts archlinux" {
		t.Errorf("started %q", got)
	}
}

// The one file that can't be removed during downloads is the one the running
// download is about to replace.
func TestRemovingWhatIsBeingReplaced(t *testing.T) {
	dir := sampleDrive(t)
	s := newServer(t, testDirs(t), dir)
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	f := useFakeDownloads(s)

	rec := request(t, s, http.MethodPost, "/api/update", map[string]any{"entry": "netbootxyz", "removal": "move-aside"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	waitDownloads(t, s, func(st stateJSON) bool { return current(st.Downloads) == "netbootxyz" })

	rec = request(t, s, http.MethodPost, "/api/remove", map[string]any{"paths": []string{"netboot.xyz.iso"}, "how": "delete"})
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "being replaced") {
		t.Errorf("removing the file being replaced: %d %s", rec.Code, rec.Body)
	}
	if rec := request(t, s, http.MethodPost, "/api/removed/empty", nil); rec.Code != http.StatusConflict {
		t.Errorf("emptying the archive during a download: %d, want 409", rec.Code)
	}
	f.end("netbootxyz", errors.New("stopped for the test"))
	waitIdle(t, s)
}
