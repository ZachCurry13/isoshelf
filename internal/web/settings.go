package web

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

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

// updateSettings reads the settings, lets change alter them, and writes them
// back - with nothing else allowed in between.
//
// Every change to this file is read-modify-write, and there is more than one
// writer now that isoshelf updates images on a schedule: a switch flicked on
// the page and the scheduler noting when it last ran can otherwise land on
// each other, and one of the two changes simply vanishes. Which is exactly
// the bug this file already warns about between the page and the command
// line, in a form that a lock can actually fix.
func (s *Server) updateSettings(change func(*settings.Settings)) settings.Settings {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()
	current := s.loadSettings()
	change(&current)
	s.saveSettings(current)
	return current
}

// settingsRequest is what the page sends. Every field is optional: the page
// sends one switch at a time, so anything left out is left as it was, and
// false could not say that where a pointer can.
type settingsRequest struct {
	CatalogAuto *bool  `json:"catalog_auto"`
	OldFiles    string `json:"old_files"`
	// AutoCheck is whether isoshelf checks the images for updates by itself;
	// AppUpdateCheck whether it looks for a newer isoshelf.
	AutoCheck      *bool `json:"auto_check"`
	AppUpdateCheck *bool `json:"app_update_check"`
	// AutoUpdate is whether isoshelf updates the images by itself, and
	// AutoUpdateEvery how often.
	AutoUpdate      *bool  `json:"auto_update"`
	AutoUpdateEvery string `json:"auto_update_every"`
	// Appearance fields, one at a time like the rest.
	Theme        *string `json:"theme"`
	HighContrast *bool   `json:"high_contrast"`
	LargerText   *bool   `json:"larger_text"`
	ReduceMotion *bool   `json:"reduce_motion"`
	// Reset puts every choice back to its default.
	Reset bool `json:"reset"`
}

// setSettings stores the choices that live on this computer rather than in a
// folder's state. Anything the request leaves out is left as it was, so the
// page can send one switch at a time.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	current := s.updateSettings(func(c *settings.Settings) { req.applyTo(c) })

	// Turning it on means looking now rather than within the quarter hour.
	if req.AutoUpdate != nil && *req.AutoUpdate {
		s.inBackground(s.autoUpdateIfDue)
	}
	// Saying yes to looking for a newer isoshelf means looking now, rather
	// than at the next start.
	if (req.AppUpdateCheck != nil || req.Reset) && settings.On(current.AppUpdateCheck) && s.notice == nil {
		go s.checkAppUpdate()
	}
	// Turning the catalog back on, whether by switch or by reset, means it
	// should look for a newer list now rather than at the next start.
	if current.CatalogAuto == nil || *current.CatalogAuto {
		if req.CatalogAuto != nil || req.Reset {
			s.startCatalogRefresh(false)
		}
	}
	s.getState(w, r)
}

// applyTo puts this request onto the settings, in one place so that reading
// what a request does doesn't mean reading a handler.
func (req settingsRequest) applyTo(current *settings.Settings) {
	if req.Reset {
		*current = current.Reset()
	}
	if req.CatalogAuto != nil {
		current.CatalogAuto = req.CatalogAuto
	}
	if choice := settings.CleanOldFiles(req.OldFiles); choice != "" {
		current.OldFiles = choice
	}
	if req.AutoCheck != nil {
		current.AutoCheck = req.AutoCheck
	}
	if req.AppUpdateCheck != nil {
		current.AppUpdateCheck = req.AppUpdateCheck
	}
	if req.AutoUpdate != nil {
		current.AutoUpdate = req.AutoUpdate
		if *req.AutoUpdate {
			// Due now rather than a day after the switch was flicked:
			// somebody who has just turned this on wants to see it work.
			current.AutoUpdateLast = time.Time{}
			// And it needs checking by itself, or it has nothing to act on.
			// A switch that quietly does nothing is worse than no switch.
			current.AutoCheck = req.AutoUpdate
		}
	}
	if req.AutoUpdateEvery != "" {
		current.AutoUpdateEvery = settings.CleanEvery(req.AutoUpdateEvery)
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
