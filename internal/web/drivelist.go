package web

import (
	"mime"
	"net/http"
	"path/filepath"
	"slices"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/drivelist"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

// A drive's list (#11, #12): saved as a file to keep somewhere else, and a
// folder filled from a list - a remembered folder's, or a saved file - by
// downloading every image on it that isn't here, through the ordinary
// downloads. The backup is the list, not the files: everything on it is
// still on its project's own server. See fill.go for the filling.

// saveList offers this folder's list as a file.
func (s *Server) saveList(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.target == "" || s.st == nil {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	}
	l := s.listLocked()
	s.mu.Unlock()
	data, err := drivelist.Encode(l)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't write the list ("+err.Error()+").")
		return
	}
	name := "isoshelf-list-" + filepath.Base(l.Folder) + ".toml"
	w.Header().Set("Content-Type", "application/toml; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Write(data)
}

// listLocked is this folder's list: every image the last scan found that
// the catalog knows, and the ones it usually carries that are missing now.
// s.mu must be held.
func (s *Server) listLocked() drivelist.List {
	l := drivelist.List{Folder: s.target, Made: s.cfg.Now().UTC()}
	seen := map[string]bool{}
	if s.report != nil {
		for _, it := range s.report.Items {
			if it.Entry == nil || it.Path == "" || it.Older || seen[it.Entry.ID] {
				continue
			}
			seen[it.Entry.ID] = true
			l.Images = append(l.Images, drivelist.Image{
				ID: it.Entry.ID, Name: it.Entry.Name, Version: it.Version, File: filepath.ToSlash(it.Path),
				Starred: s.st.Track(it.Entry.ID).Starred,
			})
		}
	}
	for _, id := range s.st.UsualSet() {
		if !seen[id] {
			l.Images = append(l.Images, s.listImage(id, s.st.Tracks))
		}
	}
	return l
}

// listImage names an image the list only has an id for.
func (s *Server) listImage(id string, tracks map[string]state.Track) drivelist.Image {
	img := drivelist.Image{ID: id, Starred: tracks[id].Starred}
	if e := s.cat.Entry(id); e != nil {
		img.Name = e.Name
	}
	return img
}

// mirrorListLocked is the list of a folder isoshelf remembers, from the
// copy of its records kept on this computer: its usual set, which is what
// it carried, whether or not the drive still exists. s.mu must be held.
func (s *Server) mirrorListLocked(id string) (drivelist.List, bool) {
	if s.cfg.Dirs.Portable || s.cfg.Dirs.Config == "" {
		return drivelist.List{}, false
	}
	mirrors, err := state.LoadMirrors(s.cfg.Dirs.Config)
	if err != nil {
		return drivelist.List{}, false
	}
	i := slices.IndexFunc(mirrors, func(m state.Mirror) bool { return m.TargetID == id })
	if i < 0 {
		return drivelist.List{}, false
	}
	m := mirrors[i]
	l := drivelist.List{Folder: m.Path, Made: m.SavedAt}
	// What it held at its last scan, and what it usually carries: a drive
	// scanned once has no usual set yet, and its last scan is what was lost.
	ids := slices.Clone(m.UsualSet())
	if len(m.History) > 0 {
		ids = append(ids, m.History[len(m.History)-1].Entries...)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		if m.Tracks[id].NotExpected {
			continue
		}
		l.Images = append(l.Images, s.listImage(id, m.Tracks))
	}
	return l, len(l.Images) > 0
}

// why says, in words, why isoshelf can't download an image on a list.
func why(e *catalog.Entry) string {
	switch {
	case e == nil:
		return "isoshelf's list of images doesn't know it"
	case e.Updates() == catalog.UpdatesManual:
		return "isoshelf only links to its download page"
	case e.Updates() == catalog.UpdatesCheckOnly:
		return "isoshelf checks its version but can't download it"
	}
	return ""
}
