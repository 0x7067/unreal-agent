//go:build linux || darwin

package workerhost

import (
	"errors"
	"os/exec"
	"syscall"
)

// Isolate the worker from terminal broadcasts. Interrupt only the worker first:
// its owners must cancel and join shell groups and subagents in dependency order.
func isolateProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// This covers descendants still in the worker group, including an abandoned
// embedded runner. Shell tools own separate groups and join those in the worker.
// Deliberately detached processes are not a process-tree sandbox boundary.
func killProcessGroup(command *exec.Cmd) error {
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
