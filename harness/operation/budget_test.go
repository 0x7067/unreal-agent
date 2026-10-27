package operation

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func TestBudgetWarningCancellationRetainsUninterruptibleOwnership(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		a := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 1})
		lease, err := a.Acquire()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := &LocalOperationManager{options: LocalOperationManagerOptions{ResourceAccount: a, now: func() time.Time { return now }}, primitiveEvents: make(chan primitives.PrimitiveEvent)}
		current := &localRunningOperation{ctx: ctx, cancel: cancel, lease: lease, remoteJobHandlerIndex: -1, operation: Operation{ID: "quiet", Status: StatusAwaiting}, activity: OperationActivity{ID: "quiet", StartedAt: now, Phase: ActivityRunning, Budget: executionbudget.Policy{WarnAfter: time.Second, Timeout: 2 * time.Second}}}
		operations := map[ID]*localRunningOperation{"quiet": current}
		started := make(chan struct{})
		release := make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		defer unblock()
		active := m.acceptLocalStep(operations, current, Step{Dispatches: []PrimitiveDispatch{{Type: primitives.PrimitiveDispatchCompute, Data: primitives.ComputeRequest{Source: "quiet", Run: func(context.Context) (any, error) { close(started); <-release; return nil, nil }}}}})
		<-started
		now = now.Add(time.Second)
		m.checkBudgets(operations)
		warning := m.ActivitySnapshot().Operations[0]
		if warning.Phase != ActivityWarning || warning.WarningAt.IsZero() || ctx.Err() != nil {
			t.Fatalf("warning: %+v", warning)
		}
		now = now.Add(time.Second)
		m.checkBudgets(operations)
		stopping := m.ActivitySnapshot().Operations[0]
		if stopping.Phase != ActivityDraining || stopping.CancellationRequestedAt.IsZero() || stopping.WarningAt.After(stopping.CancellationRequestedAt) {
			t.Fatalf("stopping: %+v", stopping)
		}
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal("budget did not request cancellation")
		}
		if _, err := a.Acquire(); !errors.Is(err, executionbudget.ErrCapacity) {
			t.Fatal("timeout released capacity")
		}
		drained := make(chan struct{})
		go func() { m.drainLocalPrimitives(active); close(drained) }()
		synctest.Wait()
		select {
		case <-drained:
			t.Fatal("claimed drain while callback blocked")
		default:
		}
		if !m.ActivitySnapshot().Operations[0].Owned {
			t.Fatal("work forgotten during drain")
		}
		unblock()
		<-drained
		current.activePrimitives = 0
		current.terminal = true
		m.finishOwned(operations, current)
		done := m.ActivitySnapshot()
		if done.Account.Owned != 0 || done.Operations[0].Phase != ActivityTerminal || done.Operations[0].Owned {
			t.Fatalf("after drain: %+v", done)
		}
	})
}

func TestBudgetHealthyQuietWorkNotStalled(t *testing.T) {
	now := time.Now()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m := &LocalOperationManager{options: LocalOperationManagerOptions{now: func() time.Time { return now }}}
	current := &localRunningOperation{ctx: ctx, cancel: cancel, operation: Operation{ID: "healthy"}, activity: OperationActivity{ID: "healthy", StartedAt: now, Phase: ActivityRunning, Budget: executionbudget.Policy{WarnAfter: time.Hour, Timeout: 2 * time.Hour}}}
	now = now.Add(30 * time.Minute)
	m.checkBudgets(map[ID]*localRunningOperation{"healthy": current})
	if ctx.Err() != nil || current.activity.Phase != ActivityRunning || !current.activity.WarningAt.IsZero() {
		t.Fatal("silence treated as stalled")
	}
}

