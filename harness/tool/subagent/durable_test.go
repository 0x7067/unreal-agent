package subagent

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func durableOperation(spec operation.Spec) operation.Operation {
	return operation.Operation{Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
}

func TestDurableTranslationAndV1Unchanged(t *testing.T) {
	for _, durable := range []bool{false, true} {
		ctx := &recordingContext{}
		translator := NewAgent()
		if durable {
			translator = NewDurableAgent(session.ID("stable-parent"))
		}
		status := translator.Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"do it","purpose":"testing","lifetime":"background","owned_resources":[]}`})
		if status.Error != "" || len(ctx.specs) != 1 {
			t.Fatalf("%+v / %+v", status, ctx.specs)
		}
		op := durableOperation(ctx.specs[0])
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			t.Fatal(err)
		}
		if !durable {
			if state.Plan.Version != PlanVersion {
				t.Fatal(state.Plan)
			}
			continue
		}
		plan, err := DecodeDurableRun(op)
		if err != nil || plan.ParentID != "stable-parent" || plan.Version != 1 || plan.Run.Purpose != "testing" {
			t.Fatalf("%+v: %v", plan, err)
		}
	}
}

func TestDurableInvalidArgumentsNeverSubmit(t *testing.T) {
	for _, args := range []string{`{"name":"worker","prompt":"ok"}`, `{"name":"worker","prompt":"ok","lifetime":"task"}`, `{`, `{"name":"worker","prompt":" "}`, `{"name":"parent","prompt":"ok"}`, `{"name":"worker","prompt":"ok","lifetime":"forever"}`, `{"name":"worker","prompt":"` + strings.Repeat("x", 256*1024+1) + `"}`, `{"name":"worker","prompt":"ok","purpose":"` + strings.Repeat("x", 2049) + `"}`} {
		ctx := &recordingContext{}
		status := NewDurableAgent("parent-session").Translate(ctx, llm.ToolCall{Arguments: args})
		if status.Error == "" || len(ctx.specs) != 0 || len(status.WaitingFor) != 0 {
			t.Fatalf("invalid accepted: %+v", status)
		}
	}
	for _, parent := range []session.ID{"", " bad", session.ID(strings.Repeat("x", 129))} {
		ctx := &recordingContext{}
		status := NewDurableAgent(parent).Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"ok","lifetime":"background"}`})
		if status.Error == "" || len(ctx.specs) != 0 {
			t.Fatal(status)
		}
	}
}

func TestDurableCanonicalStrict(t *testing.T) {
	for _, data := range []string{
		`{"Version":2,"ParentID":"p","Run":{"Name":"worker","Prompt":"ok"}}`,
		`{"Version":1,"ParentID":"","Run":{"Name":"worker","Prompt":"ok"}}`,
		`{"Version":1,"ParentID":"p","Unknown":true,"Run":{"Name":"worker","Prompt":"ok"}}`,
		`{"Version":1,"ParentID":"p","Run":{"Name":"worker","Prompt":"ok","Unknown":true}}`,
	} {
		spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: RunPlanType, Version: DurablePlanVersion, Data: []byte(data)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeDurableRun(durableOperation(spec)); err == nil {
			t.Fatalf("accepted %s", data)
		}
	}
	ctx := &recordingContext{}
	NewDurableAgent("configured-parent").Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"ok","lifetime":"background","ParentID":"attacker"}`})
	plan, err := DecodeDurableRun(durableOperation(ctx.specs[0]))
	if err != nil || plan.ParentID != "configured-parent" {
		t.Fatalf("parent override: %+v %v", plan, err)
	}
	op := durableOperation(ctx.specs[0])
	for _, version := range []operation.RemoteJobPlanVersion{1, 3} {
		var state operation.RemoteJobState
		if err := json.Unmarshal(op.State, &state); err != nil {
			t.Fatal(err)
		}
		state.Plan.Version = version
		altered := op
		altered.State, _ = json.Marshal(state)
		if _, err := DecodeDurableRun(altered); err == nil {
			t.Fatal("accepted unsupported version")
		}
	}
	op.Type = operation.Type("other")
	if _, err := DecodeDurableRun(op); err == nil {
		t.Fatal("accepted wrong envelope")
	}
}

func TestDurableResultPureScope(t *testing.T) {
	ctx := &recordingContext{}
	translator := NewDurableAgent("parent")
	status := translator.Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"ok","lifetime":"background"}`})
	op := durableOperation(ctx.specs[0])
	var state operation.RemoteJobState
	if err := json.Unmarshal(op.State, &state); err != nil {
		t.Fatal(err)
	}
	state.Handle = []byte(`{"SessionID":"child","Handle":"opaque","Generation":4,"Link":{"ParentID":"parent"}}`)
	op.State, _ = json.Marshal(state)
	result, err := translator.TranslateResult("call", status, []operation.Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	r := result.(Result)
	if r.Handle != "opaque" || r.Generation != 4 || !r.Running || len(ctx.specs) != 1 {
		t.Fatalf("%+v", r)
	}
	if _, err := translator.TranslateResult("call", tool.ErrorStatus("invalid", 0), []operation.Operation{op}); err == nil {
		t.Fatal("accepted mixed error/operations")
	}
}

func TestDurableMetadataBounds(t *testing.T) {
	valid := DurableRunPlan{Version: 1, ParentID: session.ID(strings.Repeat("p", 128)), Run: RunPlan{Name: "worker", Lifetime: "background", Prompt: strings.Repeat("x", 256*1024), Purpose: strings.Repeat("x", 2048), ParentTask: strings.Repeat("x", 2048), GraphID: strings.Repeat("g", 128), TaskID: strings.Repeat("t", 128), TaskGeneration: 3, OwnedResources: make([]string, 64)}}
	for i := range valid.Run.OwnedResources {
		valid.Run.OwnedResources[i] = strings.Repeat("r", 2048)
	}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*DurableRunPlan){
		func(p *DurableRunPlan) { p.Run.ParentTask += "x" },
		func(p *DurableRunPlan) { p.Run.GraphID += "x" },
		func(p *DurableRunPlan) { p.Run.TaskID = " " },
		func(p *DurableRunPlan) { p.Run.OwnedResources = make([]string, 65) },
		func(p *DurableRunPlan) { p.Run.OwnedResources = []string{strings.Repeat("r", 2049)} },
	} {
		p := valid
		mutate(&p)
		if err := p.Validate(); err == nil {
			t.Fatal("accepted oversized metadata")
		}
	}
}

func TestDurableStrictEnvelopeMembers(t *testing.T) {
	ctx := &recordingContext{}
	NewDurableAgent("parent").Translate(ctx, llm.ToolCall{Arguments: `{"name":"worker","prompt":"ok","lifetime":"background"}`})
	op := durableOperation(ctx.specs[0])
	for _, replacement := range []struct{ from, to string }{
		{`"Plan":{`, `"Plan":{"Unknown":true,`},
		{`{"Plan":`, `{"Unknown":true,"Plan":`},
	} {
		altered := op
		altered.State = []byte(strings.Replace(string(op.State), replacement.from, replacement.to, 1))
		if string(altered.State) == string(op.State) {
			t.Fatalf("test replacement missed: %s", op.State)
		}
		if _, err := DecodeDurableRun(altered); err == nil {
			t.Fatal("accepted envelope unknown member")
		}
	}
}
