// Package appdir decides where isoshelf keeps its own files: next to the
// executable in portable mode, or in the user's config folder otherwise.
package appdir

import (
	"os"
	"path/filepath"
)

// MarkerFile turns on portable mode when it sits next to the executable.
const MarkerFile = "portable"

// Dirs are the folders isoshelf uses for its own files.
type Dirs struct {
	Portable bool
	// App is the folder holding the executable. Scans skip it.
	App string
	// Config holds settings, the user's catalog copy and logs, plus the state
	// mirrors when not in portable mode.
	Config string
	// Temp is for temporary files. Downloads are staged in the target instead.
	Temp string
	// DefaultTarget is the folder to use when none is given. In portable mode
	// it's the folder holding the app folder, normally the drive itself.
	DefaultTarget string
}

// Find locates the folders for the running executable.
func Find() (Dirs, error) {
	exe, err := os.Executable()
	if err != nil {
		return Dirs{}, err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return find(exe, os.UserConfigDir, os.TempDir)
}

func find(exe string, userConfigDir func() (string, error), tempDir func() string) (Dirs, error) {
	app := filepath.Dir(exe)
	if info, err := os.Stat(filepath.Join(app, MarkerFile)); err == nil && info.Mode().IsRegular() {
		return Dirs{
			Portable:      true,
			App:           app,
			Config:        app,
			Temp:          filepath.Join(app, "tmp"),
			DefaultTarget: filepath.Dir(app),
		}, nil
	}
	config, err := userConfigDir()
	if err != nil {
		return Dirs{}, err
	}
	return Dirs{
		App:    app,
		Config: filepath.Join(config, "isoshelf"),
		Temp:   filepath.Join(tempDir(), "isoshelf"),
	}, nil
}
