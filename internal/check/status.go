package check

import (
	"fmt"
	"path"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/verify"
	"github.com/ZachCurry13/isoshelf/internal/version"
)

func notBootableNote(e *catalog.Entry, profile scan.Profile) string {
	exts := strings.Join(profile.Extensions(), " ")
	if e.Fixup != "" {
		return fmt.Sprintf("%s lists only %s; Make bootable can fix this (%s)", profile, exts, e.Fixup)
	}
	return fmt.Sprintf("%s lists only %s", profile, exts)
}

// decide sets an item's status from the check result. recorded is the
// SHA-256 recorded for the file, or empty.
func decide(it *Item, rel *source.Release, art *resolve.Artifact, err error, recorded string) {
	if err != nil {
		switch it.Status {
		case NotBootable: // keep the more useful note
		case Missing:
			it.Note = err.Error()
		default:
			it.Status, it.Note = CheckFailed, err.Error()
		}
		return
	}
	e := it.Entry
	it.Latest, it.Release = rel.Version, rel.URL
	if art != nil {
		it.LatestFile = art.Filename
		if v, ok := e.MatchName(art.Filename); ok && v != "" {
			it.Latest = v
		}
	}
	if it.Status == Missing {
		return
	}
	it.EOL = fileCycleEOL(e, rel, it.Version)

	status, note := compare(it, e, art, recorded)
	switch {
	case it.Status == NotBootable:
		// Keep the status; the latest version is still worth showing.
	case status == UpToDate && it.EOL:
		it.Status = EOL
	default:
		it.Status, it.Note = status, note
	}
}

func compare(it *Item, e *catalog.Entry, art *resolve.Artifact, recorded string) (Status, string) {
	if e.FixedName {
		switch {
		case art == nil || art.Checksum == nil:
			return Unknown, "no published checksum to compare with"
		case art.Checksum.Algorithm != verify.SHA256:
			return Unknown, fmt.Sprintf("the published checksum is %s; only SHA-256 can be compared", art.Checksum.Algorithm)
		case recorded == "":
			return Unknown, "not hashed yet"
		case recorded == art.Checksum.Hex:
			return UpToDate, ""
		}
		return UpdateAvailable, "the published checksum changed"
	}

	if art != nil && strings.EqualFold(art.Filename, path.Base(it.Path)) {
		if recorded != "" && art.Checksum != nil && art.Checksum.Algorithm == verify.SHA256 && recorded != art.Checksum.Hex {
			return ChecksumMismatch, "the file doesn't match its published SHA-256"
		}
		return UpToDate, ""
	}
	if it.Version == "" {
		return Unknown, "the filename has no version to compare"
	}
	if version.Compare(it.Latest, it.Version) > 0 {
		return UpdateAvailable, ""
	}
	return UpToDate, ""
}

// fileCycleEOL reports whether the endoflife.date cycle the file belongs to
// has reached end of life. A pinned channel is the file's cycle; otherwise
// the cycle is found from the file's version.
func fileCycleEOL(e *catalog.Entry, rel *source.Release, v string) bool {
	if e.Source.Type != catalog.SourceEndOfLife {
		return false
	}
	var c *source.Cycle
	if ch := e.Source.Channel; ch != "latest" && ch != "lts" {
		c = rel.CycleFor(rel.Cycle)
	} else {
		c = rel.CycleFor(v)
	}
	return c != nil && c.EOL
}
