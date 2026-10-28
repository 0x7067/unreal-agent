package workerhost

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const graphRecoveryRun operation.ID = "graph-recovery-original"
const graphRecoveryName = "recover-candidates"

func graphRecoveryOperation(t *testing.T, id operation.ID, typ operation.RemoteJobPlanType, plan any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: taskgraph.DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: id, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
}

func graphRecoveryCheckpoint(op operation.Operation) (*taskgraph.DurableCheckpoint, error) {
	_, cp, err := taskgraph.DecodeDurableRun(op)
	if err == nil && cp == nil {
		err = fmt.Errorf("missing durable checkpoint")
	}
	return cp, err
}

// Resume intentionally omits finished operations. Once the run terminalizes,
// the canonical ToolCallStatus, not an obsolete unfinished checkpoint, is truth.
func graphRecoveryLatest(t *testing.T, dir string) (operation.Operation, *taskgraph.DurableCheckpoint) {
	t.Helper()
	store, err := localfile.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := store.Resume(t.Context(), m21Session)
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range restored.Operations {
		if op.ID == graphRecoveryRun {
			cp, err := graphRecoveryCheckpoint(op)
			if err != nil {
				t.Fatal(err)
			}
			return op, cp
		}
	}
	var latest operation.Operation
	for _, item := range m21ReadItems(t, dir) {
		if s, ok := item.Data.(sessionstore.ToolCallStatus); ok {
			for _, op := range s.Operations {
				if op.ID == graphRecoveryRun {
					latest = op
				}
			}
		}
	}
	cp, err := graphRecoveryCheckpoint(latest)
	if err != nil {
		t.Fatal(err)
	}
	return latest, cp
}

func graphRecoveryCounters(t *testing.T, workspace string, d bool) {
	t.Helper()
	for _, id := range []string{"A", "B", "C", "D"} {
		data, err := os.ReadFile(filepath.Join(workspace, id+"-counter"))
		if id == "D" && !d {
			if !os.IsNotExist(err) {
				t.Fatalf("blocked D ran: %q %v", data, err)
			}
		} else if err != nil || string(data) != "x" {
			t.Fatalf("%s must execute exactly once: %q %v", id, data, err)
		}
	}
}

func graphRecoveryCandidates(t *testing.T, cp *taskgraph.DurableCheckpoint) {
	t.Helper()
	for id, want := range map[string]string{"A": taskgraph.Accepted, "B": taskgraph.Completed, "C": taskgraph.Completed, "D": taskgraph.Pending} {
		n := cp.State.Graph.Find(id)
		if n == nil || n.Status != want || n.Generation != 1 || n.Stale {
			t.Fatalf("%s candidate changed: %+v", id, n)
		}
	}
	if cp.State.Graph.Find("A").Evidence != "actual A counter inspected" {
		t.Fatal("accepted A evidence lost")
	}
}

