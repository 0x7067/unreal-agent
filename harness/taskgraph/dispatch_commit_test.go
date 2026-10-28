package taskgraph

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// The forwarding consumer is always live: manager/runtime updates are
// unbuffered, and waiting for a commit must not stop update delivery.
type dispatchTestManager struct {
	operation.Manager
	updates chan operation.Operation
	drained chan struct{}
}

func (m *dispatchTestManager) Updates() <-chan operation.Operation { return m.updates }

type dispatchFixture struct {
	r         *Runtime
	m         *dispatchTestManager
	cancel    context.CancelFunc
	workspace string
	stopOnce  sync.Once
}

func newDispatchFixture(t *testing.T, workspace, scope string) *dispatchFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if err := os.MkdirAll(filepath.Join(workspace, "operations"), 0700); err != nil {
		t.Fatal(err)
	}
	r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: filepath.Join(workspace, "operations"), Shell: "/bin/sh", AllowBash: true, DurableParentID: "parent", DurableScope: scope})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	m := &dispatchTestManager{Manager: operation.NewLocalOperationManager(ctx, r.Handlers()...), updates: make(chan operation.Operation, 256), drained: make(chan struct{})}
	go func() {
		defer close(m.drained)
		defer close(m.updates)
		for op := range m.Manager.Updates() {
			select {
			case m.updates <- op:
			case <-ctx.Done():
			}
		}
	}()
	f := &dispatchFixture{r: r, m: m, cancel: cancel, workspace: workspace}
	t.Cleanup(func() { f.stop(t) })
	return f
}
func dispatchBarrier(t *testing.T, r *Runtime, inspect func(*runtimeState)) {
	t.Helper()
	done := make(chan struct{})
	if err := r.request(func(s *runtimeState) { inspect(s); close(done) }); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("actor callback barrier did not complete")
	}
}
func (f *dispatchFixture) stop(t *testing.T) {
	t.Helper()
	f.stopOnce.Do(func() {
		var managers []operation.Manager
		dispatchBarrier(t, f.r, func(s *runtimeState) {
			for _, g := range s.graphs {
				managers = append(managers, g.manager)
			}
		})
		f.cancel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := f.r.Wait(ctx); err != nil {
			t.Errorf("runtime did not join managers/scans: %v", err)
			return
		}
		for _, m := range managers {
			select {
			case _, open := <-m.Updates():
				if open {
					t.Error("nested manager not fully drained after Wait")
				}
			default:
				t.Error("nested manager update channel still open after Wait")
			}
		}
		select {
		case <-f.m.drained:
		case <-ctx.Done():
			t.Error("outer manager did not drain")
		}
		select {
		case <-f.r.requests.done:
		default:
			t.Error("mailbox did not join")
		}
	})
}
func dispatchOperation(t *testing.T, id string, typ operation.RemoteJobPlanType, plan any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
}
func dispatchRun(t *testing.T, tasks []Task) operation.Operation {
	return dispatchOperation(t, "run", RunPlanType, DurableRunPlan{Version: 1, ParentID: "parent", Run: RunPlan{Name: "graph", Tasks: tasks}})
}
func dispatchTask(id string) Task {
	return Task{ID: id, Command: "printf '" + id + "\\n' >> " + id + "-sentinel", Reads: []string{}, Writes: []string{id + "-sentinel"}, Acceptance: "inspect command counter"}
}
func dispatchCheckpoint(t *testing.T, op operation.Operation) *DurableCheckpoint {
	t.Helper()
	_, cp, err := DecodeDurableRun(op)
	if err != nil {
		t.Fatal(err)
	}
	if cp == nil {
		t.Fatal("missing wrapped durable checkpoint")
	}
	return cp
}
func dispatchWithCheckpoint(t *testing.T, op operation.Operation, cp *DurableCheckpoint) operation.Operation {
	t.Helper()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	state.Handle, err = json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	op.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return op
}
func dispatchReserved(t *testing.T, f *dispatchFixture) operation.Operation {
	t.Helper()
	return runtimeWait(t, f.m, func(op operation.Operation) bool {
		if op.ID != "run" {
			return false
		}
		cp := dispatchCheckpoint(t, op)
		a := cp.State.Attempts["a"]
		return a != nil && a.Phase == "execution" && a.CommandDispatched && a.ChildOperation.Status == operation.StatusReady
	})
}

