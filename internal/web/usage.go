package web

import (
	"net/http"
	"runtime"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// The usage page (v0.8.6): what this folder saw, week by week, and the facts
// about where isoshelf is running that are otherwise spread across Settings.
// It is asked for only when somebody opens it, not on every refresh of the
// page, because working it out reads every record the folder has.

// usageWeeks is how many weeks the page shows: two months, enough to see a
// pattern without the table outgrowing a phone.
const usageWeeks = 8

type usageJSON struct {
	Weeks  []state.Week `json:"weeks"`
	System systemJSON   `json:"system"`
}

// systemJSON is where isoshelf runs and how it is set up, in one place.
type systemJSON struct {
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
	// Mode is "desktop", "portable", "server" or "container".
	Mode    string    `json:"mode"`
	Started time.Time `json:"started"`

	Folder  string     `json:"folder,omitempty"`
	Profile string     `json:"profile,omitempty"`
	Files   int        `json:"files"`
	Bytes   int64      `json:"bytes"`
	Space   *spaceJSON `json:"space,omitempty"`
	// Archive is what waits in .isoshelf/removed, still using room.
	Archive  removedJSON `json:"archive"`
	LastScan *time.Time  `json:"last_scan,omitempty"`
	// CheckedAt is when the oldest answer about updates was given.
	CheckedAt *time.Time `json:"checked_at,omitempty"`

	Records   string `json:"records,omitempty"`
	ConfigDir string `json:"config_dir,omitempty"`

	CatalogEntries  int        `json:"catalog_entries"`
	CatalogRevision int        `json:"catalog_revision"`
	CatalogUpdated  *time.Time `json:"catalog_updated,omitempty"`
	CatalogAuto     bool       `json:"catalog_auto"`

	AutoCheck       bool   `json:"auto_check"`
	AutoUpdate      bool   `json:"auto_update"`
	AutoUpdateEvery string `json:"auto_update_every,omitempty"`
	Sharing         bool   `json:"sharing"`
	// Peer is the other isoshelf this one copies from, when one is in use.
	Peer string `json:"peer,omitempty"`
}

func (s *Server) getUsage(w http.ResponseWriter, r *http.Request) {
	page := s.pageState()

	s.mu.Lock()
	weeks := []state.Week{}
	var files int
	var bytes int64
	var lastScan *time.Time
	if s.st != nil {
		weeks = s.st.Weeks(s.cfg.Now(), usageWeeks)
		for _, rec := range s.st.Files {
			files++
			bytes += rec.Size
		}
		if n := len(s.st.History); n > 0 {
			at := s.st.History[n-1].Time
			lastScan = &at
		}
	}
	started, revision := s.startedAt, s.cat.Revision
	s.mu.Unlock()

	sys := systemJSON{
		Version:         page.Version,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		Mode:            s.mode(),
		Started:         started,
		Folder:          page.Target,
		Profile:         page.Profile,
		Files:           files,
		Bytes:           bytes,
		Space:           page.Space,
		Archive:         page.Removed,
		LastScan:        lastScan,
		CheckedAt:       page.CheckedAt,
		Records:         page.Records.File,
		ConfigDir:       page.ConfigDir,
		CatalogEntries:  page.Catalog.Entries,
		CatalogRevision: revision,
		CatalogUpdated:  page.Catalog.UpdatedAt,
		CatalogAuto:     page.Catalog.Auto,
		AutoCheck:       page.AutoCheck,
		AutoUpdate:      page.AutoUpdate,
		Sharing:         page.Peer.Sharing,
	}
	if sys.AutoUpdate {
		sys.AutoUpdateEvery = page.AutoUpdateEvery
	}
	if page.Peer.On {
		sys.Peer = page.Peer.Address
	}
	writeJSON(w, http.StatusOK, usageJSON{Weeks: weeks, System: sys})
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
