package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/ZachCurry13/isoshelf/internal/appdir"
	"github.com/ZachCurry13/isoshelf/internal/appupdate"
	"github.com/ZachCurry13/isoshelf/internal/web"
)

// serveUI runs the web UI on localhost until ctx is done, or until it hands
// over to a new isoshelf that has just been put in its place.
func serveUI(ctx context.Context, e *env, opts options) (err error) {
	dirs, err := findDirs(e)
	if err != nil {
		return err
	}
	cat, catSource, err := loadCatalog(dirs, opts.catalog, e.stderr)
	if err != nil {
		return err
	}
	cat = withOwnImages(cat, dirs, e.stderr)

	// A program that isoshelf has just updated starts with a handover: the
	// port the page in the browser is on, and the update to confirm, or to
	// undo if this program can't come up. Either way the browser stays where
	// it is, so no new tab.
	hand, handed := appupdate.ReadHandover(e.getenv)
	trial := handed && hand.Stage != ""
	exe, _ := os.Executable()
	if !trial {
		// What an earlier update left beside the program, if anything: a
		// staging folder, or an old program that was still running when the
		// new one tried to remove it.
		appupdate.Tidy(exe, dirs.Portable)
	}
	if handed {
		opts.port, opts.noBrowser = hand.Port, true
	}
	var confirmed atomic.Bool
	if trial {
		defer func() {
			if r := recover(); r != nil && !confirmed.Load() {
				rollBack(e, hand, fmt.Errorf("it stopped: %v", r))
				panic(r)
			}
		}()
	}

	// Loopback unless told otherwise. Listening anywhere else is server mode:
	// a container, or a machine someone opens the page on from their desk.
	host := opts.listen
	if host == "" {
		host = "127.0.0.1"
	}
	listener, err := listen(host, opts.port, handed)
	if err != nil {
		err = fmt.Errorf("can't listen on %s port %d (is isoshelf already running?): %w", host, opts.port, err)
		if trial {
			return rollBack(e, hand, err)
		}
		return err
	}
	anyHost := !isLoopback(host)
	port := listener.Addr().(*net.TCPAddr).Port

	token, err := linkToken(e, dirs, anyHost)
	if err != nil {
		return err
	}
	warnAboutFolders(e, opts.folder, anyHost, dirs.Config)
	url := fmt.Sprintf("http://%s/?token=%s", listener.Addr(), token)
	// The username and password, if there is one or the environment gives
	// one. Before the server starts, so the log reads in the order things
	// happen and nobody can reach the setup form ahead of this being said.
	plainAddress := ""
	if anyHost {
		plainAddress = fmt.Sprintf("http://<this-machine>:%d/", port)
	}
	setUpLogin(e, dirs, anyHost, plainAddress)

	srv := &uiServer{addr: listener.Addr().String(), errs: make(chan error, 1), done: make(chan struct{})}

	ui := web.New(web.Config{
		Dirs:          dirs,
		Catalog:       cat,
		CatalogSource: catSource,
		HTTP:          e.http,
		GitHubToken:   e.getenv("GITHUB_TOKEN"),
		Version:       version,
		Token:         token,
		AnyHost:       anyHost,
		Target:        opts.folder,
		Now:           e.now,
		SelfUpdate: web.SelfUpdateConfig{
			Exe:       exe,
			Container: e.getenv("ISOSHELF_CONTAINER") != "",
			Restart: func(program, stage string) error {
				return srv.handOver(program, appupdate.Handover{Port: port, Stage: stage, From: version}, token)
			},
			From:   hand.From,
			Failed: hand.Failed,
		},
	})
	srv.handler = ui
	// Whatever isoshelf does while nobody is asking - updating the images on
	// a schedule, when that is turned on - runs alongside the server and
	// stops with it.
	go ui.Run(ctx)
	srv.start(listener)
	if trial {
		go keepUpdate(e, hand, &confirmed)
	}

	printAddress(e, anyHost, listener.Addr(), url, token, dirs)
	if handed && hand.From != "" {
		fmt.Fprintf(e.stdout, "Updated from %s.\n", hand.From)
	}
	if e.listening != nil {
		e.listening(url)
	}
	if !opts.noBrowser && !anyHost {
		if err := e.openBrowser(url); err != nil {
			fmt.Fprintln(e.stderr, "isoshelf: couldn't open the browser; open the link above yourself:", err)
		}
	}

	select {
	case err := <-srv.errs:
		return err
	case <-srv.done:
		return nil // the new isoshelf has this window now
	case <-ctx.Done():
	}
	return srv.shutdown()
}

// warnAboutFolders says out loud what is wrong with the folders isoshelf is
// about to use, before it shows up later as something stranger.
func warnAboutFolders(e *env, folder string, anyHost bool, config string) {
	// A folder named on the command line that isn't there is worth saying out
	// loud. In a container it is the commonest first mistake - the mount was
	// spelled differently, or left out - and without this isoshelf comes up
	// with an empty folder chooser and no hint about why.
	if folder != "" {
		if info, err := os.Stat(folder); err != nil || !info.IsDir() {
			fmt.Fprintf(e.stderr, "isoshelf: can't open the folder %s: %v\n", folder, folderTrouble(info, err))
			if anyHost {
				fmt.Fprintln(e.stderr, "isoshelf: in a container this usually means nothing is mounted there. Check the mount, or choose a folder on the page.")
			}
		}
	}
	// A folder isoshelf can read but not write is the other half of the same
	// mistake, and on its own it only shows up later as "permission denied"
	// from whichever part of isoshelf happened to write first.
	checkWritable(e.stderr, config, folder)
}

// printAddress says where the page is.
func printAddress(e *env, anyHost bool, addr net.Addr, url, token string, dirs appdir.Dirs) {
	port := addr.(*net.TCPAddr).Port
	if !anyHost {
		fmt.Fprintf(e.stdout, "isoshelf is running at:\n\n  %s\n\nKeep this window open while you use it. Press Ctrl+C to stop.\n", url)
		return
	}
	// The address it bound to is rarely the address anyone types, so say
	// what to do rather than printing 0.0.0.0 and hoping.
	fmt.Fprintf(e.stdout, "isoshelf is listening on %s.\n\nOpen it from this machine's own address:\n\n  http://<this-machine>:%d/\n\n", addr, port)
	if haveLogin(dirs) {
		fmt.Fprintf(e.stdout, "Sign in with the username and password you set. Forgotten them? Set\nISOSHELF_USERNAME and ISOSHELF_PASSWORD and restart, or run\n\"isoshelf password\" on this machine.\n")
	} else {
		fmt.Fprintf(e.stdout, "The first thing it asks is to choose a username and password. Until\nsomebody does, this link gets in without one:\n\n  http://<this-machine>:%d/?token=%s\n\nIt stops working the moment a password is set.\n", port, token)
	}
}

// folderTrouble says what is wrong with a folder in words, since "no such
// file or directory" and "not a directory" are different mistakes.
func folderTrouble(info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info != nil && !info.IsDir() {
		return errors.New("it is a file, not a folder")
	}
	return nil
}
