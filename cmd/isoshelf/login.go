package main

import (
	"fmt"
	"io"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/auth"
)

// The username and password, from outside the page.
//
// On a NAS the install form is where people expect to put a password, so two
// environment variables set one before isoshelf has ever been opened. That
// closes the window where whoever arrives first chooses it - and when they
// are not set, the log says plainly that the window is open, rather than
// leaving somebody to find out later.

const (
	userEnv     = "ISOSHELF_USERNAME"
	passwordEnv = "ISOSHELF_PASSWORD"
)

// setUpLogin applies a username and password given in the environment, and
// says what the login situation is either way. server is whether isoshelf is
// answering to the network rather than only to this machine.
func setUpLogin(e *env, dirs appdir.Dirs, server bool, address string) {
	user, password := e.getenv(userEnv), e.getenv(passwordEnv)
	have, _ := auth.Load(dirs.Config)

	if user != "" || password != "" {
		if err := auth.CheckNew(user, password); err != nil {
			fmt.Fprintf(e.stderr, "isoshelf: %s and %s were set but can't be used: %v\n", userEnv, passwordEnv, err)
		} else if have != nil && have.Matches(user, password) {
			// Already what is on disk. Writing it again would only change
			// the salt, and a container restarts with these set every time.
			return
		} else if err := auth.Set(dirs.Config, user, password); err != nil {
			fmt.Fprintf(e.stderr, "isoshelf: couldn't save the login from %s: %v\n", userEnv, err)
		} else {
			fmt.Fprintf(e.stderr, "isoshelf: the login is %q, from %s and %s.\n", user, userEnv, passwordEnv)
			return
		}
		have, _ = auth.Load(dirs.Config)
	}

	if !server {
		return // a desktop opens itself with the link; there is nothing to say
	}
	if have != nil {
		fmt.Fprintf(e.stderr, "isoshelf: sign in as %q, or use the link below.\n", have.User)
		return
	}
	sayNobodyHasSetOneYet(e.stderr, address)
}

// sayNobodyHasSetOneYet is the one thing about this that has to be said out
// loud: until somebody sets a password, whoever opens the address first is
// the one who chooses it. That is how a NAS app's first run normally works,
// and it is still worth saying rather than leaving to be discovered.
func sayNobodyHasSetOneYet(w io.Writer, address string) {
	fmt.Fprintln(w, "isoshelf: nobody has set a username and password yet.")
	if address != "" {
		fmt.Fprintf(w, "isoshelf:   Open %s and the first thing it asks is to choose one.\n", address)
	}
	fmt.Fprintf(w, "isoshelf:   Until then, whoever reaches this address first is the one who sets it, so\n")
	fmt.Fprintf(w, "isoshelf:   do it now rather than later. Setting %s and %s before\n", userEnv, passwordEnv)
	fmt.Fprintln(w, "isoshelf:   isoshelf starts skips this entirely.")
}

// haveLogin says whether a username and password are set, for the startup
// message: a link that no longer works is worse than no link at all.
func haveLogin(dirs appdir.Dirs) bool {
	a, err := auth.Load(dirs.Config)
	return err == nil && a != nil
}
