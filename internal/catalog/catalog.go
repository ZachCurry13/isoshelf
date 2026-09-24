// Package catalog loads and validates the image catalog: which filenames
// belong to which track, and where each track publishes its releases and
// checksums.
//
// A catalog is a TOML file. Each [[entry]] is one track: a single product,
// edition, architecture and channel. Updates stay inside a track, so a
// 32-bit track never "updates" to a 64-bit image.
package catalog

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// SchemaVersion is the catalog format this build reads.
const SchemaVersion = 1

// Source types: where an entry learns about its latest version.
const (
	SourceEndOfLife = "endoflife" // endoflife.date API
	SourceGitHub    = "github"    // GitHub releases
	SourceListing   = "listing"   // checksum file, directory index or JSON, read by regex
	SourceStatic    = "static"    // version pinned in the catalog
	SourceManual    = "manual"    // inventory only
)

// Signature shapes for Artifact.Signature.
const (
	SignatureManifest    = "manifest"    // detached signature over the manifest
	SignatureClearsigned = "clearsigned" // the manifest itself is clearsigned
	SignatureImage       = "image"       // detached signature over the image
)

// Catalog is a parsed and validated catalog.
type Catalog struct {
	Schema int `toml:"schema"`
	// Revision rises every time the catalog changes, written YYYYMMDDNN: the
	// date plus a two-digit counter, so a second change on one day still
	// sorts after the first and before tomorrow.
	// isoshelf uses it to tell a downloaded catalog from the one built into
	// the binary, so a copy downloaded months ago never shadows a newer
	// built-in one after an upgrade.
	Revision int                   `toml:"revision"`
	Keys     map[string]SigningKey `toml:"keys"`
	Entries  []Entry               `toml:"entry"`

	byID map[string]*Entry
}

// SigningKey is an OpenPGP public key that signs manifests or images.
type SigningKey struct {
	// File is the armored key, relative to the catalog file.
	File string `toml:"file"`
	// Fingerprints are the primary key fingerprints the file must contain,
	// normalized to uppercase hex without spaces.
	Fingerprints []string `toml:"fingerprints"`
}

// Entry is one track.
type Entry struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
	Arch string `toml:"arch"`
	// Match is a regular expression that must match a whole filename. A
	// named group "version" captures the version.
	Match string `toml:"match"`
	// FixedName marks images whose filename never changes between releases.
	// They have no version group; an update is a changed published checksum.
	FixedName bool `toml:"fixed_name"`
	// Samples are real filenames that must match this entry and no other.
	Samples []string `toml:"samples"`
	// Page is the human download page.
	Page string `toml:"page"`
	// Site is the project's home page, and Forum its community.
	Site  string `toml:"site"`
	Forum string `toml:"forum"`
	// Category groups entries in lists: see Categories.
	Category string `toml:"category"`
	// Family groups the tracks of one product, such as every MX Linux entry.
	Family string `toml:"family"`
	// Icon is the project's logo, as a Simple Icons slug, and IconColor its
	// brand color as #rrggbb.
	Icon      string `toml:"icon"`
	IconColor string `toml:"icon_color"`
	// Fixup makes a downloaded file bootable: "extract", "convert" or
	// "rename:<extension>".
	Fixup string `toml:"fixup"`
	// Caution is something worth knowing before using an image, in one plain
	// sentence of fact: support has ended, it is an unofficial modification,
	// a preview that expires. It is shown as a mark, never as a block; people
	// keep old images on purpose, for a VM or an old PC.
	Caution string `toml:"caution"`
	// Find says where to find the file on the download page, for an image
	// fetched by hand (v0.8.3, #58): "The 32-bit ISO is MX-{version}_386.iso,
	// in the Xfce folder." {version} stands for the newest version when the
	// check knows it. isoshelf learned the field in v0.8.3, and the built-in
	// catalog may only use it once older copies no longer matter, because
	// they refuse a catalog with a field they don't know.
	Find string `toml:"find"`
	// Popular marks images that turn up in public "best of" round-ups. It is
	// a hand-picked hint for sorting, dated in docs/catalog-sources.md, not a
	// rating and not a count of anything users did.
	Popular bool `toml:"popular"`
	// Size is roughly how big the download is, in bytes. It is a hint for the
	// page and the free-space check, not a promise: it changes with every
	// release. Filled in from a live run of the recorder.
	Size int64 `toml:"size"`
	// KnownHashes are known-good SHA-256 digests, normalized to lowercase.
	KnownHashes []string  `toml:"known_hashes"`
	Source      Source    `toml:"source"`
	Artifact    *Artifact `toml:"artifact"`

	match *regexp.Regexp
}

