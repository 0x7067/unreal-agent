package taskgraph

import (
	"context"
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func runtimeOperation(t *testing.T, id string, typ operation.RemoteJobPlanType, plan any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: PlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
}
func runtimeWait(t *testing.T, m operation.Manager, p func(operation.Operation) bool) operation.Operation {
	t.Helper()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case op := <-m.Updates():
			if p(op) {
				return op
			}
		case <-timer.C:
			t.Fatal("timed out waiting for runtime update")
		}
	}
}
func runtimeGraph(t *testing.T, op operation.Operation) Graph {
	t.Helper()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	var cp checkpoint
	if err = json.Unmarshal(state.Handle, &cp); err != nil {
		t.Fatal(err)
	}
	return cp.Graph
}
func TestRuntimeRequiresExplicitAcceptance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	task := Task{ID: "a", Command: "printf candidate", Reads: []string{}, Writes: []string{}, Acceptance: "verify candidate"}
	if err = m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})); err != nil {
		t.Fatal(err)
	}
	done := runtimeWait(t, m, func(op operation.Operation) bool {
		g := runtimeGraph(t, op)
		return op.ID == "run" && g.Find("a").Status == Completed
	})
	if done.Status != operation.StatusAwaiting {
		t.Fatalf("candidate prematurely terminal: %s", done.Status)
	}
	if err = m.Add(runtimeOperation(t, "accept", ControlPlanType, ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "candidate verified"})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" && op.Status == operation.StatusCompleted })
}
func TestRuntimeFailureDoesNotBlockIndependentTask(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	tasks := []Task{{ID: "a", Command: "exit 7", Reads: []string{}, Writes: []string{}, Acceptance: "success"}, {ID: "b", Command: "sleep .05; printf ok", Reads: []string{}, Writes: []string{}, Acceptance: "success"}}
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks}))
	op := runtimeWait(t, m, func(op operation.Operation) bool {
		g := runtimeGraph(t, op)
		return g.Find("a").Status == Failed && g.Find("b").Status == Completed
	})
	if op.Status != operation.StatusAwaiting {
		t.Fatal(op.Status)
	}
}
func TestRuntimeReaderChangedIsNotAcceptable(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "input"), []byte("old"), 0600)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "printf new > input", Reads: []string{"input"}, Writes: []string{}, Acceptance: "success"}}}))
	runtimeWait(t, m, func(op operation.Operation) bool { g := runtimeGraph(t, op); return g.Find("a").Status == Failed })
}

