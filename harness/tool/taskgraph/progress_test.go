package taskgraph

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func TestPendingResultReportsOperationStateAndPreservesRuntimeUpdates(t *testing.T) {
	ctx := &recordingContext{}
	status := New().Translate(ctx, llm.ToolCall{Arguments: `{"action":"status","name":"goal"}`})
	spec := ctx.specs[0]
	for _, phase := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling} {
		for _, update := range []string{"", "task tests claims the whole workspace; symlinks are unsupported", "cancellation requested; waiting for scans and children to drain"} {
			t.Run(string(phase)+"/"+update, func(t *testing.T) {
				current := operation.Operation{ID: "pending", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: phase}
				state, err := operation.DecodeRemoteJobState(current)
				if err != nil {
					t.Fatal(err)
				}
				state.TerminalResult = update
				current.State, err = json.Marshal(state)
				if err != nil {
					t.Fatal(err)
				}
				got, err := New().TranslateResult("call", status, []operation.Operation{current})
				if err != nil {
					t.Fatal(err)
				}
				result := got.(Result)
				for _, want := range []string{"status: " + string(phase), "does not prove that a command has started", "scan phases", "current generation"} {
					if !strings.Contains(result.Text, want) {
						t.Fatalf("missing %q: %s", want, result.Text)
					}
				}
				if strings.Contains(result.Text, "operation is running") {
					t.Fatal("submission misreported as execution")
				}
				if update != "" && !strings.Contains(result.Text, "Runtime update:\n"+update) {
					t.Fatalf("runtime update lost: %s", result.Text)
				}
				wire := result.ToLLMResult()
				if !result.Running || !wire.Running || len(wire.Output) != 1 || wire.Output[0].Value != result.Text {
					t.Fatalf("pending result changed at model boundary: %+v", wire)
				}
			})
		}
	}
}
