package contextbuilder

import (
	"encoding/json/jsontext"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestBuildCompactionSelectsSamePrefixAsReconstruction(t *testing.T) {
	current := NewBuilder().(*builder)
	for turn, total := range []int64{1_000, 101_000, 104_000, 109_000, 119_000} {
		addCompactionTestTurn(t, current, turn, total)
	}
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "before compaction"})
	before := buildCompactionContext(t, current)
	checkpoints := slices.Clone(current.prefixTokens)
	staged := slices.Clone(current.stagedSuffix)
	result, err := current.BuildCompaction()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, buildCompactionContext(t, current)) || !reflect.DeepEqual(checkpoints, current.prefixTokens) || !reflect.DeepEqual(staged, current.stagedSuffix) {
		t.Fatal("building compaction mutated ordinary context or accounting")
	}
	history := result.Request.Input
	if len(history) != 8 {
		t.Fatalf("history has %d items, want system, whole turns A and B, and summary instruction", len(history))
	}
	for index, want := range map[int]string{1: "A input", 3: "A output", 4: "B input", 6: "B output", 7: "Summarise"} {
		if message := history[index].Data.(llm.Message); message.Text != want {
			t.Fatalf("history[%d] = %#v, want %q", index, message, want)
		}
	}
	current.AddToolResult("pending", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "completed during compaction"}}, false)
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "during compaction"})
	// Even a commit without a response checkpoint must leave the selected prefix unchanged.
	current.Commit()
	again, err := current.BuildCompaction()
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatalf("arrivals changed summary request: %v", err)
	}
	rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
	if !ok {
		t.Fatal("compaction failed")
	}
	accepted := buildCompactionContext(t, current).Request.Input
	want := []llm.Item{accepted[0], {Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "summary"}}}
	want = append(want, accepted[7:]...)
	if got := buildCompactionContext(t, rebuilt).Request.Input; !reflect.DeepEqual(got, want) {
		t.Fatalf("reconstruction did not replace exactly the summarized prefix: %#v", got)
	}
}

func TestBuildCompactionPreservesNativeHistory(t *testing.T) {
	current := NewBuilder().(*builder)
	current.SetSystemPrompt("Project constraint: preserve /tmp/project.go")
	call := llm.Item{ProviderID: "fc-123", Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-123", Name: "bash", Arguments: `{"command":"cat /tmp/project.go"}`}}
	reasoning := llm.Item{ProviderID: "rs-123", Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning","encrypted":"opaque-state"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "Need to inspect the file"}}}
	addCompactionTestTurn(t, current, 0, 10_000, reasoning, call)
	current.AddToolResult("call-123", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
	addCompactionTestTurn(t, current, 1, 20_000)
	current.AddToolResult("call-123", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "cat: /tmp/project.go: No such file or directory"}}, false)
	addCompactionTestTurn(t, current, 2, 50_000)
	before := buildCompactionContext(t, current)
	result, err := current.BuildCompaction()
	if err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(before.Request.Input)
	want[0] = llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleSystem, Text: strings.TrimSpace(compactionPrompt)}}
	want = append(want, llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Summarise"}})
	if !reflect.DeepEqual(result.Request.Input, want) {
		t.Fatalf("native history changed: %#v", result.Request.Input)
	}
	if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
		t.Fatal("building compaction changed ordinary provider replay data")
	}
	result.Request.Input[1] = llm.Item{}
	if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
		t.Fatal("compaction request shares its input slice with the original builder")
	}
}

func TestBuildCompactionCompletesPendingCallsAtCutoff(t *testing.T) {
	current := NewBuilder().(*builder)
	var calls []llm.Item
	for _, id := range []string{"finished", "running", "z", "a"} {
		calls = append(calls, llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: id, Name: "test"}})
	}
	addCompactionTestTurn(t, current, 0, 10_000, calls...)
	current.AddToolResult("finished", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
	current.AddToolResult("running", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
	addCompactionTestTurn(t, current, 1, 35_000)
	current.AddToolResult("z", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "finished in retained history"}}, false)
	addCompactionTestTurn(t, current, 2, 50_000)
	current.AddToolResult("a", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "finished after latest checkpoint"}}, false)
	before := buildCompactionContext(t, current)
	result, err := current.BuildCompaction()
	if err != nil {
		t.Fatal(err)
	}
	cutoff, _ := current.compactionCutoff()
	end := current.prefixTokens[cutoff].committedPrefixIndex
	want := slices.Clone(before.Request.Input[:end])
	want[0] = llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleSystem, Text: strings.TrimSpace(compactionPrompt)}}
	for _, id := range []string{"z", "a"} {
		want = append(want, llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: id, Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ToolCallRunningPayload}}, Running: true}})
	}
	want = append(want, llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Summarise"}})
	if !reflect.DeepEqual(result.Request.Input, want) {
		t.Fatalf("history with pending results = %#v, want %#v", result.Request.Input, want)
	}
	if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
		t.Fatal("completing native history changed the original builder")
	}
	current.Commit()
	again, err := current.BuildCompaction()
	if err != nil || !reflect.DeepEqual(result, again) {
		t.Fatalf("committing newer results changed summary request: %v", err)
	}
}

