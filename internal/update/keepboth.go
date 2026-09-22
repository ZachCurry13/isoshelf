package update

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Keeping both copies of an image whose filename never changes.
//
// Some projects publish under one name forever - netboot.xyz.iso, and the
// rest of the fixed-name images. Updating one used to mean choosing between
// the old file and the new: there was nowhere for the second one to go.
//
// There is: the new download carries its version in its name
// (netboot.xyz-2.0.87.iso), and the file already on the drive is not touched
// at all. That is the point of doing it this way round. Nothing that exists
// is renamed, so nothing that points at a file by name can break - and the
// new file says on its face which version it is, which is the question
// someone with two copies is actually asking.
//
// The new file's name no longer matches the catalog's, so its record is
// marked as one the user assigned, with the entry and version already known
// from the download. Without that the next scan would call it unknown.

// KeepBothName is the name the new download takes so that both copies can
// live in the folder: the image's usual name with its version worked in, or
// the day it arrived when the project doesn't say what the version is.
func KeepBothName(target, filename, version string, now time.Time) string {
	dir, base := path.Split(filepath.ToSlash(filename))
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	label := safeLabel(version)
	if label == "" {
		label = now.UTC().Format("2006-01-02")
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
