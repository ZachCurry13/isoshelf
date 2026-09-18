package web

import (
	"encoding/json"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

func (s *Server) setTrack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entry   string `json:"entry"`
		KeepOld *bool  `json:"keep_old"`
		Starred *bool  `json:"starred"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.run != nil && s.run.job == nil:
		writeError(w, http.StatusConflict, "Wait until the scan finishes.")
		return
	case s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.cat.Entry(req.Entry) == nil:
		writeError(w, http.StatusBadRequest, "Unknown image.")
		return
	}
	t := s.st.Track(req.Entry)
	if req.KeepOld != nil {
		t.KeepOld = *req.KeepOld
	}
	if req.Starred != nil {
		t.Starred = *req.Starred
	}
	s.st.SetTrack(req.Entry, t)
	if err := s.saveTrackLocked(req.Entry, t); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the setting: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tracks": s.st.Tracks, "usual_set": nonNil(s.st.UsualSet())})
}

// saveTrackLocked writes one track's settings to the folder's state; s.mu
// must be held. While a download runs, the state on disk is ahead of the one
// in memory (it knows the files placed since the last scan), so the setting
// goes into the copy on disk rather than overwriting it with an older one.
func (s *Server) saveTrackLocked(entry string, t state.Track) error {
	if s.run == nil || s.run.job == nil {
		return s.st.Save(s.target)
	}
	disk, err := state.Load(s.target)
	if err != nil {
		return err
	}
	disk.SetTrack(entry, t)
	return disk.Save(s.target)
}