// This is a controlled persistence fixture, NOT a live internal crash hook.
// All child operations/results come from a real Runtime and real shell manager.
// Only after their actual terminal receipts and Runtime.Wait do we rewind C's
// reducer/scan boundary to output_validation, retaining its terminal child.
func graphRecoveryFixture(t *testing.T, dir string) *taskgraph.DurableCheckpoint {
	t.Helper()
	store, err := localfile.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	tasks := []taskgraph.Task{}
	for _, id := range []string{"A", "B", "C", "D"} {
		task := taskgraph.Task{ID: id, Command: "printf x >> " + id + "-counter", Reads: []string{}, Writes: []string{id + "-counter"}, Acceptance: "inspect exact single-byte counter"}
		if id == "D" {
			task.DependsOn = []string{"B"}
		}
		tasks = append(tasks, task)
	}
	plan := taskgraph.DurableRunPlan{Version: 1, ParentID: m21Session, Run: taskgraph.RunPlan{Name: graphRecoveryName, Tasks: tasks, Concurrency: 3}}
	original := graphRecoveryOperation(t, graphRecoveryRun, taskgraph.RunPlanType, plan)
	const turn session.TurnID = "graph-recovery-turn"
	if err = store.AppendTurn(t.Context(), m21Session, session.Turn{ID: turn, Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]any{"action": "start", "name": graphRecoveryName, "tasks": tasks, "concurrency": 3})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: turn, Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "graph-original-call", Name: tool.TaskGraphName, Arguments: string(args)}}}}}); err != nil {
		t.Fatal(err)
	}
	if err = store.AppendToolCallStatus(t.Context(), m21Session, sessionstore.ToolCallStatus{TurnID: turn, CallID: "graph-original-call", Status: tool.CallStatus{WaitingFor: []operation.ID{original.ID}}, Operations: []operation.Operation{original}}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	base := filepath.Join(dir, "operations", string(m21Session))
	if err := os.MkdirAll(base, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := taskgraph.New(ctx, taskgraph.Config{Workspace: dir, BaseDirectory: base, Shell: "/bin/sh", AllowBash: true, DurableParentID: m21Session, DurableScope: "fixture-scope"})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	handlers := r.Handlers()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			join, done := context.WithTimeout(context.Background(), 5*time.Second)
			defer done()
			if err := r.Wait(join); err != nil {
				t.Errorf("actual nested manager join: %v", err)
			}
			for _, h := range handlers {
				for range h.RemoteJobUpdates() {
				}
			}
		})
	}
	defer stop()
	if err = handlers[0].AddRemoteJob(original); err != nil {
		t.Fatal(err)
	}
	accepted := false
	var chosen operation.Operation
	var clone taskgraph.DurableCheckpoint
	for chosen.ID == "" {
		select {
		case op := <-handlers[0].RemoteJobUpdates():
			if err = store.SaveOperation(t.Context(), m21Session, op); err != nil {
				t.Fatal(err)
			}
			if err = r.NotifyCheckpointCommitted(op); err != nil {
				t.Fatal(err)
			}
			cp, err := graphRecoveryCheckpoint(op)
			if err != nil {
				t.Fatal(err)
			}
			if !accepted && cp.State.Graph.Find("A").Status == taskgraph.Completed {
				c := graphRecoveryOperation(t, "accept-original-A", taskgraph.ControlPlanType, taskgraph.DurableControlPlan{Version: 1, ParentID: m21Session, Scope: "fixture-scope", Control: taskgraph.ControlPlan{Name: graphRecoveryName, Action: "accept", TaskID: "A", Generation: 1, Evidence: "actual A counter inspected"}})
				if err = handlers[1].AddRemoteJob(c); err != nil {
					t.Fatal(err)
				}
				accepted = true
			}
			if cp.State.Graph.Find("A").Status == taskgraph.Accepted && cp.State.Graph.Find("B").Status == taskgraph.Completed && cp.State.Graph.Find("C").Status == taskgraph.Completed {
				data, err := json.Marshal(cp)
				if err != nil {
					t.Fatal(err)
				}
				if err = json.Unmarshal(data, &clone); err != nil {
					t.Fatal(err)
				}
				chosen = op
			}
		case op := <-handlers[1].RemoteJobUpdates():
			if op.Status == operation.StatusFailed {
				t.Fatalf("accept A failed: %+v", op)
			}
		case <-ctx.Done():
			t.Fatal("real fixture commands did not complete")
		}
	}
	// A stale owner scope must fail in the actual actor without accepting B.
	stale := graphRecoveryOperation(t, "stale-scope-B", taskgraph.ControlPlanType, taskgraph.DurableControlPlan{Version: 1, ParentID: m21Session, Scope: "retired-scope", Control: taskgraph.ControlPlan{Name: graphRecoveryName, Action: "accept", TaskID: "B", Generation: 1, Evidence: "must not mutate"}})
	if err = handlers[1].AddRemoteJob(stale); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case op := <-handlers[1].RemoteJobUpdates():
			if op.ID != stale.ID {
				continue
			}
			state, err := operation.DecodeRemoteJobState(op)
			if err != nil || op.Status != operation.StatusFailed || !strings.Contains(state.TerminalError, "scope") {
				t.Fatalf("stale scope admitted: %+v %v", state, err)
			}
			goto scopeRejected
		case op := <-handlers[0].RemoteJobUpdates():
			cp, err := graphRecoveryCheckpoint(op)
			if err != nil {
				t.Fatal(err)
			}
			graphRecoveryCandidates(t, cp)
		case <-ctx.Done():
			t.Fatal("stale scope check timed out")
		}
	}
