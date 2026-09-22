package update

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Keeping both copies of an image whose filename never changes.
//
// Some projects publish under one name forever - netboot.xyz.iso, and the
// rest of the fixed-name images. Updating one used to mean choosing between
// the old file and the new: there was nowhere for the second one to go.
//
// There is: the old file steps aside under a name of its own, and the new
// download takes the name it has always had. That way round on purpose.
// Anything pointing at the unchanging name - a Proxmox VM, a script, a
// shortcut - keeps working and quietly gets the newer image, which is what
// it wanted. On a Ventoy drive both simply appear in the boot menu, which is
// the whole point of keeping both.
//
// The renamed file no longer matches the catalog by name, so its record is
// marked as one the user assigned. Without that the next scan would call a
// file it has known for months an unknown file.

// KeepBoth moves the file already in the folder out of the way of a new one
// of the same name and returns the name it now has. The new file is not
// written here: fetch places it once this returns, so a failure leaves the
// folder as it was.
func KeepBoth(target, rel string, st *state.State, now time.Time) (string, error) {
	full, err := insideTarget(target, rel)
	if err != nil {
		return "", err
	}
	var rec state.FileRecord
	if st != nil {
		rec = st.Files[rel]
	}
	aside := asideName(target, rel, rec, now)
	if err := os.Rename(full, filepath.Join(target, filepath.FromSlash(aside))); err != nil {
		return "", fmt.Errorf("couldn't move %s aside to keep both copies: %w", rel, err)
	}
	if st != nil {
		// Same image, new name. Assigned keeps the next scan from deciding it
		// has never seen this file before.
		if rec.Entry != "" {
			rec.Assigned = true
		}
		st.Files[aside] = rec
		delete(st.Files, rel)
	}
	return aside, nil
}

// asideName is what the older copy is called: its version when isoshelf knows
// it, and otherwise the day it arrived, which is the question someone is
// actually asking when they look at two of the same image.
func asideName(target, rel string, rec state.FileRecord, now time.Time) string {
	dir, base := path.Split(filepath.ToSlash(rel))
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	label := safeLabel(rec.Version)
	if label == "" {
		when := rec.PlacedAt
		if when.IsZero() {
			when = rec.FirstSeen
		}
		if when.IsZero() {
			when = rec.ModTime
		}
		if when.IsZero() {
			when = now
		}
		label = when.UTC().Format("2006-01-02")
	}

	candidate := dir + stem + "-" + label + ext
	for i := 2; taken(target, candidate); i++ {
		candidate = fmt.Sprintf("%s%s-%s-%d%s", dir, stem, label, i, ext)
	}
	return candidate
}

// safeLabel keeps a version usable as part of a filename. Versions are
// normally plain (24.04, 2.0.72), but a catalog of the user's own can hold
// anything, and none of it may turn into a path.
func safeLabel(v string) string {
	v = strings.TrimSpace(v)
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_', r == '+':
			return r
		}
		return -1
	}, v)
	if len(clean) > 40 {
		clean = clean[:40]
	}
	return strings.Trim(clean, ".-_+")
}

func taken(target, rel string) bool {
	_, err := os.Lstat(filepath.Join(target, filepath.FromSlash(rel)))
	return err == nil
}
