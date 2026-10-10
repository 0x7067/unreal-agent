package taskgraph

import (
	"context"
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"os"
	"path/filepath"
	"testing"
)

func runtimeNode(t *testing.T, op operation.Operation, id string) *Node {
	g := runtimeGraph(t, op)
	return g.Find(id)
}
func TestRuntimeStaleAcceptanceContinuesWithoutAnotherControl(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true, Shell: "/bin/sh"})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	tasks := []Task{
		{ID: "a", Command: "printf original > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "original"},
		{ID: "b", Command: "cat a > b", Reads: []string{"a"}, Writes: []string{"b"}, DependsOn: []string{"a"}, Acceptance: "original"},
	}
	if err = m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "a").Status == Completed
	})
	if err = m.Add(runtimeOperation(t, "accept-a", ControlPlanType, ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "original checked"})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "b").Status == Completed
	})
	if err = os.WriteFile(filepath.Join(dir, "a"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = m.Add(runtimeOperation(t, "accept-b", ControlPlanType, ControlPlan{Name: "g", Action: "accept", TaskID: "b", Generation: 1, Evidence: "old checked"})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		if op.ID != "run" {
			return false
		}
		a := runtimeNode(t, op, "a")
		return a.Generation == 2 && (a.Status == Running || a.Status == Completed)
	})
}

func TestRuntimeCancelingChildDoesNotDispatchOnResume(t *testing.T) {
	for _, whole := range []bool{false, true} {
		for _, phase := range []operation.ShellPhase{"", operation.ShellPhaseCreateDirectory} {
			t.Run(map[bool]string{false: "task", true: "graph"}[whole]+"/"+string(phase), func(t *testing.T) {
				dir := t.TempDir()
				tasks := []Task{{ID: "a", Command: "printf side-effect > sentinel", Reads: []string{}, Writes: []string{"sentinel"}, Acceptance: "sentinel"}, {ID: "b", Command: "true", Reads: []string{}, Writes: []string{}, DependsOn: []string{"a"}, Acceptance: "true"}}
				g, err := NewGraph("g", tasks, 1)
				if err != nil {
					t.Fatal(err)
				}
				if err = g.Start("a"); err != nil {
					t.Fatal(err)
				}
				if err = g.Cancel("a"); err != nil {
					t.Fatal(err)
				}
				if whole {
					if err = g.Cancel("b"); err != nil {
						t.Fatal(err)
					}
				}
				spec, err := operation.NewShellSpec(operation.ShellInput{Command: tasks[0].Command, Shell: "/bin/sh", Directory: dir}, t.TempDir(), operation.DefaultMaxOutputLength)
				if err != nil {
					t.Fatal(err)
				}
				status := operation.StatusReady
				if phase != "" {
					var shell operation.ShellState
					if err = json.Unmarshal(spec.State, &shell); err != nil {
						t.Fatal(err)
					}
					shell.Phase = phase
					if spec.State, err = json.Marshal(shell); err != nil {
						t.Fatal(err)
					}
					status = operation.StatusAwaiting
				}
				cp := checkpoint{Version: 1, Graph: g, Canceling: whole, Attempts: map[string]*attempt{"a": {Generation: 1, ChildOperation: operation.Operation{ID: "saved", Type: spec.Type, Version: spec.Version, State: spec.State, Status: status, MaxOutputLength: spec.MaxOutputLength}}}}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true, Shell: "/bin/sh"})
				if err != nil {
					t.Fatal(err)
				}
				m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
				op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})
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
				if err = m.Add(*step.Operation); err != nil {
					t.Fatal(err)
				}
				runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" && runtimeNode(t, op, "a").Status == Canceled })
				if _, err = os.Stat(filepath.Join(dir, "sentinel")); !os.IsNotExist(err) {
					t.Fatalf("canceled ready child dispatched: %v", err)
				}
			})
		}
	}
}
