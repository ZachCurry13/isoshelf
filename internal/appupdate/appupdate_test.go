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
			if r.URL.Path != "/repos/ZachCurry13/isoshelf/releases" {
				t.Errorf("unexpected request for %s", r.URL)
			}
			// Below 1.0 every release is marked a pre-release, which is
			// exactly why the list is asked for instead of "latest".
			w.Write([]byte(`[{"tag_name": "` + latest + `", "prerelease": true, "html_url": "https://github.com/ZachCurry13/isoshelf/releases/tag/` + latest + `"}]`))
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

	// Within the hour the answer comes from the cache, even for a new client.
	latest = "v0.3.0"
	if n, _ := Check(ctx, newClient(), config, "v0.1.0", t0.Add(50*time.Minute)); n == nil || n.Latest != "v0.2.0" || calls.Load() != 1 {
		t.Errorf("cached: got %+v after %d requests, want v0.2.0 after 1", n, calls.Load())
	}
	if n, _ := Check(ctx, newClient(), config, "v0.2.0", t0.Add(55*time.Minute)); n != nil {
		t.Errorf("same version: got notice %+v", n)
	}

	// After an hour it asks again.
	if n, _ := Check(ctx, newClient(), config, "v0.2.0", t0.Add(61*time.Minute)); n == nil || n.Latest != "v0.3.0" || calls.Load() != 2 {
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

// Which release somebody is told about depends on which they are running.
//
// Below 1.0 every release is a pre-release, so leaving those out - which is
// what GitHub's own "latest release" does - would mean telling nobody about
// anything. After 1.0 a released version must never quietly point at a test
// build, while anyone running one keeps hearing about the next.
func TestWhichReleaseIsOffered(t *testing.T) {
	releases := []release{
		{TagName: "v2.0.0-rc1", Prerelease: true, HTMLURL: "rc"},
		{TagName: "v1.2.0", HTMLURL: "stable"},
		{TagName: "v1.1.0", HTMLURL: "older"},
		{TagName: "v1.3.0", Draft: true, HTMLURL: "draft"},
	}
	for _, c := range []struct {
		running, want, why string
	}{
		{"v1.1.0", "v1.2.0", "a released version is offered the newest released one"},
		{"v2.0.0-rc1", "v2.0.0-rc1", "somebody on a test build is offered test builds"},
		{"v1.2.0-rc3", "v2.0.0-rc1", "and the newest of them"},
		{"v0.4.9", "v2.0.0-rc1", "below 1.0 everything is a pre-release, so they all count"},
	} {
		if got := pick(releases, c.running).TagName; got != c.want {
			t.Errorf("running %s: offered %s, want %s - %s", c.running, got, c.want, c.why)
		}
	}
	// A draft is nobody's business: it isn't published.
	for _, running := range []string{"v1.1.0", "v1.2.0-rc1"} {
		if got := pick(releases, running).TagName; got == "v1.3.0" {
			t.Errorf("running %s was offered a draft", running)
		}
	}
	// Nothing to offer is not a crash.
	if got := pick(nil, "v1.0.0").TagName; got != "" {
		t.Errorf("an empty list offered %q", got)
	}
}

func TestIsPrerelease(t *testing.T) {
	for _, c := range []struct {
		version string
		want    bool
	}{
		{"v0.4.9", true}, {"0.1.0", true},
		{"v1.0.0", false}, {"v2.3.4", false},
		{"v1.0.0-rc1", true}, {"v1.0.0-beta.2", true},
	} {
		if got := isPrerelease(c.version); got != c.want {
			t.Errorf("isPrerelease(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}
