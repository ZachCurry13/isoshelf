// Package state keeps what isoshelf knows about a target folder, in
// <target>/.isoshelf/state.json: its profile, a record per file, per-track
// settings and scan history. A State is not safe for concurrent use.
package state

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/catalog"
	"github.com/ZachCurry13/isoshelf/internal/scan"
)

const (
	// DirName is isoshelf's folder inside a target. It is the only place
	// isoshelf writes to during a scan or check.
	DirName = ".isoshelf"

	fileName      = "state.json"
	formatVersion = 1
	maxHistory    = 100

	// The usual set is the starred tracks plus entries seen in at least
	// usualMinSeen of the last usualWindow scans.
	usualWindow  = 10
	usualMinSeen = 2
)

var targetIDPattern = regexp.MustCompile(`^[A-Za-z0-9]{8,64}$`)

// State is everything isoshelf remembers about one target.
type State struct {
	Version int `json:"version"`
	// TargetID tells targets apart when their path or drive letter changes.
	TargetID string       `json:"target_id"`
	Profile  scan.Profile `json:"profile"`
	// Files holds a record per file from the last scan, keyed by its path
	// relative to the target.
	Files map[string]FileRecord `json:"files"`
	// Tracks holds per-entry settings, keyed by entry id. Entries with
	// default settings are left out.
	Tracks  map[string]Track `json:"tracks"`
	History []ScanRecord     `json:"history"`
}

// FileRecord is what isoshelf knows about one file. It stays valid while the
// file's size and modification time are unchanged.
type FileRecord struct {
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	// Entry and Version come from the catalog match, or from the user when
	// Assigned is set.
	Entry    string `json:"entry,omitempty"`
	Version  string `json:"version,omitempty"`
	Assigned bool   `json:"assigned,omitempty"`
	// SHA256 is the file's hash in lowercase hex, once computed.
	SHA256   string    `json:"sha256,omitempty"`
	HashedAt time.Time `json:"hashed_at,omitzero"`
	// SourceURL and PlacedAt are set when isoshelf downloaded the file.
	SourceURL string    `json:"source_url,omitempty"`
	PlacedAt  time.Time `json:"placed_at,omitzero"`
}

func (r FileRecord) current(f scan.File) bool {
	return r.Size == f.Size && r.ModTime.Equal(f.ModTime)
}

// Track holds the user's settings for one entry.
type Track struct {
	// KeepOld keeps old files after an update. The default (false) replaces
	// them once the new file is downloaded and verified.
	KeepOld bool `json:"keep_old,omitempty"`
	// Starred puts the entry in the usual set even if it's not on the target.
	Starred bool `json:"starred,omitempty"`
}

// ScanRecord is one entry in the scan history.
type ScanRecord struct {
	Time time.Time `json:"time"`
	// Entries are the ids of the entries found, sorted.
	Entries      []string `json:"entries"`
	Unrecognized int      `json:"unrecognized,omitempty"`
}

// New returns an empty state with a fresh target id.
func New(profile scan.Profile) *State {
	return &State{
		Version:  formatVersion,
		TargetID: rand.Text(),
		Profile:  profile,
		Files:    map[string]FileRecord{},
		Tracks:   map[string]Track{},
	}
}

// Load reads the state of target. A target without state gets a new one with
// the suggested profile; nothing is written until Save.
func Load(target string) (*State, error) {
	name := filepath.Join(target, DirName, fileName)
	data, err := os.ReadFile(name)
	if errors.Is(err, fs.ErrNotExist) {
		return New(scan.SuggestProfile(target)), nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	switch {
	case s.Version > formatVersion:
		return nil, fmt.Errorf("%s was written by a newer version of isoshelf", name)
	case s.Version != formatVersion:
		return nil, fmt.Errorf("%s: unsupported version %d", name, s.Version)
	case !targetIDPattern.MatchString(s.TargetID):
		return nil, fmt.Errorf("%s: invalid target_id %q", name, s.TargetID)
	}
	if _, err := scan.ParseProfile(string(s.Profile)); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if s.Files == nil {
		s.Files = map[string]FileRecord{}
	}
	if s.Tracks == nil {
		s.Tracks = map[string]Track{}
	}
	return &s, nil
}

// Save writes the state to <target>/.isoshelf/state.json. The old file is
// only replaced once the new one is completely written.
func (s *State) Save(target string) error {
	dir := filepath.Join(target, DirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, fileName), s)
}

