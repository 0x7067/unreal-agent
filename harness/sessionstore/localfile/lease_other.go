//go:build !linux && !darwin

package localfile

import (
	"errors"
	"os"
)

func acquireLeaseFile(directory, name string) (*os.File, error) {
	return nil, errors.New("exclusive session leases are unsupported on this platform")
}