func TestBuildCompactionIncludesPreviousSummary(t *testing.T) {
	fresh, ok := newCompactionHistory(t).Compact(usableCompactionResponse("Previous summary: keep /src/main.go"))
	if !ok {
		t.Fatal("first compaction failed")
	}
	current := fresh.(*builder)
	addCompactionTestTurn(t, current, 5, 20_000)
	addCompactionTestTurn(t, current, 6, 35_000)
	result, err := current.BuildCompaction()
	if err != nil {
		t.Fatal(err)
	}
	history := result.Request.Input
	if summary := history[1].Data.(llm.Message); summary.Text != "Previous summary: keep /src/main.go" {
		t.Fatalf("previous summary missing: %#v", summary)
	}
	if last := history[len(history)-2].Data.(llm.Message); last.Text != "F output" {
		t.Fatalf("summary must include history through F and retain G: %#v", last)
	}
}

func TestBuildCompactionPreservesModel(t *testing.T) {
	for _, test := range []struct {
		name  string
		limit *int64
	}{
		{name: "unset"},
		{name: "configured", limit: new(int64(32_768))},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := newCompactionHistory(t)
			model := llm.Model{ID: "current-model", CompactionThreshold: 64_000, MaxOutputTokens: test.limit, ReasoningEffort: llm.ReasoningEffortHigh}
			current.SetModel(model)
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: inbox.Settings{ReasoningEffort: llm.ReasoningEffortLow}})
			model.ReasoningEffort = llm.ReasoningEffortLow
			current.AddTool(llm.Tool{Name: "test"})
			result, err := current.BuildCompaction()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(result.Request.Model, model) || !reflect.DeepEqual(buildCompactionContext(t, current).Request.Model, model) {
				t.Fatal("building summary changed model settings")
			}
			if len(result.Request.Tools) != 0 {
				t.Fatal("summary request includes tools")
			}
			rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
			if !ok || !reflect.DeepEqual(buildCompactionContext(t, rebuilt).Request.Model, model) {
				t.Fatal("compaction lost updated model settings")
			}
		})
	}
}

func TestCompactionPreservesStructuredToolResults(t *testing.T) {
	for _, location := range []string{"summarized", "retained", "staged"} {
		t.Run(location, func(t *testing.T) {
			current := NewBuilder().(*builder)
			call := llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "image", Name: "ViewImage"}}
			addCompactionTestTurn(t, current, 0, 10_000, call)
			output := []llm.ToolResultOutput{
				{Kind: llm.ToolResultText, Value: "Image preview"},
				{Kind: llm.ToolResultImage, Value: "data:image/png;base64,aGVsbG8="},
			}
			if location == "summarized" {
				current.AddToolResult("image", output, false)
			}
			addCompactionTestTurn(t, current, 1, 35_000)
			if location == "retained" {
				current.AddToolResult("image", output, false)
			}
			addCompactionTestTurn(t, current, 2, 50_000)
			if location == "staged" {
				current.AddToolResult("image", output, false)
			}
			result, err := current.BuildCompaction()
			if err != nil {
				t.Fatal(err)
			}
			rebuilt, ok := current.Compact(usableCompactionResponse("summary"))
			if !ok {
				t.Fatal("compaction failed")
			}
			if location != "summarized" {
				result = buildCompactionContext(t, rebuilt)
			}
			want := llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "image", Output: output}}
			if !slices.ContainsFunc(result.Request.Input, func(item llm.Item) bool { return reflect.DeepEqual(item, want) }) {
				t.Fatalf("%s structured tool result was lost: %#v", location, result.Request.Input)
			}
		})
	}
}

func TestBuildCompactionWithoutEligibleHistory(t *testing.T) {
	for _, total := range []int64{0, 20_000} {
		current := NewBuilder().(*builder)
		if total != 0 {
			addCompactionTestTurn(t, current, 0, total)
		}
		if _, err := current.BuildCompaction(); err == nil {
			t.Fatal("built summary without an eligible prefix")
		}
	}
}