// This deliberately stands in for command-layer durable OpSave, NOT the real
// session store integration. NotifyCheckpointCommitted is called only after a
// serialized operation has been saved and read back from the test's temp file.
func dispatchSimulateOpSave(t *testing.T, op operation.Operation) operation.Operation {
	t.Helper()
	data, err := json.Marshal(op)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "saved-operation.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var saved operation.Operation
	if err = json.Unmarshal(data, &saved); err != nil {
		t.Fatal(err)
	}
	return saved
}
func dispatchCommit(t *testing.T, f *dispatchFixture, op operation.Operation) {
	t.Helper()
	saved := dispatchSimulateOpSave(t, op)
	if err := f.r.NotifyCheckpointCommitted(saved); err != nil {
		t.Fatal(err)
	}
	// Admission is not execution: this FIFO callback proves the ack ran.
	dispatchBarrier(t, f.r, func(*runtimeState) {})
}
func dispatchNoCommand(t *testing.T, workspace, id string) {
	t.Helper()
	// The actor barrier is the primary ordering proof; this bounded sentinel
	// observation additionally catches accidental asynchronous process admission.
	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	path := filepath.Join(workspace, id+"-sentinel")
	for {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("command ran before authorization (%s): %v", path, err)
		}
		select {
		case <-timer.C:
			return
		case <-tick.C:
		}
	}
}
func dispatchPending(t *testing.T, f *dispatchFixture) {
	t.Helper()
	dispatchBarrier(t, f.r, func(s *runtimeState) {
		g := s.graphs["graph"]
		if g == nil {
			t.Error("missing live graph")
			return
		}
		a := g.Attempts["a"]
		if a == nil || len(g.pendingDispatch) != 1 || g.pendingDispatch[a.ChildOperation.ID] != a.Generation || a.ChildOperation.Status != operation.StatusReady || g.Graph.Find("a").Status != Running {
			t.Error("ack changed the pending dispatch reservation")
		}
	})
}
func dispatchCount(t *testing.T, workspace, id string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(workspace, id+"-sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != id+"\n" {
		t.Fatalf("command counter = %q; want exactly one %s command", data, id)
	}
}

func TestDurableDispatchCommitFence(t *testing.T) {
	f := newDispatchFixture(t, t.TempDir(), "current-scope")
	if err := f.m.Add(dispatchRun(t, []Task{dispatchTask("a")})); err != nil {
		t.Fatal(err)
	}
	reserved := dispatchReserved(t, f)
	dispatchPending(t, f)
	dispatchNoCommand(t, f.workspace, "a")
	for _, kind := range []string{"scope", "parent", "owner", "child", "generation"} {
		t.Run("stale-"+kind, func(t *testing.T) {
			op := reserved
			cp := dispatchCheckpoint(t, op)
			switch kind {
			case "scope":
				cp.Scope = "old-scope"
			case "parent":
				cp.ParentID = "other-parent"
				state, _ := operation.DecodeRemoteJobState(op)
				var p DurableRunPlan
				if err := json.Unmarshal(state.Plan.Data, &p); err != nil {
					t.Fatal(err)
				}
				p.ParentID = cp.ParentID
				state.Plan.Data, _ = json.Marshal(p)
				op.State, _ = json.Marshal(state)
			case "owner":
				op.ID = "different-owner"
				cp.State.Attempts["a"].ChildOperation.ID = graphChildID(op.ID, "a", cp.State.Attempts["a"].Generation)
			case "child":
				child, _ := operation.DecodeShellState(cp.State.Attempts["a"].ChildOperation)
				child.OutSize = 1
				cp.State.Attempts["a"].ChildOperation.State, _ = json.Marshal(child)
			case "generation":
				cp.State.Attempts["a"].Generation++
				cp.State.Graph.Find("a").Generation++
				cp.State.Attempts["a"].ChildOperation.ID = graphChildID(op.ID, "a", cp.State.Attempts["a"].Generation)
			}
			op = dispatchWithCheckpoint(t, op, cp)
			// These are well-formed but unauthorized acks, not merely codec failures.
			dispatchCheckpoint(t, op)
			dispatchCommit(t, f, op)
			dispatchPending(t, f)
			dispatchNoCommand(t, f.workspace, "a")
		})
	}
	dispatchCommit(t, f, reserved)
	// Duplicate delivery while the command may still be running.
	dispatchCommit(t, f, reserved)
	runtimeWait(t, f.m, func(op operation.Operation) bool {
		return op.ID == "run" && dispatchCheckpoint(t, op).State.Graph.Find("a").Status == Completed
	})
	dispatchCommit(t, f, reserved)
	f.stop(t)
	dispatchCount(t, f.workspace, "a")
}

