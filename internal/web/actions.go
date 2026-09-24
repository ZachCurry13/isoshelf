package web

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// startUpdate queues a download of an entry's newest file. What happens to
// the old files follows the track's "replace old file" setting and the way of
// removing the page asked for. Another download running is no reason to
// refuse: this one waits its turn (see queue.go).
func (s *Server) startUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Entry string `json:"entry"`
		// Removal is "keep", "move-aside" or "delete": what happens to the
		// files the new one replaces.
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
	case entry == nil:
		writeError(w, http.StatusBadRequest, "Unknown image.")
		return
	case s.queuedLocked(entry.ID):
		writeError(w, http.StatusConflict, entry.Name+" is already in the downloads.")
		return
	}

	// The page sends "keep" when the replace switch is off. It asks instead
	// for images whose filename never changes, where keeping both is
	// impossible, and that answer is the one to follow.
	removal := update.Removal(req.Removal)
	if removal != update.Keep && removal != update.MoveAside && removal != update.DeleteNow {
		writeError(w, http.StatusBadRequest, "Say what to do with the old file.")
		return
	}

	// The entry's current files, so they can be replaced afterwards.
	var old []string
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Path != "" && it.Entry != nil && it.Entry.ID == entry.ID {
				old = append(old, it.Path)
			}
		}
	}

	s.nextJob++
	id := s.nextJob
	s.queue = append(s.queue, &job{
		id: id, target: s.target, entry: entry.ID, name: entry.Name,
		size: entry.Size, removal: removal, old: old,
	})
	s.lastErr = ""
	s.startNextLocked()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "id": id})
}

// runUpdate does the download itself, on a state loaded fresh from disk so
// the page can keep reading the current one.
func (s *Server) runUpdate(ctx context.Context, j *job) (string, error) {
	st, err := s.recordsFor(j.target).Load(j.target)
	if err != nil {
		return "", err
	}
	base := st.Clone()
	client := s.client()
	fetcher := fetch.New(s.cfg.Version)
	fetcher.HTTP = s.cfg.HTTP
	fetcher.GitHubToken = s.cfg.GitHubToken

	// Another isoshelf on the network, when one is set up: it may already
	// hold this file, and a minute over the LAN beats an hour over the wire.
	// The fetcher needs its session to fetch from it, and a cookie jar only
	// gives cookies to the host they came from, so the project's own site
	// never sees it.
	near := s.peerFor(j.target)
	if near != nil {
		fetcher.HTTP = withJar(s.cfg.HTTP, near.HTTP.Jar)
		fetcher.HostHeaders = map[string]map[string]string{near.Address: near.Headers()}
	}

	res, err := update.Run(ctx, update.Options{
		Target: j.target, Entry: s.catalog().Entry(j.entry), Client: client, Fetcher: fetcher,
		State: st, Old: j.old, Removal: j.removal, Now: s.cfg.Now,
		Pinned: pinnedAmong(st, j.old),
		// Somewhere closer than the internet, when one is set up.
		Nearer: nearer(near, s.noteAboutPeer),
		Progress: func(p fetch.Progress) {
			// Where the bytes are coming from, worked out here rather than
			// in the page: only this side knows which peer was offered, and
			// a URL is not something to ask somebody to read.
			from := sourceName(p.URL, near)
			s.mu.Lock()
			if s.downloading != nil {
				s.downloading.progress = inventory.Progress{
					Stage: inventory.Stage(p.Stage), File: p.Filename, Done: p.Done, Total: p.Total,
				}
				// An empty URL means this stage doesn't know; the last place
				// that did is still where the file is coming from.
				if from != "" {
					s.downloading.from = from
				}
			}
			s.mu.Unlock()
		},
	})

	// While this downloaded, other changes may have reached the state file:
	// files removed or identified, stars. Only this download's own changes
	// go on top of them (see statefile.go).
	s.mu.Lock()
	saveErr := s.saveMerged(j.target, base, st)
	if s.st != nil && s.target == j.target {
		state.Merge(base, st, s.st)
	}
	s.mu.Unlock()
	if err == nil {
		err = saveErr
	}
	return unverifiedNote(res, err), err
}

// unverifiedNote explains a download nothing could check, which is why the
// old file is still there.
func unverifiedNote(res *update.Result, err error) string {
	switch {
	case err != nil || res == nil || res.Verified:
		return ""
	case len(res.Kept) > 0:
		return "This project publishes no checksum, so isoshelf couldn't verify the download. Your old file was kept - remove it once you're happy with the new one."
	default:
		return "This project publishes no checksum, so isoshelf couldn't verify the download."
	}
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
	case s.scanningLocked() != "":
		writeError(w, http.StatusConflict, s.scanningLocked())
		return
	case len(req.Paths) == 0:
		writeError(w, http.StatusBadRequest, "Nothing to remove.")
		return
	}
	for _, p := range req.Paths {
		if s.updatingLocked(p) {
			writeError(w, http.StatusConflict, p+" is being replaced by the download running now. Wait for it to finish.")
			return
		}
	}

	base := s.st.Clone()
	removed, err := update.Remove(s.target, req.Paths, update.Removal(req.How), s.st, s.cat, s.cfg.Now())
	if saveErr := s.saveStateLocked(base); err == nil {
		err = saveErr
	}
	if len(removed) > 0 {
		// isoshelf changed the folder itself just now, so this is not a
		// change worth offering a scan for: the report is kept in step below.
		s.updatedAt = s.cfg.Now()
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
	switch {
	case s.target == "":
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	case s.busyLocked() != "":
		// A download can be archiving an old file into that folder right now.
		writeError(w, http.StatusConflict, s.busyLocked())
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
func (s *Server) recentTargetsLocked() []rememberedJSON {
	return s.recentTargets()
}
