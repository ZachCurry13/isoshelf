// Package update puts new images in place and removes unwanted ones. It is
// the only package that deletes anything, and it follows isoshelf's rules:
// only image files inside the target, never without being asked, and never a
// file that failed its checksum.
package update

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// Removal says what happens to a file the user no longer wants, or to the old
// file after an update.
type Removal string

const (
	// Keep leaves the old file where it is.
	Keep Removal = "keep"
	// MoveAside moves it into .isoshelf/removed, which is instant and can be
	// undone; the space is freed when the user empties that folder.
	MoveAside Removal = "move-aside"
	// DeleteNow deletes it straight away.
	DeleteNow Removal = "delete"
)

// Options describe one update.
type Options struct {
	// Target is the folder the image belongs in.
	Target string
	// Entry is the track to update.
	Entry *catalog.Entry
	// Client asks the source where the newest file is; Fetcher downloads it.
	Client  *remote.Client
	Fetcher *fetch.Client
	// State is updated in place. The caller saves it.
	State *state.State
	// Old are the entry's current files, as paths relative to Target.
	Old []string
	// Removal says what happens to the old files once the new one is in
	// place. It only ever applies to files of this entry.
	Removal Removal
	// Pinned are the old files the user pinned. Whatever Removal says, they
	// stay exactly where they are: the new file goes beside them, taking a
	// name with its version in it if it would otherwise land on one.
	Pinned []string
	// Version, when set, is the version to install instead of the newest.
	// (Reserved for "install an older version"; not used yet.)
	Version  string
	Progress func(fetch.Progress)
	Now      func() time.Time
	// Nearer, when set, is asked whether somewhere closer than the internet
	// already holds this exact file - another isoshelf on the same network,
	// usually the one on the machine the images live on. Whatever it returns
	// is tried before the project's own site.
	//
	// It is only ever asked when the project publishes a checksum, because
	// that checksum is what makes a copy from anywhere else safe to try: the
	// bytes are checked against it before they are placed, and a copy that
	// doesn't match costs one fall back to the real source. Without a
	// checksum there is nothing to check a stranger's bytes against, so the
	// project's own site is the only place isoshelf will look.
	Nearer func(filename, sha256 string) []string
}

// Result describes what an update did.
type Result struct {
	// File is the new file's name in the target.
	File    string
	Version string
	Size    int64
	SHA256  string
	// Verified is false when the project publishes no checksum.
	Verified bool
	URL      string
	// Removed lists the old files that were moved aside or deleted.
	Removed []string
	// Kept lists old files left in place.
	Kept []string
}

// ErrNothingToDownload means the entry has no download information: it is
// manual or check-only.
var ErrNothingToDownload = errors.New("this image has no download source yet")

// ErrSameName is returned when the new file would take the place of one
// already in the folder, because this image's filename never carries a
// version. Only the user can say what should happen to the old copy, so the
// download stops and asks rather than guessing.
var ErrSameName = errors.New("the new file would take the old one's place")

