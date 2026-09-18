// Package update puts new images in place and removes unwanted ones. It is
// the only package that deletes anything, and it follows isoshelf's rules:
// only image files inside the target, never without being asked, and never a
// file that failed its checksum.
package update

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/state"
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
	// Version, when set, is the version to install instead of the newest.
	// (Reserved for "install an older version"; not used yet.)
	Version  string
	Progress func(fetch.Progress)
	Now      func() time.Time
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
	// keeping both is impossible and the old one has to go first. It only
	// moves once the new file is downloaded and verified.
	sameName := slices.Contains(opts.Old, artifact.Filename)
	if sameName && (opts.Removal == Keep || opts.Removal == "") {
		return nil, fmt.Errorf("%s always has the same filename, so the new file would take its place; choose to move the old one aside or delete it", artifact.Filename)
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
	var replaced string
	request := fetch.Request{
		URLs:     artifact.URLs,
		Filename: artifact.Filename,
		Dir:      opts.Target,
		Size:     artifact.Size,
		Checksum: artifact.Checksum,
		Replace:  sameName,
	}
	if sameName {
		request.BeforePlace = func() error {
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
		File: artifact.Filename, Version: version, Size: downloaded.Size,
		SHA256: downloaded.SHA256, Verified: downloaded.Verified, URL: downloaded.URL,
	}
	if replaced != "" {
		result.Removed = append(result.Removed, replaced)
	}
	if err := opts.State.Placed(opts.Target, artifact.Filename, state.FileRecord{
		Entry:     opts.Entry.ID,
		Version:   version,
		SHA256:    downloaded.SHA256,
		HashedAt:  opts.Now().UTC(),
		SourceURL: downloaded.URL,
		PlacedAt:  opts.Now().UTC(),
	}); err != nil {
		return result, err
	}

	for _, old := range opts.Old {
		if old == artifact.Filename {
			continue // the new file took its place
		}
		if opts.Removal == Keep || opts.Removal == "" || unverified {
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
