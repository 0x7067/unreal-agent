package contextbuilder

import (
	"encoding/json/jsontext"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestNeedsCompactionUsesLatestCheckpoint(t *testing.T) {
	for _, test := range []struct {
		name      string
		threshold int64
		usage     []llm.Usage
		want      bool
	}{
		{name: "no checkpoint", threshold: 83_616},
		{name: "zero threshold", usage: []llm.Usage{{InputTokens: 100_000}}, want: true},
		{name: "below threshold", threshold: 83_616, usage: []llm.Usage{{InputTokens: 80_000, OutputTokens: 3_615}}},
		{name: "at threshold", threshold: 83_616, usage: []llm.Usage{{InputTokens: 80_000, OutputTokens: 3_616}}, want: true},
		{name: "above threshold", threshold: 83_616, usage: []llm.Usage{{InputTokens: 80_000, OutputTokens: 3_617}}, want: true},
		{name: "latest only", threshold: 83_616, usage: []llm.Usage{{InputTokens: 90_000}, {InputTokens: 40_000}}},
		{name: "missing latest usage", threshold: 83_616, usage: []llm.Usage{{InputTokens: 90_000}, {}}},
		{name: "included tokens", threshold: 83_616, usage: []llm.Usage{{
			InputTokens: 80_000, OutputTokens: 3_615,
			CachedInputTokens: 50_000, CacheWriteInputTokens: 10_000, ReasoningTokens: 2_000,
		}}},
		{name: "small threshold", threshold: 1_000, usage: []llm.Usage{{InputTokens: 1_000}}, want: true},
		{name: "small threshold missing usage", threshold: 8_000, usage: []llm.Usage{{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := NewBuilder()
			current.SetModel(llm.Model{CompactionThreshold: test.threshold})
			for _, usage := range test.usage {
				current.AddModelResponse(llm.Response{Usage: usage})
			}
			if got := current.NeedsCompaction(); got != test.want {
				t.Fatalf("NeedsCompaction() = %t, want %t", got, test.want)
			}
			current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: strings.Repeat("result", 100_000)}}, false)
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "new input"})
			if got := current.NeedsCompaction(); got != test.want {
				t.Fatalf("staged arrivals changed NeedsCompaction() to %t", got)
			}
			current.Commit()
			if got := current.NeedsCompaction(); got != test.want {
				t.Fatalf("committing arrivals changed NeedsCompaction() to %t", got)
			}
		})
	}
}

func TestNeedsCompactionUsesUpdatedThreshold(t *testing.T) {
	current := NewBuilder()
	current.SetModel(llm.Model{ID: "model", CompactionThreshold: 200_000})
	current.AddModelResponse(llm.Response{Usage: llm.Usage{InputTokens: 90_000}})
	if current.NeedsCompaction() {
		t.Fatal("usage below threshold needs compaction")
	}
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: inbox.Settings{CompactionThreshold: new(int64(80_000))}})
	if !current.NeedsCompaction() {
		t.Fatal("lowering the threshold did not enable compaction")
	}
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: inbox.Settings{Model: "model", CompactionThreshold: new(int64(200_000))}})
	if current.NeedsCompaction() {
		t.Fatal("raising the threshold kept the lower threshold")
	}
}

func TestModelSettingsResetCheckpointsOnlyWhenModelChanges(t *testing.T) {
	for _, test := range []struct {
		name     string
		settings inbox.Settings
		reset    bool
	}{
		{name: "omitted"},
		{name: "same model", settings: inbox.Settings{Model: "initial", CompactionThreshold: new(int64(50_000))}},
		{name: "threshold only", settings: inbox.Settings{CompactionThreshold: new(int64(50_000))}},
		{name: "reasoning effort", settings: inbox.Settings{ReasoningEffort: llm.ReasoningEffortHigh}},
		{name: "different model", settings: inbox.Settings{Model: "next"}, reset: true},
		{name: "different model and threshold", settings: inbox.Settings{Model: "next", CompactionThreshold: new(int64(50_000))}, reset: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := newCompactionHistory(t)
			current.SetModel(llm.Model{ID: "initial", CompactionThreshold: 50_000})
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "pending"})
			before := buildCompactionContext(t, current).Request.Input
			checkpoints := append([]prefixToken(nil), current.prefixTokens...)
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: test.settings})
			if !reflect.DeepEqual(buildCompactionContext(t, current).Request.Input, before) {
				t.Fatal("settings changed conversation history")
			}
			if !test.reset {
				if !reflect.DeepEqual(current.prefixTokens, checkpoints) || !current.NeedsCompaction() {
					t.Fatal("settings for the same model discarded measured usage")
				}
				return
			}
			if len(current.prefixTokens) != 0 || current.NeedsCompaction() {
				t.Fatal("model switch retained old usage checkpoints")
			}
			if _, err := current.BuildCompaction(); err == nil {
				t.Fatal("model switch reused the old compaction cutoff")
			}
			addCompactionTestTurn(t, current, 5, 30_000)
			want := []prefixToken{{committedPrefixIndex: len(current.committedPrefix), Tokens: 30_000}}
			if !reflect.DeepEqual(current.prefixTokens, want) {
				t.Fatalf("new model checkpoints = %#v, want %#v", current.prefixTokens, want)
			}
			if cutoff, ok := current.compactionCutoff(); !ok || cutoff != 0 {
				t.Fatal("new model did not establish a fresh compaction cutoff")
			}
		})
	}
}

