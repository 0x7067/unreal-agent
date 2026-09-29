package messagesapi

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

func liveStream(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/live-" + name + ".sse")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func observeStream(state *streamingState, stream []byte) error {
	for frame := range bytes.SplitSeq(stream, []byte("\n\n")) {
		if data := bytes.TrimSpace(primitives.SSEData(frame)); len(data) != 0 {
			if err := state.observe(data); err != nil {
				return err
			}
		}
	}
	return nil
}

func observeEvents(t *testing.T, state *streamingState, events ...string) {
	t.Helper()
	for _, event := range events {
		if err := state.observe([]byte(event)); err != nil {
			t.Fatalf("%s: %v", event, err)
		}
	}
}

func TestStreamFixtures(t *testing.T) {
	for _, test := range []struct {
		name                     string
		input, output, reasoning int64
		items                    int
		stop                     llm.StopReason
	}{
		{"text", 18, 4, 0, 1, llm.StopComplete},
		{"tools", 397, 92, 0, 2, llm.StopComplete},
		{"tool-results", 553, 35, 0, 1, llm.StopComplete},
		{"thinking", 74, 547, 210, 2, llm.StopComplete},
		{"truncated-tool", 401, 32, 0, 0, llm.StopMaxOutputTokens},
	} {
		t.Run(test.name, func(t *testing.T) {
			var state streamingState
			if err := observeStream(&state, liveStream(t, test.name)); err != nil {
				t.Fatal(err)
			}
			body, err := state.unwrap()
			if err != nil {
				t.Fatal(err)
			}
			response, err := decodeResponse(body)
			if err != nil {
				t.Fatal(err)
			}
			usage := response.Usage
			if response.Stop != test.stop || len(response.Output) != test.items || usage.InputTokens != test.input || usage.OutputTokens != test.output || usage.ReasoningTokens != test.reasoning {
				t.Fatalf("response = %#v", response)
			}
			if !bytes.Contains(usage.Raw, []byte(`"service_tier":"standard"`)) || !bytes.Contains(usage.Raw, []byte(`"cache_creation":`)) {
				t.Fatalf("lost usage metadata: %s", usage.Raw)
			}
			switch test.name {
			case "text":
				if want := message(llm.RoleAssistant, "hello"); !reflect.DeepEqual(response.Output[0], want) {
					t.Fatalf("text = %#v, want %#v", response.Output[0], want)
				}
			case "tools":
				want := []llm.Item{
					{Type: llm.ItemToolCall, ProviderID: "toolu_01RiEXhooJeGyuotGN6zWi4p", Data: llm.ToolCall{
						CallID: "toolu_01RiEXhooJeGyuotGN6zWi4p", Name: "capture", Arguments: `{"value":"alpha"}`,
					}},
					{Type: llm.ItemToolCall, ProviderID: "toolu_01Y4rMvNFq6LJFGnoBUxBruV", Data: llm.ToolCall{
						CallID: "toolu_01Y4rMvNFq6LJFGnoBUxBruV", Name: "capture", Arguments: `{"value":"beta"}`,
					}},
				}
				if !reflect.DeepEqual(response.Output, want) {
					t.Fatalf("calls = %#v, want %#v", response.Output, want)
				}
			case "tool-results":
				want := message(llm.RoleAssistant, `I ran both captures at the same time, and both worked: one captured "alpha" and the other captured "beta".`)
				if !reflect.DeepEqual(response.Output[0], want) {
					t.Fatalf("text = %#v, want %#v", response.Output[0], want)
				}
			case "thinking":
				reasoning, ok := response.Output[0].Data.(llm.ProviderItem)
				if !ok || response.Output[0].Type != llm.ItemProvider || reasoning.Display != nil {
					t.Fatalf("thinking = %#v", response.Output[0])
				}
				answer, ok := response.Output[1].Data.(llm.Message)
				if !ok || !strings.Contains(answer.Text, "7417") {
					t.Fatalf("answer = %#v", response.Output[1])
				}
				var signed struct {
					Signature string `json:"signature"`
				}
				if err := apijson.Unmarshal(reasoning.Raw, &signed); err != nil {
					t.Fatal(err)
				}
				const signatureSHA256 = "e16bd2907c90a268ba2ea1a8f3b9601e6eceae98bdfc19f0a105174099b5035b"
				if got := fmt.Sprintf("%x", sha256.Sum256([]byte(signed.Signature))); got != signatureSHA256 {
					t.Fatalf("captured signature changed: SHA-256 = %s", got)
				}
				request := validRequest()
				request.Model.ReasoningEffort = llm.ReasoningEffortHigh
				request.Input = append(request.Input, response.Output...)
				request.Input = append(request.Input, message(llm.RoleUser, "Continue"))
				body, err := requestBody(request)
				if err != nil {
					t.Fatal(err)
				}
				var replay struct {
					Messages []struct {
						Role    string `json:"role"`
						Content []struct {
							Type      string `json:"type"`
							Signature string `json:"signature"`
						} `json:"content"`
					} `json:"messages"`
				}
				if err := apijson.Unmarshal(body, &replay); err != nil {
					t.Fatal(err)
				}
				if len(replay.Messages) != 3 || replay.Messages[1].Role != "assistant" || len(replay.Messages[1].Content) != 2 {
					t.Fatalf("unexpected replay messages: %#v", replay.Messages)
				}
				thinking := replay.Messages[1].Content[0]
				if thinking.Type != "thinking" || thinking.Signature != signed.Signature {
					t.Fatal("replay lost or changed the captured thinking signature")
				}
			}
		})
	}
}

