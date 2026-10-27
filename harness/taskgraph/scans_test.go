package taskgraph

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type scanGate struct {
	phase            string
	entered, release chan struct{}
	once             sync.Once
}

func newScanGate(phase string) *scanGate {
	return &scanGate{phase: phase, entered: make(chan struct{}), release: make(chan struct{})}
}
func (g *scanGate) hook(_ context.Context, phase string) error {
	if phase == g.phase {
		g.once.Do(func() { close(g.entered) })
		<-g.release
	}
	return nil // deliberately ignores cancellation to prove real drain ownership
}
func awaitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker gate")
	}
}
func scanRuntime(t *testing.T, dir string, hook func(context.Context, string) error) (*Runtime, operation.Manager, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r, err := New(ctx, Config{Workspace: dir, BaseDirectory: t.TempDir(), AllowBash: true, scanHook: hook})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
	t.Cleanup(func() {
		cancel()
		ctx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := r.Wait(ctx); err != nil {
			t.Errorf("runtime did not drain: %v", err)
		}
	})
	return r, m, cancel
}
func decodeCheckpoint(t *testing.T, op operation.Operation) checkpoint {
	t.Helper()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	var cp checkpoint
	if err = json.Unmarshal(state.Handle, &cp); err != nil {
		t.Fatal(err)
	}
	return cp
}
func submitControl(t *testing.T, m operation.Manager, id string, p ControlPlan) {
	t.Helper()
	if err := m.Add(runtimeOperation(t, id, ControlPlanType, p)); err != nil {
		t.Fatal(err)
	}
}
func statusNow(t *testing.T, m operation.Manager, id string) operation.Operation {
	t.Helper()
	submitControl(t, m, id, ControlPlan{Name: "g", Action: "status"})
	// runtimeWait has a broad test deadline; additionally assert a narrow latency
	// bound independent of releasing the intentionally blocked scan.
	start := time.Now()
	op := runtimeWait(t, m, func(op operation.Operation) bool { return string(op.ID) == id && terminal(op.Status) })
	if time.Since(start) > time.Second {
		t.Fatal("status was not promptly delivered")
	}
	return op
}
func TestRuntimeStatusResponsiveDuringEveryScanPhase(t *testing.T) {
	for _, phase := range []string{"input_validation", "output_validation", "accept", "accepted_recheck", "recheck"} {
		t.Run(phase, func(t *testing.T) {
			gate := newScanGate(phase)
			defer close(gate.release)
			dir := t.TempDir()
			_, m, _ := scanRuntime(t, dir, gate.hook)
			tasks := []Task{{ID: "a", Command: "printf result > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}}
			if phase == "accepted_recheck" {
				tasks = append(tasks, Task{ID: "b", Command: "cat a > b", Reads: []string{"a"}, Writes: []string{"b"}, DependsOn: []string{"a"}, Acceptance: "inspect"})
			}
			if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})); err != nil {
				t.Fatal(err)
			}
			if phase == "accept" || phase == "accepted_recheck" || phase == "recheck" {
				runtimeWait(t, m, func(op operation.Operation) bool {
					return op.ID == "run" && runtimeNode(t, op, "a").Status == Completed
				})
				submitControl(t, m, "accept", ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "checked"})
			}
			awaitSignal(t, gate.entered)
			status := statusNow(t, m, "status")
			remote, err := operation.DecodeRemoteJobState(status)
			if err != nil {
				t.Fatal(err)
			}
			var diagnostic struct {
				Scanning bool        `json:"scanning"`
				Scan     *scanStatus `json:"scan"`
			}
			if err := json.Unmarshal([]byte(remote.TerminalResult), &diagnostic); err != nil {
				t.Fatal(err)
			}
			wantKind, wantTask := phase, "a"
			if phase == "accepted_recheck" {
				wantKind, wantTask = "input_validation", "b"
			}
			info := diagnostic.Scan
			if !diagnostic.Scanning || info == nil {
				t.Fatal("missing scan diagnostics")
			}
			if info.Graph != "g" || info.TaskID != wantTask || info.Generation != 1 || info.Kind != wantKind || info.Phase != phase || info.StartedAt.IsZero() || info.PhaseStartedAt.IsZero() || info.ElapsedMilliseconds < 0 || info.PhaseElapsedMilliseconds < 0 {
				t.Fatalf("incorrect current scan metadata: %+v", info)
			}
			if strings.Contains(string(remote.Handle), "command_started") || !strings.Contains(string(remote.Handle), "command_dispatched") {
				t.Fatal("untruthful dispatch field")
			}
			cp := decodeCheckpoint(t, status)
			if err := cp.Graph.Validate(); err != nil {
				t.Fatal(err)
			}
			if phase == "input_validation" && cp.Attempts["a"].CommandDispatched {
				t.Fatal("prelaunch scan reported command dispatch")
			}
			// A second snapshot cannot advance the graph or enqueue another worker.
			again := statusNow(t, m, "status-again")
			cp2 := decodeCheckpoint(t, again)
			if cp.Graph.Find("a").Generation != cp2.Graph.Find("a").Generation {
				t.Fatal("snapshot mutated graph")
			}
		})
	}
}
func TestRuntimeCanceledScanRetainsClaimsUntilDrain(t *testing.T) {
	gate := newScanGate("input_validation")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	dir := t.TempDir()
	_, m, _ := scanRuntime(t, dir, gate.hook)
	tasks := []Task{{ID: "a", Command: "printf forbidden > sentinel", Reads: []string{}, Writes: []string{"sentinel"}, Acceptance: "inspect"}, {ID: "b", Command: "cat sentinel", Reads: []string{"sentinel"}, Writes: []string{}, Acceptance: "inspect"}}
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, gate.entered)
	submitControl(t, m, "cancel-task", ControlPlan{Name: "g", Action: "cancel", TaskID: "a"})
	ack := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "cancel-task" && terminal(op.Status) })
	cp := decodeCheckpoint(t, ack)
	if cp.Graph.Find("a").Status != Canceling || cp.Graph.Find("b").Status != Pending {
		t.Fatalf("ownership released early: %+v", cp.Graph)
	}
	revised := tasks[0]
	revised.Command = "true"
	submitControl(t, m, "revise-active", ControlPlan{Name: "g", Action: "revise", Tasks: []Task{revised}})
	failed := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "revise-active" && terminal(op.Status) })
	if failed.Status != operation.StatusFailed {
		t.Fatal("revision replaced a draining reservation")
	}
	submitControl(t, m, "cancel-graph", ControlPlan{Name: "g", Action: "cancel"})
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "cancel-graph" && op.Status == operation.StatusAwaiting
	})
	cp = decodeCheckpoint(t, statusNow(t, m, "draining"))
	if cp.Graph.Find("a").Status != Canceling {
		t.Fatal("scan drain not owned")
	}
	release()
	runDone, controlDone := false, false
	runtimeWait(t, m, func(op operation.Operation) bool {
		runDone = runDone || op.ID == "run" && op.Status == operation.StatusCanceled
		controlDone = controlDone || op.ID == "cancel-graph" && op.Status == operation.StatusCompleted
		return runDone && controlDone
	})
	if _, err := os.Stat(filepath.Join(dir, "sentinel")); !os.IsNotExist(err) {
		t.Fatal("canceled scan launched child", err)
	}
}
func TestRuntimeWaitJoinsCanceledScan(t *testing.T) {
	gate := newScanGate("input_validation")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	dir := t.TempDir()
	r, m, cancel := scanRuntime(t, dir, gate.hook)
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "touch sentinel", Reads: []string{}, Writes: []string{"sentinel"}, Acceptance: "inspect"}}})); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, gate.entered)
	cancel()
	ctx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if err := r.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait returned before scan drained: %v", err)
	}
	release()
	ctx2, stop2 := context.WithTimeout(context.Background(), time.Second)
	defer stop2()
	if err := r.Wait(ctx2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sentinel")); !os.IsNotExist(err) {
		t.Fatal("shutdown launched canceled task")
	}
}
func TestRuntimeAttemptDiagnostics(t *testing.T) {
	for _, kind := range []string{"symlink", "nonzero", "mutation", "success"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			task := Task{ID: "a", Command: "printf ok", Reads: []string{}, Writes: []string{}, Acceptance: "inspect"}
			category := ""
			dispatched := true
			want := Completed
			switch kind {
			case "symlink":
				if err := os.Symlink(t.TempDir(), filepath.Join(dir, "alias")); err != nil {
					t.Fatal(err)
				}
				task.Reads = []string{"."}
				category = "input_validation"
				dispatched = false
				want = Failed
			case "nonzero":
				task.Command = "exit 7"
				category = "execution"
				want = Failed
			case "mutation":
				if err := os.WriteFile(filepath.Join(dir, "input"), []byte("old"), 0600); err != nil {
					t.Fatal(err)
				}
				task.Command = "printf new > input"
				task.Reads = []string{"input"}
				category = "input_mutation"
				want = Failed
			}
			_, m, _ := scanRuntime(t, dir, nil)
			if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})); err != nil {
				t.Fatal(err)
			}
			op := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" && runtimeNode(t, op, "a").Status == want })
			a := decodeCheckpoint(t, op).Attempts["a"]
			if a.CommandDispatched != dispatched || a.FailureCategory != category || a.StartedAt.IsZero() || a.FinishedAt.IsZero() {
				t.Fatalf("incorrect diagnostics: %+v", a)
			}
			if kind == "symlink" && (!strings.Contains(a.FailureReason, "alias") || !strings.Contains(a.FailureReason, "symlink")) {
				t.Fatal(a.FailureReason)
			}
			if kind == "nonzero" && (a.ExitCode == nil || *a.ExitCode != 7) {
				t.Fatalf("missing shell exit code: %+v", a)
			}
			if kind == "success" && a.Phase != "awaiting_acceptance" {
				t.Fatal(a.Phase)
			}
		})
	}
}
func TestRuntimeStatusIsPureSnapshotOfChangedCandidate(t *testing.T) {
	var scans atomic.Int32
	dir := t.TempDir()
	_, m, _ := scanRuntime(t, dir, func(context.Context, string) error { scans.Add(1); return nil })
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "printf old > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}}})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "a").Status == Completed
	})
	count := scans.Load()
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	cp := decodeCheckpoint(t, statusNow(t, m, "snapshot"))
	if scans.Load() != count || cp.Graph.Find("a").Generation != 1 || cp.Graph.Find("a").Status != Completed {
		t.Fatal("status rechecked or advanced")
	}
}
func TestRuntimeAcceptScanCannotAcceptRevisedGeneration(t *testing.T) {
	gate := newScanGate("accept")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	dir := t.TempDir()
	_, m, _ := scanRuntime(t, dir, gate.hook)
	task := Task{ID: "a", Command: "printf old > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "a").Status == Completed
	})
	submitControl(t, m, "accept", ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "old inspected"})
	awaitSignal(t, gate.entered)
	task.Command = "printf revised > a"
	submitControl(t, m, "revise", ControlPlan{Name: "g", Action: "revise", Tasks: []Task{task}})
	runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "revise" && terminal(op.Status) })
	cp := decodeCheckpoint(t, statusNow(t, m, "revised-snapshot"))
	if cp.Graph.Find("a").Generation != 2 || cp.Graph.Find("a").Status != Pending {
		t.Fatal("revision not applied")
	}
	release()
	result := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "accept" && terminal(op.Status) })
	if result.Status != operation.StatusFailed {
		t.Fatal("stale scan accepted revised generation")
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "a").Generation == 2 && runtimeNode(t, op, "a").Status == Completed
	})
}
func TestPlanWarnings(t *testing.T) {
	g := Graph{Nodes: []Node{{Task: Task{ID: "a", Reads: nil, Writes: []string{"a"}}}, {Task: Task{ID: "b", Reads: []string{"a"}, Writes: []string{}}}}}
	text := strings.Join(planWarnings(g), ";")
	if !strings.Contains(text, "whole workspace") || !strings.Contains(text, "conflicting") || !strings.Contains(text, "symlinks") {
		t.Fatal(text)
	}
}

