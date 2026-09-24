package appupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Swapped is what putting an update in place changed, written down before
// the new program starts, so that whichever program runs next - the new one,
// or the old one if the new one won't start - knows what to undo.
type Swapped struct {
	// From and To are the versions.
	From, To string
	// Run is the program to start now: the running one's new place.
	Run   string
	Moves []Move
}

// Move is one program replaced.
type Move struct {
	// Was is where the old program lived, and Backup where it waits now.
	Was, Backup string
	// Now is where the new program is.
	Now string
}

// manifestName is the record of a swap, in the staging folder.
const manifestName = "swapped.json"

// Swap puts a staged update in place. Each old program is renamed aside,
// never deleted, and the new one renamed into its place; Windows lets a
// running program be renamed, though not replaced. If any step fails,
// everything done so far is put back and nothing has changed.
func (s *Staged) Swap(from string) (*Swapped, error) {
	out := &Swapped{From: from, To: s.Version}
	for _, f := range s.Files {
		moves, err := swapOne(f)
		out.Moves = append(out.Moves, moves...)
		if err != nil {
			out.Restore()
			return nil, err
		}
		if f.Running {
			out.Run = f.To
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(s.Dir, manifestName), data, 0o644)
	}
	if err != nil {
		out.Restore()
		return nil, fmt.Errorf("couldn't write down what the update changed, so it was undone: %w", err)
	}
	return out, nil
}

// swapOne replaces one program. A program keeping its name has one old file
// to set aside; one taking the plain name may find another file already
// there, and that is set aside too rather than overwritten.
func swapOne(f StagedFile) ([]Move, error) {
	var moves []Move
	for _, old := range []string{f.Path, f.To} {
		if len(moves) > 0 && old == moves[0].Was {
			continue
		}
		if _, err := os.Stat(old); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		backup := backupName(old)
		os.Remove(backup) // a backup from an earlier update, already undone or confirmed
		if err := os.Rename(old, backup); err != nil {
			return moves, fmt.Errorf("couldn't set %s aside: %w", filepath.Base(old), err)
		}
		moves = append(moves, Move{Was: old, Backup: backup})
	}
	if err := os.Rename(f.New, f.To); err != nil {
		return moves, fmt.Errorf("couldn't put the new %s in place: %w", filepath.Base(f.To), err)
	}
	if len(moves) == 0 {
		moves = append(moves, Move{})
	}
	moves[0].Now = f.To
	return moves, nil
}

// Restore undoes a swap: each new program is taken out and each old one put
// back where it was. It carries on past a failure so as much as possible is
// put back, and says what it couldn't.
//
// Every new program comes out before any old one goes back: a program taking
// the plain name sets aside a file that was already there, and putting that
// back first and then removing "the new one" would remove the wrong file.
func (w *Swapped) Restore() error {
	var errs []error
	for _, m := range w.Moves {
		if m.Now != "" {
			if err := os.Remove(m.Now); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	for i := len(w.Moves) - 1; i >= 0; i-- {
		m := w.Moves[i]
		if m.Backup != "" {
			if err := os.Rename(m.Backup, m.Was); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// Confirm keeps a swap: the new programs have started, so the old ones and
// the staging folder go. On Windows the old program may still be finishing
// for a moment, and can't be removed until it has; so this tries for a while.
func (w *Swapped) Confirm(dir string) {
	for try := 0; try < 20; try++ {
		left := 0
		for _, m := range w.Moves {
			if m.Backup == "" {
				continue
			}
			if err := os.Remove(m.Backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
				left++
			}
		}
		if left == 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	os.RemoveAll(filepath.Join(dir, StageDir))
}

// LoadSwapped reads the record Swap wrote.
func LoadSwapped(stageDir string) (*Swapped, error) {
	data, err := os.ReadFile(filepath.Join(stageDir, manifestName))
	if err != nil {
		return nil, err
	}
	var w Swapped
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, err
	}
	return &w, nil
}
