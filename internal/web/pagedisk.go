package web

import (
	"net/http"
	"os"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/space"
	"github.com/ZachCurry13/isoshelf/internal/update"
)

// Most of what the page shows is held in memory, but a few parts of it are
// read from a disk: the folders isoshelf remembers, the room left, whether
// the folder changed, who can sign in. Those are read before the lock is
// taken, because a NAS that has gone to sleep must not hold up the whole page.

// diskState is the part of the page's state that comes from a disk.
type diskState struct {
	recent  []rememberedJSON
	room    space.Usage
	changed bool
	login   loginJSON
}

func (s *Server) readDisk() diskState {
	return diskState{
		recent:  s.recentTargets(),
		room:    s.targetSpace(),
		changed: s.folderChanged(),
		login:   s.loginInfo(),
	}
}

func (s *Server) getState(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.pageState())
}

// pageState is what the page is told: everything it shows, as of now.
func (s *Server) pageState() stateJSON {
	s.mu.Lock()
	running := s.scanning
	s.mu.Unlock()
	disk := s.readDisk()
	s.mu.Lock()
	defer s.mu.Unlock()
	// A scan that ended while the disk was being read saved its folder's copy
	// in isoshelf's own folder after the list of folders had been read. This
	// answer would say the scan was over and carry the list from before it:
	// the folder just scanned missing, and a "folder changed" measured
	// against the scan before. Read again. Once is enough - another scan would
	// have to start and finish inside one read of the disk.
	if running != nil && s.scanning != running {
		s.mu.Unlock()
		disk = s.readDisk()
		s.mu.Lock()
	}
	out := s.stateLocked(disk.recent, disk.room)
	out.Login = disk.login
	out.FolderChanged = disk.changed && s.scanning == nil && s.downloading == nil
	return out
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
