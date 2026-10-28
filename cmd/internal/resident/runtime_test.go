package resident

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func TestRuntimeControlIdleReplacesOnlyCurrentGeneration(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "idle upgrade must not invoke provider", 500)
	}))
	t.Cleanup(provider.Close)
	_, backend, state, _, _ := residentLifecycleFixture(t, provider, true)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	c := lifecycleDial(t, ctx, backend, state)
	reader := faultReader(t, c)
	before := *c.Snapshot().Header.Status
	checked, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "check"})
	if err != nil || checked.State != "ready" || checked.Generation != before.Generation || checked.RunningBuild != before.Build || checked.CandidateBuild == "" {
		t.Fatalf("candidate check: %+v %v", checked, err)
	}
	stale, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: before.Generation + 9})
	var rejected *attachclient.RuntimeError
	if !errors.As(err, &rejected) || stale.State != "failed" || stale.Generation != before.Generation {
		t.Fatalf("stale replacement not refused: %+v %v", stale, err)
	}
	replaced, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: before.Generation})
	if err != nil || replaced.State != "completed" || replaced.Generation != before.Generation+1 || replaced.RunningBuild != before.Build {
		t.Fatalf("idle replacement: %+v %v", replaced, err)
	}
	if calls.Load() != 0 {
		t.Fatal("upgrade called provider")
	}
	again, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: before.Generation})
	if err == nil || again.Generation != replaced.Generation || again.State == "completed" {
		t.Fatalf("old-generation retry replaced a new worker: %+v %v", again, err)
	}
	if c.ControlScope() == "" || c.ControlScope() == c.Snapshot().Hello.Incarnation+"/1" {
		t.Fatal("client did not learn trusted new lifecycle generation")
	}
	_ = c.Close()
	select {
	case <-reader:
	case <-ctx.Done():
		t.Fatal("client reader did not join")
	}
}
func TestRuntimeReplacementBlockedByActiveModelDoesNotCancelIt(t *testing.T) {
	entered, canceled := make(chan struct{}), make(chan struct{})
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
		close(canceled)
	}))
	t.Cleanup(provider.Close)
	_, backend, state, _, _ := residentLifecycleFixture(t, provider, true)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	c := lifecycleDial(t, ctx, backend, state)
	faultReader(t, c)
	gen := c.Snapshot().Header.Status.Generation
	if err := c.Submit(ctx, inbox.Input{ID: "runtime-model-private", Kind: inbox.InputExternal, Payload: []byte(`"private blocked model"`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("model never started")
	}
	result, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: gen})
	var blocked *attachclient.RuntimeError
	if !errors.As(err, &blocked) || result.State != "blocked" || result.Generation != gen {
		t.Fatalf("active work not blocked: %+v %v", result, err)
	}
	select {
	case <-canceled:
		t.Fatal("runtime upgrade canceled unrelated model")
	default:
	}
	checked, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "check"})
	if err != nil || checked.Generation != gen {
		t.Fatalf("active worker changed: %+v %v", checked, err)
	}
}
func TestRuntimeCapabilityRemainsOptIn(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("provider called") }))
	t.Cleanup(provider.Close)
	_, backend, state, _, _ := residentLifecycleFixture(t, provider)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	c := lifecycleDial(t, ctx, backend, state)
	faultReader(t, c)
	if _, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "check"}); err == nil {
		t.Fatal("disabled capability accepted")
	}
}
