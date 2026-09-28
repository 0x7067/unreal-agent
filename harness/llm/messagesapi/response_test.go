package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/internal/anthropicapi"
)

func responseBody(stop, content string) []byte {
	return fmt.Appendf(nil, `{"type":"message","id":"msg-1","role":"assistant","model":"claude-test","stop_reason":%q,"content":%s,"usage":{"input_tokens":10,"output_tokens":5}}`, stop, content)
}

func TestResponsePreservesBlockOrderAndThinking(t *testing.T) {
	const thinking = `{"type":"thinking","thinking":"Considering tools","signature":"signed","provider_state":{"v":2}}`
	const redacted = `{"type":"redacted_thinking","data":"opaque"}`
	const omitted = `{"type":"thinking","thinking":"","signature":"omitted-but-signed"}`
	decoded, err := decodeResponse(responseBody("tool_use", `[`+thinking+`,
		{"type":"text","text":"First"},
		{"type":"tool_use","id":"a","name":"Bash","input":{"number":9007199254740993},"caller":{"type":"direct"}},
		`+redacted+`,
		{"type":"text","text":"Second"},
		`+omitted+`,
		{"type":"tool_use","id":"b","name":"Bash","input":{}}
	]`))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.ID != "msg-1" || decoded.Stop != llm.StopComplete || decoded.Failure != nil || len(decoded.Output) != 7 {
		t.Fatalf("response = %#v", decoded)
	}
	wantTypes := []llm.ItemType{llm.ItemReasoning, llm.ItemMessage, llm.ItemToolCall, llm.ItemReasoning, llm.ItemMessage, llm.ItemReasoning, llm.ItemToolCall}
	for i, item := range decoded.Output {
		if item.Type != wantTypes[i] {
			t.Fatalf("block %d: %#v", i, item)
		}
		if err := item.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for index, want := range map[int]string{0: thinking, 3: redacted, 5: omitted} {
		reasoning := decoded.Output[index].Data.(llm.Reasoning)
		if string(reasoning.Raw) != want {
			t.Fatalf("thinking block %d changed: %s", index, reasoning.Raw)
		}
		if index == 0 && !reflect.DeepEqual(reasoning.Summary, []string{"Considering tools"}) {
			t.Fatalf("thinking summary = %#v", reasoning.Summary)
		}
		if index != 0 && len(reasoning.Summary) != 0 {
			t.Fatalf("invented reasoning summary = %#v", reasoning.Summary)
		}
	}
	if decoded.Output[1].Data.(llm.Message).Text != "First" || decoded.Output[4].Data.(llm.Message).Text != "Second" {
		t.Fatal("text blocks changed")
	}
	call := decoded.Output[2].Data.(llm.ToolCall)
	if call.CallID != "a" || call.Name != "Bash" || call.Arguments != `{"number":9007199254740993}` || decoded.Output[2].ProviderID != "a" {
		t.Fatalf("call = %#v", decoded.Output[2])
	}
	input := append([]llm.Item{message(llm.RoleUser, "Start")}, decoded.Output...)
	input = append(input, toolResult("b", "B", false), toolResult("a", "A", false))
	body, _ := encodeRequest(t, llm.Request{Model: llm.Model{ID: "claude-test"}, Input: input})
	var replay struct {
		Messages []struct {
			Content []jsontext.Value `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &replay); err != nil {
		t.Fatal(err)
	}
	for index, want := range map[int]string{0: thinking, 3: redacted, 5: omitted} {
		var gotFields, wantFields map[string]any
		if err := json.Unmarshal(replay.Messages[1].Content[index], &gotFields); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(want), &wantFields); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotFields, wantFields) {
			t.Fatalf("replayed thinking %d changed: %s", index, replay.Messages[1].Content[index])
		}
	}
}

func TestResponseStopReasons(t *testing.T) {
	for stop, want := range map[string]llm.StopReason{
		"end_turn": llm.StopComplete, "tool_use": llm.StopComplete, "stop_sequence": llm.StopComplete,
		"max_tokens": llm.StopMaxOutputTokens, "model_context_window_exceeded": llm.StopMaxOutputTokens,
		"refusal": llm.StopRefused,
	} {
		t.Run(stop, func(t *testing.T) {
			decoded, err := decodeResponse(responseBody(stop, `[{"type":"text","text":"Answer"}]`))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Stop != want || len(decoded.Output) != 1 {
				t.Fatalf("response = %#v", decoded)
			}
			if (decoded.Failure != nil) != (want == llm.StopRefused) {
				t.Fatalf("failure = %#v", decoded.Failure)
			}
		})
	}
	for _, stop := range []string{"pause_turn", "unknown", ""} {
		if _, err := decodeResponse(responseBody(stop, `[]`)); err == nil {
			t.Fatalf("accepted unsupported stop reason %q", stop)
		}
	}
}

func TestResponseRefusalDetails(t *testing.T) {
	for _, test := range []struct {
		name    string
		details string
		want    llm.Failure
	}{
		{"absent", "", llm.Failure{Code: "refusal", Message: "response refused"}},
		{"null", `,"stop_details":null`, llm.Failure{Code: "refusal", Message: "response refused"}},
		{"full", `,"stop_details":{"type":"refusal","category":"policy_violation","explanation":"Provider explanation"}`, llm.Failure{Code: "policy_violation", Message: "Provider explanation"}},
		{"category only", `,"stop_details":{"type":"refusal","category":"policy_violation","explanation":null}`, llm.Failure{Code: "policy_violation", Message: "response refused"}},
		{"explanation only", `,"stop_details":{"type":"refusal","category":null,"explanation":"Provider explanation"}`, llm.Failure{Code: "refusal", Message: "Provider explanation"}},
		{"null fields", `,"stop_details":{"type":"refusal","category":null,"explanation":null}`, llm.Failure{Code: "refusal", Message: "response refused"}},
		{"empty fields", `,"stop_details":{"type":"refusal","category":"","explanation":""}`, llm.Failure{Code: "refusal", Message: "response refused"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := responseBody("refusal", `[]`)
			body = append(body[:len(body)-1], test.details+"}"...)
			decoded, err := decodeResponse(body)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Stop != llm.StopRefused || len(decoded.Output) != 0 || decoded.Failure == nil || *decoded.Failure != test.want {
				t.Fatalf("response = %#v, want failure %#v", decoded, test.want)
			}
		})
	}
}

func TestResponsePreservesFinalCallsAtTokenLimit(t *testing.T) {
	for _, stop := range []string{"max_tokens", "model_context_window_exceeded"} {
		for _, input := range []string{`{}`, `{"command":"ls"}`} {
			decoded, err := decodeResponse(responseBody(stop, `[
				{"type":"tool_use","id":"complete","name":"Bash","input":{"command":"pwd"}},
				{"type":"text","text":"Another command"},
				{"type":"tool_use","id":"last","name":"Bash","input":`+input+`}
			]`))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Stop != llm.StopMaxOutputTokens || len(decoded.Output) != 3 || decoded.Output[0].Data.(llm.ToolCall).CallID != "complete" || decoded.Output[1].Type != llm.ItemMessage || decoded.Output[2].Data.(llm.ToolCall).CallID != "last" {
				t.Fatalf("response = %#v", decoded)
			}
		}
	}
}

func TestResponsePreservesToolArguments(t *testing.T) {
	for _, input := range []string{
		`{}`,
		`{
  "z": [9007199254740993, 1e1000, -0, 1.2300],
  "a": {"escaped": "\u0061\/", "value": null}
}`,
	} {
		decoded, err := decodeResponse(responseBody("tool_use", `[{"type":"tool_use","id":"a","name":"Bash","input":`+input+`}]`))
		if err != nil {
			t.Fatal(err)
		}
		call := decoded.Output[0].Data.(llm.ToolCall)
		if call.Arguments != input {
			t.Fatalf("arguments = %s, want %s", call.Arguments, input)
		}
	}
}

func TestResponseRejectsUnsupportedBlocks(t *testing.T) {
	for name, content := range map[string]string{
		"server tool":        `{"type":"server_tool_use","id":"server-1","name":"web_search","input":{}}`,
		"server result":      `{"type":"web_search_tool_result","tool_use_id":"server-1","content":[]}`,
		"citations":          `{"type":"text","text":"Cited answer","citations":[{"type":"web_search_result_location","encrypted_index":"reference"}]}`,
		"toolset":            `{"type":"tool_use","id":"a","name":"click","toolset_name":"browser","input":{}}`,
		"programmatic call":  `{"type":"tool_use","id":"a","name":"Bash","input":{},"caller":{"type":"code_execution_20260120","tool_id":"server-1"}}`,
		"unknown block":      `{"type":"unknown"}`,
		"missing signature":  `{"type":"thinking","thinking":"Not signed"}`,
		"missing ciphertext": `{"type":"redacted_thinking"}`,
		"missing input":      `{"type":"tool_use","id":"a","name":"Bash"}`,
		"nonobject input":    `{"type":"tool_use","id":"a","name":"Bash","input":[]}`,
		"null input":         `{"type":"tool_use","id":"a","name":"Bash","input":null}`,
		"string input":       `{"type":"tool_use","id":"a","name":"Bash","input":"{}"}`,
		"number input":       `{"type":"tool_use","id":"a","name":"Bash","input":42}`,
		"boolean input":      `{"type":"tool_use","id":"a","name":"Bash","input":true}`,
		"duplicate argument": `{"type":"tool_use","id":"a","name":"Bash","input":{"nested":{"x":1,"x":2}}}`,
		"malformed input":    `{"type":"tool_use","id":"a","name":"Bash","input":{"x":}}`,
		"missing call id":    `{"type":"tool_use","name":"Bash","input":{}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeResponse(responseBody("end_turn", `[`+content+`]`)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestResponseRejectsDuplicateCalls(t *testing.T) {
	call := `{"type":"tool_use","id":"a","name":"Bash","input":{}}`
	_, err := decodeResponse(responseBody("tool_use", `[`+call+`,`+call+`]`))
	if err == nil || !strings.Contains(err.Error(), "duplicate tool call") {
		t.Fatalf("error = %v", err)
	}
}

func TestResponseUsage(t *testing.T) {
	const usage = `{"input_tokens":7,"cache_read_input_tokens":30,"cache_creation_input_tokens":20,"output_tokens":11,"output_tokens_details":{"thinking_tokens":5},"extra":{"cost":0.25}}`
	decoded, err := decodeResponse([]byte(`{"id":"msg-1","type":"message","role":"assistant","stop_reason":"end_turn","content":[],"usage":` + usage + `}`))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Usage.InputTokens != 57 || decoded.Usage.CachedInputTokens != 30 || decoded.Usage.CacheWriteInputTokens != 20 ||
		decoded.Usage.OutputTokens != 11 || decoded.Usage.ReasoningTokens != 5 || string(decoded.Usage.Raw) != usage {
		t.Fatalf("usage = %#v", decoded.Usage)
	}
	withoutCache := responseUsage(anthropicapi.Usage{InputTokens: 7, OutputTokens: 11})
	if withoutCache.InputTokens != 7 || withoutCache.OutputTokens != 11 || withoutCache.CachedInputTokens != 0 || withoutCache.CacheWriteInputTokens != 0 {
		t.Fatalf("usage without cache = %#v", withoutCache)
	}
}

func TestResponseRejectsInvalidEnvelopes(t *testing.T) {
	for _, body := range []string{
		`{`, `{}`, `{"type":"error","error":{"message":"Failure"}}`,
		`{"type":"message","role":"assistant","stop_reason":null}`,
		`{"type":"message","role":"assistant","stop_reason":"end_turn","usage":[]}`,
		`{"type":"message","role":"assistant","stop_reason":"end_turn","usage":{"input_tokens":"invalid"}}`,
		`{"type":"message","role":"assistant","stop_reason":"end_turn","usage":{},"usage":null}`,
	} {
		if _, err := decodeResponse([]byte(body)); err == nil {
			t.Fatalf("accepted invalid response %s", body)
		}
	}
}

func TestResponseOptionalUsage(t *testing.T) {
	for _, usage := range []string{"", "null", "{}"} {
		body := `{"type":"message","role":"assistant","stop_reason":"end_turn","content":[]`
		if usage != "" {
			body += `,"usage":` + usage
		}
		decoded, err := decodeResponse([]byte(body + "}"))
		if err != nil {
			t.Fatal(err)
		}
		var want llm.Usage
		if usage != "" {
			want.Raw = jsontext.Value(usage)
		}
		if !reflect.DeepEqual(decoded.Usage, want) {
			t.Fatalf("usage = %#v, want %#v", decoded.Usage, want)
		}
	}
}