func TestDurableDispatchCancelBeforeCommit(t *testing.T) {
	f := newDispatchFixture(t, t.TempDir(), "scope")
	if err := f.m.Add(dispatchRun(t, []Task{dispatchTask("a")})); err != nil {
		t.Fatal(err)
	}
	reserved := dispatchReserved(t, f)
	dispatchPending(t, f)
	dispatchNoCommand(t, f.workspace, "a")
	saved := dispatchSimulateOpSave(t, reserved)
	f.stop(t) // Context cancellation and actual manager/scan joins precede late ack.
	if err := f.r.NotifyCheckpointCommitted(saved); err == nil {
		t.Error("ack admitted after context cancellation")
	}
	dispatchNoCommand(t, f.workspace, "a")
}

func TestDurableDispatchPrelaunchRestoreRequiresFreshCommit(t *testing.T) {
	workspace := t.TempDir()
	first := newDispatchFixture(t, workspace, "old-scope")
	if err := first.m.Add(dispatchRun(t, []Task{dispatchTask("a")})); err != nil {
		t.Fatal(err)
	}
	prelaunch := runtimeWait(t, first.m, func(op operation.Operation) bool {
		cp := dispatchCheckpoint(t, op)
		a := cp.State.Attempts["a"]
		return a != nil && a.Phase == "input_validation" && !a.CommandDispatched
	})
	oldReserved := dispatchReserved(t, first)
	first.stop(t)
	if err := ValidateDurableRecovery(prelaunch, "parent"); err != nil {
		t.Fatal(err)
	}
	restored := newDispatchFixture(t, workspace, "fresh-scope")
	if err := restored.m.Add(dispatchSimulateOpSave(t, prelaunch)); err != nil {
		t.Fatal(err)
	}
	freshReserved := dispatchReserved(t, restored)
	if cp := dispatchCheckpoint(t, freshReserved); cp.Scope != "fresh-scope" {
		t.Fatalf("restore emitted old scope: %s", cp.Scope)
	}
	dispatchPending(t, restored)
	dispatchNoCommand(t, workspace, "a")
	dispatchCommit(t, restored, oldReserved)
	dispatchPending(t, restored)
	dispatchNoCommand(t, workspace, "a")
	dispatchCommit(t, restored, freshReserved)
	runtimeWait(t, restored.m, func(op operation.Operation) bool {
		return dispatchCheckpoint(t, op).State.Graph.Find("a").Status == Completed
	})
	restored.stop(t)
	dispatchCount(t, workspace, "a")
}

