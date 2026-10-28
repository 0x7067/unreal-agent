//go:build linux || darwin

package attachproto

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/session"
	"golang.org/x/sys/unix"
)

// openRuntime anchors every lookup to a descriptor and never follows a symlink.
// The root is pre-existing and exactly 0700. Neither Listen nor Dial mutates its
// ancestors. Root/current-user ancestors must be non-writable by other users;
// a sticky shared ancestor is allowed only with a private, user-owned child.
func openRuntime(directory string) (int, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return -1, errors.New("runtime directory must be an explicit clean absolute path")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = unix.Close(fd)
		}
	}()
	uid := uint32(os.Geteuid())
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return -1, err
	}
	for _, component := range strings.Split(strings.TrimPrefix(directory, "/"), "/") {
		if component == "" || component == "." || component == ".." {
			return -1, errors.New("invalid runtime directory component")
		}
		if stat.Uid != 0 && stat.Uid != uid {
			return -1, errors.New("runtime ancestor must be owned by root or current user")
		}
		shared := stat.Mode&0o022 != 0
		if shared && stat.Mode&unix.S_ISVTX == 0 {
			return -1, errors.New("runtime ancestor is writable by other users")
		}
		next, err := unix.Openat(fd, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return -1, fmt.Errorf("open runtime directory: %w", err)
		}
		_ = unix.Close(fd)
		fd = next
		if err := unix.Fstat(fd, &stat); err != nil {
			return -1, err
		}
		if shared && (stat.Uid != uid || stat.Mode&0o7777 != 0o700) {
			return -1, errors.New("sticky shared ancestor requires a private user-owned child")
		}
	}
	if stat.Uid != uid || stat.Mode&0o7777 != 0o700 {
		return -1, errors.New("runtime directory must be owned by current user with mode 0700")
	}
	ok = true
	return fd, nil
}

// ValidateRuntimeDirectory checks the transport namespace without creating
// state, lock files or sockets, before any worker or provider can be started.
func ValidateRuntimeDirectory(directory string) error {
	fd, err := openRuntime(directory)
	if err != nil {
		return err
	}
	_ = unix.Close(fd)
	if len(filepath.Join(directory, strings.Repeat("0", 64)+".sock")) > 103 {
		return errors.New("Unix endpoint path exceeds 103 bytes")
	}
	return nil
}

// EndpointPath returns the canonical state/session namespace in an existing,
// private runtime directory. It does not create directories or files.
func EndpointPath(runtimeDir, stateDir string, id session.ID) (string, error) {
	path, err := endpointPath(runtimeDir, stateDir, id)
	if err != nil {
		return "", err
	}
	fd, err := openRuntime(runtimeDir)
	if err != nil {
		return "", err
	}
	_ = unix.Close(fd)
	return path, nil
}

func sameSocket(a, b *unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Uid == b.Uid &&
		a.Mode&unix.S_IFMT == unix.S_IFSOCK && b.Mode&unix.S_IFMT == unix.S_IFSOCK &&
		a.Nlink == 1 && b.Nlink == 1
}

// lockEndpoint locks a persistent inode, not the socket itself. A process must
// own this lock before examining/removing a stale endpoint, and until cleanup.
func lockEndpoint(directoryFD int, name string) (*os.File, error) {
	fd, err := unix.Openat(directoryFD, name, unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	fail := func(err error) (*os.File, error) { _ = file.Close(); return nil, err }
	var stat, named unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fail(err)
	}
	if stat.Uid != uint32(os.Geteuid()) || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o7777 != 0o600 || stat.Nlink != 1 {
		return fail(errors.New("endpoint lock must be a private, singly-linked, user-owned regular file"))
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fail(fmt.Errorf("endpoint is already owned or cannot be locked: %w", err))
	}
	if err := unix.Fstatat(directoryFD, name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fail(err)
	}
	if stat.Dev != named.Dev || stat.Ino != named.Ino || stat.Mode != named.Mode || stat.Uid != named.Uid || named.Nlink != 1 {
		return fail(errors.New("endpoint lock changed during acquisition"))
	}
	return file, nil
}

