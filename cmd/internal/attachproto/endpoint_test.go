//go:build linux || darwin

package attachproto

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/session"
	"golang.org/x/sys/unix"
)

// testing.T.TempDir paths may exceed Darwin's 103-byte socket path budget.
func endpointDirs(t *testing.T) (string, string) {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "at-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(root, "state")
	if err := os.Mkdir(state, 0700); err != nil {
		t.Fatal(err)
	}
	return root, state
}

func TestEndpointLifecycleAndCredentials(t *testing.T) {
	root, state := endpointDirs(t)
	endpoint, err := Listen(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	if len(endpoint.Path) > 103 {
		t.Fatalf("socket path too long: %s", endpoint.Path)
	}
	info, err := os.Stat(endpoint.Path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("socket metadata: %v %v", info, err)
	}
	lockBefore, err := os.Stat(endpoint.Path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Listen(root, state, "test"); err == nil {
		other.Close()
		t.Fatal("duplicate endpoint ownership accepted")
	}
	if err := endpoint.Listener.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	accepted := make(chan error, 1)
	go func() {
		conn, err := endpoint.Listener.AcceptUnix()
		if err == nil {
			defer conn.Close()
			err = CheckPeer(conn)
		}
		accepted <- err
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := Dial(ctx, root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckPeer(conn); err != nil {
		t.Fatal(err)
	}
	conn.Close()
	if err := <-accepted; err != nil {
		t.Fatalf("server credentials: %v", err)
	}
	if err := CheckPeer(nil); err == nil {
		t.Fatal("nil credentials accepted")
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(endpoint.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains: %v", err)
	}
	lockAfter, err := os.Stat(endpoint.Path + ".lock")
	if err != nil || !os.SameFile(lockBefore, lockAfter) {
		t.Fatalf("persistent lock changed: %v", err)
	}
	next, err := Listen(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	lockNext, err := os.Stat(endpoint.Path + ".lock")
	if err != nil || !os.SameFile(lockBefore, lockNext) {
		t.Fatal("relisten replaced lock inode")
	}
}

func TestEndpointRuntimeValidation(t *testing.T) {
	root, state := endpointDirs(t)
	for _, mode := range []os.FileMode{0755, 0777, 0700 | os.ModeSticky} {
		if err := os.Chmod(root, mode); err != nil {
			t.Fatal(err)
		}
		if _, err := EndpointPath(root, state, "test"); err == nil {
			t.Fatalf("accepted runtime mode %o", mode)
		}
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	link := root + "-link"
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(link)
	if _, err := EndpointPath(link, state, "test"); err == nil {
		t.Fatal("followed runtime symlink")
	}
	for _, id := range []session.ID{"", ".", "..", "../escape", "a/b", "a b"} {
		if _, err := EndpointPath(root, state, id); err == nil {
			t.Fatalf("accepted session %q", id)
		}
	}
	for _, dir := range []string{"relative", root + "/../" + filepath.Base(root), "/tmp/" + strings.Repeat("x", 100)} {
		if _, err := EndpointPath(dir, state, "test"); err == nil {
			t.Fatalf("accepted runtime %q", dir)
		}
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(state, alias); err != nil {
		t.Fatal(err)
	}
	a, err := EndpointPath(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := EndpointPath(root, alias, "test")
	if err != nil || a != b {
		t.Fatalf("state alias differs: %s %s %v", a, b, err)
	}
}

func TestDialDoesNotMutateFilesystem(t *testing.T) {
	root, state := endpointDirs(t)
	path, err := EndpointPath(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := Dial(context.Background(), root, state, "test"); err == nil {
		conn.Close()
		t.Fatal("dial missing endpoint succeeded")
	}
	for _, p := range []string{path, path + ".lock"} {
		if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Dial created %s: %v", p, err)
		}
	}
	missing := filepath.Join(root, "missing")
	_, _ = Dial(context.Background(), missing, state, "test")
	if _, err := os.Stat(missing); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Dial created runtime: %v", err)
	}
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	stale.Close()
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if conn, err := Dial(context.Background(), root, state, "test"); err == nil {
		conn.Close()
		t.Fatal("stale dial succeeded")
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("Dial removed/replaced stale endpoint: %v", err)
	}
	if _, err := os.Stat(path + ".lock"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Dial created lock: %v", err)
	}
	endpoint, err := Listen(root, state, "test")
	if err != nil {
		t.Fatalf("stale socket recovery: %v", err)
	}
	defer endpoint.Close()
}

func TestEndpointRefusesForeignNamesAndActiveSocket(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "lockSymlink", "active"} {
		t.Run(kind, func(t *testing.T) {
			root, state := endpointDirs(t)
			path, err := EndpointPath(root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			target := path
			switch kind {
			case "file":
				err = os.WriteFile(path, []byte("keep"), 0600)
			case "symlink":
				err = os.Symlink(state, path)
			case "lockSymlink":
				target = path + ".lock"
				err = os.Symlink(state, target)
			case "active":
				var listener *net.UnixListener
				listener, err = net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
				if err == nil {
					listener.SetUnlinkOnClose(false)
					defer listener.Close()
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(target)
			if err != nil {
				t.Fatal(err)
			}
			if endpoint, err := Listen(root, state, "test"); err == nil {
				endpoint.Close()
				t.Fatal("unsafe existing name accepted")
			}
			after, err := os.Lstat(target)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("existing name changed: %v", err)
			}
		})
	}
}

func TestEndpointClosePreservesReplacement(t *testing.T) {
	root, state := endpointDirs(t)
	endpoint, err := Listen(root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	if err := os.Remove(endpoint.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(endpoint.Path, []byte("replacement"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := endpoint.Close(); err == nil {
		t.Fatal("replacement was not reported")
	}
	data, err := os.ReadFile(endpoint.Path)
	if err != nil || string(data) != "replacement" {
		t.Fatalf("replacement removed: %q %v", data, err)
	}
}

func TestSameSocketRequiresIdentityAndSingleLink(t *testing.T) {
	a := unix.Stat_t{Dev: 1, Ino: 2, Uid: uint32(os.Geteuid()), Mode: unix.S_IFSOCK | 0600, Nlink: 1}
	b := a
	if !sameSocket(&a, &b) {
		t.Fatal("identical socket rejected")
	}
	b.Ino++
	if sameSocket(&a, &b) {
		t.Fatal("different inode accepted")
	}
	b = a
	b.Nlink = 2
	if sameSocket(&a, &b) {
		t.Fatal("multiply linked socket accepted")
	}
	b = a
	b.Mode = unix.S_IFREG | 0600
	if sameSocket(&a, &b) {
		t.Fatal("regular file accepted")
	}
}

// A real kernel credential mismatch must fail in each direction, rather than
// merely checking that two connections from this test process are accepted.
func TestCheckPeerRejectsDifferentUIDBothDirections(t *testing.T) {
	const helperKey = "ATTACHPROTO_DIFFERENT_UID_PEER"
	if path := os.Getenv(helperKey); path != "" {
		connection, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		if err := CheckPeer(connection); err == nil {
			t.Fatal("client accepted a different-UID server")
		}
		return
	}
	if os.Geteuid() != 0 {
		t.Skip("changing peer UID requires root")
	}
	root, _ := endpointDirs(t)
	// This intentionally is not a production runtime root: another UID must be
	// able to reach the test socket to exercise the kernel credential rejection.
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "foreign.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, binary, "-test.run=^TestCheckPeerRejectsDifferentUIDBothDirections$", "-test.count=1")
	child.Env = append(os.Environ(), helperKey+"="+path)
	child.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 65534, Gid: 65534}}
	var output strings.Builder
	child.Stdout = &output
	child.Stderr = &output
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	connection, acceptErr := listener.AcceptUnix()
	var peerErr error
	if acceptErr == nil {
		peerErr = CheckPeer(connection)
		connection.Close()
	}
	// Always reap the helper, including on acceptance failure.
	childErr := child.Wait()
	if acceptErr != nil {
		t.Fatalf("accept: %v; helper: %v %s", acceptErr, childErr, output.String())
	}
	if peerErr == nil {
		t.Fatal("server accepted a different-UID client")
	}
	if childErr != nil {
		t.Fatalf("client credential rejection: %v %s", childErr, output.String())
	}
}
