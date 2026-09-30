package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

const firstFallback = `{"type":"fallback","from":{"model":"primary"},"to":{"model":"second"},"trigger":{"type":"refusal","category":null},"extra":9007199254740993}`
const lastFallback = `{"type":"fallback","from":{"model":"second"},"to":{"model":"final"},"trigger":{"type":"refusal","category":"cyber"},"extra":1e1000}`

func TestFallbackContinuationPersistsAndReplays(t *testing.T) {
	blocks := []string{
		`{"type":"thinking","thinking":"discard unsigned thinking"}`,
		`{"type":"text","text":"First partial answer"}`,
		`{"type":"tool_use","id":"call","name":"discard","input":[],"toolset_name":"unsupported"}`,
		`{"type":"redacted_thinking"}`,
		firstFallback,
		`{"type":"thinking","thinking":"discard second model"}`,
		`{"type":"text","text":"Second partial answer"}`,
		`{"type":"tool_use","id":"call","name":"discard","input":{},"caller":{"type":"unsupported"}}`,
		lastFallback,
		`{"type":"thinking","thinking":"Final thinking","signature":"signed","extra":9007199254740993}`,
		`{"type":"redacted_thinking","data":"opaque"}`,
		`{"type":"tool_use","id":"call","name":"Bash","input":{"n":9007199254740993}}`,
		`{"type":"text","text":"Final answer"}`,
	}
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("streaming=%v", streaming), func(t *testing.T) {
			wantIndices := []int{4, 8, 9, 10, 11, 12}
			responseBlocks := []string{firstFallback, lastFallback}
			responseBlocks = append(responseBlocks, blocks[9:]...)
			body := responseBody("tool_use", "["+strings.Join(responseBlocks, ",")+"]")
			if streaming {
				wantIndices = []int{1, 4, 6, 8, 9, 10, 11, 12}
				var state streamingState
				observeEvents(t, &state, streamMessageStart)
				for index, block := range blocks {
					observeEvents(t, &state,
						fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":%s}`, index*2, block),
						fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, index*2),
					)
				}
				observeEvents(t, &state,
					`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":4,"iterations":[{"type":"fallback_message","model":"final","output_tokens":4}]}}`,
					`{"type":"message_stop"}`,
				)
				var err error
				body, err = state.unwrap()
				if err != nil {
					t.Fatal(err)
				}
			}
			response, err := decodeResponse(body)
			if err != nil {
				t.Fatal(err)
			}
			if len(response.Output) != len(wantIndices) {
				t.Fatalf("output = %#v", response.Output)
			}
			calls := 0
			for _, item := range response.Output {
				if item.Type == llm.ItemToolCall {
					calls++
					call := item.Data.(llm.ToolCall)
					if call.Name != "Bash" || call.CallID != "call" {
						t.Fatalf("superseded tool call survived: %#v", item)
					}
				}
			}
			if calls != 1 {
				t.Fatalf("tool calls = %d, want 1", calls)
			}
			persisted, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(persisted), "discard") {
				t.Fatalf("persisted superseded content: %s", persisted)
			}
			var restored llm.Response
			if err := json.Unmarshal(persisted, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(restored, response) {
				t.Fatal("persistence changed the response")
			}
			builder := contextbuilder.NewBuilder()
			builder.SetModel(llm.Model{ID: "primary"})
			if err := builder.AddExternalInput(inbox.Input{ID: "user", Kind: inbox.InputExternal, Payload: jsontext.Value(`"Start"`)}); err != nil {
				t.Fatal(err)
			}
			builder.Commit()
			builder.AddModelResponse(restored)
			builder.AddToolResult("call", []llm.ToolResultOutput{{Kind: llm.ToolResultText, Value: "Done"}}, false)
			built, err := builder.Build()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := requestBody(built.Request, true)
			if err != nil {
				t.Fatal(err)
			}
			var replay struct {
				Messages []struct {
					Role    string           `json:"role"`
					Content []jsontext.Value `json:"content"`
				} `json:"messages"`
			}
			if err := json.Unmarshal(encoded, &replay); err != nil {
				t.Fatal(err)
			}
			if len(replay.Messages) != 3 || replay.Messages[1].Role != "assistant" || len(replay.Messages[1].Content) != len(wantIndices) || len(replay.Messages[2].Content) != 1 {
				t.Fatalf("replay = %s", encoded)
			}
			for position, index := range wantIndices {
				assertFallbackJSONEqual(t, replay.Messages[1].Content[position], []byte(blocks[index]))
			}
		})
	}
}

func assertFallbackJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue, wantValue any
	if err := apijson.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if err := apijson.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}

func TestFallbackStreamFiltersBeforeBuildingToolInput(t *testing.T) {
	var state streamingState
	observeEvents(t, &state,
		streamMessageStart,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"discard","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"unfinished\":"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":`+firstFallback+`}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"kept","name":"Bash","input":{}}}`,
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"n\":9007199254740993}"}}`,
		`{"type":"content_block_stop","index":2}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		`{"type":"message_stop"}`,
	)
	body, err := state.unwrap()
	if err != nil {
		t.Fatal(err)
	}
	response, err := decodeResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Output) != 2 {
		t.Fatalf("response = %#v", response)
	}
	assertFallbackJSONEqual(t, response.Output[0].Data.(llm.ProviderItem).Raw, []byte(firstFallback))
	if call := response.Output[1].Data.(llm.ToolCall); call.CallID != "kept" || call.Arguments != `{"n":9007199254740993}` {
		t.Fatalf("tool call = %#v", call)
	}
}

func TestFallbackPreservesFinalResponseValidation(t *testing.T) {
	for _, block := range []string{
		`{"type":"thinking","thinking":"unsigned"}`,
		`{"type":"redacted_thinking"}`,
		`{"type":"tool_use","id":"call","name":"Bash","input":[]}`,
		`{"type":"fallback","from":{"model":"primary"}}`,
		`{"type":"fallback","to":{"model":"fallback"}}`,
	} {
		if _, err := decodeResponse(responseBody("end_turn", "["+firstFallback+","+block+"]")); err == nil {
			t.Fatalf("accepted invalid final block %s", block)
		}
	}
}

func TestFallbackRejectsUnsupportedBlocks(t *testing.T) {
	for _, kind := range []string{
		"server_tool_use", "connector_text", "container_upload",
		"web_search_tool_result", "web_fetch_tool_result", "code_execution_tool_result",
		"bash_code_execution_tool_result", "text_editor_code_execution_tool_result", "tool_search_tool_result",
	} {
		t.Run(kind, func(t *testing.T) {
			block := fmt.Sprintf(`{"type":%q}`, kind)
			for _, content := range []string{block, block + "," + firstFallback, firstFallback + "," + block} {
				if _, err := decodeResponse(responseBody("end_turn", "["+content+"]")); err == nil || !strings.Contains(err.Error(), "unsupported output block type") {
					t.Fatalf("content = %s, error = %v", content, err)
				}
			}
			var state streamingState
			observeEvents(t, &state, streamMessageStart)
			event := fmt.Sprintf(`{"type":"content_block_start","index":0,"content_block":%s}`, block)
			if err := state.observe([]byte(event)); err == nil || !strings.Contains(err.Error(), "unsupported output block type") {
				t.Fatalf("streamed block = %s, error = %v", block, err)
			}
		})
	}
}

func TestFallbackBlockDoesNotChangeStopReason(t *testing.T) {
	for stop, want := range map[string]llm.StopReason{"end_turn": llm.StopComplete, "max_tokens": llm.StopMaxOutputTokens, "refusal": llm.StopRefused} {
		response, err := decodeResponse(responseBody(stop, "["+firstFallback+"]"))
		if err != nil {
			t.Fatal(err)
		}
		if response.Stop != want || (response.Failure != nil) != (want == llm.StopRefused) {
			t.Fatalf("response = %#v", response)
		}
	}
}
