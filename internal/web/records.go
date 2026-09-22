package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Where a folder's records are kept is the one setting that owns files. The
// rest of Settings is switches; this one has to pick up what isoshelf has
// learned about the folder and put it down somewhere else, with nothing lost
// on the way and nothing deleted that the user didn't choose to lose.

// recordsJSON is what the page shows and sends back.
type recordsJSON struct {
	// Location is settings.InFolder, WithApp or Elsewhere.
	Location string `json:"location"`
	// Dir is the folder for Elsewhere.
	Dir string `json:"dir,omitempty"`
	// File is where this folder's records are right now, so Settings can
	// point at the actual file rather than describing it.
	File string `json:"file,omitempty"`
}

// recordsLocked describes where the open folder's records are. s.mu must be
// held.
func (s *Server) recordsLocked(saved settings.Settings) recordsJSON {
	if s.target == "" {
		return recordsJSON{Location: settings.InFolder}
	}
	r := saved.RecordsFor(s.target)
	out := recordsJSON{Location: r.Location, Dir: r.Dir}
	home := state.Home(saved.RecordsHome(s.target, s.cfg.Dirs.Config))
	if file, err := home.File(s.target); err == nil {
		out.File = file
	}
	return out
}

// setRecords moves the open folder's records somewhere else and remembers
// that that is where they now are.
func (s *Server) setRecords(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Location string `json:"location"`
		Dir      string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	choice := settings.CleanRecordsLocation(req.Location)
	if choice == "" {
		writeError(w, http.StatusBadRequest, "Choose where to keep this folder's records.")
		return
	}

	// Nothing may be reading or writing the records while they move: a scan
	// or a download that saved afterwards would save to the old place, and
	// its work would be the part that went missing.
	s.mu.Lock()
	target, busy := s.target, s.busyLocked()
	s.mu.Unlock()
	if target == "" {
		writeError(w, http.StatusBadRequest, "Open a folder first.")
		return
	}
	if busy != "" {
		writeError(w, http.StatusConflict, busy)
		return
	}

	want := settings.Records{Location: choice, Dir: req.Dir}
	if choice == settings.Elsewhere {
		home, err := state.CleanHome(req.Dir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		want.Dir = string(home)
		if under, err := isUnder(target, want.Dir); err == nil && under {
			writeError(w, http.StatusBadRequest,
				"That folder is inside the one isoshelf is looking after, so its records would be on the same drive as the images. Keep them in the folder instead.")
			return
		}
	}

	saved := s.loadSettings()
	from := state.Home(saved.RecordsHome(target, s.cfg.Dirs.Config))
	after := saved
	after.SetRecordsFor(target, want)
	to := state.Home(after.RecordsHome(target, s.cfg.Dirs.Config))

	moved, err := state.Move(from, to, target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't move this folder's records: "+err.Error())
		return
	}
	if err := settings.Save(s.cfg.Dirs.Config, after); err != nil {
		// The records are at the new place but nothing would look for them
		// there. Put them back rather than leave them stranded.
		state.Move(to, from, target)
		writeError(w, http.StatusInternalServerError, "Couldn't remember where the records went, so they were put back: "+err.Error())
		return
	}

	// Read them from where they now are, so the page shows what isoshelf will
	// actually use from here on.
	st, err := to.Load(target)
	s.mu.Lock()
	if err == nil && s.target == target {
		s.st = st
	}
	if moved.Kept {
		s.warnings = append(s.warnings, "isoshelf already had records for this folder in "+filepath.Dir(moved.To)+
			", so those are the ones it is using. The older ones are still in "+moved.From+" - delete them yourself if you don't want them.")
	}
	s.mu.Unlock()
	s.getState(w, r)
}

// isUnder reports whether dir is inside parent, or is parent itself. Rel
// gives an error for two different drives, which is a plain no.
func isUnder(parent, dir string) (bool, error) {
	rel, err := filepath.Rel(parent, dir)
	if err != nil {
		return false, err
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))), nil
}

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
	saved := s.loadSettings()
	saved.SetRecordsFor(req.Path, settings.Records{Location: settings.InFolder})
	s.saveSettings(saved)
	s.getState(w, r)
}

// sameFolder compares two paths the way the rest of the page does: the same
// spelling, ignoring case, since Windows and macOS do.
func sameFolder(a, b string) bool {
	return a != "" && strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
