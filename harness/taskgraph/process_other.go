//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package taskgraph

func processGroupAbsent(group int) bool { return false }
