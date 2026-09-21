package catalog

import (
	"slices"
	"strings"
)

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
		ec.problem("icon_color", "%q is not a color like #0078d4", e.IconColor)
	}
	// A caution is one sentence someone reads while hovering a mark, so it
	// stays short and on one line.
	switch c := e.Caution; {
	case strings.ContainsAny(c, "\r\n"):
		ec.problem("caution", "must be one line")
	case len(c) > maxCaution:
		ec.problem("caution", "is %d characters; keep it under %d, one plain sentence", len(c), maxCaution)
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
