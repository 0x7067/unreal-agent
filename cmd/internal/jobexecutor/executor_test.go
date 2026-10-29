package jobexecutor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestLostSubmitResponseReattachAndAuthority(t *testing.T) {
	// macOS limits Unix socket paths to 104 bytes; t.TempDir() exceeds it there.
	root, err := testtmp.MkdirTemp("je-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	count, gate := filepath.Join(root, "count"), filepath.Join(root, "gate")
	token := "private-test-token-not-a-checkpoint-secret"
	var drops atomic.Int32
	cfg := Config{Directory: filepath.Join(root, "executor"), EnvironmentID: "test-env", Token: token,
		Jobs:               map[string]TrustedJob{"fixture": {Path: "/bin/sh", Args: []string{"-c", `printf 'launch\n' >> "$1"; printf first; while [ ! -f "$2" ]; do sleep 0.01; done; printf second; exit 7`, "fixture", count, gate}}},
		DropSubmitResponse: func(Handle) bool { return drops.Add(1) == 1 }}
	s, err := Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			if err := s.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	c := NewClient(filepath.Join(cfg.Directory, "executor.sock"), token)
	p := Plan{Version: Version, OperationID: "op", JobID: "job", AttemptID: "attempt", ControllerID: "controller", EnvironmentID: cfg.EnvironmentID, JobName: "fixture"}
	snap, err := c.Submit(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if drops.Load() != 2 {
		t.Fatalf("submit response-loss retry count = %d", drops.Load())
	}
	h := snap.Handle
	c.Close() // A new independent client reattaches; client loss must not stop work.
	c = NewClient(filepath.Join(cfg.Directory, "executor.sock"), token)
	defer c.Close()
	poll := func(want string, offset int) Snapshot {
		t.Helper()
		for {
			v, e := c.Inspect(ctx, h, offset)
			if e != nil {
				t.Fatal(e)
			}
			if string(v.Output) == want {
				return v
			}
			select {
			case <-ctx.Done():
				t.Fatalf("waiting for %q: %+v", want, v)
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	first := poll("first", 0)
	if first.State != "running" || first.Handle != h {
		t.Fatalf("not live same handle: %+v", first)
	}
	for name, mutate := range map[string]func(*Handle){"attempt": func(h *Handle) { h.AttemptID = "stale" }, "controller": func(h *Handle) { h.ControllerID = "stale" }, "environment": func(h *Handle) { h.EnvironmentID = "stale" }, "schema": func(h *Handle) { h.Version++ }, "fingerprint": func(h *Handle) { h.Fingerprint = "stale" }} {
		t.Run(name, func(t *testing.T) {
			bad := h
			mutate(&bad)
			if _, e := c.Inspect(ctx, bad, 0); e == nil {
				t.Fatal("inspect accepted stale authority")
			}
			if _, e := c.Cancel(ctx, bad); e == nil {
				t.Fatal("cancel accepted stale authority")
			}
		})
	}
	for _, change := range []func(*Plan){func(p *Plan) { p.AttemptID = "new" }, func(p *Plan) { p.Version++ }, func(p *Plan) { p.ControllerID = "other" }, func(p *Plan) { p.JobName = "arbitrary-command" }} {
		bad := p
		change(&bad)
		if _, e := c.Submit(ctx, bad); e == nil {
			t.Fatal("submit accepted identity/schema conflict")
		}
	}
	if _, e := c.Inspect(ctx, h, -1); e == nil {
		t.Fatal("negative offset accepted")
	}
	if err := os.WriteFile(gate, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var terminal Snapshot
	for {
		terminal, err = c.Inspect(ctx, h, first.Offset)
		if err != nil {
			t.Fatal(err)
		}
		if terminal.State == "failed" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("no terminal wait: %+v", terminal)
		case <-time.After(5 * time.Millisecond):
		}
	}
	if string(terminal.Output) != "second" || terminal.Offset != len("firstsecond") || terminal.ExitCode != 7 {
		t.Fatalf("terminal did not reflect real Wait/incremental output: %+v", terminal)
	}
	again, e := c.Submit(ctx, p)
	if e != nil || again.Handle != h || again.State != "failed" {
		t.Fatalf("duplicate terminal submit: %+v %v", again, e)
	}
	launches, e := os.ReadFile(count)
	if e != nil || string(launches) != "launch\n" {
		t.Fatalf("executed other than once: %q %v", launches, e)
	}
	entries, e := os.ReadDir(cfg.Directory)
	if e != nil {
		t.Fatal(e)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			b, e := os.ReadFile(filepath.Join(cfg.Directory, entry.Name()))
			if e != nil {
				t.Fatal(e)
			}
			if strings.Contains(string(b), token) || strings.Contains(string(b), "/bin/sh") {
				t.Fatal("credential or trusted command leaked into checkpoint")
			}
		}
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	closed = true
	if _, e = c.Inspect(ctx, h, 0); e == nil || !strings.Contains(e.Error(), "executor unavailable") {
		t.Fatalf("unavailability not explicit: %v", e)
	}
	if _, e = c.Submit(ctx, p); e == nil || !strings.Contains(e.Error(), "executor unavailable") {
		t.Fatalf("submit fell back: %v", e)
	}
	launches, e = os.ReadFile(count)
	if e != nil || string(launches) != "launch\n" {
		t.Fatalf("unavailable executor launched fallback: %q %v", launches, e)
	}
}

func TestCloseJoinsLiveChildAndService(t *testing.T) {
	// macOS limits Unix socket paths to 104 bytes; t.TempDir() exceeds it there.
	root, err := testtmp.MkdirTemp("je-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Start(Config{Directory: filepath.Join(root, "executor"), EnvironmentID: "env", Token: strings.Repeat("s", 24), Jobs: map[string]TrustedJob{"wait": {Path: "/bin/sh", Args: []string{"-c", "printf ready; exec sleep 60"}}}})
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(filepath.Join(root, "executor", "executor.sock"), strings.Repeat("s", 24))
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	v, err := c.Submit(ctx, Plan{Version: Version, OperationID: "op", JobID: "job", AttemptID: "a", ControllerID: "c", EnvironmentID: "env", JobName: "wait"})
	if err != nil {
		_ = s.Close()
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	a := s.jobs[v.Handle.JobID]
	select {
	case <-a.done:
	default:
		t.Fatal("child wait not joined")
	}
	if a.cmd.ProcessState == nil || a.record.State != "canceled" {
		t.Fatalf("child not reaped: %+v", a.record)
	}
	select {
	case <-s.serveDone:
	default:
		t.Fatal("HTTP listener not joined")
	}
}
