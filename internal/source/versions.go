package source

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

// gitHub lists the releases whose tag matches (and which carry a matching
// asset, when the entry names one), in GitHub's order: newest first.
func gitHub(ctx context.Context, client *remote.Client, s catalog.Source) ([]Release, error) {
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

	// Drafts and prereleases never count as releases.
	var out []Release
	for _, r := range releases {
		m := tagRE.FindStringSubmatch(r.TagName)
		if r.Draft || r.Prerelease || m == nil {
			continue
		}
		rel := Release{Version: r.TagName, Tag: r.TagName, URL: r.HTMLURL}
		if i := tagRE.SubexpIndex("version"); i >= 0 && m[i] != "" {
			rel.Version = m[i]
		}
		if assetRE == nil {
			out = append(out, rel)
			continue
		}
		for _, a := range r.Assets {
			if assetRE.MatchString(a.Name) {
				rel.Asset = &Asset{Name: a.Name, URL: a.URL, Size: a.Size}
				if a.Digest != nil {
					if hex, ok := strings.CutPrefix(*a.Digest, "sha256:"); ok {
						rel.Asset.SHA256 = strings.ToLower(hex)
					}
				}
				out = append(out, rel)
				break
			}
		}
	}
	switch {
	case len(out) > 0:
		return out, nil
	case assetRE != nil:
		return nil, fmt.Errorf("no release of %s has a tag matching %q and an asset matching %q", s.Repo, s.Tag, s.Asset)
	}
	return nil, fmt.Errorf("no release of %s has a tag matching %q", s.Repo, s.Tag)
}

// listing lists every version the page names, newest first. Where one
// version is matched more than once, the first match is the one kept.
func listing(ctx context.Context, client *remote.Client, s catalog.Source) ([]Release, error) {
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
	var out []Release
	seen := map[string]bool{}
	for _, m := range re.FindAllSubmatch(resp.Body, -1) {
		if v := string(m[i]); v != "" && !seen[v] {
			seen[v] = true
			out = append(out, Release{Version: v, Matched: string(m[0])})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no version matching %q found at %s", s.Regex, s.URL)
	}
	slices.SortStableFunc(out, func(a, b Release) int { return version.Compare(b.Version, a.Version) })
	return out, nil
}
