package main

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
)

func tokenEnv(t *testing.T, value string) (*env, appdir.Dirs) {
	t.Helper()
	dirs := appdir.Dirs{Config: filepath.Join(t.TempDir(), "isoshelf")}
	e := &env{
		stdout: io.Discard, stderr: io.Discard,
		getenv: func(k string) string {
			if k == "ISOSHELF_TOKEN" {
				return value
			}
			return ""
		},
	}
	return e, dirs
}

// On a desktop the link is new every run: the browser opens with it and
// nothing has to remember it.
func TestLinkTokenIsFreshOnADesktop(t *testing.T) {
	e, dirs := tokenEnv(t, "")
	first, err := linkToken(e, dirs, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := linkToken(e, dirs, false)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("a desktop reused the same link token")
	}
	if _, err := os.Stat(filepath.Join(dirs.Config, tokenFileName)); err == nil {
		t.Error("a desktop wrote a token file; it has no reason to")
	}
}

// A server keeps its token, because it restarts and a bookmark should still
// work afterwards. And it writes it where only its own user can read it.
func TestLinkTokenSurvivesAServerRestart(t *testing.T) {
	e, dirs := tokenEnv(t, "")
	first, err := linkToken(e, dirs, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 16 {
		t.Errorf("the token isn't much of a secret: %q", first)
	}

	again, err := linkToken(e, dirs, true)
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Errorf("a restart changed the link: %q then %q", first, again)
	}

	path := filepath.Join(dirs.Config, tokenFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("no token file: %v", err)
	}
	// Windows has no Unix permission bits: Go reports 0666 for any regular
	// file there, whatever mode it was written with. The mode still matters
	// everywhere isoshelf runs as a server, and a container is Linux.
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("the token file is %v; it is a password, so it should be 0600", perm)
		}
	}
	saved, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(saved)) != first {
		t.Errorf("the file holds %q, the link uses %q (%v)", saved, first, err)
	}
}

// Whatever is in the environment wins, and nothing is written down.
func TestLinkTokenFromTheEnvironment(t *testing.T) {
	e, dirs := tokenEnv(t, "a-token-somebody-chose-themselves")
	got, err := linkToken(e, dirs, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "a-token-somebody-chose-themselves" {
		t.Errorf("token = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dirs.Config, tokenFileName)); err == nil {
		t.Error("a token given in the environment was written to disk as well")
	}
}

// A short one is refused rather than quietly accepted: it is the only thing
// between a stranger on the network and somebody's images.
func TestLinkTokenRefusesAShortOne(t *testing.T) {
	e, dirs := tokenEnv(t, "hunter2")
	if _, err := linkToken(e, dirs, true); err == nil {
		t.Error("a seven-character token was accepted")
	} else if !strings.Contains(err.Error(), "16") {
		t.Errorf("the complaint doesn't say how long it should be: %v", err)
	}
}

// A token file somebody has emptied or mangled is replaced rather than used.
func TestLinkTokenReplacesAUselessFile(t *testing.T) {
	e, dirs := tokenEnv(t, "")
	if err := os.MkdirAll(dirs.Config, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dirs.Config, tokenFileName)
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := linkToken(e, dirs, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 16 {
		t.Errorf("an empty token file was used as the token: %q", got)
	}
}

func TestFolderTrouble(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "an-image.iso")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A folder is fine.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := folderTrouble(info, nil); got != nil {
		t.Errorf("a real folder: %v", got)
	}

	// A file is a different mistake from a missing one, and says so.
	info, err = os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	got := folderTrouble(info, nil)
	if got == nil || !strings.Contains(got.Error(), "file, not a folder") {
		t.Errorf("a file: %v", got)
	}

	// And whatever stat said is passed through rather than reworded.
	_, statErr := os.Stat(filepath.Join(dir, "nothing-here"))
	if got := folderTrouble(nil, statErr); got != statErr {
		t.Errorf("a missing folder: %v, want the error stat gave", got)
	}
}
