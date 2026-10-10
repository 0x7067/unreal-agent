package taskgraph

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

type recordingContext struct{ specs []operation.Spec }

func (ctx *recordingContext) Submit(spec operation.Spec) operation.ID {
	ctx.specs = append(ctx.specs, spec)
	return "op-1"
}

func translated(t *testing.T, arguments string) operation.RemoteJobState {
	t.Helper()
	ctx := &recordingContext{}
	status := New().Translate(ctx, llm.ToolCall{Arguments: arguments})
	if status.Error != "" || !reflect.DeepEqual(status.WaitingFor, []operation.ID{"op-1"}) || len(ctx.specs) != 1 {
		t.Fatalf("status=%+v specs=%d", status, len(ctx.specs))
	}
	spec := ctx.specs[0]
	if spec.Type != operation.TypeRemoteJob || spec.Version != operation.VersionRemoteJob {
		t.Fatalf("spec=%+v", spec)
	}
	var state operation.RemoteJobState
	if err := json.Unmarshal(spec.State, &state); err != nil {
		t.Fatal(err)
	}
	if state.Plan.Version != graph.PlanVersion {
		t.Fatalf("version=%d", state.Plan.Version)
	}
	return state
}

func TestStartPreservesIndependentAndConservativeClaims(t *testing.T) {
	state := translated(t, `{"action":"start","name":"goal","concurrency":2,"tasks":[{"id":"independent","command":"go test ./...","acceptance":"tests pass","reads":[],"writes":[]},{"id":"conservative","prompt":"inspect changes","acceptance":"reviewed"}]}`)
	if state.Plan.Type != graph.RunPlanType {
		t.Fatal(state.Plan.Type)
	}
	var plan graph.RunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Name != "goal" || plan.Concurrency != 2 || len(plan.Tasks) != 2 {
		t.Fatalf("plan=%+v", plan)
	}
	if plan.Tasks[0].Reads == nil || plan.Tasks[0].Writes == nil || len(plan.Tasks[0].Reads) != 0 || len(plan.Tasks[0].Writes) != 0 {
		t.Fatalf("explicit claims lost: %+v", plan.Tasks[0])
	}
	if plan.Tasks[1].Reads != nil || plan.Tasks[1].Writes != nil {
		t.Fatalf("conservative claims lost: %+v", plan.Tasks[1])
	}
}

func TestControlsPreserveCurrentAttemptAndExistingDependencies(t *testing.T) {
	for _, action := range []string{"accept", "reject"} {
		state := translated(t, `{"action":"`+action+`","name":"goal","task_id":"test","generation":7,"evidence":"checked revision abc"}`)
		var plan graph.ControlPlan
		if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
			t.Fatal(err)
		}
		if state.Plan.Type != graph.ControlPlanType || plan.Action != action || plan.TaskID != "test" || plan.Generation != 7 || plan.Evidence != "checked revision abc" {
			t.Fatalf("plan=%+v", plan)
		}
	}
	state := translated(t, `{"action":"revise","name":"goal","tasks":[{"id":"verify","command":"go test ./...","acceptance":"passes","depends_on":["existing"],"writes":[]}]}`)
	var plan graph.ControlPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.Tasks[0].DependsOn, []string{"existing"}) {
		t.Fatal(plan)
	}
	for _, action := range []string{"status", "cancel", "invalidate", "retry"} {
		translated(t, controlArgs(action))
	}
}
func controlArgs(action string) string {
	if action == "status" || action == "cancel" {
		return `{"action":"` + action + `","name":"goal"}`
	}
	return `{"action":"` + action + `","name":"goal","task_id":"test"}`
}

