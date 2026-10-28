package recoverygate

import (
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

func m27Graph(t *testing.T, control bool) operation.Operation {
	t.Helper()
	var plan any = taskgraph.DurableRunPlan{Version: 1, ParentID: "parent", Run: taskgraph.RunPlan{Name: "graph", Tasks: []taskgraph.Task{{ID: "a", Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "checked"}}}}
	typ := taskgraph.RunPlanType
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
	return operation.Operation{ID: "graph", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
}

func m27GraphEdit(t *testing.T, op operation.Operation, edit func(*operation.RemoteJobState)) operation.Operation {
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

func TestM27GraphRecoveryStrictAndMixed(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting} {
		graph := m27Graph(t, false)
		graph.Status = status
		child := durableChildFixture(t)
		ops := []operation.Operation{graph, child}
		before, err := json.Marshal(ops)
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(ops); err != nil {
			t.Fatalf("mixed graph and child: %v", err)
		}
		after, _ := json.Marshal(ops)
		if !reflect.DeepEqual(before, after) {
			t.Fatal("mutated canonical operations")
		}
	}
	for _, version := range []operation.Version{0, 1, 3, 99} {
		op := m27Graph(t, false)
		op.Version = version
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("operation version %d: %v", version, err)
		}
	}
	cases := []struct {
		name string
		edit func(*operation.RemoteJobState)
	}{
		{"old graph", func(s *operation.RemoteJobState) { s.Plan.Version = 1 }},
		{"future graph", func(s *operation.RemoteJobState) { s.Plan.Version = 3 }},
		{"wrong owner kind", func(s *operation.RemoteJobState) { s.Plan.Type = "subagent.run" }},
		{"unknown payload", func(s *operation.RemoteJobState) {
			s.Plan.Data = []byte(`{"version":1,"parent_id":"parent","run":{"name":"graph","tasks":[]},"unknown":true}`)
		}},
		{"missing parent", func(s *operation.RemoteJobState) {
			s.Plan.Data = []byte(`{"version":1,"run":{"name":"graph","tasks":[]}}`)
		}},
		{"bad parent", func(s *operation.RemoteJobState) {
			s.Plan.Data = []byte(`{"version":1,"parent_id":"../parent","run":{"name":"graph","tasks":[]}}`)
		}},
		{"payload version", func(s *operation.RemoteJobState) {
			s.Plan.Data = []byte(`{"version":2,"parent_id":"parent","run":{"name":"graph","tasks":[]}}`)
		}},
		{"agent task", func(s *operation.RemoteJobState) {
			s.Plan.Data = []byte(`{"version":1,"parent_id":"parent","run":{"name":"graph","tasks":[{"id":"a","prompt":"inspect","acceptance":"checked"}]}}`)
		}},
		{"bad checkpoint", func(s *operation.RemoteJobState) { s.Handle = []byte(`{"version":99}`) }},
		{"terminal error", func(s *operation.RemoteJobState) { s.TerminalError = "uncertain" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := m27GraphEdit(t, m27Graph(t, false), tc.edit)
			if err := Validate([]operation.Operation{durableChildFixture(t), op}); !errors.Is(err, operation.ErrUnsupported) {
				t.Fatalf("want fail-closed refusal: %v", err)
			}
		})
	}
	op := m27Graph(t, false)
	op.State = append(append([]byte{}, op.State[:len(op.State)-1]...), []byte(`,"unknown":true}`)...)
	if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
		t.Fatalf("unknown envelope: %v", err)
	}
}

func TestM27GraphControlPendingRefusedTerminalInert(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling} {
		op := m27Graph(t, true)
		op.Status = status
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("pending %s control replay authorized: %v", status, err)
		}
	}
	for _, status := range []operation.Status{operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		op := m27Graph(t, true)
		op.Status = status
		op.State = []byte(`not JSON`)
		op.Version = 99
		if err := Validate([]operation.Operation{op}); err != nil {
			t.Fatalf("terminal history must be inert: %v", err)
		}
	}
}
