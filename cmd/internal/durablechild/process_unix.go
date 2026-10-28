//go:build linux || darwin

package durablechild

import (
	"os/exec"
	"syscall"
)

// The independent host must not inherit the replaceable parent's process group.
// It owns its own worker groups; this does not detach it from an owning cgroup.
func isolateChild(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return nil
}
