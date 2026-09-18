package web

import (
	"net/http"
	"os"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// stateJSON is everything the page shows.
type stateJSON struct {
	Version   string                 `json:"version"`
	Portable  bool                   `json:"portable"`
	Target    string                 `json:"target"`
	Profile   string                 `json:"profile"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
	Run       *runJSON               `json:"run,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Warnings  []string               `json:"warnings"`
	Report    *check.ReportJSON      `json:"report,omitempty"`
	Tracks    map[string]state.Track `json:"tracks"`
	UsualSet  []string               `json:"usual_set"`
	Recent    []string               `json:"recent_targets"`
	Bookmarks []string               `json:"bookmarks"`
	// ReplaceAction is the answer the user usually gives when an update
	// replaces a file.
	ReplaceAction string            `json:"replace_action,omitempty"`
	Removed       removedJSON       `json:"removed"`
	Catalog       catalogStatusJSON `json:"catalog"`
	AppUpdate     *appupdate.Notice `json:"app_update,omitempty"`
	// ReportURL is where a missing image can be reported.
	ReportURL string `json:"report_url,omitempty"`
	// Space is the room left in the folder, when the disk says.
	Space *spaceJSON `json:"space,omitempty"`
	// FolderChanged is set when the folder has been written to since the last
	// scan, so the page can offer to look again.
	FolderChanged bool `json:"folder_changed,omitempty"`
	// Downloads is the download queue: the one running, the ones waiting and
	// the ones that ended.
	Downloads downloadsJSON `json:"downloads"`
}

// spaceJSON is the room left where images are kept.
type spaceJSON struct {
	Free  int64 `json:"free"`
	Total int64 `json:"total"`
}

// removedJSON describes what waits in .isoshelf/removed.
type removedJSON struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

type runJSON struct {
	Kind    string    `json:"kind"`
	Stage   string    `json:"stage"`
	File    string    `json:"file,omitempty"`
	Done    int64     `json:"done"`
	Total   int64     `json:"total"`
	Item    int       `json:"item,omitempty"`
	Items   int       `json:"items,omitempty"`
	Started time.Time `json:"started"`
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	// Both of these read a disk, so they happen before the lock is taken: a
	// NAS that has gone to sleep must not hold up the whole page.
	recent := s.recentTargets()
	room := s.targetSpace()
	changed := s.folderChanged()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.stateLocked(recent, room)
	out.FolderChanged = changed && s.run == nil
	writeJSON(w, http.StatusOK, out)
}

// spaceInterval is how long the free space is trusted before asking again.
// The page asks for the state every half second while a scan runs, and on a
// network share every answer costs a round trip.
const spaceInterval = 5 * time.Second

// folderChanged reports whether the folder has been written to since the last
// scan: a file dropped in with a file manager, or one deleted there. It is
// one stat of the folder itself, and a folder isoshelf can't read simply
// isn't reported as changed.
//
// Only the top level is watched. A change deep inside a Ventoy drive's
// subfolders doesn't move the folder's own timestamp, so Scan remains the
// honest answer for those.
func (s *Server) folderChanged() bool {
	s.mu.Lock()
	target, scanned := s.target, s.updatedAt
	s.mu.Unlock()
	if target == "" || scanned.IsZero() {
		return false
	}
	info, err := os.Stat(target)
	if err != nil {
		return false
	}
	return info.ModTime().After(scanned)
}

// targetSpace returns the room left in the folder the images are kept in.
// A folder that can't say is not an error worth showing: the page leaves the
// number out instead.
func (s *Server) targetSpace() space.Usage {
	s.mu.Lock()
	target, cached, at, of := s.target, s.room, s.roomAt, s.roomOf
	s.mu.Unlock()
	if target == "" {
		return space.Usage{}
	}
	if of == target && !at.IsZero() && s.cfg.Now().Sub(at) < spaceInterval {
		return cached
	}
	usage, _ := space.Of(target)

	s.mu.Lock()
	s.room, s.roomAt, s.roomOf = usage, s.cfg.Now(), target
	s.mu.Unlock()
	return usage
}

// removedInfo counts what waits in the target's removed folder.
func (s *Server) removedInfo(target string) removedJSON {
	if target == "" {
		return removedJSON{}
	}
	files, bytes, err := update.Removed(target)
	if err != nil {
		return removedJSON{}
	}
	return removedJSON{Files: len(files), Bytes: bytes}
}

// stateLocked builds the page state; s.mu must be held.
func (s *Server) stateLocked(recent []string, room space.Usage) stateJSON {
	out := stateJSON{
		Version:       s.cfg.Version,
		Portable:      s.cfg.Dirs.Portable,
		Target:        s.target,
		Error:         s.lastErr,
		Warnings:      nonNil(s.warnings),
		Tracks:        map[string]state.Track{},
		UsualSet:      []string{},
		Recent:        recent,
		Bookmarks:     nonNil(s.loadSettings().Bookmarks),
		ReplaceAction: s.loadSettings().ReplaceAction,
		Removed:       s.removedInfo(s.target),
		Catalog:       s.catalogStatusLocked(),
		ReportURL:     "https://github.com/" + appupdate.Repo + "/issues/new",
		AppUpdate:     s.notice,
		Downloads:     s.downloadsLocked(),
	}
	if room.Known() {
		out.Space = &spaceJSON{Free: room.Free, Total: room.Total}
	}
	if s.st != nil {
		out.Profile = string(s.st.Profile)
		out.Tracks = s.st.Tracks
		out.UsualSet = nonNil(s.st.UsualSet())
	}
	if s.report != nil {
		j := s.report.JSON()
		out.Report = &j
		t := s.updatedAt
		out.UpdatedAt = &t
	}
	if s.run != nil {
		out.Run = &runJSON{
			Kind:    s.run.kind,
			Stage:   string(s.run.progress.Stage),
			File:    s.run.progress.File,
			Done:    s.run.progress.Done,
			Total:   s.run.progress.Total,
			Item:    s.run.progress.Item,
			Items:   s.run.progress.Items,
			Started: s.run.started,
		}
	}
	return out
}
