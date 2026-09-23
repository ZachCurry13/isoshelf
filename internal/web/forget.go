package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// forgetFolder takes a folder off the list isoshelf remembers. It removes the
// copy of its history in isoshelf's own folder and the answer it was given
// about where its records live - and nothing else. The folder itself, its own
// records, its archive and every image in it are untouched, which is what the
// button has to mean if anyone is to press it.
func (s *Server) forgetFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		ID   string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if strings.TrimSpace(req.Path) == "" || strings.TrimSpace(req.ID) == "" {
		writeError(w, http.StatusBadRequest, "Which folder?")
		return
	}
	s.mu.Lock()
	open := s.target
	s.mu.Unlock()
	if sameFolder(open, req.Path) {
		writeError(w, http.StatusConflict, "That's the folder you have open. Open another one first.")
		return
	}
	if err := state.Forget(s.cfg.Dirs.Config, req.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't forget that folder: "+err.Error())
		return
	}
	s.updateSettings(func(c *settings.Settings) {
		c.SetRecordsFor(req.Path, settings.Records{Location: settings.InFolder})
	})
	s.getState(w, r)
}

// sameFolder compares two paths the way the rest of the page does: the same
// spelling, ignoring case, since Windows and macOS do.
func sameFolder(a, b string) bool {
	return a != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