func TestRuntimeOutputScanCancellationRetainsReservation(t *testing.T) {
	gate := newScanGate("output_validation")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	dir := t.TempDir()
	_, m, _ := scanRuntime(t, dir, gate.hook)
	tasks := []Task{{ID: "a", Command: "printf original > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}, {ID: "b", Command: "touch b", Reads: []string{"a"}, Writes: []string{"b"}, Acceptance: "inspect"}}
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: tasks})); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, gate.entered)
	submitControl(t, m, "cancel-task", ControlPlan{Name: "g", Action: "cancel", TaskID: "a"})
	ack := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "cancel-task" && terminal(op.Status) })
	cp := decodeCheckpoint(t, ack)
	if cp.Graph.Find("a").Status != Canceling || cp.Graph.Find("b").Status != Pending {
		t.Fatal("output scan released claims before draining")
	}
	if _, err := os.Stat(filepath.Join(dir, "b")); !os.IsNotExist(err) {
		t.Fatal("conflicting child launched during scan")
	}
	submitControl(t, m, "cancel-graph", ControlPlan{Name: "g", Action: "cancel"})
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "cancel-graph" && op.Status == operation.StatusAwaiting
	})
	release()
	runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" && op.Status == operation.StatusCanceled })
	if _, err := os.Stat(filepath.Join(dir, "b")); !os.IsNotExist(err) {
		t.Fatal("canceled conflicting task launched")
	}
}
func TestRuntimeResumeExplicitPrelaunchCheckpoint(t *testing.T) {
	for _, canceling := range []bool{false, true} {
		t.Run(map[bool]string{false: "run", true: "cancel"}[canceling], func(t *testing.T) {
			dir, base := t.TempDir(), t.TempDir()
			task := Task{ID: "a", Command: "printf resumed > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}
			graph, err := NewGraph("g", []Task{task}, 1)
			if err != nil {
				t.Fatal(err)
			}
			graph.Start("a")
			state := &runtimeState{runtime: &Runtime{config: Config{Workspace: dir, BaseDirectory: base, Shell: "/bin/sh"}}}
			execution := &graphExecution{owner: operation.Operation{ID: "run"}, checkpoint: checkpoint{Graph: graph}}
			child, err := state.newChild(execution, graph.Find("a"))
			if err != nil {
				t.Fatal(err)
			}
			a := &attempt{Generation: 1, ChildOperation: child, Phase: "input_validation"}
			if canceling {
				graph.Cancel("a")
			}
			cp := checkpoint{Version: 1, Graph: graph, Attempts: map[string]*attempt{"a": a}, Canceling: canceling}
			if err := validateAttempts(cp); err != nil {
				t.Fatal(err)
			}
			_, m, _ := scanRuntime(t, dir, nil)
			op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})
			remote, _ := operation.DecodeRemoteJobState(op)
			remote.Handle, _ = json.Marshal(cp)
			step, err := operation.UpdateRemoteJob(op, remote, operation.StatusAwaiting)
			if err != nil {
				t.Fatal(err)
			}
			if err = m.Add(*step.Operation); err != nil {
				t.Fatal(err)
			}
			want := Completed
			if canceling {
				want = Canceled
			}
			runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" && runtimeNode(t, op, "a").Status == want })
			content, err := os.ReadFile(filepath.Join(dir, "a"))
			if canceling {
				if !os.IsNotExist(err) {
					t.Fatal("canceled prelaunch checkpoint dispatched")
				}
			} else if err != nil || string(content) != "resumed" {
				t.Fatalf("prelaunch did not resume: %q %v", content, err)
			}
			a.CommandDispatched = true
			if err := validateAttempts(cp); err == nil {
				t.Fatal("inconsistent dispatch boundary accepted")
			}
		})
	}
}

