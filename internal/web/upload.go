package web

import (
	"errors"
	"net/http"
	"path"

	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
	"github.com/ZachCurry13/isoshelf/internal/upload"
)

// Adding a file from the user's own computer. The request body is the file
// itself rather than a form, which keeps it streaming straight to the disk:
// these are images, and one of them can be eight gigabytes.
//
//	POST /api/upload?name=<filename>&replace=<move-aside|delete>
//
// The work itself is internal/upload's. What belongs here is the same
// state-file discipline a download keeps: the records are loaded fresh, the
// upload changes its own copy, and only that change is carried onto the file
// as it is on disk when it finishes, so whatever else happened meanwhile is
// still there.
func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	target := s.target
	if target == "" {
		s.mu.Unlock()
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	}
	// The folder can't be switched out from under a file that is arriving.
	s.uploads++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.uploads--
		s.mu.Unlock()
	}()

	st, err := s.recordsFor(target).Load(target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	base := st.Clone()

	res, err := upload.Place(r.Context(), r.Body, upload.Options{
		Target:  target,
		Name:    r.URL.Query().Get("name"),
		Profile: st.Profile,
		Size:    r.ContentLength,
		Room:    s.targetSpace(),
		Replace: update.Removal(r.URL.Query().Get("replace")),
		State:   st,
		Now:     s.cfg.Now,
	})
	if err != nil {
		uploadError(w, err, r.URL.Query().Get("name"))
		return
	}

	// The file is in place. Save this upload's own change onto the records as
	// they are now, and let the page's copy know too.
	s.mu.Lock()
	saveErr := s.saveMerged(target, base, st)
	if s.st != nil && s.target == target {
		state.Merge(base, st, s.st)
	}
	// A new file wants the same look a copied-in one gets: what is it, and
	// does the catalog know it. The scan does that, once nothing else is
	// running.
	s.placed = true
	s.lastErr = ""
	s.startNextLocked()
	s.mu.Unlock()

	if saveErr != nil {
		writeError(w, http.StatusInternalServerError, "The file is in the folder, but isoshelf couldn't write down what it knows: "+saveErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "added",
		"name":     res.Name,
		"size":     res.Size,
		"replaced": res.Replaced,
	})
}

// uploadError turns the refusals into words and a status the page can act on.
func uploadError(w http.ResponseWriter, err error, name string) {
	base := path.Base(name)
	switch {
	case errors.Is(err, upload.ErrExists):
		// Not a failure so much as a question, like the one a download asks:
		// the page offers the two answers rather than a dead end.
		writeJSON(w, http.StatusConflict, map[string]any{
			"error": "There's already a file called " + base + " here. Say what should happen to the one you have. " +
				"Nothing in the folder has changed.",
			"conflict": true,
			"name":     base,
		})
	case errors.Is(err, upload.ErrNotAnImage):
		writeError(w, http.StatusBadRequest,
			base+" isn't an image file, so isoshelf won't put it in this folder. Disk images and ISOs only.")
	case errors.Is(err, upload.ErrNoRoom):
		writeError(w, http.StatusBadRequest,
			"There isn't room in this folder for "+base+". Clear some space and try again.")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
