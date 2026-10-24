package taskgraph

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func TestRuntimeWaitJoinsExecutingCallbackAndMailbox(t *testing.T) {
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
		cancel()
		<-r.requests.done
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := r.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait with executing cancellation-ignoring callback = %v", err)
		}
		unblock()
		if err := r.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		for _, updates := range []<-chan operation.Operation{r.run.updates, r.control.updates} {
			if _, open := <-updates; open {
				t.Fatal("Wait returned before runtime update channel closed")
			}
		}
	})
}

// Completion of a remote operation is not completion of its handler's actor.
// This fake publishes its result immediately, but deliberately delays cleanup.
type lifecycleHandler struct {
	updates  chan operation.Operation
	canceled chan struct{}
	release  chan struct{}
	done     chan struct{}
}

func newLifecycleHandler(ctx context.Context) *lifecycleHandler {
	h := &lifecycleHandler{updates: make(chan operation.Operation, 1), canceled: make(chan struct{}), release: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(h.done)
		defer close(h.updates)
		<-ctx.Done()
		close(h.canceled)
		<-h.release
	}()
	return h
}
func (*lifecycleHandler) RemoteJobPlanType() operation.RemoteJobPlanType {
	return toolsubagent.RunPlanType
}
func (*lifecycleHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.PlanVersion
}
func (h *lifecycleHandler) RemoteJobUpdates() <-chan operation.Operation { return h.updates }
func (h *lifecycleHandler) AddRemoteJob(op operation.Operation) error {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	step, err := operation.UpdateRemoteJob(op, state, operation.StatusCompleted)
	if err == nil {
		h.updates <- *step.Operation
	}
	return err
}
func (*lifecycleHandler) CancelRemoteJob(operation.ID, string) error { return nil }
func (h *lifecycleHandler) Wait(ctx context.Context) error {
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func lifecycleUpdate(t *testing.T, r *Runtime, predicate func(operation.Operation) bool) operation.Operation {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case op := <-r.run.updates:
			if predicate(op) {
				return op
			}
		case op := <-r.control.updates:
			if predicate(op) {
				return op
			}
		case <-deadline.C:
			t.Fatal("timed out waiting for lifecycle update")
		}
	}
}

func TestRuntimeWaitJoinsAllCompletedGraphExecutions(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		created := make(chan *lifecycleHandler, 2)
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base,
			NewAgentHandlers: func(ctx context.Context) ([]operation.RemoteJobHandler, error) {
				h := newLifecycleHandler(ctx)
				created <- h
				return []operation.RemoteJobHandler{h}, nil
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		var handlers []*lifecycleHandler
		defer func() {
			for _, h := range handlers {
				select {
				case <-h.release:
				default:
					close(h.release)
				}
			}
		}()
		for _, name := range []string{"first", "second"} {
			op := runtimeOperation(t, name, RunPlanType, RunPlan{Name: name, Tasks: []Task{{ID: "a", Prompt: "fake", Reads: []string{}, Writes: []string{}, Acceptance: "check"}}})
			if err := r.run.AddRemoteJob(op); err != nil {
				t.Fatal(err)
			}
			h := <-created
			handlers = append(handlers, h)
			lifecycleUpdate(t, r, func(op operation.Operation) bool {
				graph := runtimeGraph(t, op)
				return op.ID == operation.ID(name) && graph.Find("a").Status == Completed
			})
			if err := r.control.AddRemoteJob(runtimeOperation(t, "accept-"+name, ControlPlanType, ControlPlan{Name: name, Action: "accept", TaskID: "a", Generation: 1, Evidence: "verified"})); err != nil {
				t.Fatal(err)
			}
			lifecycleUpdate(t, r, func(op operation.Operation) bool {
				return op.ID == operation.ID(name) && op.Status == operation.StatusCompleted
			})
			<-h.canceled // A terminal graph stops its manager's derived context.
		}
		var managers []operation.Manager
		barrier := make(chan struct{})
		if err := r.request(func(s *runtimeState) {
			if s.active != nil {
				t.Error("completed graphs left active execution")
			}
			for _, g := range s.graphs {
				managers = append(managers, g.manager)
			}
			close(barrier)
		}); err != nil {
			t.Fatal(err)
		}
		<-barrier
		cancel()
		for _, h := range handlers {
			waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
			if err := r.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Wait before all completed graphs' handler cleanup = %v", err)
			}
			stop()
			close(h.release)
			<-h.done
		}
		if err := r.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		for _, m := range managers {
			if _, open := <-m.Updates(); open {
				t.Fatal("nested manager not drained at runtime completion")
			}
		}
		select {
		case <-r.requests.done:
		default:
			t.Fatal("runtime Wait did not join mailbox actor")
		}
	})
}

func TestRuntimeWaitDoesNotCancel(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base})
		if err != nil {
			t.Fatal(err)
		}
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := r.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait on live runtime = %v", err)
		}
		if ctx.Err() != nil {
			t.Fatal("Wait canceled runtime")
		}
		cancel()
		if err := r.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

