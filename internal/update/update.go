// Package update puts new images in place and removes unwanted ones. It is
// the only package that deletes anything, and it follows isoshelf's rules:
// only image files inside the target, never without being asked, and never a
// file that failed its checksum.
package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/resolve"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/source"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// RemovedDir is where files moved aside wait inside the target's .isoshelf
// folder, until the user empties it.
const RemovedDir = "removed"

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

// Removable reports whether isoshelf may remove a file: it must be an image
// file inside the target, either one the catalog recognizes or one with an
// image extension or image content. Notes, archives and anything outside the
// target are refused.
func Removable(target, rel string, st *state.State, cat *catalog.Catalog) error {
	full, err := insideTarget(target, rel)
	if err != nil {
		return err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a file", rel)
	}
	name := path.Base(rel)
	if st != nil {
		if rec, ok := st.Files[rel]; ok && rec.Entry != "" {
			return nil // the catalog recognizes it
		}
	}
	if cat != nil && len(cat.Match(name)) > 0 {
		return nil
	}
	if slices.Contains(catalog.ImageExtensions, strings.ToLower(filepath.Ext(name))) {
		return nil
	}
	switch info, _ := sniff.File(full); info.Kind {
	case sniff.ISO, sniff.Disk, sniff.RawCD, sniff.WIM, sniff.VHD, sniff.VHDX:
		return nil
	}
	return fmt.Errorf("%s isn't an image file, so isoshelf won't remove it", rel)
}

// Remove moves aside or deletes files the user no longer wants. Every file is
// checked with Removable first, and its record is dropped from the state.
func Remove(target string, files []string, how Removal, st *state.State, cat *catalog.Catalog, now time.Time) ([]string, error) {
	if how != MoveAside && how != DeleteNow {
		return nil, fmt.Errorf("update: %q is not a way to remove files", how)
	}
	var removed []string
	var errs []error
	for _, rel := range files {
		if err := Removable(target, rel, st, cat); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := removeFile(target, rel, how, st, now); err != nil {
			errs = append(errs, err)
			continue
		}
		removed = append(removed, rel)
	}
	return removed, errors.Join(errs...)
}

// removeFile moves one file into .isoshelf/removed or deletes it.
func removeFile(target, rel string, how Removal, st *state.State, now time.Time) error {
	full, err := insideTarget(target, rel)
	if err != nil {
		return err
	}
	gone := state.GoneRemoved
	if how == MoveAside {
		gone = state.GoneMovedAside
		dir := filepath.Join(target, state.DirName, RemovedDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		name := path.Base(rel)
		aside := filepath.Join(dir, name)
		if _, err := os.Stat(aside); err == nil {
			aside = filepath.Join(dir, now.UTC().Format("20060102-150405")+"-"+name)
		}
		if err := os.Rename(full, aside); err != nil {
			return err
		}
	} else if err := os.Remove(full); err != nil {
		return err
	}
	if st != nil {
		st.Archived(rel, gone, now)
	}
	return nil
}

// Restore moves a file back from .isoshelf/removed into the folder.
func Restore(target, name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q is not a plain filename", name)
	}
	aside := filepath.Join(target, state.DirName, RemovedDir, name)
	if _, err := os.Stat(aside); err != nil {
		return fmt.Errorf("%s is no longer waiting in the removed folder", name)
	}
	back := filepath.Join(target, name)
	if _, err := os.Stat(back); err == nil {
		return fmt.Errorf("%s is already in the folder", name)
	}
	return os.Rename(aside, back)
}

// Removed lists the files waiting in .isoshelf/removed and the space they use.
func Removed(target string) (files []string, bytes int64, err error) {
	dir := filepath.Join(target, state.DirName, RemovedDir)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		files = append(files, e.Name())
		bytes += info.Size()
	}
	return files, bytes, nil
}

// EmptyRemoved deletes everything waiting in .isoshelf/removed, freeing the
// space. This is the one place where files are deleted without naming them
// one by one, and the user asks for it explicitly.
func EmptyRemoved(target string) (int, error) {
	dir := filepath.Join(target, state.DirName, RemovedDir)
	files, _, err := Removed(target)
	if err != nil {
		return 0, err
	}
	deleted := 0
	var errs []error
	for _, name := range files {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			errs = append(errs, err)
			continue
		}
		deleted++
	}
	return deleted, errors.Join(errs...)
}

// insideTarget turns a path relative to the target into a full path, and
// refuses anything that would leave the target or isoshelf's own folder.
func insideTarget(target, rel string) (string, error) {
	if rel == "" || path.IsAbs(rel) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("%q is not a path inside the folder", rel)
	}
	clean := path.Clean(filepath.ToSlash(rel))
	if clean == "." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("%q is not a path inside the folder", rel)
	}
	if first, _, _ := strings.Cut(clean, "/"); strings.EqualFold(first, state.DirName) {
		return "", fmt.Errorf("%q is one of isoshelf's own files", rel)
	}
	full := filepath.Join(target, filepath.FromSlash(clean))
	// Resolve links so a link can't point outside the folder.
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	realFull, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	if inside, err := filepath.Rel(realTarget, realFull); err != nil || strings.HasPrefix(inside, "..") {
		return "", fmt.Errorf("%q is outside the folder", rel)
	}
	return full, nil
}

// Files returns the paths of an entry's files in a scan, newest first.
func Files(res *scan.Result, st *state.State, entry string) []string {
	var out []string
	for _, f := range res.Files {
		if rec, ok := st.Files[f.Path]; ok && rec.Entry == entry {
			out = append(out, f.Path)
		}
	}
	return out
}
