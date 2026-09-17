package appdir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFind(t *testing.T) {
	drive := t.TempDir()
	app := filepath.Join(drive, "isoshelf")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(app, "isoshelf-linux-amd64")
	userConfig := func() (string, error) { return "/home/zach/.config", nil }
	temp := func() string { return "/tmp" }

	// Without the marker: installed mode.
	got, err := find(exe, userConfig, temp)
	if err != nil {
		t.Fatal(err)
	}
	want := Dirs{
		App:    app,
		Config: filepath.Join("/home/zach/.config", "isoshelf"),
		Temp:   filepath.Join("/tmp", "isoshelf"),
	}
	if got != want {
		t.Errorf("installed:\n got %+v\nwant %+v", got, want)
	}

	// A folder called "portable" doesn't count; a file does.
	if err := os.Mkdir(filepath.Join(app, MarkerFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _ := find(exe, userConfig, temp); got.Portable {
		t.Error("a folder named portable turned on portable mode")
	}
	if err := os.Remove(filepath.Join(app, MarkerFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, MarkerFile), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	failing := func() (string, error) { return "", errors.New("no home folder") }
	got, err = find(exe, failing, temp) // portable mode never asks for the config dir
	if err != nil {
		t.Fatal(err)
	}
	want = Dirs{
		Portable:      true,
		App:           app,
		Config:        app,
		Temp:          filepath.Join(app, "tmp"),
		DefaultTarget: drive,
	}
	if got != want {
		t.Errorf("portable:\n got %+v\nwant %+v", got, want)
	}
}
