package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/scan"
)

// mirrorDir is the folder for mirrors inside the app's config folder.
const mirrorDir = "targets"

// Mirror is the part of a target's state copied to the app's config folder, so
// the usual set and history survive a dead drive. Portable mode doesn't write
// mirrors, because it must not write to the host computer.
type Mirror struct {
	TargetID string `json:"target_id"`
	// Path is where the target was last seen.
	Path    string           `json:"path"`
	Profile scan.Profile     `json:"profile"`
	Tracks  map[string]Track `json:"tracks"`
	History []ScanRecord     `json:"history"`
	SavedAt time.Time        `json:"saved_at"`
	// Files and Bytes are what the folder held when the mirror was saved, so
	// the list of folders isoshelf remembers can say how big each one is
	// without going near a drive that may not be plugged in. Both are absent
	// in mirrors written before v0.4.3, and then they are simply not shown.
	Files int   `json:"files,omitempty"`
	Bytes int64 `json:"bytes,omitempty"`
}

// UsualSet returns the usual set as of the last time the mirror was saved.
func (m Mirror) UsualSet() []string {
	return usualSet(m.Tracks, m.History)
}

// SaveMirror writes the mirror for target to <configDir>/targets/<id>.json.
func (s *State) SaveMirror(configDir, target string, now time.Time) error {
	if !targetIDPattern.MatchString(s.TargetID) {
		return fmt.Errorf("invalid target id %q", s.TargetID)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	dir := filepath.Join(configDir, mirrorDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var bytes int64
	for _, f := range s.Files {
		bytes += f.Size
	}
	return writeJSON(filepath.Join(dir, s.TargetID+".json"), Mirror{
		TargetID: s.TargetID,
		Path:     abs,
		Profile:  s.Profile,
		Tracks:   s.Tracks,
		History:  s.History,
		SavedAt:  now.UTC(),
		Files:    len(s.Files),
		Bytes:    bytes,
	})
}

// Forget removes the mirror for id. It is the copy in isoshelf's own folder
// and nothing else: the folder keeps its own records, its archive and every
// image in it, so forgetting a folder here is forgetting a row in a list.
func Forget(configDir, id string) error {
	if !targetIDPattern.MatchString(id) {
		return fmt.Errorf("invalid target id %q", id)
	}
	err := os.Remove(filepath.Join(configDir, mirrorDir, id+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil // already gone, which is what was wanted
	}
	return err
}

// LoadMirrors reads every mirror in configDir, most recently saved first.
func LoadMirrors(configDir string) ([]Mirror, error) {
	entries, err := os.ReadDir(filepath.Join(configDir, mirrorDir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var mirrors []Mirror
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		name := filepath.Join(configDir, mirrorDir, e.Name())
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		var m Mirror
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		mirrors = append(mirrors, m)
	}
	slices.SortFunc(mirrors, func(a, b Mirror) int { return b.SavedAt.Compare(a.SavedAt) })
	return mirrors, nil
}
