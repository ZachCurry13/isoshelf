package state

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// RecordScan updates the file records from a scan and adds the scan to the
// history. A file that changed or disappeared loses its record, including its
// hash and any assignment. Files the scan couldn't read keep their records.
func (s *State) RecordScan(res *scan.Result, now time.Time) {
	files := make(map[string]FileRecord, len(res.Files))
	for path, rec := range s.Files {
		for _, p := range res.Problems {
			if path == p.Path || strings.HasPrefix(path, p.Path+"/") {
				files[path] = rec
			}
		}
	}

	// Anything that isn't here any more is remembered, so it can be found or
	// downloaded again later.
	for path, rec := range s.Files {
		if _, kept := files[path]; !kept && !inScan(res, path) {
			s.archive(path, rec, GoneVanished, now)
		}
	}

	found := map[string]bool{}
	unrecognized := 0
	// On the very first scan every file is new to isoshelf, which says
	// nothing about when it arrived; only later scans can tell.
	seenBefore := len(s.History) > 0
	for _, f := range res.Files {
		s.forgetArchived(f.Path)
		rec, ok := s.Files[f.Path]
		if !ok || !rec.current(f) {
			rec = FileRecord{Size: f.Size, ModTime: f.ModTime}
			if seenBefore {
				rec.FirstSeen = now.UTC()
			}
		}
		if !rec.Assigned {
			rec.Entry, rec.Version = "", ""
			if len(f.Matches) == 1 {
				rec.Entry, rec.Version = f.Matches[0].Entry.ID, f.Matches[0].Version
			}
		}
		if rec.Entry == "" {
			unrecognized++
		} else {
			found[rec.Entry] = true
		}
		files[f.Path] = rec
	}
	s.Files = files

	// An image somebody stopped expecting is expected again once it is back:
	// they said they had removed it, and now it is here.
	for id := range found {
		if t := s.Track(id); t.NotExpected {
			t.NotExpected = false
			s.SetTrack(id, t)
		}
	}

	s.History = append(s.History, ScanRecord{
		Time:         now.UTC(),
		Entries:      slices.Sorted(maps.Keys(found)),
		Unrecognized: unrecognized,
	})
	if extra := len(s.History) - maxHistory; extra > 0 {
		s.History = slices.Clone(s.History[extra:])
	}
}

// inScan reports whether the scan listed this path.
func inScan(res *scan.Result, path string) bool {
	return slices.ContainsFunc(res.Files, func(f scan.File) bool { return f.Path == path })
}

// Assign records that the file at path belongs to an entry the catalog didn't
// match by name, such as a renamed download. The caller checks that the entry
// exists. The assignment lasts until the file changes.
func (s *State) Assign(path, entry, version string) error {
	rec, ok := s.Files[path]
	if !ok {
		return fmt.Errorf("%s is not in the last scan", path)
	}
	rec.Entry, rec.Version, rec.Assigned = entry, version, true
	s.Files[path] = rec
	return nil
}

// Unassign undoes an assignment, so the file goes back to being whatever the
// catalog makes of its name.
func (s *State) Unassign(path string) error {
	rec, ok := s.Files[path]
	if !ok {
		return fmt.Errorf("%s is not in the last scan", path)
	}
	rec.Entry, rec.Version, rec.Assigned = "", "", false
	s.Files[path] = rec
	return nil
}

// Track returns the settings for an entry.
func (s *State) Track(entry string) Track {
	return s.Tracks[entry]
}

// SetTrack stores the settings for an entry.
func (s *State) SetTrack(entry string, t Track) {
	if t == (Track{}) {
		delete(s.Tracks, entry)
		return
	}
	s.Tracks[entry] = t
}

// Placed records a file isoshelf downloaded into the target. Its size and
// modification time come from the file itself, so later scans recognize it
// without rehashing.
func (s *State) Placed(target, rel string, rec FileRecord) error {
	info, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	rec.Size, rec.ModTime = info.Size(), info.ModTime()
	s.Files[rel] = rec
	return nil
}
