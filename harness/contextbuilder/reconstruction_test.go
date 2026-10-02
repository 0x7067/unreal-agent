package contextbuilder

import (
	"encoding/json/jsontext"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestCompactionRebuildsContextAndPreservesStaging(t *testing.T) {
	for _, completion := range []string{"during compaction", "after compaction"} {
		t.Run(completion, func(t *testing.T) {
			current := NewBuilder(tool.Skill{Name: "review", Description: "Review code", Path: "/skills/review"}).(*builder)
			limit := int64(4096)
			current.SetModel(llm.Model{ID: "model", MaxOutputTokens: &limit, ReasoningEffort: llm.ReasoningEffortHigh})
			current.SetSystemPrompt("System instructions")
			current.AddTool(llm.Tool{Name: "test", Type: llm.ToolFunction})
			for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
				var output []llm.Item
				if turn == 3 {
					output = []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "test"}}}
				}
				if turn == 4 {
					current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
				}
				addCompactionTestTurn(t, current, turn, total, output...)
			}
			current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "before"})
			before := buildCompactionContext(t, current)
			original := append([]llm.Item(nil), before.Request.Input...)
			if _, ok := current.compactionCutoff(); !ok {
				t.Fatal("expected eligible prefix")
			}
			current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "during"})
			if completion == "during compaction" {
				current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
			}
			current.SetSystemPrompt("Updated instructions")
			response := usableCompactionResponse("First summary")
			response.Output = append([]llm.Item{{ProviderID: "reasoning", Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "summary reasoning"}}}}, response.Output...)
			response.Output[1].ProviderID = "summary-message"
			response.Output = append(response.Output, llm.Item{ProviderID: "summary-continuation", Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Next steps"}})
			response.Usage = llm.Usage{InputTokens: 1_000_000, OutputTokens: 50_000}
			rebuilt, ok := current.Compact(response)
			if !ok || rebuilt == current {
				t.Fatal("summary did not produce a fresh builder")
			}
			if completion == "after compaction" {
				rebuilt.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
				current.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
			}
			accepted := buildCompactionContext(t, current)
			built := buildCompactionContext(t, rebuilt)
			start := 0
			for index, item := range accepted.Request.Input {
				if item.Type == llm.ItemMessage && item.Data.(llm.Message).Text == "D input" {
					start = index
					break
				}
			}
			want := accepted.Request
			want.Input = []llm.Item{accepted.Request.Input[0], {Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Next steps"}}}
			want.Input = append(want.Input, accepted.Request.Input[start:]...)
			if start == 0 || !reflect.DeepEqual(built.Request, want) {
				t.Fatalf("rebuilt request = %#v, want %#v", built.Request, want)
			}
			if !reflect.DeepEqual(before.Request.Input, original) {
				t.Fatal("compaction changed an earlier request snapshot")
			}
			if _, ok := rebuilt.(*builder).compactionCutoff(); ok {
				t.Fatal("old-context or summary-request usage was reused")
			}
			again, compacted := rebuilt.Compact(response)
			if compacted || again != rebuilt {
				t.Fatal("duplicate response reused old-context usage")
			}
		})
	}
}

func TestCompactionReplacesPreviousSummaryUsingNewUsage(t *testing.T) {
	current := newCompactionHistory(t)
	if _, ok := current.compactionCutoff(); !ok {
		t.Fatal("expected eligible prefix")
	}
	fresh, ok := current.Compact(usableCompactionResponse("First summary"))
	if !ok {
		t.Fatal("first compaction failed")
	}
	rebuilt := fresh.(*builder)
	addCompactionTestTurn(t, rebuilt, 5, 20_000)
	if _, ok := rebuilt.compactionCutoff(); ok {
		t.Fatal("context within the retention budget was compacted")
	}
	addCompactionTestTurn(t, rebuilt, 6, 35_000)
	if _, ok := rebuilt.compactionCutoff(); !ok {
		t.Fatal("two comparable new checkpoints did not allow compaction")
	}
	second, ok := rebuilt.Compact(usableCompactionResponse("Second summary"))
	if !ok {
		t.Fatal("second compaction failed")
	}
	built := buildCompactionContext(t, second)
	want := withPreamble(
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Second summary"}},
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "G input"}},
		llm.Item{Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "G reasoning"}}},
		llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "G output"}},
	)
	if !reflect.DeepEqual(built.Request.Input, want) {
		t.Fatalf("second context = %#v, want %#v", built.Request.Input, want)
	}
}

func TestCompactionUsesLastAssistantMessage(t *testing.T) {
	for _, phase := range []string{"final_answer", "commentary", ""} {
		t.Run(phase, func(t *testing.T) {
			current := newCompactionHistory(t)
			response := usableCompactionResponse("I am summarizing the conversation.")
			response.Output[0].Data = llm.Message{Role: llm.RoleAssistant, Text: "I am summarizing the conversation.", Phase: "commentary"}
			response.Output = append(response.Output,
				llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Last summary", Phase: phase}},
				llm.Item{Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning"}`), Display: &llm.ProviderDisplay{Kind: llm.ProviderDisplayReasoning, Text: "reasoning"}}},
				llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "not an assistant summary"}},
				llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: " \n"}},
			)
			rebuilt, ok := current.Compact(response)
			if !ok {
				t.Fatal("compaction failed")
			}
			got := buildCompactionContext(t, rebuilt).Request.Input[1].Data.(llm.Message)
			if got != (llm.Message{Role: llm.RoleUser, Text: "Last summary"}) {
				t.Fatalf("summary = %#v", got)
			}
		})
	}
}

func TestCompactionSelectionStaysStableAcrossStagedArrivals(t *testing.T) {
	current := NewBuilder().(*builder)
	for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
		var output []llm.Item
		if turn == 1 {
			output = []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "pending", Name: "test"}}}
		}
		addCompactionTestTurn(t, current, turn, total, output...)
	}
	before, ok := current.compactionCutoff()
	if !ok {
		t.Fatal("expected eligible prefix")
	}
	current.AddToolResult("pending", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: ""}}, true)
	current.AddControlMessage(inbox.ControlMessage{Mode: inbox.Heartbeat, Reason: "during"})
	current.AddToolResult("pending", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "done"}}, false)
	after, ok := current.compactionCutoff()
	if !ok || after != before {
		t.Fatalf("staged arrivals changed the cut from %d to %d (%t)", before, after, ok)
	}
}

func newCompactionHistory(t *testing.T) *builder {
	t.Helper()
	current := NewBuilder().(*builder)
	for turn, total := range []int64{10_000, 18_000, 35_000, 43_000, 55_000} {
		addCompactionTestTurn(t, current, turn, total)
	}
	return current
}

func usableCompactionResponse(text string) llm.Response {
	return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{
		Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: text},
	}}}
}

func buildCompactionContext(t *testing.T, current Builder) Result {
	t.Helper()
	built, err := current.Build()
	if err != nil {
		t.Fatal(err)
	}
	return built
}
