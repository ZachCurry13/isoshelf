package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// A folder isoshelf can read but not write is the commonest thing to go wrong
// in a container, and on its own it produces a wall of "permission denied"
// from whichever part of isoshelf tried first - a message that says what
// failed and nothing about what to do. This file checks both folders once, at
// startup, and says the one thing that fixes it: the folder has to belong to
// the user isoshelf is running as.
//
// It never stops isoshelf. A folder that can only be read still shows what is
// in it and still checks it for updates, which is worth having; and saying
// "this won't work" for a folder that turns out to be fine would be worse
// than the wall of messages.

// checkWritable warns about each folder isoshelf needs to write to and can't.
// config is isoshelf's own folder; folder is the images folder, or "".
func checkWritable(w io.Writer, config, folder string) {
	if config != "" {
		if err := canWrite(config); err != nil {
			fmt.Fprintf(w, "isoshelf: can't write to %s, which is where isoshelf keeps its own files: %v\n", config, err)
			fmt.Fprintln(w, "isoshelf:   Until that is fixed: the link's secret changes every restart, settings aren't")
			fmt.Fprintln(w, "isoshelf:   remembered, and the copy of each folder's history isn't kept.")
			sayHowToFix(w, config)
		}
	}
	if folder != "" {
		if info, err := os.Stat(folder); err != nil || !info.IsDir() {
			return // already complained about separately
		}
		if err := canWrite(folder); err != nil {
			fmt.Fprintf(w, "isoshelf: can't write to %s, the folder of images: %v\n", folder, err)
			fmt.Fprintln(w, "isoshelf:   isoshelf will still list what's in it and check it for updates. It won't be")
			fmt.Fprintln(w, "isoshelf:   able to download anything into it, archive anything, or remember what it found.")
			sayHowToFix(w, folder)
		}
	}
}

// sayHowToFix names the user isoshelf is actually running as, because that is
// the number that has to go in the other half of the answer and nobody can
// guess it from outside. It also names the folder to change, which is not
// always the one that failed: isoshelf's own folder sits inside the one
// somebody mounted, and it is the mount whose permissions decide.
func sayHowToFix(w io.Writer, dir string) {
	fix := nearest(dir)
	fmt.Fprintf(w, "isoshelf:   To fix it, %s has to belong to the user isoshelf runs as%s.\n", fix, runningAs())
	fmt.Fprintln(w, "isoshelf:     On TrueNAS: either set the app's User and Group ID to whoever owns that")
	fmt.Fprintln(w, "isoshelf:     dataset (Datasets, the dataset, Permissions says who that is), or change")
	fmt.Fprintln(w, "isoshelf:     the dataset's owner to the user above. Either works; one of them has to.")
	fmt.Fprintln(w, "isoshelf:     Change it on the host machine, not in here: that folder is a mount, and")
	fmt.Fprintln(w, "isoshelf:     its permissions come from the other side of it.")
}

// nearest is the deepest folder on this path that exists, which is the one
// whose permissions are actually in the way. Naming /config/isoshelf when
// /config is the mount sends somebody looking for a folder that isn't there.
func nearest(dir string) string {
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// runningAs is " (user 568, group 568)" on Unix, and nothing on Windows,
// where those numbers don't exist and the message would be nonsense.
func runningAs() string {
	if runtime.GOOS == "windows" {
		return ""
	}
	return fmt.Sprintf(" (user %d, group %d)", os.Getuid(), os.Getgid())
}

// canWrite reports whether a file can actually be made in dir, by making one
// and taking it away again. Asking the permission bits instead gets the wrong
// answer often enough to be useless: a mount can be read-only whatever the
// bits say, and group membership is not something to work out by hand.
func canWrite(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		if errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrExist) {
			// The folder itself can't be made. Whether it exists and is
			// unwritable, or its parent is, the answer is the same.
			return err
		}
		return err
	}
	f, err := os.CreateTemp(dir, ".isoshelf-can-write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return nil
}
