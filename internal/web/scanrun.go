package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

func (s *Server) start(w http.ResponseWriter, online bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "":
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.busyLocked() != "":
		writeError(w, http.StatusConflict, s.busyLocked())
		return
	}
	s.lastErr = ""
	s.startScanLocked(online)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

// startScanLocked starts a scan, or a check when online; s.mu must be held
// and nothing may be running.
func (s *Server) startScanLocked(online bool) {
	ctx, cancel := context.WithCancel(context.Background())
	kind := "scan"
	if online {
		kind = "check"
	}
	s.run = &run{kind: kind, started: s.cfg.Now(), progress: inventory.Progress{Stage: inventory.Scanning}, cancel: cancel}
	go s.execute(ctx, s.target, s.st.Profile, online)
}

func (s *Server) execute(ctx context.Context, target string, profile scan.Profile, online bool) {
	client := s.client()
	res, err := inventory.Run(ctx, inventory.Options{
		Target:  target,
		Profile: profile,
		Online:  online,
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
			if s.run != nil {
				s.run.progress = p
			}
			s.mu.Unlock()
		},
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = nil
	if res != nil && s.target == target {
		s.report, s.st, s.scan, s.warnings, s.updatedAt = res.Report, res.State, res.Scan, res.Warnings, s.cfg.Now()
	}
	switch {
	case errors.Is(err, context.Canceled):
		s.lastErr = "Stopped. What was found so far is shown."
	case err != nil:
		s.lastErr = err.Error()
	}
	// Downloads added while the scan ran go now.
	s.startNextLocked()
}

// cancel stops what is running. For downloads that means all of them: the
// one running stops, and the ones waiting are taken off the queue.
func (s *Server) cancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.run != nil {
		if s.run.job != nil {
			s.queue = nil
		}
		s.run.cancel()
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopping"})
}
