package subagent

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// These tests don't emit session messages, so no inbox delivery is expected.
// Embedding the interface makes any unexpected delivery fail rather than hide it.
type lifecycleInbox struct{ inbox.Writer }

func TestSupervisorWaitDoesNotCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := s.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait on live supervisor = %v", err)
		}
		barrier := make(chan struct{})
		if err := s.do(func(*state) { close(barrier) }); err != nil {
			t.Fatal(err)
		}
		<-barrier
		cancel()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		for _, h := range s.Handlers() {
			if err := h.(interface{ Wait(context.Context) error }).Wait(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, open := <-h.RemoteJobUpdates(); open {
				t.Fatal("Wait returned before update channel closed")
			}
		}
	})
}

const lifecycleHelperEnv = "UNREAL_SUBAGENT_LIFECYCLE_HELPER"

// Re-executed by the integration test. Observe cancellation but deliberately
// remain alive until the test releases us, proving actor exit is not a join.
func TestSupervisorLifecycleChild(t *testing.T) {
	dir := os.Getenv(lifecycleHelperEnv)
	if dir == "" {
		return
	}
	interrupted := make(chan os.Signal, 1)
	signal.Notify(interrupted, os.Interrupt)
	defer signal.Stop(interrupted)
	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ready"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	<-interrupted
	if err := os.WriteFile(filepath.Join(dir, "canceled"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	// EOF must not be mistaken for process exit: the reader now reaches
	// command.Wait while this child remains gated and alive.
	_ = os.Stdout.Close()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("helper never released")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func lifecycleFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", path)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSupervisorWaitJoinsChildReadersAndCommandWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exec interrupt cancellation requires Unix signals")
	}
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	s, err := New(ctx, Config{
		Command: []string{os.Args[0], "-test.run=^TestSupervisorLifecycleChild$", "--"},
		Env:     append(os.Environ(), lifecycleHelperEnv+"="+dir),
		Inbox:   lifecycleInbox{},
		Stderr:  os.Stderr,
	})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release"), nil, 0600)
		cancel()
		waitCtx, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := s.Wait(waitCtx); err != nil {
			t.Errorf("supervisor cleanup: %v", err)
		}
	})
	data, err := json.Marshal(toolsubagent.RunPlan{Name: "gated", Prompt: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "gated-child", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}

	// Concurrent Wait calls begin before child admission. A Wait implementation
	// that directly waits on a currently-empty reader WaitGroup returns early.
	const waiters = 8
	results := make(chan error, waiters)
	var started sync.WaitGroup
	started.Add(waiters)
	for range waiters {
		go func() {
			started.Done()
			results <- s.Wait(t.Context())
		}()
	}
	started.Wait()
	if err := s.run.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	lifecycleFile(t, filepath.Join(dir, "ready"))
	cancel()
	lifecycleFile(t, filepath.Join(dir, "canceled"))
	waitCtx, stop := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer stop()
	if err := s.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait while canceled child is gated = %v", err)
	}
	select {
	case err := <-results:
		t.Fatalf("concurrent Wait returned before child exit: %v", err)
	default:
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for range waiters {
		select {
		case err := <-results:
			if err != nil {
				t.Fatal(err)
			}
		case <-deadline.C:
			t.Fatal("Wait did not join released child")
		}
	}
	for _, h := range s.Handlers() {
		if _, open := <-h.RemoteJobUpdates(); open {
			t.Fatal("updates still open after Wait")
		}
	}
}

func TestSupervisorWaitJoinsExecutingActorCallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
		if err != nil {
			t.Fatal(err)
		}
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		defer unblock()
		if err := s.do(func(*state) { close(entered); <-release }); err != nil {
			t.Fatal(err)
		}
		<-entered
		cancel()
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := s.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait while supervisor actor callback is gated = %v", err)
		}
		unblock()
		if err := s.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}
