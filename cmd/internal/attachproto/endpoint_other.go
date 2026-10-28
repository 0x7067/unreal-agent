//go:build !linux && !darwin

package attachproto

import (
	"context"
	"errors"
	"net"

	"github.com/unreallabsai/unreal-agent/harness/session"
)

func EndpointPath(runtimeDir, stateDir string, id session.ID) (string, error) {
	return "", errors.New("secure local attach is unsupported on this platform")
}

func Listen(runtimeDir, stateDir string, id session.ID) (*Endpoint, error) {
	return nil, errors.New("secure local attach is unsupported on this platform")
}

func Dial(ctx context.Context, runtimeDir, stateDir string, id session.ID) (*net.UnixConn, error) {
	return nil, errors.New("secure local attach is unsupported on this platform")
}

func ValidateRuntimeDirectory(string) error {
	return errors.New("secure Unix attach is unsupported on this platform")
}