func TestRuntimeAcceptedDependencyLaunchesWhileSiblingRuns(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	tasks := []Task{{ID: "a", Command: "printf a > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "a exists"}, {ID: "b", Command: "printf started > b; sleep 2", Reads: []string{}, Writes: []string{"b"}, Acceptance: "b complete"}, {ID: "c", Command: "cat a > c", DependsOn: []string{"a"}, Reads: []string{"a"}, Writes: []string{"c"}, Acceptance: "c matches a"}}
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks, Concurrency: 2}))
	runtimeWait(t, m, func(op operation.Operation) bool {
		g := runtimeGraph(t, op)
		return g.Find("a").Status == Completed && g.Find("b").Status == Running
	})
	if _, err = os.Stat(filepath.Join(dir, "b")); err != nil {
		t.Fatal("independent b never overlapped a", err)
	}
	m.Add(runtimeOperation(t, "accept", ControlPlanType, ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "a verified"}))
	op := runtimeWait(t, m, func(op operation.Operation) bool {
		if op.ID != "run" {
			return false
		}
		g := runtimeGraph(t, op)
		return g.Find("c").Status == Completed
	})
	g := runtimeGraph(t, op)
	if g.Find("b").Status != Running {
		t.Fatal("c waited for unrelated b")
	}
}
func TestRuntimeFreshnessBeforeAcceptance(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "printf original > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "original"}}}))
	runtimeWait(t, m, func(op operation.Operation) bool { g := runtimeGraph(t, op); return g.Find("a").Status == Completed })
	os.WriteFile(filepath.Join(dir, "a"), []byte("changed"), 0600)
	m.Add(runtimeOperation(t, "accept", ControlPlanType, ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "original"}))
	op := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "accept" })
	if op.Status != operation.StatusFailed {
		t.Fatal("changed result accepted")
	}
	g := runtimeGraph(t, runtimeWait(t, m, func(op operation.Operation) bool {
		g := runtimeGraph(t, op)
		return op.ID == "run" && g.Find("a").Generation == 2
	}))
	if g.Find("a").Status == Accepted {
		t.Fatal("stale result still accepted")
	}
}
func TestRuntimeUnsupportedCheckpoint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "true", Acceptance: "true"}}})
	state, _ := operation.DecodeRemoteJobState(op)
	state.Handle, _ = json.Marshal(checkpoint{Version: 2})
	step, _ := operation.UpdateRemoteJob(op, state, operation.StatusAwaiting)
	m.Add(*step.Operation)
	failed := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" })
	if failed.Status != operation.StatusFailed {
		t.Fatal(failed.Status)
	}
}
func TestRuntimeResumePendingAndAccepted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a"), []byte("verified"), 0600)
	tasks := []Task{{ID: "a", Command: "exit 9", Reads: []string{}, Writes: []string{"a"}, Acceptance: "verified"}, {ID: "b", Command: "cat a > b", DependsOn: []string{"a"}, Reads: []string{"a"}, Writes: []string{"b"}, Acceptance: "matches"}}
	graph, err := NewGraph("g", tasks, 2)
	if err != nil {
		t.Fatal(err)
	}
	graph.Start("a")
	graph.Complete("a", 1, "verified", nil)
	graph.Accept("a", 1, "verified on disk")
	childSpec, _ := operation.NewShellSpec(operation.ShellInput{Command: "exit 9", Shell: "/bin/sh", Directory: dir}, t.TempDir(), operation.DefaultMaxOutputLength)
	fingerprint, _ := Fingerprint(dir, []string{"a"})
	cp := checkpoint{Version: 1, Graph: graph, Attempts: map[string]*attempt{"a": {Generation: 1, ChildOperation: operation.Operation{ID: "saved-a", Type: childSpec.Type, Version: childSpec.Version, State: childSpec.State, Status: operation.StatusCompleted, MaxOutputLength: childSpec.MaxOutputLength}, OutputFingerprint: fingerprint}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})
	state, _ := operation.DecodeRemoteJobState(op)
	state.Handle, _ = json.Marshal(cp)
	step, _ := operation.UpdateRemoteJob(op, state, operation.StatusAwaiting)
	m.Add(*step.Operation)
	done := runtimeWait(t, m, func(op operation.Operation) bool { g := runtimeGraph(t, op); return g.Find("b").Status == Completed })
	g := runtimeGraph(t, done)
	if g.Find("a").Status != Accepted {
		t.Fatal("accepted result restarted")
	}
}
func TestRuntimeCancellationRetainsLeaseUntilTerminal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), Shell: "/bin/sh", AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "sleep 10", Reads: []string{}, Writes: []string{}, Acceptance: "done"}}}))
	runtimeWait(t, m, func(op operation.Operation) bool {
		state, _ := operation.DecodeRemoteJobState(op)
		var cp checkpoint
		json.Unmarshal(state.Handle, &cp)
		a := cp.Attempts["a"]
		if a == nil {
			return false
		}
		var shell operation.ShellState
		json.Unmarshal(a.ChildOperation.State, &shell)
		return shell.Phase == operation.ShellPhaseProcess && shell.ProcessGroupID > 1
	})
	m.Add(runtimeOperation(t, "cancel", ControlPlanType, ControlPlan{Name: "g", Action: "cancel"}))
	sawLease := false
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case op := <-m.Updates():
			g := runtimeGraph(t, op)
			if g.Find("a").Status == Canceling {
				sawLease = true
			}
			if op.ID == "run" && op.Status == operation.StatusCanceled {
				if !sawLease {
					t.Fatal("no canceling lease checkpoint")
				}
				if g.Find("a").Status != Canceled {
					t.Fatal("graph canceled before task terminal")
				}
				return
			}
		case <-deadline.C:
			t.Fatal("cancellation did not drain")
		}
	}
}
func TestRuntimeResumeUnknownProcessRetainsReservation(t *testing.T) {
	dir := t.TempDir()
	graph, _ := NewGraph("g", []Task{{ID: "a", Command: "printf should-not-run > sentinel", Reads: []string{}, Writes: []string{}, Acceptance: "done"}}, 1)
	graph.Start("a")
	spec, _ := operation.NewShellSpec(operation.ShellInput{Command: graph.Find("a").Task.Command, Shell: "/bin/sh", Directory: dir}, t.TempDir(), operation.DefaultMaxOutputLength)
	var shell operation.ShellState
	json.Unmarshal(spec.State, &shell)
	shell.Phase = operation.ShellPhaseProcess
	shell.ProcessGroupID = 0
	encoded, _ := json.Marshal(shell)
	cp := checkpoint{Version: 1, Graph: graph, Attempts: map[string]*attempt{"a": {Generation: 1, ChildOperation: operation.Operation{ID: "saved", Type: spec.Type, Version: spec.Version, State: encoded, Status: operation.StatusAwaiting, MaxOutputLength: spec.MaxOutputLength}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{graph.Find("a").Task}})
	state, _ := operation.DecodeRemoteJobState(op)
	state.Handle, _ = json.Marshal(cp)
	step, _ := operation.UpdateRemoteJob(op, state, operation.StatusAwaiting)
	m.Add(*step.Operation)
	got := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" })
	g := runtimeGraph(t, got)
	if g.Find("a").Status != Running {
		t.Fatal("unknown process released reservation")
	}
	if _, err = os.Stat(filepath.Join(dir, "sentinel")); !os.IsNotExist(err) {
		t.Fatal("unknown process restarted")
	}
}

func TestRuntimeStatusExposesCandidateSnapshot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "printf candidate", Reads: []string{}, Writes: []string{}, Acceptance: "verify candidate"}}}))
	runtimeWait(t, m, func(op operation.Operation) bool { g := runtimeGraph(t, op); return g.Find("a").Status == Completed })
	m.Add(runtimeOperation(t, "status", ControlPlanType, ControlPlan{Name: "g", Action: "status"}))
	status := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "status" })
	state, _ := operation.DecodeRemoteJobState(status)
	var snapshot struct {
		Graph    Graph  `json:"graph"`
		Blockers string `json:"blockers"`
	}
	if err = json.Unmarshal([]byte(state.TerminalResult), &snapshot); err != nil {
		t.Fatal(err)
	}
	if n := snapshot.Graph.Find("a"); n == nil || n.Result != "candidate" || n.Generation != 1 || n.Status != Completed || snapshot.Blockers == "" {
		t.Fatalf("status lacks reviewable candidate: %s", state.TerminalResult)
	}
}
func TestRuntimeRejectsWritableSymlinkBeforeExecution(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "sentinel")
	if err := os.Symlink(outside, filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "printf escaped > alias/sentinel", Reads: []string{}, Writes: []string{"alias/sentinel"}, Acceptance: "created"}}}))
	runtimeWait(t, m, func(op operation.Operation) bool { g := runtimeGraph(t, op); return g.Find("a").Status == Failed })
	if _, err = os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("task executed through writable symlink")
	}
}
