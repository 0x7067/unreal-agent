//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package operation

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Open every path component relative to an already opened directory descriptor.
// O_NOFOLLOW applies to each component, not just the final file. Never chmod,
// truncate, or replace an existing object (including hard links and symlinks).
func saveRemoteJobArtifact(directory, name, text string) error {
	fd, err := unix.Open(string(filepath.Separator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer func() { unix.Close(fd) }()
	parts := strings.Split(strings.TrimPrefix(directory, string(filepath.Separator)), string(filepath.Separator))
	for i, part := range parts {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid artifact directory component")
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENOENT) {
			if err = unix.Mkdirat(fd, part, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
				return err
			}
			next, err = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		if err != nil {
			return err
		}
		unix.Close(fd)
		fd = next
		if i == len(parts)-1 {
			var st unix.Stat_t
			if err := unix.Fstat(fd, &st); err != nil {
				return err
			}
			if st.Mode&0o777 != 0o700 || st.Uid != uint32(os.Geteuid()) {
				return fmt.Errorf("artifact directory must be private and owned by current user")
			}
		}
	}
	out, err := unix.Openat(fd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0o600)
	if errors.Is(err, unix.EEXIST) {
		// Content-addressed names make replay idempotent. Reuse only an exact,
		// private regular file. O_NONBLOCK avoids hanging on a malicious FIFO.
		old, err := unix.Openat(fd, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		file := os.NewFile(uintptr(old), name)
		defer file.Close()
		var st unix.Stat_t
		if err := unix.Fstat(old, &st); err != nil {
			return err
		}
		if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0o777 != 0o600 || st.Uid != uint32(os.Geteuid()) || st.Nlink != 1 || st.Size != int64(len(text)) {
			return fmt.Errorf("existing artifact is not a private matching regular file")
		}
		data, err := io.ReadAll(io.LimitReader(file, int64(len(text))+1))
		if err != nil {
			return err
		}
		if string(data) != text {
			return fmt.Errorf("existing artifact contents differ")
		}
		return nil
	}
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(out), name)
	_, writeErr := io.WriteString(file, text)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	return errors.Join(writeErr, file.Close())
}
