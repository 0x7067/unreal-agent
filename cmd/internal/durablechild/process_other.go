//go:build !linux && !darwin

package durablechild

import (
	"errors"
	"os/exec"
)

func isolateChild(*exec.Cmd) error {
	return errors.New("independently owned child hosts unsupported on this platform")
}
