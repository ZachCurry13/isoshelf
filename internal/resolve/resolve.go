// Package resolve turns an entry's latest release into the exact file: its
// name, where to download it, and its published checksum.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/verify"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

// ErrNoArtifact means the entry has nothing to download: it's manual or
// check-only.
var ErrNoArtifact = errors.New("entry has nothing to download")

// Artifact is a resolved file.
type Artifact struct {
	Filename string
	// URLs are download locations: the official one first, then mirrors.
	URLs []string
	// Size is the size in bytes, if the source says (GitHub does).
	Size int64
	// Checksum is the published checksum, or nil if none is published, in
	// which case a download is "unverified".
	Checksum *verify.Checksum
	// ChecksumURL is where the checksum came from.
	ChecksumURL string
}

// Resolve finds the file for rel, the latest release of entry e.
func Resolve(ctx context.Context, client *remote.Client, e *catalog.Entry, rel *source.Release) (*Artifact, error) {
	if rel.Asset != nil {
		a := &Artifact{Filename: rel.Asset.Name, URLs: []string{rel.Asset.URL}, Size: rel.Asset.Size}
		if rel.Asset.SHA256 != "" {
			a.Checksum = &verify.Checksum{Name: rel.Asset.Name, Algorithm: verify.SHA256, Hex: rel.Asset.SHA256}
			a.ChecksumURL = "https://api.github.com/repos/" + e.Source.Repo + "/releases"
		}
		return a, nil
	}
	spec := e.Artifact
	if spec == nil || e.Source.Type == catalog.SourceManual {
		return nil, ErrNoArtifact
	}

	vars := map[string]string{"version": rel.Version, "cycle": rel.Cycle, "tag": rel.Tag}
	base, err := expandURL(nil, spec.Base, vars)
	if err != nil {
		return nil, fmt.Errorf("artifact.base: %w", err)
	}
	fileRE, err := catalog.ExpandRegexp(spec.File, vars)
	if err != nil {
		return nil, fmt.Errorf("artifact.file: %w", err)
	}

	a := &Artifact{}
	if spec.Manifest == "" || slices.Contains(catalog.Placeholders(spec.Manifest), "file") {
		// A listing that matched the whole filename has already said which
		// file it is, which matters where the download folder can't be
		// listed. Otherwise the folder is read to find it.
		if name := path.Base(rel.Matched); rel.Matched != "" && fileRE.MatchString(name) {
			a.Filename = name
		} else if a.Filename, err = findInIndex(ctx, client, base, fileRE); err != nil {
			return nil, err
		}
		vars["file"] = a.Filename
	}

	if spec.Manifest != "" {
		manifestURL, err := expandURL(base, spec.Manifest, vars)
		if err != nil {
			return nil, fmt.Errorf("artifact.manifest: %w", err)
		}
		resp, err := client.Get(ctx, manifestURL.String())
		if err != nil {
			return nil, err
		}
		if resp.URL.Host != manifestURL.Host {
			return nil, fmt.Errorf("checksum file %s redirected to %s; checksums must come from the official site", manifestURL, resp.URL.Host)
		}
		checksums := verify.ParseManifest(resp.Body)
		if a.Filename == "" {
			var names []string
			for _, c := range checksums {
				names = append(names, c.Name)
			}
			if a.Filename = newest(names, fileRE); a.Filename == "" {
				return nil, fmt.Errorf("no file matching %q in %s", fileRE, manifestURL)
			}
		}
		c, ok := verify.Strongest(checksums, a.Filename)
		if !ok {
			return nil, fmt.Errorf("%s has no checksum for %s", manifestURL, a.Filename)
		}
		a.Checksum, a.ChecksumURL = &c, manifestURL.String()
	}

	fileRef := &url.URL{Path: a.Filename}
	a.URLs = append(a.URLs, base.ResolveReference(fileRef).String())
	for _, m := range spec.Mirrors {
		mirror, err := expandURL(nil, m, vars)
		if err != nil {
			return nil, fmt.Errorf("artifact.mirrors: %w", err)
		}
		a.URLs = append(a.URLs, mirror.ResolveReference(fileRef).String())
	}
	return a, nil
}

// expandURL expands a URL template and resolves it against base, if given.
func expandURL(base *url.URL, tmpl string, vars map[string]string) (*url.URL, error) {
	s, err := catalog.Expand(tmpl, vars)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	return u, nil
}

var hrefPattern = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']+)["']`)

// findInIndex reads the directory index at dir and returns the newest file
// whose name matches fileRE.
func findInIndex(ctx context.Context, client *remote.Client, dir *url.URL, fileRE *regexp.Regexp) (string, error) {
	resp, err := client.Get(ctx, dir.String())
	if err != nil {
		return "", err
	}
	var names []string
	for _, m := range hrefPattern.FindAllStringSubmatch(string(resp.Body), -1) {
		ref, err := url.Parse(html.UnescapeString(m[1]))
		if err != nil || strings.HasSuffix(ref.Path, "/") {
			continue // not a URL, or a folder
		}
		names = append(names, path.Base(ref.Path))
	}
	name := newest(names, fileRE)
	if name == "" {
		return "", fmt.Errorf("no file matching %q in %s", fileRE, dir)
	}
	return name, nil
}

// newest returns the name matching re that sorts last as a version, or "".
func newest(names []string, re *regexp.Regexp) string {
	var best string
	for _, n := range names {
		if re.MatchString(n) && (best == "" || version.Compare(n, best) > 0) {
			best = n
		}
	}
	return best
}
