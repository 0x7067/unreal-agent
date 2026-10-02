package messagesapi

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func validRequest() llm.Request {
	return llm.Request{Model: llm.Model{ID: "claude-test"}, Input: []llm.Item{message(llm.RoleUser, "Hello")}}
}

func newTestAdapter(t *testing.T, config Config) llm.Adapter {
	t.Helper()
	remote := primitives.NewRemoteClient()
	t.Cleanup(func() {
		if err := remote.Close(); err != nil {
			t.Error(err)
		}
	})
	adapter, err := NewAdapter(remote, config)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestAdapterValidatesConfiguration(t *testing.T) {
	if _, err := NewAdapter(nil, Config{Endpoint: "http://example.invalid/messages"}); err == nil {
		t.Fatal("accepted nil remote client")
	}
	remote := primitives.NewRemoteClient()
	t.Cleanup(func() {
		if err := remote.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, config := range []Config{
		{},
		{Endpoint: "http://example.invalid/messages", MaxAttempts: new(0)},
		{Endpoint: "http://example.invalid/messages", MaxAttempts: new(-1)},
		{Endpoint: "http://example.invalid/messages", CacheTTL: "10m"},
		{Endpoint: "http://example.invalid/messages", CacheTTL: "default"},
	} {
		if _, err := NewAdapter(remote, config); err == nil {
			t.Fatalf("accepted config=%#v", config)
		}
	}
}

func TestAdapterAppliesCacheTTL(t *testing.T) {
	stream := liveStream(t, "text")
	requests := make(chan wireRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wire wireRequest
		if err := json.UnmarshalRead(r.Body, &wire); err != nil {
			t.Error(err)
			return
		}
		requests <- wire
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := w.Write(stream); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	adapter := newTestAdapter(t, Config{Endpoint: server.URL, CacheTTL: CacheTTL1h})
	if _, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{}); err != nil {
		t.Fatal(err)
	}
	wire := <-requests
	if wire.CacheControl["type"] != "ephemeral" || wire.CacheControl["ttl"] != "1h" {
		t.Fatalf("cache control = %#v, want TTL 1h", wire.CacheControl)
	}
}

func TestAdapterRetriesHTTPFailuresWithTheSameRequest(t *testing.T) {
	stream := liveStream(t, "text")
	for _, status := range []int{408, 409, 429, 500, 529} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			bodies := make(chan []byte, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				bodies <- body
				if len(bodies) == 1 {
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Retry-After-Ms", "1")
					w.WriteHeader(status)
					if _, err := fmt.Fprint(w, `{"type":"error","error":{"type":"api_error","message":"retry"}}`); err != nil {
						t.Error(err)
					}
					return
				}
				w.Header().Set("Content-Type", "text/event-stream")
				if _, err := w.Write(stream); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			var traces []Exchange
			adapter := newTestAdapter(t, Config{Endpoint: server.URL, MaxAttempts: new(2), Trace: func(value Exchange) { traces = append(traces, value) }})
			response, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
			if err != nil || len(bodies) != 2 || len(traces) != 1 || response.Output[0].Data.(llm.Message).Text != "hello" {
				t.Fatalf("response=%#v error=%v requests=%d traces=%d", response, err, len(bodies), len(traces))
			}
			if !bytes.Equal(<-bodies, <-bodies) {
				t.Fatal("request changed on retry")
			}
		})
	}
}

func TestAdapterStopsAtAttemptLimit(t *testing.T) {
	for _, test := range []struct {
		status, attempts int
		disable          bool
	}{{401, 1, false}, {400, 1, false}, {503, 2, false}, {503, 1, true}} {
		var count atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After-Ms", "1")
			w.Header().Set("Request-Id", "req-test")
			if test.disable {
				w.Header().Set("X-Should-Retry", "false")
			}
			w.WriteHeader(test.status)
			if _, err := fmt.Fprint(w, `{"type":"error","error":{"type":"api_error","message":"failed"}}`); err != nil {
				t.Error(err)
			}
		}))
		adapter := newTestAdapter(t, Config{Endpoint: server.URL, MaxAttempts: new(2)})
		_, err := adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
		server.Close()
		var apiError *APIError
		if !errors.As(err, &apiError) || apiError.StatusCode != test.status || apiError.RequestID != "req-test" || count.Load() != int32(test.attempts) {
			t.Fatalf("error=%v requests=%d test=%#v", err, count.Load(), test)
		}
	}
}

func TestAdapterCancellationStopsHTTPWork(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.Copy(io.Discard, r.Body); err != nil {
			t.Error(err)
			return
		}
		close(started)
		<-r.Context().Done()
		close(stopped)
	}))
	defer server.Close()
	adapter := newTestAdapter(t, Config{Endpoint: server.URL})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := adapter.Respond(ctx, validRequest(), llm.RequestOptions{})
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	<-stopped
}

func TestAdapterDoesNotSendCanceledOrInvalidRequests(t *testing.T) {
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { count.Add(1) }))
	defer server.Close()
	adapter := newTestAdapter(t, Config{Endpoint: server.URL})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := adapter.Respond(ctx, validRequest(), llm.RequestOptions{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error=%v", err)
	}
	invalid := validRequest()
	invalid.Tools = []llm.Tool{{Type: llm.ToolHosted, Name: "web_search"}}
	if _, err := adapter.Respond(t.Context(), invalid, llm.RequestOptions{}); err == nil {
		t.Fatal("accepted a hosted tool")
	}
	if count.Load() != 0 {
		t.Fatal("unexpected provider requests")
	}
}

func TestProviderErrorFallback(t *testing.T) {
	for _, test := range []struct{ body, want string }{{"", "Bad Gateway"}, {"<html>upstream failed</html>", "upstream failed"}} {
		err := providerError(502, []byte(test.body), nil)
		if err.StatusCode != 502 || !strings.Contains(err.Message, test.want) {
			t.Fatalf("error=%#v", err)
		}
	}
}

func TestProviderErrorPreservesMetadataWithEmptyMessage(t *testing.T) {
	const body = `{"type":"error","request_id":"req-body","error":{"type":"overloaded_error","message":""}}`
	err := providerError(http.StatusOK, []byte(body), http.Header{"Request-Id": {"req-header"}})
	if err.Type != "overloaded_error" || err.RequestID != "req-body" || err.Message != body {
		t.Fatalf("error=%#v", err)
	}
}
