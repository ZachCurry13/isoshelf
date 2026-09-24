package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appupdate"
)

// These build the real program and run it, because what they check is
// exactly what can't be faked: that a program on trial after an update comes
// back on the port the page is on, with the same link, and either keeps the
// update or undoes it - on Windows, where a running program can be renamed
// but not deleted, as well as on Linux. Nothing here goes near the network:
// every request the programs might make goes to a proxy that isn't there.

const handoverToken = "handover-test-token-0123456789"

func TestAnUpdatedProgramComesBackOnTheSamePort(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the program")
	}
	suffix := appupdate.Suffix(runtime.GOOS, runtime.GOARCH)
	dir := tidyTempDir(t)
	exe := buildIsoshelf(t, filepath.Join(dir, "isoshelf-"+suffix), "v9.0.1")
	old := exe + ".old"
	os.WriteFile(old, []byte("the program the update replaced"), 0o755)
	stage := writeSwap(t, dir, exe, old)

	port := freePort(t)
	// The old program may hold the port for a moment after it hands over.
	hold, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	go func() { time.Sleep(700 * time.Millisecond); hold.Close() }()

	var out bytes.Buffer
	cmd := exec.Command(exe, "ui", "--no-browser")
	cmd.Env = childEnv(t, appupdate.Handover{Port: port, Stage: stage, From: "v9.0.0"})
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })

	base := "http://127.0.0.1:" + strconv.Itoa(port)
	waitFor(t, "the new program to listen on the old port", func() bool {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			resp.Body.Close()
		}
		return err == nil && resp.StatusCode == http.StatusOK
	})
	// The link the page was opened with still gets in.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(base + "/?token=" + handoverToken)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("the old link answered %d, want the redirect that signs it in", resp.StatusCode)
	}
	// And it keeps the update: the old program and the staging folder go.
	waitFor(t, "the update to be kept", func() bool {
		return gone(old) && gone(stage)
	})
	if !bytes.Contains(out.Bytes(), []byte("Updated from v9.0.0.")) {
		t.Errorf("it didn't say it was updated:\n%s", out.Bytes())
	}
}

func TestAProgramThatCantComeUpPutsTheOldOneBack(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the program")
	}
	suffix := appupdate.Suffix(runtime.GOOS, runtime.GOARCH)
	dir := tidyTempDir(t)
	exe := buildIsoshelf(t, filepath.Join(dir, "isoshelf-"+suffix), "v9.0.1")
	old := buildIsoshelf(t, exe+".old", "v9.0.0")
	oldSum := fileSum(t, old)
	stage := writeSwap(t, dir, exe, old)

	// Somebody else has the port for good, so neither program can listen.
	port := freePort(t)
	hold, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		t.Fatal(err)
	}
	defer hold.Close()

	var out bytes.Buffer
	cmd := exec.Command(exe, "ui", "--no-browser")
	cmd.Env = childEnv(t, appupdate.Handover{Port: port, Stage: stage, From: "v9.0.0"})
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("it never gave up:\n%s", out.Bytes())
	}

	if got := fileSum(t, exe); got != oldSum {
		t.Errorf("the program in place isn't the old one after undoing:\n%s", out.Bytes())
	}
	if !gone(old) {
		t.Error("the old program is still set aside as well as back in place")
	}
	if !bytes.Contains(out.Bytes(), []byte("didn't start")) {
		t.Errorf("it didn't say the update was undone:\n%s", out.Bytes())
	}
}

// buildIsoshelf builds this program to path as version, with the port wait
// short enough for a test.
func buildIsoshelf(t *testing.T, path, version string) string {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", path,
		"-ldflags", "-X main.version="+version+" -X main.listenWait=2s", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building: %v\n%s", err, out)
	}
	return path
}

// writeSwap writes the record Swap leaves: the new program at exe, the old
// one set aside at old.
func writeSwap(t *testing.T, dir, exe, old string) string {
	t.Helper()
	stage := filepath.Join(dir, appupdate.StageDir)
	os.MkdirAll(stage, 0o755)
	data, _ := json.Marshal(appupdate.Swapped{
		From: "v9.0.0", To: "v9.0.1", Run: exe, Dir: stage,
		Moves: []appupdate.Move{{Was: exe, Backup: old, Now: exe}},
	})
	if err := os.WriteFile(filepath.Join(stage, "swapped.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return stage
}

// childEnv is the environment the program on trial starts with: its own
// settings folders, no way out to the network, and the handover.
func childEnv(t *testing.T, h appupdate.Handover) []string {
	t.Helper()
	home := t.TempDir()
	env := append(os.Environ(),
		"APPDATA="+home, "LOCALAPPDATA="+home, "XDG_CONFIG_HOME="+home, "HOME="+home,
		"HTTP_PROXY=http://127.0.0.1:1", "HTTPS_PROXY=http://127.0.0.1:1", "NO_PROXY=",
	)
	return h.Environ(env, handoverToken)
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(200 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func gone(path string) bool {
	_, err := os.Stat(path)
	return errors.Is(err, fs.ErrNotExist)
}

func fileSum(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(data)
}

// tidyTempDir is a temporary folder whose programs may still be exiting when
// the test ends; on Windows a running program can't be deleted, so removing
// it waits for them.
func tidyTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "isoshelf-handover-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for try := 0; try < 50 && os.RemoveAll(dir) != nil; try++ {
			time.Sleep(200 * time.Millisecond)
		}
	})
	return dir
}
