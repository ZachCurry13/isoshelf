package web

import (
	"net/http"
	"path"
	"slices"
	"strconv"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// The History section (v0.8.13): what arrived in the folder, what an update
// changed, and what left, newest first, with when and how big. It replaced
// the week-by-week Usage section. It records nothing of its own: it is worked
// out from what the folder's records already hold (state.Events), so it can't
// disagree with them.

// eventJSON is one thing that happened, with what the page needs to draw it
// the way every other list of images is drawn.
type eventJSON struct {
	state.Event
	Name      string `json:"name"`
	Icon      string `json:"icon,omitempty"`
	IconColor string `json:"icon_color,omitempty"`
	// Downloadable is true when isoshelf can fetch this image again, and Page
	// is where its project publishes it.
	Downloadable bool   `json:"downloadable"`
	Page         string `json:"page,omitempty"`
	// Waiting is true while an archived file still sits in
	// .isoshelf/removed, where the Archive section can put it back.
	Waiting bool `json:"waiting,omitempty"`
}

// getHistory lists the events of the last ?days= days, or all of them when
// days is missing or 0. It is asked for again only when something changed,
// because it reads every record the folder has.
func (s *Server) getHistory(w http.ResponseWriter, r *http.Request) {
	var since time.Time
	if days, _ := strconv.Atoi(r.URL.Query().Get("days")); days > 0 {
		since = s.cfg.Now().AddDate(0, 0, -days)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	events := []eventJSON{}
	if s.st != nil {
		waiting, _, _ := update.Removed(s.target)
		for _, e := range s.st.Events(since) {
			item := eventJSON{
				Event: e, Name: e.Path,
				Waiting: e.Kind == state.EventArchived && slices.Contains(waiting, path.Base(e.Path)),
			}
			if entry := s.cat.Entry(e.Entry); entry != nil {
				item.Name = entry.Name
				item.Page, item.Icon, item.IconColor = entry.Page, entry.Icon, entry.IconColor
				item.Downloadable = entry.Updates() == "download"
			}
			events = append(events, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}
