package web

import (
	"runtime"
	"time"
)

// systemJSON is where this isoshelf runs: what Settings → isoshelf itself
// says under the version (v0.8.13; the Usage section used to).
type systemJSON struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// Mode is "desktop", "portable", "server" or "container".
	Mode    string    `json:"mode"`
	Started time.Time `json:"started"`
	// CatalogRevision is the revision of the list of images in use.
	CatalogRevision int `json:"catalog_revision"`
}

// systemLocked is the facts about this isoshelf; s.mu must be held.
func (s *Server) systemLocked() systemJSON {
	return systemJSON{
		OS: runtime.GOOS, Arch: runtime.GOARCH, Mode: s.mode(),
		Started: s.startedAt, CatalogRevision: s.cat.Revision,
	}
}

// mode says how isoshelf is being run, in the page's own words for it.
func (s *Server) mode() string {
	switch {
	case s.cfg.SelfUpdate.Container:
		return "container"
	case s.cfg.AnyHost:
		return "server"
	case s.cfg.Dirs.Portable:
		return "portable"
	}
	return "desktop"
}
