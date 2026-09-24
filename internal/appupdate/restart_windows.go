package appupdate

import (
	"os"
	"os/exec"
)

// Restart starts program in this process's place. Windows has no way to
// replace a running process, so the new one is started in the same console
// window - which stays open, because it is still in use - and handedOver says
// this one should now exit.
func Restart(program string, args, env []string) (handedOver bool, err error) {
	cmd := exec.Command(program, args...)
	cmd.Env = env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return false, err
	}
	return true, nil
}
