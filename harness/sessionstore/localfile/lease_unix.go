//go:build linux || darwin

package localfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func acquireLeaseFile(directory, name string) (*os.File, error) {
	// Anchor each lookup to an opened directory and apply NOFOLLOW to every
	// component, including existing parents. MkdirAll followed by an ordinary
	// open would allow a symlink to redirect the lease away from the store.
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = unix.Close(fd) }()
	uid := uint32(os.Geteuid())
	var directoryStat unix.Stat_t
	if err := unix.Fstat(fd, &directoryStat); err != nil {
		return nil, err
	}
	private := directoryStat.Uid == uid && directoryStat.Mode&0o077 == 0
	for _, part := range strings.Split(strings.TrimPrefix(directory, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			return nil, errors.New("invalid session lease directory component")
		}
		// Before the first private ancestor, the namespace must not let
		// another user rename a component. A root/current-user-owned sticky
		// directory (notably /tmp) is safe only when its child is ours.
		if directoryStat.Uid != 0 && directoryStat.Uid != uid {
			return nil, errors.New("session lease ancestor must be owned by root or current user")
		}
		shared := !private && directoryStat.Mode&0o022 != 0
		if shared && directoryStat.Mode&unix.S_ISVTX == 0 {
			return nil, errors.New("session lease ancestor is a shared writable namespace")
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENOENT) {
			if err := unix.Mkdirat(fd, part, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
				return nil, fmt.Errorf("create session lease directory: %w", err)
			}
			next, err = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		if err != nil {
			return nil, fmt.Errorf("open session lease directory: %w", err)
		}
		_ = unix.Close(fd)
		fd = next
		if err := unix.Fstat(fd, &directoryStat); err != nil {
			return nil, err
		}
		if shared && directoryStat.Uid != uid {
			return nil, errors.New("session lease child of sticky directory must be owned by current user")
		}
		if directoryStat.Uid != 0 && directoryStat.Uid != uid {
			return nil, errors.New("session lease ancestor must be owned by root or current user")
		}
		private = private || directoryStat.Uid == uid && directoryStat.Mode&0o077 == 0
	}
	if directoryStat.Uid != uid || !private {
		return nil, errors.New("session lease directory must be owned by current user and have a private ancestor (no group/world permissions)")
	}
	// Do not truncate, chmod, or write an existing file. NONBLOCK prevents a
	// malicious FIFO from blocking acquisition before its type is checked.
	lockFD, err := unix.Openat(fd, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open session lease file: %w", err)
	}
	file := os.NewFile(uintptr(lockFD), filepath.Join(directory, name))
	var stat unix.Stat_t
	if err := unix.Fstat(lockFD, &stat); err != nil {
		_ = file.Close()
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o7777 != 0o600 || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 {
		_ = file.Close()
		return nil, errors.New("session lease file must be a private (0600), singly-linked regular file owned by current user")
	}
	if err := unix.Flock(lockFD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrSessionLocked
		}
		return nil, fmt.Errorf("lock session lease file: %w", err)
	}
	// Fail closed if the name was replaced during acquisition. Never unlink:
	// neither this path nor the error path may detach an inode another owner
	// could still be using.
	var named unix.Stat_t
	if err := unix.Fstatat(fd, name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		_ = file.Close()
		return nil, err
	}
	if named.Dev != stat.Dev || named.Ino != stat.Ino || named.Nlink != 1 || named.Mode != stat.Mode || named.Uid != stat.Uid {
		_ = file.Close()
		return nil, errors.New("session lease file changed during acquisition")
	}
	return file, nil
}
