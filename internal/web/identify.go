package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"

	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/identify"
)

// guessJSON is one suggestion of what a file might be.
type guessJSON struct {
	Entry   string `json:"entry"`
	Name    string `json:"name"`
	Arch    string `json:"arch,omitempty"`
	Version string `json:"version,omitempty"`
	Score   int    `json:"score"`
	Sure    bool   `json:"sure"`
	Reason  string `json:"reason"`
	Updates string `json:"updates"`
	Icon    string `json:"icon,omitempty"`

	IconColor string `json:"icon_color,omitempty"`
}

// getGuesses answers with what the file at ?path= might be. It reads only
// what the last scan already learned, so it costs nothing and touches no
// disk.
func (s *Server) getGuesses(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Query().Get("path")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scan == nil || s.st == nil {
		writeError(w, http.StatusBadRequest, "Scan the folder first.")
		return
	}

	guesses := []guessJSON{}
	for _, g := range identify.Suggest(s.scan, s.st, s.cat, file) {
		guesses = append(guesses, guessJSON{
			Entry: g.Entry.ID, Name: g.Entry.Name, Arch: g.Entry.Arch, Version: g.Version,
			Score: g.Score, Sure: g.Sure(), Reason: g.Reason, Updates: g.Entry.Updates(),
			Icon: g.Entry.Icon, IconColor: g.Entry.IconColor,
		})
	}
	var assigned string
	if rec, ok := s.st.Files[file]; ok && rec.Assigned {
		assigned = rec.Entry
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path": file, "name": path.Base(file), "assigned": assigned, "guesses": guesses,
	})
}

// identifyFile records what the user says a file is, or forgets an earlier
// answer when entry is empty. Nothing on disk is touched or renamed: the
// answer is remembered in the folder's state and lasts until the file itself
// changes.
func (s *Server) identifyFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path    string `json:"path"`
		Entry   string `json:"entry"`
		Version string `json:"version"`
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
	case s.run != nil:
		writeError(w, http.StatusConflict, "Wait until the current job finishes.")
		return
	}

	var message string
	if req.Entry == "" {
		if err := s.st.Unassign(req.Path); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		message = fmt.Sprintf("%s is unrecognized again.", path.Base(req.Path))
	} else {
		entry := s.cat.Entry(req.Entry)
		if entry == nil {
			writeError(w, http.StatusBadRequest, "Unknown image.")
			return
		}
		if err := s.st.Assign(req.Path, entry.ID, req.Version); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		message = fmt.Sprintf("%s is now treated as %s.", path.Base(req.Path), entry.Name)
	}

	if err := s.st.Save(s.target); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that: "+err.Error())
		return
	}
	// The report is built from the state, so it has to be built again. Only
	// the offline part: what the file is has changed, not what the newest
	// version is. The page picks the online check up again if it had run.
	wasChecked := s.report != nil && s.report.Checked
	s.report = check.Offline(s.scan, s.st, s.cat)
	s.updatedAt = s.cfg.Now()
	writeJSON(w, http.StatusOK, map[string]any{"status": "saved", "message": message, "recheck": wasChecked})
}
