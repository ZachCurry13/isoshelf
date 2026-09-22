package web

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// What this isoshelf has handed to other isoshelfs.
//
// The maintainer's ask: the machine doing the sharing should keep track of
// which drives it has served. Two reasons it is worth the few lines. The
// smaller one is that somebody who turns sharing on should be able to see
// that it is doing something, and for whom. The larger one is that a NAS
// which knows what a drive took from it knows most of what that drive holds -
// which is the list "rebuild a drive" (#11) needs when the drive is gone.
//
// It is kept in isoshelf's own folder, never in the images folder: it is
// about this machine's dealings, not about the images.

const servedFileName = "served.json"

// Served is one drive this isoshelf has given files to.
type Served struct {
	// ID is the asking folder's target id, which is how the same drive is
	// recognized again after it moves or is renamed.
	ID string `json:"id"`
	// Name is what the asker calls itself, for somebody to read.
	Name string `json:"name,omitempty"`
	// Address is where it asked from, last time.
	Address string `json:"address,omitempty"`
	// Files are the images it has taken, newest first.
	Files []ServedFile `json:"files,omitempty"`
	// Bytes is everything it has taken, added up.
	Bytes int64     `json:"bytes,omitempty"`
	First time.Time `json:"first,omitzero"`
	Last  time.Time `json:"last,omitzero"`
}

// ServedFile is one image handed over.
type ServedFile struct {
	Name   string    `json:"name"`
	SHA256 string    `json:"sha256,omitempty"`
	Size   int64     `json:"size,omitempty"`
	At     time.Time `json:"at,omitzero"`
}

// maxServedFiles is how many files are remembered per drive. Enough to
// rebuild a drive from, and bounded so a file isoshelf writes can't grow
// without end.
const maxServedFiles = 500

// servedMu holds the file still for a read-modify-write, the same way the
// settings file is held: there is one writer per request and requests
// overlap.
var servedMu sync.Mutex

// noteServed records that a file went out. It is best effort: failing to
// write down what happened must never stop the file going.
func (s *Server) noteServed(who Served, file ServedFile) {
	if s.cfg.Dirs.Config == "" || who.ID == "" {
		return
	}
	servedMu.Lock()
	defer servedMu.Unlock()
	all := loadServed(s.cfg.Dirs.Config)
	now := s.cfg.Now()

	at := slices.IndexFunc(all, func(d Served) bool { return d.ID == who.ID })
	if at < 0 {
		who.First = now
		all = append(all, who)
		at = len(all) - 1
	}
	drive := &all[at]
	if who.Name != "" {
		drive.Name = who.Name
	}
	drive.Address = who.Address
	drive.Last = now

	file.At = now
	// The same file taken again replaces its earlier line rather than adding
	// one: what matters is that the drive has it, not how often it asked.
	drive.Files = slices.DeleteFunc(drive.Files, func(f ServedFile) bool {
		if f.Name == file.Name && f.SHA256 == file.SHA256 {
			drive.Bytes -= f.Size
			return true
		}
		return false
	})
	drive.Files = append([]ServedFile{file}, drive.Files...)
	drive.Bytes += file.Size
	if len(drive.Files) > maxServedFiles {
		for _, dropped := range drive.Files[maxServedFiles:] {
			drive.Bytes -= dropped.Size
		}
		drive.Files = drive.Files[:maxServedFiles]
	}
	slices.SortFunc(all, func(a, b Served) int { return b.Last.Compare(a.Last) })
	saveServed(s.cfg.Dirs.Config, all)
}

// loadServed reads the record, or nothing when there isn't one. A damaged
// file is treated as no file: this is a note about what happened, and losing
// it must never stop isoshelf.
func loadServed(configDir string) []Served {
	data, err := os.ReadFile(filepath.Join(configDir, servedFileName))
	if err != nil {
		return nil
	}
	var all []Served
	if err := json.Unmarshal(data, &all); err != nil {
		return nil
	}
	return all
}

func saveServed(configDir string, all []Served) {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return
	}
	writeFileQuietly(filepath.Join(configDir, servedFileName), append(data, '\n'))
}

// writeFileQuietly writes through a temporary file, so a crash never leaves
// half a record behind.
func writeFileQuietly(name string, data []byte) {
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	os.Rename(tmp.Name(), name)
}

// forgetServed drops one drive from the record.
func forgetServed(configDir, id string) error {
	servedMu.Lock()
	defer servedMu.Unlock()
	all := loadServed(configDir)
	kept := slices.DeleteFunc(all, func(d Served) bool { return d.ID == id })
	if len(kept) == len(all) {
		return nil
	}
	saveServed(configDir, kept)
	return nil
}

// askerFrom reads who is asking, from the headers the asking isoshelf sets.
// Anything in them came from another machine, so it is kept short, stripped
// of line breaks, and never used as a path.
func askerFrom(id, name, address string) Served {
	clean := func(s string, max int) string {
		s = strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return -1
			}
			return r
		}, strings.TrimSpace(s))
		if len(s) > max {
			s = s[:max]
		}
		return s
	}
	return Served{ID: clean(id, 64), Name: clean(name, 80), Address: clean(address, 64)}
}

var errNoServedRecord = errors.New("nothing served yet")

// servedRecord is what the page shows, or errNoServedRecord.
func (s *Server) servedRecord() ([]Served, error) {
	if s.cfg.Dirs.Config == "" {
		return nil, errNoServedRecord
	}
	if _, err := os.Stat(filepath.Join(s.cfg.Dirs.Config, servedFileName)); errors.Is(err, fs.ErrNotExist) {
		return nil, errNoServedRecord
	}
	servedMu.Lock()
	defer servedMu.Unlock()
	return loadServed(s.cfg.Dirs.Config), nil
}
