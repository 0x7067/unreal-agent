package bash_test

import (
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	"slices"
	"strings"
	"testing"
)

func TestRecordedExitSurvivesOutputRecoveryErrorProjection(t *testing.T) {
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: "owned", Shell: "/bin/sh", Directory: "/workspace"}, "/captures", 4096)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "old-shell", Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusFailed}
	state, err := operation.DecodeShellState(op)
	if err != nil {
		t.Fatal(err)
	}
	code := 17
	state.PendingExitCode = &code
	state.TerminalError = "open capture: no such file"
	state.OutPath = "/captures/old-shell/out"
	state.ErrPath = "/captures/old-shell/err"
	op.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	before := slices.Clone(op.State)
	result, err := bash.New(bash.Config{}).TranslateResult("call", tool.CallStatus{}, []operation.Operation{op})
	if err != nil {
		t.Fatal(err)
	}
	output := result.ToLLMResult().Output[0].Value
	if !strings.Contains(output, "command exit code 17 was recorded") || !strings.Contains(output, "output recovery failed") || !strings.Contains(output, state.OutPath) || strings.Contains(output, "still running") || !slices.Equal(before, op.State) {
		t.Fatalf("known exit lost/invented/mutated: %s", output)
	}
}
