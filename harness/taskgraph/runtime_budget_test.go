package taskgraph

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type budgetGraphHandler struct {
	updates  chan operation.Operation
	added    chan operation.Operation
	canceled chan operation.ID
}

func (*budgetGraphHandler) RemoteJobPlanType() operation.RemoteJobPlanType {
	return toolsubagent.RunPlanType
}
func (*budgetGraphHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.PlanVersion
}
func (h *budgetGraphHandler) RemoteJobUpdates() <-chan operation.Operation { return h.updates }
func (h *budgetGraphHandler) AddRemoteJob(op operation.Operation) error    { h.added <- op; return nil }
func (h *budgetGraphHandler) CancelRemoteJob(id operation.ID, _ string) error {
	h.canceled <- id
	return nil
}

func TestRuntimeSharesAccountAndRetainsClaimsThroughActualDrain(t *testing.T) {
	account := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 2})
	ctx, cancel := context.WithCancel(t.Context())
	h := &budgetGraphHandler{updates: make(chan operation.Operation), added: make(chan operation.Operation, 1), canceled: make(chan operation.ID, 1)}
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), ResourceAccount: account, NewAgentHandlers: func(childCtx context.Context) ([]operation.RemoteJobHandler, error) {
		if executionbudget.AccountFromContext(childCtx) != account {
			t.Error("graph factory did not inherit root account")
		}
		return []operation.RemoteJobHandler{h}, nil
	}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{ResourceAccount: account}, r.Handlers()...)
	closeUpdates := sync.OnceFunc(func() { close(h.updates) })
	defer func() {
		cancel()
		closeUpdates()
		for range m.Updates() {
		}
		if err := r.Wait(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if err := m.Add(runtimeOperation(t, "graph", RunPlanType, RunPlan{Name: "shared", Tasks: []Task{{ID: "review", Prompt: "blocked helper", Reads: []string{}, Writes: []string{"report.txt"}, Acceptance: "report reviewed"}}})); err != nil {
		t.Fatal(err)
	}
	var child operation.Operation
	select {
	case child = <-h.added:
	case <-time.After(5 * time.Second):
		t.Fatal("child not admitted")
	}
	if snapshot := account.Snapshot(); snapshot.Owned != 2 {
		t.Fatalf("root and child not counted together: %+v", snapshot)
	}
	if _, err := account.Acquire(); !errors.Is(err, executionbudget.ErrCapacity) {
		t.Fatalf("shared bound not enforced: %v", err)
	}
	if err := m.Cancel("graph", "stop graph"); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-h.canceled:
		if id != child.ID {
			t.Fatalf("canceled=%s child=%s", id, child.ID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("child cancellation not requested")
	}
	// Cancellation acknowledgment is not drain. Inspect state only through the
	// runtime owner; do not mutate or invoke the reducer from another goroutine.
	barrier := make(chan struct{})
	if err := r.request(func(s *runtimeState) {
		if n := s.graphs["shared"].Graph.Find("review"); n.Status != Canceling {
			t.Errorf("claim released before child terminal: %+v", n)
		}
		close(barrier)
	}); err != nil {
		t.Fatal(err)
	}
	<-barrier
	if snapshot := account.Snapshot(); snapshot.Owned != 2 {
		t.Fatalf("cancellation released account early: %+v", snapshot)
	}
	child.Status = operation.StatusCanceled
	h.updates <- child
	runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "graph" && op.Status == operation.StatusCanceled })
	// The runtime retains joins until shutdown even after terminal publication.
	// Successful Wait is the final ownership barrier, not a terminal text report.
	cancel()
	closeUpdates()
	for range m.Updates() {
	}
	if err := r.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if snapshot := account.Snapshot(); snapshot.Owned != 0 {
		t.Fatalf("drained graph retained account: %+v", snapshot)
	}
}
