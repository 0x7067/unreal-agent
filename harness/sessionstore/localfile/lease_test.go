//go:build linux || darwin

package localfile

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/session"
	"golang.org/x/sys/unix"
)

func leaseTestDirectory(t *testing.T) string {
	t.Helper()
	// Darwin's system temporary directory can have a symlink ancestor. Tests
	// supply the real path; the lease API intentionally rejects symlink paths.
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func TestLeaseExclusiveCloseAndReopen(t *testing.T) {
	directory := leaseTestDirectory(t)
	const id = session.ID("exclusive")
	lease, err := AcquireLease(directory, id)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Close() })
	path := filepath.Join(directory, string(id)+sessionLeaseSuffix)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.Mode().Perm() != 0o600 || before.Size() != 0 {
		t.Fatalf("lock mode/size = %v/%d, want 0600/0 (not a PID file)", before.Mode(), before.Size())
	}
	if other, err := AcquireLease(directory, id); !errors.Is(err, ErrSessionLocked) {
		_ = other.Close()
		t.Fatalf("competing acquire = %v, want ErrSessionLocked", err)
	}
	independent, err := AcquireLease(directory, "different")
	if err != nil {
		t.Fatalf("independent session: %v", err)
	}
	defer independent.Close()

	var closers sync.WaitGroup
	for range 8 {
		closers.Go(func() {
			if err := lease.Close(); err != nil {
				t.Errorf("Close: %v", err)
			}
		})
	}
	closers.Wait()
	reopened, err := AcquireLease(directory, id)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("Close/reopen replaced the stable lock inode")
	}
	// Closing the old lease again must not release a later owner's lease.
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if other, err := AcquireLease(directory, id); !errors.Is(err, ErrSessionLocked) {
		_ = other.Close()
		t.Fatalf("acquire after stale Close = %v, want ErrSessionLocked", err)
	}
	if err := (*Lease)(nil).Close(); err != nil {
		t.Fatal(err)
	}
	if err := new(Lease).Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseAllowsEffectivelyPrivateDirectory(t *testing.T) {
	// In Go 1.27 t.TempDir's leaf can be 0777 & umask, beneath testing's
	// private 0700 ancestor. Do not "fix" the caller's leaf permissions.
	for _, mode := range []os.FileMode{0o755, 0o775, 0o777} {
		t.Run(fmt.Sprintf("%o", mode), func(t *testing.T) {
			root := leaseTestDirectory(t)
			if err := os.Chmod(root, 0o700); err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(root, "public", "sessions")
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{filepath.Dir(directory), directory} {
				if err := os.Chmod(path, mode); err != nil {
					t.Fatal(err)
				}
			}
			lease, err := AcquireLease(directory, "inherited-private")
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			if other, err := AcquireLease(directory, "inherited-private"); !errors.Is(err, ErrSessionLocked) {
				_ = other.Close()
				t.Fatalf("competing acquisition = %v, want ErrSessionLocked", err)
			}
			for _, path := range []string{filepath.Dir(directory), directory} {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != mode {
					t.Fatalf("caller directory permissions changed: %v, %v", info, err)
				}
			}
		})
	}
}

