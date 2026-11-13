// Package source finds the latest release of a catalog entry's track, from
// endoflife.date, GitHub releases, a download listing, or the catalog itself.
package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
)

// ErrManual means the entry is manual and has no source to ask.
var ErrManual = errors.New("manual entries have no update source")

// Release is the latest release of a track.
type Release struct {
	Version string
	// Cycle is the endoflife.date release cycle, such as "22.3" or "25".
	Cycle string
	// Tag is the GitHub release tag.
	Tag string
	// URL is a page about the release, if the source has one.
	URL string
	// Matched is the whole text a listing's regex matched for this version.
	// When the regex matches a filename, this is the exact file, which lets
	// the resolver name it even where the download folder can't be listed.
	Matched string
	// Asset is the matched GitHub release asset, when the entry sets
	// source.asset.
	Asset *Asset
	// Cycles lists the endoflife.date cycles that belong to the track, newest
	// first, so the cycle of a file on the drive can be checked for end of
	// life.
	Cycles []Cycle
}

// Cycle is an endoflife.date release cycle.
type Cycle struct {
	Name        string
	ReleaseDate string // YYYY-MM-DD
	LTS         bool
	EOL         bool
	EOLFrom     string // YYYY-MM-DD, or empty
	// Latest is the newest version in the cycle, or empty if endoflife.date
	// doesn't track one.
	Latest string
}

// Asset is a file attached to a GitHub release.
type Asset struct {
	Name string
	URL  string
	Size int64
	// SHA256 is GitHub's digest in lowercase hex, or empty if it has none.
	SHA256 string
}

// CycleFor returns the cycle a version belongs to: the cycle named exactly
// like the version, or the longest one whose name is a prefix of it ending at
// a separator ("21" for "21.3"). It returns nil if none fits.
func (r *Release) CycleFor(v string) *Cycle {
	var best *Cycle
	for i := range r.Cycles {
		c := &r.Cycles[i]
		fits := v == c.Name || (strings.HasPrefix(v, c.Name) && len(v) > len(c.Name) && strings.ContainsRune(".-_", rune(v[len(c.Name)])))
		if fits && (best == nil || len(c.Name) > len(best.Name)) {
			best = c
		}
	}
	return best
}

// Latest asks an entry's source for the latest release of its track.
func Latest(ctx context.Context, client *remote.Client, e *catalog.Entry) (*Release, error) {
	all, err := Versions(ctx, client, e)
	if err != nil {
		return nil, err
	}
	return &all[0], nil
}

// Versions asks an entry's source for every release of its track it still
// publishes, newest first: each endoflife.date cycle of the track (its latest
// version), each matching GitHub release, each version a listing shows. A
// static entry has the one; a manual entry has none (ErrManual). There is
// always at least one when the error is nil.
func Versions(ctx context.Context, client *remote.Client, e *catalog.Entry) ([]Release, error) {
	s := e.Source
	switch s.Type {
	case catalog.SourceEndOfLife:
		return endOfLife(ctx, client, s)
	case catalog.SourceGitHub:
		return gitHub(ctx, client, s)
	case catalog.SourceListing:
		return listing(ctx, client, s)
	case catalog.SourceStatic:
		return []Release{{Version: s.Version}}, nil
	case catalog.SourceManual:
		return nil, ErrManual
	}
	return nil, fmt.Errorf("unknown source type %q", s.Type)
}

// endOfLife lists the latest version of each cycle on the track, newest
// first.
func endOfLife(ctx context.Context, client *remote.Client, s catalog.Source) ([]Release, error) {
	u := "https://endoflife.date/api/v1/products/" + url.PathEscape(s.Product) + "/"
	resp, err := client.Get(ctx, u)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Result struct {
			Releases []struct {
				Name        string  `json:"name"`
				ReleaseDate string  `json:"releaseDate"`
				IsLTS       bool    `json:"isLts"`
				IsEOL       bool    `json:"isEol"`
				EOLFrom     *string `json:"eolFrom"`
				Latest      *struct {
					Name string `json:"name"`
					Link string `json:"link"`
				} `json:"latest"`
			} `json:"releases"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return nil, fmt.Errorf("endoflife.date %s: %w", s.Product, err)
	}

	var filter *regexp.Regexp
	if s.Cycles != "" {
		if filter, err = catalog.WholeRegexp(s.Cycles); err != nil {
			return nil, err
		}
	}
	rel := &Release{}
	links := map[string]string{}
	for _, r := range doc.Result.Releases {
		if filter != nil && !filter.MatchString(r.Name) {
			continue
		}
		c := Cycle{Name: r.Name, ReleaseDate: r.ReleaseDate, LTS: r.IsLTS, EOL: r.IsEOL}
		if r.EOLFrom != nil {
			c.EOLFrom = *r.EOLFrom
		}
		if r.Latest != nil {
			c.Latest = r.Latest.Name
			links[r.Name] = r.Latest.Link
		}
		rel.Cycles = append(rel.Cycles, c)
	}
	slices.SortStableFunc(rel.Cycles, func(a, b Cycle) int { return strings.Compare(b.ReleaseDate, a.ReleaseDate) })

	var out []Release
	for _, c := range rel.Cycles {
		if !onChannel(c, s.Channel) {
			continue
		}
		r := *rel
		r.Cycle, r.Version, r.URL = c.Name, c.Latest, links[c.Name]
		if r.Version == "" {
			r.Version = c.Name // cycles like Linux Mint's "22.3" are the version
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("endoflife.date has no %q release of %s", s.Channel, s.Product)
	}
	return out, nil
}

// onChannel says whether a cycle belongs to the track a channel names: every
// cycle for "latest", the long-term ones for "lts", or the one named.
func onChannel(c Cycle, channel string) bool {
	switch channel {
	case "latest":
		return true
	case "lts":
		return c.LTS
	}
	return c.Name == channel
}
