package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// The choices below belong to this computer rather than to any one folder, so
// they live in the settings file that internal/settings owns. The web UI and
// the command line read the same file; keeping a second copy of the fields
// here once meant a choice made on the page quietly erased one made on the
// command line.

func (s *Server) loadSettings() settings.Settings {
	return settings.Load(s.cfg.Dirs.Config)
}

func (s *Server) saveSettings(st settings.Settings) {
	settings.Save(s.cfg.Dirs.Config, st) // best effort: settings are a convenience
}

// setSettings stores the choices that live on this computer rather than in a
// folder's state. Anything the request leaves out is left as it was, so the
// page can send one switch at a time.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CatalogAuto *bool  `json:"catalog_auto"`
		OldFiles    string `json:"old_files"`
		// Appearance fields are sent one at a time, so each is a pointer:
		// absent means "leave it alone", which false could not say.
		Theme        *string `json:"theme"`
		HighContrast *bool   `json:"high_contrast"`
		LargerText   *bool   `json:"larger_text"`
		ReduceMotion *bool   `json:"reduce_motion"`
		// Reset puts every choice back to its default.
		Reset bool `json:"reset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	current := s.loadSettings()
	if req.Reset {
		current = current.Reset()
	}
	if req.CatalogAuto != nil {
		current.CatalogAuto = req.CatalogAuto
	}
	if choice := settings.CleanOldFiles(req.OldFiles); choice != "" {
		current.OldFiles = choice
	}
	if req.Theme != nil {
		current.Appearance.Theme = settings.CleanTheme(*req.Theme)
	}
	if req.HighContrast != nil {
		current.Appearance.HighContrast = *req.HighContrast
	}
	if req.LargerText != nil {
		current.Appearance.LargerText = *req.LargerText
	}
	if req.ReduceMotion != nil {
		current.Appearance.ReduceMotion = *req.ReduceMotion
	}
	s.saveSettings(current)
	// Turning the catalog back on, whether by switch or by reset, means it
	// should look for a newer list now rather than at the next start.
	if current.CatalogAuto == nil || *current.CatalogAuto {
		if req.CatalogAuto != nil || req.Reset {
			s.startCatalogRefresh(false)
		}
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
