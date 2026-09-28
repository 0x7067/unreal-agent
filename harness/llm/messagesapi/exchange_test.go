package messagesapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestAdapterRequiresEventStream(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/plain", ""} {
		t.Run(contentType, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header()["Content-Type"] = []string{contentType}
				if _, err := w.Write(responseBody("end_turn", `[]`)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			adapter := newTestAdapter(t, Config{Endpoint: server.URL, MaxAttempts: new(2)})
			response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
			if err == nil || !strings.Contains(err.Error(), "expected text/event-stream") || len(response.Output) != 0 || attempts.Load() != 1 {
				t.Fatalf("response=%#v error=%v attempts=%d", response, err, attempts.Load())
			}
		})
	}
}

func TestAdapterAssemblesChunkedLiveStream(t *testing.T) {
	stream := liveStream(t, "thinking")
	stopped := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(stopped)
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		for chunk := range slices.Chunk(stream, 37) {
			if _, err := w.Write(chunk); err != nil {
				t.Error(err)
				return
			}
			w.(http.Flusher).Flush()
		}
		// Completion is message_stop, not the server closing its connection.
		<-r.Context().Done()
	}))
	defer server.Close()
	var trace Exchange
	adapter := newTestAdapter(t, Config{Endpoint: server.URL, Trace: func(value Exchange) { trace = value }})
	response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
	if err != nil {
		t.Fatal(err)
	}
	<-stopped
	if response.Usage.ReasoningTokens != 210 || len(response.Output) != 2 || trace.StatusCode != 200 || bytes.Contains(trace.ResponseBody, []byte("event:")) {
		t.Fatalf("response=%#v trace=%#v", response, trace)
	}
	decoded, err := decodeResponse(trace.ResponseBody)
	if err != nil || !reflect.DeepEqual(response, decoded) {
		t.Fatalf("trace response differs: %v", err)
	}
}

func TestAdapterStreamEmitsOnlyFinalizedBlocks(t *testing.T) {
	for _, finalized := range []bool{false, true} {
		t.Run(fmt.Sprintf("finalized=%v", finalized), func(t *testing.T) {
			stream := liveStream(t, "truncated-tool")
			if finalized {
				stream = bytes.Replace(stream, []byte("event: message_delta"), []byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\nevent: message_delta"), 1)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := w.Write(stream); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			var trace Exchange
			adapter := newTestAdapter(t, Config{Endpoint: server.URL, Trace: func(value Exchange) { trace = value }})
			response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
			want := 0
			if finalized {
				want = 1
			}
			if err != nil || response.Stop != llm.StopMaxOutputTokens || len(response.Output) != want {
				t.Fatalf("response = %#v, error = %v", response, err)
			}
			decoded, err := decodeResponse(trace.ResponseBody)
			if err != nil || !reflect.DeepEqual(decoded, response) {
				t.Fatalf("assembled response differs from trace: %v", err)
			}
		})
	}
}

func TestAdapterRetriesInterruptedStreamWithoutLeakingPartialCalls(t *testing.T) {
	prefix, _, found := bytes.Cut(liveStream(t, "tools"), []byte("event: message_delta"))
	if !found {
		t.Fatal("fixture missing message_delta")
	}
	for name, suffix := range map[string]string{
		"EOF":           "",
		"overloaded":    "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n",
		"empty message": "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"\"}}\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int32
			var traces []Exchange
			success := liveStream(t, "text")
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Retry-After-Ms", "1")
				body := success
				if attempts.Add(1) == 1 {
					body = append(bytes.Clone(prefix), suffix...)
				}
				if _, err := w.Write(body); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			adapter := newTestAdapter(t, Config{Endpoint: server.URL, MaxAttempts: new(2), Trace: func(value Exchange) { traces = append(traces, value) }})
			response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
			if err != nil || attempts.Load() != 2 || len(traces) != 1 || len(response.Output) != 1 || response.Output[0].Type != llm.ItemMessage {
				t.Fatalf("response=%#v error=%v attempts=%d traces=%d", response, err, attempts.Load(), len(traces))
			}
		})
	}
}

func TestAdapterStreamFailureDoesNotReturnPartialOutput(t *testing.T) {
	prefix, _, _ := bytes.Cut(liveStream(t, "tools"), []byte("event: message_delta"))
	for name, suffix := range map[string]string{
		"EOF":            "",
		"provider error": "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"invalid_request_error\",\"message\":\"failed\"}}\n\n",
		"malformed":      "event: message_delta\ndata: {\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.Header().Set("Content-Type", "text/event-stream")
				w.Header().Set("Request-Id", "req-stream")
				if _, err := w.Write(append(bytes.Clone(prefix), suffix...)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			adapter := newTestAdapter(t, Config{Endpoint: server.URL, MaxAttempts: new(1)})
			response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
			if err == nil || len(response.Output) != 0 || attempts.Load() != 1 {
				t.Fatalf("response=%#v error=%v attempts=%d", response, err, attempts.Load())
			}
			if name == "provider error" {
				var apiError *APIError
				if !errors.As(err, &apiError) || apiError.StatusCode != 200 || apiError.Type != "invalid_request_error" || apiError.RequestID != "req-stream" {
					t.Fatalf("error=%v", err)
				}
			}
		})
	}
}

func TestAdapterCancellationPreventsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		if _, err := fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"wait"}}`); err != nil {
			t.Error(err)
		}
		w.(http.Flusher).Flush()
		cancel()
	}))
	defer server.Close()
	adapter := newTestAdapter(t, Config{Endpoint: server.URL})
	if _, err := adapter.Respond(ctx, validRequest(), llm.RequestOptions{}); !errors.Is(err, context.Canceled) || attempts.Load() != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts.Load())
	}
}
