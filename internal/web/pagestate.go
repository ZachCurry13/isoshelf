package web

import (
	"net/http"
	"os"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/settings"
	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// stateJSON is everything the page shows.
type stateJSON struct {
	Version  string `json:"version"`
	Portable bool   `json:"portable"`
	// Server is true when isoshelf answers to more than localhost - a NAS,
	// a container, anything somebody opens from another machine. The page
	// uses it to name the browser tab, because somebody running one on their
	// desktop and one on their NAS has two tabs called the same thing.
	Server    bool                   `json:"server"`
	Target    string                 `json:"target"`
	Profile   string                 `json:"profile"`
	UpdatedAt *time.Time             `json:"updated_at,omitempty"`
	Run       *runJSON               `json:"run,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Warnings  []string               `json:"warnings"`
	Report    *check.ReportJSON      `json:"report,omitempty"`
	Tracks    map[string]state.Track `json:"tracks"`
	UsualSet  []string               `json:"usual_set"`
	Recent    []rememberedJSON       `json:"recent_targets"`
	Bookmarks []string               `json:"bookmarks"`
	// OldFiles is what happens to the copy an update replaces, for images
	// that haven't been given their own answer.
	OldFiles string `json:"old_files,omitempty"`
	// AutoCheck is whether isoshelf checks for updates by itself, and
	// AppUpdateCheck whether it looks for a newer isoshelf.
	AutoCheck      bool `json:"auto_check"`
	AppUpdateCheck bool `json:"app_update_check"`
	// AutoUpdate is whether isoshelf updates the images by itself, and
	// AutoUpdateEvery how often.
	AutoUpdate      bool   `json:"auto_update"`
	AutoUpdateEvery string `json:"auto_update_every"`

	// ArchiveAfter is how many days a file waits in the archive before
	// isoshelf deletes it, 0 for never, and ArchiveDue is what the next
	// sweep would take at that setting - so Settings can say what will
	// happen before it happens.
	ArchiveAfter    int   `json:"archive_after"`
	ArchiveDue      int   `json:"archive_due"`
	ArchiveDueBytes int64 `json:"archive_due_bytes"`
	// CheckedAt is when the oldest answer the report rests on was given, so
	// the page can say how fresh it really is rather than when it last drew.
	CheckedAt *time.Time `json:"checked_at,omitempty"`
	// Appearance is how the page should look: the answers in Settings.
	Appearance settings.Appearance `json:"appearance"`
	// ConfigDir is where isoshelf keeps its own files. Settings shows it so
	// nobody has to hunt for it.
	ConfigDir string `json:"config_dir,omitempty"`
	// Records is where this folder's records are kept.
	Records recordsJSON `json:"records"`
	// Peer is the other isoshelf this one looks at before the internet, and
	// whether this one shares its own images.
	Peer peerJSON `json:"peer"`
	// Login is the username and password, for Settings to show and change.
	Login     loginJSON         `json:"login"`
	Removed   removedJSON       `json:"removed"`
	Catalog   catalogStatusJSON `json:"catalog"`
	AppUpdate *appupdate.Notice `json:"app_update,omitempty"`
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
	Kind  string `json:"kind"`
	Stage string `json:"stage"`
	File  string `json:"file,omitempty"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
	Item  int    `json:"item,omitempty"`
	Items int    `json:"items,omitempty"`
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	// Both of these read a disk, so they happen before the lock is taken: a
	// NAS that has gone to sleep must not hold up the whole page.
	recent := s.recentTargets()
	room := s.targetSpace()
	changed := s.folderChanged()
	// Who can get in is read from disk too, and depends on this request:
	// whether this browser came in with a password or with the link.
	login := s.loginInfo()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.stateLocked(recent, room)
	out.Login = login
	out.FolderChanged = changed && s.scanning == nil && s.downloading == nil
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

// rememberedJSON is one folder isoshelf remembers: where it is, when it was last
// looked at, and what it held then. The numbers are what the last scan saw,
// not what is there now - the drive may not even be plugged in.
type rememberedJSON struct {
	Path     string    `json:"path"`
	ID       string    `json:"id"`
	LastUsed time.Time `json:"last_used,omitzero"`
	Files    int       `json:"files,omitempty"`
	Bytes    int64     `json:"bytes,omitempty"`
}

// warningsLocked is what the page should say, including the one line about a
// download isoshelf started by itself. s.mu must be held.
func (s *Server) warningsLocked() []string {
	out := s.warnings
	if s.autoNote != "" {
		out = append([]string{s.autoNote}, out...)
	}
	// Said every time the page loads, not once at startup: somebody being
	// signed out repeatedly needs the reason in front of them, and this one
	// stays true until the folder's permissions are fixed.
	if note := s.sessionKeyNote(); note != "" {
		out = append([]string{note}, out...)
	}
	return out
}

// stateLocked builds the page state; s.mu must be held.
func (s *Server) stateLocked(recent []rememberedJSON, room space.Usage) stateJSON {
	saved := s.loadSettings()
	out := stateJSON{
		Version:         s.cfg.Version,
		Portable:        s.cfg.Dirs.Portable,
		Server:          s.cfg.AnyHost,
		Target:          s.target,
		Error:           s.lastErr,
		Warnings:        nonNil(s.warningsLocked()),
		Tracks:          map[string]state.Track{},
		UsualSet:        []string{},
		Recent:          recent,
		Bookmarks:       nonNil(saved.Bookmarks),
		OldFiles:        saved.OldFiles,
		AutoCheck:       settings.On(saved.AutoCheck),
		AppUpdateCheck:  settings.On(saved.AppUpdateCheck),
		AutoUpdate:      saved.AutoUpdate != nil && *saved.AutoUpdate,
		AutoUpdateEvery: settings.CleanEvery(saved.AutoUpdateEvery),
		ArchiveAfter:    cleanArchiveAfter(saved.ArchiveAfter),
		Appearance:      saved.Appearance,
		ConfigDir:       s.cfg.Dirs.Config,
		Records:         s.recordsLocked(saved),
		Peer:            s.peerInfo(saved),
		Removed:         s.removedInfo(s.target),
		Catalog:         s.catalogStatusLocked(),
		ReportURL:       "https://github.com/" + appupdate.Repo + "/issues/new",
		AppUpdate:       s.notice,
		Downloads:       s.downloadsLocked(),
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
		if !s.report.CheckedAt.IsZero() {
			at := s.report.CheckedAt
			out.CheckedAt = &at
		}
		t := s.updatedAt
		out.UpdatedAt = &t
	}
	// Run is the scan's own progress card. A download's progress rides with
	// the download itself, in downloadsJSON.Current.
	if s.scanning != nil {
		out.Run = &runJSON{
			Kind:  s.scanning.kind,
			Stage: string(s.scanning.progress.Stage),
			File:  s.scanning.progress.File,
			Done:  s.scanning.progress.Done,
			Total: s.scanning.progress.Total,
			Item:  s.scanning.progress.Item,
			Items: s.scanning.progress.Items,
		}
	}
	// What the archive timer would take on its next sweep, so Settings can
	// say what will happen before it happens rather than afterwards.
	if out.ArchiveAfter > 0 {
		names, bytes := staleArchived(s.target, s.st, out.ArchiveAfter, s.cfg.Now())
		out.ArchiveDue, out.ArchiveDueBytes = len(names), bytes
	}
	return out
}
