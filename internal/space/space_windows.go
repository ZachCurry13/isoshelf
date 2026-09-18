package space

import (
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// of asks Windows about the volume holding dir. It works for drive letters,
// mapped network drives and UNC paths alike.
func of(dir string) (Usage, error) {
	path, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return Usage{}, err
	}
	// The first number is what this user may write, which is smaller than the
	// free space when the volume has a quota.
	var available, total, free uint64
	ret, _, err := getDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(path)),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)),
	)
	if ret == 0 {
		return Usage{}, err
	}
	return Usage{Free: int64(available), Total: int64(total)}, nil
}