// A terminal execution can leave an operation manager draining asynchronous
// cleanup. Updates may still need a consumer after that execution is inactive.
type lifecycleManager struct {
	updates chan operation.Operation
	stopped chan struct{}
	release chan struct{}
}

func (*lifecycleManager) Add(operation.Operation) error         { return nil }
func (*lifecycleManager) Cancel(operation.ID, string) error     { return nil }
func (m *lifecycleManager) Updates() <-chan operation.Operation { return m.updates }

func TestRuntimeWaitDrainsInactiveManagersWithoutExternalConsumer(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base})
		if err != nil {
			t.Fatal(err)
		}
		var managers []*lifecycleManager
		for range 2 {
			m := &lifecycleManager{updates: make(chan operation.Operation), stopped: make(chan struct{}), release: make(chan struct{})}
			managers = append(managers, m)
			go func() {
				<-m.stopped
				<-m.release
				m.updates <- operation.Operation{} // Runtime must drain this itself.
				close(m.updates)
			}()
		}
		defer func() {
			for _, m := range managers {
				select {
				case <-m.release:
				default:
					close(m.release)
				}
			}
		}()
		barrier := make(chan struct{})
		if err := r.request(func(s *runtimeState) {
			for i, name := range []string{"first", "second"} {
				m := managers[i]
				s.graphs[name] = &graphExecution{manager: m, terminal: true, stop: func() { close(m.stopped) }}
			}
			close(barrier)
		}); err != nil {
			t.Fatal(err)
		}
		<-barrier
		cancel()
		// Shutdown must cancel all managers before blocking to join any one.
		for _, m := range managers {
			<-m.stopped
		}
		for _, m := range managers {
			waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
			if err := r.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Wait before inactive manager drained = %v", err)
			}
			stop()
			close(m.release)
		}
		if err := r.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

// A factory can start a handler before discovering a configuration error. No
// graph execution or local manager is installed in that case, but the runtime
// still owns the returned handler's cleanup obligation.
func TestRuntimeWaitJoinsHandlersFromFailedSetup(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		created := make(chan *lifecycleHandler, 1)
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base,
			NewAgentHandlers: func(ctx context.Context) ([]operation.RemoteJobHandler, error) {
				h := newLifecycleHandler(ctx)
				created <- h
				return []operation.RemoteJobHandler{h}, errors.New("setup failed after handler start")
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		op := runtimeOperation(t, "failed-setup", RunPlanType, RunPlan{Name: "failed-setup", Tasks: []Task{{ID: "a", Prompt: "fake", Reads: []string{}, Writes: []string{}, Acceptance: "check"}}})
		if err := r.run.AddRemoteJob(op); err != nil {
			t.Fatal(err)
		}
		h := <-created
		var release sync.Once
		unblock := func() { release.Do(func() { close(h.release) }) }
		defer unblock()
		lifecycleUpdate(t, r, func(op operation.Operation) bool {
			return op.ID == "failed-setup" && op.Status == operation.StatusFailed
		})
		<-h.canceled
		cancel()
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := r.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Wait before failed setup handler cleanup = %v", err)
		}
		unblock()
		if err := r.Wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		select {
		case <-h.done:
		default:
			t.Fatal("Wait returned before failed setup handler exited")
		}
	})
}

// A manager actor may be inside a remote handler call that cannot finish until
// its nested context is canceled. A synchronous Cancel must not prevent the
// runtime from delivering that cancellation during shutdown.
type lifecycleCancelManager struct {
	*lifecycleManager
	canceled chan struct{}
}

func (m *lifecycleCancelManager) Cancel(operation.ID, string) error {
	<-m.stopped
	close(m.canceled)
	return context.Canceled
}

func TestRuntimeShutdownStopsNestedContextBeforeCancel(t *testing.T) {
	workspace, base := t.TempDir(), t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: base})
		if err != nil {
			t.Fatal(err)
		}
		m := &lifecycleCancelManager{
			lifecycleManager: &lifecycleManager{updates: make(chan operation.Operation), stopped: make(chan struct{})},
			canceled:         make(chan struct{}),
		}
		var once sync.Once
		stopManager := func() { once.Do(func() { close(m.stopped) }) }
		defer stopManager()
		go func() {
			<-m.stopped
			close(m.updates)
		}()
		barrier := make(chan struct{})
		if err := r.request(func(s *runtimeState) {
			g := &graphExecution{
				checkpoint: checkpoint{
					Graph:    Graph{Nodes: []Node{{Task: Task{ID: "a"}, Status: Running, Generation: 1}}},
					Attempts: map[string]*attempt{"a": {Generation: 1, ChildOperation: operation.Operation{ID: "child"}}},
				},
				manager: m,
				stop:    stopManager,
			}
			s.graphs["active"] = g
			s.active = g
			close(barrier)
		}); err != nil {
			t.Fatal(err)
		}
		<-barrier
		cancel()
		waitCtx, stop := context.WithTimeout(t.Context(), time.Second)
		defer stop()
		if err := r.Wait(waitCtx); err != nil {
			t.Fatalf("shutdown blocked on Cancel before stopping nested context: %v", err)
		}
		select {
		case <-m.canceled:
		default:
			t.Fatal("active child cancellation was not attempted")
		}
	})
}
