package workerruntime

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func m27Operation(t *testing.T, id operation.ID, kind operation.RemoteJobPlanType, version operation.RemoteJobPlanVersion, plan any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: kind, Version: version, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: id, Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
}
func m27Run(t *testing.T) operation.Operation {
	t.Helper()
	return m27Operation(t, "graph", taskgraph.RunPlanType, taskgraph.DurablePlanVersion, taskgraph.DurableRunPlan{Version: 1, ParentID: "parent", Run: taskgraph.RunPlan{Name: "graph", Tasks: []taskgraph.Task{{ID: "a", Command: "printf 'once\\n' >> sentinel", Reads: []string{}, Writes: []string{"sentinel"}, Acceptance: "inspect sentinel"}}}})
}
func m27Bootstrap(t *testing.T) workerproto.Bootstrap {
	t.Helper()
	b := workerproto.Bootstrap{SessionID: "parent", ControlScope: "control", DurableGraphs: true, Directory: t.TempDir(), Workspace: t.TempDir(), ChildRuntimeDirectory: t.TempDir(), DurableChildCommand: []string{"/bin/false"}}
	for _, directory := range []string{b.Directory, b.Workspace, b.ChildRuntimeDirectory} {
		if err := os.Chmod(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(b.Directory, "operations", string(b.SessionID)), 0700); err != nil {
		t.Fatal(err)
	}
	return b
}
func m27History(ops ...operation.Operation) sessionstore.LifecycleHistory {
	retained := map[operation.ID]operation.Operation{}
	for _, op := range ops {
		retained[op.ID] = op
	}
	return sessionstore.LifecycleHistory{LastRemoteSequence: 1, RetainedRemoteOperations: retained}
}
func m27Edit(t *testing.T, op operation.Operation, edit func(*operation.RemoteJobState)) operation.Operation {
	t.Helper()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	edit(&state)
	op.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return op
}
func m27Control(t *testing.T, parent session.ID) operation.Operation {
	t.Helper()
	return m27Operation(t, "control", taskgraph.ControlPlanType, taskgraph.DurablePlanVersion, taskgraph.DurableControlPlan{Version: 1, ParentID: parent, Scope: "scope", Control: taskgraph.ControlPlan{Name: "graph", Action: "status"}})
}
func TestM27DurableOwnershipDiscriminatesGraphAndChild(t *testing.T) {
	b := m27Bootstrap(t)
	graph := m27Run(t)
	if err := validateDurableOwnership(t.Context(), b, m27History(graph), nil); err != nil {
		t.Fatal(err)
	}
	disabled := b
	disabled.DurableGraphs = false
	if err := validateDurableOwnership(t.Context(), disabled, m27History(graph), nil); err == nil {
		t.Fatal("graph opt-in bypassed")
	}
	child := m27Operation(t, "child-control", toolsubagent.ControlPlanType, toolsubagent.PlanVersion, toolsubagent.ControlPlan{Action: "list"})
	child.Status = operation.StatusCompleted
	mixed := m27History(graph, child)
	if err := validateDurableOwnership(t.Context(), b, mixed, nil); err == nil {
		t.Fatal("graph opt-in authorized a subagent owner")
	}
	b.DurableSubagents = true
	childRun := m27Operation(t, "child-run", toolsubagent.RunPlanType, toolsubagent.DurablePlanVersion, toolsubagent.DurableRunPlan{Version: 1, ParentID: "parent", Run: toolsubagent.RunPlan{Name: "helper", Prompt: "inspect only", Lifetime: "background"}})
	if err := validateDurableOwnership(t.Context(), b, m27History(graph, childRun), nil); err != nil {
		t.Fatalf("mixed graph and unsubmitted independently owned child: %v", err)
	}
	if _, err := os.Stat(filepath.Join(b.Directory, "durable-children")); !os.IsNotExist(err) {
		t.Fatalf("reconciliation created a child reservation: %v", err)
	}
	if err := validateDurableOwnership(t.Context(), b, mixed, nil); err != nil {
		t.Fatalf("separate graph and child controls: %v", err)
	}
	impostor := m27Edit(t, graph, func(s *operation.RemoteJobState) { s.Plan.Type = toolsubagent.RunPlanType })
	if err := validateDurableOwnership(t.Context(), b, m27History(impostor), nil); err == nil {
		t.Fatal("graph payload classified as child owner")
	}
	stale := m27Edit(t, graph, func(s *operation.RemoteJobState) {
		s.Plan.Data = []byte(`{"version":1,"parent_id":"other","run":{"name":"graph","tasks":[{"id":"a","command":"true","acceptance":"checked"}]}}`)
	})
	if err := validateDurableOwnership(t.Context(), b, m27History(graph), []operation.Operation{stale}); err == nil {
		t.Fatal("latest parent mismatch ignored")
	}
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		control := m27Control(t, "parent")
		control.Status = status
		err := validateDurableOwnership(t.Context(), b, m27History(control), nil)
		terminal := status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled
		if (err == nil) != terminal {
			t.Fatalf("control %s: %v", status, err)
		}
	}
	control := m27Control(t, "other")
	control.Status = operation.StatusCompleted
	if err := validateDurableOwnership(t.Context(), b, m27History(control), nil); err == nil {
		t.Fatal("terminal control parent mismatch authorized")
	}
	malformed := m27Control(t, "parent")
	malformed.Status = operation.StatusCompleted
	malformed = m27Edit(t, malformed, func(s *operation.RemoteJobState) {
		s.Plan.Data = append(append([]byte{}, s.Plan.Data[:len(s.Plan.Data)-1]...), []byte(`,"unknown":true}`)...)
	})
	if err := validateDurableOwnership(t.Context(), b, m27History(malformed), nil); err == nil {
		t.Fatal("malformed terminal control ownership authorized")
	}
}

