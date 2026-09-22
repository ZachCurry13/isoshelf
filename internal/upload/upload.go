// Package upload puts a file from the user's own computer into the folder:
// dragged onto the page, or picked with the file chooser.
//
// It is the one way an image arrives without isoshelf having fetched it, so
// the rules it keeps are the ones the rest of isoshelf keeps. Only an image
// file, decided the same way a scan decides what to list. Only into the
// folder the user chose, never isoshelf's own folder and never out of it by
// way of a name with a path in it. Never on top of a file already there
// unless the user has said what should happen to that file, and then the old
// one is moved aside rather than deleted unless deleting is what they chose.
// And nothing is put in place until the whole file has arrived: it is written
// to .isoshelf/incoming first, which scans skip, so a half-arrived file is
// never mistaken for an image and a connection that drops leaves the folder
// as it was.
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// incomingDir is where a file is written while it arrives, inside isoshelf's
// own folder.
const incomingDir = "incoming"

var (
	// ErrNotAnImage means the file isn't one this folder would list.
	ErrNotAnImage = errors.New("that isn't an image file")
	// ErrExists means a file of that name is already there and the caller
	// hasn't said what should happen to it.
	ErrExists = errors.New("a file of that name is already here")
	// ErrNoRoom means the folder hasn't the space for it.
	ErrNoRoom = errors.New("there isn't room for it")
)

// Options describe one upload.
type Options struct {
	// Target is the folder the file goes into.
	Target string
	// Name is the file's name as the browser gave it. Any path in front of it
	// is dropped: a file only ever lands in the top of the folder.
	Name string
	// Profile decides what counts as an image here, the same way a scan does.
	Profile scan.Profile
	// Size is what the browser said the file is, or 0 when it didn't say. It
	// is used to refuse an upload that wouldn't fit before it starts, never
	// to decide when the file has finished arriving.
	Size int64
	// Room is the space where the images are kept. A zero value means the
	// filesystem didn't say, and the upload goes ahead.
	Room space.Usage
	// Replace says what happens to a file of the same name already there:
	// update.MoveAside or update.DeleteNow. Empty refuses instead, so that
	// nothing is ever overwritten without the user having been asked.
	Replace update.Removal
	// State is the folder's records, which the new file is written into.
	State *state.State
	// Now defaults to time.Now.
	Now func() time.Time
}

// Result is what arrived.
type Result struct {
	// Name is the file's name in the folder.
	Name string
	// Size is how many bytes were actually written.
	Size int64
	// Replaced is the file this one took the place of, if any.
	Replaced string
}

// Place writes everything src has into the folder as opts.Name.
//
// The file is fully written and flushed to the disk before anything in the
// folder changes, so a failed or abandoned upload can't cost the user the
// file they already had.
func Place(ctx context.Context, src io.Reader, opts Options) (*Result, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	name, err := cleanName(opts.Name)
	if err != nil {
		return nil, err
	}
	if !opts.Profile.Lists(name) {
		return nil, fmt.Errorf("%w: %s", ErrNotAnImage, name)
	}
	if !opts.Room.Fits(opts.Size) {
		return nil, fmt.Errorf("%w: %s needs more room than this folder has left", ErrNoRoom, name)
	}

	final := filepath.Join(opts.Target, name)
	_, statErr := os.Lstat(final)
	switch {
	case statErr == nil && opts.Replace != update.MoveAside && opts.Replace != update.DeleteNow:
		return nil, fmt.Errorf("%w: %s", ErrExists, name)
	case statErr != nil && !errors.Is(statErr, os.ErrNotExist):
		return nil, statErr
	}
	replacing := statErr == nil

	written, tmp, err := receive(ctx, src, opts.Target)
	if err != nil {
		return nil, err
	}
	// From here the file is on the disk in full. Clean it up unless it makes
	// it all the way into place.
	placed := false
	defer func() {
		if !placed {
			os.Remove(tmp)
		}
	}()

	if replacing {
		// The old file is moved aside (or deleted, if that is what was asked)
		// only now, with the new one safely written.
		if err := update.Displace(opts.Target, name, opts.Replace, opts.State, now()); err != nil {
			return nil, err
		}
	}
	if err := os.Rename(tmp, final); err != nil {
		return nil, err
	}
	placed = true

	res := &Result{Name: name, Size: written}
	if replacing {
		res.Replaced = name
	}
	if opts.State != nil {
		// No entry or version: the scan that follows matches it against the
		// catalog, exactly as it would a file copied in with Explorer. What
		// isoshelf does know is when it arrived.
		if err := opts.State.Placed(opts.Target, name, state.FileRecord{PlacedAt: now().UTC()}); err != nil {
			return res, err
		}
	}
	return res, nil
}

// receive writes src to a file in .isoshelf/incoming and returns its path.
// The file is flushed to the disk before it is handed back, because the next
// thing the caller does is move a file the user already had.
func receive(ctx context.Context, src io.Reader, target string) (int64, string, error) {
	dir := filepath.Join(target, state.DirName, incomingDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, "", err
	}
	f, err := os.CreateTemp(dir, "upload-*.part")
	if err != nil {
		return 0, "", err
	}
	name := f.Name()
	written, err := io.Copy(f, src)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return 0, "", err
	}
	return written, name, nil
}

// cleanName reduces what the browser sent to a plain filename in the top of
// the folder. A browser sends only the basename, but it is what a request
// says it is, so this doesn't take that on trust.
func cleanName(name string) (string, error) {
	n := path.Base(strings.TrimSpace(filepath.ToSlash(name)))
	switch {
	case n == "" || n == "." || n == ".." || n == "/":
		return "", errors.New("that file has no name")
	case strings.ContainsRune(n, 0) || strings.ContainsRune(n, '\\'):
		// A backslash is a separator on Windows, so a name holding one could
		// reach out of the folder there even though path.Base left it alone.
		return "", fmt.Errorf("%q is not a name a file can have", name)
	case strings.EqualFold(n, state.DirName):
		return "", fmt.Errorf("%s is isoshelf's own folder", n)
	}
	return n, nil
}
