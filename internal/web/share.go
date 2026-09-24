package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// Letting another isoshelf copy an image from this one.
//
// A NAS that already holds a 6 GB image and a laptop about to fetch the same
// one from the other side of the world is a silly way round, and the NAS is
// usually the one that has it. So an isoshelf can offer its images to another
// isoshelf on the same network, which tries that copy first and falls back to
// the project's own site if it isn't there.
//
// What makes this safe to try first is that nothing about verification
// changes. The checksum still comes from the project's own HTTPS site, never
// from the machine serving the bytes, and the bytes are still checked against
// it before anything is placed. A copy that turns out to be wrong - stale,
// corrupt, or served by something pretending to be an isoshelf - costs one
// fall back to the real source and nothing else. That is the same rule
// mirrors have always lived under, applied to one more mirror.
//
// It is off until somebody turns it on. Sharing means this machine will hand
// whole images to anyone who can sign in, which is a different thing from
// letting them manage the folder, and it should be a decision rather than a
// default.

// shareHave answers whether this isoshelf holds a particular file, named and
// hashed. Both have to match: the name alone would say yes to a stale copy
// under the same name, which for an image whose filename never changes is
// exactly the copy the asker is trying to replace.
func (s *Server) shareHave(w http.ResponseWriter, r *http.Request) {
	if !s.sharing() {
		writeError(w, http.StatusForbidden, "This isoshelf isn't sharing its images.")
		return
	}
	name, sum := r.URL.Query().Get("name"), strings.ToLower(r.URL.Query().Get("sha256"))
	path, size := s.sharedFile(name, sum)
	if path == "" {
		writeJSON(w, http.StatusOK, map[string]any{"have": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"have": true, "size": size})
}

// shareFile serves the bytes. http.ServeFile does the work, which means
// ranges and resuming come with it - the asking isoshelf resumes from this
// one exactly as it would from a project's site.
func (s *Server) shareFile(w http.ResponseWriter, r *http.Request) {
	if !s.sharing() {
		http.Error(w, "this isoshelf isn't sharing its images", http.StatusForbidden)
		return
	}
	path, _ := s.sharedFile(r.PathValue("name"), strings.ToLower(r.URL.Query().Get("sha256")))
	if path == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")

	// Who took it, so this machine knows what each drive it serves holds.
	// Recorded when the whole file goes out, not on every range: a resumed
	// download is one file, asked for in pieces.
	if r.Header.Get("Range") == "" {
		s.noteServed(
			askerFrom(r.Header.Get(askerIDHeader), r.Header.Get(askerNameHeader), r.RemoteAddr),
			ServedFile{Name: info.Name(), SHA256: strings.ToLower(r.URL.Query().Get("sha256")), Size: info.Size()})
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

// The asking isoshelf names itself in these, so the sharing one can keep
// track of which drives it has served. Neither is trusted for anything: they
// decide what a line in a list says, never what is handed over.
const (
	askerIDHeader   = "X-Isoshelf-Folder"
	askerNameHeader = "X-Isoshelf-Folder-Name"
)

// sharing says whether this isoshelf offers its images to others.
//
// Only a server shares. An isoshelf on a desktop answers that computer alone,
// so nobody could ask it for a file - and a switch left on from before would
// otherwise make every scan hash every image for nothing.
func (s *Server) sharing() bool {
	share := s.loadSettings().ShareImages
	return s.cfg.AnyHost && share != nil && *share
}

// sharedFile turns a name and a hash into a path, or "" for anything that
// isn't a file this isoshelf holds under exactly that name with exactly that
// hash.
//
// The name is never joined onto the folder and used. It is looked up in the
// records, which hold what the last scan found, and the path that comes back
// is the one isoshelf wrote there - so "../../etc/passwd" doesn't find a
// record and doesn't become a path. The hash has to match the record too,
// which is what makes the answer mean "this exact file" rather than "a file
// by that name".
func (s *Server) sharedFile(name, sum string) (string, int64) {
	if name == "" || sum == "" {
		return "", 0
	}
	s.mu.Lock()
	target, st := s.target, s.st
	s.mu.Unlock()
	if target == "" || st == nil {
		return "", 0
	}
	for path, rec := range st.Files {
		if rec.SHA256 == "" || !strings.EqualFold(rec.SHA256, sum) {
			continue
		}
		if filepath.Base(filepath.FromSlash(path)) != name {
			continue
		}
		full := filepath.Join(target, filepath.FromSlash(path))
		info, err := os.Stat(full)
		if err != nil || !info.Mode().IsRegular() {
			return "", 0
		}
		// The record is only as fresh as the last scan. A file that has
		// changed since is not the file that was hashed, and handing it over
		// would be handing over something nobody checked.
		if info.Size() != rec.Size || !info.ModTime().Equal(rec.ModTime) {
			return "", 0
		}
		return full, info.Size()
	}
	return "", 0
}

// setSharing turns this isoshelf's own sharing on or off.
func (s *Server) setSharing(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Share *bool `json:"share"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Share == nil {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.updateSettings(func(c *settings.Settings) { c.ShareImages = req.Share })
	s.getState(w, r)
}
