package web

import (
	"encoding/json"
	"net/http"
	"path"
	"slices"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// archiveItemJSON is one image that used to be in the folder.
type archiveItemJSON struct {
	Path    string    `json:"path"`
	Entry   string    `json:"entry,omitempty"`
	Name    string    `json:"name"`
	Version string    `json:"version,omitempty"`
	Size    int64     `json:"size"`
	Gone    string    `json:"gone"`
	GoneAt  time.Time `json:"gone_at"`
	// Restorable is true while the file still waits in .isoshelf/removed.
	Restorable bool `json:"restorable"`
	// Downloadable is true when isoshelf can fetch this image again.
	Downloadable bool   `json:"downloadable"`
	Page         string `json:"page,omitempty"`
	Icon         string `json:"icon,omitempty"`
	IconColor    string `json:"icon_color,omitempty"`
}

// getArchive lists the images that have left this folder.
func (s *Server) getArchive(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []archiveItemJSON{}
	if s.st == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	waiting, _, _ := update.Removed(s.target)

	for _, past := range s.st.Archive() {
		item := archiveItemJSON{
			Path: past.Path, Entry: past.Entry, Name: past.Path, Version: past.Version,
			Size: past.Size, Gone: past.Gone, GoneAt: past.GoneAt,
			Restorable: past.Gone == state.GoneMovedAside && slices.Contains(waiting, path.Base(past.Path)),
		}
		if e := s.cfg.Catalog.Entry(past.Entry); e != nil {
			item.Name = e.Name
			item.Page, item.Icon, item.IconColor = e.Page, e.Icon, e.IconColor
			item.Downloadable = e.Updates() == "download"
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// restore puts a moved-aside file back in the folder.
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "" || s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.run != nil:
		writeError(w, http.StatusConflict, "Wait until the current job finishes.")
		return
	}
	if err := update.Restore(s.target, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}
