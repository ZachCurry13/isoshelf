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
	"github.com/ZachCurry13/isoshelf/internal/version"
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
	s := e.Source
	switch s.Type {
	case catalog.SourceEndOfLife:
		return endOfLife(ctx, client, s)
	case catalog.SourceGitHub:
		return gitHub(ctx, client, s)
	case catalog.SourceListing:
		return listing(ctx, client, s)
	case catalog.SourceStatic:
		return &Release{Version: s.Version}, nil
	case catalog.SourceManual:
		return nil, ErrManual
	}
	return nil, fmt.Errorf("unknown source type %q", s.Type)
}

func endOfLife(ctx context.Context, client *remote.Client, s catalog.Source) (*Release, error) {
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

	i := slices.IndexFunc(rel.Cycles, func(c Cycle) bool {
		switch s.Channel {
		case "latest":
			return true
		case "lts":
			return c.LTS
		}
		return c.Name == s.Channel
	})
	if i < 0 {
		return nil, fmt.Errorf("endoflife.date has no %q release of %s", s.Channel, s.Product)
	}
	chosen := rel.Cycles[i]
	rel.Cycle, rel.Version, rel.URL = chosen.Name, chosen.Latest, links[chosen.Name]
	if rel.Version == "" {
		rel.Version = chosen.Name // cycles like Linux Mint's "22.3" are the version
	}
	return rel, nil
}

func gitHub(ctx context.Context, client *remote.Client, s catalog.Source) (*Release, error) {
	resp, err := client.Get(ctx, "https://api.github.com/repos/"+s.Repo+"/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		HTMLURL    string `json:"html_url"`
		Assets     []struct {
			Name   string  `json:"name"`
			Size   int64   `json:"size"`
			Digest *string `json:"digest"`
			URL    string  `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(resp.Body, &releases); err != nil {
		return nil, fmt.Errorf("GitHub releases of %s: %w", s.Repo, err)
	}

	tagRE, err := catalog.WholeRegexp(s.Tag)
	if err != nil {
		return nil, err
	}
	var assetRE *regexp.Regexp
	if s.Asset != "" {
		if assetRE, err = catalog.WholeRegexp(s.Asset); err != nil {
			return nil, err
		}
	}

	// GitHub lists the newest releases first. Drafts and prereleases never
	// count as updates.
	for _, r := range releases {
		m := tagRE.FindStringSubmatch(r.TagName)
		if r.Draft || r.Prerelease || m == nil {
			continue
		}
		rel := &Release{Version: r.TagName, Tag: r.TagName, URL: r.HTMLURL}
		if i := tagRE.SubexpIndex("version"); i >= 0 && m[i] != "" {
			rel.Version = m[i]
		}
		if assetRE == nil {
			return rel, nil
		}
		for _, a := range r.Assets {
			if assetRE.MatchString(a.Name) {
				rel.Asset = &Asset{Name: a.Name, URL: a.URL, Size: a.Size}
				if a.Digest != nil {
					if hex, ok := strings.CutPrefix(*a.Digest, "sha256:"); ok {
						rel.Asset.SHA256 = strings.ToLower(hex)
					}
				}
				return rel, nil
			}
		}
	}
	if assetRE != nil {
		return nil, fmt.Errorf("no release of %s has a tag matching %q and an asset matching %q", s.Repo, s.Tag, s.Asset)
	}
	return nil, fmt.Errorf("no release of %s has a tag matching %q", s.Repo, s.Tag)
}

func listing(ctx context.Context, client *remote.Client, s catalog.Source) (*Release, error) {
	resp, err := client.Get(ctx, s.URL)
	if err != nil {
		return nil, err
	}
	re, err := regexp.Compile(s.Regex)
	if err != nil {
		return nil, err
	}
	i := re.SubexpIndex("version")
	if i < 0 {
		return nil, fmt.Errorf("source.regex %q has no version group", s.Regex)
	}
	var latest, matched string
	for _, m := range re.FindAllSubmatch(resp.Body, -1) {
		if v := string(m[i]); v != "" && (latest == "" || version.Compare(v, latest) > 0) {
			latest, matched = v, string(m[0])
		}
	}
	if latest == "" {
		return nil, fmt.Errorf("no version matching %q found at %s", s.Regex, s.URL)
	}
	return &Release{Version: latest, Matched: matched}, nil
}
