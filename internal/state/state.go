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
	"os"
	"path/filepath"
	"regexp"
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
	// Past remembers images that used to be here, newest first.
	Past []ArchiveEntry `json:"past,omitempty"`
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
	// FirstSeen is when a scan first found this file, for files that turned
	// up after the folder's first scan. Where the system records when a file
	// was created, that is used instead; this covers the rest.
	FirstSeen time.Time `json:"first_seen,omitzero"`
}

func (r FileRecord) current(f scan.File) bool {
	return r.Size == f.Size && r.ModTime.Equal(f.ModTime)
}

// Track holds the user's settings for one entry.
type Track struct {
	// KeepOld keeps old files after an update. The default (false) replaces
	// them once the new file is downloaded and verified. Superseded by
	// OldFiles, kept so older state files still make sense; see Choice.
	KeepOld bool `json:"keep_old,omitempty"`
	// OldFiles says what happens to this image's old files after an update:
	// "replace", "archive" or "keep". Empty means not chosen yet; see Choice.
	OldFiles string `json:"old_files,omitempty"`
	// Starred puts the entry in the usual set even if it's not on the target.
	Starred bool `json:"starred,omitempty"`
}

// Choice says what happens to this image's old files after an update:
// "replace", "archive" or "keep". It falls back to the older keep_old
// setting, and to "replace" when neither was set.
func (t Track) Choice() string {
	if t.OldFiles != "" {
		return t.OldFiles
	}
	if t.KeepOld {
		return "keep"
	}
	return "replace"
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

// Load reads the state of target from its own folder. A target without state
// gets a new one with the suggested profile; nothing is written until Save.
func Load(target string) (*State, error) { return Home("").Load(target) }

// Load reads the state of target from wherever this Home keeps it.
func (h Home) Load(target string) (*State, error) {
	name, err := h.File(target)
	if err != nil {
		return nil, err
	}
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
func (s *State) Save(target string) error { return Home("").Save(s, target) }

// Save writes the state to wherever this Home keeps target's records.
func (h Home) Save(s *State, target string) error {
	name, err := h.File(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return err
	}
	return writeJSON(name, s)
}

// NeedsHash returns the scanned files that need hashing and have no hash for
// their current contents. Normally that means images whose filename never
// changes, where a changed published checksum is how an update shows up.
// Call it after RecordScan.
// all, when set, hashes every recognized file rather than only those. That
// is what sharing needs: a file isoshelf downloaded has its hash from the
// download, but one copied in by hand has none, and a hash is how another
// isoshelf asks for a particular file rather than a name.
func (s *State) NeedsHash(res *scan.Result, cat *catalog.Catalog, all bool) []scan.File {
	var out []scan.File
	for _, f := range res.Files {
		rec, ok := s.Files[f.Path]
		if !ok || !rec.current(f) || rec.SHA256 != "" {
			continue
		}
		if all && rec.Entry != "" {
			out = append(out, f)
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
	return writeFileAtomic(name, append(data, '\n'))
}

// writeFileAtomic writes data through a temporary file in the same folder and
// renames it into place, so the file at name is either the old one or the new
// one and never half of either.
func writeFileAtomic(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), filepath.Base(name)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // fails harmlessly after the rename
	if _, err := tmp.Write(data); err != nil {
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
