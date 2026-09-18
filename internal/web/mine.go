package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/usercat"
)

// addMyImage records what the user says a file is, for images no catalog will
// ever know. It writes an entry into their own catalog file and ties the file
// to it, in one step: naming an image is the same act as identifying it.
func (s *Server) addMyImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		Arch     string `json:"arch"`
		Category string `json:"category"`
		Page     string `json:"page"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "" || s.st == nil || s.scan == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder and scan it first.")
		return
	case s.busyLocked() != "":
		writeError(w, http.StatusConflict, s.busyLocked())
		return
	case s.cfg.Dirs.Config == "":
		writeError(w, http.StatusBadRequest, "There is nowhere to keep your own images.")
		return
	}
	if _, known := s.st.Files[req.Path]; !known {
		writeError(w, http.StatusBadRequest, req.Path+" is not in the last scan.")
		return
	}

	mine, err := usercat.Add(s.cfg.Dirs.Config, s.cat, usercat.Image{
		Name:     req.Name,
		Filename: path.Base(req.Path),
		Arch:     req.Arch,
		Category: req.Category,
		Page:     req.Page,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged, err := catalog.Merge(s.cat, mine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.cat = merged

	// The entry matches this exact filename, so the next scan finds it by
	// itself; tie the file to it now so the page shows it straight away.
	if err := s.st.Assign(req.Path, usercat.ID(req.Name), ""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.Save(s.target); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that: "+err.Error())
		return
	}
	s.report = check.Offline(s.scan, s.st, s.cat)
	s.updatedAt = s.cfg.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "saved",
		"message": fmt.Sprintf("%s is now called %s. It's in your own list, at %s.", path.Base(req.Path), strings.TrimSpace(req.Name), usercat.Path(s.cfg.Dirs.Config)),
	})
}

// countMine is how many entries the user named themselves.
func countMine(cat *catalog.Catalog) int {
	n := 0
	for i := range cat.Entries {
		if usercat.Mine(cat.Entries[i].ID) {
			n++
		}
	}
	return n
}
