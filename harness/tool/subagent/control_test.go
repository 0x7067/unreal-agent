package subagent

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestAgentControlPureSerializablePlans(t *testing.T) {
	for _, action := range []string{"list", "status", "message", "stop"} {
		t.Run(action, func(t *testing.T) {
			args := `{"action":"` + action + `"}`
			want := ControlPlan{Action: action}
			if action != "list" {
				want.Handle, want.Generation = "opaque-not-a-name", 7
				args = `{"action":"` + action + `","handle":"opaque-not-a-name","generation":7}`
			}
			if action == "message" {
				want.Message = "review feedback"
				args = `{"action":"message","handle":"opaque-not-a-name","generation":7,"message":"review feedback"}`
			}
			// The only available capability is allocation of inert specs. No Hub,
			// filesystem, inbox, cancellation callback, or supervisor is involved.
			ctx := &recordingContext{}
			got := NewAgentControl().Translate(ctx, llm.ToolCall{Arguments: args})
			if got.Error != "" || len(ctx.specs) != 1 {
				t.Fatalf("status=%+v specs=%v", got, ctx.specs)
			}
			var state operation.RemoteJobState
			if err := json.Unmarshal(ctx.specs[0].State, &state); err != nil {
				t.Fatal(err)
			}
			if state.Plan.Type != ControlPlanType || state.Plan.Version != 1 {
				t.Fatalf("plan=%+v", state.Plan)
			}
			var plan ControlPlan
			if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(plan, want) {
				t.Fatalf("got=%+v want=%+v", plan, want)
			}
		})
	}
}

func TestAgentControlRejectsInvalidBeforeSubmit(t *testing.T) {
	for _, args := range []string{`{`, `{}`, `{"action":"kill"}`, `{"action":"status","handle":"name"}`, `{"action":"stop","handle":"x","generation":0}`, `{"action":"stop","handle":"x","generation":-1}`, `{"action":"message","handle":"x","generation":1,"message":" "}`, `{"action":"list","handle":"x"}`, `{"action":"status","handle":"x","generation":1,"message":"stop"}`} {
		ctx := &recordingContext{}
		status := NewAgentControl().Translate(ctx, llm.ToolCall{Arguments: args})
		if status.Error == "" || len(ctx.specs) != 0 {
			t.Fatalf("args=%s status=%+v specs=%v", args, status, ctx.specs)
		}
	}
}

func TestAgentMetadataOptionalAndLegacyReads(t *testing.T) {
	var legacy RunPlan
	if err := json.Unmarshal([]byte(`{"Name":"worker","Prompt":"work"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Lifetime != "" || legacy.Purpose != "" || legacy.OwnedResources != nil {
		t.Fatalf("legacy=%+v", legacy)
	}
	ctx := &recordingContext{}
	status := NewAgent().Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"work","purpose":"review","parent_task":"request","lifetime":"task","owned_resources":["report.txt"]}`})
	if status.Error != "" {
		t.Fatal(status.Error)
	}
	var state operation.RemoteJobState
	if err := json.Unmarshal(ctx.specs[0].State, &state); err != nil {
		t.Fatal(err)
	}
	var plan RunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Purpose != "review" || plan.ParentTask != "request" || plan.Lifetime != "task" || !reflect.DeepEqual(plan.OwnedResources, []string{"report.txt"}) {
		t.Fatalf("plan=%+v", plan)
	}
	ctx = &recordingContext{}
	status = NewAgent().Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"work","lifetime":"forever"}`})
	if status.Error == "" || len(ctx.specs) != 0 {
		t.Fatalf("status=%+v specs=%v", status, ctx.specs)
	}
}

func TestAgentControlTruthfulPendingAndTerminalResults(t *testing.T) {
	ctx := &recordingContext{}
	translator := NewAgentControl()
	translator.Translate(ctx, llm.ToolCall{Arguments: `{"action":"list"}`})
	spec := ctx.specs[0]
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		current := operation.Operation{ID: "control", Type: spec.Type, Version: spec.Version, State: spec.State, Status: status, MaxOutputLength: spec.MaxOutputLength}
		result, err := translator.TranslateResult("call", tool.CallStatus{}, []operation.Operation{current})
		if err != nil {
			t.Fatal(err)
		}
		llmResult := result.ToLLMResult()
		pending := status == operation.StatusReady || status == operation.StatusAwaiting || status == operation.StatusCanceling
		if llmResult.Running != pending {
			t.Fatalf("%s running=%v", status, llmResult.Running)
		}
		if pending && !strings.Contains(llmResult.Output[0].Value, "does not prove") {
			t.Fatalf("overclaimed pending: %+v", llmResult)
		}
	}
}

func TestCompletedStopRequestDoesNotImplyExecutionDrained(t *testing.T) {
	ctx := &recordingContext{}
	translator := NewAgentControl()
	translator.Translate(ctx, llm.ToolCall{Arguments: `{"action":"stop","handle":"opaque","generation":1}`})
	spec := ctx.specs[0]
	var state operation.RemoteJobState
	if err := json.Unmarshal(spec.State, &state); err != nil {
		t.Fatal(err)
	}
	outcome := ControlOutcome{Action: "stop", Handle: "opaque", Generation: 1, State: "cancellation_requested", DrainConfirmed: false}
	encoded, err := json.Marshal(outcome)
	if err != nil {
		t.Fatal(err)
	}
	state.TerminalResult = string(encoded)
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	current := operation.Operation{ID: "stop", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: data, Status: operation.StatusCompleted}
	result, err := translator.TranslateResult("call", tool.CallStatus{}, []operation.Operation{current})
	if err != nil {
		t.Fatal(err)
	}
	typed, ok := result.(ControlResult)
	if !ok || typed.Running || typed.Status != operation.StatusCompleted || typed.Outcome == nil || typed.Outcome.DrainConfirmed || typed.Outcome.State != "cancellation_requested" {
		t.Fatalf("result=%+v", result)
	}
	if !strings.Contains(typed.ToLLMResult().Output[0].Value, `"DrainConfirmed":false`) {
		t.Fatalf("missing truthful drain barrier: %+v", typed)
	}
}

func TestAgentProgressExposesOpaqueAttemptHandleWithoutChangingLegacyResult(t *testing.T) {
	ctx := &recordingContext{}
	translator := NewAgent()
	translator.Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"work"}`})
	spec := ctx.specs[0]
	var state operation.RemoteJobState
	if err := json.Unmarshal(spec.State, &state); err != nil {
		t.Fatal(err)
	}
	state.Handle = []byte(`{"SessionID":"session","Handle":"opaque","Generation":4}`)
	state.TerminalResult = "candidate"
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "agent", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: encoded, Status: operation.StatusAwaiting}
	result, err := translator.TranslateResult("call", tool.CallStatus{}, []operation.Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	typed := result.(Result)
	if typed.Handle != "opaque" || typed.Generation != 4 || !typed.Running || !strings.Contains(typed.ToLLMResult().Output[0].Value, "Agent handle: opaque; generation: 4") {
		t.Fatalf("progress=%+v", typed)
	}
	op.Status = operation.StatusCompleted
	result, err = translator.TranslateResult("call", tool.CallStatus{}, []operation.Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.ToLLMResult().Output[0].Value; got != "candidate" {
		t.Fatalf("terminal result changed=%q", got)
	}
}
