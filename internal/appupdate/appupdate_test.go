package appupdate

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
)

func TestCheck(t *testing.T) {
	var calls atomic.Int32
	latest := "v0.2.0"
	// Each Check gets a fresh client, as each run of the app does, so only the
	// cache file can prevent a request.
	newClient := func() *remote.Client {
		c := remote.New("test")
		c.HTTP = &http.Client{Transport: remotetest.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Path != "/repos/ZachCurry13/isoshelf/releases/latest" {
				t.Errorf("unexpected request for %s", r.URL)
			}
			w.Write([]byte(`{"tag_name": "` + latest + `", "html_url": "https://github.com/ZachCurry13/isoshelf/releases/tag/` + latest + `"}`))
		}))}
		return c
	}
	config := t.TempDir()
	ctx := context.Background()
	t0 := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	if n, err := Check(ctx, newClient(), config, "dev", t0); n != nil || err != nil || calls.Load() != 0 {
		t.Errorf("dev build: got %v, %v after %d requests; want no check", n, err, calls.Load())
	}

	n, err := Check(ctx, newClient(), config, "v0.1.0", t0)
	if err != nil {
		t.Fatal(err)
	}
	if n == nil || n.Latest != "v0.2.0" || n.Current != "v0.1.0" {
		t.Fatalf("notice = %+v, want v0.2.0 over v0.1.0", n)
	}

	// Within a day the answer comes from the cache, even for a new client.
	latest = "v0.3.0"
	if n, _ := Check(ctx, newClient(), config, "v0.1.0", t0.Add(23*time.Hour)); n == nil || n.Latest != "v0.2.0" || calls.Load() != 1 {
		t.Errorf("cached: got %+v after %d requests, want v0.2.0 after 1", n, calls.Load())
	}
	if n, _ := Check(ctx, newClient(), config, "v0.2.0", t0.Add(time.Hour)); n != nil {
		t.Errorf("same version: got notice %+v", n)
	}

	// After a day it asks again.
	if n, _ := Check(ctx, newClient(), config, "v0.2.0", t0.Add(25*time.Hour)); n == nil || n.Latest != "v0.3.0" || calls.Load() != 2 {
		t.Errorf("stale cache: got %+v after %d requests, want v0.3.0 after 2", n, calls.Load())
	}
}

func TestCheckError(t *testing.T) {
	c := remote.New("test")
	c.HTTP = &http.Client{Transport: remotetest.Handler(http.NotFoundHandler())}
	// A private repository looks like this to GitHub's API.
	if n, err := Check(context.Background(), c, t.TempDir(), "v0.1.0", time.Now()); n != nil || err == nil {
		t.Errorf("got %v, %v; want an error and no notice", n, err)
	}
}
