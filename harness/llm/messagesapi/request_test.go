package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

type wireBlock struct {
	Type      string            `json:"type"`
	Text      string            `json:"text"`
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Input     jsontext.Value    `json:"input"`
	ToolUseID string            `json:"tool_use_id"`
	Content   []wireBlock       `json:"content"`
	Source    map[string]string `json:"source"`
}

type wireMessage struct {
	Role    string      `json:"role"`
	Content []wireBlock `json:"content"`
}

type wireRequest struct {
	Model        string           `json:"model"`
	MaxTokens    int              `json:"max_tokens"`
	Stream       bool             `json:"stream"`
	CacheControl map[string]any   `json:"cache_control"`
	System       []wireBlock      `json:"system"`
	Messages     []wireMessage    `json:"messages"`
	Thinking     map[string]any   `json:"thinking"`
	OutputConfig map[string]any   `json:"output_config"`
	Tools        []map[string]any `json:"tools"`
}

func message(role llm.Role, text string) llm.Item {
	return llm.Item{Type: llm.ItemMessage, Data: llm.Message{Role: role, Text: text}}
}

func toolCall(id string) llm.Item {
	return llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: id, Name: "Bash", Arguments: `{}`}}
}

func toolResult(id, text string, running bool) llm.Item {
	return llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{
		CallID: id, Running: running, Output: []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: text}},
	}}
}

func encodeRequest(t *testing.T, request llm.Request) ([]byte, wireRequest) {
	t.Helper()
	body, err := requestBody(request, false)
	if err != nil {
		t.Fatal(err)
	}
	var wire wireRequest
	if err := apijson.Unmarshal(body, &wire); err != nil {
		t.Fatal(err)
	}
	return body, wire
}

func TestRequestSettingsAndTools(t *testing.T) {
	request := llm.Request{
		Model: llm.Model{ID: "claude-test", MaxOutputTokens: new(int64(16384)), ReasoningEffort: llm.ReasoningEffortHigh},
		Input: []llm.Item{
			message(llm.RoleSystem, "System instructions"),
			message(llm.RoleUser, "First"), message(llm.RoleUser, "Second"),
		},
		Tools: []llm.Tool{{Type: llm.ToolFunction, Name: "Bash", Description: "Run a command", Parameters: map[string]any{
			"type": "object", "required": []string{"command"}, "additionalProperties": false,
			"properties": map[string]any{"command": map[string]any{"type": "string"}},
		}}},
	}
	_, wire := encodeRequest(t, request)
	if wire.Model != "claude-test" || wire.MaxTokens != 16384 || !wire.Stream || wire.CacheControl["type"] != "ephemeral" {
		t.Fatalf("request settings = %#v", wire)
	}
	if wire.Thinking["type"] != "adaptive" || wire.OutputConfig["effort"] != "high" {
		t.Fatalf("thinking = %#v, output config = %#v", wire.Thinking, wire.OutputConfig)
	}
	if len(wire.System) != 1 || wire.System[0].Text != "System instructions" {
		t.Fatalf("system = %#v", wire.System)
	}
	if len(wire.Messages) != 1 || wire.Messages[0].Role != "user" || len(wire.Messages[0].Content) != 2 {
		t.Fatalf("messages = %#v", wire.Messages)
	}
	if len(wire.Tools) != 1 || wire.Tools[0]["type"] != "custom" || wire.Tools[0]["name"] != "Bash" || wire.Tools[0]["description"] != "Run a command" || wire.Tools[0]["strict"] != false {
		t.Fatalf("tools = %#v", wire.Tools)
	}
	schema := wire.Tools[0]["input_schema"].(map[string]any)
	if schema["type"] != "object" || schema["additionalProperties"] != false || !reflect.DeepEqual(schema["required"], []any{"command"}) {
		t.Fatalf("schema = %#v", schema)
	}
}