func TestRuntimeCancelPendingAcceptanceDoesNotAcceptCandidate(t *testing.T) {
	gate := newScanGate("accept")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	dir := t.TempDir()
	_, m, _ := scanRuntime(t, dir, gate.hook)
	task := Task{ID: "a", Command: "printf candidate", Reads: []string{}, Writes: []string{}, Acceptance: "inspect"}
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		return op.ID == "run" && runtimeNode(t, op, "a").Status == Completed
	})
	submitControl(t, m, "accept", ControlPlan{Name: "g", Action: "accept", TaskID: "a", Generation: 1, Evidence: "checked"})
	awaitSignal(t, gate.entered)
	if err := m.Cancel("accept", "review withdrawn"); err != nil {
		t.Fatal(err)
	}
	cp := decodeCheckpoint(t, statusNow(t, m, "during-cancel"))
	if cp.Graph.Find("a").Status != Completed {
		t.Fatal("canceling acceptance changed candidate")
	}
	release()
	result := runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "accept" && terminal(op.Status) })
	if result.Status != operation.StatusCanceled {
		t.Fatalf("canceled acceptance result: %s", result.Status)
	}
	cp = decodeCheckpoint(t, statusNow(t, m, "after-cancel"))
	if cp.Graph.Find("a").Status != Completed || cp.Graph.Find("a").Generation != 1 {
		t.Fatal("canceled scan accepted or invalidated candidate")
	}
}

