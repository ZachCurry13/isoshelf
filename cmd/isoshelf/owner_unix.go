//go:build !windows

package main

import (
	"os"
	"syscall"
)

// fileOwner is the user and group a folder belongs to.
type fileOwner struct{ uid, gid int }

// ownerOf reads who owns dir. A filesystem that doesn't say is not worth a
// complaint of its own; the caller simply leaves that line out.
func ownerOf(dir string) (fileOwner, bool) {
	info, err := os.Stat(dir)
	if err != nil {
		return fileOwner{}, false
	}
	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileOwner{}, false
	}
	return fileOwner{uid: int(sys.Uid), gid: int(sys.Gid)}, true
}
