package main

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

func TestModelRequestExhaustionIsVisibleWithoutPretendingAResponse(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	m.responding = true
	r := inbox.ModelRequestRecord{Version: 1, Action: "exhausted", RequestID: "request", TurnID: "turn", TurnType: "regular", Attempt: 2, Limit: 2, Inputs: 1, ExternalInputs: 1}
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ModelRequest, Reason: "No automatic retry; submit new input. Provider billing may be duplicated.", Parameters: r})
	if err != nil {
		t.Fatal(err)
	}
	m.apply(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "exhaustion", Kind: inbox.InputControl, Payload: payload}})
	if m.responding || len(m.lines) != 1 || m.lines[0].label != "Model request exhausted" || !strings.Contains(m.lines[0].text, "No automatic retry") || len(m.tools) != 0 {
		t.Fatal("exhaustion was hidden or displayed as a model/tool result")
	}
}

func TestIncompleteResponseIsNotShownAsPendingTools(t *testing.T) {
	for _, stop := range []llm.StopReason{"", llm.StopInterrupted, llm.StopMaxOutputTokens} {
		t.Run(string(stop), func(t *testing.T) {
			palette, err := loadTheme(defaultTheme)
			if err != nil {
				t.Fatal(err)
			}
			m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
			m.apply(sessionstore.Item{Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "partial", Response: llm.Response{Stop: stop, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "partial", Name: "Bash", Arguments: `{"command":"touch forbidden"}`}}}}}})
			if len(m.tools) != 0 || len(m.lines) != 1 {
				t.Fatal("partial output displayed as pending/executing tools or incomplete notice hidden")
			}
		})
	}
}

func TestCompactionExhaustionRemainsAnExplicitNotice(t *testing.T) {
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(t.Context(), nil, tool.NewRegistry(tool.StaticTranslators{}), "workspace", "sessions", options{theme: palette})
	m.responding = true
	record := inbox.CompactionRequestRecord{Version: 1, Request: inbox.ModelRequestRecord{Version: 1, Action: "exhausted", RequestID: "summary", TurnID: "turn", TurnType: "compaction", Attempt: 3, Limit: 3}, ManualInputID: "manual"}
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.CompactionRequest, Reason: "Original conversation and queued input preserved; no summary invented.", Parameters: record})
	if err != nil {
		t.Fatal(err)
	}
	m.apply(sessionstore.Item{Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "exhausted-summary", Kind: inbox.InputControl, Payload: payload}})
	if m.responding || len(m.lines) != 1 || m.lines[0].label != "Compaction request exhausted" || !strings.Contains(m.lines[0].text, "preserved") || len(m.tools) != 0 {
		t.Fatal("summary exhaustion hidden or rendered as response/tool authority")
	}
}