func TestRuntimeResumeAllAcceptedRechecksBeforeTerminal(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	task := Task{ID: "a", Command: "printf restored > a", Reads: []string{}, Writes: []string{"a"}, Acceptance: "inspect"}
	fingerprintWrite(t, dir, "a", "original")
	fingerprint := fingerprintMust(t, dir, []string{"a"})
	graph, err := NewGraph("g", []Task{task}, 1)
	if err != nil {
		t.Fatal(err)
	}
	graph.Start("a")
	graph.Complete("a", 1, "original", nil)
	graph.Accept("a", 1, "checked original")
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: task.Command, Shell: "/bin/sh", Directory: dir}, base, operation.DefaultMaxOutputLength)
	if err != nil {
		t.Fatal(err)
	}
	cp := checkpoint{Version: 1, Graph: graph, Attempts: map[string]*attempt{"a": {Generation: 1, OutputFingerprint: fingerprint, ChildOperation: operation.Operation{ID: "old-child", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusCompleted, MaxOutputLength: spec.MaxOutputLength}}}}
	fingerprintWrite(t, dir, "a", "changed")
	_, m, _ := scanRuntime(t, dir, nil)
	op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})
	remote, _ := operation.DecodeRemoteJobState(op)
	remote.Handle, _ = json.Marshal(cp)
	step, err := operation.UpdateRemoteJob(op, remote, operation.StatusAwaiting)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Add(*step.Operation); err != nil {
		t.Fatal(err)
	}
	runtimeWait(t, m, func(op operation.Operation) bool {
		if op.ID != "run" {
			return false
		}
		if op.Status == operation.StatusCompleted {
			t.Fatal("resumed stale accepted graph completed without recheck")
		}
		n := runtimeNode(t, op, "a")
		return n.Generation == 2 && n.Status == Completed
	})
}