func TestRequestDefaultsAndEfforts(t *testing.T) {
	for _, effort := range []llm.ReasoningEffort{"", llm.ReasoningEffortLow, llm.ReasoningEffortMedium, llm.ReasoningEffortHigh, llm.ReasoningEffortXHigh, llm.ReasoningEffortMax} {
		t.Run(string(effort), func(t *testing.T) {
			body, wire := encodeRequest(t, llm.Request{
				Model: llm.Model{ID: "claude-test", ReasoningEffort: effort}, Input: []llm.Item{message(llm.RoleUser, "Hello")},
			})
			if wire.MaxTokens != 128_000 {
				t.Fatalf("max tokens = %d", wire.MaxTokens)
			}
			if effort == "" {
				if wire.Thinking != nil || wire.OutputConfig != nil {
					t.Fatalf("unset thinking configuration was included: %s", body)
				}
			} else if wire.Thinking["type"] != "adaptive" || wire.OutputConfig["effort"] != string(effort) {
				t.Fatalf("effort changed: %s", body)
			}
			var fields map[string]jsontext.Value
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			if _, exists := fields["tools"]; exists {
				t.Fatal("absent tools were included")
			}
		})
	}
}

func TestRequestOrdersPrimaryResultsByCalls(t *testing.T) {
	for _, order := range [][]string{{"a", "b"}, {"b", "a"}} {
		for _, running := range []bool{false, true} {
			request := llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
				message(llm.RoleUser, "Start"),
				toolCall("a"), message(llm.RoleAssistant, "Also checking"), toolCall("b"),
				message(llm.RoleUser, "Steering one"), toolResult(order[0], order[0]+" result", running),
				message(llm.RoleUser, "Steering two"), toolResult(order[1], order[1]+" result", running),
			}}
			_, wire := encodeRequest(t, request)
			assistant := wire.Messages[1].Content
			if len(assistant) != 3 || assistant[0].ID != "a" || assistant[1].Text != "Also checking" || assistant[2].ID != "b" {
				t.Fatalf("assistant order changed: %#v", assistant)
			}
			user := wire.Messages[2].Content
			if len(user) != 4 || user[0].ToolUseID != "a" || user[1].ToolUseID != "b" || user[2].Text != "Steering one" || user[3].Text != "Steering two" {
				t.Fatalf("user blocks = %#v", user)
			}
			if user[0].Content[0].Text != "a result" || user[1].Content[0].Text != "b result" {
				t.Fatalf("results changed: %#v", user)
			}
		}
	}
}

func TestRequestRepeatedResultsBecomeUserContent(t *testing.T) {
	_, wire := encodeRequest(t, llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
		message(llm.RoleUser, "Start"), toolCall("a"),
		toolResult("a", "still running", true), message(llm.RoleUser, "Steering"), toolResult("a", "done", false),
		message(llm.RoleAssistant, "Acknowledged"), toolResult("a", "another update", true),
	}})
	user := wire.Messages[2].Content
	if len(user) != 4 || user[0].Type != "tool_result" || user[0].Content[0].Text != "still running" || user[1].Text != "Steering" ||
		!strings.Contains(user[2].Text, `call "a", completed`) || user[3].Text != "done" {
		t.Fatalf("first user turn = %#v", user)
	}
	user = wire.Messages[4].Content
	if len(user) != 2 || user[0].Type != "text" || !strings.Contains(user[0].Text, `call "a", running`) || user[1].Text != "another update" {
		t.Fatalf("later user turn = %#v", user)
	}
}

func TestRequestToolOutputPreservesImagesAndEmptyResults(t *testing.T) {
	output := []llm.ToolResultOutput{
		{Kind: llm.ToolResultText, Value: "First image"},
		{Kind: llm.ToolResultImage, Value: "data:image/png;base64,aGVsbG8="},
		{Kind: llm.ToolResultText, Value: "Second image"},
		{Kind: llm.ToolResultImage, Value: "https://example.com/image.jpg"},
	}
	result := llm.Item{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "a", Output: output}}
	_, wire := encodeRequest(t, llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
		message(llm.RoleUser, "Start"), toolCall("a"), toolCall("b"), result,
		{Type: llm.ItemToolResult, Data: llm.ToolResult{CallID: "b"}},
		message(llm.RoleAssistant, "More"), result,
	}})
	native := wire.Messages[2].Content[0].Content
	update := wire.Messages[4].Content[1:]
	if !reflect.DeepEqual(native, update) || len(native) != 4 || native[0].Text != "First image" || native[2].Text != "Second image" {
		t.Fatalf("native = %#v, update = %#v", native, update)
	}
	if !reflect.DeepEqual(native[1].Source, map[string]string{"type": "base64", "media_type": "image/png", "data": "aGVsbG8="}) ||
		!reflect.DeepEqual(native[3].Source, map[string]string{"type": "url", "url": "https://example.com/image.jpg"}) {
		t.Fatalf("image sources = %#v", native)
	}
	if len(wire.Messages[2].Content[1].Content) != 0 {
		t.Fatal("empty tool output was replaced with invented content")
	}
}

