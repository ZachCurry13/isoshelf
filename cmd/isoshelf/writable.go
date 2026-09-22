package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ZachCurry13/isoshelf/internal/state"
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
	if folder == "" {
		return
	}
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return // already complained about separately
	}
	if err := canWrite(folder); err != nil {
		fmt.Fprintf(w, "isoshelf: can't write to %s, the folder of images: %v\n", folder, err)
		fmt.Fprintln(w, "isoshelf:   isoshelf will still list what's in it and check it for updates. It won't be")
		fmt.Fprintln(w, "isoshelf:   able to download anything into it, archive anything, or remember what it found.")
		sayHowToFix(w, folder)
		return // the folder itself is the problem; what's inside it can wait
	}
	// The folder can be written to, which is not the same as isoshelf's own
	// folder inside it being writable. That one is often older than the
	// current arrangement - made on an earlier run, by whichever user
	// isoshelf was then - and it is where every download is staged, so a
	// folder that looks fine can still fail at the first download with
	// nothing having warned about it. Which is exactly what happened.
	inside := filepath.Join(folder, state.DirName)
	if info, err := os.Stat(inside); err == nil && info.IsDir() {
		if err := canWrite(inside); err != nil {
			fmt.Fprintf(w, "isoshelf: can't write to %s: %v\n", inside, err)
			fmt.Fprintln(w, "isoshelf:   That folder is isoshelf's own, inside your images folder: downloads are")
			fmt.Fprintln(w, "isoshelf:   staged there, the archive lives there, and what isoshelf has learned about")
			fmt.Fprintln(w, "isoshelf:   the folder is kept there. Every download will fail with \"permission denied\"")
			fmt.Fprintln(w, "isoshelf:   until this is fixed, even though the images folder around it is fine.")
			sayWhoMadeIt(w, inside)
			sayHowToFixTheInnerFolder(w, inside)
		}
	}
}

// sayWhoMadeIt names the user that owns the folder, because the whole story
// is in the difference between that number and the one isoshelf runs as - and
// because it says how it happened: the folder was made on an earlier run, by
// whoever isoshelf was then, and the app's user has changed since.
func sayWhoMadeIt(w io.Writer, dir string) {
	owner, ok := ownerOf(dir)
	if !ok {
		return
	}
	fmt.Fprintf(w, "isoshelf:   It belongs to user %d, group %d - isoshelf made it on an earlier run, as\n", owner.uid, owner.gid)
	fmt.Fprintln(w, "isoshelf:   whichever user it was then. Changing the app's user afterwards leaves this")
	fmt.Fprintln(w, "isoshelf:   folder behind, owned by the old one.")
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

// sayHowToFixTheInnerFolder is different advice from sayHowToFix, and the
// difference matters. There the mount itself is wrong, so changing the app's
// user is one of the two answers. Here the mount is fine and one folder
// inside it is stale, so changing the app's user would only break the folder
// that currently works. The answer is to hand this one folder over.
func sayHowToFixTheInnerFolder(w io.Writer, dir string) {
	fmt.Fprintf(w, "isoshelf:   To fix it, give that one folder to the user isoshelf runs as%s. Do NOT\n", runningAs())
	fmt.Fprintln(w, "isoshelf:   change the app's user to match it instead: the images folder around it is")
	fmt.Fprintln(w, "isoshelf:   already right, and changing the user would break that one too.")
	fmt.Fprintf(w, "isoshelf:     On TrueNAS: open a shell on the host and run  chown -R %s %s\n", chownArgs(), hostPathHint(dir))
	fmt.Fprintln(w, "isoshelf:     - using the path on the host, which is the dataset you mounted, not the")
	fmt.Fprintln(w, "isoshelf:     path above. If the folder holds nothing you want, deleting it works too:")
	fmt.Fprintln(w, "isoshelf:     isoshelf makes a new one, owned by the right user, at the next scan.")
	fmt.Fprintln(w, "isoshelf:     What is in it: the archive of removed images, part-finished downloads,")
	fmt.Fprintln(w, "isoshelf:     and what isoshelf has worked out about this folder.")
}

// chownArgs is "568:568", the pair to hand a folder to.
func chownArgs() string {
	if runtime.GOOS == "windows" {
		return "<user>:<group>"
	}
	return fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid())
}

// hostPathHint keeps the folder's own name on the end, since that part is the
// same on both sides of a mount and makes the command easier to finish.
func hostPathHint(dir string) string {
	return "<the dataset you mounted>/" + filepath.Base(dir)
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
