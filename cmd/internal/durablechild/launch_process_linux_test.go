//go:build linux

package durablechild_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablechild"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"golang.org/x/sys/unix"
)

func TestRealOwnerLaunchSurvivesParentProcessGroupKill(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	root := t.TempDir()
	mustFixture(os.Chmod(root, 0700))
	runtimeDir, err := os.MkdirTemp("/tmp", "u26-launch-")
	mustFixture(err)
	defer os.RemoveAll(runtimeDir)
	t.Setenv("HOME", root)
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	exe, err := os.Executable()
	mustFixture(err)
	entered, release, joined := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			close(entered)
			defer close(joined)
		}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		completed(w, "independent launch answer")
	}))
	defer server.Close()
	f := launchFixture{Config: durablechild.Config{Directory: root, RuntimeDirectory: runtimeDir, Workspace: root, Provider: "openai", BaseURL: server.URL, Model: "fake", Command: []string{exe}}, Request: durablechild.Request{ParentID: "owned-launch-parent", OperationID: "owned-launch-operation", Name: "owned", Prompt: "one launch", Lifetime: "background"}, Ready: filepath.Join(root, "ready.json")}
	data, err := json.Marshal(f)
	mustFixture(err)
	path := filepath.Join(root, "fixture.json")
	mustFixture(os.WriteFile(path, data, 0600))
	parent := exec.Command(exe, "m26-launch-owner", path)
	parent.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	parent.Stderr = os.Stderr
	mustFixture(parent.Start())
	waited := false
	defer func() {
		if !waited {
			_ = syscall.Kill(-parent.Process.Pid, syscall.SIGKILL)
			_ = parent.Wait()
		}
	}()
	var ready struct {
		Link    durablechild.Link
		HostPID int
	}
	waitUntil(t, ctx, func() bool { b, e := os.ReadFile(f.Ready); return e == nil && json.Unmarshal(b, &ready) == nil })
	fd, err := unix.PidfdOpen(ready.HostPID, 0)
	mustFixture(err)
	defer unix.Close(fd)
	// The pidfd observes only this authenticated child incarnation; no saved PID
	// is used as execution/cancellation authority, and no original Cmd.Wait is claimed.
	var child *durablechild.Connection
	defer func() {
		unblock()
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if child == nil {
			child, _ = durablechild.Open(cleanup, f.Config, f.Request, false)
		}
		if child == nil {
			t.Error("independent child cleanup unavailable")
			return
		}
		if err := child.Shutdown(cleanup); err != nil {
			t.Errorf("shutdown admission: %v", err)
		}
		p := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, e := unix.Poll(p, 8000)
		if e != nil || n != 1 || p[0].Revents&unix.POLLIN == 0 {
			t.Errorf("specific independent host exit unconfirmed: %d %v", n, e)
		}
		_ = child.Close()
		s, e := localfile.New(ready.Link.Directory)
		if e != nil {
			t.Error(e)
			return
		}
		h, e := sessionstore.ReadLifecycle(cleanup, s, ready.Link.ChildID)
		if e != nil || h.LastDrainSequence == 0 {
			t.Errorf("child effect cleanup unproven: %+v %v", h, e)
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	mustFixture(syscall.Kill(-parent.Process.Pid, syscall.SIGKILL))
	if err := parent.Wait(); err == nil {
		t.Fatal("parent did not get killed")
	}
	waited = true
	select {
	case <-joined:
		t.Fatal("child request canceled with parent process group")
	default:
	}
	child, err = durablechild.Open(ctx, f.Config, f.Request, false)
	if err != nil {
		t.Fatal(err)
	}
	if child.Link() != ready.Link || child.ClientSnapshot().Hello.HostPID != ready.HostPID {
		t.Fatal("child replaced after parent loss")
	}
	if err := child.SubmitInitial(ctx); err != nil {
		t.Fatal(err)
	}
	unblock()
	waitUntil(t, ctx, func() bool {
		a, found, e := child.Answer(ctx)
		if e != nil {
			t.Fatal(e)
		}
		return found && a.Text == "independent launch answer"
	})
	if calls.Load() != 1 {
		t.Fatalf("duplicate child execution: %d", calls.Load())
	}
}
