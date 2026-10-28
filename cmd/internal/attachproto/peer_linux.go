//go:build linux

package attachproto

import (
	"errors"
	"net"

	"golang.org/x/sys/unix"
)

// CheckPeer validates the kernel-supplied credentials, not wire claims. Servers
// must call this immediately after AcceptUnix; Dial calls it for the client.
func CheckPeer(connection *net.UnixConn) error {
	if connection == nil {
		return errors.New("nil Unix peer")
	}
	raw, err := connection.SyscallConn()
	if err != nil {
		return err
	}
	var peerErr error
	if err := raw.Control(func(fd uintptr) {
		credentials, err := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if err != nil {
			peerErr = err
		} else {
			peerErr = authorizePeerUID(credentials.Uid)
		}
	}); err != nil {
		return err
	}
	return peerErr
}