// This deliberately invokes actor callbacks after cancellation, not after a
// worker returns context.Canceled: select can choose an already successful,
// buffered result over Done. It is the actor's responsibility not to apply it.
type scanCountingManager struct{ adds int }

func (m *scanCountingManager) Add(operation.Operation) error     { m.adds++; return nil }
func (*scanCountingManager) Cancel(operation.ID, string) error   { return nil }
func (*scanCountingManager) Updates() <-chan operation.Operation { return nil }

func canceledScanState(t *testing.T) (*runtimeState, *graphExecution, *scanCountingManager, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r := &Runtime{ctx: ctx, config: Config{Workspace: t.TempDir(), BaseDirectory: t.TempDir(), Shell: "/bin/sh"}, scanResults: make(chan scanResult, 1)}
	r.run, r.control = &runtimeHandler{runtime: r}, &runtimeHandler{runtime: r}
	graph, err := NewGraph("g", []Task{
		{ID: "a", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "inspect"},
		{ID: "b", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "inspect"},
	}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Start("a"); err != nil {
		t.Fatal(err)
	}
	manager := &scanCountingManager{}
	g := &graphExecution{checkpoint: checkpoint{Version: 1, Graph: graph, Attempts: map[string]*attempt{}}, owner: operation.Operation{ID: "run"}, manager: manager, stop: func() {}}
	s := &runtimeState{runtime: r, graphs: map[string]*graphExecution{"g": g}, active: g}
	child, err := s.newChild(g, g.Graph.Find("a"))
	if err != nil {
		t.Fatal(err)
	}
	g.Attempts["a"] = &attempt{Generation: 1, ChildOperation: child, Phase: "input_validation"}
	return s, g, manager, cancel
}
func TestRuntimeShutdownDiscardsSuccessfulQueuedScanResults(t *testing.T) {
	for _, kind := range []string{"input_validation", "output_validation", "accept", "recheck"} {
		t.Run(kind, func(t *testing.T) {
			s, g, manager, cancel := canceledScanState(t)
			a := g.Attempts["a"]
			if kind != "input_validation" {
				a.CommandDispatched = true
				a.ChildOperation.Status = operation.StatusCompleted
				a.Phase = "output_validation"
			}
			if kind == "accept" || kind == "recheck" {
				g.Graph.Complete("a", 1, "candidate", nil)
				a.OutputFingerprint = "observed"
				a.Phase = "awaiting_acceptance"
			}
			if kind == "accept" {
				g.pendingAccept = &acceptRequest{plan: ControlPlan{TaskID: "a", Generation: 1, Evidence: "checked"}}
			}
			if kind == "recheck" {
				g.Graph.Accept("a", 1, "checked")
			}
			before, err := json.Marshal(g.checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			job := &scanJob{g: g, id: "a", generation: 1, kind: kind}
			s.scanning = job
			cancel()
			s.scanned(scanResult{job: job, before: "observed", output: "observed"})
			after, err := json.Marshal(g.checkpoint)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) || manager.adds != 0 || g.terminal || g.finalFresh {
				t.Fatalf("shutdown applied %s: adds=%d before=%s after=%s", kind, manager.adds, before, after)
			}
			if s.scanning != nil {
				t.Fatal("result did not release drain ownership")
			}
			if kind == "accept" && g.pendingAccept == nil {
				t.Fatal("shutdown applied acceptance")
			}
			if len(s.runtime.run.queue) != 0 || len(s.runtime.control.queue) != 0 {
				t.Fatal("shutdown published result")
			}
		})
	}
}
func TestRuntimeShutdownPreventsAdvanceAndNewScan(t *testing.T) {
	for _, trigger := range []string{"advance", "startScan", "child_completion"} {
		t.Run(trigger, func(t *testing.T) {
			s, g, manager, cancel := canceledScanState(t)
			a := g.Attempts["a"]
			if trigger == "child_completion" {
				a.CommandDispatched = true
				a.Phase = "execution"
			}
			cancel()
			switch trigger {
			case "advance":
				s.advance(g)
			case "startScan":
				s.startScan(g, g.Graph.Find("a"), "input_validation")
			case "child_completion":
				child := a.ChildOperation
				child.Status = operation.StatusCompleted
				s.child(child)
			}
			if s.scanning != nil || manager.adds != 0 || g.Graph.Find("b").Status != Pending {
				t.Fatal("shutdown admitted work or started a scan")
			}
		})
	}
}

func TestRuntimeShutdownStopsAllContextsBeforeBlockedScanJoin(t *testing.T) {
	gate := newScanGate("input_validation")
	var once sync.Once
	release := func() { once.Do(func() { close(gate.release) }) }
	defer release()
	r, m, cancel := scanRuntime(t, t.TempDir(), gate.hook)
	if err := m.Add(runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{{ID: "a", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "inspect"}}})); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, gate.entered)
	// Synthetic inactive sibling contexts are independent of r.ctx, just like
	// real nested managers; joining the active scan must not keep them alive.
	stopped := []chan struct{}{make(chan struct{}), make(chan struct{})}
	barrier := make(chan struct{})
	if err := r.request(func(s *runtimeState) {
		for i, name := range []string{"sibling-one", "sibling-two"} {
			updates := make(chan operation.Operation)
			close(updates)
			ch := stopped[i]
			s.graphs[name] = &graphExecution{terminal: true, manager: &lifecycleManager{updates: updates}, stop: func() { close(ch) }}
		}
		close(barrier)
	}); err != nil {
		t.Fatal(err)
	}
	awaitSignal(t, barrier)
	cancel()
	for _, ch := range stopped {
		awaitSignal(t, ch)
	}
	ctx, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer stop()
	if err := r.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait did not retain scan ownership: %v", err)
	}
	release()
	ctx2, stop2 := context.WithTimeout(context.Background(), time.Second)
	defer stop2()
	if err := r.Wait(ctx2); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeScanStatusTimingPureSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, g, _, _ := canceledScanState(t)
		job := &scanJob{g: g, id: "a", generation: 1, kind: "accept", startedAt: time.Now().UTC()}
		job.setProgress("accept")
		s.scanning = job
		time.Sleep(1250 * time.Millisecond)
		first := s.scanStatus()
		if first.ElapsedMilliseconds != 1250 || first.PhaseElapsedMilliseconds != 1250 {
			t.Fatalf("incorrect milliseconds: %+v", first)
		}
		// This is the only worker-written state. Snapshotting must not rewrite it.
		job.setProgress("accepted_recheck")
		progress := job.progress.Load()
		time.Sleep(250 * time.Millisecond)
		second := s.scanStatus()
		if second.ElapsedMilliseconds != 1500 || second.PhaseElapsedMilliseconds != 250 || second.Phase != "accepted_recheck" || second.Kind != "accept" || !second.StartedAt.Equal(first.StartedAt) || !second.PhaseStartedAt.Equal(progress.startedAt) {
			t.Fatalf("incorrect recheck timing: %+v", second)
		}
		if job.progress.Load() != progress || s.scanning != job || g.Graph.Find("a").Status != Running {
			t.Fatal("snapshot mutated progress or graph")
		}
	})
}

