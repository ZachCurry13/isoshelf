//go:build windows

package main

// fileOwner is the user and group a folder belongs to.
type fileOwner struct{ uid, gid int }

// ownerOf says nothing on Windows: there are no uid and gid numbers there, so
// the line about who owns a folder would be meaningless even if one could be
// invented. The permissions advice this supports is about containers, which
// are Linux.
func ownerOf(string) (fileOwner, bool) { return fileOwner{}, false }
