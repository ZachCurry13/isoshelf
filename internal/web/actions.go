package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// startUpdate downloads an entry's newest file and puts it in place. What
// happens to the old files follows the track's "replace old file" setting and
// the way of removing the page asked for.
func (s *Server) startUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entry string `json:"entry"`
		// Removal is "move-aside" or "delete"; ignored when the track keeps
		// old files.
		Removal string `json:"removal"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.cat.Entry(req.Entry)
	switch {
	case s.target == "" || s.st == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.run != nil:
		writeError(w, http.StatusConflict, "Something is already running.")
		return
	case entry == nil:
		writeError(w, http.StatusBadRequest, "Unknown image.")
		return
	}

	removal := update.Removal(req.Removal)
	if s.st.Track(entry.ID).KeepOld {
		removal = update.Keep
	}
	if removal != update.Keep && removal != update.MoveAside && removal != update.DeleteNow {
		writeError(w, http.StatusBadRequest, "Say what to do with the old file.")
		return
	}

	// The entry's current files, so they can be replaced afterwards.
	var old []string
	checked := false
	if s.report != nil {
		checked = s.report.Checked
		for _, it := range s.report.Items {
			if it.Path != "" && it.Entry != nil && it.Entry.ID == entry.ID {
				old = append(old, it.Path)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.run = &run{kind: "update", started: s.cfg.Now(), cancel: cancel,
		progress: inventory.Progress{Stage: inventory.Stage(fetch.Downloading), File: entry.Name}}
	// The filename replaces the entry name as soon as the download starts.
	s.lastErr = ""
	target := s.target
	go s.executeUpdate(ctx, target, entry.ID, removal, old, checked)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

func (s *Server) executeUpdate(ctx context.Context, target, entryID string, removal update.Removal, old []string, checked bool) {
	err := s.runUpdate(ctx, target, entryID, removal, old)

	// Whatever happened, take a fresh look at the folder so the page shows
	// what is really there now.
	if ctx.Err() == nil {
		client := remote.New(s.cfg.Version)
		client.HTTP = s.cfg.HTTP
		client.GitHubToken = s.cfg.GitHubToken
		res, runErr := inventory.Run(ctx, inventory.Options{
			Target: target, Online: checked, Client: client, Catalog: s.catalog(),
			Dirs: s.cfg.Dirs, Now: s.cfg.Now,
			Progress: func(p inventory.Progress) {
				s.mu.Lock()
				if s.run != nil {
					s.run.progress = p
				}
				s.mu.Unlock()
			},
		})
		s.mu.Lock()
		if res != nil && s.target == target {
			s.report, s.st, s.warnings, s.updatedAt = res.Report, res.State, res.Warnings, s.cfg.Now()
		}
		if err == nil && runErr != nil {
			err = runErr
		}
		s.mu.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.run = nil
	switch {
	case errors.Is(err, context.Canceled):
		s.lastErr = "The update was stopped. A part-finished download is kept, so it can carry on later."
	case err != nil:
		s.lastErr = err.Error()
	}
}

// runUpdate does the download itself, on a state loaded fresh from disk so
// the page can keep reading the current one.
func (s *Server) runUpdate(ctx context.Context, target, entryID string, removal update.Removal, old []string) error {
	st, err := state.Load(target)
	if err != nil {
		return err
	}
	client := s.client()
	fetcher := fetch.New(s.cfg.Version)
	fetcher.HTTP = s.cfg.HTTP
	fetcher.GitHubToken = s.cfg.GitHubToken

	_, err = update.Run(ctx, update.Options{
		Target: target, Entry: s.catalog().Entry(entryID), Client: client, Fetcher: fetcher,
		State: st, Old: old, Removal: removal, Now: s.cfg.Now,
		Progress: func(p fetch.Progress) {
			s.mu.Lock()
			if s.run != nil {
				s.run.progress = inventory.Progress{
					Stage: inventory.Stage(p.Stage), File: p.Filename, Done: p.Done, Total: p.Total,
				}
			}
			s.mu.Unlock()
		},
	})
	if saveErr := st.Save(target); err == nil {
		err = saveErr
	}
	return err
}

// remove moves files aside or deletes them, after the page has asked.
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
		How   string   `json:"how"` // "move-aside" or "delete"
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
	case len(req.Paths) == 0:
		writeError(w, http.StatusBadRequest, "Nothing to remove.")
		return
	}

	removed, err := update.Remove(s.target, req.Paths, update.Removal(req.How), s.st, s.cat, s.cfg.Now())
	if saveErr := s.st.Save(s.target); err == nil {
		err = saveErr
	}
	if len(removed) > 0 && s.report != nil {
		gone := map[string]bool{}
		for _, p := range removed {
			gone[p] = true
		}
		items := s.report.Items[:0]
		for _, it := range s.report.Items {
			if !gone[it.Path] {
				items = append(items, it)
			}
		}
		s.report.Items = items
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.stateLocked(s.recentTargetsLocked(), s.room))
}

// emptyRemoved deletes everything waiting in .isoshelf/removed.
func (s *Server) emptyRemoved(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.target == "" {
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	}
	if _, err := update.EmptyRemoved(s.target); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.stateLocked(s.recentTargetsLocked(), s.room))
}

// recentTargetsLocked is recentTargets for callers that already hold the lock;
// it only reads the config folder.
func (s *Server) recentTargetsLocked() []string {
	return s.recentTargets()
}