func TestCheckpointPrelaunchRejectsNoninitialChildState(t *testing.T) {
	for _, kind := range []string{"shell_phase", "shell_process", "shell_pending_exit", "shell_result", "shell_error", "shell_output", "shell_output_path", "shell_size", "agent_handle", "agent_result", "agent_error", "agent_output_directory", "agent_outstanding_input"} {
		t.Run(kind, func(t *testing.T) {
			s, g, _, _ := canceledScanState(t)
			n := g.Graph.Find("a")
			if strings.HasPrefix(kind, "agent_") {
				n.Task.Command, n.Task.Prompt = "", "fake"
			}
			child, err := s.newChild(g, n)
			if err != nil {
				t.Fatal(err)
			}
			a := g.Attempts["a"]
			a.ChildOperation = child
			if err := validateAttempts(g.checkpoint); err != nil {
				t.Fatalf("initial child rejected: %v", err)
			}
			if strings.HasPrefix(kind, "shell_") {
				state, err := operation.DecodeShellState(child)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "shell_phase":
					state.Phase = operation.ShellPhaseProcess
				case "shell_process":
					state.ProcessGroupID = 42
				case "shell_pending_exit":
					code := 0
					state.PendingExitCode = &code
				case "shell_result":
					state.Result = &operation.ShellResult{}
				case "shell_error":
					state.TerminalError = "failed"
				case "shell_output":
					state.InlineOut = []byte("executed")
				case "shell_output_path":
					state.OutPath = "/tmp/out"
				case "shell_size":
					state.OutSize = 1
				}
				a.ChildOperation.State, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
			} else {
				state, err := operation.DecodeRemoteJobState(child)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "agent_handle":
					state.Handle = []byte(`{"started":true}`)
				case "agent_result":
					state.TerminalResult = "executed"
				case "agent_error":
					state.TerminalError = "failed"
				case "agent_output_directory":
					state.OutputDirectory = "/tmp/out"
				case "agent_outstanding_input":
					state.OutstandingInput = []byte(`{}`)
				}
				a.ChildOperation.State, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := validateAttempts(g.checkpoint); err == nil || !strings.Contains(err.Error(), "prelaunch") {
				t.Fatalf("noninitial %s accepted: %v", kind, err)
			}
		})
	}
}