// Run downloads the entry's newest file, verifies it, places it in the
// target, and then keeps or removes the old files as Removal says.
func Run(ctx context.Context, opts Options) (*Result, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Entry == nil || opts.State == nil || opts.Client == nil || opts.Fetcher == nil {
		return nil, errors.New("update: missing entry, state or clients")
	}
	if opts.Version != "" {
		return nil, errors.New("installing a chosen version isn't supported yet")
	}

	rel, err := source.Latest(ctx, opts.Client, opts.Entry)
	if errors.Is(err, source.ErrManual) {
		return nil, ErrNothingToDownload
	}
	if err != nil {
		return nil, err
	}
	artifact, err := resolve.Resolve(ctx, opts.Client, opts.Entry, rel)
	if errors.Is(err, resolve.ErrNoArtifact) {
		return nil, ErrNothingToDownload
	}
	if err != nil {
		return nil, err
	}

	progress := opts.Progress
	if progress == nil {
		progress = func(fetch.Progress) {}
	}
	// Images whose filename never changes land on top of the old file, so
	// something has to happen to it first. It only happens once the new file
	// is downloaded and verified.
	//
	// Keeping both is possible now: the new download carries its version in
	// its name (see keepboth.go) and the file already there isn't touched, so
	// there is no clash left to resolve. Only an update with no answer at all
	// is refused, because that is the page asking.
	sameName := slices.Contains(opts.Old, artifact.Filename)
	// A pinned file is kept whatever the answer, so one the new file would
	// land on means keeping both: the new one takes a versioned name.
	pinnedClash := slices.Contains(opts.Pinned, artifact.Filename)
	if sameName && opts.Removal == "" && !pinnedClash {
		return nil, fmt.Errorf("%w: %s always has the same filename, so the new file would land on top of the one you have", ErrSameName, artifact.Filename)
	}
	// placeAs is the name the new file actually gets, which is the usual one
	// unless both copies are being kept.
	placeAs := artifact.Filename
	keepBoth := sameName && (opts.Removal == Keep || pinnedClash)
	if keepBoth {
		placeAs = KeepBothName(opts.Target, artifact.Filename, rel.Version, opts.Now())
		sameName = false
	}
	// A download nothing can check never replaces anything on its own: the old
	// files stay until the user has looked at the new one. The built-in
	// catalog only lists downloads with a published checksum, but a catalog
	// of the user's own, or a GitHub release from before GitHub published
	// digests, can lack one. An old file with the same name can't stay where
	// it is, so it is archived, which can be undone, and never deleted.
	unverified := artifact.Checksum == nil
	beforeRemoval := opts.Removal
	if unverified {
		beforeRemoval = MoveAside
	}
	// Somewhere closer than the internet, if there is one and if these bytes
	// can be checked when they arrive.
	urls := artifact.URLs
	var near []string
	if opts.Nearer != nil && artifact.Checksum != nil && artifact.Checksum.Algorithm == verify.SHA256 {
		if near = opts.Nearer(artifact.Filename, artifact.Checksum.Hex); len(near) > 0 {
			urls = append(append([]string{}, near...), urls...)
		}
	}

	var replaced string
	request := fetch.Request{
		URLs:     urls,
		Filename: placeAs,
		Dir:      opts.Target,
		Size:     artifact.Size,
		Checksum: artifact.Checksum,
		Replace:  sameName,
	}
	if sameName {
		request.BeforePlace = func() error {
			if missing(opts.Target, artifact.Filename) {
				return nil // removed by hand while the new one downloaded
			}
			if err := removeFile(opts.Target, artifact.Filename, beforeRemoval, opts.State, opts.Now()); err != nil {
				return err
			}
			opts.State.Archived(artifact.Filename, state.GoneReplaced, opts.Now())
			replaced = artifact.Filename
			return nil
		}
	}
	downloaded, err := opts.Fetcher.Download(ctx, request, progress)
	if err != nil {
		return nil, err
	}

	version := rel.Version
	if v, ok := opts.Entry.MatchName(artifact.Filename); ok && v != "" {
		version = v
	}
	result := &Result{
		File: placeAs, Version: version, Size: downloaded.Size,
		SHA256: downloaded.SHA256, Verified: downloaded.Verified, URL: downloaded.URL,
	}
	if replaced != "" {
		result.Removed = append(result.Removed, replaced)
	}
	if err := opts.State.Placed(opts.Target, placeAs, state.FileRecord{
		Entry:   opts.Entry.ID,
		Version: version,
		// A kept-both file carries its version in its name, so the catalog
		// won't recognize it by name; say outright what it is, or the next
		// scan would call the file it just downloaded an unknown one.
		Assigned:  keepBoth,
		SHA256:    downloaded.SHA256,
		HashedAt:  opts.Now().UTC(),
		SourceURL: downloaded.URL,
		PlacedAt:  opts.Now().UTC(),
		Origin:    originOf(downloaded, artifact, near, opts.Now()),
	}); err != nil {
		return result, err
	}

	for _, old := range opts.Old {
		if old == placeAs {
			continue // the new file took its place
		}
		if missing(opts.Target, old) {
			continue // removed by hand while the new one downloaded
		}
		if opts.Removal == Keep || opts.Removal == "" || unverified || slices.Contains(opts.Pinned, old) {
			result.Kept = append(result.Kept, old)
			continue
		}
		if err := removeFile(opts.Target, old, opts.Removal, opts.State, opts.Now()); err != nil {
			return result, fmt.Errorf("the new file is in place, but %s: %w", old, err)
		}
		result.Removed = append(result.Removed, old)
	}
	return result, nil
}

// missing reports whether a file has already left the folder: the user can
// remove or archive it while its replacement is still downloading.
func missing(target, rel string) bool {
	local := filepath.FromSlash(rel)
	if !filepath.IsLocal(local) {
		return false
	}
	_, err := os.Lstat(filepath.Join(target, local))
	return errors.Is(err, fs.ErrNotExist)
}

// originOf says where a download came from and what proved it: the project's
// site or another isoshelf, and - when the bytes matched - the checksum the
// project publishes. A copy from another isoshelf that matched that checksum
// is as proven as a download; the checksum never came from the copy.
func originOf(got *fetch.Result, artifact *resolve.Artifact, near []string, now time.Time) state.Origin {
	o := state.Origin{How: state.OriginDownload, From: got.URL, At: now.UTC()}
	if slices.Contains(near, got.URL) {
		o.How = state.OriginCopy
	}
	if got.Verified {
		o.Checked, o.CheckedAt = artifact.ChecksumURL, now.UTC()
	}
	return o
}
