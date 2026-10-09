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

type recordingContext struct {
	specs []operation.Spec
}

func (ctx *recordingContext) Submit(spec operation.Spec) operation.ID {
	ctx.specs = append(ctx.specs, spec)
	return operation.ID("operation-1")
}

func TestValidateName(t *testing.T) {
	for _, name := range []string{"worker", "a", "x_1-b"} {
		if err := ValidateName(name); err != nil {
			t.Errorf("ValidateName(%q) = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "parent", "Worker", "-x", strings.Repeat("a", 41), "a b"} {
		if err := ValidateName(name); err == nil {
			t.Errorf("ValidateName(%q) = nil, want error", name)
		}
	}
}

func TestAgentTranslateSubmitsRunPlan(t *testing.T) {
	ctx := &recordingContext{}
	status := NewAgent().Translate(ctx, llm.ToolCall{
		CallID:    "call-1",
		Name:      "Agent",
		Arguments: `{"name":"worker","prompt":"do it"}`,
	})
	if status.Error != "" || !reflect.DeepEqual(status.WaitingFor, []operation.ID{"operation-1"}) {
		t.Fatalf("status = %#v", status)
	}
	if len(ctx.specs) != 1 {
		t.Fatalf("submitted specs = %d, want 1", len(ctx.specs))
	}
	spec := ctx.specs[0]
	if spec.Type != operation.TypeRemoteJob {
		t.Fatalf("spec type = %q, want %q", spec.Type, operation.TypeRemoteJob)
	}
	var state operation.RemoteJobState
	if err := json.Unmarshal(spec.State, &state); err != nil {
		t.Fatalf("decode remote job state: %v", err)
	}
	if state.Plan.Type != RunPlanType || state.Plan.Version != PlanVersion {
		t.Fatalf("plan type/version = %q/%d", state.Plan.Type, state.Plan.Version)
	}
	var plan RunPlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatalf("decode run plan: %v", err)
	}
	if want := (RunPlan{Name: "worker", Prompt: "do it"}); plan != want {
		t.Fatalf("run plan = %#v, want %#v", plan, want)
	}
}

func TestAgentTranslateRejectsInvalidArguments(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments string
	}{
		{name: "invalid JSON", arguments: `{`},
		{name: "missing prompt", arguments: `{"name":"worker"}`},
		{name: "blank prompt", arguments: `{"name":"worker","prompt":"  "}`},
		{name: "reserved name", arguments: `{"name":"parent","prompt":"do it"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &recordingContext{}
			status := NewAgent().Translate(ctx, llm.ToolCall{Name: "Agent", Arguments: test.arguments})
			if status.Error == "" || len(ctx.specs) != 0 {
				t.Fatalf("status = %#v, specs = %d", status, len(ctx.specs))
			}
		})
	}
}

func TestSendMessageSubmitsMessagePlan(t *testing.T) {
	ctx := &recordingContext{}
	status := NewSendMessage().Translate(ctx, llm.ToolCall{
		Name:      "SendMessage",
		Arguments: `{"to":"worker","message":"hi"}`,
	})
	if status.Error != "" || len(status.WaitingFor) != 1 || len(ctx.specs) != 1 {
		t.Fatalf("status = %#v, specs = %d", status, len(ctx.specs))
	}
	var state operation.RemoteJobState
	if err := json.Unmarshal(ctx.specs[0].State, &state); err != nil {
		t.Fatalf("decode remote job state: %v", err)
	}
	if state.Plan.Type != MessagePlanType || state.Plan.Version != PlanVersion {
		t.Fatalf("plan type/version = %q/%d", state.Plan.Type, state.Plan.Version)
	}
	var plan MessagePlan
	if err := json.Unmarshal(state.Plan.Data, &plan); err != nil {
		t.Fatalf("decode message plan: %v", err)
	}
	if want := (MessagePlan{To: "worker", Message: "hi"}); plan != want {
		t.Fatalf("message plan = %#v, want %#v", plan, want)
	}
}

func TestSendMessageRejectsParentAndMissingMessage(t *testing.T) {
	for _, test := range []struct {
		name      string
		arguments string
	}{
		{name: "parent target", arguments: `{"to":"parent","message":"hi"}`},
		{name: "missing message", arguments: `{"to":"worker"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := &recordingContext{}
			status := NewSendMessage().Translate(ctx, llm.ToolCall{Name: "SendMessage", Arguments: test.arguments})
			if status.Error == "" || len(ctx.specs) != 0 {
				t.Fatalf("status = %#v, specs = %d", status, len(ctx.specs))
			}
		})
	}
}

func TestSendToParentSubmitsValue(t *testing.T) {
	ctx := &recordingContext{}
	status := NewSendToParent().Translate(ctx, llm.ToolCall{
		Name:      "SendMessage",
		Arguments: `{"to":"parent","message":"hi"}`,
	})
	if status.Error != "" || len(status.WaitingFor) != 1 || len(ctx.specs) != 1 {
		t.Fatalf("status = %#v, specs = %d", status, len(ctx.specs))
	}
	if ctx.specs[0].Type != operation.TypeValue {
		t.Fatalf("spec type = %q, want %q", ctx.specs[0].Type, operation.TypeValue)
	}
}

