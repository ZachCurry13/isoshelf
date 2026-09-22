package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/update"
)

// The download queue. Add and Update never refuse because something else is
// downloading: they join the queue, and downloads run one at a time in the
// queue's order, which the page can change. Once the queue is empty the folder
// is scanned once, so the list shows what arrived.
//
// Invariant: while the queue holds a job, s.downloading is set. Whatever
// clears it calls startNextLocked straight after, under the same lock, so
// there is never a moment with jobs waiting and nothing running. A scan has
// its own slot (s.scanning) and neither waits for the other.

// maxFinished is how many finished downloads the page is shown.
const maxFinished = 20

// job is one download, waiting or running.
type job struct {
	id      int
	target  string
	entry   string
	name    string
	size    int64
	removal update.Removal
	// old holds the entry's files at the time it was queued, replaced once
	// the new one is in place. Empty when the image is being added.
	old []string
}

// finishedJob is a download that ended, and how.
type finishedJob struct {
	job
	outcome string // "done", "failed" or "stopped"
	message string
	// conflict marks the one failure that is really a question: a file of
	// the same name is already there.
	conflict bool
	at       time.Time
}

type downloadsJSON struct {
	Current  *jobJSON  `json:"current,omitempty"`
	Queued   []jobJSON `json:"queued"`
	Finished []jobJSON `json:"finished"`
}

type jobJSON struct {
	ID    int    `json:"id"`
	Entry string `json:"entry"`
	Name  string `json:"name"`
	// Size is roughly how big the download is, from the catalog.
	Size int64 `json:"size,omitempty"`
	// Update is set when the download replaces files already in the folder,
	// rather than adding an image that isn't there.
	Update  bool   `json:"update,omitempty"`
	Outcome string `json:"outcome,omitempty"`
	Message string `json:"message,omitempty"`
	// Conflict marks a download that stopped to ask what happens to the file
	// already in the folder, rather than one that went wrong.
	Conflict bool       `json:"conflict,omitempty"`
	At       *time.Time `json:"at,omitempty"`
	// Stage, Done and Total describe the download running now, and are set
	// only on Current.
	Stage string `json:"stage,omitempty"`
	Done  int64  `json:"done,omitempty"`
	Total int64  `json:"total,omitempty"`
}

func (j *job) json() jobJSON {
	return jobJSON{ID: j.id, Entry: j.entry, Name: j.name, Size: j.size, Update: len(j.old) > 0}
}

// downloadsLocked describes the queue; s.mu must be held.
func (s *Server) downloadsLocked() downloadsJSON {
	out := downloadsJSON{Queued: []jobJSON{}, Finished: []jobJSON{}}
	if s.downloading != nil {
		j := s.downloading.job.json()
		// The dock draws its own progress from these, since the scan's card
		// has the s.scanning slot to itself.
		p := s.downloading.progress
		j.Stage, j.Done, j.Total = string(p.Stage), p.Done, p.Total
		out.Current = &j
	}
	for _, j := range s.queue {
		out.Queued = append(out.Queued, j.json())
	}
	for i := range s.finished {
		f := &s.finished[i]
		j := f.job.json()
		j.Outcome, j.Message, j.Conflict = f.outcome, f.message, f.conflict
		at := f.at
		j.At = &at
		out.Finished = append(out.Finished, j)
	}
	return out
}

// queuedLocked reports whether an entry is waiting or downloading.
func (s *Server) queuedLocked(entry string) bool {
	if s.downloading != nil && s.downloading.job.entry == entry {
		return true
	}
	for _, j := range s.queue {
		if j.entry == entry {
			return true
		}
	}
	return false
}

// busyLocked says why the folder can't be switched and the archive can't be
// emptied right now, or "" when they can: both wait for any scan or download,
// because both pull the ground out from under one. Scanning only waits for
// another scan (scanBusyLocked), and changes to single files only for a scan
// (scanningLocked).
func (s *Server) busyLocked() string {
	switch {
	case s.scanning != nil:
		return "Wait until the scan finishes, or stop it."
	case s.downloading != nil || len(s.queue) > 0:
		return "Wait until the downloads finish, or stop them."
	case s.uploads > 0:
		return "Wait until the file you're adding has finished arriving."
	default:
		return ""
	}
}