func TestNeedsCompactionResetsWithReconstruction(t *testing.T) {
	current := newCompactionHistory(t)
	current.SetModel(llm.Model{CompactionThreshold: 50_000})
	if !current.NeedsCompaction() {
		t.Fatal("expected compaction before reconstruction")
	}
	summary := usableCompactionResponse("summary")
	summary.Usage = llm.Usage{InputTokens: 200_000, OutputTokens: 4_000}
	rebuilt, ok := current.Compact(summary)
	if !ok {
		t.Fatal("compaction failed")
	}
	if rebuilt.NeedsCompaction() {
		t.Fatal("rebuilt context reused old or summary-request usage")
	}
	rebuilt.Commit()
	rebuilt.AddModelResponse(llm.Response{Usage: llm.Usage{InputTokens: 55_000}})
	if !rebuilt.NeedsCompaction() {
		t.Fatal("rebuilt context did not use the new ordinary checkpoint")
	}
}

func TestBuilderRecordsTokenCheckpoints(t *testing.T) {
	current := NewBuilder().(*builder)
	first := llm.Response{
		Output: []llm.Item{
			{ProviderID: "reasoning", Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "thinking"}}},
			{ProviderID: "message", Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Working."}},
			{ProviderID: "call", Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "A", Name: "test"}},
		},
		Usage: llm.Usage{
			InputTokens: 100, OutputTokens: 20,
			CachedInputTokens: 70, CacheWriteInputTokens: 10, ReasoningTokens: 5,
			Raw: jsontext.Value(`{"input_tokens":100,"output_tokens":20}`),
		},
	}
	current.AddModelResponse(first)
	second := llm.Response{
		Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Continuing."}}},
		Usage:  llm.Usage{InputTokens: 150, OutputTokens: 10},
	}
	current.AddModelResponse(second)

	want := append(withPreamble(first.Output...), second.Output...)
	if !reflect.DeepEqual(current.committedPrefix, want) {
		t.Fatalf("committed prefix = %#v, want %#v", current.committedPrefix, want)
	}
	checkpoints := []prefixToken{
		{committedPrefixIndex: 4, Tokens: 120},
		{committedPrefixIndex: 5, Tokens: 160},
	}
	if !reflect.DeepEqual(current.prefixTokens, checkpoints) {
		t.Fatalf("checkpoints = %#v, want %#v", current.prefixTokens, checkpoints)
	}
}

func TestBuilderRecordsEmptyResponseCheckpoints(t *testing.T) {
	current := NewBuilder().(*builder)
	current.Commit()
	current.Commit()
	current.AddModelResponse(llm.Response{Usage: llm.Usage{InputTokens: 100}})
	current.Commit()
	current.AddModelResponse(llm.Response{})
	want := []prefixToken{
		{committedPrefixIndex: 1, Tokens: 100},
		{committedPrefixIndex: 1, Tokens: 0},
	}
	if !reflect.DeepEqual(current.prefixTokens, want) {
		t.Fatalf("checkpoints = %#v, want %#v", current.prefixTokens, want)
	}
	built, err := current.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(built.Request.Input, withPreamble()) {
		t.Fatalf("empty entries changed request input: %#v", built.Request.Input)
	}
}

func TestCompactionStartsNewTokenCheckpoints(t *testing.T) {
	current := newCompactionHistory(t)
	current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "large result"}}, false)
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "new input"})
	original := append([]prefixToken(nil), current.prefixTokens...)
	summary := usableCompactionResponse("summary")
	summary.Usage = llm.Usage{InputTokens: 200_000, OutputTokens: 4_000}
	fresh, ok := current.Compact(summary)
	if !ok {
		t.Fatal("compaction failed")
	}
	rebuilt := fresh.(*builder)
	if len(rebuilt.prefixTokens) != 0 {
		t.Fatal("compaction retained checkpoints from the old context")
	}
	if _, ok := rebuilt.compactionCutoff(); ok {
		t.Fatal("compaction reused unmeasured context")
	}
	before := buildCompactionContext(t, rebuilt).Request.Input
	rebuilt.Commit()
	response := usableCompactionResponse("F output")
	response.Usage = llm.Usage{InputTokens: 49_000, OutputTokens: 1_000}
	rebuilt.AddModelResponse(response)
	want := append(before, response.Output...)
	if !reflect.DeepEqual(rebuilt.committedPrefix, want) {
		t.Fatal("recording the first checkpoint changed context items")
	}
	checkpoints := []prefixToken{{committedPrefixIndex: len(want), Tokens: 50_000}}
	if !reflect.DeepEqual(rebuilt.prefixTokens, checkpoints) {
		t.Fatalf("first response checkpoints = %#v, want %#v", rebuilt.prefixTokens, checkpoints)
	}
	cutoff, ok := rebuilt.compactionCutoff()
	if !ok || rebuilt.prefixTokens[cutoff].committedPrefixIndex != len(want) {
		t.Fatal("first response's input was excluded from the compaction cut")
	}
	addCompactionTestTurn(t, rebuilt, 6, 60_000)
	checkpoints = append(checkpoints, prefixToken{committedPrefixIndex: len(want) + 3, Tokens: 60_000})
	if !reflect.DeepEqual(rebuilt.prefixTokens, checkpoints) {
		t.Fatalf("subsequent checkpoints = %#v, want %#v", rebuilt.prefixTokens, checkpoints)
	}
	second, ok := rebuilt.Compact(summary)
	if !ok || len(second.(*builder).prefixTokens) != 0 {
		t.Fatal("second compaction did not reset checkpoints")
	}
	want = withPreamble(llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "summary"}})
	want = append(want, rebuilt.committedPrefix[len(before)+len(response.Output):]...)
	if got := buildCompactionContext(t, second).Request.Input; !reflect.DeepEqual(got, want) {
		t.Fatalf("second compaction = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(current.prefixTokens, original) {
		t.Fatal("compaction changed the original builder's checkpoints")
	}
}
