package contextbuilder

import (
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"reflect"
	"strings"
	"testing"
)

func TestUnusableCompactionPreservesOriginal(t *testing.T) {
	for _, response := range []llm.Response{
		{Stop: llm.StopComplete},
		usableCompactionResponse(" \t\n"),
		{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "not a summary"}}}},
		{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "partial"}}}},
		{Stop: llm.StopRefused, Output: usableCompactionResponse("refusal").Output},
		{Stop: llm.StopComplete, Failure: &llm.Failure{Code: "failed"}, Output: usableCompactionResponse("partial").Output},
	} {
		current := newCompactionHistory(t)
		before := buildCompactionContext(t, current)
		rebuilt, ok := current.Compact(response)
		if ok || rebuilt != current || !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
			t.Fatalf("unusable response changed history: %+v", response)
		}
	}
}

func TestFailedModelOutputIsNotContext(t *testing.T) {
	current := newCompactionHistory(t)
	before := buildCompactionContext(t, current)
	current.AddModelResponse(llm.Response{Failure: &llm.Failure{Code: "failed"}, Output: []llm.Item{
		{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "partial"}},
		{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "partial"}},
	}})
	if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
		t.Fatal("failed output entered model context")
	}
}

func FuzzCompactionSummarySafety(f *testing.F) {
	f.Add("", uint8(0), uint8(0), false)
	f.Add(" \n\t", uint8(0), uint8(0), false)
	f.Add("Summary", uint8(1), uint8(0), false)
	f.Add("Summary", uint8(0), uint8(0), false)
	f.Add("Summary", uint8(0), uint8(0), true)
	f.Add("Summary", uint8(0), uint8(1), false)
	f.Fuzz(func(t *testing.T, text string, roleIndex, stopIndex uint8, failed bool) {
		if len(text) > 4096 {
			t.Skip()
		}
		current := newCompactionHistory(t)
		before := buildCompactionContext(t, current)
		roles := []llm.Role{llm.RoleAssistant, llm.RoleUser, llm.RoleSystem}
		stops := []llm.StopReason{llm.StopComplete, llm.StopRefused, llm.StopMaxOutputTokens, ""}
		response := llm.Response{Stop: stops[int(stopIndex)%len(stops)], Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: roles[int(roleIndex)%len(roles)], Text: text}}}}
		if failed {
			response.Failure = &llm.Failure{Code: "failed"}
		}
		rebuilt, ok := current.Compact(response)
		usable := response.Stop == llm.StopComplete && !failed && roles[int(roleIndex)%len(roles)] == llm.RoleAssistant && strings.TrimSpace(text) != ""
		if ok != usable {
			t.Fatalf("accepted=%t usable=%t", ok, usable)
		}
		if !reflect.DeepEqual(before, buildCompactionContext(t, current)) {
			t.Fatal("original builder mutated")
		}
		if !usable && rebuilt != current {
			t.Fatal("unusable summary replaced history")
		}
	})
}

func TestUsageWithoutHistoryCannotCompact(t *testing.T) {
	current := NewBuilder()
	current.AddModelResponse(llm.Response{Usage: llm.Usage{InputTokens: 30_000}})
	if _, err := current.BuildCompaction(); err == nil {
		t.Fatal("compaction accepted usage without history")
	}
	if rebuilt, ok := current.Compact(usableCompactionResponse("summary")); ok || rebuilt != current {
		t.Fatal("summary replaced an empty history")
	}
}

// This shape matches messagesapi.response for stop_reason=refusal: text becomes
// an assistant message and stop_details supplies Failure, alongside StopRefused.
func TestRefusalWithFailurePreservesAssistantHistory(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		current := newCompactionHistory(t)
		before := buildCompactionContext(t, current)
		message := llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "I cannot help with that."}}
		response := llm.Response{Stop: llm.StopRefused, Failure: &llm.Failure{Code: "policy_violation", Message: "Provider explanation"}, Output: []llm.Item{message}, Usage: llm.Usage{TokenUsage: llm.TokenUsage{InputTokens: 123, OutputTokens: 4}}}
		if mixed {
			response.Output = append(response.Output, llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "unsafe"}}, llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "not assistant output"}})
		}
		originalOutput := append([]llm.Item(nil), response.Output...)
		current.AddModelResponse(response)
		after := buildCompactionContext(t, current)
		want := append(append([]llm.Item(nil), before.Request.Input...), message)
		if !reflect.DeepEqual(after.Request.Input, want) {
			t.Fatalf("refusal history = %+v, want %+v", after.Request.Input, want)
		}
		if !reflect.DeepEqual(response.Output, originalOutput) {
			t.Fatal("response output was mutated while filtering")
		}
		if got := current.prefixTokens; got[len(got)-1].Tokens != 127 {
			t.Fatal("refusal usage was not retained")
		}
	}
}
