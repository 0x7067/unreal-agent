//go:build !linux && !darwin

package workerruntime

import "io"

func prepareInput(input io.Reader) (io.Reader, func(), error) {
	return input, func() {}, nil
}
