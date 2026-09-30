package messagesapi

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestUsageByModel(t *testing.T) {
	const counters = `"input_tokens":7,"cache_read_input_tokens":30,"cache_creation_input_tokens":20,"output_tokens":11,"output_tokens_details":{"thinking_tokens":5}`
	for name, test := range map[string]struct {
		model, stop, iterations string
		want                    map[string]llm.TokenUsage
	}{
		"single model": {
			model: "served", stop: "end_turn",
			want: map[string]llm.TokenUsage{"served": {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11, ReasoningTokens: 5}},
		},
		"empty model": {
			stop: "end_turn",
			want: map[string]llm.TokenUsage{"": {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11, ReasoningTokens: 5}},
		},
		"null iterations": {
			model: "served", stop: "end_turn", iterations: `null`,
			want: map[string]llm.TokenUsage{"served": {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11, ReasoningTokens: 5}},
		},
		"empty iterations": {
			model: "served", stop: "end_turn", iterations: `[]`,
			want: map[string]llm.TokenUsage{"served": {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11, ReasoningTokens: 5}},
		},
		"multiple hops and repeated model": {
			model: "primary", stop: "end_turn",
			iterations: `[
				{"type":"message","model":"primary","input_tokens":10,"cache_read_input_tokens":3,"cache_creation_input_tokens":2,"output_tokens":2},
				{"type":"message","model":"primary","input_tokens":5,"cache_read_input_tokens":7,"output_tokens":1},
				{"type":"message","model":"second","input_tokens":2,"output_tokens":0},
				{"type":"fallback_message","model":"final","input_tokens":7,"cache_read_input_tokens":30,"cache_creation_input_tokens":20,"output_tokens":11}
			]`,
			want: map[string]llm.TokenUsage{
				"primary": {InputTokens: 27, CachedInputTokens: 10, CacheWriteInputTokens: 2, OutputTokens: 3},
				"second":  {InputTokens: 2},
				"final":   {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11},
			},
		},
		"sticky routing": {
			model: "served", stop: "end_turn",
			iterations: `[{"type":"fallback_message","model":"served","input_tokens":7,"cache_read_input_tokens":30,"cache_creation_input_tokens":20,"output_tokens":11}]`,
			want:       map[string]llm.TokenUsage{"served": {InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11}},
		},
		"all models declined": {
			model: "final", stop: "refusal",
			iterations: `[{"type":"message","model":"primary","input_tokens":9,"output_tokens":2},{"type":"fallback_message","model":"final","input_tokens":7,"output_tokens":11}]`,
			want:       map[string]llm.TokenUsage{"primary": {InputTokens: 9, OutputTokens: 2}, "final": {InputTokens: 7, OutputTokens: 11}},
		},
		"unattributed iteration": {
			model: "served", stop: "end_turn",
			iterations: `[{"type":"message","model":null,"input_tokens":9,"output_tokens":2},{"type":"fallback_message","model":"served","input_tokens":7,"output_tokens":11}]`,
			want:       map[string]llm.TokenUsage{"": {InputTokens: 9, OutputTokens: 2}, "served": {InputTokens: 7, OutputTokens: 11}},
		},
	} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/streaming=%v", name, streaming), func(t *testing.T) {
				usage := "{" + counters
				if test.iterations != "" {
					usage += `,"iterations":` + test.iterations
				}
				usage += "}"
				body := []byte(fmt.Sprintf(`{"type":"message","id":"msg","role":"assistant","model":%q,"stop_reason":%q,"content":[],"usage":%s}`, test.model, test.stop, usage))
				if streaming {
					var state streamingState
					observeEvents(t, &state,
						fmt.Sprintf(`{"type":"message_start","message":{"type":"message","id":"msg","role":"assistant","model":%q,"content":[],"usage":{"input_tokens":1,"output_tokens":0}}}`, test.model),
						fmt.Sprintf(`{"type":"message_delta","delta":{"stop_reason":%q},"usage":%s}`, test.stop, usage),
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
				if !reflect.DeepEqual(response.Usage.ByModel, test.want) {
					t.Fatalf("usage by model = %#v, want %#v", response.Usage.ByModel, test.want)
				}
				if got := response.Usage.TokenUsage; got != (llm.TokenUsage{InputTokens: 57, CachedInputTokens: 30, CacheWriteInputTokens: 20, OutputTokens: 11, ReasoningTokens: 5}) {
					t.Fatalf("top-level usage changed: %#v", got)
				}
				var wire struct {
					Usage jsontext.Value `json:"usage"`
				}
				if err := json.Unmarshal(body, &wire); err != nil {
					t.Fatal(err)
				}
				if string(response.Usage.Raw) != string(wire.Usage) {
					t.Fatal("raw usage changed")
				}
			})
		}
	}
}

func TestUsageRejectsMalformedIterations(t *testing.T) {
	for _, iterations := range []string{`{}`, `[{"model":42}]`, `[{"model":"served","input_tokens":"bad"}]`} {
		body := []byte(`{"type":"message","role":"assistant","stop_reason":"end_turn","usage":{"iterations":` + iterations + `}}`)
		if _, err := decodeResponse(body); err == nil {
			t.Fatalf("accepted malformed iterations: %s", iterations)
		}
	}
}
