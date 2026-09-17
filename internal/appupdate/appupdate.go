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

	checkInterval = 24 * time.Hour
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
		resp, err := client.Get(ctx, "https://api.github.com/repos/"+Repo+"/releases/latest")
		if err != nil {
			return nil, err
		}
		var rel struct {
			TagName string `json:"tag_name"`
			HTMLURL string `json:"html_url"`
		}
		if err := json.Unmarshal(resp.Body, &rel); err != nil {
			return nil, fmt.Errorf("latest isoshelf release: %w", err)
		}
		c = cache{CheckedAt: now.UTC(), Latest: rel.TagName, URL: rel.HTMLURL}
		if data, err := json.MarshalIndent(c, "", "  "); err == nil && os.MkdirAll(configDir, 0o755) == nil {
			os.WriteFile(name, data, 0o644) // best effort: at worst we ask again next time
		}
	}

	if c.Latest != "" && version.Compare(strings.TrimPrefix(c.Latest, "v"), strings.TrimPrefix(current, "v")) > 0 {
		return &Notice{Current: current, Latest: c.Latest, URL: c.URL}, nil
	}
	return nil, nil
}