func TestRuntimeLegacyV1WithoutDiagnosticFieldsRetainsUnknownLaunch(t *testing.T) {
	dir, base := t.TempDir(), t.TempDir()
	task := Task{ID: "a", Command: "touch forbidden", Reads: []string{}, Writes: []string{"forbidden"}, Acceptance: "inspect"}
	graph, err := NewGraph("g", []Task{task}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := graph.Start("a"); err != nil {
		t.Fatal(err)
	}
	s := &runtimeState{runtime: &Runtime{config: Config{Workspace: dir, BaseDirectory: base, Shell: "/bin/sh"}}}
	g := &graphExecution{checkpoint: checkpoint{Graph: graph}, owner: operation.Operation{ID: "run"}}
	child, err := s.newChild(g, graph.Find("a"))
	if err != nil {
		t.Fatal(err)
	}
	// Deliberately use only the OLD v1 shape, not an attempt struct with zeroed
	// diagnostics (which would still serialize command_dispatched:false).
	raw, err := json.Marshal(map[string]any{"version": 1, "graph": graph, "attempts": map[string]any{"a": map[string]any{"generation": 1, "child_operation": child, "before_reads": ""}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "command_dispatched") || strings.Contains(string(raw), "phase") {
		t.Fatal("not an old checkpoint")
	}
	_, m, _ := scanRuntime(t, dir, nil)
	op := runtimeOperation(t, "run", RunPlanType, RunPlan{Name: "g", Tasks: []Task{task}})
	remote, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	remote.Handle = raw
	step, err := operation.UpdateRemoteJob(op, remote, operation.StatusAwaiting)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Add(*step.Operation); err != nil {
		t.Fatal(err)
	}
	op = runtimeWait(t, m, func(op operation.Operation) bool { return op.ID == "run" })
	cp := decodeCheckpoint(t, op)
	a := cp.Attempts["a"]
	if cp.Graph.Find("a").Status != Running || a.Blocker == "" || !a.CommandDispatched {
		t.Fatalf("old checkpoint lost reservation: %+v", a)
	}
	if _, err := os.Stat(filepath.Join(dir, "forbidden")); !os.IsNotExist(err) {
		t.Fatal("old Ready child replayed")
	}
}
