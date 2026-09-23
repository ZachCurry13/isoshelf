package web

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Where a folder's records are kept is the one setting that owns files. The
// rest of Settings is switches; this one has to pick up what isoshelf has
// learned about the folder and put it down somewhere else, with nothing lost
// on the way and nothing deleted that the user didn't choose to lose. So it
// asks first (planRecords), and says afterwards what it did (setRecords).

// recordsJSON is what the page shows and sends back.
type recordsJSON struct {
	// Location is settings.InFolder, WithApp or Elsewhere.
	Location string `json:"location"`
	// Dir is the folder for Elsewhere.
	Dir string `json:"dir,omitempty"`
	// File is where this folder's records are right now, so Settings can
	// point at the actual file rather than describing it.
	File string `json:"file,omitempty"`
	// Moved is what a move just did. Only the answer to that move carries
	// it; the page keeps it for as long as it goes on saying so.
	Moved *movedJSON `json:"moved,omitempty"`
}

// movedJSON is what moving the records did, or would do.
type movedJSON struct {
	// What is "move"; "keep", when both places had records and the ones
	// already where they were going win; "use", when only that place had
	// any; "none", when nothing has been saved about the folder yet; or
	// "same", when nothing changes.
	What      string    `json:"what"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	FromDir   string    `json:"from_dir"`
	ToDir     string    `json:"to_dir"`
	FromSaved time.Time `json:"from_saved,omitzero"`
	ToSaved   time.Time `json:"to_saved,omitzero"`
}

func movedFor(m state.Moved) *movedJSON {
	out := &movedJSON{
		From: m.From, To: m.To, FromDir: filepath.Dir(m.From), ToDir: filepath.Dir(m.To),
		FromSaved: m.FromSaved, ToSaved: m.ToSaved,
	}
	switch {
	case m.From == m.To:
		out.What = "same"
	case m.Moved:
		out.What = "move"
	case m.Kept:
		out.What = "keep"
	case !m.ToSaved.IsZero():
		out.What = "use"
	default:
		out.What = "none"
	}
	return out
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

// recordsChange is a request to keep the open folder's records somewhere
// else, worked out: the folder, where its records are, where they would go,
// and the settings that would say so.
type recordsChange struct {
	target   string
	from, to state.Home
	after    settings.Settings
}

// readRecordsChange reads what the page asked for and checks that it can be
// done. Asking and doing both come through here, so nobody is asked a
// question whose answer would then be refused. It writes any refusal itself.
func (s *Server) readRecordsChange(w http.ResponseWriter, r *http.Request) (recordsChange, bool) {
	var req struct {
		Location string `json:"location"`
		Dir      string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return recordsChange{}, false
	}
	choice := settings.CleanRecordsLocation(req.Location)
	if choice == "" {
		writeError(w, http.StatusBadRequest, "Choose where to keep this folder's records.")
		return recordsChange{}, false
	}

	// Nothing may be reading or writing the records while they move: a scan
	// or a download that saved afterwards would save to the old place, and
	// its work would be the part that went missing.
	s.mu.Lock()
	target, busy := s.target, s.busyLocked()
	s.mu.Unlock()
	if target == "" {
		writeError(w, http.StatusBadRequest, "Open a folder first.")
		return recordsChange{}, false
	}
	if busy != "" {
		writeError(w, http.StatusConflict, busy)
		return recordsChange{}, false
	}

	want := settings.Records{Location: choice, Dir: req.Dir}
	if choice == settings.Elsewhere {
		home, err := state.CleanHome(req.Dir)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return recordsChange{}, false
		}
		want.Dir = string(home)
		if under, err := isUnder(target, want.Dir); err == nil && under {
			writeError(w, http.StatusBadRequest,
				"That folder is inside the one isoshelf is looking after, so its records would be on the same drive as the images. Keep them in the folder instead.")
			return recordsChange{}, false
		}
	}

	// Where they are now has to be worked out before the new answer is set:
	// after is a copy of saved, but the answers are a map, and a copied map
	// is the same map. Asked afterwards, saved would already say the new place.
	saved := s.loadSettings()
	from := state.Home(saved.RecordsHome(target, s.cfg.Dirs.Config))
	after := saved
	after.SetRecordsFor(target, want)
	return recordsChange{
		target: target,
		from:   from,
		to:     state.Home(after.RecordsHome(target, s.cfg.Dirs.Config)),
		after:  after,
	}, true
}

// planRecords says what moving the open folder's records would do, and does
// none of it, so the page can ask before anything moves.
func (s *Server) planRecords(w http.ResponseWriter, r *http.Request) {
	c, ok := s.readRecordsChange(w, r)
	if !ok {
		return
	}
	plan, err := state.Plan(c.from, c.to, c.target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't look at this folder's records: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, movedFor(plan))
}

// setRecords moves the open folder's records somewhere else, remembers that
// that is where they now are, and says what it did - including when it moved
// nothing, because records were already waiting where they were going.
func (s *Server) setRecords(w http.ResponseWriter, r *http.Request) {
	c, ok := s.readRecordsChange(w, r)
	if !ok {
		return
	}
	moved, err := state.Move(c.from, c.to, c.target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't move this folder's records: "+err.Error())
		return
	}
	if err := settings.Save(s.cfg.Dirs.Config, c.after); err != nil {
		// The records are at the new place but nothing would look for them
		// there. Put them back rather than leave them stranded.
		state.Move(c.to, c.from, c.target)
		writeError(w, http.StatusInternalServerError, "Couldn't remember where the records went, so they were put back: "+err.Error())
		return
	}

	// Read them from where they now are, so the page shows what isoshelf will
	// actually use from here on.
	st, err := c.to.Load(c.target)
	s.mu.Lock()
	if err == nil && s.target == c.target {
		s.st = st
	}
	s.mu.Unlock()
	out := s.pageState()
	out.Records.Moved = movedFor(moved)
	writeJSON(w, http.StatusOK, out)
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