func TestSharedAccountAcrossManagersRetainsRemoteUntilTerminal(t *testing.T) {
	// The real actor and remote contract are exercised, not just account arithmetic.
	a := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 1})
	ctx, cancel := context.WithCancel(executionbudget.ContextWithAccount(t.Context(), a))
	defer cancel()
	handler := &budgetRemote{updates: make(chan Operation), added: make(chan Operation, 1), canceled: make(chan ID, 1)}
	first := NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{}, handler)
	second := NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{})
	spec, err := NewRemoteJobSpec(RemoteJobPlan{Type: "budget-test", Version: 1, Data: jsontext.Value(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "parent", Type: spec.Type, Version: spec.Version, Status: StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
	if err := first.Add(op); err != nil {
		t.Fatal(err)
	}
	launched := <-handler.added
	if err := first.Cancel(op.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	<-handler.canceled
	if a.Snapshot().Owned != 1 {
		t.Fatal("cancel ack released account")
	}
	child := op
	child.ID = "nested"
	if err := second.Add(child); err != nil {
		t.Fatal(err)
	}
	select {
	case rejected := <-second.Updates():
		if rejected.Status != StatusFailed {
			t.Fatalf("overload: %+v", rejected)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nested rejection blocked")
	}
	launched.Status = StatusCanceled
	handler.updates <- launched
	select {
	case done := <-first.Updates():
		if done.Status != StatusCanceled {
			t.Fatal(done.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("terminal not routed")
	}
	// Receiving the update precedes the actor's final lease bookkeeping.
	deadline := time.Now().Add(time.Second)
	for a.Snapshot().Owned != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if a.Snapshot().Owned != 0 {
		t.Fatal("terminal drain retained capacity")
	}
	cancel()
	for range first.Updates() {
	}
	for range second.Updates() {
	}
}

type budgetRemote struct {
	cancelErr error
	updates   chan Operation
	added     chan Operation
	canceled  chan ID
}

func (*budgetRemote) RemoteJobPlanType() RemoteJobPlanType       { return "budget-test" }
func (*budgetRemote) RemoteJobPlanVersion() RemoteJobPlanVersion { return 1 }
func (h *budgetRemote) AddRemoteJob(op Operation) error          { h.added <- op; return nil }
func (h *budgetRemote) CancelRemoteJob(id ID, _ string) error    { h.canceled <- id; return h.cancelErr }
func (h *budgetRemote) RemoteJobUpdates() <-chan Operation       { return h.updates }

func TestQuietShellReachesObservableBudgetAndActualDrain(t *testing.T) {
	account := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 1})
	ctx, cancel := context.WithCancel(t.Context())
	var clock atomic.Int64
	start := time.Now()
	clock.Store(start.UnixNano())
	ticks := make(chan time.Time)
	m := NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{ResourceAccount: account, now: func() time.Time { return time.Unix(0, clock.Load()) }, budgetTicks: ticks})
	defer func() {
		cancel()
		for range m.Updates() {
		}
	}()
	spec, err := NewShellSpec(ShellInput{Shell: "/bin/sh", Command: "exec sleep 30", Budget: executionbudget.Policy{WarnAfter: time.Second, Timeout: 2 * time.Second}}, t.TempDir(), 1)
	if err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "quiet-shell", Type: spec.Type, Version: spec.Version, Status: StatusReady, MaxOutputLength: spec.MaxOutputLength, State: spec.State}
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case update := <-m.Updates():
			state, err := DecodeShellState(update)
			if err != nil {
				t.Fatal(err)
			}
			if update.Status == StatusFailed {
				t.Fatalf("shell failed: %s", state.TerminalError)
			}
			if state.ProcessGroupID > 0 {
				goto processStarted
			}
		case <-deadline:
			t.Fatal("quiet shell did not start")
		}
	}
processStarted:
	clock.Store(start.Add(time.Second).UnixNano())
	ticks <- time.Time{}
	// Duplicate admission is a barrier after the preceding budget check, without
	// scheduling work or requiring wall sleeps to observe the warning phase.
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	warning := m.ActivitySnapshot().Operations[0]
	if warning.Phase != ActivityWarning || warning.CancellationRequestedAt.IsZero() == false || !warning.ProcessStarted {
		t.Fatalf("warning: %+v", warning)
	}
	if account.Snapshot().Owned != 1 {
		t.Fatal("quiet live process unaccounted")
	}
	clock.Store(start.Add(2 * time.Second).UnixNano())
	ticks <- time.Time{}
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	stopping := m.ActivitySnapshot().Operations[0]
	if stopping.CancellationRequestedAt.IsZero() || stopping.DrainingAt.IsZero() {
		t.Fatalf("budget not observable: %+v", stopping)
	}
	for {
		select {
		case update := <-m.Updates():
			if !localOperationFinished(update.Status) {
				continue
			}
			if update.Status != StatusCanceled {
				t.Fatalf("budget outcome: %s", update.Status)
			}
			state, err := DecodeShellState(update)
			if err != nil {
				t.Fatal(err)
			}
			if state.TerminalError == "" {
				t.Fatal("cancellation not represented in terminal checkpoint")
			}
			// Update publication may precede the actor bookkeeping; a duplicate Add
			// confirms completion of the previous turn before inspecting resource drain.
			if err := m.Add(op); err != nil {
				t.Fatal(err)
			}
			done := m.ActivitySnapshot()
			if done.Account.Owned != 0 || done.Operations[0].Owned || !done.Operations[0].ProcessCompletionObserved || done.Operations[0].Phase != ActivityTerminal {
				t.Fatalf("incomplete drain: %+v", done)
			}
			return
		case <-deadline:
			t.Fatal("quiet shell budget did not reach actual drain")
		}
	}
}

func TestUnknownRemoteCancellationRetainsCapacityUntilProvenTerminal(t *testing.T) {
	a := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 1})
	ctx, cancel := context.WithCancel(t.Context())
	h := &budgetRemote{updates: make(chan Operation), added: make(chan Operation, 1), canceled: make(chan ID, 1), cancelErr: errors.New("transport failed")}
	m := NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{ResourceAccount: a}, h)
	defer func() {
		cancel()
		for range m.Updates() {
		}
	}()
	spec, err := NewRemoteJobSpec(RemoteJobPlan{Type: "budget-test", Version: 1, Data: jsontext.Value(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	op := Operation{ID: "unknown", Type: spec.Type, Version: spec.Version, Status: StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	launched := <-h.added
	if err := m.Cancel(op.ID, "stop"); err != nil {
		t.Fatal(err)
	}
	<-h.canceled
	select {
	case failed := <-m.Updates():
		if failed.Status != StatusFailed {
			t.Fatal(failed.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("unknown outcome missing")
	}
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	snapshot := m.ActivitySnapshot()
	if snapshot.Account.Owned != 1 || !snapshot.Operations[0].Owned || snapshot.Operations[0].Phase != ActivityDraining {
		t.Fatalf("unknown outcome released ownership: %+v", snapshot)
	}
	launched.Status = StatusCanceled
	h.updates <- launched
	deadline := time.Now().Add(time.Second)
	for a.Snapshot().Owned != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if err := m.Add(op); err != nil {
		t.Fatal(err)
	}
	snapshot = m.ActivitySnapshot()
	if snapshot.Account.Owned != 0 || snapshot.Operations[0].Phase != ActivityTerminal || snapshot.Operations[0].Status != StatusFailed {
		t.Fatalf("later terminal failed to prove drain without rewriting failure: %+v", snapshot)
	}
}
