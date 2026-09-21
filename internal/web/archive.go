package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// archiveItemJSON is one image that used to be in the folder.
type archiveItemJSON struct {
	Path    string    `json:"path"`
	Entry   string    `json:"entry,omitempty"`
	Name    string    `json:"name"`
	Version string    `json:"version,omitempty"`
	Size    int64     `json:"size"`
	Gone    string    `json:"gone"`
	GoneAt  time.Time `json:"gone_at"`
	// OnDisk is true while the file itself still waits in .isoshelf/removed,
	// using room. That is what tells the archive from the history: the
	// history is only a record of images that have left.
	OnDisk bool `json:"on_disk"`
	// Restorable is true when it can go back now. A file whose name has been
	// taken by the one that replaced it is still in the archive, still uses
	// room and can still be emptied - it just can't be put back until that
	// name is free.
	Restorable bool `json:"restorable"`
	// Downloadable is true when isoshelf can fetch this image again.
	Downloadable bool   `json:"downloadable"`
	Page         string `json:"page,omitempty"`
	Icon         string `json:"icon,omitempty"`
	IconColor    string `json:"icon_color,omitempty"`
}

// getArchive lists the images that have left this folder.
func (s *Server) getArchive(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := []archiveItemJSON{}
	if s.st == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": items})
		return
	}
	waiting, _, _ := update.Removed(s.target)
	noted := map[string]bool{}
	// A file can only go back if its name is free: restore won't put it on
	// top of whatever is there now, so the page mustn't offer to.
	free := func(name string) bool {
		_, err := os.Lstat(filepath.Join(s.target, name))
		return err != nil
	}

	for _, past := range s.st.Archive() {
		noted[path.Base(past.Path)] = true
		item := archiveItemJSON{
			Path: past.Path, Entry: past.Entry, Name: past.Path, Version: past.Version,
			Size: past.Size, Gone: past.Gone, GoneAt: past.GoneAt,
			OnDisk: past.Gone == state.GoneMovedAside && slices.Contains(waiting, path.Base(past.Path)),
		}
		item.Restorable = item.OnDisk && free(path.Base(past.Path))
		if e := s.cat.Entry(past.Entry); e != nil {
			item.Name = e.Name
			item.Page, item.Icon, item.IconColor = e.Page, e.Icon, e.IconColor
			item.Downloadable = e.Updates() == "download"
		}
		items = append(items, item)
	}

	// Files waiting in .isoshelf/removed that no note mentions. A file
	// replaced by one of the same name loses its note at the next scan,
	// because that path is in the folder again - but the old file is still on
	// the disk, still using room, and still the one thing the archive exists
	// to let someone undo. Listing what is actually there keeps the archive
	// honest about both.
	for _, name := range waiting {
		if noted[name] {
			continue
		}
		item := archiveItemJSON{Path: name, Name: name, Gone: state.GoneMovedAside, OnDisk: true, Restorable: free(name)}
		if info, err := os.Stat(filepath.Join(s.target, state.DirName, update.RemovedDir, name)); err == nil {
			item.Size, item.GoneAt = info.Size(), info.ModTime()
		}
		items = append(items, item)
	}
	// Most recent first, however the item got here.
	slices.SortStableFunc(items, func(a, b archiveItemJSON) int { return b.GoneAt.Compare(a.GoneAt) })
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// restore puts a moved-aside file back in the folder.
func (s *Server) restore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
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
	}
	if err := update.Restore(s.target, req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// The page scans after putting a file back, but not while downloads run;
	// the scan that follows them picks it up instead.
	if s.downloading != nil {
		s.placed = true
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restored"})
}
