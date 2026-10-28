package operation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"encoding/json/jsontext"
	"fmt"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func awaitStoppedForTest(t *testing.T, m *LocalOperationManager) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	return m.AwaitStopped(ctx)
}

func TestAwaitStoppedLocalShellConcurrentAndAfterExit(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Unused registered remote handlers must not poison local-only proof.
	m := NewLocalOperationManager(ctx, lifecycleNewTestRemoteJobHandler())
	dir := t.TempDir()
	t.Setenv("LIFECYCLE_PID", filepath.Join(dir, "pid"))
	t.Setenv("LIFECYCLE_STOPPED", filepath.Join(dir, "stopped"))
	current := lifecycleNewShellOperation(t, "lifecycle-shell", ShellInput{
		Shell:   lifecycleTestShellPath,
		Command: `trap 'printf stopped > "$LIFECYCLE_STOPPED"; exit 0' TERM; echo $$ > "$LIFECYCLE_PID"; while :; do sleep 0.05; done`,
	}, t.TempDir(), 64)
	if err := m.Add(current); err != nil {
		t.Fatal(err)
	}
	// ProcessStarted observation establishes a real primitive, not just admission.
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(filepath.Join(dir, "pid"))
		if err == nil && strings.TrimSpace(string(data)) != "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("shell did not start")
		}
		time.Sleep(time.Millisecond)
	}
	data, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if err := awaitStoppedForTest(t, m); err != nil {
				t.Errorf("await: %v", err)
			}
		})
	}
	wg.Wait()
	if err := awaitStoppedForTest(t, m); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("shell still exists after stop: %v", err)
	}
	lifecycleWaitForFileContents(t, filepath.Join(dir, "stopped"), "stopped")
	// No Updates consumer was needed for primitive cleanup or actor return.
	if _, open := <-m.Updates(); open {
		t.Fatal("updates still open after actor returned")
	}
}

func TestAwaitStoppedCanceledWaitDoesNotCancelManager(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := NewLocalOperationManager(ctx)
	waitCtx, stopWaiting := context.WithCancel(t.Context())
	stopWaiting()
	if err := m.AwaitStopped(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error: %v", err)
	}
	current := lifecycleNewShellOperation(t, "after-canceled-wait", ShellInput{Shell: lifecycleTestShellPath, Command: "printf alive"}, t.TempDir(), 64)
	if err := m.Add(current); err != nil {
		t.Fatal(err)
	}
	if terminal := lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID); terminal.Status != StatusCompleted {
		t.Fatalf("operation: %#v", terminal)
	}
	cancel()
	if err := awaitStoppedForTest(t, m); err != nil {
		t.Fatal(err)
	}
}

type lifecycleBlockedForwarder struct {
	*lifecycleTestRemoteJobHandler
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (h *lifecycleBlockedForwarder) RemoteJobUpdates() <-chan Operation {
	h.once.Do(func() { close(h.entered) })
	<-h.release
	return h.lifecycleTestRemoteJobHandler.updates
}

func TestAwaitStoppedWaitsForForwardingWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h := &lifecycleBlockedForwarder{lifecycleTestRemoteJobHandler: lifecycleNewTestRemoteJobHandler(), entered: make(chan struct{}), release: make(chan struct{})}
	unblock := sync.OnceFunc(func() { close(h.release) })
	defer unblock()
	m := NewLocalOperationManager(ctx, h)
	<-h.entered
	cancel()
	waitCtx, end := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer end()
	if err := m.AwaitStopped(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("worker not joined: %v", err)
	}
	unblock()
	if err := awaitStoppedForTest(t, m); err != nil {
		t.Fatal(err)
	}
}

