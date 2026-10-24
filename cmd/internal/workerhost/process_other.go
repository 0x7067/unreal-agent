//go:build !linux && !darwin

package workerhost

import "os/exec"

func isolateProcess(command *exec.Cmd)         {}
func killProcessGroup(command *exec.Cmd) error { return command.Process.Kill() }
