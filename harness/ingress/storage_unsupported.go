//go:build !linux && !darwin

package ingress

import (
	"errors"
	"os"
)

func openStorage(directory, id string) (*os.File, *os.File, error) {
	return nil, nil, errors.New("durable ingress ownership is unsupported on this platform")
}
func releaseLock(lock *os.File) error { return lock.Close() }
