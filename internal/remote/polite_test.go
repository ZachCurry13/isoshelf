package remote

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// A server that says how long to wait gets exactly that, whoever runs it -
// not only GitHub - and not the doubling wait isoshelf would otherwise use.
func TestAServerThatSaysWaitIsWaitedFor(t *testing.T) {
	var asked atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if asked.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), Retries: 2}

	start := time.Now()
	resp, err := c.Get(context.Background(), srv.URL+"/list")
	if err != nil {
		t.Fatal(err)
	}
	if string(resp.Body) != "ok" {
		t.Errorf("got %q", resp.Body)
	}
	if waited := time.Since(start); waited < 900*time.Millisecond {
		t.Errorf("tried again after %v; the server asked for a second", waited)
	}
}

// Asked to come back in an hour, isoshelf doesn't sit there, and doesn't try
// again anyway: it asks once and says why it stopped.
func TestALongWaitIsNotRetried(t *testing.T) {
	var asked atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked.Add(1)
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := &Client{HTTP: srv.Client(), Retries: 3}

	_, err := c.Get(context.Background(), srv.URL+"/list")
	var busy *BusyError
	if !errors.As(err, &busy) {
		t.Fatalf("got %v, want a BusyError", err)
	}
	if n := asked.Load(); n != 1 {
		t.Errorf("asked %d times, want once", n)
	}
	if !strings.Contains(busy.Error(), "60 minutes") {
		t.Errorf("%q doesn't say how long it was asked to wait", busy.Error())
	}
}

func TestRetryAfterReadsSecondsAndDates(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		header string
		want   time.Duration
	}{
		{"120", 2 * time.Minute},
		{"Thu, 24 Sep 2026 12:05:00 GMT", 5 * time.Minute},
		{"Thu, 24 Sep 2026 11:00:00 GMT", 0}, // already past
		{"soon", 0},
		{"", 0},
	} {
		h := http.Header{}
		if c.header != "" {
			h.Set("Retry-After", c.header)
		}
		if got := retryAfter(h, now); got != c.want {
			t.Errorf("%q: %v, want %v", c.header, got, c.want)
		}
	}
}

// Without a wait from the server, the doubling wait is spread so that many
// copies of isoshelf that failed together don't come back together.
func TestTheWaitIsSpread(t *testing.T) {
	base := 4 * time.Second
	seen := map[time.Duration]bool{}
	for range 50 {
		d := NextWait(errors.New("connection reset"), base)
		if d < base/2 || d >= base+base/2 {
			t.Fatalf("waited %v, want between %v and %v", d, base/2, base+base/2)
		}
		seen[d] = true
	}
	if len(seen) < 10 {
		t.Errorf("only %d different waits in 50; it isn't spreading them", len(seen))
	}
}
