package scan

import (
	"os"
	"syscall"
	"time"
)

// created returns when a file was created in the folder it's in. Windows keeps
// this for every file, and a copy gets a new one, so it is when the image
// arrived here, not when it was built. Network shares report it too.
func created(info os.FileInfo) time.Time {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, data.CreationTime.Nanoseconds())
	}
	return time.Time{}
}
