package messagesapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

type retryHintTransport struct {
	headers http.Header
	calls   []time.Time
}

func (transport *retryHintTransport) RoundTrip(*http.Request) (*http.Response, error) {
	transport.calls = append(transport.calls, time.Now())
	headers := transport.headers.Clone()
	headers.Set("Content-Type", "application/json")
	return &http.Response{StatusCode: 429, Header: headers, Body: io.NopCloser(strings.NewReader(`{"type":"error","error":{"type":"rate_limit_error","message":"wait"}}`))}, nil
}

func TestExchangeRetryHintsOverrideBackoff(t *testing.T) {
	for _, test := range []struct {
		name           string
		headers        http.Header
		floor, ceiling time.Duration
	}{
		{"capped seconds", http.Header{"Retry-After": {"120"}}, 30 * time.Second, 30 * time.Second},
		{"ms precedence", http.Header{"Retry-After-Ms": {"3000"}, "Retry-After": {"120"}}, 3 * time.Second, 3 * time.Second},
		{"short hint replaces local", http.Header{"Retry-After-Ms": {"1"}}, time.Millisecond, time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				transport := &retryHintTransport{headers: test.headers}
				remote := primitives.NewRemoteClientWithHTTPClient(&http.Client{Transport: transport})
				defer remote.Close()
				adapter, err := NewAdapter(remote, Config{Endpoint: "http://example.invalid/messages", MaxAttempts: new(2)})
				if err != nil {
					t.Fatal(err)
				}
				_, err = adapter.Respond(t.Context(), validRequest(), llm.RequestOptions{})
				if err == nil || len(transport.calls) != 2 {
					t.Fatalf("calls=%d err=%v", len(transport.calls), err)
				}
				delay := transport.calls[1].Sub(transport.calls[0])
				if delay < test.floor || delay > test.ceiling {
					t.Fatalf("delay=%s want [%s,%s]", delay, test.floor, test.ceiling)
				}
				if !time.Now().Equal(transport.calls[1]) {
					t.Fatal("waited after final attempt")
				}
			})
		})
	}
}

func TestExchangeHugeHintCancellation(t *testing.T) {
	for _, headers := range []http.Header{
		{"Retry-After": {"120"}}, {"Retry-After": {"1e300"}}, {"Retry-After-Ms": {"1e300"}}, {"Retry-After": {"Fri, 31 Dec 9999 23:59:59 GMT"}},
	} {
		synctest.Test(t, func(t *testing.T) {
			transport := &retryHintTransport{headers: headers}
			remote := primitives.NewRemoteClientWithHTTPClient(&http.Client{Transport: transport})
			defer remote.Close()
			adapter, err := NewAdapter(remote, Config{Endpoint: "http://example.invalid/messages", MaxAttempts: new(3)})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := adapter.Respond(ctx, validRequest(), llm.RequestOptions{}); done <- err }()
			synctest.Wait()
			if len(transport.calls) != 1 {
				t.Fatalf("calls=%d", len(transport.calls))
			}
			cancel()
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel error=%v", err)
			}
			ctx, stop := context.WithTimeout(t.Context(), time.Second)
			defer stop()
			if _, err := adapter.Respond(ctx, validRequest(), llm.RequestOptions{}); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline error=%v", err)
			}
			if len(transport.calls) != 2 {
				t.Fatal("retried after cancellation/deadline")
			}
		})
	}
}