func TestDurableDispatchReservedRestoreRefused(t *testing.T) {
	workspace := t.TempDir()
	first := newDispatchFixture(t, workspace, "old-scope")
	if err := first.m.Add(dispatchRun(t, []Task{dispatchTask("a")})); err != nil {
		t.Fatal(err)
	}
	reserved := dispatchReserved(t, first)
	first.stop(t)
	// The Ready child has no PID to settle. The dispatch marker is an unknown
	// launch boundary, not permission to replay or conclude the process exited.
	if err := ValidateDurableRecovery(reserved, "parent"); err == nil || !strings.Contains(err.Error(), "not proven terminal") {
		t.Fatalf("reserved recovery = %v", err)
	}
	restored := newDispatchFixture(t, workspace, "new-scope")
	if err := restored.m.Add(dispatchSimulateOpSave(t, reserved)); err != nil {
		t.Fatal(err)
	}
	result := runtimeWait(t, restored.m, func(op operation.Operation) bool { return op.ID == "run" })
	state, err := operation.DecodeRemoteJobState(result)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != operation.StatusFailed || !strings.Contains(state.TerminalError, "not proven terminal") {
		t.Fatalf("unknown launch was not refused: %s %q", result.Status, state.TerminalError)
	}
	dispatchBarrier(t, restored.r, func(s *runtimeState) {
		if s.graphs["graph"] != nil {
			t.Error("refused restore installed graph/manager")
		}
	})
	restored.stop(t)
	dispatchNoCommand(t, workspace, "a")
}

func TestDurableDispatchControlsAreParentScopeGenerationFenced(t *testing.T) {
	f := newDispatchFixture(t, t.TempDir(), "scope")
	a, b := dispatchTask("a"), dispatchTask("b")
	b.DependsOn = []string{"a"}
	if err := f.m.Add(dispatchRun(t, []Task{a, b})); err != nil {
		t.Fatal(err)
	}
	reserved := dispatchReserved(t, f)
	before := dispatchCheckpoint(t, reserved).State.Graph
	for _, kind := range []string{"scope", "parent"} {
		control := DurableControlPlan{Version: 1, ParentID: "parent", Scope: "scope", Control: ControlPlan{Name: "graph", Action: "cancel"}}
		if kind == "scope" {
			control.Scope = "old-scope"
		} else {
			control.ParentID = "other-parent"
		}
		id := "bad-" + kind
		if err := f.m.Add(dispatchOperation(t, id, ControlPlanType, control)); err != nil {
			t.Fatal(err)
		}
		result := runtimeWait(t, f.m, func(op operation.Operation) bool { return op.ID == operation.ID(id) })
		if result.Status != operation.StatusFailed {
			t.Fatalf("stale %s control = %s", kind, result.Status)
		}
		dispatchBarrier(t, f.r, func(s *runtimeState) {
			g := s.graphs["graph"]
			if g.Canceling || !reflect.DeepEqual(before, g.Graph) {
				t.Error("unauthorized control mutated graph before rejection")
			}
		})
	}
	dispatchPending(t, f)
	dispatchCommit(t, f, reserved)
	done := runtimeWait(t, f.m, func(op operation.Operation) bool {
		return op.ID == "run" && dispatchCheckpoint(t, op).State.Graph.Find("a").Status == Completed
	})
	if done.Status != operation.StatusAwaiting {
		t.Fatal("completed candidate finished graph without acceptance")
	}
	control := DurableControlPlan{Version: 1, ParentID: "parent", Scope: "scope", Control: ControlPlan{Name: "graph", Action: "accept", TaskID: "a", Generation: 2, Evidence: "stale generation"}}
	if err := f.m.Add(dispatchOperation(t, "stale-accept", ControlPlanType, control)); err != nil {
		t.Fatal(err)
	}
	rejected := runtimeWait(t, f.m, func(op operation.Operation) bool { return op.ID == "stale-accept" })
	if rejected.Status != operation.StatusFailed {
		t.Fatalf("stale generation accepted: %s", rejected.Status)
	}
	dispatchBarrier(t, f.r, func(s *runtimeState) {
		g := s.graphs["graph"]
		if g.Graph.Find("a").Status != Completed || g.Graph.Find("b").Status != Pending || g.Attempts["b"] != nil || len(g.pendingDispatch) != 0 {
			t.Error("completed unaccepted candidate unblocked dependent")
		}
	})
	dispatchNoCommand(t, f.workspace, "b")
	f.stop(t)
	dispatchCount(t, f.workspace, "a")
}
