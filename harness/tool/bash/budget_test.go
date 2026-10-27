package bash_test

import (
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	"testing"
	"time"
)

func TestOptionalWallBudgetWireAndValidation(t *testing.T) {
	translator := bash.New(bash.Config{Shell: "/bin/sh", BaseDirectory: "/operations"})
	for _, args := range []string{
		`{"command":"sleep 30","timeout_seconds":2,"warn_after_seconds":1}`,
		`{"command":"sleep 30","timeout_seconds":0.001}`,
		`{"command":"sleep 30","warn_after_seconds":10}`,
	} {
		ctx := &recordingContext{}
		status := translator.Translate(ctx, llm.ToolCall{Arguments: args})
		if status.Error != "" || len(ctx.specs) != 1 {
			t.Fatalf("%s: %+v", args, status)
		}
		spec := ctx.specs[0]
		if spec.Version != operation.VersionShellBudget {
			t.Fatal("explicit budget not versioned")
		}
		op := operation.Operation{Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State}
		state, err := operation.DecodeShellState(op)
		if err != nil {
			t.Fatal(err)
		}
		if state.Input.Budget.Timeout == 0 && state.Input.Budget.WarnAfter == 0 {
			t.Fatal("budget lost")
		}
		op.Version = operation.VersionShell
		if _, err := operation.DecodeShellState(op); err == nil {
			t.Fatal("v3 silently interpreted budget")
		}
	}
	for _, value := range []string{`0`, `-1`, `null`, `"2"`, `true`, `1e30`, `0.00000000001`} {
		ctx := &recordingContext{}
		status := translator.Translate(ctx, llm.ToolCall{Arguments: `{"command":"quiet","timeout_seconds":` + value + `}`})
		if status.Error == "" || len(ctx.specs) != 0 {
			t.Fatalf("invalid timeout accepted: %s", value)
		}
	}
	ctx := &recordingContext{}
	status := translator.Translate(ctx, llm.ToolCall{Arguments: `{"command":"quiet","timeout_seconds":1,"warn_after_seconds":2}`})
	if status.Error == "" {
		t.Fatal("warning after timeout accepted")
	}
	// Legacy calls stay wire version 3 with no budget.
	ctx = &recordingContext{}
	status = translator.Translate(ctx, llm.ToolCall{Arguments: `{"command":"quiet","max_output_length":1}`})
	if status.Error != "" || ctx.specs[0].Version != operation.VersionShell {
		t.Fatal("legacy spec changed")
	}
	spec := ctx.specs[0]
	state, err := operation.DecodeShellState(operation.Operation{Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State})
	if err != nil || state.Input.Budget.Timeout != 0*time.Second {
		t.Fatal("output limit treated as wall budget")
	}
}
