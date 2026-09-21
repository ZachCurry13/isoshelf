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

// What happens by default to the copy an update replaces. Each image can be
// given its own answer instead; this is what the ones that haven't been do.
const (
	OldReplace = "replace"
	OldArchive = "archive"
	OldKeep    = "keep"
)

// CleanOldFiles turns anything unexpected into "", which means the default,
// so a hand-edited file can't leave an image with an answer isoshelf doesn't
// understand.
func CleanOldFiles(choice string) string {
	switch choice {
	case OldReplace, OldArchive, OldKeep:
		return choice
	}
	return ""
}

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
	// OldFiles is what happens by default to the copy an update replaces:
	// OldReplace, OldArchive or OldKeep. An image that has been given its own
	// answer in its details panel wins over this one.
	OldFiles string `json:"old_files,omitempty"`
	// ReplaceAction is what versions before v0.3.1 wrote here, in the words
	// the download code uses. Load turns it into OldFiles and forgets it.
	ReplaceAction string `json:"replace_action,omitempty"`
	// Appearance is how the page looks.
	Appearance Appearance `json:"appearance,omitzero"`
}

// Reset returns the settings with every choice back at its default. What is
// not a choice stays: the folder that is open, the folders pinned in the
// chooser, and where isoshelf keeps what it has learned. Someone who presses
// "Reset to defaults" wants the switches back, not their folders forgotten.
func (s Settings) Reset() Settings {
	return Settings{
		Target:        s.Target,
		Bookmarks:     s.Bookmarks,
		StateLocation: s.StateLocation,
		StateDir:      s.StateDir,
	}
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
	// Before v0.3.1 the answer was kept in the download code's words, and
	// nothing on the page read it. Carry it over once, then let it go.
	if s.OldFiles == "" && s.ReplaceAction == "move-aside" {
		s.OldFiles = OldArchive
	} else if s.OldFiles == "" && s.ReplaceAction == "delete" {
		s.OldFiles = OldReplace
	}
	s.ReplaceAction = ""
	s.Appearance.Theme = CleanTheme(s.Appearance.Theme)
	s.OldFiles = CleanOldFiles(s.OldFiles)
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
