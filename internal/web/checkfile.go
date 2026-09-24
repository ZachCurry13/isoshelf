package web

import (
	"encoding/json"
	"net/http"
)

// checkFile is Check it (v0.8.0, #62): read one file, and see whether it is
// the release the project publishes. Only when somebody asks, because
// reading a 6 GB image off a USB stick takes minutes (the maintainer,
// 2026-09-24: "A lot of times I just trust it").
//
// It is an ordinary scan that hashes this file too, and goes online for the
// project's answer if the one isoshelf has is stale. The page shows it the
// way it shows any scan, and the check that ends it is what marks the file
// proven when its hash is the published one (inventory.Run, MarkChecked).
func (s *Server) checkFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "" || s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.scanBusyLocked() != "":
		writeError(w, http.StatusConflict, s.scanBusyLocked())
		return
	}
	if _, ok := s.st.Files[req.Path]; !ok {
		writeError(w, http.StatusBadRequest, "That file isn't in the folder any more. Scan again.")
		return
	}
	s.lastErr = ""
	s.startScanLocked(askToProve, req.Path)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}