func TestInvalidCallsSubmitNothing(t *testing.T) {
	validTask := `{"id":"a","command":"true","acceptance":"checked"}`
	cases := []string{
		`{`, `null`, `{} `, `{"action":"unknown","name":"goal"}`, `{"action":"status","name":" "}`,
		`{"action":"status","name":"goal","typo":1}`, `{"action":"status","name":"goal"} {}`,
		`{"action":"start","name":"goal","tasks":[{"id":"a","command":"true","acceptance":"ok","typo":1}]}`,
		`{"action":"start","name":"goal","tasks":[` + validTask + `],"concurrency":17}`,
		`{"action":"start","name":"goal","tasks":[{"id":"a","command":"true","acceptance":"ok","depends_on":["b"]},{"id":"b","command":"true","acceptance":"ok","depends_on":["a"]}]}`,
		`{"action":"start","name":"goal","tasks":[{"id":"a","command":"true","prompt":"do it","acceptance":"ok"}]}`,
		`{"action":"start","name":"goal","tasks":[{"id":"a","command":"true","acceptance":"ok","writes":["../outside"]}]}`,
		`{"action":"revise","name":"goal","tasks":[{"id":"a","command":"true"}]}`,
		`{"action":"revise","name":"goal","tasks":[{"id":"a","command":"true","acceptance":"ok","depends_on":["a"]}]}`,
		`{"action":"accept","name":"goal","task_id":"a","evidence":"ok"}`,
		`{"action":"reject","name":"goal","task_id":"a","generation":1}`,
		`{"action":"retry","name":"goal"}`, `{"action":"invalidate","name":"goal"}`,
		`{"action":"status","name":"goal","tasks":[]}`, `{"action":"status","name":"goal","generation":1}`,
	}
	for _, args := range cases {
		t.Run(args, func(t *testing.T) {
			ctx := &recordingContext{}
			status := New().Translate(ctx, llm.ToolCall{Arguments: args})
			if status.Error == "" || len(ctx.specs) != 0 || len(status.WaitingFor) != 0 {
				t.Fatalf("status=%+v specs=%d", status, len(ctx.specs))
			}
		})
	}
}

func TestResultLifecycleAndProtocolBoundary(t *testing.T) {
	ctx := &recordingContext{}
	status := New().Translate(ctx, llm.ToolCall{Arguments: `{"action":"status","name":"goal"}`})
	spec := ctx.specs[0]
	current := operation.Operation{ID: "op-1", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusAwaiting}
	for _, phase := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		current.Status = phase
		state, err := operation.DecodeRemoteJobState(current)
		if err != nil {
			t.Fatal(err)
		}
		state.TerminalResult = "task status"
		state.TerminalError = "failure details"
		current.State, err = json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		got, err := New().TranslateResult("call", status, []operation.Operation{current})
		if err != nil {
			t.Fatal(err)
		}
		result := got.(Result)
		running := phase == operation.StatusReady || phase == operation.StatusAwaiting || phase == operation.StatusCanceling
		if result.CallID != "call" || result.Running != running {
			t.Fatalf("result=%+v", result)
		}
		if running && !strings.Contains(result.Text, "current generation") {
			t.Fatal(result.Text)
		}
		if !running && result.Text != "task status" {
			t.Fatal(result.Text)
		}
		if (phase == operation.StatusFailed || phase == operation.StatusCanceled) && result.Error != "failure details" {
			t.Fatal(result)
		}
		if result.ToLLMResult().Running != running {
			t.Fatal("running flag lost")
		}
	}
	for _, mutate := range []func(*operation.Operation){
		func(op *operation.Operation) { op.Type = operation.TypeValue },
		func(op *operation.Operation) { op.Version++ },
		func(op *operation.Operation) {
			var state operation.RemoteJobState
			json.Unmarshal(op.State, &state)
			state.Plan.Type = "unrelated"
			op.State, _ = json.Marshal(state)
		},
		func(op *operation.Operation) {
			var state operation.RemoteJobState
			json.Unmarshal(op.State, &state)
			state.Plan.Version++
			op.State, _ = json.Marshal(state)
		},
	} {
		bad := current
		mutate(&bad)
		if _, err := New().TranslateResult("call", status, []operation.Operation{bad}); err == nil {
			t.Fatal("accepted unrelated operation")
		}
	}
	if _, err := New().TranslateResult("call", status, nil); err == nil {
		t.Fatal("accepted missing operation")
	}
}
