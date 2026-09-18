package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type settings struct {
	Target string `json:"target,omitempty"`
	// CatalogAuto is nil until the user says either way; the default is on.
	CatalogAuto *bool `json:"catalog_auto,omitempty"`
	// Bookmarks are folders the user pinned in the chooser.
	Bookmarks []string `json:"bookmarks,omitempty"`
	// ReplaceAction is what the user last chose for the file an update
	// replaces: "move-aside" or "delete".
	ReplaceAction string `json:"replace_action,omitempty"`
}

func (s *Server) loadSettings() settings {
	var st settings
	if data, err := os.ReadFile(filepath.Join(s.cfg.Dirs.Config, settingsFile)); err == nil {
		json.Unmarshal(data, &st)
	}
	return st
}

func (s *Server) saveSettings(st settings) {
	if s.cfg.Dirs.Config == "" {
		return
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err == nil && os.MkdirAll(s.cfg.Dirs.Config, 0o755) == nil {
		os.WriteFile(filepath.Join(s.cfg.Dirs.Config, settingsFile), data, 0o644) // best effort
	}
}

// setSettings stores the choices that live on this computer rather than in a
// folder's state.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CatalogAuto   *bool  `json:"catalog_auto"`
		ReplaceAction string `json:"replace_action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.CatalogAuto != nil {
		current := s.loadSettings()
		current.CatalogAuto = req.CatalogAuto
		s.saveSettings(current)
		if *req.CatalogAuto {
			s.startCatalogRefresh(false)
		}
	}
	if req.ReplaceAction == "move-aside" || req.ReplaceAction == "delete" {
		current := s.loadSettings()
		current.ReplaceAction = req.ReplaceAction
		s.saveSettings(current)
	}
	s.getState(w, r)
}

// setBookmark pins or unpins a folder, so a NAS share or an image library
// doesn't have to be found again every time.
func (s *Server) setBookmark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`
		Remove bool   `json:"remove"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	path := strings.TrimSpace(req.Path)
	if path == "" {
		writeError(w, http.StatusBadRequest, "Which folder?")
		return
	}
	current := s.loadSettings()
	current.Bookmarks = slices.DeleteFunc(current.Bookmarks, func(b string) bool { return strings.EqualFold(b, path) })
	if !req.Remove {
		current.Bookmarks = append(current.Bookmarks, path)
		slices.SortFunc(current.Bookmarks, func(a, b string) int {
			return strings.Compare(strings.ToLower(a), strings.ToLower(b))
		})
	}
	s.saveSettings(current)
	s.getState(w, r)
}
