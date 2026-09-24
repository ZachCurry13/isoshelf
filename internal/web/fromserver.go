package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/fetch"
	"github.com/ZachCurry13/isoshelf/internal/inventory"
	"github.com/ZachCurry13/isoshelf/internal/peer"
	"github.com/ZachCurry13/isoshelf/internal/state"
	"github.com/ZachCurry13/isoshelf/internal/verify"
)

// Copying from your server (v0.8.1, #62): what the other isoshelf has that
// this folder doesn't - catalog images or not - and a copy of any of it.
//
// A copy is checked against the server's hash, which proves it arrived as it
// is over there and nothing more. So it is recorded as a copy, with the
// server's account of its own file in Before, and never as checked; the
// next check says whether it is the release the project publishes, the same
// way it would for any file. A copy never replaces anything already here.

// peerFileJSON is one file the server has and this folder doesn't.
type peerFileJSON struct {
	peer.SharedFile
	// Image is this isoshelf's catalog name for it, when its catalog knows
	// the entry the server named.
	Image string `json:"image,omitempty"`
}

// peerFiles lists what the server could hand over that this folder hasn't
// got: by hash, and by name, since a copy can't land on a name that's taken.
func (s *Server) peerFiles(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"files": []peerFileJSON{}, "set_up": s.loadSettings().Peer.Use()}
	s.mu.Lock()
	target := s.target
	have := map[string]bool{}
	if s.st != nil {
		for p, rec := range s.st.Files {
			have[path.Base(p)] = true
			if rec.SHA256 != "" {
				have[strings.ToLower(rec.SHA256)] = true
			}
		}
	}
	s.mu.Unlock()
	if target == "" || out["set_up"] == false {
		writeJSON(w, http.StatusOK, out)
		return
	}
	files, near, err := s.listPeer(r.Context(), target)
	if err != nil {
		out["error"] = err.Error()
		writeJSON(w, http.StatusOK, out)
		return
	}
	list := []peerFileJSON{}
	for _, f := range files {
		if have[f.Name] || have[strings.ToLower(f.SHA256)] {
			continue
		}
		j := peerFileJSON{SharedFile: f}
		if e := s.catalog().Entry(f.Entry); e != nil {
			j.Image = e.Name
		}
		list = append(list, j)
	}
	out["files"], out["server"] = list, near.Address
	writeJSON(w, http.StatusOK, out)
}

// listPeer signs in to the server and asks what it shares.
func (s *Server) listPeer(ctx context.Context, target string) ([]peer.SharedFile, *peer.Client, error) {
	near := s.peerFor(target)
	if near == nil {
		return nil, nil, errors.New("Couldn't reach your server, or sign in to it. Check the address and login under Settings.")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	files, err := near.List(ctx)
	return files, near, err
}

// copyFromPeer queues a copy of one file the server has.
func (s *Server) copyFromPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" || req.SHA256 == "" {
		writeError(w, http.StatusBadRequest, "bad request")
		return
	}
	s.mu.Lock()
	target := s.target
	s.mu.Unlock()
	if target == "" {
		writeError(w, http.StatusBadRequest, "Choose a folder first.")
		return
	}
	// Asked again rather than trusted from the page: what is recorded about
	// the copy is what the server says now.
	files, _, err := s.listPeer(r.Context(), target)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var want *peer.SharedFile
	for i := range files {
		if files[i].Name == req.Name && strings.EqualFold(files[i].SHA256, req.SHA256) {
			want = &files[i]
		}
	}
	if want == nil {
		writeError(w, http.StatusConflict, "Your server doesn't have that file any more.")
		return
	}
	if _, err := os.Lstat(filepath.Join(target, req.Name)); err == nil {
		writeError(w, http.StatusConflict, "A file called "+req.Name+" is already here, and a copy never replaces anything.")
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	name := want.Name
	entry := ""
	// s.cat, not s.catalog(): that takes the lock this already holds.
	if e := s.cat.Entry(want.Entry); e != nil {
		name, entry = e.Name, e.ID
	}
	s.nextJob++
	id := s.nextJob
	s.queue = append(s.queue, &job{id: id, target: target, entry: entry, name: name, size: want.Size, copy: want})
	s.lastErr = ""
	s.startNextLocked()
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "queued", "id": id})
}

// runCopy copies one file from the server, checked against its hash.
func (s *Server) runCopy(ctx context.Context, j *job) (string, error) {
	c := j.copy
	near := s.peerFor(j.target)
	if near == nil {
		return "", errors.New("Couldn't reach your server to copy " + c.Name + ".")
	}
	if _, err := os.Lstat(filepath.Join(j.target, c.Name)); err == nil {
		return "", fmt.Errorf("a file called %s is already here, and a copy never replaces anything", c.Name)
	}
	st, err := s.recordsFor(j.target).Load(j.target)
	if err != nil {
		return "", err
	}
	base := st.Clone()
	fetcher := fetch.New(s.cfg.Version)
	fetcher.HTTP = withJar(s.cfg.HTTP, near.HTTP.Jar)
	fetcher.HostHeaders = map[string]map[string]string{near.Address: near.Headers()}
	got, err := fetcher.Download(ctx, fetch.Request{
		URLs: []string{near.FileURL(c.Name, c.SHA256)}, Filename: c.Name, Dir: j.target, Size: c.Size,
		Checksum: &verify.Checksum{Name: c.Name, Algorithm: verify.SHA256, Hex: strings.ToLower(c.SHA256)},
	}, func(p fetch.Progress) {
		s.mu.Lock()
		if s.downloading != nil {
			s.downloading.progress = inventory.Progress{
				Stage: inventory.Stage(p.Stage), File: p.Filename, Done: p.Done, Total: p.Total,
			}
			s.downloading.from = "your server, " + near.Address
		}
		s.mu.Unlock()
	})
	if err != nil {
		return "", err
	}

	now := s.cfg.Now().UTC()
	before := c.Origin
	if before.At.IsZero() {
		before.At = c.Since
	}
	rec := state.FileRecord{
		SHA256: got.SHA256, HashedAt: now, SourceURL: got.URL, PlacedAt: now,
		// Checked stays empty: matching the server's hash proves the copy is
		// the server's file, not that it is the release.
		Origin: state.Origin{How: state.OriginCopy, From: got.URL, At: now},
		Before: before,
	}
	// What the server was told by hand about the file, it passes on as that:
	// somebody's word, recorded the way a name given by hand is.
	if c.Assigned && s.catalog().Entry(c.Entry) != nil {
		rec.Entry, rec.Version, rec.Assigned = c.Entry, c.Version, true
	}
	if err := st.Placed(j.target, c.Name, rec); err != nil {
		return "", err
	}
	s.mu.Lock()
	saveErr := s.saveMerged(j.target, base, st)
	if s.st != nil && s.target == j.target {
		state.Merge(base, st, s.st)
	}
	s.mu.Unlock()
	return "Copied from your server. It matches your server's copy; the next check says whether it is the release the project publishes.", saveErr
}
