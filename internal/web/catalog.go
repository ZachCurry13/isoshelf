package web

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/catupdate"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/remote"
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
	// Changed is the day the list itself last changed (YYYY-MM-DD), and
	// ChangesURL where those changes are written down.
	Changed    string `json:"changed,omitempty"`
	ChangesURL string `json:"changes_url"`
}

// changesURL is the catalog's own changelog, separate from isoshelf's.
const changesURL = "https://github.com/" + appupdate.Repo + "/blob/main/CATALOG-CHANGES.md"

// revisionDate reads the day out of a YYYYMMDDNN revision, or "" if it isn't
// one.
func revisionDate(revision int) string {
	digits := strconv.Itoa(revision)
	if len(digits) < 8 {
		return ""
	}
	day, err := time.Parse("20060102", digits[:8])
	if err != nil {
		return ""
	}
	return day.Format("2006-01-02")
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
		Changed: revisionDate(s.cat.Revision),

		ChangesURL: changesURL,
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
	if !fetched.Newer(s.cat) {
		// An older list than the one already in use, which happens when a
		// build carries a newer catalog than the project has published.
		s.catNote = ""
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
