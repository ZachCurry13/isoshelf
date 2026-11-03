package web

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

// How many downloads run together and which (schedule.go): never two from one
// server, two at once with no limit, and under a speed limit another only
// while the ones going leave room for it.

// clock is a time that only moves when a test moves it.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// queueServer is a server with fake downloads that allows its own default of
// downloads at once, on a clock the test controls.
func queueServer(t *testing.T) (*Server, *fakeDownloads, *clock) {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	clk := &clock{t: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)}
	dir := sampleDrive(t)
	s := New(Config{
		Dirs: testDirs(t), Catalog: cat, HTTP: &http.Client{Transport: remotetest.Recorded()},
		Version: "dev", Token: testToken, Target: dir, Now: clk.now,
	})
	if rec := request(t, s, http.MethodPost, "/api/scan", nil); rec.Code != http.StatusAccepted {
		t.Fatalf("scan: %d", rec.Code)
	}
	waitIdle(t, s)
	f := useFakeDownloads(s)
	s.mu.Lock()
	s.atOnce = 0 // schedule.go's own default, which is what is being tested
	s.mu.Unlock()
	return s, f, clk
}

// running lists the entries downloading now, oldest first.
func running(d downloadsJSON) string {
	var names []string
	for _, j := range d.Running {
		names = append(names, j.Entry)
	}
	return strings.Join(names, " ")
}

func stopAll(t *testing.T, s *Server) {
	t.Helper()
	s.mu.Lock()
	s.queue = nil
	for _, r := range s.running {
		r.cancel()
	}
	s.mu.Unlock()
	waitIdle(t, s)
}

// Two downloads from different servers go together; a third waits its turn.
func TestTwoDownloadsFromDifferentServersRunTogether(t *testing.T) {
	s, f, _ := queueServer(t)
	defer stopAll(t, s)
	add(t, s, "ubuntu-desktop-lts")
	add(t, s, "archlinux")
	add(t, s, "gparted-live")
	// Both, and their progress: a slot is filled a moment before the download
	// reports any.
	st := waitDownloads(t, s, func(st stateJSON) bool {
		r := st.Downloads.Running
		return len(r) == 2 && r[0].Stage != "" && r[1].Stage != ""
	})
	if got := running(st.Downloads); got != "ubuntu-desktop-lts archlinux" {
		t.Errorf("running %q, want the first two, from different servers", got)
	}
	if got := queued(st.Downloads); got != "gparted-live" {
		t.Errorf("waiting %q, want the third", got)
	}
	// Each carries its own progress, which is what the bar draws a row from.
	for _, j := range st.Downloads.Running {
		if j.Total == 0 || j.Stage == "" {
			t.Errorf("%s has no progress of its own: %+v", j.Entry, j)
		}
	}
	// When one ends, the waiting one takes its place.
	f.end("ubuntu-desktop-lts", nil)
	st = waitDownloads(t, s, func(st stateJSON) bool { return running(st.Downloads) == "archlinux gparted-live" })
	if len(st.Downloads.Finished) != 1 || st.Downloads.Finished[0].Outcome != "done" {
		t.Errorf("finished: %+v", st.Downloads.Finished)
	}
}

// A project's server sees one connection from one person, however long the
// queue is: a waiting download whose server is busy is passed over for the
// next one that isn't, and goes when its server is free.
func TestNeverTwoFromTheSameServer(t *testing.T) {
	s, f, _ := queueServer(t)
	defer stopAll(t, s)
	s.mu.Lock()
	same := s.hostOfLocked(&job{entry: "fedora-workstation"}) == s.hostOfLocked(&job{entry: "fedora-kde"})
	s.mu.Unlock()
	if !same {
		t.Fatal("the two Fedora entries are meant to share a server for this test")
	}
	add(t, s, "fedora-workstation")
	add(t, s, "fedora-kde")
	add(t, s, "archlinux")
	st := waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Running) == 2 })
	if got := running(st.Downloads); got != "fedora-workstation archlinux" {
		t.Errorf("running %q: Fedora KDE should have been passed over for Arch", got)
	}
	if got := queued(st.Downloads); got != "fedora-kde" {
		t.Errorf("waiting %q, want Fedora KDE", got)
	}
	// Room has opened up, but its server is still busy.
	f.end("archlinux", nil)
	st = waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Finished) == 1 })
	if running(st.Downloads) != "fedora-workstation" || queued(st.Downloads) != "fedora-kde" {
		t.Errorf("with its server busy, Fedora KDE started anyway: running %q, waiting %q", running(st.Downloads), queued(st.Downloads))
	}
	f.end("fedora-workstation", nil)
	waitDownloads(t, s, func(st stateJSON) bool { return running(st.Downloads) == "fedora-kde" })
}

