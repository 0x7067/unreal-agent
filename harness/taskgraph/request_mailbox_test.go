package taskgraph

import (
	"context"
	"encoding/json/v2"
	"errors"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func TestRuntimeMailboxUnboundedFIFO(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		q := newRuntimeRequests(ctx)
		const count = 4096
		got := -1
		// No execution consumer exists yet. Every admission must complete.
		for i := range count {
			if err := q.submit(func(*runtimeState) { got = i }); err != nil {
				t.Fatal(err)
			}
		}
		for i := range count {
			fn := <-q.output
			fn(nil)
			if got != i {
				t.Fatalf("request %d executed at FIFO position %d", got, i)
			}
		}
		cancel()
		<-q.done
	})
}

func TestRuntimeMailboxAddCancelOrder(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		defer unblock()
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base,
			NewAgentHandlers: func(context.Context) ([]operation.RemoteJobHandler, error) {
				close(entered)
				<-release
				// No providers: the agent child will be unsupported by the
				// nested manager, which still exercises real cancel/drain.
				return nil, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		owner := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Prompt: "fake", Reads: []string{}, Writes: []string{}, Acceptance: "check"}}})
		if err := r.run.AddRemoteJob(owner); err != nil {
			t.Fatal(err)
		}
		<-entered
		for _, id := range []string{"before", "after"} {
			if id == "after" {
				if err := r.run.CancelRemoteJob(owner.ID, "test FIFO"); err != nil {
					t.Fatal(err)
				}
			}
			if err := r.control.AddRemoteJob(runtimeOperation(t, id, ControlPlanType, ControlPlan{Name: "g", Action: "status"})); err != nil {
				t.Fatal(err)
			}
		}
		// Both handlers share a queue, and none of their admissions waits for
		// this factory. The two status snapshots must bracket the cancellation.
		unblock()
		for _, id := range []operation.ID{"before", "after"} {
			op := <-r.control.updates
			if op.ID != id || op.Status != operation.StatusCompleted {
				t.Fatalf("control update = %s/%s, want %s/completed", op.ID, op.Status, id)
			}
			state, err := operation.DecodeRemoteJobState(op)
			if err != nil {
				t.Fatal(err)
			}
			var cp checkpoint
			if err := json.Unmarshal(state.Handle, &cp); err != nil {
				t.Fatal(err)
			}
			if cp.Canceling != (id == "after") {
				t.Fatalf("%s snapshot canceling = %t; Add/Cancel reordered", id, cp.Canceling)
			}
		}
		cancel()
		for range r.run.updates {
		}
		for range r.control.updates {
		}
		<-r.requests.done
	})
}

func TestRuntimeMailboxCancellationStopsAdmissionAndDropsPending(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base})
		if err != nil {
			t.Fatal(err)
		}
		entered, release := make(chan struct{}), make(chan struct{})
		var once sync.Once
		unblock := func() { once.Do(func() { close(release) }) }
		defer unblock()
		if err := r.request(func(*runtimeState) { close(entered); <-release }); err != nil {
			t.Fatal(err)
		}
		<-entered
		executed := false
		if err := r.request(func(*runtimeState) { executed = true }); err != nil {
			t.Fatal(err)
		}
		cancel()
		// Mailbox stops even while runtime execution remains blocked.
		<-r.requests.done
		for range 100 {
			for _, h := range []*runtimeHandler{r.run, r.control} {
				if err := h.AddRemoteJob(operation.Operation{}); !errors.Is(err, context.Canceled) {
					t.Fatalf("Add after cancellation = %v", err)
				}
				if err := h.CancelRemoteJob("id", "reason"); !errors.Is(err, context.Canceled) {
					t.Fatalf("Cancel after cancellation = %v", err)
				}
			}
		}
		unblock()
		for range r.run.updates {
		}
		for range r.control.updates {
		}
		if executed {
			t.Fatal("pending request executed after shutdown")
		}
	})
}

func TestRuntimeMailboxAlreadyCanceled(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		q := newRuntimeRequests(ctx)
		if err := q.submit(func(*runtimeState) { t.Fatal("executed") }); !errors.Is(err, context.Canceled) {
			t.Fatalf("request with canceled context = %v", err)
		}
		<-q.done
	})
}
