package remote

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

func testClient(h http.HandlerFunc) *Client {
	c := New("test")
	c.HTTP = &http.Client{Transport: remotetest.Handler(h)}
	c.Backoff = time.Millisecond
	return c
}

func TestGetCachesAndSendsHeaders(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "isoshelf/test ") {
			t.Errorf("User-Agent = %q", ua)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want the GitHub token", got)
		}
		w.Write([]byte("hello"))
	})
	c.GitHubToken = "secret"

	for range 2 {
		resp, err := c.Get(context.Background(), "https://api.github.com/repos/a/b/releases")
		if err != nil {
			t.Fatal(err)
		}
		if string(resp.Body) != "hello" || resp.URL.Host != "api.github.com" {
			t.Errorf("response = %q from %v", resp.Body, resp.URL)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("fetched %d times, want 1 (cached)", calls.Load())
	}
}

func TestGetTokenOnlyForGitHub(t *testing.T) {
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("GitHub token sent to another host")
		}
	})
	c.GitHubToken = "secret"
	if _, err := c.Get(context.Background(), "https://example.org/"); err != nil {
		t.Fatal(err)
	}
}

func TestGetRefusesPlainHTTP(t *testing.T) {
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "http://example.org/insecure", http.StatusFound)
		}
	})
	if _, err := c.Get(context.Background(), "http://example.org/"); !errors.Is(err, errInsecure) {
		t.Errorf("http URL: got %v, want errInsecure", err)
	}
	if _, err := c.Get(context.Background(), "https://example.org/start"); !errors.Is(err, errInsecure) {
		t.Errorf("redirect to http: got %v, want errInsecure", err)
	}
}

func TestGetRetries(t *testing.T) {
	var calls atomic.Int32
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			http.Error(w, "busy", http.StatusBadGateway)
			return
		}
		w.Write([]byte("ok"))
	})
	resp, err := c.Get(context.Background(), "https://example.org/flaky")
	if err != nil || string(resp.Body) != "ok" {
		t.Fatalf("got %v, %v; want ok after retries", resp, err)
	}

	// A 404 is final: no retries.
	calls.Store(0)
	notFound := testClient(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	})
	_, err = notFound.Get(context.Background(), "https://example.org/missing")
	var status *StatusError
	if !errors.As(err, &status) || status.Code != http.StatusNotFound {
		t.Errorf("got %v, want a 404 StatusError", err)
	}
	if calls.Load() != 1 {
		t.Errorf("404 was tried %d times, want 1", calls.Load())
	}
}

func TestGetRateLimit(t *testing.T) {
	reset := time.Date(2026, 9, 17, 15, 4, 0, 0, time.UTC)
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1789657440")
		http.Error(w, "rate limited", http.StatusForbidden)
	})
	_, err := c.Get(context.Background(), "https://api.github.com/repos/a/b/releases")
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("got %v, want RateLimitError", err)
	}
	if !rl.Reset.Equal(reset) {
		t.Errorf("reset = %v, want %v", rl.Reset.UTC(), reset)
	}
	if !strings.Contains(rl.Error(), "resets at") {
		t.Errorf("message %q doesn't say when it resets", rl.Error())
	}
}

func TestGetTooLarge(t *testing.T) {
	c := testClient(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, MaxBody+1))
	})
	if _, err := c.Get(context.Background(), "https://example.org/huge"); err == nil {
		t.Error("want an error for a response over MaxBody")
	}
}

func TestRecorded(t *testing.T) {
	c := New("test")
	c.HTTP = &http.Client{Transport: remotetest.Recorded()}
	resp, err := c.Get(context.Background(), "https://api.github.com/repos/netbootxyz/netboot.xyz/releases?per_page=100")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(resp.Body), "netboot.xyz.iso") {
		t.Error("recorded GitHub response doesn't mention netboot.xyz.iso")
	}
	var status *StatusError
	if _, err := c.Get(context.Background(), "https://example.org/not-recorded"); !errors.As(err, &status) {
		t.Errorf("unrecorded URL: got %v, want a StatusError", err)
	}
}
