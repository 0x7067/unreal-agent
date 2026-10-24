package taskgraph

import (
	"context"
	"encoding/json/v2"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func identityCheckpoint(t *testing.T, dir, base string) checkpoint {
	t.Helper()
	g := validationCheckpoint(t, false, false, 2)
	// No filesystem inputs or outputs: both terminal child checkpoints are valid
	// and would complete successfully if each had an unambiguous identity.
	g.Nodes[0].Task.Writes = []string{}
	fingerprint, err := Fingerprint(dir, []string{})
	if err != nil {
		t.Fatal(err)
	}
	cp := checkpoint{Version: 1, Graph: g, Attempts: map[string]*attempt{}}
	for i := range cp.Graph.Nodes {
		n := &cp.Graph.Nodes[i]
		n.Status = Running
		spec, err := operation.NewShellSpec(operation.ShellInput{Command: n.Task.Command, Shell: "/bin/sh", Directory: dir}, base, operation.DefaultMaxOutputLength)
		if err != nil {
			t.Fatal(err)
		}
		cp.Attempts[n.Task.ID] = &attempt{Generation: n.Generation, BeforeReads: fingerprint, ChildOperation: operation.Operation{ID: operation.ID("child-" + n.Task.ID), Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusCompleted}}
	}
	if err := cp.Graph.Validate(); err != nil {
		t.Fatal(err)
	}
	return cp
}

func TestCheckpointCurrentChildIdentity(t *testing.T) {
	cp := identityCheckpoint(t, t.TempDir(), t.TempDir())
	if err := validateAttempts(cp); err != nil {
		t.Fatalf("distinct current IDs rejected: %v", err)
	}
	cp.Attempts["b"].ChildOperation.ID = cp.Attempts["a"].ChildOperation.ID
	if err := validateAttempts(cp); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate current child IDs: got %v, want duplicate error", err)
	}
	// Historical attempts cannot consume current updates and must not reserve IDs.
	cp.Graph.Find("b").Generation++
	cp.Graph.Find("b").Status = Pending
	if err := validateAttempts(cp); err != nil {
		t.Fatalf("obsolete duplicate ID rejected: %v", err)
	}
	// Check both traversal orders: obsolete first and current second as well.
	cp.Graph.Find("b").Generation = 1
	cp.Graph.Find("b").Status = Running
	cp.Graph.Find("a").Generation++
	cp.Graph.Find("a").Status = Pending
	if err := validateAttempts(cp); err != nil {
		t.Fatalf("obsolete first duplicate ID rejected: %v", err)
	}
}

func TestCheckpointDuplicateRestoreRejectedBeforeHandlerSetup(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	cp := identityCheckpoint(t, dir, base)
	cp.Attempts["b"].ChildOperation.ID = cp.Attempts["a"].ChildOperation.ID
	ctx, cancel := context.WithCancel(context.Background())
	var setup atomic.Int32
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: base, AllowBash: true, NewAgentHandlers: func(context.Context) ([]operation.RemoteJobHandler, error) {
		setup.Add(1)
		return nil, nil
	}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		waitCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := r.Wait(waitCtx); err != nil {
			t.Errorf("runtime shutdown: %v", err)
		}
	})
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	tasks := []Task{cp.Graph.Nodes[0].Task, cp.Graph.Nodes[1].Task}
	op := runtimeOperation(t, "restore-duplicate", RunPlanType, RunPlan{Name: cp.Graph.Name, Tasks: tasks, Concurrency: 2})
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	state.Handle, err = json.Marshal(cp)
	if err != nil {
		t.Fatal(err)
	}
	step, err := operation.UpdateRemoteJob(op, state, operation.StatusAwaiting)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Add(*step.Operation); err != nil {
		t.Fatal(err)
	}
	result := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "restore-duplicate" })
	if result.Status != operation.StatusFailed {
		t.Errorf("malformed restore status = %s, want failed", result.Status)
	}
	resultState, err := operation.DecodeRemoteJobState(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resultState.TerminalError, "duplicate") {
		t.Errorf("restore error = %q, want duplicate identity", resultState.TerminalError)
	}
	if calls := setup.Load(); calls != 0 {
		t.Errorf("handler setup invoked %d times before malformed restore rejected", calls)
	}
}
