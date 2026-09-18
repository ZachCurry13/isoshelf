package drives

import (
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getVolumeInformationW = kernel32.NewProc("GetVolumeInformationW")
)

// label asks Windows for a volume's name. It works for drive letters, mapped
// network drives and UNC paths; an unlabelled or unreadable volume gives "".
func label(path string) string {
	root, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	// A volume label is at most 32 characters, but the buffer has to hold a
	// full component name plus its terminator.
	name := make([]uint16, syscall.MAX_PATH+1)
	ret, _, _ := getVolumeInformationW.Call(
		uintptr(unsafe.Pointer(root)),
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)),
		0, 0, 0, 0, 0,
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(name)
}
