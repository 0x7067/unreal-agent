package taskgraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// This fixed experiment is deliberately separate from the default CI suite.
// It measures runtime scheduling, not a model's decomposition or review quality.
func TestSchedulingEvaluation(t *testing.T) {
	if os.Getenv("UNREAL_TASKGRAPH_EVALUATION") != "1" {
		t.Skip("set UNREAL_TASKGRAPH_EVALUATION=1 for the fixed timing experiment")
	}
	var serial, parallel []time.Duration
	for repetition := range 5 {
		serial = append(serial, evaluateAcceptedGoal(t, 1, false))
		parallel = append(parallel, evaluateAcceptedGoal(t, 4, false))
		t.Logf("pair %d: serial=%s parallel=%s", repetition+1, serial[repetition], parallel[repetition])
	}
	slices.Sort(serial)
	slices.Sort(parallel)
	improvement := 1 - float64(parallel[2])/float64(serial[2])
	// Programmed work is 3*200ms + 50ms serial, or 200ms + 50ms parallel.
	// The excess includes shell startup, snapshots, scheduling and acceptance;
	// it is an upper bound on coordination overhead, not a CPU attribution.
	t.Logf("median serial=%s parallel=%s improvement=%.1f%% excess above programmed work: serial=%s parallel=%s; model tokens=0",
		serial[2], parallel[2], improvement*100, serial[2]-650*time.Millisecond, parallel[2]-250*time.Millisecond)
	if improvement < .20 {
		t.Fatalf("fixed 20%% median runtime speedup threshold not met: %.1f%%", improvement*100)
	}
	chainSerial := evaluateAcceptedGoal(t, 1, true)
	chainParallel := evaluateAcceptedGoal(t, 4, true)
	t.Logf("chain correctness: serial=%s parallel=%s (no speedup requirement)", chainSerial, chainParallel)
}

func evaluateAcceptedGoal(t *testing.T, concurrency int, chain bool) time.Duration {
	t.Helper()
	workspace := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	r, err := New(ctx, Config{Workspace: workspace, BaseDirectory: t.TempDir(), AllowBash: true})
	if err != nil {
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	tasks := []Task{}
	for i, letter := range []string{"A", "B", "C"} {
		id := fmt.Sprintf("part%d", i)
		item := Task{ID: id, Command: fmt.Sprintf("sleep .2; printf %s > %s", letter, id), Reads: []string{}, Writes: []string{id}, Acceptance: "file contains exactly " + letter}
		if chain && i > 0 {
			item.DependsOn = []string{fmt.Sprintf("part%d", i-1)}
		}
		tasks = append(tasks, item)
	}
	tasks = append(tasks, Task{ID: "combine", Command: "sleep .05; cat part0 part1 part2 > result", DependsOn: []string{"part0", "part1", "part2"}, Reads: []string{"part0", "part1", "part2"}, Writes: []string{"result"}, Acceptance: "result contains exactly ABC"})
	start := time.Now()
	if err := m.Add(runtimeOperation(t, "goal", RunPlanType, RunPlan{Name: "evaluation", Concurrency: concurrency, Tasks: tasks})); err != nil {
		t.Fatal(err)
	}
	submitted := map[string]uint64{}
	var last Graph
	for {
		select {
		case op := <-m.Updates():
			if op.Status == operation.StatusFailed {
				state, _ := operation.DecodeRemoteJobState(op)
				t.Fatalf("operation %s failed: %s; last graph=%+v", op.ID, state.TerminalError, last)
			}
			if op.ID != "goal" {
				continue
			}
			g := runtimeGraph(t, op)
			last = g
			if op.Status == operation.StatusFailed {
				t.Fatalf("goal failed: %+v", g)
			}
			for _, node := range g.Nodes {
				if node.Status == Failed {
					t.Fatalf("task failed: %+v", node)
				}
				if node.Status != Completed || submitted[node.Task.ID] == node.Generation {
					continue
				}
				path := node.Task.ID
				expected := map[string]string{"part0": "A", "part1": "B", "part2": "C", "combine": "ABC"}[path]
				if path == "combine" {
					path = "result"
				}
				actual, err := os.ReadFile(filepath.Join(workspace, path))
				if err != nil || string(actual) != expected {
					t.Fatalf("candidate %s: got %q, err=%v; want %q", node.Task.ID, actual, err, expected)
				}
				submitted[node.Task.ID] = node.Generation
				accept := ControlPlan{Name: g.Name, Action: "accept", TaskID: node.Task.ID, Generation: node.Generation, Evidence: "read exact expected output from current workspace"}
				id := fmt.Sprintf("accept-%s-%d", node.Task.ID, node.Generation)
				if err := m.Add(runtimeOperation(t, id, ControlPlanType, accept)); err != nil {
					t.Fatal(err)
				}
			}
			if op.Status == operation.StatusCompleted {
				if !g.AllAccepted() {
					t.Fatal("goal completed before all acceptance gates")
				}
				return time.Since(start)
			}
		case <-ctx.Done():
			t.Fatalf("accepted goal did not finish: %v; last graph=%+v", ctx.Err(), last)
		}
	}
}