func removeStale(directoryFD int, name, path string) error {
	var before unix.Stat_t
	if err := unix.Fstatat(directoryFD, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil
		}
		return err
	}
	if before.Mode&unix.S_IFMT != unix.S_IFSOCK || before.Uid != uint32(os.Geteuid()) || before.Nlink != 1 {
		return errors.New("existing endpoint is not a singly-linked user-owned socket")
	}
	connection, err := net.DialTimeout("unix", path, 250*time.Millisecond)
	if err == nil {
		_ = connection.Close()
		return errors.New("existing endpoint has an active listener")
	}
	if !errors.Is(err, unix.ECONNREFUSED) {
		return fmt.Errorf("cannot establish that endpoint is stale: %w", err)
	}
	var after unix.Stat_t
	if err := unix.Fstatat(directoryFD, name, &after, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if !sameSocket(&before, &after) {
		return errors.New("endpoint changed during stale check")
	}
	return unix.Unlinkat(directoryFD, name, 0)
}

func Listen(runtimeDir, stateDir string, id session.ID) (*Endpoint, error) {
	path, err := endpointPath(runtimeDir, stateDir, id)
	if err != nil {
		return nil, err
	}
	directoryFD, err := openRuntime(runtimeDir)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)
	lock, err := lockEndpoint(directoryFD, name+".lock")
	if err != nil {
		_ = unix.Close(directoryFD)
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = unix.Close(directoryFD)
			_ = lock.Close()
		}
	}()
	if err := removeStale(directoryFD, name, path); err != nil {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		return nil, err
	}
	// Go's automatic path cleanup is not inode-safe; only our Close may unlink.
	listener.SetUnlinkOnClose(false)
	var bound unix.Stat_t
	if err := unix.Fstatat(directoryFD, name, &bound, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		_ = listener.Close()
		return nil, err // No identity evidence: never remove an unowned name.
	}
	if bound.Mode&unix.S_IFMT != unix.S_IFSOCK || bound.Uid != uint32(os.Geteuid()) || bound.Nlink != 1 {
		_ = listener.Close()
		return nil, errors.New("bound endpoint is not a user-owned socket")
	}
	cleanup := func() error {
		var named unix.Stat_t
		if err := unix.Fstatat(directoryFD, name, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			if errors.Is(err, unix.ENOENT) {
				return nil
			}
			return err
		}
		if !sameSocket(&bound, &named) {
			return errors.New("endpoint replaced; refusing to remove it")
		}
		return unix.Unlinkat(directoryFD, name, 0)
	}
	// The private directory protects the bind-to-chmod interval. Never change
	// the process-global umask, which would race unrelated goroutines.
	if err := unix.Fchmodat(directoryFD, name, 0o600, 0); err != nil {
		_ = listener.Close()
		_ = cleanup()
		return nil, err
	}
	endpoint := &Endpoint{Listener: listener, Path: path}
	endpoint.closeFn = func() error {
		closeErr := listener.Close()
		if errors.Is(closeErr, net.ErrClosed) {
			closeErr = nil
		}
		cleanupErr := cleanup()
		dirErr := unix.Close(directoryFD)
		lockErr := lock.Close() // Ownership is always released last.
		return errors.Join(closeErr, cleanupErr, dirErr, lockErr)
	}
	ok = true
	return endpoint, nil
}

// Dial only reads existing directory/socket metadata; it never creates a root,
// lock file, or socket and never removes a stale endpoint.
func Dial(ctx context.Context, runtimeDir, stateDir string, id session.ID) (*net.UnixConn, error) {
	path, err := endpointPath(runtimeDir, stateDir, id)
	if err != nil {
		return nil, err
	}
	fd, err := openRuntime(runtimeDir)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	var stat unix.Stat_t
	if err := unix.Fstatat(fd, filepath.Base(path), &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFSOCK || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0o7777 != 0o600 || stat.Nlink != 1 {
		return nil, errors.New("endpoint must be a private, singly-linked, user-owned socket")
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "unix", path)
	if err != nil {
		return nil, err
	}
	peer, ok := connection.(*net.UnixConn)
	if !ok {
		_ = connection.Close()
		return nil, errors.New("endpoint did not provide a Unix connection")
	}
	if err := CheckPeer(peer); err != nil {
		_ = peer.Close()
		return nil, err
	}
	return peer, nil
}
