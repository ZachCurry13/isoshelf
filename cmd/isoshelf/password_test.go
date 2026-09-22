package main

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/auth"
)

func passwordSetup(t *testing.T) (*env, appdir.Dirs, *bytes.Buffer) {
	t.Helper()
	dirs := appdir.Dirs{Config: filepath.Join(t.TempDir(), "isoshelf")}
	var out bytes.Buffer
	return &env{
		stdout: &out, stderr: io.Discard,
		dirs:   &dirs,
		getenv: func(string) string { return "" },
	}, dirs, &out
}

// The way back in when the password is forgotten, now that the link isn't
// one. It has to work from a shell with nothing else set up.
func TestPasswordCommandSetsALogin(t *testing.T) {
	e, dirs, out := passwordSetup(t)
	in := strings.NewReader("zach\na good long password\na good long password\n")
	if err := setPassword(e, in, nil); err != nil {
		t.Fatal(err)
	}
	a, err := auth.Load(dirs.Config)
	if err != nil || a == nil {
		t.Fatalf("no login was written: %+v, %v", a, err)
	}
	if !a.Matches("zach", "a good long password") {
		t.Error("the login it wrote doesn't match what was typed")
	}
	if !strings.Contains(out.String(), `"zach"`) {
		t.Errorf("it didn't say what the login is now: %s", out)
	}
}

// Given a username, it doesn't ask for one - so changing only the password
// is two lines, not three.
func TestPasswordCommandTakesTheUsernameAsAnArgument(t *testing.T) {
	e, dirs, _ := passwordSetup(t)
	in := strings.NewReader("a good long password\na good long password\n")
	if err := setPassword(e, in, []string{"zach"}); err != nil {
		t.Fatal(err)
	}
	if a, _ := auth.Load(dirs.Config); a == nil || a.User != "zach" {
		t.Errorf("username = %+v, want zach", a)
	}
}

// Changing the password of an existing login keeps the username, so somebody
// locked out doesn't have to remember which one they chose.
func TestPasswordCommandKeepsTheUsernameItAlreadyHas(t *testing.T) {
	e, dirs, _ := passwordSetup(t)
	if err := auth.Set(dirs.Config, "zach", "the one that was forgotten"); err != nil {
		t.Fatal(err)
	}
	in := strings.NewReader("a brand new password\na brand new password\n")
	if err := setPassword(e, in, nil); err != nil {
		t.Fatal(err)
	}
	a, _ := auth.Load(dirs.Config)
	if a == nil || a.User != "zach" {
		t.Fatalf("username = %+v, want it kept", a)
	}
	if !a.Matches("zach", "a brand new password") || a.Matches("zach", "the one that was forgotten") {
		t.Error("the password wasn't actually changed")
	}
}

// Two different passwords are refused rather than one of them being picked.
func TestPasswordCommandRefusesATypo(t *testing.T) {
	e, dirs, _ := passwordSetup(t)
	in := strings.NewReader("zach\na good long password\na different password\n")
	err := setPassword(e, in, nil)
	if err == nil || !strings.Contains(err.Error(), "aren't the same") {
		t.Errorf("a typo gave %v, want a complaint about the two not matching", err)
	}
	if a, _ := auth.Load(dirs.Config); a != nil {
		t.Error("a login was written despite the typo")
	}
}

// A password too short to be worth having is refused here as well as on the
// page: this command must not be the way round the rule.
func TestPasswordCommandRefusesAShortOne(t *testing.T) {
	e, dirs, _ := passwordSetup(t)
	in := strings.NewReader("zach\nshort\nshort\n")
	if err := setPassword(e, in, nil); err == nil {
		t.Error("a short password was accepted")
	}
	if a, _ := auth.Load(dirs.Config); a != nil {
		t.Error("a short password was written anyway")
	}
}

// Nothing to read is a plain complaint, not a panic or an empty password.
func TestPasswordCommandWithNothingToRead(t *testing.T) {
	e, _, _ := passwordSetup(t)
	if err := setPassword(e, strings.NewReader(""), nil); err == nil {
		t.Error("an empty input was accepted")
	}
}
