package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/check"
	"github.com/ZachCurry13/isoshelf/internal/remote/remotetest"
	"github.com/ZachCurry13/isoshelf/internal/sampledrive"
	"github.com/ZachCurry13/isoshelf/internal/state"
)

type result struct {
	code           int
	stdout, stderr string
}

// runWith runs the command with test folders, recorded responses and a fixed
// clock.
func runWith(t *testing.T, dirs appdir.Dirs, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), args, &env{
		stdout: &stdout,
		stderr: &stderr,
		dirs:   &dirs,
		http:   &http.Client{Transport: remotetest.Recorded()},
		now:    func() time.Time { return time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC) },
		getenv: func(string) string { return "" },
	})
	return result{code, stdout.String(), stderr.String()}
}

func installed(t *testing.T) appdir.Dirs {
	t.Helper()
	return appdir.Dirs{App: t.TempDir(), Config: t.TempDir(), Temp: t.TempDir()}
}

// sampleDrive creates a folder holding the sample drive's filenames.
func sampleDrive(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range sampledrive.Files {
		if err := os.WriteFile(filepath.Join(dir, f.Name), []byte("stand-in"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func decode(t *testing.T, out string) jsonOutput {
	t.Helper()
	var r jsonOutput
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out)
	}
	return r
}

func item(r jsonOutput, path string) *check.ItemJSON {
	for i := range r.Items {
		if r.Items[i].Path == path {
			return &r.Items[i]
		}
	}
	return nil
}

func TestScanJSON(t *testing.T) {
	dirs := installed(t)
	drive := sampleDrive(t)

	res := runWith(t, dirs, "scan", "--json", drive)
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	r := decode(t, res.stdout)
	if r.Checked || r.Profile != "ventoy" || len(r.Items) != len(sampledrive.Files)-1 {
		t.Errorf("checked %v, profile %q, %d items", r.Checked, r.Profile, len(r.Items))
	}
	for path, want := range map[string]string{
		"linuxmint-22.3-cinnamon-64bit.iso":   "not checked",
		"HBCD_PE_x64.iso":                     "manual",
		"Windows.iso":                         "unrecognized",
		"FydeOS_for_PC_iris_v22.0-SP1-io.bin": "not bootable",
	} {
		if it := item(r, path); it == nil || it.Status != want {
			t.Errorf("%s: got %+v, want %s", path, it, want)
		}
	}

	// The scan saved state in the target and a mirror in the config folder.
	st, err := state.Load(drive)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.History) != 1 || st.Files["netboot.xyz.iso"].SHA256 == "" {
		t.Errorf("state: %d scans, netboot.xyz hash %q", len(st.History), st.Files["netboot.xyz.iso"].SHA256)
	}
	if mirrors, err := state.LoadMirrors(dirs.Config); err != nil || len(mirrors) != 1 {
		t.Errorf("mirrors: %v, %v", mirrors, err)
	}
}

func TestCheckTable(t *testing.T) {
	dirs := installed(t)
	drive := sampleDrive(t)

	res := runWith(t, dirs, "check", drive, "--no-hash") // flags may follow the folder
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	for _, want := range []string{
		"STATUS", "LATEST",
		"update available (EOL)", // MX Linux 21.3
		"pop-os_22.04_amd64_intel_56.iso",
		"not hashed yet", // --no-hash leaves fixed-name images undecided
		"25 image(s): ",
	} {
		if !strings.Contains(res.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, res.stdout)
		}
	}
}

func TestCheckJSON(t *testing.T) {
	res := runWith(t, installed(t), "check", "--json", sampleDrive(t))
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	r := decode(t, res.stdout)
	if !r.Checked {
		t.Error("report not marked as checked")
	}
	for path, want := range map[string]struct{ status, latest string }{
		"pop-os_22.04_amd64_intel_56.iso":   {"update available", "58"},
		"linuxmint-22.3-cinnamon-64bit.iso": {"up to date", "22.3"},
		"CentOS-7-i386-Minimal-2009.iso":    {"EOL", "2009"},
		"netboot.xyz.iso":                   {"update available", "3.0.3"},
	} {
		if it := item(r, path); it == nil || it.Status != want.status || it.Latest != want.latest {
			t.Errorf("%s: got %+v, want %s, latest %s", path, it, want.status, want.latest)
		}
	}
	if res.stderr != "" {
		t.Errorf("JSON mode wrote to stderr: %q", res.stderr)
	}
}

