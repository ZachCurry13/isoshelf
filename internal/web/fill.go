package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ZachCurry13/isoshelf/internal/drivelist"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// fillRequest names the list to fill this folder from: a remembered
// folder's (Folder, its id) or a list file's text (Text). Fill also takes
// the ids to download, which the page shows first and lets people untick.
type fillRequest struct {
	Folder string   `json:"folder"`
	Text   string   `json:"text"`
	IDs    []string `json:"ids"`
}

type fillItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size,omitempty"`
	Why  string `json:"why,omitempty"`
}

// fillPlan is what filling from a list would do.
type fillPlan struct {
	From string `json:"from"`
	// Download is what isoshelf would fetch; Here what this folder already
	// has (or has queued); Cannot what it can't fetch, each with why.
	Download []fillItem `json:"download"`
	Here     int        `json:"here"`
	Cannot   []fillItem `json:"cannot"`
	// Bytes is about how much Download comes to, Free the room here, and
	// Fits whether it does, keeping a little spare.
	Bytes int64 `json:"bytes"`
	Free  int64 `json:"free,omitempty"`
	Fits  bool  `json:"fits"`
}

// fillPlanLocked works a list out against this folder. s.mu must be held.
func (s *Server) fillPlanLocked(req fillRequest) (*fillPlan, string) {
	if s.target == "" || s.st == nil {
		return nil, "Choose a folder first."
	}
	var l drivelist.List
	switch {
	case req.Folder != "":
		var ok bool
		if l, ok = s.mirrorListLocked(req.Folder); !ok {
			return nil, "isoshelf doesn't remember which images that folder had."
		}
	case req.Text != "":
		var err error
		if l, err = drivelist.Decode([]byte(req.Text)); err != nil {
			return nil, err.Error() + ". Choose the file isoshelf saved for you."
		}
	default:
		return nil, "Choose a folder or a list to fill this one from."
	}
	here := map[string]bool{}
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Entry != nil && it.Path != "" {
				here[it.Entry.ID] = true
			}
		}
	}
	p := &fillPlan{From: l.Folder, Download: []fillItem{}, Cannot: []fillItem{}}
	for _, img := range l.Images {
		e := s.cat.Entry(img.ID)
		name := img.Name
		if e != nil {
			name = e.Name
		}
		if name == "" {
			name = img.ID
		}
		switch {
		case e != nil && (here[e.ID] || s.queuedLocked(e.ID)):
			p.Here++
		case why(e) != "":
			p.Cannot = append(p.Cannot, fillItem{ID: img.ID, Name: name, Why: why(e)})
		default:
			p.Download = append(p.Download, fillItem{ID: e.ID, Name: name, Size: e.Size})
			p.Bytes += e.Size
		}
	}
	p.Fits = s.room.Fits(p.Bytes)
	if s.room.Known() {
		p.Free = s.room.Free
	}
	return p, ""
}

func (s *Server) planFill(w http.ResponseWriter, r *http.Request) {
	var req fillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "That request wasn't understood. Reload the page and try again.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, msg := s.fillPlanLocked(req)
	if p == nil {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// fill queues the chosen images from the plan, and stars the ones the list
// had starred, so this folder's usual set carries on where the old one left
// off. Room comes first: a fill that can't fit is refused before it starts,
// not discovered half way.
func (s *Server) fill(w http.ResponseWriter, r *http.Request) {
	var req fillRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "That request wasn't understood. Reload the page and try again.")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if msg := s.scanningLocked(); msg != "" {
		writeError(w, http.StatusConflict, msg)
		return
	}
	p, msg := s.fillPlanLocked(req)
	if p == nil {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	chosen := map[string]bool{}
	for _, id := range req.IDs {
		chosen[id] = true
	}
	var queue []fillItem
	var bytes int64
	for _, it := range p.Download {
		if chosen[it.ID] {
			queue = append(queue, it)
			bytes += it.Size
		}
	}
	if !s.room.Fits(bytes) {
		writeError(w, http.StatusConflict, "Those images need about "+formatSize(bytes)+" and this folder has "+formatSize(s.room.Free)+" free. Untick some, or make room, then try again.")
		return
	}
	base := s.st.Clone()
	if l, _ := s.listForStarsLocked(req); l != nil {
		for _, img := range l.Images {
			if t := s.st.Track(img.ID); img.Starred && !t.Starred && s.cat.Entry(img.ID) != nil {
				t.Starred = true
				s.st.SetTrack(img.ID, t)
			}
		}
	}
	for _, it := range queue {
		s.nextJob++
		s.queue = append(s.queue, &job{
			id: s.nextJob, target: s.target, entry: it.ID, name: it.Name, size: it.Size, removal: update.Keep,
		})
	}
	if err := s.saveStateLocked(base); err != nil {
		writeError(w, http.StatusInternalServerError, "The downloads are queued, but the stars couldn't be saved ("+err.Error()+").")
		return
	}
	s.lastErr = ""
	s.startNextLocked()
	writeJSON(w, http.StatusAccepted, map[string]any{"queued": len(queue)})
}

// listForStarsLocked reads the request's list again for its stars.
func (s *Server) listForStarsLocked(req fillRequest) (*drivelist.List, bool) {
	if req.Folder != "" {
		l, ok := s.mirrorListLocked(req.Folder)
		return &l, ok
	}
	l, err := drivelist.Decode([]byte(req.Text))
	return &l, err == nil
}

// formatSize is a size the way the page writes one.
func formatSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%d MB", b>>20)
	}
	return fmt.Sprintf("%d KB", b>>10)
}
