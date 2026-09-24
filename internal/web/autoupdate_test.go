package web

import (
	"net/http"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// Off unless somebody turns it on, and it stays that way. This is the one
// thing isoshelf does that changes a drive while its owner isn't looking.
func TestUpdatingByItselfIsOffUntilAskedFor(t *testing.T) {
	dirs, drive := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, drive)
	if got := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil)); got.AutoUpdate {
		t.Error("a fresh isoshelf updates images by itself")
	}
	// And a tick with nothing turned on does nothing at all.
	s.autoUpdateIfDue()
	if st := waitIdle(t, s); len(st.Downloads.Queued) > 0 || st.Downloads.Current != nil {
		t.Errorf("something was downloaded without being asked for: %+v", st.Downloads)
	}
}

// Turning it on turns checking on too: it has nothing to act on otherwise,
// and a switch that quietly does nothing is worse than no switch.
func TestUpdatingByItselfNeedsCheckingByItself(t *testing.T) {
	dirs := testDirs(t)
	if err := settings.Save(dirs.Config, settings.Settings{AutoCheck: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, sampleDrive(t))
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"auto_update": true})
	// Turning it on starts a run of its own; let that finish before looking,
	// or this test races the run it just asked for.
	s.background.Wait()
	waitIdle(t, s)
	saved := settings.Load(dirs.Config)
	if !settings.On(saved.AutoCheck) {
		t.Error("updating by itself was turned on while checking by itself stayed off")
	}
}

func boolPtr(b bool) *bool { return &b }

// How often is remembered, and anything unknown falls back to every day.
func TestHowOftenIsRemembered(t *testing.T) {
	dirs := testDirs(t)
	s := newServer(t, dirs, sampleDrive(t))
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"auto_update": true})
	rec := request(t, s, http.MethodPost, "/api/settings", map[string]any{"auto_update_every": "week"})
	if got := decode[stateJSON](t, rec); got.AutoUpdateEvery != settings.EveryWeek {
		t.Errorf("how often = %q, want %q", got.AutoUpdateEvery, settings.EveryWeek)
	}
	rec = request(t, s, http.MethodPost, "/api/settings", map[string]any{"auto_update_every": "whenever"})
	if got := decode[stateJSON](t, rec); got.AutoUpdateEvery != settings.EveryDay {
		t.Errorf("a how-often nobody knows = %q, want every day", got.AutoUpdateEvery)
	}
	s.background.Wait()
	waitIdle(t, s)
}

// Each image follows the answer it already carries. What happens unattended
// has to be what the page has been saying would happen, or somebody's drive
// does something they were told it wouldn't.
func TestWhatHappensToTheOldFileFollowsTheImagesOwnAnswer(t *testing.T) {
	item := check.Item{Path: "linuxmint-22.3.iso", LatestFile: "linuxmint-22.4.iso"}
	fixed := check.Item{Path: "netboot.xyz.iso", LatestFile: "netboot.xyz.iso"}

	for _, c := range []struct {
		why      string
		item     check.Item
		track    state.Track
		fallback string
		want     update.Removal
	}{
		{"its own answer wins", item, state.Track{OldFiles: settings.OldArchive}, settings.OldReplace, update.MoveAside},
		{"and over the older keep switch", item, state.Track{OldFiles: settings.OldReplace, KeepOld: true}, "", update.DeleteNow},
		{"the old keep switch still counts", item, state.Track{KeepOld: true}, settings.OldReplace, update.Keep},
		{"no answer falls back to Settings", item, state.Track{}, settings.OldArchive, update.MoveAside},
		{"and to replacing when nothing is set", item, state.Track{}, "", update.DeleteNow},
		{"a fixed name can't keep both, so it archives", fixed, state.Track{OldFiles: settings.OldKeep}, "", update.MoveAside},
		{"a changing name can", item, state.Track{OldFiles: settings.OldKeep}, "", update.Keep},
	} {
		if got := removalFor(c.item, c.track, c.fallback); got != c.want {
			t.Errorf("%s: %q, want %q", c.why, got, c.want)
		}
	}
}

// It stops before the folder is full rather than filling it. A drive with no
// room left is a drive that won't boot anything either.
func TestItStopsBeforeFillingTheFolder(t *testing.T) {
	dirs, drive := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, drive)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	s.mu.Lock()
	updates := 0
	for _, item := range s.report.Items {
		if item.Status == check.UpdateAvailable && item.Entry != nil {
			updates++
		}
	}
	// Barely any room: nothing should be queued at all.
	added, stopped := s.queueUpdatesLocked(space.Usage{Free: 1 << 20, Total: 100 << 30})
	queued := len(s.queue)
	s.mu.Unlock()

	if updates == 0 {
		t.Skip("this sample drive has no updates waiting, so there is nothing to stop")
	}
	if added != 0 || queued != 0 {
		t.Errorf("queued %d downloads with no room for them", added)
	}
	if stopped == "" {
		t.Error("it stopped but didn't say why")
	}
}

