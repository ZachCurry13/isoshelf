package web

import (
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/lastcheck"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

// countingServer is the recorded responses with a tally, so a test can say
// how many times isoshelf went to a website.
func countingServer(t *testing.T, dirs appdir.Dirs, target string, asked *atomic.Int64) *Server {
	t.Helper()
	cat, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	recorded := remotetest.Recorded()
	counting := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		asked.Add(1)
		return recorded.RoundTrip(r)
	})
	return New(Config{
		Dirs:    dirs,
		Catalog: cat,
		HTTP:    &http.Client{Transport: counting},
		Version: "dev",
		Token:   testToken,
		Target:  target,
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The point of remembering: opening the page a second time costs nothing,
// and Refresh asks anyway.
func TestASecondScanAsksNobodyAgain(t *testing.T) {
	dirs := testDirs(t)
	drive := sampleDrive(t)
	var asked atomic.Int64
	s := countingServer(t, dirs, drive, &asked)

	request(t, s, http.MethodPost, "/api/scan", nil)
	st := waitIdle(t, s)
	if !st.Report.Checked {
		t.Fatal("a scan didn't check for updates")
	}
	first := asked.Load()
	if first == 0 {
		t.Fatal("the first scan asked nobody")
	}
	if st.CheckedAt == nil {
		t.Error("the page isn't told how fresh the answers are")
	}

	// Everything was asked moments ago, so nothing should be asked again.
	asked.Store(0)
	request(t, s, http.MethodPost, "/api/scan", nil)
	if waitIdle(t, s); asked.Load() != 0 {
		t.Errorf("a second scan asked %d times; it should have used what it remembered", asked.Load())
	}

	// Refresh is the way to insist.
	asked.Store(0)
	request(t, s, http.MethodPost, "/api/check", nil)
	waitIdle(t, s)
	if asked.Load() == 0 {
		t.Error("Refresh used remembered answers instead of asking again")
	}

	// And the answers outlive this server.
	if _, err := os.Stat(filepath.Join(dirs.Config, lastcheck.FileName)); err != nil {
		t.Errorf("the answers weren't written down: %v", err)
	}
	next := lastcheck.Load(dirs.Config)
	if len(next.Entries) == 0 {
		t.Error("the file is there but holds nothing")
	}
}

// Turning it off means isoshelf stays off the internet until asked.
func TestCheckingCanBeTurnedOff(t *testing.T) {
	dirs := testDirs(t)
	drive := sampleDrive(t)
	var asked atomic.Int64
	s := countingServer(t, dirs, drive, &asked)

	request(t, s, http.MethodPost, "/api/settings", map[string]any{"auto_check": false})
	asked.Store(0)
	request(t, s, http.MethodPost, "/api/scan", nil)
	st := waitIdle(t, s)
	if asked.Load() != 0 {
		t.Errorf("with checking off, a scan still asked %d times", asked.Load())
	}
	if st.Report == nil || st.Report.Checked {
		t.Errorf("with checking off, the scan still reported a check: %+v", st.Report)
	}
	if st.AutoCheck {
		t.Error("the page still thinks checking by itself is on")
	}

	// Refresh still works: off means "not by yourself", not "never".
	request(t, s, http.MethodPost, "/api/check", nil)
	if st := waitIdle(t, s); st.Report == nil || !st.Report.Checked {
		t.Errorf("Refresh didn't check: %+v", st.Report)
	}
}
