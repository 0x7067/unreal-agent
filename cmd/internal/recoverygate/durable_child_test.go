package recoverygate

import (
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func durableChildFixture(t *testing.T) operation.Operation {
	t.Helper()
	data, err := json.Marshal(toolsubagent.DurableRunPlan{Version: 1, ParentID: "stable-parent", Run: toolsubagent.RunPlan{Name: "helper", Prompt: "inspect candidate", Lifetime: "background"}})
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "child", Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength, State: spec.State}
}
func mutateDurableChild(t *testing.T, op operation.Operation, mutate func(map[string]any)) operation.Operation {
	t.Helper()
	var state map[string]any
	if err := json.Unmarshal(op.State, &state); err != nil {
		t.Fatal(err)
	}
	mutate(state)
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	op.State = data
	return op
}

func TestDurableChildGateAllowsOnlyStrictUnfinishedRun(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting} {
		t.Run(string(status), func(t *testing.T) {
			op := durableChildFixture(t)
			op.Status = status
			if _, err := toolsubagent.DecodeDurableRun(op); err != nil {
				t.Fatalf("invalid fixture: %v", err)
			}
			before := op
			before.State = op.State.Clone()
			if err := Validate([]operation.Operation{op}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(op, before) {
				t.Fatal("gate mutated canonical intent")
			}
		})
	}
}

func TestDurableChildGateRejectsMalformedNestedIntent(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"unknown envelope", func(s map[string]any) { s["Unknown"] = true }},
		{"unknown plan envelope", func(s map[string]any) { s["Plan"].(map[string]any)["Unknown"] = true }},
		{"unknown durable payload", func(s map[string]any) { s["Plan"].(map[string]any)["Data"].(map[string]any)["Unknown"] = true }},
		{"unknown nested run", func(s map[string]any) {
			s["Plan"].(map[string]any)["Data"].(map[string]any)["Run"].(map[string]any)["Unknown"] = true
		}},
		{"missing parent", func(s map[string]any) { delete(s["Plan"].(map[string]any)["Data"].(map[string]any), "ParentID") }},
		{"bad parent", func(s map[string]any) { s["Plan"].(map[string]any)["Data"].(map[string]any)["ParentID"] = " parent " }},
		{"bad payload version", func(s map[string]any) { s["Plan"].(map[string]any)["Data"].(map[string]any)["Version"] = 2 }},
		{"old plan version", func(s map[string]any) { s["Plan"].(map[string]any)["Version"] = 1 }},
		{"future plan version", func(s map[string]any) { s["Plan"].(map[string]any)["Version"] = 3 }},
		{"unknown plan type", func(s map[string]any) { s["Plan"].(map[string]any)["Type"] = "unknown.run" }},
		{"taskgraph plan", func(s map[string]any) { s["Plan"].(map[string]any)["Type"] = "taskgraph.run" }},
		{"terminal result", func(s map[string]any) { s["TerminalResult"] = "done" }},
		{"terminal error", func(s map[string]any) { s["TerminalError"] = "failed" }},
		{"bad phase", func(s map[string]any) { s["Phase"] = "running" }},
		{"empty prompt", func(s map[string]any) {
			s["Plan"].(map[string]any)["Data"].(map[string]any)["Run"].(map[string]any)["Prompt"] = " "
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := mutateDurableChild(t, durableChildFixture(t), tc.mutate)
			if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
				t.Fatalf("want unsupported refusal, got %v", err)
			}
		})
	}
}

func TestDurableChildGateStatusAndOperationVersionFences(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusCanceling, operation.StatusUnsupported, "running", "unknown", ""} {
		op := durableChildFixture(t)
		op.Status = status
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("status %q: %v", status, err)
		}
	}
	for _, version := range []operation.Version{0, 1, 3} {
		op := durableChildFixture(t)
		op.Version = version
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("operation version %d: %v", version, err)
		}
	}
	for _, status := range []operation.Status{operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		op := durableChildFixture(t)
		op.Status = status
		op.Version = 1
		op.State = []byte(`not JSON`)
		if err := Validate([]operation.Operation{op}); err != nil {
			t.Fatalf("terminal old history must remain inert: %v", err)
		}
	}
}
