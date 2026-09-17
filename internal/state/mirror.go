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
	return writeJSON(filepath.Join(dir, s.TargetID+".json"), Mirror{
		TargetID: s.TargetID,
		Path:     abs,
		Profile:  s.Profile,
		Tracks:   s.Tracks,
		History:  s.History,
		SavedAt:  now.UTC(),
	})
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