const streamMessageStart = `{"type":"message_start","message":{"id":"msg-1","type":"message","role":"assistant","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`

func TestStreamInterleavesSparseBlocksInIndexOrder(t *testing.T) {
	var state streamingState
	observeEvents(t, &state,
		streamMessageStart,
		`{"type":"content_block_start","index":9,"content_block":{"type":"tool_use","id":"b","name":"capture","input":{}}}`,
		`{"type":"content_block_start","index":3,"content_block":{"type":"text","text":"Starting "}}`,
		`{"type":"content_block_start","index":6,"content_block":{"type":"tool_use","id":"a","name":"capture","input":{}}}`,
		`{"type":"content_block_delta","index":9,"delta":{"type":"input_json_delta","partial_json":"{\"n\":"}}`,
		`{"type":"content_block_delta","index":3,"delta":{"type":"text_delta","text":"both"}}`,
		`{"type":"ping"}`,
		`{"type":"future_event","data":{"ignored":true}}`,
		`{"type":"content_block_delta","index":6,"delta":{"type":"input_json_delta","partial_json":"{\"value\":\"a\"}"}}`,
		`{"type":"content_block_delta","index":9,"delta":{"type":"input_json_delta","partial_json":"9007199254740993}"}}`,
		`{"type":"content_block_stop","index":9}`,
		`{"type":"content_block_stop","index":9}`,
		`{"type":"content_block_stop","index":3}`,
		`{"type":"content_block_stop","index":6}`,
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
	want := []llm.Item{
		{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Starting both"}},
		{Type: llm.ItemToolCall, ProviderID: "a", Data: llm.ToolCall{CallID: "a", Name: "capture", Arguments: `{"value":"a"}`}},
		{Type: llm.ItemToolCall, ProviderID: "b", Data: llm.ToolCall{CallID: "b", Name: "capture", Arguments: `{"n":9007199254740993}`}},
	}
	if !reflect.DeepEqual(response.Output, want) {
		t.Fatalf("output = %#v, want %#v", response.Output, want)
	}
}

func TestStreamOmitsUnfinalizedBlocksRegardlessOfStopReason(t *testing.T) {
	for _, stop := range []string{"end_turn", "tool_use", "max_tokens", "model_context_window_exceeded", "refusal"} {
		t.Run(stop, func(t *testing.T) {
			var state streamingState
			observeEvents(t, &state,
				streamMessageStart,
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"unfinalized"}}`,
				`{"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"unfinalized","signature":"signed"}}`,
				`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"open","name":"capture","input":{}}}`,
				`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"incomplete\":"}}`,
				`{"type":"content_block_start","index":3,"content_block":{"type":"tool_use","id":"ready","name":"capture","input":{}}}`,
				`{"type":"content_block_stop","index":3}`,
				`{"type":"content_block_start","index":7,"content_block":{"type":"redacted_thinking","data":"unfinalized"}}`,
				`{"type":"content_block_start","index":8,"content_block":{"type":"tool_use","id":"valid-but-open","name":"capture","input":{}}}`,
				fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q}}`, stop),
				`{"type":"message_stop"}`,
			)
			body, err := state.unwrap()
			if err != nil {
				t.Fatal(err)
			}
			response, err := decodeResponse(body)
			if err != nil || len(response.Output) != 1 || response.Output[0].Data.(llm.ToolCall).CallID != "ready" {
				t.Fatalf("response = %#v, error = %v", response, err)
			}
		})
	}
}

func TestStreamMergesMessageUpdatesAndCumulativeUsage(t *testing.T) {
	var state streamingState
	observeEvents(t, &state,
		`{"type":"message_start","message":{"id":"original","type":"message","role":"assistant","content":[],"metadata":{"initial":true},"usage":{"input_tokens":9,"cache_read_input_tokens":80,"output_tokens":1,"details":{"a":9007199254740993,"b":2}}}}`,
		`{"type":"message_delta","delta":{"id":"updated","type":"message","role":"assistant","model":"new","metadata":null,"content":["replaced at flush"]},"usage":{"input_tokens":0,"cache_read_input_tokens":null,"output_tokens":7,"details":{"b":0,"c":1e1000}}}`,
		`{"type":"message_delta","delta":{"stop_reason":"refusal","stop_details":{"type":"refusal","category":"policy_violation","explanation":"Provider explanation"}},"usage":{"output_tokens":8,"details":{"a":null},"output_tokens_details":{"thinking_tokens":2}}}`,
		`{"type":"message_delta","delta":{"id":null,"stop_reason":null,"stop_details":null,"absent":null}}`,
		`{"type":"message_stop"}`,
	)
	body, err := state.unwrap()
	if err != nil {
		t.Fatal(err)
	}
	var got, want map[string]any
	if err := apijson.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if err := apijson.Unmarshal([]byte(`{"id":"updated","type":"message","role":"assistant","model":"new","metadata":{"initial":true},"content":[],"stop_reason":"refusal","stop_details":{"type":"refusal","category":"policy_violation","explanation":"Provider explanation"},"usage":{"input_tokens":0,"cache_read_input_tokens":80,"output_tokens":8,"details":{"a":9007199254740993,"b":0,"c":1e1000},"output_tokens_details":{"thinking_tokens":2}}}`), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("assembled message = %s", body)
	}
	response, err := decodeResponse(body)
	if err != nil || response.Usage.InputTokens != 80 || response.Usage.OutputTokens != 8 || response.Usage.ReasoningTokens != 2 || response.Failure == nil || response.Failure.Message != "Provider explanation" {
		t.Fatalf("response = %#v, error = %v", response, err)
	}
}

func TestStreamPreservesThinkingForReplay(t *testing.T) {
	var state streamingState
	observeEvents(t, &state,
		streamMessageStart,
		`{"type":"content_block_start","index":4,"content_block":{"type":"thinking","thinking":"first ","signature":"start-","provider_state":{"n":9007199254740993}}}`,
		`{"type":"content_block_delta","index":4,"delta":{"type":"thinking_delta","thinking":"second"}}`,
		`{"type":"content_block_delta","index":4,"delta":{"type":"signature_delta","signature":"end"}}`,
		`{"type":"content_block_stop","index":4}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
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
	reasoning := response.Output[0].Data.(llm.ProviderItem)
	if (reasoning.Display == nil || reasoning.Display.Text != "first second") || !bytes.Contains(reasoning.Raw, []byte(`"signature":"start-end"`)) || !bytes.Contains(reasoning.Raw, []byte(`9007199254740993`)) {
		t.Fatalf("reasoning = %#v", reasoning)
	}
	request := validRequest()
	request.Input = append(request.Input, response.Output...)
	replay, err := requestBody(request)
	if err != nil || !bytes.Contains(replay, []byte(`"signature":"start-end"`)) || !bytes.Contains(replay, []byte(`9007199254740993`)) {
		t.Fatalf("replay = %s, error = %v", replay, err)
	}
}

func TestStreamRejectsInvalidTransitions(t *testing.T) {
	const start = `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
	const delta = `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"a"}}`
	const stop = `{"type":"content_block_stop","index":0}`
	for name, events := range map[string][]string{
		"block before message": {start},
		"delta before message": {`{"type":"message_delta","delta":{}}`},
		"stop before message":  {`{"type":"message_stop"}`},
		"duplicate message":    {streamMessageStart, streamMessageStart},
		"duplicate block":      {streamMessageStart, start, start},
		"nonexistent delta":    {streamMessageStart, delta},
		"nonexistent stop":     {streamMessageStart, stop},
		"finalized delta":      {streamMessageStart, start, stop, delta},
		"recreated block":      {streamMessageStart, start, stop, start},
		"missing index":        {streamMessageStart, `{"type":"content_block_start","content_block":{"type":"text"}}`},
		"negative index":       {streamMessageStart, `{"type":"content_block_start","index":-1,"content_block":{"type":"text"}}`},
		"wrong delta":          {streamMessageStart, start, `{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"a"}}`},
		"invalid payload":      {streamMessageStart, start, `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":42}}`},
		"invalid JSON":         {`{`},
		"missing type":         {`{}`},
	} {
		t.Run(name, func(t *testing.T) {
			var state streamingState
			var err error
			for _, event := range events {
				if err = state.observe([]byte(event)); err != nil {
					break
				}
			}
			if err == nil {
				t.Fatal("accepted invalid transition")
			}
		})
	}
}

func TestStreamRequiresMessageStop(t *testing.T) {
	var state streamingState
	observeEvents(t, &state, streamMessageStart)
	if body, err := state.unwrap(); body != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("body = %s, error = %v", body, err)
	}
}

func TestStreamRejectsFinalizedInvalidToolInput(t *testing.T) {
	var state streamingState
	observeEvents(t, &state,
		streamMessageStart,
		`{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"a","name":"capture","input":{}}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"unfinished\":"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}`,
	)
	if err := state.observe([]byte(`{"type":"message_stop"}`)); err == nil {
		t.Fatal("accepted invalid finalized tool input")
	}
}