// Source says how to find an entry's latest version. Which fields apply
// depends on Type.
type Source struct {
	Type string `toml:"type"`

	// endoflife: Product is the endoflife.date product id. Channel is
	// "latest", "lts" or a pinned release cycle. Cycles, if set, must match a
	// whole cycle name for that cycle to belong to the track (endoflife.date
	// lists LMDE under Linux Mint, for example).
	Product string `toml:"product"`
	Channel string `toml:"channel"`
	Cycles  string `toml:"cycles"`

	// github: Repo is "owner/name". Tag matches the release tag and may
	// capture a version. Asset, if set, matches the image among the release
	// assets; otherwise the release is only a version label and Artifact
	// says where the file lives.
	Repo  string `toml:"repo"`
	Tag   string `toml:"tag"`
	Asset string `toml:"asset"`

	// listing: URL is fetched and Regex, which must have a version group,
	// finds the version in it.
	URL   string `toml:"url"`
	Regex string `toml:"regex"`

	// static: the pinned version.
	Version string `toml:"version"`
}

// Artifact says where the image and its checksums are published. An entry
// without one is check-only: it compares versions but can't download. Its fields
// are templates: {version} is available for all sources, {cycle} for
// endoflife and {tag} for github. Manifest and Sig may also use {file}, the
// resolved image filename.
type Artifact struct {
	// Base is the official https:// folder; it ends with "/".
	Base string `toml:"base"`
	// File matches the image filename, found in the manifest (or in the
	// directory index at Base when the manifest name uses {file}).
	File string `toml:"file"`
	// Manifest is the checksum file, relative to Base.
	Manifest string `toml:"manifest"`
	// Signature is "", "manifest", "clearsigned" or "image".
	Signature string `toml:"signature"`
	// Sig is the detached signature file, relative to Base.
	Sig string `toml:"sig"`
	// Key names a [keys] table.
	Key string `toml:"key"`
	// Mirrors are extra folders for the image bytes only. Checksums and
	// signatures always come from Base.
	Mirrors []string `toml:"mirrors"`
}

//go:embed default.toml
var defaultFS embed.FS

// Default returns the catalog built into the binary.
func Default() (*Catalog, error) {
	return Load(defaultFS, "default.toml")
}

// Load reads and validates the catalog file name in fsys. Signing key files
// are looked up relative to it. On failure the error is usually an *Error
// listing every problem found.
func Load(fsys fs.FS, name string) (*Catalog, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	var c Catalog
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, decodeError(name, err)
	}
	if err := c.validate(fsys, name); err != nil {
		return nil, err
	}
	return &c, nil
}

// Problem is one mistake in a catalog file.
type Problem struct {
	Line  int    // line in the file, or 0 when unknown
	Entry string // entry id, if it has a valid one
	Index int    // 1-based entry position, or 0 for catalog-wide problems
	Field string // for example "source.channel"
	Msg   string
}

func (p Problem) String() string {
	var b strings.Builder
	if p.Line > 0 {
		fmt.Fprintf(&b, "line %d: ", p.Line)
	}
	if p.Entry != "" || p.Index > 0 {
		fmt.Fprintf(&b, "entry %s: ", entryLabel(p.Entry, p.Index))
	}
	if p.Field != "" {
		b.WriteString(p.Field + ": ")
	}
	b.WriteString(p.Msg)
	return b.String()
}

// Error lists everything wrong with a catalog file, so it can be fixed in one
// pass.
type Error struct {
	File     string
	Problems []Problem
}

func (e *Error) Error() string {
	if len(e.Problems) == 1 {
		return fmt.Sprintf("catalog %s: %s", e.File, e.Problems[0])
	}
	var b strings.Builder
	fmt.Fprintf(&b, "catalog %s: %d problems:", e.File, len(e.Problems))
	for _, p := range e.Problems {
		b.WriteString("\n  " + p.String())
	}
	return b.String()
}

func entryLabel(id string, index int) string {
	if id != "" {
		return fmt.Sprintf("%q", id)
	}
	return fmt.Sprintf("#%d", index)
}

// decodeError turns a TOML decoding error into an *Error with line numbers.
func decodeError(file string, err error) error {
	out := &Error{File: file}
	var strict *toml.StrictMissingError
	var decode *toml.DecodeError
	switch {
	case errors.As(err, &strict):
		for i := range strict.Errors {
			line, _ := strict.Errors[i].Position()
			key := strings.Join(strict.Errors[i].Key(), ".")
			out.Problems = append(out.Problems, Problem{Line: line, Msg: fmt.Sprintf("unknown key %q", key)})
		}
	case errors.As(err, &decode):
		line, _ := decode.Position()
		out.Problems = append(out.Problems, Problem{Line: line, Msg: strings.TrimPrefix(decode.Error(), "toml: ")})
	default:
		out.Problems = append(out.Problems, Problem{Msg: strings.TrimPrefix(err.Error(), "toml: ")})
	}
	return out
}

// What isoshelf can do for an entry, as returned by Entry.Updates.
const (
	UpdatesDownload  = "download"   // check for updates and download them
	UpdatesCheckOnly = "check-only" // check for updates, nothing to download yet
	UpdatesManual    = "manual"     // recognize files and link the download page
)

// Updates says what isoshelf can do for the entry.
func (e *Entry) Updates() string {
	switch {
	case e.Source.Type == SourceManual:
		return UpdatesManual
	case e.Artifact != nil || (e.Source.Type == SourceGitHub && e.Source.Asset != ""):
		return UpdatesDownload
	}
	return UpdatesCheckOnly
}