func TestPortableDefaultsToDrive(t *testing.T) {
	drive := sampleDrive(t)
	app := filepath.Join(drive, "isoshelf")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	// An image inside the app folder must not be listed.
	if err := os.WriteFile(filepath.Join(app, "netboot.xyz-multiarch.iso"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirs := appdir.Dirs{Portable: true, App: app, Config: app, Temp: filepath.Join(app, "tmp"), DefaultTarget: drive}

	res := runWith(t, dirs, "scan", "--json", "--no-hash")
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	r := decode(t, res.stdout)
	if r.Target != drive {
		t.Errorf("target = %q, want the drive %q", r.Target, drive)
	}
	if item(r, "isoshelf/netboot.xyz-multiarch.iso") != nil {
		t.Error("the app folder was scanned")
	}
	if _, err := os.Stat(filepath.Join(app, "targets")); !os.IsNotExist(err) {
		t.Error("portable mode wrote a mirror")
	}
}

func TestProfileIsRemembered(t *testing.T) {
	dirs := installed(t)
	drive := sampleDrive(t)
	if res := runWith(t, dirs, "scan", "--json", "--no-hash", "--profile", "proxmox", drive); res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	res := runWith(t, dirs, "scan", "--json", "--no-hash", drive)
	if r := decode(t, res.stdout); r.Profile != "proxmox" {
		t.Errorf("profile = %q, want proxmox remembered", r.Profile)
	}
}

func TestUserCatalog(t *testing.T) {
	dirs := installed(t)
	drive := sampleDrive(t)
	userCatalog := `schema = 1
[[entry]]
id = "windows-media-tool"
name = "Windows (Media Creation Tool)"
arch = "x86_64"
match = 'Windows\.iso'
samples = ["Windows.iso"]
page = "https://www.microsoft.com/software-download/"
[entry.source]
type = "manual"
`
	if err := os.WriteFile(filepath.Join(dirs.Config, "catalog.toml"), []byte(userCatalog), 0o644); err != nil {
		t.Fatal(err)
	}
	res := runWith(t, dirs, "scan", "--json", "--no-hash", drive)
	if res.code != 0 {
		t.Fatalf("exit %d: %s", res.code, res.stderr)
	}
	r := decode(t, res.stdout)
	if it := item(r, "Windows.iso"); it == nil || it.Entry != "windows-media-tool" {
		t.Errorf("Windows.iso = %+v, want the user catalog's entry", it)
	}
}

func TestUsageErrors(t *testing.T) {
	dirs := installed(t)
	tests := []struct {
		args     []string
		wantCode int
		wantErr  string
	}{
		{[]string{"frobnicate"}, 2, `unknown command "frobnicate"`},
		{[]string{"scan"}, 1, "which folder?"},
		{[]string{"scan", "a", "b"}, 2, "give one folder"},
		{[]string{"scan", "--profile", "usb", "a"}, 2, `unknown profile "usb"`},
		{[]string{"scan", "--bogus", "a"}, 2, "not defined"},
		{[]string{"scan", filepath.Join(t.TempDir(), "missing")}, 1, ""},
	}
	for _, tt := range tests {
		res := runWith(t, dirs, tt.args...)
		if res.code != tt.wantCode || !strings.Contains(res.stderr, tt.wantErr) {
			t.Errorf("%q: exit %d, stderr %q; want exit %d containing %q", tt.args, res.code, res.stderr, tt.wantCode, tt.wantErr)
		}
	}

	if res := runWith(t, dirs, "version"); res.code != 0 || res.stdout != "isoshelf dev\n" {
		t.Errorf("version: exit %d, %q", res.code, res.stdout)
	}
}

func TestUI(t *testing.T) {
	dirs := installed(t)
	drive := sampleDrive(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urls := make(chan string, 1)
	opened := make(chan string, 1)
	done := make(chan int, 1)
	var stdout, stderr bytes.Buffer
	go func() {
		done <- run(ctx, []string{"ui", "--port", "0", drive}, &env{
			stdout:      &stdout,
			stderr:      &stderr,
			dirs:        &dirs,
			http:        &http.Client{Transport: remotetest.Recorded()},
			getenv:      func(string) string { return "" },
			openBrowser: func(url string) error { opened <- url; return nil },
			listening:   func(url string) { urls <- url },
		})
	}()

	var url string
	select {
	case url = <-urls:
	case code := <-done:
		t.Fatalf("ui exited with %d: %s", code, stderr.String())
	case <-time.After(10 * time.Second):
		t.Fatal("ui didn't start")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") || !strings.Contains(url, "/?token=") {
		t.Errorf("url = %q", url)
	}

	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noRedirect.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("token link: %s", resp.Status)
	}
	if got := <-opened; got != url {
		t.Errorf("opened %q, want %q", got, url)
	}

	cancel()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("exit %d: %s", code, stderr.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ui didn't stop")
	}
}
