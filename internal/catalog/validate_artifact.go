package catalog

import (
	"io/fs"
	"path"
	"slices"
	"strings"
)

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
