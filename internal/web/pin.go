package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Pinning a file (v0.7.0, #54): one answer in Settings says what happens to
// the file an update replaces, and a pin says "not this one". A pinned file
// stays exactly where it is - update.Options.Pinned - and is left out of the
// older-versions review on the page.

// setPin pins or unpins one file.
func (s *Server) setPin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`
		Pinned bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	switch {
	case s.scanningLocked() != "":
		s.mu.Unlock()
		writeError(w, http.StatusConflict, s.scanningLocked())
		return
	case s.st == nil:
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	}
	base := s.st.Clone()
	if err := s.st.Pin(req.Path, req.Pinned); err != nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "That file isn't in the folder any more. Scan again.")
		return
	}
	err := s.saveStateLocked(base)
	s.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save the pin: "+err.Error())
		return
	}
	s.getState(w, r)
}

// pinnedAmong is the pinned files among old, from records read just now: a
// pin made while the download waited its turn still counts.
func pinnedAmong(st *state.State, old []string) []string {
	var out []string
	for _, path := range st.Pinned() {
		if slices.Contains(old, path) {
			out = append(out, path)
		}
	}
	return out
}

// migrateChoicesLocked turns a folder's per-image answers from before v0.7.0
// into pins, once, and says what it did. s.mu must be held; the caller saves
// when it returns true.
func (s *Server) migrateChoicesLocked(st *state.State) bool {
	fallback := settings.CleanOldFiles(s.loadSettings().OldFiles)
	if fallback == "" {
		fallback = settings.OldReplace
	}
	m := st.MigrateChoices(fallback)
	if m == (state.Migrated{}) {
		return false
	}
	var parts []string
	if m.Pinned > 0 {
		parts = append(parts, fmt.Sprintf("%s set to keep both copies now %s pinned files",
			plural(m.Pinned, "image"), haveHas(m.Pinned)))
	}
	if m.Following > 0 {
		parts = append(parts, fmt.Sprintf("%s with an answer of %s own now %s the setting in Settings",
			plural(m.Following, "image"), itsTheir(m.Following), followFollows(m.Following)))
	}
	msg := "Each image's own answer for old files has been replaced by pins: "
	for i, p := range parts {
		if i > 0 {
			msg += ", and "
		}
		msg += p
	}
	s.warnings = append(s.warnings, msg+".")
	return true
}

func haveHas(n int) string {
	if n == 1 {
		return "has"
	}
	return "have"
}

func itsTheir(n int) string {
	if n == 1 {
		return "its"
	}
	return "their"
}

func followFollows(n int) string {
	if n == 1 {
		return "follows"
	}
	return "follow"
}