// Stopping one leaves the others going.
func TestStoppingOneDownloadLeavesTheOther(t *testing.T) {
	s, _, _ := queueServer(t)
	defer stopAll(t, s)
	add(t, s, "ubuntu-desktop-lts")
	arch := add(t, s, "archlinux")
	waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Running) == 2 })
	request(t, s, http.MethodPost, "/api/queue/drop", map[string]any{"id": arch})
	st := waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Finished) == 1 })
	if st.Downloads.Finished[0].Entry != "archlinux" || st.Downloads.Finished[0].Outcome != "stopped" {
		t.Errorf("finished: %+v", st.Downloads.Finished)
	}
	if got := running(st.Downloads); got != "ubuntu-desktop-lts" {
		t.Errorf("running %q after stopping Arch, want Ubuntu still going", got)
	}
}

// The speed limit decides how many: another starts only while what is running
// leaves room under it, measured, and never before a download's speed is
// known.
func TestUnderASpeedLimitAnotherStartsOnlyWhenThereIsRoom(t *testing.T) {
	s, _, clk := queueServer(t)
	defer stopAll(t, s)
	// 100 megabits a second is 12.5 million bytes.
	if rec := request(t, s, http.MethodPost, "/api/settings", map[string]any{"download_limit": 100}); rec.Code != http.StatusOK {
		t.Fatalf("setting the limit: %d %s", rec.Code, rec.Body)
	}
	if got := s.limiter.Rate(); got != 12_500_000 {
		t.Fatalf("the limiter holds %d bytes a second, want 12,500,000", got)
	}

	add(t, s, "ubuntu-desktop-lts")
	add(t, s, "archlinux")
	add(t, s, "gparted-live")
	add(t, s, "fedora-workstation")
	waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Running) == 1 })

	step := func(speeds ...float64) (running int) {
		t.Helper()
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, v := range speeds {
			s.running[i].speed = v
		}
		s.startNextLocked()
		return len(s.running)
	}

	// Just started: its speed isn't known, so nothing else yet.
	if got := step(0); got != 1 {
		t.Errorf("%d running before the first had been measured, want 1", got)
	}
	// Measured, and it is using most of the limit: no room.
	clk.advance(10 * time.Second)
	if got := step(10_000_000); got != 1 {
		t.Errorf("%d running with 80%% of the limit in use, want 1", got)
	}
	// A slow server leaves most of it unused: room for another.
	if got := step(2_000_000); got != 2 {
		t.Errorf("%d running with 16%% of the limit in use, want 2", got)
	}
	// The newcomer hasn't been measured: wait.
	if got := step(2_000_000, 0); got != 2 {
		t.Errorf("%d running before the second had been measured, want 2", got)
	}
	clk.advance(10 * time.Second)
	if got := step(2_000_000, 3_000_000); got != 3 {
		t.Errorf("%d running with 40%% in use, want 3", got)
	}
	// Three is the most, however much room there is.
	clk.advance(10 * time.Second)
	if got := step(100, 100, 100); got != 3 {
		t.Errorf("%d running, want never more than %d", got, atOnceMost)
	}
	st := waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Running) == 3 })
	if got := queued(st.Downloads); got != "fedora-workstation" {
		t.Errorf("waiting %q, want the fourth", got)
	}
	// What the page is told includes each one's speed.
	if st.Downloads.Running[0].Speed != 100 {
		t.Errorf("the page is told %d bytes a second for the first, want 100", st.Downloads.Running[0].Speed)
	}
}

// With no limit set there is nothing to measure against, so it is two.
func TestWithNoLimitItIsTwo(t *testing.T) {
	s, _, clk := queueServer(t)
	defer stopAll(t, s)
	add(t, s, "ubuntu-desktop-lts")
	add(t, s, "archlinux")
	add(t, s, "gparted-live")
	waitDownloads(t, s, func(st stateJSON) bool { return len(st.Downloads.Running) == 2 })
	clk.advance(time.Hour)
	s.mu.Lock()
	s.running[0].speed, s.running[1].speed = 1, 1 // almost nothing: still two
	s.startNextLocked()
	n := len(s.running)
	s.mu.Unlock()
	if n != 2 {
		t.Errorf("%d running with no limit set, want 2 however slow they are", n)
	}
}

// A limit can be taken away and changed while downloads run.
func TestTheLimitIsCleanedAndCanBeChanged(t *testing.T) {
	s, _, _ := queueServer(t)
	defer stopAll(t, s)
	for _, c := range []struct {
		ask  int
		want int64
	}{{100, 12_500_000}, {0, 0}, {-5, 0}, {1_000_000, 12_500_000_000}, {300, 37_500_000}} {
		request(t, s, http.MethodPost, "/api/settings", map[string]any{"download_limit": c.ask})
		if got := s.limiter.Rate(); got != c.want {
			t.Errorf("asked for %d megabits: the limiter holds %d bytes a second, want %d", c.ask, got, c.want)
		}
	}
	st := decode[stateJSON](t, request(t, s, http.MethodGet, "/api/state", nil))
	if st.DownloadLimit != 300 {
		t.Errorf("the page is told the limit is %d, want 300", st.DownloadLimit)
	}
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"reset": true})
	if got := s.limiter.Rate(); got != 0 {
		t.Errorf("after a reset the limiter holds %d, want no limit", got)
	}
}
