package web

import (
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// The folder's state file has more than one writer: a download that runs for
// an hour, and the user removing, archiving, identifying or starring things
// meanwhile. Nobody saves their whole copy over it. Each writer keeps a copy
// of the state from before its change, and saveStateLocked carries only the
// change over to the file as it is on disk now (state.Merge). A scan is the
// exception: it rewrites everything, so nothing else runs beside it.

// scanningLocked says why the folder's files can't be changed right now, or
// "" when they can: only while a scan or check runs. s.mu must be held.
func (s *Server) scanningLocked() string {
	if s.run != nil && s.run.job == nil {
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
	disk, err := state.Load(target)
	if err != nil {
		return err
	}
	state.Merge(base, changed, disk)
	return disk.Save(target)
}

// updatingLocked reports whether the download running now will replace path,
// which makes it the one file that can't be removed by hand until it's done.
// s.mu must be held.
func (s *Server) updatingLocked(path string) bool {
	return s.run != nil && s.run.job != nil && slices.Contains(s.run.job.old, path)
}
