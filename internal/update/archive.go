package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// RemovedDir is where files moved aside wait inside the target's .isoshelf
// folder, until the user empties it.
const RemovedDir = "removed"

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
