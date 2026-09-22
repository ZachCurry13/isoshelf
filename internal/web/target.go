package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/scan"
	"github.com/ZachCurry13/isoshelf/internal/settings"
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
	s.updateSettings(func(c *settings.Settings) { c.Target = abs })
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

// recentTargets lists the folders isoshelf remembers, newest first: where
// each one is, when it was last looked at and what it held then. Portable
// mode keeps no history on the computer, so the list is empty there.
//
// All of it comes from the copies in isoshelf's own folder, never from the
// drives themselves: a folder in this list may be a NAS that is asleep or a
// stick in a drawer, and opening the page should not go looking for them.
func (s *Server) recentTargets() []rememberedJSON {
	out := []rememberedJSON{}
	if s.cfg.Dirs.Portable || s.cfg.Dirs.Config == "" {
		return out
	}
	mirrors, err := state.LoadMirrors(s.cfg.Dirs.Config)
	if err != nil {
		return out
	}
	seen := map[string]bool{}
	for _, m := range mirrors {
		if m.Path == "" || seen[m.Path] {
			continue
		}
		seen[m.Path] = true
		out = append(out, rememberedJSON{
			Path: m.Path, ID: m.TargetID, LastUsed: m.SavedAt,
			Files: m.Files, Bytes: m.Bytes,
		})
	}
	return out
}
