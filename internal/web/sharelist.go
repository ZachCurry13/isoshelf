package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/state"
)

// Listing what this isoshelf shares (v0.8.1, #62): every file another
// isoshelf could copy from here, with what this one knows about it. The
// asking isoshelf shows the ones its own folder hasn't got.
//
// What it hands over about a file - its entry, its version, where it came
// from - is this isoshelf's word, and the asker records it as that: a copy
// arrives with the server's account of it in Before, never as proof. The
// bytes are checked against the hash listed here, which proves they arrived
// as they are on this machine and nothing more.

// sharedFileJSON is one file on offer.
type sharedFileJSON struct {
	Name     string       `json:"name"`
	SHA256   string       `json:"sha256"`
	Size     int64        `json:"size"`
	Entry    string       `json:"entry,omitempty"`
	Version  string       `json:"version,omitempty"`
	Assigned bool         `json:"assigned,omitempty"`
	Origin   state.Origin `json:"origin,omitzero"`
	// Since is when this isoshelf first had the file, for "your server has
	// had it since March".
	Since time.Time `json:"since,omitzero"`
}

// shareList answers what this isoshelf would hand over: files with a hash,
// still exactly as they were when hashed. Only while sharing is on.
func (s *Server) shareList(w http.ResponseWriter, r *http.Request) {
	if !s.sharing() {
		writeError(w, http.StatusForbidden, "This isoshelf isn't sharing its images.")
		return
	}
	s.mu.Lock()
	target, st := s.target, s.st
	var files map[string]state.FileRecord
	if st != nil {
		files = st.Clone().Files
	}
	s.mu.Unlock()

	out := []sharedFileJSON{}
	seen := map[string]bool{}
	for p, rec := range files {
		name := path.Base(p)
		// shareFile finds a file by its name and hash; of two files by one
		// name, the first listed is the one offered.
		if rec.SHA256 == "" || seen[name] || strings.HasPrefix(p, state.DirName+"/") {
			continue
		}
		info, err := os.Stat(filepath.Join(target, filepath.FromSlash(p)))
		if err != nil || !info.Mode().IsRegular() || info.Size() != rec.Size || !info.ModTime().Equal(rec.ModTime) {
			continue
		}
		seen[name] = true
		out = append(out, sharedFileJSON{
			Name: name, SHA256: rec.SHA256, Size: rec.Size,
			Entry: rec.Entry, Version: rec.Version, Assigned: rec.Assigned,
			Origin: rec.Origin, Since: firstHad(rec),
		})
	}
	slices.SortFunc(out, func(a, b sharedFileJSON) int { return strings.Compare(a.Name, b.Name) })
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

// firstHad is the earliest this isoshelf knew it had the file, or zero.
func firstHad(rec state.FileRecord) time.Time {
	var first time.Time
	for _, t := range []time.Time{rec.Origin.At, rec.PlacedAt, rec.FirstSeen} {
		if !t.IsZero() && (first.IsZero() || t.Before(first)) {
			first = t
		}
	}
	return first
}
