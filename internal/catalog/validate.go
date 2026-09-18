package catalog

import (
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"path"
	"regexp"
	"slices"
)

// Arches are the valid values of Entry.Arch.
var Arches = []string{"x86_64", "x86", "arm64", "arm", "multi"}

// An entry size has to be believable for a bootable image: netboot.xyz is a
// couple of megabytes, a Windows or "everything" DVD is a few tens of gigabytes.
const (
	minImageSize = 1 << 20  // 1 MB
	maxImageSize = 64 << 30 // 64 GB

	// maxCaution keeps a caution to a sentence.
	maxCaution = 200
)

// Categories group entries in lists. An entry without one counts as "other".
var Categories = []string{"desktop", "gaming", "server", "boards", "security", "rescue", "windows", "other"}

// ImageExtensions are the file types Ventoy lists in its boot menu.
var ImageExtensions = []string{".iso", ".wim", ".img", ".vhd", ".vhdx", ".efi"}

var (
	idPattern          = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	productPattern     = regexp.MustCompile(`^[a-z0-9]+(?:[-.][a-z0-9]+)*$`)
	cyclePattern       = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-._][A-Za-z0-9]+)*$`)
	repoPattern        = regexp.MustCompile(`^[A-Za-z0-9-]+/[A-Za-z0-9._-]+$`)
	sha256Pattern      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	iconPattern        = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)
	colorPattern       = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fingerprintPattern = regexp.MustCompile(`^(?:[0-9A-F]{40}|[0-9A-F]{64})$`)
)

// sampleValues fill placeholders when checking that a template is well formed.
var sampleValues = map[string]string{
	"cycle":   "1",
	"version": "1.0",
	"tag":     "v1.0",
	"file":    "image.iso",
}

// templateVars returns the placeholders a source type provides.
func templateVars(sourceType string) []string {
	switch sourceType {
	case SourceEndOfLife:
		return []string{"cycle", "version"}
	case SourceGitHub:
		return []string{"tag", "version"}
	case SourceListing, SourceStatic:
		return []string{"version"}
	}
	return nil
}

type validator struct {
	fsys     fs.FS
	dir      string // folder of the catalog file inside fsys
	problems []Problem
}

func (v *validator) problem(field, format string, args ...any) {
	v.problems = append(v.problems, Problem{Field: field, Msg: fmt.Sprintf(format, args...)})
}

// validate checks everything that can be checked offline, compiles the match
// patterns and normalizes hashes and fingerprints.
func (c *Catalog) validate(fsys fs.FS, file string) error {
	v := &validator{fsys: fsys, dir: path.Dir(file)}

	switch c.Schema {
	case SchemaVersion:
	case 0:
		v.problem("schema", "missing; add schema = %d at the top", SchemaVersion)
	default:
		v.problem("schema", "is %d, but this version of isoshelf reads schema %d", c.Schema, SchemaVersion)
		return &Error{File: file, Problems: v.problems}
	}

	for _, name := range slices.Sorted(maps.Keys(c.Keys)) {
		v.checkKey(name, c.Keys[name])
	}

	if len(c.Entries) == 0 {
		v.problem("entry", "the catalog has no entries")
	}
	ids := map[string]int{}
	hashes := map[string]string{}
	for i := range c.Entries {
		ec := entryCheck{v: v, e: &c.Entries[i], index: i + 1}
		ec.checkIdentity(ids)
		ec.checkSource()
		ec.checkMatch()
		ec.checkSamples()
		ec.checkExtras(hashes)
		ec.checkArtifact(c.Keys)
	}
	v.checkSampleMatches(c)

	if len(v.problems) > 0 {
		return &Error{File: file, Problems: v.problems}
	}
	c.byID = make(map[string]*Entry, len(c.Entries))
	for i := range c.Entries {
		c.byID[c.Entries[i].ID] = &c.Entries[i]
	}
	return nil
}

// entryCheck records problems for one entry.
type entryCheck struct {
	v     *validator
	e     *Entry
	index int
}

func (ec entryCheck) problem(field, format string, args ...any) {
	id := ec.e.ID
	if !idPattern.MatchString(id) {
		id = ""
	}
	ec.v.problems = append(ec.v.problems, Problem{
		Entry: id,
		Index: ec.index,
		Field: field,
		Msg:   fmt.Sprintf(format, args...),
	})
}

func (ec entryCheck) manual() bool {
	return ec.e.Source.Type == SourceManual
}

// checkOnly reports whether a non-manual entry has nowhere to download from.
func (ec entryCheck) checkOnly() bool {
	s := ec.e.Source
	fromAsset := s.Type == SourceGitHub && s.Asset != ""
	return templateVars(s.Type) != nil && ec.e.Artifact == nil && !fromAsset
}

// checkSampleMatches checks that every sample matches its own entry and no
// other. It runs after all match patterns are compiled.
func (v *validator) checkSampleMatches(c *Catalog) {
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.match == nil {
			continue
		}
		ec := entryCheck{v: v, e: e, index: i + 1}
		needVersion := !e.FixedName && !ec.manual()
		for _, s := range e.Samples {
			version, ok := e.MatchName(s)
			if !ok {
				ec.problem("samples", "%q does not match this entry", s)
				continue
			}
			if needVersion && version == "" {
				ec.problem("samples", "%q matches but captures an empty version", s)
			}
			for j := range c.Entries {
				other := &c.Entries[j]
				if _, ok := other.MatchName(s); ok && j != i {
					ec.problem("samples", "%q also matches entry %s", s, entryLabel(other.ID, j+1))
				}
			}
		}
	}
}

type urlKind int

const (
	officialURL urlKind = iota // absolute https://
	relativeURL                // relative to artifact.base, or absolute https://
	webURL                     // absolute http:// or https://
)

func (ec entryCheck) checkURL(field, tmpl string, vars []string, kind urlKind) {
	if !ec.checkPlaceholders(field, tmpl, vars) {
		return
	}
	s, err := Expand(tmpl, sampleValues)
	if err != nil {
		ec.problem(field, "%v", err)
		return
	}
	u, err := url.Parse(s)
	if err != nil {
		ec.problem(field, "%q is not a valid URL", tmpl)
		return
	}
	switch kind {
	case officialURL:
		if u.Scheme != "https" || u.Host == "" {
			ec.problem(field, "%q must be an https:// URL", tmpl)
		}
	case relativeURL:
		relative := !u.IsAbs() && u.Host == ""
		official := u.Scheme == "https" && u.Host != ""
		if !relative && !official {
			ec.problem(field, "%q must be relative to artifact.base or an https:// URL", tmpl)
		}
	case webURL:
		if (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			ec.problem(field, "%q must be an http:// or https:// URL", tmpl)
		}
	}
}
