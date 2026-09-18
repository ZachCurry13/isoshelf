// Package logos finds the distro logos the page shows. Logos for the
// built-in catalog ship inside isoshelf; anything else, such as an entry
// added after this release or one from the user's own catalog, is fetched
// once and kept in the settings folder. Images without a logo get coloured
// initials drawn by the page.
//
// The logos come from Simple Icons (CC0 1.0). They are used to identify the
// projects they belong to, and their trademarks stay with their owners.
package logos

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// MaxSize is the largest logo file isoshelf accepts.
const MaxSize = 64 << 10

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)

// SourceURL is where a logo is fetched from.
func SourceURL(slug string) string {
	return "https://cdn.jsdelivr.net/npm/simple-icons@latest/icons/" + slug + ".svg"
}

// Store serves logos from the ones built in, then the ones fetched earlier,
// and fetches anything still missing.
type Store struct {
	// Builtin holds the logos that ship with isoshelf, named <slug>.svg.
	Builtin fs.FS
	// CacheDir is where fetched logos are kept; empty means none are kept and
	// nothing is fetched.
	CacheDir string
	// Client fetches missing logos. Nil means no fetching.
	Client *remote.Client
}

// ErrNoLogo means there is no logo for this name, so the page draws initials.
var ErrNoLogo = errors.New("no logo")

// Get returns the SVG for a name, fetching and keeping it if it's new.
func (s *Store) Get(ctx context.Context, slug string) ([]byte, error) {
	if !slugPattern.MatchString(slug) {
		return nil, ErrNoLogo
	}
	if s.Builtin != nil {
		if data, err := fs.ReadFile(s.Builtin, slug+".svg"); err == nil {
			return data, nil
		}
	}
	if s.CacheDir == "" {
		return nil, ErrNoLogo
	}
	cached := filepath.Join(s.CacheDir, slug+".svg")
	if data, err := os.ReadFile(cached); err == nil {
		if len(data) == 0 {
			return nil, ErrNoLogo // remembered: this one doesn't exist
		}
		return data, nil
	}
	if s.Client == nil {
		return nil, ErrNoLogo
	}

	resp, err := s.Client.Get(ctx, SourceURL(slug))
	if err != nil {
		var status *remote.StatusError
		if errors.As(err, &status) && status.Code == 404 {
			// Remember the miss, so the same name isn't asked for again.
			s.write(cached, nil)
			return nil, ErrNoLogo
		}
		return nil, err
	}
	if len(resp.Body) > MaxSize || !strings.Contains(string(resp.Body), "<svg") {
		return nil, fmt.Errorf("%s doesn't look like a logo", slug)
	}
	s.write(cached, resp.Body)
	return resp.Body, nil
}

func (s *Store) write(name string, data []byte) {
	if os.MkdirAll(filepath.Dir(name), 0o755) == nil {
		os.WriteFile(name, data, 0o644) // best effort: at worst it's fetched again
	}
}
