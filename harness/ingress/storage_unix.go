//go:build linux || darwin

package ingress

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// Walk directory descriptors rather than checking paths and then reopening them.
// O_NOFOLLOW is applied to every component, including the supplied storage root.
func privateDirectory(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	current := os.NewFile(uintptr(fd), "/")
	components := strings.Split(strings.TrimPrefix(absolute, "/"), "/")
	for _, part := range components {
		if part == "" {
			continue
		}
		next, err := openDirectoryAt(current, part)
		_ = current.Close()
		if err != nil {
			return nil, err
		}
		current = next
	}
	if err := checkPrivate(current, true); err != nil {
		_ = current.Close()
		return nil, err
	}
	return current, nil
}

func openDirectoryAt(parent *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENOENT) {
		if err = unix.Mkdirat(int(parent.Fd()), name, 0700); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, err
		}
		fd, err = unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	}
	if err != nil {
		return nil, fmt.Errorf("open ingress directory %q: %w", name, err)
	}
	file := os.NewFile(uintptr(fd), name)
	// Also sync existing links: a prior failed Open may have created this
	// directory but failed its parent-directory fsync before returning.
	if err := parent.Sync(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func checkPrivate(file *os.File, directory bool) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return err
	}
	kind := uint32(unix.S_IFREG)
	if directory {
		kind = unix.S_IFDIR
	}
	if uint32(stat.Mode)&unix.S_IFMT != kind || uint32(stat.Mode)&0077 != 0 || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("ingress %q must be privately owned with no group/world permissions", file.Name())
	}
	if !directory && stat.Nlink != 1 {
		return fmt.Errorf("ingress %q must not have hard links", file.Name())
	}
	return nil
}

func openPrivateFile(parent *os.File, name string) (*os.File, error) {
	flags := unix.O_RDWR | unix.O_APPEND | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
	fd, err := unix.Openat(int(parent.Fd()), name, flags|unix.O_CREAT|unix.O_EXCL, 0600)
	if errors.Is(err, unix.EEXIST) {
		fd, err = unix.Openat(int(parent.Fd()), name, flags, 0)
	}
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if err := checkPrivate(file, false); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func openStorage(directory, id string) (*os.File, *os.File, error) {
	root, err := privateDirectory(directory)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	dir, err := openDirectoryAt(root, id)
	if err != nil {
		return nil, nil, err
	}
	defer dir.Close()
	if err := checkPrivate(dir, true); err != nil {
		return nil, nil, err
	}
	lock, err := openPrivateFile(dir, ".lock")
	if err != nil {
		return nil, nil, err
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, nil, ErrLocked
		}
		return nil, nil, fmt.Errorf("lock ingress: %w", err)
	}
	// No data file is opened (or repaired) until ownership is acquired.
	file, err := openPrivateFile(dir, "journal.jsonl")
	if err != nil {
		_ = releaseLock(lock)
		return nil, nil, err
	}
	if err = errors.Join(lock.Sync(), file.Sync(), dir.Sync()); err != nil {
		_ = file.Close()
		_ = releaseLock(lock)
		return nil, nil, err
	}
	return file, lock, nil
}

func releaseLock(lock *os.File) error {
	return errors.Join(unix.Flock(int(lock.Fd()), unix.LOCK_UN), lock.Close())
}
