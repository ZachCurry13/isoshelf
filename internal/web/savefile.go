package web

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// saveFile hands one image in the folder to the browser, so it can be saved
// on the computer looking at the page: the Download button in each row. On a
// NAS or a Proxmox host that is the way to get an image onto a laptop
// without going back to the project's site.
//
// The path is looked up in the records, which hold what the last scan found,
// and never joined onto the folder on its own say-so - so "../../etc/passwd"
// finds no record and doesn't become a path. Only images isoshelf already
// lists can come out of here.
func (s *Server) saveFile(w http.ResponseWriter, r *http.Request) {
	// Turned off in Settings means turned off here too, not just a button
	// taken off the page.
	if !settings.On(s.loadSettings().AllowDownloads) {
		http.Error(w, "Downloads are turned off in Settings.", http.StatusForbidden)
		return
	}
	rel := r.URL.Query().Get("path")
	s.mu.Lock()
	target, st := s.target, s.st
	s.mu.Unlock()
	if rel == "" || target == "" || st == nil {
		http.NotFound(w, r)
		return
	}
	if _, ok := st.Files[rel]; !ok {
		http.Error(w, "That file isn't in the folder any more. Scan again.", http.StatusNotFound)
		return
	}
	f, err := os.Open(filepath.Join(target, filepath.FromSlash(rel)))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	// Saved under its own name; FormatMediaType spells one with spaces or
	// accents so it survives the trip.
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": info.Name()}))
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}
