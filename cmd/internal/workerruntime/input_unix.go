//go:build linux || darwin

package workerruntime

import (
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// Inherited stdin is created with os.NewFile on a blocking descriptor, so it
// is not registered with Go's poller and SetReadDeadline returns ErrNoDeadline.
// Give the pump a private pollable descriptor. Dup shares status flags: Run is
// the exclusive reader, and cleanup restores those flags before returning the
// caller's still-open descriptor. Never close a file under a blocking syscall:
// that does not reliably interrupt an inherited stdin read either.
func prepareInput(input io.Reader) (io.Reader, func(), error) {
	file, ok := input.(*os.File)
	if !ok {
		return input, func() {}, nil
	}
	if file.SetReadDeadline(time.Time{}) == nil {
		return input, func() {}, nil
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		return input, func() {}, nil
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, nil, err
	}
	var fd, flags int
	var setupErr error
	err = raw.Control(func(original uintptr) {
		flags, setupErr = unix.FcntlInt(original, unix.F_GETFL, 0)
		if setupErr != nil {
			return
		}
		fd, setupErr = unix.FcntlInt(original, unix.F_DUPFD_CLOEXEC, 0)
		if setupErr != nil {
			return
		}
		setupErr = unix.SetNonblock(fd, true)
		if setupErr != nil {
			_ = unix.Close(fd)
		}
	})
	if err != nil {
		return nil, nil, err
	}
	if setupErr != nil {
		return nil, nil, setupErr
	}
	reader := os.NewFile(uintptr(fd), file.Name())
	return reader, func() {
		// The pump has joined before cleanup, so no read can race flag restoration.
		_, _ = unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags)
		_ = reader.Close()
	}, nil
}