func TestM27DurableOwnershipStrictEnvelopeAndVersions(t *testing.T) {
	b := m27Bootstrap(t)
	for _, version := range []operation.RemoteJobPlanVersion{0, 1, 3, 99} {
		op := m27Edit(t, m27Run(t), func(s *operation.RemoteJobState) { s.Plan.Version = version })
		if err := validateDurableOwnership(t.Context(), b, m27History(op), nil); err == nil {
			t.Fatalf("plan version %d admitted", version)
		}
	}
	for _, data := range []string{`{}`, `null`, `{"version":2,"parent_id":"parent","run":{}}`, `{"version":1,"parent_id":"parent","run":{},"unknown":true}`} {
		op := m27Edit(t, m27Run(t), func(s *operation.RemoteJobState) { s.Plan.Data = []byte(data) })
		if err := validateDurableOwnership(t.Context(), b, m27History(op), nil); err == nil {
			t.Fatalf("malformed run admitted: %s", data)
		}
	}
}

func TestM27DurableExecutionPinnedConfiguration(t *testing.T) {
	b := m27Bootstrap(t)
	op := m27Run(t)
	plan, _, err := taskgraph.DecodeDurableRun(op)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := taskgraph.NewGraph(plan.Run.Name, plan.Run.Tasks, plan.Run.Concurrency)
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := filepath.EvalSymlinks(b.Workspace)
	if err != nil {
		t.Fatal(err)
	}
	expected := taskgraph.DurableExecution{Workspace: workspace, Shell: "/bin/sh", BaseDirectory: filepath.Join(b.Directory, "operations", string(b.SessionID))}
	withExecution := func(execution taskgraph.DurableExecution) operation.Operation {
		return m27Edit(t, op, func(s *operation.RemoteJobState) {
			s.Handle, err = json.Marshal(map[string]any{"version": 2, "parent_id": "parent", "scope": "scope", "execution": execution, "state": map[string]any{"version": 1, "graph": graph, "attempts": map[string]any{}}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	pinned := withExecution(expected)
	if err := validateDurableOwnership(t.Context(), b, m27History(pinned), nil); err != nil {
		t.Fatalf("valid pinned checkpoint: %v", err)
	}
	for _, field := range []string{"workspace", "shell", "base"} {
		conflict := expected
		switch field {
		case "workspace":
			conflict.Workspace = t.TempDir()
		case "shell":
			conflict.Shell = "/bin/false"
		case "base":
			conflict.BaseDirectory = t.TempDir()
		}
		if err := validateDurableOwnership(t.Context(), b, m27History(withExecution(conflict)), nil); err == nil {
			t.Fatalf("%s conflict admitted", field)
		}
	}
	wrongParent := m27Edit(t, pinned, func(s *operation.RemoteJobState) {
		var cp taskgraph.DurableCheckpoint
		if err := json.Unmarshal(s.Handle, &cp); err != nil {
			t.Fatal(err)
		}
		cp.ParentID = "other"
		s.Handle, err = json.Marshal(cp)
		if err != nil {
			t.Fatal(err)
		}
	})
	if err := validateDurableOwnership(t.Context(), b, m27History(wrongParent), nil); err == nil {
		t.Fatal("checkpoint parent mismatch admitted")
	}
}

// Embedding is intentional: this spy exposes only the coordinator's save
// boundary. No session, provider, user output, or other store method is used.
type m27CheckpointSpy struct {
	sessionstore.Store
	fail  error
	calls int
	saved operation.Operation
	path  string
}

func (s *m27CheckpointSpy) SaveOperation(_ context.Context, id session.ID, op operation.Operation) error {
	s.calls++
	if id != "parent" {
		return errors.New("wrong session")
	}
	if s.fail != nil {
		return s.fail
	}
	data, err := json.Marshal(op)
	if err != nil {
		return err
	}
	if err = os.WriteFile(s.path, data, 0600); err != nil {
		return err
	}
	data, err = os.ReadFile(s.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.saved)
}

func m27Next(t *testing.T, updates <-chan operation.Operation, predicate func(operation.Operation) bool) operation.Operation {
	t.Helper()
	timer := time.NewTimer(4 * time.Second)
	defer timer.Stop()
	for {
		select {
		case op, ok := <-updates:
			if !ok {
				t.Fatal("runtime updates closed before expected checkpoint")
			}
			if predicate(op) {
				return op
			}
		case <-timer.C:
			t.Fatal("expected real runtime checkpoint not observed")
		}
	}
}
func TestM27CommittedCheckpointStoreFencesRealRuntimeDispatch(t *testing.T) {
	for _, failure := range []bool{true, false} {
		name := "successful save releases once"
		if failure {
			name = "failed save never dispatches"
		}
		t.Run(name, func(t *testing.T) {
			b := m27Bootstrap(t)
			ctx, cancel := context.WithCancel(context.Background())
			r, err := taskgraph.New(ctx, taskgraph.Config{Workspace: b.Workspace, BaseDirectory: filepath.Join(b.Directory, "operations", string(b.SessionID)), Shell: "/bin/sh", AllowBash: true, DurableParentID: b.SessionID, DurableScope: "scope"})
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			joined := false
			defer func() {
				cancel()
				if !joined {
					wait, stop := context.WithTimeout(context.Background(), 4*time.Second)
					defer stop()
					if err := r.Wait(wait); err != nil {
						t.Errorf("runtime/nested manager join: %v", err)
					}
				}
			}()
			// Capture actual handler updates, not a synthetic dispatcher or mock runtime.
			updates := r.Handlers()[0].RemoteJobUpdates()
			if err := r.Handlers()[0].AddRemoteJob(m27Run(t)); err != nil {
				t.Fatal(err)
			}
			reserved := m27Next(t, updates, func(op operation.Operation) bool {
				_, cp, err := taskgraph.DecodeDurableRun(op)
				if err != nil {
					t.Fatalf("runtime emitted invalid checkpoint: %v", err)
				}
				if cp == nil {
					return false
				}
				a := cp.State.Attempts["a"]
				return a != nil && a.CommandDispatched && a.Phase == "execution" && a.ChildOperation.Status == operation.StatusReady
			})
			if _, err := os.Stat(filepath.Join(b.Workspace, "sentinel")); !os.IsNotExist(err) {
				t.Fatalf("dispatch before save: %v", err)
			}
			saveErr := errors.New("injected durable save failure")
			spy := &m27CheckpointSpy{path: filepath.Join(t.TempDir(), "checkpoint.json")}
			if failure {
				spy.fail = saveErr
			}
			store := committedCheckpointStore{Store: spy, owners: &ownedExecution{runtimes: []*taskgraph.Runtime{r}}}
			err = store.SaveOperation(t.Context(), b.SessionID, reserved)
			if failure {
				if !errors.Is(err, saveErr) {
					t.Fatalf("save error not preserved: %v", err)
				}
				// A genuine status round trip orders the observation after SaveOperation.
				if err := r.Handlers()[1].AddRemoteJob(m27Control(t, "parent")); err != nil {
					t.Fatal(err)
				}
				status := m27Next(t, r.Handlers()[1].RemoteJobUpdates(), func(op operation.Operation) bool { return op.ID == "control" })
				state, err := operation.DecodeRemoteJobState(status)
				if err != nil {
					t.Fatal(err)
				}
				if status.Status != operation.StatusCompleted {
					t.Fatalf("status barrier failed: %s %s", status.Status, state.TerminalError)
				}
				// Keep the real actor and manager live briefly after the ordered
				// barrier, so cancellation cannot hide an accidentally queued ack.
				// The final joined sentinel check below also rules out late launch.
				observe := time.NewTimer(50 * time.Millisecond)
				tick := time.NewTicker(time.Millisecond)
				for observing := true; observing; {
					if _, err := os.Stat(filepath.Join(b.Workspace, "sentinel")); !os.IsNotExist(err) {
						tick.Stop()
						observe.Stop()
						t.Fatalf("failed save released live runtime reservation: %v", err)
					}
					select {
					case <-observe.C:
						observing = false
					case <-tick.C:
					}
				}
				tick.Stop()
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(spy.saved, reserved) {
					t.Fatal("save did not retain exact reserved checkpoint")
				}
				// Duplicate durable acknowledgments cannot release the slot twice.
				if err := store.SaveOperation(t.Context(), b.SessionID, reserved); err != nil {
					t.Fatal(err)
				}
				m27Next(t, updates, func(op operation.Operation) bool {
					_, cp, err := taskgraph.DecodeDurableRun(op)
					if err != nil {
						t.Fatal(err)
					}
					return cp != nil && cp.State.Graph.Find("a").Status == taskgraph.Completed
				})
			}
			cancel()
			wait, stop := context.WithTimeout(context.Background(), 4*time.Second)
			defer stop()
			if err := r.Wait(wait); err != nil {
				t.Fatalf("actual runtime/nested managers did not join: %v", err)
			}
			joined = true
			// Wait includes nested manager AwaitStopped: no late launch can follow this check.
			data, err := os.ReadFile(filepath.Join(b.Workspace, "sentinel"))
			if failure {
				if !os.IsNotExist(err) {
					t.Fatalf("failed save dispatched actual shell: %q %v", data, err)
				}
				if spy.calls != 1 {
					t.Fatalf("save calls=%d", spy.calls)
				}
			} else {
				if err != nil || string(data) != "once\n" {
					t.Fatalf("real command must run exactly once: %q %v", data, err)
				}
				if spy.calls != 2 {
					t.Fatalf("save calls=%d", spy.calls)
				}
			}
		})
	}
}
