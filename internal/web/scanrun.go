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
type asking int

const (
	askIfDue asking = iota
	askAgain
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
func (s *Server) startScanLocked(ask asking) {
	ctx, cancel := context.WithCancel(context.Background())
	mem := s.memoryFor(ask)
	kind := "scan"
	if mem != nil {
		kind = "check"
	}
	s.scanning = &run{kind: kind, started: s.cfg.Now(), progress: inventory.Progress{Stage: inventory.Scanning}, cancel: cancel}
	go s.execute(ctx, s.target, s.st.Profile, mem)
}

// memoryFor says whether this scan goes online and what it may reuse. Nil
// means the folder only: either nothing was asked for, or the user turned
// checking by itself off and this isn't Refresh.
func (s *Server) memoryFor(ask asking) check.Memory {
	switch ask {
	case askAgain:
		return s.memory.Asking()
	case askIfDue:
		if settings.On(s.loadSettings().AutoCheck) {
			return s.memory
		}
	}
	return nil
}

func (s *Server) execute(ctx context.Context, target string, profile scan.Profile, mem check.Memory) {
	client := s.client()
	res, err := inventory.Run(ctx, inventory.Options{
		Target:  target,
		Profile: profile,
		Online:  mem != nil,
		Memory:  mem,
		Client:  client,
		Catalog: s.catalog(),
		Dirs:    s.cfg.Dirs,
		Now:     s.cfg.Now,
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

	s.mu.Lock()
	defer s.mu.Unlock()
	s.scanning = nil
	if res != nil && s.target == target {
		s.report, s.st, s.scan, s.warnings, s.updatedAt = res.Report, res.State, res.Scan, res.Warnings, s.cfg.Now()
	}
	switch {
	case errors.Is(err, context.Canceled):
		s.lastErr = "Stopped. What was found so far is shown."
	case err != nil:
		s.lastErr = err.Error()
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
