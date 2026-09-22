package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

func (s *Server) setTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Profile string `json:"profile"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	busy := s.busyLocked()
	s.mu.Unlock()
	if busy != "" {
		writeError(w, http.StatusConflict, busy)
		return
	}
	if err := s.openTarget(req.Path, req.Profile); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.getState(w, r)
}

// openTarget makes path the current folder, with profile if given.
func (s *Server) openTarget(path, profile string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("Choose a folder.")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return errors.New("That folder doesn't exist or can't be opened: " + abs)
	}
	st, err := s.recordsFor(abs).Load(abs)
	if err != nil {
		return err
	}
	if profile != "" {
		p, err := scan.ParseProfile(profile)
		if err != nil {
			return err
		}
		st.Profile = p
	}

	s.mu.Lock()
	s.target, s.st, s.report, s.scan, s.lastErr, s.warnings = abs, st, nil, nil, "", nil
	s.mu.Unlock()
	saved := s.loadSettings()
	saved.Target = abs
	s.saveSettings(saved)
	return nil
}

// checkAppUpdate looks for a newer isoshelf once, in the background.
func (s *Server) checkAppUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := appupdate.Check(ctx, s.client(), s.cfg.Dirs.Config, s.cfg.Version, s.cfg.Now())
	if err != nil || n == nil {
		return
	}
	s.mu.Lock()
	s.notice = n
	s.mu.Unlock()
}

// recentTargets lists folders checked before, newest first. Portable mode
// keeps no history on the computer.
func (s *Server) recentTargets() []string {
	out := []string{}
	if s.cfg.Dirs.Portable || s.cfg.Dirs.Config == "" {
		return out
	}
	mirrors, err := state.LoadMirrors(s.cfg.Dirs.Config)
	if err != nil {
		return out
	}
	for _, m := range mirrors {
		if m.Path != "" && !slices.Contains(out, m.Path) {
			out = append(out, m.Path)
		}
	}
	return out
}
