package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// start begins a scan. ask is what the scan does about the internet:
//
//	askIfDue   - the ordinary scan: check for updates unless Settings says
//	             not to, reusing the answers isoshelf already has
//	askAgain   - Refresh: ask every project again, however recently it was
//	             asked
//	askToProve - Check it: ask whatever isn't known fresh, even with
//	             checking by itself turned off, because somebody pressed a
//	             button whose whole point is the project's answer
type asking int

const (
	askIfDue asking = iota
	askAgain
	askToProve
)

func (s *Server) start(w http.ResponseWriter, ask asking) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "":
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.scanBusyLocked() != "":
		writeError(w, http.StatusConflict, s.scanBusyLocked())
		return
	}
	s.lastErr = ""
	s.startScanLocked(ask)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

// startScanLocked starts a scan; s.mu must be held and no other scan may be
// running. A download may be: it has its own slot.
// hash names files to hash in this run whatever else is hashed: Check it.
func (s *Server) startScanLocked(ask asking, hash ...string) {
	ctx, cancel := context.WithCancel(context.Background())
	mem := s.memoryFor(ask)
	kind := "scan"
	if mem != nil {
		kind = "check"
	}
	s.scanning = &run{kind: kind, started: s.cfg.Now(), progress: inventory.Progress{Stage: inventory.Scanning}, cancel: cancel}
	go s.execute(ctx, s.target, s.st.Profile, mem, hash)
}

// memoryFor says whether this scan goes online and what it may reuse. Nil
// means the folder only: either nothing was asked for, or the user turned
// checking by itself off and this isn't Refresh.
func (s *Server) memoryFor(ask asking) check.Memory {
	switch ask {
	case askAgain:
		return s.memory.Asking()
	case askToProve:
		return s.memory
	case askIfDue:
		if settings.On(s.loadSettings().AutoCheck) {
			return s.memory
		}
	}
	return nil
}

func (s *Server) execute(ctx context.Context, target string, profile scan.Profile, mem check.Memory, hash []string) {
	client := s.client()
	res, err := inventory.Run(ctx, inventory.Options{
		Target:  target,
		Profile: profile,
		Online:  mem != nil,
		Memory:  mem,
		Client:  client,
		Catalog: s.catalog(),
		Dirs:    s.cfg.Dirs,
		Records: s.recordsFor(target),
		// Sharing means other isoshelfs ask for files by hash, so every
		// image needs one rather than only the fixed-name ones.
		HashAll:   s.sharing(),
		HashPaths: hash,
		Now:       s.cfg.Now,
		// Show the folder's contents as soon as they're known. Hashing the
		// images whose filename never changes comes next, and on a USB drive
		// that is minutes of reading; there's no reason to stare at a spinner
		// for it.
		Interim: func(res *inventory.Result) {
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.target == target {
				s.report, s.st, s.scan, s.updatedAt = res.Report, res.State, res.Scan, s.cfg.Now()
			}
		},
		Progress: func(p inventory.Progress) {
			s.mu.Lock()
			if s.scanning != nil {
				s.scanning.progress = p
			}
			s.mu.Unlock()
		},
	})

	// Answers that came back are worth keeping whether or not the scan
	// finished: they cost a round trip each.
	if mem != nil {
		s.memory.Save() // best effort: the worst case is asking again
	}

	// The room left, before the lock: it asks a disk, and a NAS that has
	// gone to sleep must not hold the page up.
	room := s.targetSpace()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanning = nil
	if res != nil && s.target == target {
		s.report, s.st, s.scan, s.warnings, s.updatedAt = res.Report, res.State, res.Scan, res.Warnings, s.cfg.Now()
	}
	switch {
	case errors.Is(err, context.Canceled):
		s.lastErr = "Stopped. Showing what was found so far."
	case err != nil:
		s.lastErr = err.Error()
	}
	// A scan the scheduler started ends by queueing what it found. Only if
	// it actually finished: a cancelled scan has seen part of the folder,
	// and acting on half a look is how a folder gets surprised.
	if s.autoQueue {
		s.autoQueue = false
		if err == nil && s.target == target {
			if added, stopped := s.queueUpdatesLocked(room); added > 0 {
				s.autoNote = autoNote(added, stopped, s.cfg.Now())
			}
		}
	}
	// A download that placed a file while this scan ran left s.placed set and
	// no scan able to start; this is where that scan goes.
	s.startNextLocked()
}

// cancel stops what is running. For downloads that means all of them: the
// one running stops, and the ones waiting are taken off the queue.
func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.scanning != nil {
		s.scanning.cancel()
	}
	if s.downloading != nil {
		s.queue = nil
		s.downloading.cancel()
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}
