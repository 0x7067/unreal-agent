//go:build darwin

package attachproto

import (
	"errors"
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// xucredVersion is XUCRED_VERSION from <sys/ucred.h>; x/sys/unix does not export it.
const xucredVersion = 0

// CheckPeer validates Darwin's kernel xucred on both accepted and dialed peers.
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
		credentials, err := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if err != nil {
			peerErr = err
		} else if credentials.Version != xucredVersion {
			peerErr = fmt.Errorf("Unix peer has unsupported credentials")
		} else {
			peerErr = authorizePeerUID(credentials.Uid)
		}
	}); err != nil {
		return err
	}
	return peerErr
}
