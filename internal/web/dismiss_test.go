package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/space"
)

// Dismissing an update (#56) sets a date - or for good - that holds whatever
// comes out meanwhile, automatic updates leave the image alone, and undo
// brings it back.
func TestADismissedUpdateIsLeftAloneUntilUndone(t *testing.T) {
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
	s.mu.Unlock()
	if entry == "" {
		t.Skip("this sample drive has no updates waiting")
	}

	for _, c := range []struct {
		dismiss string
		want    time.Duration
		forever bool
	}{{"30", 30 * 24 * time.Hour, false}, {"forever", 0, true}} {
		if rec := request(t, s, http.MethodPost, "/api/track", map[string]string{"entry": entry, "dismiss": c.dismiss}); rec.Code != http.StatusOK {
			t.Fatalf("dismiss %s: %d %s", c.dismiss, rec.Code, rec.Body)
		}
		got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Tracks[entry]
		if got.DismissedForever != c.forever {
			t.Errorf("dismiss %s: forever=%v", c.dismiss, got.DismissedForever)
		}
		if !c.forever {
			if left := time.Until(got.DismissedUntil); left < c.want-time.Hour || left > c.want+time.Hour {
				t.Errorf("dismiss %s: until %v, want about %v from now", c.dismiss, got.DismissedUntil, c.want)
			}
		}
		s.mu.Lock()
		queued := false
		s.queueUpdatesLocked(space.Usage{Free: 500 << 30, Total: 1000 << 30})
		for _, j := range s.queue {
			queued = queued || j.entry == entry
		}
		if s.downloading != nil && s.downloading.job.entry == entry {
			queued = true
		}
		s.queue = nil
		if s.downloading != nil {
			s.downloading.cancel()
		}
		s.mu.Unlock()
		waitIdle(t, s)
		if queued {
			t.Errorf("dismiss %s: updating by itself queued the dismissed image anyway", c.dismiss)
		}
	}

	request(t, s, http.MethodPost, "/api/track", map[string]string{"entry": entry, "dismiss": ""})
	if got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)).Tracks[entry]; got.Dismissed(time.Now()) {
		t.Errorf("after undo the track is %+v, want it shown again", got)
	}
	if rec := request(t, s, http.MethodPost, "/api/track", map[string]string{"entry": entry, "dismiss": "12"}); rec.Code != http.StatusBadRequest {
		t.Errorf("dismissing for 12 days answered %d, want a refusal", rec.Code)
	}
}