// scanBusyLocked says why a scan can't start, or "" when it can. Downloads
// are no longer a reason: they write one file each through the state file's
// merge, and a scan saves the same way, so the two run side by side.
func (s *Server) scanBusyLocked() string {
	if s.scanning != nil {
		return "Wait until the scan finishes, or stop it."
	}
	return ""
}

// startNextLocked starts the next download if nothing is running. When the
// queue has just run dry and something arrived, it scans the folder instead,
// so the list catches up with what is there. s.mu must be held.
func (s *Server) startNextLocked() {
	if s.downloading != nil {
		return
	}
	if len(s.queue) == 0 {
		// The scan that catches the list up waits for a scan already running,
		// and s.placed stays set so that one's ending starts it.
		if s.placed && s.scanning == nil && s.target != "" && s.st != nil {
			s.placed = false
			// The same rule as any other scan: check for updates unless
			// Settings says not to. The answers are moments old, so this
			// costs nothing.
			s.startScanLocked(askIfDue)
		}
		return
	}
	j := s.queue[0]
	s.queue = s.queue[1:]
	ctx, cancel := context.WithCancel(context.Background())
	s.downloading = &run{kind: "update", job: j, started: s.cfg.Now(), cancel: cancel}
	go s.executeJob(ctx, j)
}

// executeJob runs one download and records how it went.
func (s *Server) executeJob(ctx context.Context, j *job) {
	note, err := s.runJob(ctx, j)

	s.mu.Lock()
	defer s.mu.Unlock()
	f := finishedJob{job: *j, outcome: "done", message: note, at: s.cfg.Now()}
	switch {
	case errors.Is(err, context.Canceled):
		f.outcome = "stopped"
		f.message = "Stopped. What downloaded so far is kept, so Resume picks up from there."
	case errors.Is(err, update.ErrSameName):
		// Not a failure so much as a question: the page offers the answers
		// rather than leaving a message nobody can act on.
		f.outcome = "failed"
		f.conflict = true
		f.message = j.name + " always uses the same filename, and a file of that name is already here. " +
			"Choose what happens to it. Nothing in the folder has changed."
	case err != nil:
		f.outcome = "failed"
		f.message = err.Error()
		s.lastErr = j.name + ": " + err.Error()
	default:
		s.placed = true
	}
	s.finished = append([]finishedJob{f}, s.finished...)
	if len(s.finished) > maxFinished {
		s.finished = s.finished[:maxFinished]
	}
	s.downloading = nil
	s.startNextLocked()
}

// moveQueued changes a waiting download's place in the queue.
func (s *Server) moveQueued(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID       int `json:"id"`
		Position int `json:"position"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	from := s.queueIndexLocked(req.ID)
	if from < 0 {
		writeError(w, http.StatusBadRequest, "That download isn't waiting any more.")
		return
	}
	to := min(max(req.Position, 0), len(s.queue)-1)
	j := s.queue[from]
	s.queue = append(s.queue[:from], s.queue[from+1:]...)
	s.queue = append(s.queue[:to], append([]*job{j}, s.queue[to:]...)...)
	writeJSON(w, http.StatusOK, s.downloadsLocked())
}

// dropQueued takes a download out of the queue. The one downloading now is
// stopped, and the next one starts.
func (s *Server) dropQueued(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.downloading != nil && s.downloading.job.id == req.ID {
		s.downloading.cancel()
		writeJSON(w, http.StatusOK, s.downloadsLocked())
		return
	}
	i := s.queueIndexLocked(req.ID)
	if i < 0 {
		writeError(w, http.StatusBadRequest, "That download isn't waiting any more.")
		return
	}
	s.queue = append(s.queue[:i], s.queue[i+1:]...)
	writeJSON(w, http.StatusOK, s.downloadsLocked())
}

// clearFinished forgets the downloads that have ended.
func (s *Server) clearFinished(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.finished = nil
	writeJSON(w, http.StatusOK, s.downloadsLocked())
}

func (s *Server) queueIndexLocked(id int) int {
	for i, j := range s.queue {
		if j.id == id {
			return i
		}
	}
	return -1
}
