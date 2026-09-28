package anthropic

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/llm/messagesapi"
)

func TestClientConfiguration(t *testing.T) {
	for _, config := range []Config{{APIKey: " "}, {APIKey: "test-key", MaxAttempts: new(0)}} {
		client, err := NewClient(config)
		if client != nil || err == nil {
			t.Fatalf("client=%v error=%v", client, err)
		}
	}
	client, err := NewClient(Config{APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClientCallsMessagesAPI(t *testing.T) {
	const responseBody = `{"type":"message","id":"msg-1","role":"assistant","content":[{"type":"text","text":"Hello"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":2}}`
	const stream = `event: message_start
data: {"type":"message_start","message":{"type":"message","id":"msg-1","role":"assistant","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/messages" || r.Header.Get("X-Api-Key") != "test-key" ||
			r.Header.Get("Anthropic-Version") != "2023-06-01" || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Accept") != "text/event-stream" {
			t.Error("incorrect Messages API request or headers")
		}
		var body struct {
			Model  string `json:"model"`
			Stream bool   `json:"stream"`
		}
		if err := json.UnmarshalRead(r.Body, &body); err != nil {
			t.Error(err)
		}
		if body.Model != "claude-test" || !body.Stream {
			t.Errorf("body=%#v", body)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprint(w, stream); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	var exchange Exchange
	client, err := NewClient(Config{APIKey: "test-key", BaseURL: server.URL + "/v1/", Trace: func(value Exchange) { exchange = value }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	response, err := client.Respond(t.Context(), llm.Request{
		Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Hello"}}},
	}, llm.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if response.ID != "msg-1" || response.Output[0].Data.(llm.Message).Text != "Hello" || exchange.StatusCode != 200 {
		t.Fatalf("response=%#v exchange=%#v", response, exchange)
	}
	var got, want any
	if err := json.Unmarshal(exchange.ResponseBody, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(responseBody), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("assembled response=%s, want %s", exchange.ResponseBody, responseBody)
	}
	if strings.Contains(string(exchange.RequestBody), "test-key") {
		t.Fatal("trace includes authentication")
	}
}

func TestClientSurfacesAuthenticationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"API key is invalid."},"request_id":null}`); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	_, err = client.Respond(t.Context(), llm.Request{
		Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "Hello"}}},
	}, llm.RequestOptions{})
	var apiError *messagesapi.APIError
	if !errors.As(err, &apiError) || apiError.Type != "authentication_error" || apiError.StatusCode != 401 || apiError.RequestID != "" {
		t.Fatalf("error=%v", err)
	}
}
