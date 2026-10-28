package lifecycleproof

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

func m27ManagedGraph(t *testing.T, control bool) operation.Operation {
	t.Helper()
	typ := taskgraph.RunPlanType
	var plan any = taskgraph.DurableRunPlan{Version: 1, ParentID: "parent", Run: taskgraph.RunPlan{Name: "graph", Tasks: []taskgraph.Task{{ID: "a", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "checked"}}}}
	if control {
		typ = taskgraph.ControlPlanType
		plan = taskgraph.DurableControlPlan{Version: 1, ParentID: "parent", Scope: "scope", Control: taskgraph.ControlPlan{Name: "graph", Action: "status"}}
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: taskgraph.DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "graph", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusAwaiting}
}

func TestM27JoinedGraphRequiresStrictExactCurrentCapability(t *testing.T) {
	for _, control := range []bool{false, true} {
		op := m27ManagedGraph(t, control)
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			t.Fatal(err)
		}
		evidence := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
		for _, version := range []operation.RemoteJobPlanVersion{0, 1, 2, 3} {
			joined := map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{state.Plan.Type: version}
			got := Reconcile(evidence, nil, map[operation.ID]operation.Operation{op.ID: op}, joined)
			if (got == nil) != (version == taskgraph.DurablePlanVersion) {
				t.Fatalf("control=%t joined=%d: %v", control, version, got)
			}
		}
		for _, bad := range []string{`{}`, `null`, `{"version":99,"parent_id":"parent"}`, `{"version":1,"parent_id":"parent","unknown":true}`} {
			current := state
			current.Plan.Data = []byte(bad)
			broken := op
			broken.State, err = json.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			got := Reconcile(evidence, nil, map[operation.ID]operation.Operation{op.ID: broken}, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{state.Plan.Type: 2})
			if got == nil || !errors.Is(got, evidence) {
				t.Fatalf("malformed joined graph discharged evidence: control=%t %s: %v", control, bad, got)
			}
		}
		// Unknown members are added to otherwise valid intent: missing required
		// fields must not accidentally be the reason a strictness test passes.
		for _, envelope := range []bool{false, true} {
			broken := op
			if envelope {
				broken.State = append(append([]byte{}, op.State[:len(op.State)-1]...), []byte(`,"unknown":true}`)...)
			} else {
				current := state
				current.Plan.Data = append(append([]byte{}, state.Plan.Data[:len(state.Plan.Data)-1]...), []byte(`,"unknown":true}`)...)
				broken.State, err = json.Marshal(current)
				if err != nil {
					t.Fatal(err)
				}
			}
			got := Reconcile(evidence, nil, map[operation.ID]operation.Operation{op.ID: broken}, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{state.Plan.Type: 2})
			if got == nil || !errors.Is(got, evidence) {
				t.Fatalf("unknown member admitted: control=%t envelope=%t", control, envelope)
			}
		}
		// Even a valid payload does not grant blanket authority to arbitrary v2 kinds.
		for _, kind := range []operation.RemoteJobPlanType{"custom.run", "subagent.run", "subagent.control"} {
			current := state
			current.Plan.Type = kind
			broken := op
			broken.State, err = json.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			if got := Reconcile(evidence, nil, map[operation.ID]operation.Operation{op.ID: broken}, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{kind: 2}); got == nil {
				t.Fatalf("arbitrary v2 %s discharged graph evidence", kind)
			}
		}
	}
}
