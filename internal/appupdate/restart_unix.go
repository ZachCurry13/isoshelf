//go:build !windows

package appupdate

import "syscall"

// Restart replaces this process with program. On Linux that is one call, and
// it keeps what matters: the same process, so a terminal keeps it in the
// foreground and Ctrl+C still reaches it, and a service manager keeps
// watching it. It returns only if it failed.
func Restart(program string, args, env []string) (handedOver bool, err error) {
	return false, syscall.Exec(program, append([]string{program}, args...), env)
}
