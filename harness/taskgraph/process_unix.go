//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package taskgraph

import (
	"errors"
	"golang.org/x/sys/unix"
)

// A stored group is evidence for a liveness query only, never a kill target.
func processGroupAbsent(group int) bool {
	return group > 1 && errors.Is(unix.Kill(-group, 0), unix.ESRCH)
}
