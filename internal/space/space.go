// Package space reports how much room is left where images are kept, so
// isoshelf can say whether a download will fit before it starts one.
//
// It uses the operating system's own call rather than adding a dependency:
// GetDiskFreeSpaceExW on Windows, statfs elsewhere. Both understand network
// shares, which matters because a target is often a NAS folder.
package space

import "fmt"

// Usage is the room on the disk holding a folder.
type Usage struct {
	// Free is what this user can still write, in bytes. On filesystems that
	// keep a reserve for root, that reserve is already subtracted.
	Free int64
	// Total is the size of the filesystem.
	Total int64
}

// Known reports whether the numbers mean anything. Some filesystems and
// network shares don't say.
func (u Usage) Known() bool { return u.Total > 0 }

// Fits reports whether a download of size bytes would fit, keeping a little
// room spare. A size that isn't known yet always fits.
func (u Usage) Fits(size int64) bool {
	if !u.Known() || size <= 0 {
		return true
	}
	return u.Free-size >= spare(u.Total)
}

// spare is the room left alone so the disk never ends up completely full:
// 1% of it, at most 1 GB.
func spare(total int64) int64 {
	const gigabyte = 1 << 30
	if reserve := total / 100; reserve < gigabyte {
		return reserve
	}
	return gigabyte
}

// Of returns the space on the disk holding dir.
func Of(dir string) (Usage, error) {
	u, err := of(dir)
	if err != nil {
		return Usage{}, fmt.Errorf("couldn't read the free space of %s: %w", dir, err)
	}
	return u, nil
}
