package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSetAndMatch(t *testing.T) {
	dir := t.TempDir()
	if a, err := Load(dir); a != nil || err != nil {
		t.Fatalf("a fresh folder already had a login: %+v, %v", a, err)
	}
	if err := Set(dir, " zach ", "a good long password"); err != nil {
		t.Fatal(err)
	}
	a, err := Load(dir)
	if err != nil || a == nil {
		t.Fatalf("Load = %+v, %v", a, err)
	}
	if a.User != "zach" {
		t.Errorf("username is %q; the spaces around it should have gone", a.User)
	}
	if !a.Matches("zach", "a good long password") {
		t.Error("the right username and password were refused")
	}
	// Spaces typed around the username on the way in are forgiven too, the
	// same as they were when it was set.
	if !a.Matches("  zach  ", "a good long password") {
		t.Error("a username with spaces around it was refused")
	}
	for _, c := range []struct{ user, password string }{
		{"zach", "a good long passwore"},
		{"zach", ""},
		{"someone", "a good long password"},
		{"", ""},
	} {
		if a.Matches(c.user, c.password) {
			t.Errorf("%q / %q was let in", c.user, c.password)
		}
	}
}

// The password itself must never be in the file, and the file must not be
// readable by everyone on the machine.
func TestTheFileHoldsNoPassword(t *testing.T) {
	dir := t.TempDir()
	const password = "correct horse battery staple"
	if err := Set(dir, "zach", password); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, FileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), password) {
		t.Errorf("the password is in the file:\n%s", body)
	}
	var a Account
	if err := json.Unmarshal(body, &a); err != nil {
		t.Fatalf("the file isn't readable JSON: %v", err)
	}
	if a.Iterations < 100_000 {
		t.Errorf("iterations = %d, which is too few to slow anybody down", a.Iterations)
	}
	// Windows reports 0666 for every regular file, whatever it was written
	// with; the mode still matters everywhere isoshelf runs as a server.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("the login file is %v; it should be 0600", perm)
		}
	}
}

// Two accounts with the same password must not have the same hash, or one
// stolen file would say which of them share a password.
func TestEachAccountGetsItsOwnSalt(t *testing.T) {
	one, two := t.TempDir(), t.TempDir()
	if err := Set(one, "zach", "the same password"); err != nil {
		t.Fatal(err)
	}
	if err := Set(two, "zach", "the same password"); err != nil {
		t.Fatal(err)
	}
	a, _ := Load(one)
	b, _ := Load(two)
	if a.Hash == b.Hash || a.Salt == b.Salt {
		t.Error("the same password gave the same hash")
	}
}

func TestCheckNew(t *testing.T) {
	cases := []struct {
		why, user, password, want string
	}{
		{"no username", "", "a good long password", "Choose a username"},
		{"a username of spaces", "   ", "a good long password", "Choose a username"},
		{"a short password", "zach", "short", "8 characters"},
		{"no password", "zach", "", "8 characters"},
		{"a username with a line break", "za\nch", "a good long password", "line breaks"},
	}
	for _, c := range cases {
		err := CheckNew(c.user, c.password)
		if err == nil {
			t.Errorf("%s was accepted", c.why)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want it to mention %q", c.why, err, c.want)
		}
	}
	if err := CheckNew("zach", "a good long password"); err != nil {
		t.Errorf("a fine username and password were refused: %v", err)
	}
}

func TestSessions(t *testing.T) {
	dir := t.TempDir()
	key, saved, err := LoadKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !saved {
		t.Fatal("a writable folder should have kept the key")
	}
	now := time.Now()
	cookie := key.Mint("zach", now)
	if !key.Valid(cookie, now.Add(time.Hour)) {
		t.Error("a session made a moment ago was refused")
	}
	if key.Valid(cookie, now.Add(SessionLife+time.Minute)) {
		t.Error("an expired session was accepted")
	}
	for _, bad := range []string{"", "nonsense", cookie + "x", strings.TrimSuffix(cookie, "A") + "B"} {
		if key.Valid(bad, now) {
			t.Errorf("%q was accepted as a session", bad)
		}
	}

	// A restart keeps everyone logged in: the key is on disk, so the same
	// cookie still checks out.
	again, _, err := LoadKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Valid(cookie, now) {
		t.Error("a restart threw everybody out")
	}

	// And Forget is what throws everybody out on purpose.
	if err := Forget(dir); err != nil {
		t.Fatal(err)
	}
	fresh, _, err := LoadKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Valid(cookie, now) {
		t.Error("signing out everywhere left the old sessions working")
	}
}

// A folder isoshelf can't write is not fatal - a key in memory still signs
// sessions - but it has to say so, because it means everyone is thrown out at
// every restart, and on a NAS that is every update and every reboot. That
// used to be silent, which is how "why do I have to log in every time?"
// became a question with no answer on the page.
func TestAKeyThatCouldNotBeSavedSaysSo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits don't keep a folder from being written on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root writes to a folder whatever its mode says")
	}
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o500); err != nil { // read and enter, not write
		t.Fatal(err)
	}

	key, saved, err := LoadKey(dir)
	if err != nil {
		t.Fatalf("a folder it can't write should not stop isoshelf: %v", err)
	}
	if saved {
		t.Error("LoadKey says it saved the key, into a folder it cannot write")
	}
	// The key still works; it just won't outlive the process.
	now := time.Now()
	if cookie := key.Mint("zach", now); !key.Valid(cookie, now) {
		t.Error("the key held in memory doesn't sign a usable session")
	}
	if _, err := os.Stat(filepath.Join(dir, KeyFileName)); err == nil {
		t.Error("a key file appeared in a folder that should not be writable")
	}
}