// With room, it queues the updates and marks them as its own doing, so the
// page can say so rather than leaving somebody wondering what they pressed.
func TestItQueuesTheUpdatesItFinds(t *testing.T) {
	dirs, drive := testDirs(t), sampleDrive(t)
	s := newServer(t, dirs, drive)
	request(t, s, http.MethodPost, "/api/scan", nil)
	waitIdle(t, s)

	s.mu.Lock()
	updates := 0
	for _, item := range s.report.Items {
		if item.Status == check.UpdateAvailable && item.Entry != nil {
			updates++
		}
	}
	if updates == 0 {
		s.mu.Unlock()
		t.Skip("this sample drive has no updates waiting")
	}
	added, _ := s.queueUpdatesLocked(space.Usage{Free: 500 << 30, Total: 1000 << 30})
	everyOneAutomatic := true
	for _, j := range s.queue {
		if !j.automatic {
			everyOneAutomatic = false
		}
	}
	running := s.downloading
	s.mu.Unlock()

	if added != updates {
		t.Errorf("queued %d of %d updates", added, updates)
	}
	if !everyOneAutomatic && running == nil {
		t.Error("a download isoshelf started itself isn't marked as such")
	}
	// Asked twice, it doesn't queue the same image again.
	s.mu.Lock()
	again, _ := s.queueUpdatesLocked(space.Usage{Free: 500 << 30, Total: 1000 << 30})
	s.mu.Unlock()
	if again != 0 {
		t.Errorf("a second look queued %d images that were already queued", again)
	}
	s.mu.Lock()
	s.queue = nil
	if s.downloading != nil {
		s.downloading.cancel()
	}
	s.mu.Unlock()
	waitIdle(t, s)
}

// A folder somebody is already using is left alone. The next tick is minutes
// away, and interrupting their work to do this would be the wrong way round.
func TestItLeavesABusyFolderAlone(t *testing.T) {
	dirs := testDirs(t)
	if err := settings.Save(dirs.Config, settings.Settings{AutoUpdate: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, sampleDrive(t))

	// A scan is running: the tick should do nothing.
	request(t, s, http.MethodPost, "/api/scan", nil)
	s.autoUpdateIfDue()
	s.mu.Lock()
	queued := len(s.queue)
	s.mu.Unlock()
	if queued > 0 {
		t.Errorf("it queued %d downloads while a scan was running", queued)
	}
	waitIdle(t, s)
	s.mu.Lock()
	s.queue = nil
	if s.downloading != nil {
		s.downloading.cancel()
	}
	s.mu.Unlock()
	waitIdle(t, s)
}

// The schedule is kept across restarts, so coming back doesn't start another
// run - which on a NAS would mean one on every app update.
func TestTheScheduleSurvivesARestart(t *testing.T) {
	dirs := testDirs(t)
	now := time.Now()
	if err := settings.Save(dirs.Config, settings.Settings{
		AutoUpdate: boolPtr(true), AutoUpdateLast: now.Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, sampleDrive(t))
	s.autoUpdateIfDue() // an hour ago, with a daily schedule: not due
	s.mu.Lock()
	scanning := s.scanning != nil
	s.mu.Unlock()
	if scanning {
		t.Error("it started a run that wasn't due yet")
	}
}

// Each isoshelf adds its own offset to the schedule, so copies that started
// together drift apart instead of asking the same servers in the same minute.
func TestTheScheduleIsSpread(t *testing.T) {
	for _, c := range []struct {
		ago  time.Duration
		want bool
	}{
		{24*time.Hour + 10*time.Minute, false}, // a day has passed, the offset hasn't
		{24*time.Hour + 50*time.Minute, true},
	} {
		dirs := testDirs(t)
		if err := settings.Save(dirs.Config, settings.Settings{
			AutoUpdate: boolPtr(true), AutoUpdateLast: time.Now().Add(-c.ago),
		}); err != nil {
			t.Fatal(err)
		}
		s := newServer(t, dirs, sampleDrive(t))
		s.autoOffset = 45 * time.Minute
		s.autoUpdateIfDue()
		s.mu.Lock()
		started := s.scanning != nil
		s.mu.Unlock()
		if started != c.want {
			t.Errorf("last run %v ago, offset 45m: started=%v, want %v", c.ago, started, c.want)
		}
		waitIdle(t, s)
	}
}
