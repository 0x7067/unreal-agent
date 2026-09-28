package anthropicapi_test

import (
	"bytes"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/internal/anthropicapi"
	"github.com/unreallabsai/unreal-agent/internal/apijson"
)

func TestCreateMessageRequest(t *testing.T) {
	cache := anthropicapi.CacheControlEphemeral{Type: anthropicapi.Ephemeral, Ttl: new(anthropicapi.N1h)}
	var blockCache anthropicapi.RequestTextBlock_CacheControl
	if err := blockCache.FromCacheControlEphemeral(cache); err != nil {
		t.Fatal(err)
	}
	var block anthropicapi.InputContentBlock
	if err := block.FromRequestTextBlock(anthropicapi.RequestTextBlock{
		Text: "hello", CacheControl: &blockCache,
	}); err != nil {
		t.Fatal(err)
	}
	var content anthropicapi.InputMessage_Content
	if err := content.FromInputMessageContent1([]anthropicapi.InputContentBlock{block}); err != nil {
		t.Fatal(err)
	}
	var tool anthropicapi.CreateMessageParams_Tools_Item
	var toolCache anthropicapi.Tool_CacheControl
	if err := toolCache.FromCacheControlEphemeral(cache); err != nil {
		t.Fatal(err)
	}
	if err := tool.FromTool(anthropicapi.Tool{
		Name: "lookup",
		InputSchema: anthropicapi.InputSchema{
			Type: "object",
			Properties: &map[string]any{
				"id": jsontext.Value(`{"type":"integer","minimum":9007199254740993}`),
			},
			Required: new([]string{}),
			AdditionalProperties: map[string]any{
				"additionalProperties": jsontext.Value(`false`),
			},
		},
		CacheControl: &toolCache,
		Strict:       new(false),
	}); err != nil {
		t.Fatal(err)
	}
	var requestCache anthropicapi.CreateMessageParams_CacheControl
	if err := requestCache.FromCacheControlEphemeral(cache); err != nil {
		t.Fatal(err)
	}
	request := anthropicapi.CreateMessageParams{
		Model: "custom-model", MaxTokens: 0,
		Messages:     []anthropicapi.InputMessage{{Role: "user", Content: content}},
		CacheControl: &requestCache,
		Stream:       new(false),
		Tools:        &[]anthropicapi.CreateMessageParams_Tools_Item{tool},
	}
	assertWireJSON(t, request, `{
		"model":"custom-model","max_tokens":0,"stream":false,
		"cache_control":{"type":"ephemeral","ttl":"1h"},
		"messages":[{"role":"user","content":[{"type":"text","text":"hello","cache_control":{"type":"ephemeral","ttl":"1h"}}]}],
		"tools":[{"name":"lookup","strict":false,"cache_control":{"type":"ephemeral","ttl":"1h"},
			"input_schema":{"type":"object","properties":{"id":{"type":"integer","minimum":9007199254740993}},"required":[],"additionalProperties":false}}]
	}`)
}

const messageJSON = `{
	"id":"msg_1","type":"message","role":"assistant","model":"custom-model",
	"content":[{"type":"text","text":"hello","citations":null}],
	"container":null,"stop_reason":"end_turn","stop_sequence":null,"stop_details":null,
	"usage":{"input_tokens":1,"output_tokens":2,"cache_creation":null,"cache_creation_input_tokens":null,
		"cache_read_input_tokens":null,"inference_geo":null,"output_tokens_details":null,"server_tool_use":null,"service_tier":null}
}`

func TestMessageResponse(t *testing.T) {
	var message anthropicapi.Message
	if err := json.Unmarshal([]byte(messageJSON), &message); err != nil {
		t.Fatal(err)
	}
	text, err := message.Content[0].AsResponseTextBlock()
	if err != nil {
		t.Fatal(err)
	}
	if text.Text != "hello" || text.Citations != nil || message.StopSequence != nil || message.Container != nil {
		t.Fatalf("unexpected response: %#v, %#v", message, text)
	}
	if err := message.Content[0].FromResponseTextBlock(text); err != nil {
		t.Fatal(err)
	}
	assertWireJSON(t, message, messageJSON)
}