func TestRequestRejectsInvalidHistory(t *testing.T) {
	for name, test := range map[string]struct {
		items []llm.Item
		want  string
	}{
		"empty":                  {nil, "messages must not be empty"},
		"wrong payload":          {[]llm.Item{{Type: llm.ItemToolCall}}, "data must be llm.ToolCall"},
		"provider without raw":   {[]llm.Item{{Type: llm.ItemProvider, Data: llm.ProviderItem{}}}, "provider item Raw is required"},
		"provider with null raw": {[]llm.Item{{Type: llm.ItemProvider, Data: llm.ProviderItem{Raw: jsontext.Value(`null`)}}}, "provider item Raw is required"},
		"unknown tag":            {[]llm.Item{{Type: "unknown"}}, "unsupported item type"},
		"unknown role":           {[]llm.Item{message("tool", "bad")}, "unsupported role"},
		"late system":            {[]llm.Item{message(llm.RoleUser, "hi"), message(llm.RoleSystem, "late")}, "system message must be the first input item"},
		"second system":          {[]llm.Item{message(llm.RoleSystem, "first"), message(llm.RoleSystem, "second")}, "system message must be the first input item"},
		"missing result":         {[]llm.Item{toolCall("a")}, "missing immediately following results"},
		"missing one result":     {[]llm.Item{toolCall("a"), toolCall("b"), toolResult("a", "done", false)}, `result for tool call "b"`},
		"result too late":        {[]llm.Item{toolCall("a"), message(llm.RoleUser, "hi"), message(llm.RoleAssistant, "wait"), toolResult("a", "done", false)}, `result for tool call "a"`},
		"unknown call":           {[]llm.Item{toolResult("a", "done", false)}, "unknown tool call"},
		"duplicate call":         {[]llm.Item{toolCall("a"), toolCall("a")}, "duplicate tool call"},
		"missing call id":        {[]llm.Item{toolCall("")}, "must have an ID and name"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := requestInput(test.items)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRequestRejectsUnsupportedSettings(t *testing.T) {
	for name, modify := range map[string]func(*llm.Request){
		"model":        func(r *llm.Request) { r.Model.ID = " " },
		"negative max": func(r *llm.Request) { r.Model.MaxOutputTokens = new(int64(-1)) },
		"zero max":     func(r *llm.Request) { r.Model.MaxOutputTokens = new(int64(0)) },
		"effort":       func(r *llm.Request) { r.Model.ReasoningEffort = "extreme" },
		"hosted tool":  func(r *llm.Request) { r.Tools = []llm.Tool{{Type: llm.ToolHosted, Name: "web_search"}} },
		"tool type":    func(r *llm.Request) { r.Tools = []llm.Tool{{Type: "unknown"}} },
		"tool name":    func(r *llm.Request) { r.Tools = []llm.Tool{{Type: llm.ToolFunction}} },
		"schema": func(r *llm.Request) {
			r.Tools = []llm.Tool{{Type: llm.ToolFunction, Name: "Bash", Parameters: map[string]any{"type": "array"}}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{message(llm.RoleUser, "Hello")}}
			modify(&request)
			if _, err := requestBody(request, false); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRequestRejectsUnsupportedToolOutput(t *testing.T) {
	for _, part := range []llm.ToolResultOutput{
		{Kind: "audio", Value: "unsupported"},
		{Kind: llm.ToolResultImage, Value: "file:///tmp/image.png"},
		{Kind: llm.ToolResultImage, Value: "data:image/svg+xml;base64,aGVsbG8="},
		{Kind: llm.ToolResultImage, Value: "data:image/png;base64,%%%"},
		{Kind: llm.ToolResultImage, Value: "data:image/png;base64,"},
	} {
		if _, err := requestToolOutput([]llm.ToolResultOutput{part}); err == nil {
			t.Fatalf("expected error for %#v", part)
		}
	}
}

func TestRequestIsStableAndPreservesNumbers(t *testing.T) {
	const arguments = `{"large":9007199254740993,"precise":0.1234567890123456789,"huge":1e1000}`
	request := llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
		message(llm.RoleUser, "Start"),
		{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "a", Name: "Bash", Arguments: arguments}},
		toolResult("a", "done", false),
	}, Tools: []llm.Tool{{Type: llm.ToolFunction, Name: "Bash", Parameters: map[string]any{
		"type": "object", "const": int64(9007199254740993), "maximum": jsontext.Value(`1e1000`),
	}}}}
	original := slices.Clone(request.Input)
	first, wire := encodeRequest(t, request)
	if string(wire.Messages[1].Content[0].Input) != `{"huge":1e1000,"large":9007199254740993,"precise":0.1234567890123456789}` ||
		!strings.Contains(string(first), `"const":9007199254740993`) || !strings.Contains(string(first), `"maximum":1e1000`) {
		t.Fatalf("numbers changed: %s", first)
	}
	for range 10 {
		next, _ := encodeRequest(t, request)
		if string(next) != string(first) {
			t.Fatal("request encoding is not deterministic")
		}
	}
	if !reflect.DeepEqual(original, request.Input) {
		t.Fatal("request encoding mutated input")
	}
}

func TestRequestReplaysMalformedArgumentsAsAnObject(t *testing.T) {
	for _, arguments := range []string{`{"unfinished":`, `[]`, `null`, ""} {
		_, wire := encodeRequest(t, llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{
			message(llm.RoleUser, "Start"),
			{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "a", Name: "Bash", Arguments: arguments}},
			toolResult("a", "invalid arguments", false),
		}})
		var got map[string]string
		if err := json.Unmarshal(wire.Messages[1].Content[0].Input, &got); err != nil {
			t.Fatal(err)
		}
		if got["invalid_arguments"] != arguments {
			t.Fatalf("arguments = %#v", got)
		}
	}
}

func TestRequestReplaysOpaqueReasoning(t *testing.T) {
	for _, raw := range []string{
		`{"type":"thinking","thinking":"escaped \\ \" \u2028","signature":"\u0073igned","data":{"future":9007199254740993}}`,
		`{"type":"thinking","thinking":null,"signature":"signed","unknown":[1e1000]}`,
		`{"type":"redacted_thinking","data":"opaque","signature":{"future":true},"thinking":42}`,
		`{"type":"thinking","signature":null}`,
		`{"type":"redacted_thinking","data":""}`,
	} {
		request := validRequest()
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(raw), &header); err != nil {
			t.Fatal(err)
		}
		request.Input = append(request.Input, llm.Item{Type: llm.ItemProvider, Data: llm.ProviderItem{Type: header.Type, Raw: jsontext.Value(raw)}})
		body, err := requestBody(request, false)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Messages []struct {
				Content []jsontext.Value `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		var want, got map[string]any
		if err := apijson.Unmarshal([]byte(raw), &want); err != nil {
			t.Fatal(err)
		}
		if err := apijson.Unmarshal(decoded.Messages[1].Content[0], &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("opaque thinking changed: %s", decoded.Messages[1].Content[0])
		}
	}
}

func TestRequestRejectsMalformedReasoningJSON(t *testing.T) {
	for _, raw := range []string{
		`{"type":"thinking","signature":"signed","signature":"duplicate"}`,
		`{"type":"redacted_thinking","data":"unfinished`,
	} {
		request := validRequest()
		request.Input = append(request.Input, llm.Item{Type: llm.ItemProvider, Data: llm.ProviderItem{Type: "thinking", Raw: jsontext.Value(raw)}})
		if _, err := requestBody(request, false); err == nil {
			t.Fatalf("accepted malformed reasoning JSON: %s", raw)
		}
	}
}
