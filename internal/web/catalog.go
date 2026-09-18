package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/catupdate"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/remote"
	"github.com/ZachCurry13/isoshelf/internal/usercat"
)

// Where the catalog in use came from.
const (
	catalogBuiltIn    = catupdate.SourceBuiltIn
	catalogDownloaded = catupdate.SourceDownloaded
	catalogOwn        = catupdate.SourceOwn
)

// catalogStatusJSON is what the page shows about the catalog itself.
type catalogStatusJSON struct {
	Entries int `json:"entries"`
	// Mine is how many of those the user named themselves.
	Mine   int    `json:"mine"`
	Source string `json:"source"`
	// Auto is whether isoshelf keeps it up to date; CanAuto is false when
	// the user supplied their own catalog, which isoshelf never overwrites.
	Auto      bool       `json:"auto"`
	CanAuto   bool       `json:"can_auto"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
	// Note is what the last update changed, and Error why one didn't happen.
	Note  string `json:"note,omitempty"`
	Error string `json:"error,omitempty"`
}

// catalog returns the catalog in use. It can be replaced while isoshelf runs.
func (s *Server) catalog() *catalog.Catalog {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cat
}

// catalogAuto reports whether isoshelf may keep the catalog up to date. The
// default is yes; a catalog the user supplied is never replaced.
func (s *Server) catalogAuto() bool {
	if s.ownCatalog() || s.cfg.Dirs.Config == "" {
		return false
	}
	if v := s.loadSettings().CatalogAuto; v != nil {
		return *v
	}
	return true
}

// catalogStatusLocked builds the catalog part of the page state; s.mu must be
// held.
func (s *Server) catalogStatusLocked() catalogStatusJSON {
	out := catalogStatusJSON{
		Entries: len(s.cat.Entries),
		Mine:    countMine(s.cat),
		Source:  s.catSource,
		CanAuto: !s.ownCatalog() && s.cfg.Dirs.Config != "",
		Note:    s.catNote,
		Error:   s.catErr,
	}
	out.Auto = out.CanAuto && s.catalogAuto()
	if at := catupdate.Downloaded(s.cfg.Dirs.Config); !at.IsZero() {
		out.UpdatedAt = &at
	}
	return out
}

// refreshCatalog asks the project for a newer catalog. force skips the
// once-a-day wait, for when the user presses the button.
func (s *Server) refreshCatalog(ctx context.Context, force bool) {
	if s.ownCatalog() || s.cfg.Dirs.Config == "" {
		return
	}
	if !force && !s.catalogAuto() {
		return
	}
	result, err := catupdate.Refresh(ctx, s.client(), s.cfg.Dirs.Config, s.cfg.Now(), force)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.catErr = err.Error()
		return
	}
	s.catErr = ""
	if result == nil {
		return // not time to ask yet
	}
	if !result.Changed {
		if force {
			s.catNote = "The catalog is already up to date."
		}
		return
	}
	fetched, err := catupdate.Load(s.cfg.Dirs.Config)
	if err != nil || fetched == nil {
		s.catErr = "The new catalog couldn't be read, so the one you have is kept."
		return
	}
	s.cat, s.catSource, s.catNote = fetched, catalogDownloaded, result.Summary()
	// The report names catalog entries, so build it again from what the last
	// scan found. New images only turn up in the folder on the next scan.
	if s.scan != nil && s.st != nil {
		s.report = check.Offline(s.scan, s.st, s.cat)
	}
}

// startCatalogRefresh runs a refresh in the background.
func (s *Server) startCatalogRefresh(force bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		s.refreshCatalog(ctx, force)
	}()
}

// updateCatalog is the "Check now" button.
func (s *Server) updateCatalog(w http.ResponseWriter, r *http.Request) {
	if s.ownCatalog() {
		writeError(w, http.StatusBadRequest, "You're using your own catalog, so isoshelf leaves it alone.")
		return
	}
	s.mu.Lock()
	s.catNote, s.catErr = "", ""
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	s.refreshCatalog(ctx, true)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catErr != "" {
		writeError(w, http.StatusBadGateway, s.catErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": s.catNote})
}

// setSettings stores the choices that live on this computer rather than in a
// folder's state.
func (s *Server) setSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CatalogAuto *bool `json:"catalog_auto"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	if req.CatalogAuto != nil {
		current := s.loadSettings()
		current.CatalogAuto = req.CatalogAuto
		s.saveSettings(current)
		if *req.CatalogAuto {
			s.startCatalogRefresh(false)
		}
	}
	s.getState(w, r)
}

// ownCatalog reports whether the catalog came from the user, by the --catalog
// flag or their own copy in the config folder. isoshelf never replaces it.
func (s *Server) ownCatalog() bool {
	return s.cfg.CatalogSource == catalogOwn
}

// client makes a client for fetching small documents: checksum files, release
// information, the published catalog.
func (s *Server) client() *remote.Client {
	c := remote.New(s.cfg.Version)
	c.HTTP = s.cfg.HTTP
	c.GitHubToken = s.cfg.GitHubToken
	return c
}

// addMyImage records what the user says a file is, for images no catalog will
// ever know. It writes an entry into their own catalog file and ties the file
// to it, in one step: naming an image is the same act as identifying it.
func (s *Server) addMyImage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path     string `json:"path"`
		Name     string `json:"name"`
		Arch     string `json:"arch"`
		Category string `json:"category"`
		Page     string `json:"page"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.target == "" || s.st == nil || s.scan == nil:
		writeError(w, http.StatusBadRequest, "Choose a folder and scan it first.")
		return
	case s.run != nil:
		writeError(w, http.StatusConflict, "Wait until the current job finishes.")
		return
	case s.cfg.Dirs.Config == "":
		writeError(w, http.StatusBadRequest, "There is nowhere to keep your own images.")
		return
	}
	if _, known := s.st.Files[req.Path]; !known {
		writeError(w, http.StatusBadRequest, req.Path+" is not in the last scan.")
		return
	}

	mine, err := usercat.Add(s.cfg.Dirs.Config, s.cat, usercat.Image{
		Name:     req.Name,
		Filename: path.Base(req.Path),
		Arch:     req.Arch,
		Category: req.Category,
		Page:     req.Page,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged, err := catalog.Merge(s.cat, mine)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.cat = merged

	// The entry matches this exact filename, so the next scan finds it by
	// itself; tie the file to it now so the page shows it straight away.
	if err := s.st.Assign(req.Path, usercat.ID(req.Name), ""); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.Save(s.target); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that: "+err.Error())
		return
	}
	s.report = check.Offline(s.scan, s.st, s.cat)
	s.updatedAt = s.cfg.Now()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "saved",
		"message": fmt.Sprintf("%s is now called %s. It's in your own list, at %s.", path.Base(req.Path), strings.TrimSpace(req.Name), usercat.Path(s.cfg.Dirs.Config)),
	})
}

// countMine is how many entries the user named themselves.
func countMine(cat *catalog.Catalog) int {
	n := 0
	for i := range cat.Entries {
		if usercat.Mine(cat.Entries[i].ID) {
			n++
		}
	}
	return n
}
