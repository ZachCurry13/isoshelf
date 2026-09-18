// Package settings holds the choices that belong to this computer rather
// than to any one folder: which folder was open last, whether the catalog
// keeps itself up to date, bookmarked folders, and where isoshelf keeps what
// it learns. The web UI and the command line read the same file, so they
// never disagree.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileName is the settings file inside the config folder.
const FileName = "ui.json"

// Where isoshelf keeps what it has learned about a folder.
const (
	// InFolder is the default: a .isoshelf folder inside the images folder.
	// The drive then carries its own memory, so plugging it into another
	// computer keeps everything isoshelf worked out.
	InFolder = "folder"
	// WithApp keeps it beside the isoshelf program instead, which suits a
	// drive isoshelf shouldn't write to.
	WithApp = "app"
	// Elsewhere keeps it in a folder the user names.
	Elsewhere = "custom"
)

// Settings is the file's contents. Every field is optional: a missing file
// means the defaults.
type Settings struct {
	Target string `json:"target,omitempty"`
	// CatalogAuto is nil until the user says either way; the default is on.
	CatalogAuto *bool `json:"catalog_auto,omitempty"`
	// Bookmarks are folders pinned in the chooser.
	Bookmarks []string `json:"bookmarks,omitempty"`
	// StateLocation is InFolder, WithApp or Elsewhere. StateDir is the folder
	// for Elsewhere.
	StateLocation string `json:"state_location,omitempty"`
	StateDir      string `json:"state_dir,omitempty"`
}

// Load reads the settings, or returns the defaults when there are none. A
// damaged file is treated as no file: settings are conveniences, and losing
// them must never stop isoshelf from starting.
func Load(configDir string) Settings {
	var s Settings
	if configDir == "" {
		return s
	}
	if data, err := os.ReadFile(filepath.Join(configDir, FileName)); err == nil {
		json.Unmarshal(data, &s)
	}
	return s
}

// Save writes the settings. Failing to save is not worth stopping for, so the
// error is returned for logging rather than for handling.
func Save(configDir string, s Settings) error {
	if configDir == "" {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, FileName), data, 0o644)
}

// StateDirFor returns the folder a target's state belongs in: "" for inside
// the folder itself, which is what state.Load and state.Save expect by
// default. appDir is where the isoshelf program lives.
func (s Settings) StateDirFor(appDir string) string {
	switch s.StateLocation {
	case WithApp:
		return appDir
	case Elsewhere:
		return s.StateDir
	}
	return ""
}
