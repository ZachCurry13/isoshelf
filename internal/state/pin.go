package state

import (
	"fmt"
	"maps"
	"slices"
)

// Pinning a file keeps that exact file whatever updates come (v0.7.0, #54).
// It replaced the per-image choice between replacing, archiving and keeping
// the old copy: one answer in Settings covers what happens to old files, and
// a pin is the exception - "not this one".

// Pin keeps or releases the file at path.
func (s *State) Pin(path string, on bool) error {
	rec, ok := s.Files[path]
	if !ok {
		return fmt.Errorf("%s is not in the last scan", path)
	}
	rec.Pinned = on
	s.Files[path] = rec
	return nil
}

// Pinned lists the pinned files, in order.
func (s *State) Pinned() []string {
	var out []string
	for _, path := range slices.Sorted(maps.Keys(s.Files)) {
		if s.Files[path].Pinned {
			out = append(out, path)
		}
	}
	return out
}

// Migrated says what MigrateChoices did.
type Migrated struct {
	// Pinned counts the images whose files were pinned because they were set
	// to keep both copies.
	Pinned int
	// Following counts the images that had an answer of their own, different
	// from the one in Settings, and now follow Settings.
	Following int
}

// MigrateChoices turns the per-image answers from before v0.7.0 into pins.
// An image set to keep both has its files pinned; every image's own answer
// is then dropped, so it follows fallback, the answer in Settings. It changes
// nothing the second time, so it can run on every scan.
func (s *State) MigrateChoices(fallback string) Migrated {
	var out Migrated
	for _, entry := range slices.Sorted(maps.Keys(s.Tracks)) {
		t := s.Tracks[entry]
		if t.OldFiles == "" && !t.KeepOld {
			continue
		}
		switch choice := t.Choice(); {
		case choice == "keep":
			for path, rec := range s.Files {
				if rec.Entry == entry && !rec.Pinned {
					rec.Pinned = true
					s.Files[path] = rec
				}
			}
			out.Pinned++
		case choice != fallback:
			out.Following++
		}
		t.OldFiles, t.KeepOld = "", false
		s.SetTrack(entry, t)
	}
	return out
}
