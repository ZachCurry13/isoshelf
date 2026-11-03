package fetch

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeClock is a limiter's clock that only moves when it is asked to wait.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }
func (c *fakeClock) sleep(_ context.Context, d time.Duration) error {
	c.t = c.t.Add(d)
	return nil
}

func testLimiter(rate int64) (*Limiter, *fakeClock) {
	c := &fakeClock{t: time.Unix(1000, 0)}
	l := NewLimiter(rate)
	l.now, l.sleep = c.now, c.sleep
	return l, c
}

// A hundred kilobytes a second means a megabyte takes about ten seconds, which
// is what the cap is for.
func TestALimiterHoldsTheSpeedToTheRate(t *testing.T) {
	l, clock := testLimiter(100_000)
	start := clock.t
	for i := 0; i < 10; i++ {
		if err := l.Wait(context.Background(), 100_000); err != nil {
			t.Fatal(err)
		}
	}
	// The first 100 KB goes at once; the other nine wait a second each.
	if got := clock.t.Sub(start); got < 8900*time.Millisecond || got > 9100*time.Millisecond {
		t.Errorf("a megabyte took %v, want about 9s after the first 100 KB", got)
	}
}

// Downloads that share a limiter share the rate: two taking turns take as long
// as one doing both.
func TestTwoDownloadsShareOneLimit(t *testing.T) {
	l, clock := testLimiter(100_000)
	start := clock.t
	for i := 0; i < 10; i++ {
		l.Wait(context.Background(), 50_000) // one download
		l.Wait(context.Background(), 50_000) // the other
	}
	if got := clock.t.Sub(start); got < 9*time.Second || got > 10*time.Second {
		t.Errorf("two downloads of 500 KB each took %v at 100 KB/s, want about 9.5s", got)
	}
}

// Time spent idle is not saved up and spent in a burst afterwards.
func TestALimiterDoesNotSaveUpIdleTime(t *testing.T) {
	l, clock := testLimiter(100_000)
	l.Wait(context.Background(), 100_000)
	clock.t = clock.t.Add(time.Hour)
	before := clock.t
	l.Wait(context.Background(), 100_000) // at once
	l.Wait(context.Background(), 100_000) // waits a second
	if got := clock.t.Sub(before); got < 900*time.Millisecond || got > 1100*time.Millisecond {
		t.Errorf("after an hour idle, two chunks took %v, want about 1s", got)
	}
}

func TestNoLimitMeansNoWaiting(t *testing.T) {
	var none *Limiter
	if err := none.Wait(context.Background(), 1<<30); err != nil {
		t.Error(err)
	}
	l, clock := testLimiter(0)
	start := clock.t
	l.Wait(context.Background(), 1<<30)
	if clock.t != start {
		t.Error("a limiter with no rate waited")
	}
	// And the rate can be changed under running downloads.
	l.SetRate(1000)
	if l.Rate() != 1000 {
		t.Errorf("rate %d after SetRate(1000)", l.Rate())
	}
}

func TestAStoppedDownloadStopsWaiting(t *testing.T) {
	l := NewLimiter(1) // a byte a second: the wait would be long
	ctx, cancel := context.WithCancel(context.Background())
	l.Wait(ctx, 1) // books the first byte
	cancel()
	done := make(chan error, 1)
	go func() { done <- l.Wait(ctx, 100) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a cancelled wait said it finished")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled download kept waiting")
	}
}

// A download under a limit is paced by it, and the file that arrives is still
// the whole file, checked as ever.
func TestADownloadUnderALimitIsPacedAndStillWhole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "example.iso", time.Unix(0, 0), bytes.NewReader(image))
	}))
	defer server.Close()

	limit, clock := testLimiter(int64(len(image) / 4)) // the image in about four seconds
	client := testClient()
	client.Limit = limit
	start := clock.t
	res, err := client.Download(context.Background(), request(t.TempDir(), server.URL, sha256Checksum(imageSHA256)), func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Verified || res.SHA256 != imageSHA256 {
		t.Errorf("the file that arrived: %+v", res)
	}
	if got := clock.t.Sub(start); got < 2500*time.Millisecond || got > 4500*time.Millisecond {
		t.Errorf("the download took %v of limited time, want about 4s", got)
	}
}

// Another isoshelf on the same network is not the connection being rationed.
func TestAHostOnTheUnlimitedListIsNotSlowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "example.iso", time.Unix(0, 0), bytes.NewReader(image))
	}))
	defer server.Close()

	limit, clock := testLimiter(1000) // would take far too long
	client := testClient()
	client.Limit = limit
	client.Unlimited = []string{strings.TrimPrefix(server.URL, "http://")}
	start := clock.t
	if _, err := client.Download(context.Background(), request(t.TempDir(), server.URL, sha256Checksum(imageSHA256)), func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if clock.t != start {
		t.Errorf("a download from an unlimited host was slowed by %v", clock.t.Sub(start))
	}
}
