package catalog

import (
	"fmt"
	"io/fs"
	"maps"
	"net/url"
	"path"
	"regexp"
	"slices"
	"strings"
)

// Arches are the valid values of Entry.Arch.
var Arches = []string{"x86_64", "x86", "arm64", "arm", "multi"}

// An entry size has to be believable for a bootable image: netboot.xyz is a
// couple of megabytes, a Windows or "everything" DVD is a few tens of gigabytes.
const (
	minImageSize = 1 << 20  // 1 MB
	maxImageSize = 64 << 30 // 64 GB
)

// Categories group entries in lists. An entry without one counts as "other".
var Categories = []string{"desktop", "server", "security", "rescue", "windows", "other"}

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

func (v *validator) checkKey(name string, k SigningKey) {
	field := "keys." + name
	if !idPattern.MatchString(name) {
		v.problem(field, "name must be lowercase letters, digits and single dashes")
	}
	switch p := path.Join(v.dir, k.File); {
	case k.File == "":
		v.problem(field+".file", "required")
	case path.IsAbs(k.File) || strings.Contains(k.File, `\`) || !fs.ValidPath(p):
		v.problem(field+".file", "%q must be a relative path inside the catalog folder, using /", k.File)
	default:
		if _, err := fs.Stat(v.fsys, p); err != nil {
			v.problem(field+".file", "cannot read %q: %v", k.File, err)
		}
	}
	if len(k.Fingerprints) == 0 {
		v.problem(field+".fingerprints", "at least one fingerprint is required")
	}
	for i, fp := range k.Fingerprints {
		k.Fingerprints[i] = strings.ToUpper(strings.ReplaceAll(fp, " ", ""))
		if !fingerprintPattern.MatchString(k.Fingerprints[i]) {
			v.problem(field+".fingerprints", "%q is not a 40 or 64 digit hex fingerprint", fp)
		}
	}
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

func (ec entryCheck) checkIdentity(ids map[string]int) {
	e := ec.e
	switch {
	case e.ID == "":
		ec.problem("id", "required")
	case !idPattern.MatchString(e.ID):
		ec.problem("id", "%q must be lowercase letters, digits and single dashes", e.ID)
	case ids[e.ID] > 0:
		ec.problem("id", "also used by entry #%d", ids[e.ID])
	default:
		ids[e.ID] = ec.index
	}
	if strings.TrimSpace(e.Name) == "" {
		ec.problem("name", "required")
	}
	if !slices.Contains(Arches, e.Arch) {
		ec.problem("arch", "%q is not one of %s", e.Arch, strings.Join(Arches, ", "))
	}
}

func (ec entryCheck) checkSource() {
	s := &ec.e.Source
	switch s.Type {
	case SourceEndOfLife, SourceGitHub, SourceListing, SourceStatic, SourceManual:
	case "":
		ec.problem("source.type", "required: endoflife, github, listing, static or manual")
		return
	default:
		ec.problem("source.type", "unknown type %q; use endoflife, github, listing, static or manual", s.Type)
		return
	}

	// Each field belongs to one source type. Setting a field another type
	// uses is almost always a mistake, so it is reported.
	for _, f := range []struct{ name, value, owner string }{
		{"product", s.Product, SourceEndOfLife},
		{"channel", s.Channel, SourceEndOfLife},
		{"cycles", s.Cycles, SourceEndOfLife},
		{"repo", s.Repo, SourceGitHub},
		{"tag", s.Tag, SourceGitHub},
		{"asset", s.Asset, SourceGitHub},
		{"url", s.URL, SourceListing},
		{"regex", s.Regex, SourceListing},
		{"version", s.Version, SourceStatic},
	} {
		switch {
		case f.owner != s.Type && f.value != "":
			ec.problem("source."+f.name, "not used by %s sources", s.Type)
		case f.owner == s.Type && f.value == "" && f.name != "asset" && f.name != "cycles":
			ec.problem("source."+f.name, "required for %s sources", s.Type)
		}
	}

	switch s.Type {
	case SourceEndOfLife:
		if s.Product != "" && !productPattern.MatchString(s.Product) {
			ec.problem("source.product", "%q is not an endoflife.date product id", s.Product)
		}
		if s.Channel != "" && s.Channel != "latest" && s.Channel != "lts" && !cyclePattern.MatchString(s.Channel) {
			ec.problem("source.channel", `%q must be "latest", "lts" or a release cycle such as "24.04"`, s.Channel)
		}
		if s.Cycles != "" {
			if _, err := WholeRegexp(s.Cycles); err != nil {
				ec.problem("source.cycles", "%v", err)
			}
		}
	case SourceGitHub:
		if s.Repo != "" && !repoPattern.MatchString(s.Repo) {
			ec.problem("source.repo", "%q must look like owner/name", s.Repo)
		}
		if s.Tag != "" {
			if _, err := WholeRegexp(s.Tag); err != nil {
				ec.problem("source.tag", "%v", err)
			}
		}
		if s.Asset != "" {
			if _, err := WholeRegexp(s.Asset); err != nil {
				ec.problem("source.asset", "%v", err)
			}
		}
	case SourceListing:
		if s.URL != "" {
			ec.checkURL("source.url", s.URL, nil, officialURL)
		}
		if s.Regex != "" {
			re, err := regexp.Compile(s.Regex)
			switch {
			case err != nil:
				ec.problem("source.regex", "%v", err)
			case re.SubexpIndex("version") < 0:
				ec.problem("source.regex", "needs a (?P<version>...) group")
			}
		}
	}
}

func (ec entryCheck) checkMatch() {
	e := ec.e
	if e.Match == "" {
		ec.problem("match", "required")
		return
	}
	re, err := WholeRegexp(e.Match)
	if err != nil {
		ec.problem("match", "%v", err)
		return
	}
	hasVersion := false
	for _, name := range re.SubexpNames() {
		switch name {
		case "":
		case "version":
			hasVersion = true
		default:
			ec.problem("match", `unknown group name %q; only "version" is used`, name)
		}
	}
	switch {
	case e.FixedName && hasVersion:
		ec.problem("match", "fixed_name entries must not capture a version")
	case !e.FixedName && !hasVersion && !ec.manual():
		ec.problem("match", "needs a (?P<version>...) group, or fixed_name = true if the filename never changes")
	}
	e.match = re
}

func (ec entryCheck) checkSamples() {
	if len(ec.e.Samples) == 0 {
		ec.problem("samples", "add at least one real filename this entry should match")
	}
	seen := map[string]bool{}
	for _, s := range ec.e.Samples {
		if s == "" || strings.ContainsAny(s, `/\`) {
			ec.problem("samples", "%q must be a bare filename", s)
		}
		if seen[s] {
			ec.problem("samples", "%q is listed twice", s)
		}
		seen[s] = true
	}
}

// checkExtras checks page, fixup and known_hashes. hashes maps each known
// hash to the label of the entry that listed it first.
func (ec entryCheck) checkExtras(hashes map[string]string) {
	e := ec.e
	for _, link := range []struct{ field, url string }{{"site", e.Site}, {"forum", e.Forum}} {
		if link.url != "" {
			ec.checkURL(link.field, link.url, nil, webURL)
		}
	}
	if e.Category != "" && !slices.Contains(Categories, e.Category) {
		ec.problem("category", "%q is not one of %s", e.Category, strings.Join(Categories, ", "))
	}
	if e.Family != "" && !idPattern.MatchString(e.Family) {
		ec.problem("family", "%q must be lowercase letters, digits and single dashes", e.Family)
	}
	if e.Icon != "" && !iconPattern.MatchString(e.Icon) {
		ec.problem("icon", "%q is not a Simple Icons name", e.Icon)
	}
	if e.IconColor != "" && !colorPattern.MatchString(e.IconColor) {
		ec.problem("icon_color", "%q is not a colour like #0078d4", e.IconColor)
	}
	// A size is a hint, so it only has to be believable for an image: a
	// mistyped one (bytes read as megabytes, or an extra zero) would be worse
	// than none, because the page would promise it.
	switch {
	case e.Size < 0:
		ec.problem("size", "can't be negative")
	case e.Size > 0 && e.Size < minImageSize:
		ec.problem("size", "%d bytes is too small for an image; sizes are in bytes", e.Size)
	case e.Size > maxImageSize:
		ec.problem("size", "%d bytes is bigger than any image isoshelf expects", e.Size)
	}

	// An entry isoshelf can't download from needs somewhere to send the user
	// instead. The exception is a manual entry, because an image someone
	// named themselves ("my old install disc") may have nowhere to point at;
	// a test holds the published catalog to the stricter rule.
	switch {
	case e.Page != "":
		ec.checkURL("page", e.Page, nil, webURL)
	case ec.checkOnly():
		ec.problem("page", `required for entries without [entry.artifact] (used by "Open download page")`)
	}

	switch {
	case e.Fixup == "", e.Fixup == "extract", e.Fixup == "convert":
	case strings.HasPrefix(e.Fixup, "rename:"):
		if ext := strings.TrimPrefix(e.Fixup, "rename:"); !slices.Contains(ImageExtensions, ext) {
			ec.problem("fixup", "rename target %q is not a type Ventoy lists (%s)", ext, strings.Join(ImageExtensions, " "))
		}
	default:
		ec.problem("fixup", `%q is not "extract", "convert" or "rename:<extension>"`, e.Fixup)
	}

	label := entryLabel(e.ID, ec.index)
	for i, h := range e.KnownHashes {
		e.KnownHashes[i] = strings.ToLower(strings.TrimSpace(h))
		h = e.KnownHashes[i]
		if !sha256Pattern.MatchString(h) {
			ec.problem("known_hashes", "%q is not a SHA-256 (64 hex digits)", h)
			continue
		}
		if first, dup := hashes[h]; dup {
			ec.problem("known_hashes", "%s... is already listed by entry %s", h[:16], first)
			continue
		}
		hashes[h] = label
	}
}

func (ec entryCheck) checkArtifact(keys map[string]SigningKey) {
	e := ec.e
	a := e.Artifact
	t := e.Source.Type

	vars := templateVars(t)
	if vars == nil && t != SourceManual {
		return // unknown source type, already reported
	}

	if e.FixedName && t != SourceManual {
		fromAsset := t == SourceGitHub && e.Source.Asset != ""
		if !fromAsset && (a == nil || a.Manifest == "") {
			ec.problem("fixed_name", "needs a published checksum (artifact.manifest or source.asset), because updates are detected by a changed checksum")
		}
	}

	switch {
	case t == SourceManual:
		if a != nil {
			ec.problem("artifact", "manual entries have no artifact; set page (and known_hashes) instead")
		}
		return
	case t == SourceGitHub && e.Source.Asset != "":
		if a != nil {
			ec.problem("artifact", "not used when source.asset is set; the file and its SHA-256 come from the GitHub release")
		}
		return
	case a == nil:
		return // check-only
	}

	withFile := append(slices.Clone(vars), "file")

	if a.Base == "" {
		ec.problem("artifact.base", "required")
	} else {
		ec.checkURL("artifact.base", a.Base, vars, officialURL)
		if !strings.HasSuffix(a.Base, "/") {
			ec.problem("artifact.base", "must end with /")
		}
	}
	if a.File == "" {
		ec.problem("artifact.file", "required")
	} else if ec.checkPlaceholders("artifact.file", a.File, vars) {
		if _, err := ExpandRegexp(a.File, sampleValues); err != nil {
			ec.problem("artifact.file", "%v", err)
		}
	}
	if a.Manifest != "" {
		ec.checkURL("artifact.manifest", a.Manifest, withFile, relativeURL)
	}
	if a.Sig != "" {
		ec.checkURL("artifact.sig", a.Sig, withFile, relativeURL)
	}
	for _, m := range a.Mirrors {
		ec.checkURL("artifact.mirrors", m, vars, webURL)
		if !strings.HasSuffix(m, "/") {
			ec.problem("artifact.mirrors", "%q must end with /", m)
		}
	}
	if len(a.Mirrors) > 0 && a.Manifest == "" {
		ec.problem("artifact.mirrors", "mirrors need artifact.manifest, so their downloads can be verified")
	}

	switch a.Signature {
	case "":
		if a.Sig != "" {
			ec.problem("artifact.sig", `set artifact.signature to say what it signs ("manifest" or "image")`)
		}
		if a.Key != "" {
			ec.problem("artifact.key", "not used without artifact.signature")
		}
	case SignatureManifest, SignatureClearsigned, SignatureImage:
		if a.Signature == SignatureClearsigned && a.Sig != "" {
			ec.problem("artifact.sig", `not used when signature = "clearsigned"; the manifest carries the signature`)
		}
		if a.Signature != SignatureClearsigned && a.Sig == "" {
			ec.problem("artifact.sig", "required when signature = %q", a.Signature)
		}
		if a.Signature != SignatureImage && a.Manifest == "" {
			ec.problem("artifact.manifest", "required when signature = %q", a.Signature)
		}
		if a.Key == "" {
			ec.problem("artifact.key", "required when signature is set")
		} else if _, ok := keys[a.Key]; !ok {
			ec.problem("artifact.key", "no [keys.%s] table in the catalog", a.Key)
		}
	default:
		ec.problem("artifact.signature", `%q is not "manifest", "clearsigned" or "image"`, a.Signature)
	}
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

// checkPlaceholders reports placeholders in tmpl that are not in allowed.
func (ec entryCheck) checkPlaceholders(field, tmpl string, allowed []string) bool {
	ok := true
	for _, name := range Placeholders(tmpl) {
		if slices.Contains(allowed, name) {
			continue
		}
		ok = false
		if len(allowed) == 0 {
			ec.problem(field, "{%s} cannot be used here; this field takes no placeholders", name)
		} else {
			ec.problem(field, "{%s} cannot be used here; available: {%s}", name, strings.Join(allowed, "}, {"))
		}
	}
	return ok
}
