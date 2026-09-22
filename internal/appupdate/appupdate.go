// Package appupdate tells users when a newer release of isoshelf is out.
package appupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

const (
	// Repo is where isoshelf releases are published.
	Repo = "ZachCurry13/isoshelf"

	checkInterval = time.Hour
	cacheFile     = "update-check.json"
)

// Notice says a newer release is available.
type Notice struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	URL     string `json:"url"`
}

func (n Notice) String() string {
	return fmt.Sprintf("isoshelf %s is available (you have %s): %s", n.Latest, n.Current, n.URL)
}

type cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
	URL       string    `json:"url"`
}

// release is one entry from GitHub's list.
type release struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

// pick chooses the newest release worth telling somebody about.
//
// A pre-release is only offered to somebody already running one. Below 1.0
// that is everybody, which is right: every release there is a pre-release and
// there is nothing else to offer. After 1.0 it means a released version never
// quietly points at a test build, while anyone who chose to run one keeps
// being told about the next.
//
// Drafts are nobody's business: they aren't published yet.
func pick(releases []release, current string) release {
	testing := isPrerelease(current)
	var best release
	for _, rel := range releases {
		if rel.Draft || rel.TagName == "" {
			continue
		}
		if rel.Prerelease && !testing {
			continue
		}
		if best.TagName == "" || newer(rel.TagName, best.TagName) {
			best = rel
		}
	}
	return best
}

// isPrerelease says whether a version is one of the ones before 1.0, which
// are pre-releases by definition, or carries a suffix like -rc1.
func isPrerelease(v string) bool {
	v = strings.TrimPrefix(v, "v")
	return strings.HasPrefix(v, "0.") || strings.ContainsAny(v, "-+")
}

func newer(a, b string) bool {
	return version.Compare(strings.TrimPrefix(a, "v"), strings.TrimPrefix(b, "v")) > 0
}

// Check returns a notice if a release newer than current exists, or nil. It
// asks GitHub at most once a day and remembers the answer in configDir.
// Development builds (version "dev") never check.
func Check(ctx context.Context, client *remote.Client, configDir, current string, now time.Time) (*Notice, error) {
	if current == "" || current == "dev" {
		return nil, nil
	}
	name := filepath.Join(configDir, cacheFile)
	var c cache
	if data, err := os.ReadFile(name); err == nil {
		json.Unmarshal(data, &c) // a damaged cache just means asking again
	}

	if age := now.Sub(c.CheckedAt); c.CheckedAt.IsZero() || age >= checkInterval || age < 0 {
		// The list rather than /releases/latest, which leaves pre-releases
		// out entirely. Below 1.0 every release is a pre-release, so asking
		// for "latest" would mean nobody was ever told about a new isoshelf.
		resp, err := client.Get(ctx, "https://api.github.com/repos/"+Repo+"/releases?per_page=20")
		if err != nil {
			return nil, err
		}
		var releases []release
		if err := json.Unmarshal(resp.Body, &releases); err != nil {
			return nil, fmt.Errorf("isoshelf releases: %w", err)
		}
		newest := pick(releases, current)
		c = cache{CheckedAt: now.UTC(), Latest: newest.TagName, URL: newest.HTMLURL}
		if data, err := json.MarshalIndent(c, "", "  "); err == nil && os.MkdirAll(configDir, 0o755) == nil {
			os.WriteFile(name, data, 0o644) // best effort: at worst we ask again next time
		}
	}

	if c.Latest != "" && version.Compare(strings.TrimPrefix(c.Latest, "v"), strings.TrimPrefix(current, "v")) > 0 {
		return &Notice{Current: current, Latest: c.Latest, URL: c.URL}, nil
	}
	return nil, nil
}
