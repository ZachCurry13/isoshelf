//go:build !windows

package scan

import (
	"os"
	"time"
)

// created has nothing portable to go on elsewhere: Linux keeps a birth time
// on some filesystems only, behind a newer system call. isoshelf remembers
// when it first saw a file instead.
func created(os.FileInfo) time.Time { return time.Time{} }