func TestAwaitStoppedRemoteCleanupEvidence(t *testing.T) {
	for _, scenario := range []string{"undrained", "terminal", "unknown", "unsupported-terminal", "admission-error"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			h := lifecycleNewTestRemoteJobHandler()
			if scenario == "admission-error" {
				h.addErr = errors.New("admission outcome unknown")
			}
			m := NewLocalOperationManager(ctx, h)
			current := lifecycleNewTestRemoteJobOperation(t, "remote-evidence")
			if err := m.Add(current); err != nil {
				t.Fatal(err)
			}
			if scenario != "admission-error" {
				<-h.adds
			}
			switch scenario {
			case "terminal":
				state, err := DecodeRemoteJobState(current)
				if err != nil {
					t.Fatal(err)
				}
				state.TerminalResult = `{"content":[]}`
				step, err := UpdateRemoteJob(current, state, StatusCompleted)
				if err != nil {
					t.Fatal(err)
				}
				h.updates <- *step.Operation
				lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID)
			case "unknown":
				close(h.updates)
				lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID)
			case "unsupported-terminal":
				invalid := current
				invalid.Status = StatusUnsupported
				h.updates <- invalid
				lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID)
			case "admission-error":
				lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID)
			}
			cancel()
			err := awaitStoppedForTest(t, m)
			var evidence *CleanupUncertainError
			if !errors.Is(err, ErrCleanupUncertain) || !errors.As(err, &evidence) {
				t.Fatalf("cleanup error: %v", err)
			}
			if !slices.Equal(evidence.UnprovenRemoteIDs, []ID{current.ID}) {
				t.Fatalf("remote evidence: %#v", evidence)
			}
			wantUnknown := scenario == "unknown" || scenario == "unsupported-terminal"
			if slices.Contains(evidence.UnknownRemoteIDs, current.ID) != wantUnknown {
				t.Fatalf("unknown evidence: %#v", evidence)
			}
			evidence.UnprovenRemoteIDs[0] = "mutated"
			var fresh *CleanupUncertainError
			if !errors.As(awaitStoppedForTest(t, m), &fresh) || fresh.UnprovenRemoteIDs[0] != current.ID {
				t.Fatal("caller mutated retained evidence")
			}
		})
	}
}

func TestAwaitStoppedUnsupportedIsNotCleanupProof(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := NewLocalOperationManager(ctx)
	current := Operation{ID: "unsupported", Type: "future", Status: StatusReady}
	if err := m.Add(current); err != nil {
		t.Fatal(err)
	}
	select {
	case update := <-m.Updates():
		if update.Status != StatusUnsupported {
			t.Fatalf("status: %v", update.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no unsupported update")
	}
	cancel()
	var evidence *CleanupUncertainError
	if err := awaitStoppedForTest(t, m); !errors.As(err, &evidence) || !slices.Equal(evidence.UnsupportedOperationIDs, []ID{current.ID}) {
		t.Fatalf("unsupported cleanup evidence: %v", err)
	}
}

const lifecycleTestShellPath = "/bin/sh"

func lifecycleNewShellOperation(t *testing.T, id ID, input ShellInput, base string, limit int) Operation {
	t.Helper()
	spec, err := NewShellSpec(input, base, limit)
	if err != nil {
		t.Fatal(err)
	}
	return Operation{ID: id, Type: spec.Type, Version: spec.Version, Status: StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
}
func lifecycleReceiveTerminalOperation(t *testing.T, updates <-chan Operation, id ID) Operation {
	t.Helper()
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case update, open := <-updates:
			if !open {
				t.Fatal("updates closed before terminal")
			}
			if update.ID == id && localOperationFinished(update.Status) {
				return update
			}
		case <-timeout.C:
			t.Fatal("no terminal update")
		}
	}
}
func lifecycleWaitForFileContents(t *testing.T, path, want string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		data, err := os.ReadFile(path)
		if err == nil && string(data) == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("file %q: data %q error %v", path, data, err)
		}
		time.Sleep(time.Millisecond)
	}
}

type lifecycleTestRemoteJobHandler struct {
	adds    chan Operation
	updates chan Operation
	addErr  error
}

