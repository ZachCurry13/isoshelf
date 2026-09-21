package update

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/sniff"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

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
// Displace moves the file a new one is about to take the place of out of the
// way, and records where it went. Keep is not an answer here: the new file
// needs the name, so the old one has to go somewhere, and the caller must
// have asked which. Unlike Remove it asks no questions about what the file
// is - the user is replacing it deliberately, by name.
func Displace(target, rel string, how Removal, st *state.State, now time.Time) error {
	if how != MoveAside && how != DeleteNow {
		return fmt.Errorf("say what should happen to %s first", rel)
	}
	return removeFile(target, rel, how, st, now)
}

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
