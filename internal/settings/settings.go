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
	"time"
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

// How often isoshelf updates the images by itself.
const (
	EveryDay  = "day"
	EveryWeek = "week"
)

// CleanEvery returns how often, or EveryDay for anything it doesn't know.
func CleanEvery(choice string) string {
	if choice == EveryWeek {
		return EveryWeek
	}
	return EveryDay
}

// Every is how long between runs.
func (s Settings) Every() time.Duration {
	if CleanEvery(s.AutoUpdateEvery) == EveryWeek {
		return 7 * 24 * time.Hour
	}
	return 24 * time.Hour
}

// Peer is another isoshelf on the network worth asking before going to the
// internet - usually the one on the machine the images already live on.
type Peer struct {
	// Address is host:port, without a scheme.
	Address string `json:"address,omitempty"`
	// User and Password sign in to it. They are kept here in the clear,
	// which is why the page says so: this is a second copy of a password for
	// a machine on the same network, and somebody who can read this file can
	// read the login for this isoshelf too.
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	// Off turns the peer off without forgetting the address.
	Off bool `json:"off,omitempty"`
}

// Use says whether to ask this peer before downloading.
func (p Peer) Use() bool { return p.Address != "" && !p.Off }

// Where isoshelf keeps what it has learned about a folder.
const (
	// InFolder is the default: a .isoshelf folder inside the images folder.
	// The drive then carries its own memory, so plugging it into another
	// computer keeps everything isoshelf worked out.
	InFolder = "folder"
	// WithApp keeps it in isoshelf's own folder instead - beside the program
	// in portable mode, in the user's config folder otherwise - which suits a
	// drive isoshelf shouldn't be writing to.
	WithApp = "app"
	// Elsewhere keeps it in a folder the user names.
	Elsewhere = "custom"
)

// CleanRecordsLocation returns choice if it is one isoshelf knows, else "",
// which means the default.
func CleanRecordsLocation(choice string) string {
	switch choice {
	case InFolder, WithApp, Elsewhere:
		return choice
	}
	return ""
}

// Records is one folder's answer to where its records are kept.
type Records struct {
	Location string `json:"location"`
	// Dir is the folder for Elsewhere, and ignored otherwise.
	Dir string `json:"dir,omitempty"`
}

// RecordsFor is the answer for folder: the one it was given, or the default.
func (s Settings) RecordsFor(folder string) Records {
	if r, ok := s.FolderRecords[key(folder)]; ok {
		if CleanRecordsLocation(r.Location) != "" {
			return r
		}
	}
	return Records{Location: InFolder}
}

// SetRecordsFor records the answer for folder. The default is stored as an
// absence, so the file doesn't fill up with folders that chose nothing.
func (s *Settings) SetRecordsFor(folder string, r Records) {
	if s.FolderRecords == nil {
		s.FolderRecords = map[string]Records{}
	}
	if CleanRecordsLocation(r.Location) == "" || r.Location == InFolder {
		delete(s.FolderRecords, key(folder))
		return
	}
	s.FolderRecords[key(folder)] = r
}

// RecordsHome is the folder holding folder's records, or "" when the folder
// keeps its own - which is the default, and what an unusable answer falls
// back to. configDir is isoshelf's own folder. The result is a path for
// state.Home: the state package doesn't read settings, and this one doesn't
// read state.
func (s Settings) RecordsHome(folder, configDir string) string {
	r := s.RecordsFor(folder)
	switch r.Location {
	case WithApp:
		return configDir
	case Elsewhere:
		if filepath.IsAbs(r.Dir) {
			return r.Dir
		}
	}
	return ""
}

// key is how a folder's path is written in the settings file: cleaned, so
// that the same folder named two ways is one entry. Case is kept - two
// folders differing only in case are the same on Windows and different on
// Linux, and merging them would hand a folder another's records.
func key(folder string) string {
	if abs, err := filepath.Abs(folder); err == nil {
		return abs
	}
	return filepath.Clean(folder)
}

// Settings is the file's contents. Every field is optional: a missing file
// means the defaults.
type Settings struct {
	Target string `json:"target,omitempty"`
	// CatalogAuto is nil until the user says either way; the default is on.
	CatalogAuto *bool `json:"catalog_auto,omitempty"`
	// Bookmarks are folders pinned in the chooser.
	Bookmarks []string `json:"bookmarks,omitempty"`
	// FolderRecords is where each folder's records are kept, keyed by the
	// folder's path, for the folders whose answer isn't the default. A folder
	// that isn't listed keeps its own records, inside itself.
	FolderRecords map[string]Records `json:"folder_records,omitempty"`
	// OldFiles is what happens by default to the copy an update replaces:
	// OldReplace, OldArchive or OldKeep. An image that has been given its own
	// answer in its details panel wins over this one.
	OldFiles string `json:"old_files,omitempty"`
	// ReplaceAction is what versions before v0.3.1 wrote here, in the words
	// the download code uses. Load turns it into OldFiles and forgets it.
	ReplaceAction string `json:"replace_action,omitempty"`
	// AutoCheck is nil until the user says either way; the default is on.
	// When it is off, isoshelf only goes online when asked to.
	AutoCheck *bool `json:"auto_check,omitempty"`
	// AutoUpdate is whether isoshelf updates the images by itself: on a
	// schedule it checks, downloads, verifies and puts the new file in place,
	// doing with the old copy what OldFiles says and leaving a pinned one be.
	// Nil and false are both off - this one is never on by default, because
	// it changes somebody's drive while they aren't looking.
	AutoUpdate *bool `json:"auto_update,omitempty"`
	// AutoUpdateEvery is how often: EveryDay or EveryWeek. Anything else
	// means EveryDay.
	AutoUpdateEvery string `json:"auto_update_every,omitempty"`
	// AutoUpdateLast is when isoshelf last updated the images by itself, so
	// a restart doesn't start another one straight away.
	AutoUpdateLast time.Time `json:"auto_update_last,omitzero"`
	// ArchiveAfter is how many days a file waits in the archive before
	// isoshelf deletes it: 7, 30, 90, or 0 for never.
	//
	// Zero is the default and has to be, because this is the one setting
	// that throws away something somebody might still want. The archive is
	// the undo for every removal and every replaced file, so a timer that
	// started deleting the moment isoshelf was upgraded would break the rule
	// the whole program rests on: nothing is deleted unless the user chose
	// it. Choosing a number here is that choice, made once, in advance.
	ArchiveAfter int `json:"archive_after,omitempty"`
	// ShareImages lets another isoshelf on the network copy images from this
	// one. Nil and false are both off: sharing hands whole images to whoever
	// can sign in, which is a different thing from letting them manage the
	// folder, and should be a decision rather than a default.
	ShareImages *bool `json:"share_images,omitempty"`
	// Peer is another isoshelf to look at before downloading from the
	// internet: its address, and what to sign in with.
	Peer Peer `json:"peer,omitzero"`
	// AppUpdateCheck is whether to look for a newer isoshelf. Nil is on.
	AppUpdateCheck *bool `json:"app_update_check,omitempty"`
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
		FolderRecords: s.FolderRecords,
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