// RecordScan updates the file records from a scan and adds the scan to the
// history. A file that changed or disappeared loses its record, including its
// hash and any assignment. Files the scan couldn't read keep their records.
func (s *State) RecordScan(res *scan.Result, now time.Time) {
	files := make(map[string]FileRecord, len(res.Files))
	for path, rec := range s.Files {
		for _, p := range res.Problems {
			if path == p.Path || strings.HasPrefix(path, p.Path+"/") {
				files[path] = rec
			}
		}
	}

	found := map[string]bool{}
	unrecognized := 0
	for _, f := range res.Files {
		rec, ok := s.Files[f.Path]
		if !ok || !rec.current(f) {
			rec = FileRecord{Size: f.Size, ModTime: f.ModTime}
		}
		if !rec.Assigned {
			rec.Entry, rec.Version = "", ""
			if len(f.Matches) == 1 {
				rec.Entry, rec.Version = f.Matches[0].Entry.ID, f.Matches[0].Version
			}
		}
		if rec.Entry == "" {
			unrecognized++
		} else {
			found[rec.Entry] = true
		}
		files[f.Path] = rec
	}
	s.Files = files

	s.History = append(s.History, ScanRecord{
		Time:         now.UTC(),
		Entries:      slices.Sorted(maps.Keys(found)),
		Unrecognized: unrecognized,
	})
	if extra := len(s.History) - maxHistory; extra > 0 {
		s.History = slices.Clone(s.History[extra:])
	}
}

// Assign records that the file at path belongs to an entry the catalog didn't
// match by name, such as a renamed download. The caller checks that the entry
// exists. The assignment lasts until the file changes.
func (s *State) Assign(path, entry, version string) error {
	rec, ok := s.Files[path]
	if !ok {
		return fmt.Errorf("%s is not in the last scan", path)
	}
	rec.Entry, rec.Version, rec.Assigned = entry, version, true
	s.Files[path] = rec
	return nil
}

// Track returns the settings for an entry.
func (s *State) Track(entry string) Track {
	return s.Tracks[entry]
}

// SetTrack stores the settings for an entry.
func (s *State) SetTrack(entry string, t Track) {
	if t == (Track{}) {
		delete(s.Tracks, entry)
		return
	}
	s.Tracks[entry] = t
}

// UsualSet returns the ids of the entries normally kept on this target: the
// starred ones, plus those seen in at least 2 of the last 10 scans.
func (s *State) UsualSet() []string {
	return usualSet(s.Tracks, s.History)
}

// Missing returns the usual-set entries the latest scan didn't find.
func (s *State) Missing() []string {
	var last []string
	if len(s.History) > 0 {
		last = s.History[len(s.History)-1].Entries
	}
	var missing []string
	for _, id := range s.UsualSet() {
		if !slices.Contains(last, id) {
			missing = append(missing, id)
		}
	}
	return missing
}

func usualSet(tracks map[string]Track, history []ScanRecord) []string {
	seen := map[string]int{}
	for _, rec := range history[max(0, len(history)-usualWindow):] {
		for _, id := range rec.Entries {
			seen[id]++
		}
	}
	usual := map[string]bool{}
	for id, n := range seen {
		if n >= usualMinSeen {
			usual[id] = true
		}
	}
	for id, t := range tracks {
		if t.Starred {
			usual[id] = true
		}
	}
	return slices.Sorted(maps.Keys(usual))
}

// NeedsHash returns the scanned files that belong to a fixed-name entry and
// have no hash for their current contents. For those images, a changed
// published checksum is how updates show up. Call it after RecordScan.
func (s *State) NeedsHash(res *scan.Result, cat *catalog.Catalog) []scan.File {
	var out []scan.File
	for _, f := range res.Files {
		rec, ok := s.Files[f.Path]
		if !ok || !rec.current(f) || rec.SHA256 != "" {
			continue
		}
		if e := cat.Entry(rec.Entry); e != nil && e.FixedName {
			out = append(out, f)
		}
	}
	return out
}

// HashFiles hashes files one at a time and records each result, unless the
// file changed while it was being read. It goes on after a file fails, but
// stops when ctx is cancelled; hashes finished before that are kept. progress,
// if not nil, is called with the bytes read so far of the current file.
func (s *State) HashFiles(ctx context.Context, target string, files []scan.File, progress func(f scan.File, done int64)) error {
	var errs []error
	for _, f := range files {
		name := filepath.Join(target, filepath.FromSlash(f.Path))
		sum, err := HashFile(ctx, name, func(done int64) {
			if progress != nil {
				progress(f, done)
			}
		})
		if err == nil {
			var info os.FileInfo
			if info, err = os.Stat(name); err == nil && info.Size() == f.Size && info.ModTime().Equal(f.ModTime) {
				s.setHash(f, sum, time.Now())
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", f.Path, err))
		}
	}
	return errors.Join(errs...)
}

func (s *State) setHash(f scan.File, sum string, now time.Time) {
	rec, ok := s.Files[f.Path]
	if !ok || !rec.current(f) {
		return
	}
	rec.SHA256, rec.HashedAt = sum, now.UTC()
	s.Files[f.Path] = rec
}

// HashFile returns the SHA-256 of a file as lowercase hex. It stops when ctx is
// cancelled, and calls progress (if not nil) with the bytes read so far.
func HashFile(ctx context.Context, name string, progress func(done int64)) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, 1<<20)
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := f.Read(buf)
		h.Write(buf[:n])
		done += int64(n)
		if progress != nil && n > 0 {
			progress(done)
		}
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

// writeJSON writes v as indented JSON through a temporary file in the same
// folder, so a crash never leaves a half-written file behind.
func writeJSON(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), name)
}