func TestSendToParentRejectsNonParentTarget(t *testing.T) {
	ctx := &recordingContext{}
	status := NewSendToParent().Translate(ctx, llm.ToolCall{
		Name:      "SendMessage",
		Arguments: `{"to":"worker","message":"hi"}`,
	})
	if status.Error == "" || len(ctx.specs) != 0 {
		t.Fatalf("status = %#v, specs = %d", status, len(ctx.specs))
	}
}

func TestSendToParentTranslateResult(t *testing.T) {
	ctx := &recordingContext{}
	translator := NewSendToParent()
	translator.Translate(ctx, llm.ToolCall{Name: "SendMessage", Arguments: `{"to":"parent","message":"hi"}`})
	spec := ctx.specs[0]
	current := operation.Operation{
		ID: "operation-1", Type: spec.Type, Version: spec.Version,
		MaxOutputLength: spec.MaxOutputLength, State: spec.State,
	}

	current.Status = operation.StatusCompleted
	result, err := translator.TranslateResult("call-1", noStatus(), []operation.Operation{current})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := result.(Result); !ok || got.Text != "Message sent to parent." || got.Running || got.Error != "" {
		t.Fatalf("completed result = %#v", result)
	}

	current.Status = operation.StatusReady
	result, err = translator.TranslateResult("call-1", noStatus(), []operation.Operation{current})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ToLLMResult().Running {
		t.Fatalf("ready result = %#v, want running", result)
	}
}

func TestAgentTranslateResult(t *testing.T) {
	status := func(id operation.ID, s operation.Status, result, errText string) operation.Operation {
		data, err := json.Marshal(RunPlan{Name: "worker", Prompt: "do it"})
		if err != nil {
			t.Fatal(err)
		}
		state, err := json.Marshal(operation.RemoteJobState{
			Plan:           operation.RemoteJobPlan{Type: RunPlanType, Version: PlanVersion, Data: data},
			TerminalResult: result,
			TerminalError:  errText,
		})
		if err != nil {
			t.Fatal(err)
		}
		return operation.Operation{
			ID: id, Type: operation.TypeRemoteJob, Version: operation.VersionRemoteJob,
			MaxOutputLength: operation.DefaultMaxOutputLength, Status: s, State: state,
		}
	}
	translator := NewAgent()

	t.Run("awaiting is running", func(t *testing.T) {
		result, err := translator.TranslateResult("call-1", noStatus(),
			[]operation.Operation{status("op", operation.StatusAwaiting, "", "")})
		if err != nil {
			t.Fatal(err)
		}
		if !result.ToLLMResult().Running {
			t.Fatalf("result = %#v, want running", result)
		}
	})

	t.Run("completed returns terminal result", func(t *testing.T) {
		result, err := translator.TranslateResult("call-1", noStatus(),
			[]operation.Operation{status("op", operation.StatusCompleted, "done", "")})
		if err != nil {
			t.Fatal(err)
		}
		llmResult := result.ToLLMResult()
		if llmResult.Running || len(llmResult.Output) != 1 || llmResult.Output[0].Value != "done" {
			t.Fatalf("llm result = %#v", llmResult)
		}
	})

	t.Run("failed reports terminal error", func(t *testing.T) {
		result, err := translator.TranslateResult("call-1", noStatus(),
			[]operation.Operation{status("op", operation.StatusFailed, "", "boom")})
		if err != nil {
			t.Fatal(err)
		}
		llmResult := result.ToLLMResult()
		if len(llmResult.Output) != 1 || !strings.Contains(llmResult.Output[0].Value, "Error: boom") {
			t.Fatalf("llm result = %#v", llmResult)
		}
	})

	t.Run("status error without operations", func(t *testing.T) {
		result, err := translator.TranslateResult("call-1", errorStatus("bad"), nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.ToLLMResult(); !strings.Contains(got.Output[0].Value, "bad") {
			t.Fatalf("llm result = %#v", got)
		}
	})

	t.Run("status error with operation is an error", func(t *testing.T) {
		_, err := translator.TranslateResult("call-1", errorStatus("bad"),
			[]operation.Operation{status("op", operation.StatusCompleted, "done", "")})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no operations and no status error is an error", func(t *testing.T) {
		_, err := translator.TranslateResult("call-1", noStatus(), nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestParseMessageRoundTrips(t *testing.T) {
	plan, err := ParseMessage(`{"to":"parent","message":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := (MessagePlan{To: "parent", Message: "x"}); plan != want {
		t.Fatalf("plan = %#v, want %#v", plan, want)
	}
}

func noStatus() tool.CallStatus {
	return tool.CallStatus{}
}

func errorStatus(text string) tool.CallStatus {
	return tool.CallStatus{Error: text}
}
