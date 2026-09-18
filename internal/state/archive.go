package state

import (
	"slices"
	"time"
)

// maxArchive is how many past images a target remembers.
const maxArchive = 500

// How an image left the folder.
const (
	// GoneRemoved means the user deleted it through isoshelf.
	GoneRemoved = "removed"
	// GoneMovedAside means it waits in .isoshelf/removed.
	GoneMovedAside = "moved-aside"
	// GoneReplaced means an update took its place.
	GoneReplaced = "replaced"
	// GoneVanished means it disappeared between scans, by some other means.
	GoneVanished = "vanished"
)

// ArchiveEntry remembers an image that used to be in the folder, so it can be
// found or downloaded again later.
type ArchiveEntry struct {
	Path      string    `json:"path"`
	Entry     string    `json:"entry,omitempty"`
	Version   string    `json:"version,omitempty"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256,omitempty"`
	SourceURL string    `json:"source_url,omitempty"`
	LastSeen  time.Time `json:"last_seen"`
	// Gone is one of the Gone constants, and GoneAt when it happened.
	Gone   string    `json:"gone"`
	GoneAt time.Time `json:"gone_at"`
}

// Archive returns the images that used to be here, most recent first.
func (s *State) Archive() []ArchiveEntry {
	return s.Past
}

// archive remembers a file that has left the folder. Older notes about the
// same path are replaced, so the list stays readable.
func (s *State) archive(path string, rec FileRecord, gone string, now time.Time) {
	s.Past = slices.DeleteFunc(s.Past, func(a ArchiveEntry) bool { return a.Path == path })
	s.Past = slices.Insert(s.Past, 0, ArchiveEntry{
		Path:      path,
		Entry:     rec.Entry,
		Version:   rec.Version,
		Size:      rec.Size,
		SHA256:    rec.SHA256,
		SourceURL: rec.SourceURL,
		LastSeen:  rec.ModTime,
		Gone:      gone,
		GoneAt:    now.UTC(),
	})
	if len(s.Past) > maxArchive {
		s.Past = s.Past[:maxArchive]
	}
}

// Archived remembers a file isoshelf removed or replaced. Removing the file
// itself is internal/update's job.
func (s *State) Archived(path string, gone string, now time.Time) {
	rec, ok := s.Files[path]
	if !ok {
		rec = FileRecord{}
	}
	s.archive(path, rec, gone, now)
	delete(s.Files, path)
}

// forgetArchived drops the note about a path that is back in the folder.
func (s *State) forgetArchived(path string) {
	s.Past = slices.DeleteFunc(s.Past, func(a ArchiveEntry) bool { return a.Path == path })
}
