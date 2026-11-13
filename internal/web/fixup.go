package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// Make bootable (#3): the page asks what the fix for a file would be
// (/api/fixup/plan), shows it, and asks for it to be done (/api/fixup). A
// rename is done at once. An extract can take minutes, so it waits its turn
// in the downloads, where its progress shows like a download's; it only
// writes to this disk, so it runs beside downloads rather than behind them.

type fixRequest struct {
	Path string `json:"path"`
	// Original is "move-aside" or "keep": what happens to the compressed
	// file once its image is out.
	Original string `json:"original"`
}

// planFixLocked works out the fix for a file of the report. s.mu must be
// held.
func (s *Server) planFixLocked(rel string) (*update.Fix, *catalog.Entry, int, string) {
	switch {
	case s.target == "" || s.st == nil:
		return nil, nil, http.StatusBadRequest, "Choose a folder first."
	case s.scanningLocked() != "":
		return nil, nil, http.StatusConflict, s.scanningLocked()
	}
	var entry *catalog.Entry
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Path == rel && it.Entry != nil {
				entry = s.cat.Entry(it.Entry.ID)
			}
		}
	}
	if entry == nil {
		return nil, nil, http.StatusBadRequest, "isoshelf doesn't know what " + path.Base(rel) + " is, so it doesn't know how to make it bootable."
	}
	if s.queuedLocked(entry.ID) {
		return nil, nil, http.StatusConflict, entry.Name + " is already in the downloads."
	}
	fix, err := update.PlanFix(s.target, rel, entry)
	if err != nil {
		return nil, nil, http.StatusConflict, err.Error()
	}
	return fix, entry, 0, ""
}

func (s *Server) planFix(w http.ResponseWriter, r *http.Request) {
	var req fixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "That request wasn't understood. Reload the page and try again.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fix, _, code, msg := s.planFixLocked(req.Path)
	if fix == nil {
		writeError(w, code, msg)
		return
	}
	writeJSON(w, http.StatusOK, fix)
}

func (s *Server) doFix(w http.ResponseWriter, r *http.Request) {
	var req fixRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "That request wasn't understood. Reload the page and try again.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fix, entry, code, msg := s.planFixLocked(req.Path)
	if fix == nil {
		writeError(w, code, msg)
		return
	}
	if fix.Kind == "rename" {
		base := s.st.Clone()
		if err := update.ApplyFix(context.Background(), s.target, fix, "", s.st, s.cfg.Now(), nil); err != nil {
			writeError(w, http.StatusConflict, "Couldn't rename it ("+err.Error()+"). Nothing changed.")
			return
		}
		if err := s.saveStateLocked(base); err != nil {
			writeError(w, http.StatusInternalServerError, "Renamed, but the folder's records couldn't be saved ("+err.Error()+"). Scan the folder again.")
			return
		}
		s.startScanLocked(askIfDue)
		writeJSON(w, http.StatusOK, map[string]any{"status": "renamed", "to": fix.To})
		return
	}
	original := update.Removal(req.Original)
	if original != update.MoveAside && original != update.Keep {
		writeError(w, http.StatusBadRequest, "Say what to do with the compressed file.")
		return
	}
	s.nextJob++
	id := s.nextJob
	s.queue = append(s.queue, &job{
		id: id, target: s.target, entry: entry.ID, name: "Unpacking " + path.Base(fix.From),
		size: fix.Size, removal: original, fix: fix,
	})
	s.startNextLocked()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "id": id})
}

// runFix unpacks, on a state loaded fresh from disk like a download's.
func (s *Server) runFix(ctx context.Context, j *job) (string, error) {
	st, err := s.recordsFor(j.target).Load(j.target)
	if err != nil {
		return "", err
	}
	base := st.Clone()
	err = update.ApplyFix(ctx, j.target, j.fix, j.removal, st, s.cfg.Now(), func(done, total int64) {
		s.mu.Lock()
		if r := j.run; r != nil {
			r.note(inventory.Progress{Stage: "unpacking", File: j.fix.To, Done: done, Total: total}, s.cfg.Now())
		}
		s.mu.Unlock()
	})
	if err != nil && !errors.Is(err, context.Canceled) && st.Files[j.fix.To].Entry == "" {
		return "", err
	}
	s.mu.Lock()
	saveErr := s.saveMerged(j.target, base, st)
	if s.st != nil && s.target == j.target {
		state.Merge(base, st, s.st)
	}
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	if j.removal == update.MoveAside {
		return path.Base(j.fix.To) + " is ready to boot. The compressed file is in the archive, where you can restore it or empty it.", saveErr
	}
	return path.Base(j.fix.To) + " is ready to boot. The compressed file is still beside it.", saveErr
}
