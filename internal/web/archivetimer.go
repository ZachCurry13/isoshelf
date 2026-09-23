package web

import (
	"path"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// Emptying the archive on a timer.
//
// This is the one thing isoshelf does by itself that throws away something
// somebody might still want. The archive is the undo for every removal and
// every file an update replaced, so the rules here are deliberately timid:
//
//   - Off unless a number is chosen. Never a default, never turned on by an
//     upgrade. The hard rule is that nothing is deleted unless the user chose
//     it, and choosing the number is that choice.
//   - Each file is judged by its own age, from when it was archived, not by
//     one sweep of the folder. A file archived this morning survives a sweep
//     that clears one from three months ago.
//   - A file isoshelf has no archive record for is never deleted. Something
//     in .isoshelf/removed that isoshelf didn't put there, or whose record
//     has been lost, has no age isoshelf can judge - so it stays.
//   - Settings says what the next sweep would take, before it takes it.

// archiveChoices are the numbers of days offered, with 0 meaning never.
var archiveChoices = []int{0, 7, 30, 90}

// cleanArchiveAfter returns a chosen number of days, or 0 for never.
func cleanArchiveAfter(days int) int {
	for _, ok := range archiveChoices {
		if days == ok {
			return days
		}
	}
	return 0
}

// staleArchived lists the files waiting in the archive that are older than
// the chosen age, with the space they use. It reads and decides nothing else,
// so Settings can say what a sweep would do and the sweep itself can use the
// same answer.
//
// st may be nil, and then nothing is stale: without the records there is no
// date to judge any file by.
func staleArchived(target string, st *state.State, days int, now time.Time) (names []string, bytes int64) {
	if target == "" || st == nil || days <= 0 {
		return nil, 0
	}
	waiting, _, err := update.Removed(target)
	if err != nil || len(waiting) == 0 {
		return nil, 0
	}
	// When each file left the folder, by the name it has in the archive.
	goneAt := map[string]time.Time{}
	for _, past := range st.Archive() {
		if past.Gone != state.GoneMovedAside && past.Gone != state.GoneReplaced {
			continue
		}
		base := path.Base(past.Path)
		// The most recent note about a name is the one that counts: a file
		// archived, restored and archived again is as old as the last time.
		if at, seen := goneAt[base]; !seen || past.GoneAt.After(at) {
			goneAt[base] = past.GoneAt
		}
	}

	older := now.Add(-time.Duration(days) * 24 * time.Hour)
	for _, name := range waiting {
		at, known := goneAt[name]
		// No record, no age, no deletion.
		if !known || at.IsZero() || !at.Before(older) {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, 0
	}
	bytes = update.RemovedBytes(target, names)
	return names, bytes
}

// emptyArchiveIfDue deletes the archived files that have waited longer than
// the chosen number of days. It runs on the same tick as the update schedule.
func (s *Server) emptyArchiveIfDue() {
	saved := s.loadSettings()
	days := cleanArchiveAfter(saved.ArchiveAfter)
	if days == 0 {
		return
	}

	s.mu.Lock()
	target, st := s.target, s.st
	// A folder being scanned, downloaded into or uploaded to is left alone:
	// its archive is about to change, and the next tick is minutes away.
	busy := target == "" || s.scanning != nil || s.downloading != nil || len(s.queue) > 0 || s.uploads > 0
	s.mu.Unlock()
	if busy {
		return
	}

	names, _ := staleArchived(target, st, days, s.cfg.Now())
	if len(names) == 0 {
		return
	}
	deleted, err := update.RemoveArchived(target, names)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.warnings = append(s.warnings,
			"Couldn't empty part of the archive on its timer: "+err.Error())
	}
	if deleted > 0 {
		// Said afterwards as well as before, because this happened while
		// nobody was watching.
		s.autoNote = plural(deleted, "archived file") + " older than " +
			plural(days, "day") + " " + wasOrWere(deleted) + " deleted."
	}
}

func wasOrWere(n int) string {
	if n == 1 {
		return "was"
	}
	return "were"
}
