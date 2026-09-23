package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// recordsDir is the folder for records kept away from their own folder.
const recordsDir = "records"

// Home is where a folder's records are kept.
//
// The empty Home is the default and the one to prefer: each folder keeps its
// own records in its own .isoshelf folder, so a drive carries everything
// isoshelf worked out about it to whatever computer it is plugged into next.
//
// A Home that names a folder keeps every folder's records together there
// instead, one file each, for a drive isoshelf should not be writing to. The
// cost is that the records are then found by the folder's path: the same
// drive at a different letter or mount point starts with nothing. The copy in
// the app's config folder (see SaveMirror) still holds the history.
//
// Only the records move. The archive of removed images and the part-finished
// downloads stay in the folder, because they are the folder's own files -
// archiving is a rename, and a rename across disks is a copy of every byte.
type Home string

// File is the file holding target's records. target should be absolute; File
// makes it so if it isn't, since the name of a records file kept away from
// its folder is worked out from that path.
func (h Home) File(target string) (string, error) {
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if h == "" {
		return filepath.Join(abs, DirName, fileName), nil
	}
	return filepath.Join(string(h), recordsDir, nameFor(abs)+".json"), nil
}

// Dir is the folder that File writes into: where to look, and what to name in
// a complaint when saving fails.
func (h Home) Dir(target string) (string, error) {
	name, err := h.File(target)
	if err != nil {
		return "", err
	}
	return filepath.Dir(name), nil
}

// nameFor turns a folder's path into a file name. A hash rather than the path
// itself, because a path holds separators, drive letters and characters no
// file name may have - and because it is a fixed length, whatever the path.
// The case of the path is kept: two folders differing only in case are the
// same folder on Windows and different ones on Linux, and treating them as
// one would hand a folder somebody else's records.
func nameFor(abs string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(abs)))
	return hex.EncodeToString(sum[:8])
}

// CleanHome checks a folder the user has named for records. It returns the
// cleaned path, or an error saying what is wrong with it in the words the
// page shows.
func CleanHome(dir string) (Home, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", errors.New("Name a folder to keep the records in.")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dir) {
		return "", errors.New("Give the whole path to the folder, starting from the top.")
	}
	return Home(abs), nil
}

// Moved says what Move did, or with Plan what it would do, so the page can say
// it too - before, so the user can say no, and after, so they know.
type Moved struct {
	// From and To are the files. Moved is false when nothing was moved.
	From, To string
	Moved    bool
	// Kept is set when there were already records for this folder where they
	// were being moved to. Those are the ones isoshelf will use, and the old
	// ones are left where they are rather than thrown away - nothing isoshelf
	// wrote is deleted unless the user chose it.
	Kept bool
	// FromSaved and ToSaved are when each file was last written, and zero
	// where there is no file. With both set, they are how somebody tells
	// which records would win before agreeing to it.
	FromSaved, ToSaved time.Time
}

// Plan says what Move would do, and does none of it. The page asks before a
// move, and its question has to describe the move that then happens - so
// both come from here rather than from two accounts that must agree.
func Plan(from, to Home, target string) (Moved, error) {
	src, err := from.File(target)
	if err != nil {
		return Moved{}, err
	}
	dst, err := to.File(target)
	if err != nil {
		return Moved{}, err
	}
	out := Moved{From: src, To: dst}
	if src == dst {
		return out, nil
	}
	if out.FromSaved, err = savedAt(src); err != nil {
		return out, err
	}
	if out.ToSaved, err = savedAt(dst); err != nil {
		return out, err
	}
	switch {
	case out.FromSaved.IsZero():
		// Nothing learned about this folder yet, or nothing here: if the
		// destination has records, those are simply the ones read from now on.
	case !out.ToSaved.IsZero():
		out.Kept = true
	default:
		out.Moved = true
	}
	return out, nil
}

// savedAt is when a records file was last written, or zero if there is none.
func savedAt(name string) (time.Time, error) {
	info, err := os.Stat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// Move takes target's records from one place to the other, which is what
// changing where a folder's records live has to do: the alternative is a
// folder that appears to have been forgotten - no history, no stars, every
// image unidentified again.
//
// A folder with no records yet is not an error; there is simply nothing to
// move. Neither is a destination that already has records for this folder: it
// keeps them, and the old ones stay where they are.
func Move(from, to Home, target string) (Moved, error) {
	out, err := Plan(from, to, target)
	if err != nil || !out.Moved {
		return out, err
	}
	out.Moved = false // until it has happened
	data, err := os.ReadFile(out.From)
	if err != nil {
		return out, err
	}
	if err := os.MkdirAll(filepath.Dir(out.To), 0o755); err != nil {
		return out, err
	}
	if err := writeFileAtomic(out.To, data); err != nil {
		return out, err
	}
	// Only once the new one is safely written.
	if err := os.Remove(out.From); err != nil {
		return out, err
	}
	out.Moved = true
	// Taking the records out of a folder can leave isoshelf's folder there
	// with nothing in it, on a drive somebody has just asked isoshelf to stop
	// writing to. Remove it if it is empty - which os.Remove does for us: it
	// refuses a folder that still holds the archive or a part-finished
	// download, and those are the user's files, not isoshelf's housekeeping.
	if from == "" {
		os.Remove(filepath.Dir(out.From))
	}
	return out, nil
}
