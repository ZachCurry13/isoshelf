package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/settings"
)

// The page sends one switch at a time, so every other choice has to survive
// each request untouched.
func TestSettingsKeepEachOther(t *testing.T) {
	dirs := testDirs(t)
	s := newServer(t, dirs, "")

	request(t, s, http.MethodPost, "/api/settings", map[string]any{"theme": "dark"})
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"larger_text": true})
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"old_files": "archive"})
	rec := request(t, s, http.MethodPost, "/api/settings", map[string]any{"high_contrast": true})

	got := decode[stateJSON](t, rec)
	if got.Appearance.Theme != settings.ThemeDark {
		t.Errorf("theme is %q, want dark", got.Appearance.Theme)
	}
	if !got.Appearance.LargerText || !got.Appearance.HighContrast {
		t.Errorf("appearance is %+v, want larger text and high contrast on", got.Appearance)
	}
	if got.OldFiles != settings.OldArchive {
		t.Errorf("old files is %q, want archive", got.OldFiles)
	}
}

// A choice the command line wrote must not be erased by a switch flicked on
// the page. Both read and write the same file, so both must keep every field
// in it.
func TestSettingsKeepFieldsTheWebUIDoesNotShow(t *testing.T) {
	dirs := testDirs(t)
	if err := settings.Save(dirs.Config, settings.Settings{
		FolderRecords: map[string]settings.Records{
			"/a/drive": {Location: settings.Elsewhere, Dir: "/somewhere"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, "")
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"theme": "light"})

	saved := settings.Load(dirs.Config)
	if got := saved.RecordsFor("/a/drive"); got.Location != settings.Elsewhere || got.Dir != "/somewhere" {
		t.Errorf("saving a theme lost where a folder's records are kept: %+v", saved)
	}
	if saved.Appearance.Theme != settings.ThemeLight {
		t.Errorf("theme is %q, want light", saved.Appearance.Theme)
	}
}

// Reset puts the switches back without forgetting the folder someone is
// working in or the folders they pinned.
func TestResetKeepsFoldersAndBookmarks(t *testing.T) {
	dirs := testDirs(t)
	off := false
	if err := settings.Save(dirs.Config, settings.Settings{
		Target:      "/images",
		Bookmarks:   []string{"/nas/iso"},
		CatalogAuto: &off,
		OldFiles:    settings.OldArchive,
		Appearance:  settings.Appearance{Theme: settings.ThemeDark, LargerText: true},
	}); err != nil {
		t.Fatal(err)
	}
	s := newServer(t, dirs, "")
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"reset": true})

	saved := settings.Load(dirs.Config)
	if saved.Target != "/images" || len(saved.Bookmarks) != 1 {
		t.Errorf("reset forgot the folders: %+v", saved)
	}
	if saved.CatalogAuto != nil || saved.OldFiles != "" || saved.Appearance != (settings.Appearance{}) {
		t.Errorf("reset left a choice behind: %+v", saved)
	}
}

// A theme nobody has heard of, hand-typed into the file or sent by something
// else, must not reach the page.
func TestUnknownThemeBecomesTheSystemOne(t *testing.T) {
	dirs := testDirs(t)
	s := newServer(t, dirs, "")
	rec := request(t, s, http.MethodPost, "/api/settings", map[string]any{"theme": "neon"})
	if got := decode[stateJSON](t, rec).Appearance.Theme; got != settings.ThemeSystem {
		t.Errorf("theme is %q, want the system one", got)
	}
}

// The settings file is the one the command line reads, under the name
// internal/settings gives it.
func TestSettingsGoInTheSharedFile(t *testing.T) {
	dirs := testDirs(t)
	s := newServer(t, dirs, "")
	request(t, s, http.MethodPost, "/api/settings", map[string]any{"reduce_motion": true})

	data, err := os.ReadFile(filepath.Join(dirs.Config, settings.FileName))
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]any
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if _, ok := file["appearance"]; !ok {
		t.Errorf("%s has no appearance: %s", settings.FileName, data)
	}
}

// A settings file written by an older isoshelf still says what should happen
// to a replaced file, in the words the download code uses. That answer must
// survive the move to the page's own words.
func TestOldReplaceAnswerIsCarriedOver(t *testing.T) {
	dirs := testDirs(t)
	if err := os.WriteFile(filepath.Join(dirs.Config, settings.FileName),
		[]byte(`{"replace_action":"move-aside"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := settings.Load(dirs.Config).OldFiles; got != settings.OldArchive {
		t.Errorf("old files is %q, want archive", got)
	}
}
