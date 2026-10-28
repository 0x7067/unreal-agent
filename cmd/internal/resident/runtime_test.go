package resident

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
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
func TestRuntimeReplacementWaitsForActiveModelWithoutCancelingIt(t *testing.T) {
	entered, release, canceled := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(entered)
		select {
		case <-release:
		case <-r.Context().Done():
			close(canceled)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"scheduled\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"m\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"finished naturally\"}]}]}}\n\n")
	}))
	t.Cleanup(provider.Close)
	_, backend, state, _, _ := residentLifecycleFixture(t, provider, true)
	defer finish()
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
	type result struct {
		s   attachproto.RuntimeStatus
		err error
	}
	done := make(chan result, 1)
	go func() {
		s, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: gen})
		done <- result{s, err}
	}()
	waitRuntimeStopArmed(t, ctx, state)
	select {
	case r := <-done:
		t.Fatalf("reload settled while model active: %+v", r)
	default:
	}
	select {
	case <-canceled:
		t.Fatal("reload canceled active model")
	default:
	}
	finish()
	select {
	case r := <-done:
		if r.err != nil || r.s.State != "completed" || r.s.Generation != gen+1 {
			t.Fatalf("scheduled replacement: %+v", r)
		}
	case <-ctx.Done():
		t.Fatal("reload did not complete after natural model finish")
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