func TestLeaseRejectsTrulySharedNamespace(t *testing.T) {
	// Deliberately do not use t.TempDir: its private ancestor would make a
	// world-writable leaf effectively private. /tmp is a system shared root
	// on both supported platforms; resolve only its platform alias.
	tmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp(tmp, "unreal-lease-shared-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o777); err != nil {
		t.Fatal(err)
	}
	privateChild := filepath.Join(root, "private")
	if err := os.Mkdir(privateChild, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{root, privateChild} {
		if lease, err := AcquireLease(directory, "shared"); err == nil {
			_ = lease.Close()
			t.Fatalf("accepted renameable shared namespace %q", directory)
		}
		if _, err := os.Lstat(filepath.Join(directory, "shared"+sessionLeaseSuffix)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("rejected acquisition created lock: %v", err)
		}
	}
	info, err := os.Stat(root)
	if err != nil || info.Mode().Perm() != 0o777 {
		t.Fatalf("shared caller directory permissions changed: %v, %v", info, err)
	}
}

func TestLeaseCreatesPrivateDirectories(t *testing.T) {
	directory := filepath.Join(leaseTestDirectory(t), "nested", "sessions")
	lease, err := AcquireLease(directory, "new")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	for _, path := range []string{directory, filepath.Dir(directory)} {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o700 {
			t.Fatalf("directory %q: info=%v err=%v", path, info, err)
		}
	}
}

func TestLeaseRejectsInvalidPaths(t *testing.T) {
	for _, id := range []session.ID{"", ".", "..", "../outside", "slash/name", "white space", "with_underscore", "new\nline"} {
		t.Run(fmt.Sprintf("id-%q", id), func(t *testing.T) {
			directory := leaseTestDirectory(t)
			if lease, err := AcquireLease(directory, id); err == nil {
				_ = lease.Close()
				t.Fatal("accepted invalid ID")
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid ID changed directory: %v, %v", entries, err)
			}
		})
	}
	if lease, err := AcquireLease(" \t ", "valid"); err == nil {
		_ = lease.Close()
		t.Fatal("accepted empty directory")
	}
	for _, intermediate := range []bool{false, true} {
		t.Run(fmt.Sprintf("symlink-directory-intermediate-%v", intermediate), func(t *testing.T) {
			root := leaseTestDirectory(t)
			target := filepath.Join(root, "real")
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(root, "link")
			if err := os.Symlink(target, link); err != nil {
				t.Fatal(err)
			}
			directory := link
			if intermediate {
				directory = filepath.Join(link, "sessions")
			}
			if lease, err := AcquireLease(directory, "symlink"); err == nil {
				_ = lease.Close()
				t.Fatal("accepted symlink directory")
			}
			entries, err := os.ReadDir(target)
			if err != nil || len(entries) != 0 {
				t.Fatalf("symlink target changed: %v, %v", entries, err)
			}
		})
	}
	for _, kind := range []string{"symlink", "hardlink", "fifo", "directory", "public-file"} {
		t.Run(kind, func(t *testing.T) {
			directory := leaseTestDirectory(t)
			path := filepath.Join(directory, "unsafe"+sessionLeaseSuffix)
			target := filepath.Join(leaseTestDirectory(t), "untouched")
			if err := os.WriteFile(target, []byte("untouched"), 0o600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, path)
			case "hardlink":
				err = os.Link(target, path)
			case "fifo":
				err = unix.Mkfifo(path, 0o600)
			case "directory":
				err = os.Mkdir(path, 0o700)
			case "public-file":
				err = os.WriteFile(path, nil, 0o600)
				if err == nil {
					err = os.Chmod(path, 0o644)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if lease, err := AcquireLease(directory, "unsafe"); err == nil {
				_ = lease.Close()
				t.Fatal("accepted unsafe lease path")
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "untouched" {
				t.Fatalf("target modified: %q, %v", data, err)
			}
		})
	}
}

const leaseChildEnvironment = "UNREAL_AGENT_LEASE_TEST_CHILD"

// TestLeaseSubprocess is invoked in an independent process by the tests below.
func TestLeaseSubprocess(t *testing.T) {
	mode := os.Getenv(leaseChildEnvironment)
	if mode == "" {
		t.Skip("subprocess helper")
	}
	lease, err := AcquireLease(os.Getenv("UNREAL_AGENT_LEASE_TEST_DIRECTORY"), "process")
	if mode == "probe" && errors.Is(err, ErrSessionLocked) {
		os.Exit(23)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if mode == "hold" {
		fmt.Println("READY")
		_, _ = io.Copy(io.Discard, os.Stdin)
	}
}

func leaseChildCommand(t *testing.T, directory, mode string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLeaseSubprocess$")
	cmd.Env = append(os.Environ(), leaseChildEnvironment+"="+mode, "UNREAL_AGENT_LEASE_TEST_DIRECTORY="+directory)
	return cmd
}

func TestLeaseConflictsAcrossProcesses(t *testing.T) {
	directory := leaseTestDirectory(t)
	lease, err := AcquireLease(directory, "process")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	cmd := leaseChildCommand(t, directory, "probe")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 23 {
		t.Fatalf("child acquisition: err=%v output=%s, want locked exit 23", err, output)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	cmd = leaseChildCommand(t, directory, "probe")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("child acquisition after close: %v, %s", err, output)
	}
}

func TestLeaseReleasedAfterProcessCrash(t *testing.T) {
	directory := leaseTestDirectory(t)
	cmd := leaseChildCommand(t, directory, "hold")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var stop sync.Once
	kill := func() {
		stop.Do(func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		})
	}
	t.Cleanup(kill)
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "READY" {
		kill()
		t.Fatalf("child readiness = %q, %v, stderr=%s", line, err, stderr.String())
	}
	if lease, err := AcquireLease(directory, "process"); !errors.Is(err, ErrSessionLocked) {
		_ = lease.Close()
		t.Fatalf("child-owned lease = %v, want ErrSessionLocked", err)
	}
	path := filepath.Join(directory, "process"+sessionLeaseSuffix)
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	kill() // No child Close or cleanup: kernel must release the flock.
	lease, err := AcquireLease(directory, "process")
	if err != nil {
		t.Fatalf("acquire after crash: %v", err)
	}
	defer lease.Close()
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("crash/reopen changed lock inode: %v", err)
	}
}

func TestLeaseIsSeparateFromSessionHistory(t *testing.T) {
	directory := leaseTestDirectory(t)
	lease, err := AcquireLease(directory, "history")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	listed, err := store.ListSessions(t.Context())
	if err != nil || len(listed) != 0 {
		t.Fatalf("lock file listed as session: %v, %v", listed, err)
	}
	// Store methods remain usable by the caller holding a lease; they must
	// not auto-lock and conflict with their own owner's lease.
	if _, err := store.Create(t.Context(), "history"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resume(t.Context(), "history"); err != nil {
		t.Fatal(err)
	}
	if other, err := AcquireLease(directory, "history"); !errors.Is(err, ErrSessionLocked) {
		_ = other.Close()
		t.Fatalf("session creation disturbed lease: %v", err)
	}
	listed, err = store.ListSessions(t.Context())
	if err != nil || len(listed) != 1 || listed[0].ID != "history" {
		t.Fatalf("session listing with lease = %v, %v", listed, err)
	}
}