scopeRejected:
	// Status is a FIFO actor observation after the failed control.
	status := graphRecoveryOperation(t, "observe-stale-scope", taskgraph.ControlPlanType, taskgraph.DurableControlPlan{Version: 1, ParentID: m21Session, Scope: "fixture-scope", Control: taskgraph.ControlPlan{Name: graphRecoveryName, Action: "status"}})
	if err = handlers[1].AddRemoteJob(status); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case op := <-handlers[1].RemoteJobUpdates():
			if op.ID == status.ID {
				state, err := operation.DecodeRemoteJobState(op)
				if err != nil || op.Status != operation.StatusCompleted {
					t.Fatalf("status after stale scope: %+v %v", state, err)
				}
				var snapshot struct {
					Graph taskgraph.Graph `json:"graph"`
				}
				if err = json.Unmarshal([]byte(state.TerminalResult), &snapshot); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(snapshot.Graph, clone.State.Graph) {
					t.Fatal("stale scope mutated reducer state")
				}
				goto observed
			}
		case <-handlers[0].RemoteJobUpdates():
		case <-ctx.Done():
			t.Fatal("scope observation timed out")
		}
	}
observed:
	stop()
	graphRecoveryCounters(t, dir, false)
	graphRecoveryCandidates(t, &clone)
	for _, id := range []string{"A", "B", "C"} {
		child := clone.State.Attempts[id].ChildOperation
		shell, err := operation.DecodeShellState(child)
		if err != nil || child.Status != operation.StatusCompleted || shell.Result == nil || shell.Result.ExitCode != 0 || shell.ProcessGroupID != 0 || shell.Phase != "" {
			t.Fatalf("%s lacks actual shell termination: %+v %v", id, shell, err)
		}
	}
	n := clone.State.Graph.Find("C")
	n.Status = taskgraph.Running
	n.Result = ""
	n.Evidence = ""
	clone.State.Attempts["C"].Phase = "output_validation"
	if err = clone.State.Graph.Validate(); err != nil {
		t.Fatal(err)
	}
	remote, err := operation.DecodeRemoteJobState(chosen)
	if err != nil {
		t.Fatal(err)
	}
	remote.Handle, err = json.Marshal(clone)
	if err != nil {
		t.Fatal(err)
	}
	chosen.State, err = json.Marshal(remote)
	if err != nil {
		t.Fatal(err)
	}
	if err = taskgraph.ValidateDurableRecovery(chosen, m21Session); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveOperation(t.Context(), m21Session, chosen); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func TestM27DurableGraphWorkerRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	firstBlocked, firstJoined := make(chan struct{}), make(chan struct{})
	freshBlocked, freshRelease := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once

	var calls atomic.Int64
	var sawCandidate, sawStale atomic.Bool
	var dir string
	var requestsMu sync.Mutex
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		requestsMu.Lock()
		var reqParts map[string]jsontext.Value
		_ = json.Unmarshal(body, &reqParts)
		requests = append(requests, string(reqParts["input"]))
		requestsMu.Unlock()
		text := string(body)
		if strings.Contains(text, "B") && strings.Contains(text, "completed") {
			sawCandidate.Store(true)
		}
		if strings.Contains(text, "generation") && (strings.Contains(text, "stale") || strings.Contains(text, "Error:")) {
			sawStale.Store(true)
		}
		action := func(task string, gen uint64) {
			completed(w, id, toolCall(id, "TaskGraph", map[string]any{"action": "accept", "name": graphRecoveryName, "task_id": task, "generation": gen, "evidence": "counter inspected; current generation"}))
		}
		switch id {
		case 1:
			completed(w, id, toolCall(id, "TaskGraph", map[string]any{"action": "status", "name": graphRecoveryName}))
		case 2:
			action("B", 999)
		case 3:
			close(firstBlocked)
			<-r.Context().Done()
			close(firstJoined)
		case 4:
			close(freshBlocked)
			select {
			case <-freshRelease:
				action("B", 1)
			case <-r.Context().Done():
			}
		default:
			// Runtime notifications can interrupt a provider request. Select
			// only from the current durable candidate; never infer that a
			// previous HTTP response's tool call actually committed.
			if id > 32 {
				t.Error("bounded candidate acceptance request count exceeded")
				http.Error(w, "candidate acceptance stalled", 500)
				return
			}
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for {
				_, cp := graphRecoveryLatest(t, dir)
				for _, task := range []string{"B", "C", "D"} {
					if cp.State.Graph.Find(task).Status == taskgraph.Completed {
						action(task, 1)
						return
					}
				}
				if cp.State.Graph.AllAccepted() {
					completed(w, id, assistant(id, "all candidates explicitly accepted"))
					return
				}
				select {
				case <-ticker.C:
				case <-r.Context().Done():
					return
				}
			}
		}
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(freshRelease) })
	cfg := m23Config(t, server.URL)
	cfg.Bootstrap.DurableGraphs = true
	cfg.Bootstrap.Subagents = false
	dir = cfg.Bootstrap.Directory
	defer func() {
		if t.Failed() {
			t.Logf("provider calls=%d", calls.Load())
			_, cp := graphRecoveryLatest(t, dir)
			for _, n := range cp.State.Graph.Nodes {
				t.Logf("LASTNODE %s status=%s gen=%d stale=%t attempt=%+v", n.Task.ID, n.Status, n.Generation, n.Stale, cp.State.Attempts[n.Task.ID])
			}
			requestsMu.Lock()
			defer requestsMu.Unlock()
			for i, req := range requests {
				if len(req) > 2200 {
					req = req[len(req)-2200:]
				}
				t.Logf("REQUEST %d tail=%s", i+1, req)
			}
		}
	}()
	original := graphRecoveryFixture(t, dir)
	h1, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	h1Closed := false
	defer func() {
		if !h1Closed {
			m23Close(t, h1, dir)
		}
	}()
	// Recovery notifications normally wake the parent; a regular input also
	// ensures this proof does not depend on the producer having an inbox writer.
	send(t, h1, "graph-recovery-observe", "Inspect recovered TaskGraph candidates")
	select {
	case <-firstBlocked:
	case <-ctx.Done():
		t.Fatal("first parent did not reach stale-accept barrier")
	}
	_, before := graphRecoveryLatest(t, dir)
	graphRecoveryCandidates(t, before)
	graphRecoveryCounters(t, dir, false)
	if before.Scope == original.Scope {
		t.Fatal("first worker did not establish fresh durable scope")
	}
	if err = h1.Kill(); err != nil {
		t.Fatal(err)
	}
	m23Close(t, h1, dir)
	h1Closed = true
	select {
	case <-firstJoined:
	case <-ctx.Done():
		t.Fatal("killed worker HTTP cancellation not joined")
	}
	h2, err := Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer m23Close(t, h2, dir)
	select {
	case <-freshBlocked:
	case <-ctx.Done():
		t.Fatal("replacement did not reach fresh acceptance barrier")
	}
	// Provider admission and durable graph-head persistence are separate
	// actor events. Require the fresh saved scope, not a timing assumption.
	var after *taskgraph.DurableCheckpoint
	freshTick := time.NewTicker(5 * time.Millisecond)
	defer freshTick.Stop()
	for {
		_, after = graphRecoveryLatest(t, dir)
		if after.Scope != before.Scope {
			break
		}
		select {
		case <-freshTick.C:
		case <-ctx.Done():
			t.Fatal("replacement did not durably establish fresh scope")
		}
	}
	graphRecoveryCandidates(t, after)
	graphRecoveryCounters(t, dir, false)
	if after.Scope == before.Scope {
		t.Fatal("replacement reused retired scope")
	}
	if !reflect.DeepEqual(before.State.Graph, after.State.Graph) {
		t.Fatal("replacement mutated stable candidate reducer state")
	}
	for _, id := range []string{"A", "B", "C"} {
		if !reflect.DeepEqual(before.State.Attempts[id].ChildOperation, after.State.Attempts[id].ChildOperation) {
			t.Fatalf("replacement changed %s terminal child receipt", id)
		}
	}
	releaseOnce.Do(func() { close(freshRelease) })
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		op, cp := graphRecoveryLatest(t, dir)
		if op.Status == operation.StatusCompleted && cp.State.Graph.AllAccepted() {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			for _, n := range cp.State.Graph.Nodes {
				t.Logf("TIMEOUTNODE %s status=%s gen=%d stale=%t", n.Task.ID, n.Status, n.Generation, n.Stale)
			}
			t.Logf("TIMEOUTROOT status=%s", op.Status)
			t.Fatal("recovered graph did not terminalize after explicit accepts")
		}
	}
	if err = h2.Close(ctx); err != nil {
		t.Fatal(err)
	}
	graphRecoveryCounters(t, dir, true)
	terminal := 0
	for _, item := range m21ReadItems(t, dir) {
		if s, ok := item.Data.(sessionstore.ToolCallStatus); ok {
			for _, op := range s.Operations {
				if op.ID == graphRecoveryRun && op.Status == operation.StatusCompleted {
					terminal++
				}
			}
		}
	}
	if terminal != 1 {
		t.Fatalf("parent canonical run completions=%d, want one", terminal)
	}
	if !sawCandidate.Load() || !sawStale.Load() {
		requestsMu.Lock()
		defer requestsMu.Unlock()
		t.Fatalf("parent did not observe candidate/stale failure: candidate=%v stale=%v requests=%v", sawCandidate.Load(), sawStale.Load(), requests)
	}
}
