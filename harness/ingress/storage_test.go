//go:build linux || darwin

package ingress

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/session"
)

func TestLockProcessHelper(t *testing.T) {
	if os.Getenv("INGRESS_TEST_HELPER") != "1" {
		return
	}
	j, err := Open(os.Getenv("INGRESS_TEST_DIRECTORY"), "session")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := j.Submit(context.Background(), input("child")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("READY")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	if err := j.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	os.Exit(0)
}

func TestExclusiveOwnershipAndProcessDeath(t *testing.T) {
	for _, kill := range []bool{false, true} {
		t.Run(fmt.Sprint("kill=", kill), func(t *testing.T) {
			dir := tempDirectory(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLockProcessHelper$")
			cmd.Env = append(os.Environ(), "INGRESS_TEST_HELPER=1", "INGRESS_TEST_DIRECTORY="+dir)
			cmd.Stderr = os.Stderr
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				_ = stdin.Close()
				if !waited {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			}()
			ready, err := bufio.NewReader(stdout).ReadString('\n')
			if err != nil || ready != "READY\n" {
				t.Fatalf("helper readiness %q: %v", ready, err)
			}
			lockPath := filepath.Join(dir, "session", ".lock")
			before, err := os.Stat(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			if other, err := Open(dir, "session"); !errors.Is(err, ErrLocked) {
				if other != nil {
					_ = other.Close()
				}
				t.Fatalf("competing process open: %v", err)
			}
			if kill {
				if err := cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := io.WriteString(stdin, "close\n"); err != nil {
					t.Fatal(err)
				}
			}
			err = cmd.Wait()
			waited = true
			if !kill && err != nil {
				t.Fatal(err)
			}
			if kill && err == nil {
				t.Fatal("killed helper unexpectedly succeeded")
			}
			j := openTest(t, dir)
			if got := pendingIDs(j); got != "child" {
				t.Fatal("lost durable child admission", got)
			}
			after, err := os.Stat(lockPath)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Fatal("lock inode replaced")
			}
			if other, err := Open(dir, "session"); !errors.Is(err, ErrLocked) {
				if other != nil {
					_ = other.Close()
				}
				t.Fatalf("same-process competing open: %v", err)
			}
		})
	}
}

func TestPathConfinementAndPrivatePermissions(t *testing.T) {
	dir := tempDirectory(t)
	for _, id := range []string{"", ".", "..", "../escape", "/absolute", "a/b", `a\b`, "with space", strings.Repeat("x", 129)} {
		if j, err := Open(dir, session.ID(id)); err == nil {
			_ = j.Close()
			t.Fatalf("accepted session ID %q", id)
		}
	}
	j := openTest(t, filepath.Join(dir, "nested", "root"))
	for _, path := range []string{filepath.Join(dir, "nested"), filepath.Join(dir, "nested", "root"), filepath.Join(dir, "nested", "root", "session")} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0700 {
			t.Fatalf("directory %s mode %v", path, info.Mode())
		}
	}
	for _, name := range []string{".lock", "journal.jsonl"} {
		info, err := os.Stat(filepath.Join(dir, "nested", "root", "session", name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("file mode %v", info.Mode())
		}
	}
	_ = j.Close()
	unsafe := filepath.Join(dir, "unsafe")
	if err := os.Mkdir(unsafe, 0755); err != nil {
		t.Fatal(err)
	}
	if j, err := Open(unsafe, "session"); err == nil {
		_ = j.Close()
		t.Fatal("accepted non-private storage directory")
	}
}

func TestRejectSymlinksAndHardlinks(t *testing.T) {
	for _, where := range []string{"root", "ancestor", "session", ".lock", "journal.jsonl", "hardlink-lock", "hardlink-journal"} {
		t.Run(where, func(t *testing.T) {
			dir := tempDirectory(t)
			target := filepath.Join(dir, "target")
			root := filepath.Join(dir, "root")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			if where == "root" {
				if err := os.Symlink(target, root); err != nil {
					t.Fatal(err)
				}
			} else if where == "ancestor" {
				if err := os.Symlink(target, root); err != nil {
					t.Fatal(err)
				}
				root = filepath.Join(root, "nested")
			} else {
				if err := os.Mkdir(root, 0700); err != nil {
					t.Fatal(err)
				}
				sessionDir := filepath.Join(root, "session")
				if where == "session" {
					if err := os.Symlink(target, sessionDir); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(sessionDir, 0700); err != nil {
						t.Fatal(err)
					}
					source := filepath.Join(target, "file")
					if err := os.WriteFile(source, []byte("protected"), 0600); err != nil {
						t.Fatal(err)
					}
					name := where
					if strings.HasPrefix(where, "hardlink-") {
						name = ".lock"
						if where == "hardlink-journal" {
							name = "journal.jsonl"
						}
						if err := os.Link(source, filepath.Join(sessionDir, name)); err != nil {
							t.Fatal(err)
						}
					} else if err := os.Symlink(source, filepath.Join(sessionDir, name)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if j, err := Open(root, "session"); err == nil {
				_ = j.Close()
				t.Fatal("accepted unsafe path")
			}
			if source, err := os.ReadFile(filepath.Join(target, "file")); err == nil && string(source) != "protected" {
				t.Fatal("modified symlink/hardlink target")
			}
		})
	}
}
