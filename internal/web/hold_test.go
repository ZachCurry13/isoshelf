package web

import (
	"net/http"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/space"
)

// An image held at an older version (#2) is left alone by updating by
// itself, and Release lets it be updated again.
func TestAHeldImageIsLeftAloneUntilReleased(t *testing.T) {
	dirs, drive := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, drive)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	s.mu.Lock()
	entry := ""
	for _, item := range s.report.Items {
		if item.Status == check.UpdateAvailable && item.Entry != nil && item.Entry.Updates() == "download" {
			entry = item.Entry.ID
			break
		}
	}
	if entry == "" {
		s.mu.Unlock()
		t.Skip("this sample drive has no updates waiting")
	}
	tr := s.st.Track(entry)
	tr.Hold = "1.0"
	s.st.SetTrack(entry, tr)
	s.queueUpdatesLocked(space.Usage{Free: 500 << 30, Total: 1000 << 30})
	queued := false
	for _, j := range s.queue {
		queued = queued || j.entry == entry
	}
	for _, r := range s.running {
		queued = queued || r.job.entry == entry
		r.cancel()
	}
	s.queue = nil
	s.mu.Unlock()
	waitIdle(t, s)
	if queued {
		t.Error("updating by itself queued an image held at an older version")
	}

	if rec := request(t, s, http.MethodPost, "/api/track", map[string]any{"entry": entry, "release_hold": true}); rec.Code != http.StatusOK {
		t.Fatalf("release: %d %s", rec.Code, rec.Body)
	}
	if got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Tracks[entry]; got.Hold != "" {
		t.Errorf("after Release the hold is still %q", got.Hold)
	}
}

// The versions a project still publishes, newest first, from the recorded
// responses; and a reason instead for an image isoshelf only links to.
func TestVersionsLists(t *testing.T) {
	s := newServer(t, testDirs(t), sampleDrive(t))
	got := decode[struct {
		Versions []versionJSON `json:"versions"`
	}](t, request(t, s, http.MethodGet, "/api/versions?entry=netbootxyz", nil))
	if len(got.Versions) < 2 || !got.Versions[0].Newest || got.Versions[1].Newest {
		t.Errorf("netbootxyz versions = %+v", got.Versions)
	}
	why := decode[struct {
		Why string `json:"why"`
	}](t, request(t, s, http.MethodGet, "/api/versions?entry=windows-11-x64", nil))
	if why.Why == "" {
		t.Error("an image isoshelf only links to gave no reason")
	}
}