func lifecycleNewTestRemoteJobHandler() *lifecycleTestRemoteJobHandler {
	return &lifecycleTestRemoteJobHandler{adds: make(chan Operation, 1), updates: make(chan Operation, 1)}
}
func (*lifecycleTestRemoteJobHandler) RemoteJobPlanType() RemoteJobPlanType       { return "test" }
func (*lifecycleTestRemoteJobHandler) RemoteJobPlanVersion() RemoteJobPlanVersion { return 1 }
func (h *lifecycleTestRemoteJobHandler) AddRemoteJob(op Operation) error {
	if h.addErr != nil {
		return h.addErr
	}
	h.adds <- op
	return nil
}
func (*lifecycleTestRemoteJobHandler) CancelRemoteJob(ID, string) error     { return nil }
func (h *lifecycleTestRemoteJobHandler) RemoteJobUpdates() <-chan Operation { return h.updates }
func lifecycleNewTestRemoteJobOperation(t *testing.T, id ID) Operation {
	t.Helper()
	spec, err := NewRemoteJobSpec(RemoteJobPlan{Type: "test", Version: 1, Data: jsontext.Value(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	return Operation{ID: id, Type: spec.Type, Version: spec.Version, Status: StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
}

func TestLocalCleanupProofTerminalBeforeCallbackDrain(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := &LocalOperationManager{primitiveEvents: make(chan primitives.PrimitiveEvent)}
	current := &localRunningOperation{ctx: ctx, cancel: cancel, remoteJobHandlerIndex: -1, operation: Operation{ID: "blocked", Type: TypeShell, Version: VersionShell, Status: StatusAwaiting}}
	operations := map[ID]*localRunningOperation{current.operation.ID: current}
	started, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	active := m.acceptLocalStep(operations, current, Step{Dispatches: []PrimitiveDispatch{{Type: primitives.PrimitiveDispatchCompute, Data: primitives.ComputeRequest{Source: primitives.SourceID(current.operation.ID), Run: func(context.Context) (any, error) { close(started); <-release; return nil, nil }}}}})
	<-started
	terminal := current.operation
	terminal.Status = StatusCompleted
	m.acceptLocalStep(operations, current, Step{Operation: &terminal})
	if err := m.LocalCleanupProof([]ID{current.operation.ID}); !errors.Is(err, ErrLocalCleanupUnproven) {
		t.Fatalf("terminal status became proof: %v", err)
	}
	if len(m.pendingUpdates) != 0 {
		t.Fatal("terminal escaped before callback drain")
	}
	drained := make(chan struct{})
	go func() { m.drainLocalPrimitives(active); close(drained) }()
	if err := m.LocalCleanupProof([]ID{current.operation.ID}); !errors.Is(err, ErrLocalCleanupUnproven) {
		t.Fatalf("blocked callback proved: %v", err)
	}
	unblock()
	<-drained
	current.activePrimitives = 0
	m.finishOwnedWithLocalReceipt(operations, current)
	if err := m.LocalCleanupProof([]ID{current.operation.ID}); err != nil {
		t.Fatal(err)
	}
	if len(m.pendingUpdates) != 1 {
		t.Fatal("terminal missing after callback drain")
	}
}

func TestLocalCleanupProofLiveShell(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := NewLocalOperationManager(ctx)
	current := lifecycleNewShellOperation(t, "proof-shell", ShellInput{Shell: lifecycleTestShellPath, Command: "printf cleaned"}, t.TempDir(), 64)
	if err := m.Add(current); err != nil {
		t.Fatal(err)
	}
	lifecycleReceiveTerminalOperation(t, m.Updates(), current.ID)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			if err := m.LocalCleanupProof([]ID{current.ID}); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		t.Fatal("proof retired live manager")
	}
	if err := m.LocalCleanupProof([]ID{"missing"}); !errors.Is(err, ErrLocalCleanupUnproven) {
		t.Fatal(err)
	}
	cancel()
	if err := awaitStoppedForTest(t, m); err != nil {
		t.Fatal(err)
	}
}

func TestLocalCleanupProofRejectsRemoteUnknownUnsupportedAndEvicted(t *testing.T) {
	m := &LocalOperationManager{}
	finish := func(id ID, kind Type, version Version, status Status, unknown bool) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		current := &localRunningOperation{ctx: ctx, cancel: cancel, remoteJobHandlerIndex: -1, remoteUnknown: unknown, operation: Operation{ID: id, Type: kind, Version: version, Status: status}}
		m.finishOwnedWithLocalReceipt(map[ID]*localRunningOperation{id: current}, current)
	}
	finish("remote", TypeRemoteJob, VersionRemoteJob, StatusCompleted, false)
	finish("unknown", TypeShell, VersionShell, StatusCompleted, true)
	finish("unsupported", TypeShell, VersionShell, StatusUnsupported, false)
	finish("bad-version", TypeShell, VersionShellBudget+1, StatusCompleted, false)
	finish("budget-version", TypeShell, VersionShellBudget, StatusCompleted, false)
	if err := m.LocalCleanupProof([]ID{"budget-version"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []ID{"remote", "unknown", "unsupported", "bad-version", "missing"} {
		if err := m.LocalCleanupProof([]ID{id}); !errors.Is(err, ErrLocalCleanupUnproven) {
			t.Fatalf("%q got proof: %v", id, err)
		}
	}
	for i := range localCleanupReceiptLimit + 1 {
		finish(ID(fmt.Sprintf("receipt-%03d", i)), TypeValue, VersionValue, StatusCompleted, false)
	}
	if len(m.localCleanupReceipts) != localCleanupReceiptLimit {
		t.Fatal("receipt retention is unbounded")
	}
	if err := m.LocalCleanupProof([]ID{"receipt-000"}); !errors.Is(err, ErrLocalCleanupUnproven) {
		t.Fatalf("evicted ID proved: %v", err)
	}
	if err := m.LocalCleanupProof([]ID{"receipt-001", "receipt-256"}); err != nil {
		t.Fatal(err)
	}
	if err := m.LocalCleanupProof(nil); err != nil {
		t.Fatal(err)
	}
}
