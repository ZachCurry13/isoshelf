package web

import (
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// The folder's state file has more than one writer: a download that runs for
// an hour, and the user removing, archiving, identifying or starring things
// meanwhile. Nobody saves their whole copy over it. Each writer keeps a copy
// of the state from before its change, and saveStateLocked carries only the
// change over to the file as it is on disk now (state.Merge). A scan saves
// the same way: it reads the whole folder, but only what it learned goes onto
// the file as it is on disk then, so a download that finished meanwhile keeps
// its file's record.

// scanningLocked says why the folder's files can't be changed right now, or
// "" when they can: only while a scan or check runs. s.mu must be held.
func (s *Server) scanningLocked() string {
	if s.scanning != nil {
		return "Wait until the scan finishes, or stop it."
	}
	return ""
}

// saveStateLocked saves the changes made to s.st since base, a copy taken
// before them, onto the folder's state as it is on disk now. s.mu must be
// held.
func (s *Server) saveStateLocked(base *state.State) error {
	return saveMerged(s.target, base, s.st)
}

// saveMerged saves the changes made between base and changed onto the state
// of target as it is on disk now.
func saveMerged(target string, base, changed *state.State) error {
	_, err := changed.SaveOnto(target, base)
	return err
}

// updatingLocked reports whether the download running now will replace path,
// which makes it the one file that can't be removed by hand until it's done.
// s.mu must be held.
func (s *Server) updatingLocked(path string) bool {
	return s.downloading != nil && slices.Contains(s.downloading.job.old, path)
}
