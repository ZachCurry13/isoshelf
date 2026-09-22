package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// isoshelf password sets the username and password from a shell on the
// machine isoshelf runs on.
//
// Since a login replaces the link's secret rather than sitting beside it,
// this is one of the two ways back for somebody who has forgotten theirs -
// the other being ISOSHELF_USERNAME and ISOSHELF_PASSWORD at the next start.
// Both need the machine itself, which is the right bar: getting back in
// should take more than reaching the address.
//
// In a container:
//
//	docker exec -it isoshelf isoshelf password
//
// What it does not do is hide the password as it is typed. Turning off a
// terminal's echo means either a second dependency or a page of
// platform-specific code for the two systems isoshelf runs on, to guard
// against somebody standing behind you at your own NAS. It says the password
// will show, and points at the environment variables for anyone who would
// rather it didn't.

func setPassword(e *env, stdin io.Reader, args []string) error {
	dirs, err := findDirs(e)
	if err != nil {
		return err
	}
	have, err := auth.Load(dirs.Config)
	if err != nil {
		return err
	}

	user := ""
	if len(args) > 0 {
		user = strings.TrimSpace(args[0])
	}
	if user == "" && have != nil {
		user = have.User
	}

	fmt.Fprintln(e.stderr, "isoshelf: what you type here will show on the screen. To set a password")
	fmt.Fprintln(e.stderr, "isoshelf: without it showing, set ISOSHELF_USERNAME and ISOSHELF_PASSWORD where")
	fmt.Fprintln(e.stderr, "isoshelf: isoshelf starts, and restart it.")

	lines := bufio.NewScanner(stdin)
	ask := func(prompt string) (string, error) {
		fmt.Fprint(e.stderr, prompt)
		if !lines.Scan() {
			if err := lines.Err(); err != nil {
				return "", err
			}
			return "", errors.New("nothing to read: run this from a terminal, or pipe the answers in")
		}
		return strings.TrimSpace(lines.Text()), nil
	}

	if user == "" {
		if user, err = ask("Username: "); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(e.stderr, "Username: %s\n", user)
	}
	password, err := ask("Password: ")
	if err != nil {
		return err
	}
	again, err := ask("Password again: ")
	if err != nil {
		return err
	}
	if password != again {
		return errors.New("those two passwords aren't the same")
	}
	if err := auth.Set(dirs.Config, user, password); err != nil {
		return err
	}
	fmt.Fprintf(e.stdout, "The login is now %q.\n", user)
	fmt.Fprintln(e.stdout, "Anyone signed in elsewhere stays signed in; \"Sign out everywhere\" in")
	fmt.Fprintln(e.stdout, "Settings ends those too.")
	return nil
}
