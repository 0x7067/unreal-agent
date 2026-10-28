//go:build !linux && !darwin

package attachproto

import (
	"errors"
	"net"
)

func CheckPeer(connection *net.UnixConn) error {
	return errors.New("Unix peer credentials are unsupported on this platform")
}
