package web

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/check"
)

func (s *Server) setTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entry    string  `json:"entry"`
		KeepOld  *bool   `json:"keep_old"`
		OldFiles *string `json:"old_files"`
		Starred  *bool   `json:"starred"`
		// NotExpected is "Stop expecting it" (#55). It unstars the image
		// too: a star means "tell me if this goes missing".
		NotExpected *bool `json:"not_expected"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.OldFiles != nil && *req.OldFiles != "replace" && *req.OldFiles != "archive" && *req.OldFiles != "keep" {
		writeError(w, http.StatusBadRequest, `old_files must be "replace", "archive" or "keep".`)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.scanningLocked() != "":
		writeError(w, http.StatusConflict, s.scanningLocked())
		return
	case s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.cat.Entry(req.Entry) == nil:
		writeError(w, http.StatusBadRequest, "Unknown image.")
		return
	}
	base := s.st.Clone()
	t := s.st.Track(req.Entry)
	if req.KeepOld != nil {
		t.KeepOld = *req.KeepOld
	}
	if req.OldFiles != nil {
		t.OldFiles = *req.OldFiles
		t.KeepOld = t.OldFiles == "keep"
	}
	if req.Starred != nil {
		t.Starred = *req.Starred
	}
	if req.NotExpected != nil {
		t.NotExpected = *req.NotExpected
		if t.NotExpected {
			t.Starred = false
		}
	}
	s.st.SetTrack(req.Entry, t)
	s.dropMissingLocked(req.Entry)
	if err := s.saveStateLocked(base); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the setting: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": s.st.Tracks, "usual_set": nonNil(s.st.UsualSet())})
}

// dropMissingLocked takes an image off the list as missing once it isn't
// expected here any more - unstarred, or not expected - rather than at the
// next scan. Only that row goes: building the report again would lose what
// the last check found for everything else. s.mu must be held.
func (s *Server) dropMissingLocked(entry string) {
	if s.report == nil || slices.Contains(s.st.UsualSet(), entry) {
		return
	}
	s.report.Items = slices.DeleteFunc(slices.Clone(s.report.Items), func(it check.Item) bool {
		return it.Status == check.Missing && it.Entry != nil && it.Entry.ID == entry
	})
}
