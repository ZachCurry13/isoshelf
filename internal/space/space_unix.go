//go:build !windows

package space

import "syscall"

// of asks the filesystem holding dir. Bavail rather than Bfree, because the
// blocks reserved for root are not room this user has.
func of(dir string) (Usage, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(dir, &fs); err != nil {
		return Usage{}, err
	}
	block := int64(fs.Bsize)
	return Usage{
		Free:  int64(fs.Bavail) * block,
		Total: int64(fs.Blocks) * block,
	}, nil
}
