package contextbuilder

import (
	"encoding/json/jsontext"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestPartialModelResponseKeepsOnlyAssistantMessages(t *testing.T) {
	for _, stop := range []llm.StopReason{llm.StopInterrupted, llm.StopMaxOutputTokens, llm.StopRefused, "", "unknown"} {
		t.Run(string(stop), func(t *testing.T) {
			assistant := llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Partial answer"}}
			output := []llm.Item{
				{Type: llm.ItemProvider, ProviderID: "reasoning-1", Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning","encrypted_content":"opaque"}`)}},
				assistant,
				// Syntactically complete arguments do not make an incomplete response executable.
				{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-1", Name: "shell", Arguments: `{"command":"echo unsafe"}`}},
				{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "not assistant"}},
				{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleSystem, Text: "not assistant"}},
				{Type: llm.ItemMessage, Data: "invalid message"},
				{Type: llm.ItemMessage, Data: (*llm.Message)(nil)},
				{Type: llm.ItemProvider, Data: llm.Message{Role: llm.RoleAssistant, Text: "wrong item type"}},
				{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "call-1"}},
			}
			current := NewBuilder().(*builder)
			current.AddModelResponse(llm.Response{Stop: stop, Output: output, Usage: llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 123, OutputTokens: 4}}})
			result, err := current.Build()
			if err != nil {
				t.Fatal(err)
			}
			if want := withPreamble(assistant); !reflect.DeepEqual(result.Request.Input, want) {
				t.Fatalf("next request input = %#v, want %#v", result.Request.Input, want)
			}
			if want := []prefixToken{{committedPrefixIndex: 2, Tokens: 127}}; !reflect.DeepEqual(current.prefixTokens, want) {
				t.Fatalf("usage checkpoints = %#v, want %#v", current.prefixTokens, want)
			}
		})
	}
}

func TestPartialModelResponseWithoutAssistantStillRecordsKnownUsage(t *testing.T) {
	current := NewBuilder().(*builder)
	current.AddModelResponse(llm.Response{
		Stop:   llm.StopMaxOutputTokens,
		Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-1", Name: "shell", Arguments: `{}`}}},
		Usage:  llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 20, OutputTokens: 5}},
	})
	result, err := current.Build()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.Request.Input, withPreamble()) {
		t.Fatalf("next request input = %#v, want preamble only", result.Request.Input)
	}
	if want := []prefixToken{{committedPrefixIndex: 1, Tokens: 25}}; !reflect.DeepEqual(current.prefixTokens, want) {
		t.Fatalf("usage checkpoints = %#v, want %#v", current.prefixTokens, want)
	}
}

func TestCompleteModelResponseRetainsCallsAndProviderState(t *testing.T) {
	output := []llm.Item{
		{Type: llm.ItemProvider, ProviderID: "reasoning-1", Data: llm.ProviderItem{Type: "reasoning", Raw: jsontext.Value(`{"type":"reasoning","encrypted_content":"opaque"}`)}},
		{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-1", Name: "shell", Arguments: `{"command":"echo safe"}`}},
	}
	current := NewBuilder()
	current.AddModelResponse(llm.Response{Stop: llm.StopComplete, Output: output})
	result, err := current.Build()
	if err != nil {
		t.Fatal(err)
	}
	if want := withPreamble(output...); !reflect.DeepEqual(result.Request.Input, want) {
		t.Fatalf("next request input = %#v, want %#v", result.Request.Input, want)
	}
}
