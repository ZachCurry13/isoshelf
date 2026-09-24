package state

import "time"

// Where a file came from, and what proved it (v0.8.0, #62).
//
// isoshelf used to keep only the address and date of a file it downloaded.
// That said nothing about a file copied from another isoshelf - whose
// address was the other isoshelf's - or one added by hand, and nothing about
// whether the bytes had ever been checked against what the project
// publishes. An Origin says both.
//
// It is only ever what happened. A file is marked checked when its hash
// matched a checksum the project publishes, on the project's own site, and
// never otherwise: a record that claims a check which didn't happen is worse
// than no record, because it is exactly what would look bad if anyone ever
// looked. It belongs to the bytes, so like the rest of a FileRecord it goes
// when the file changes.

// How a file arrived.
const (
	// OriginDownload is a download from the project's own site.
	OriginDownload = "download"
	// OriginCopy is a copy from another isoshelf.
	OriginCopy = "copy"
	// OriginUpload is a file added from the page, from somebody's own
	// computer.
	OriginUpload = "upload"
)

// Origin says how a file arrived and what proved it.
type Origin struct {
	// How is one of the Origin constants. Empty means isoshelf found the file
	// in the folder and knows nothing of how it got there.
	How string `json:"how,omitempty"`
	// From is the address it came from, and At when.
	From string    `json:"from,omitempty"`
	At   time.Time `json:"at,omitzero"`
	// Checked is the address of the published checksum the file matched,
	// and CheckedAt when. Empty means nothing has shown it is the release.
	Checked   string    `json:"checked,omitempty"`
	CheckedAt time.Time `json:"checked_at,omitzero"`
}

// MarkChecked records that the file at path matched the checksum published
// at against, unless it already says so. It reports whether it changed
// anything, so a caller knows whether there is something to save.
func (s *State) MarkChecked(path, against string, now time.Time) bool {
	rec, ok := s.Files[path]
	if !ok || against == "" || rec.Origin.Checked == against {
		return false
	}
	rec.Origin.Checked, rec.Origin.CheckedAt = against, now.UTC()
	s.Files[path] = rec
	return true
}