func TestContentBlockToolInputPreservesJSON(t *testing.T) {
	input := `{"type":"tool_use","id":"tool_1","name":"lookup","caller":{"type":"direct"},
		"input":{"id":9007199254740993,"nested":[null,true,{"decimal":0.1234567890123456789,"large":1e1000}],"empty":{}}}`
	var block anthropicapi.ContentBlock
	if err := json.Unmarshal([]byte(input), &block); err != nil {
		t.Fatal(err)
	}
	tool, err := block.AsResponseToolUseBlock()
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := tool.Input["id"].(jsonv1.Number); !ok || value.String() != "9007199254740993" {
		t.Fatalf("tool input corrupted: %s", tool.Input["id"])
	}
	selected, err := block.ValueByDiscriminator()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selected, tool) {
		t.Fatalf("discriminator selected %#v, want %#v", selected, tool)
	}
	if err := block.FromResponseToolUseBlock(tool); err != nil {
		t.Fatal(err)
	}
	assertWireJSON(t, block, input)
}

func TestInputSchemaPreservesNumbers(t *testing.T) {
	const input = `{"type":"object","required":[],"properties":{
		"id":{"const":9007199254740993},"ratio":{"minimum":0.1234567890123456789}},
		"examples":[{"large":1e1000}],"additionalProperties":false}`
	var schema anthropicapi.InputSchema
	if err := json.Unmarshal([]byte(input), &schema); err != nil {
		t.Fatal(err)
	}
	if schema.Properties == nil {
		t.Fatal("properties are missing")
	}
	property, ok := (*schema.Properties)["id"].(map[string]any)
	if !ok || property["const"] != jsonv1.Number("9007199254740993") {
		t.Fatalf("property changed: %#v", (*schema.Properties)["id"])
	}
	assertWireJSON(t, schema, input)
}

func TestNullablePointerEncoding(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{`{}`, `{}`},
		{`{"user_id":null}`, `{}`},
		{`{"user_id":""}`, `{}`},
	} {
		var metadata anthropicapi.Metadata
		if err := apijson.Unmarshal([]byte(test.input), &metadata); err != nil {
			t.Fatal(err)
		}
		assertWireJSON(t, metadata, test.want)
	}
	for _, test := range []struct{ input, want string }{
		{`{"type":"object"}`, `{"type":"object"}`},
		{`{"type":"object","properties":null,"required":null}`, `{"type":"object"}`},
		{`{"type":"object","properties":{},"required":[]}`, `{"type":"object","properties":{},"required":[]}`},
	} {
		var schema anthropicapi.InputSchema
		if err := apijson.Unmarshal([]byte(test.input), &schema); err != nil {
			t.Fatal(err)
		}
		assertWireJSON(t, schema, test.want)
	}
}

func TestMessageStreamEvents(t *testing.T) {
	for _, input := range []string{
		`{"type":"message_start","message":` + messageJSON + `}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null,"stop_details":null,"container":null},"usage":{"output_tokens":2,"cache_creation_input_tokens":null,"cache_read_input_tokens":null,"input_tokens":null,"output_tokens_details":null,"server_tool_use":null}}`,
		`{"type":"message_stop"}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":"","citations":null}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"id\":"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"considering"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"signature"}}`,
		`{"type":"content_block_stop","index":0}`,
	} {
		t.Run(input, func(t *testing.T) {
			var event anthropicapi.MessageStreamEvent
			if err := json.Unmarshal([]byte(input), &event); err != nil {
				t.Fatal(err)
			}
			value, err := event.ValueByDiscriminator()
			if err != nil {
				t.Fatal(err)
			}
			if delta, ok := value.(anthropicapi.ContentBlockDeltaEvent); ok {
				if _, err := delta.Delta.ValueByDiscriminator(); err != nil {
					t.Fatal(err)
				}
			}
			assertWireJSON(t, value, input)
		})
	}
}

func TestErrorResponse(t *testing.T) {
	input := `{"type":"error","request_id":null,"error":{"type":"overloaded_error","message":"busy"}}`
	var response anthropicapi.ErrorResponse
	if err := json.Unmarshal([]byte(input), &response); err != nil {
		t.Fatal(err)
	}
	value, err := response.Error.ValueByDiscriminator()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := value.(anthropicapi.OverloadedError); !ok {
		t.Fatalf("unexpected error type %T", value)
	}
	assertWireJSON(t, response, input)
}

func assertWireJSON(t *testing.T, value any, want string) {
	t.Helper()
	got, err := apijson.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodeJSON(t, got), decodeJSON(t, []byte(want))) {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	decoder := jsonv1.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}
