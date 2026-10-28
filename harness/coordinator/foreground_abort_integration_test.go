package coordinator

import (
	"context"
	"encoding/json/v2"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestForegroundAbortRealShellDrainsWithoutRetiringCoordinator(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	store, registry := independentToolCalls(t, 1)
	spec, err := operation.NewShellSpec(operation.ShellInput{Shell: "/bin/sh", Command: "exec sleep 30"}, t.TempDir(), 128)
	if err != nil {
		t.Fatal(err)
	}
	value := store.resume.Operations[0]
	value.Type, value.Version, value.State, value.Status, value.MaxOutputLength = spec.Type, spec.Version, spec.State, operation.StatusReady, 128
	store.resume.Operations[0] = value
	status := store.items[2].Data.(sessionstore.ToolCallStatus)
	status.Operations[0] = value
	store.items[2].Data = status
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager := operation.NewLocalOperationManager(ctx)
	done := make(chan error, 1)
	completed := make(chan struct{}, 1)
	answered := make(chan struct{}, 1)
	var allowed, calls atomic.Bool
	var started bool
	store.onSaveOperation = func(update operation.Operation) {
		state, err := operation.DecodeShellState(update)
		if err != nil {
			t.Error(err)
			return
		}
		if !started && state.ProcessGroupID > 1 {
			started = true
			submitTestInput(t, inputs, foregroundAbortInput(t, "abort-shell", "turn-1"))
		}
	}
	store.onAppendInput = func(in inbox.Input) {
		if in.Kind != inbox.InputControl {
			return
		}
		request, err := in.DecodeControlMessage()
		if err != nil {
			t.Error(err)
			return
		}
		if request.Mode == inbox.ForegroundAbortFinished && request.Parameters.(inbox.ForegroundAbortResult).Status == "completed" {
			completed <- struct{}{}
		}
	}
	current := New(Dependencies{SessionID: "session-1", Inbox: inputs, Restored: store.resume, Sessions: store, ContextBuilder: contextbuilder.NewBuilder(), Tools: registry, Operations: manager, LLM: &fakeAdapter{respond: func(context.Context, llm.Request) (llm.Response, error) {
		if !allowed.Load() {
			t.Error("aborted shell completion automatically restarted model")
		}
		calls.Store(true)
		answered <- struct{}{}
		return textResponse("new input answered"), nil
	}}})
	go func() { done <- current.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("coordinator join unconfirmed")
		}
		if err := manager.AwaitStopped(context.Background()); err != nil {
			t.Error(err)
		}
	})
	select {
	case <-completed:
	case err := <-done:
		t.Fatalf("coordinator retired during abort: %v", err)
	case <-ctx.Done():
		t.Fatal("abort completion timeout")
	}
	// The canonical completion is only emitted after the live manager receipt.
	if err := manager.LocalCleanupProof([]operation.ID{value.ID}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(15 * time.Millisecond)
	if calls.Load() {
		t.Fatal("old tool continuation called model without new input")
	}
	allowed.Store(true)
	payload, _ := json.Marshal("new external input")
	if err := inputs.Submit(ctx, inbox.Input{ID: "new-after-shell-abort", Kind: inbox.InputExternal, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-answered:
	case <-ctx.Done():
		t.Fatal("same coordinator did not accept new input after abort")
	}
	// Cleanup owns lifetime cancellation and joins the exact coordinator/manager;
	// PID absence/socket EOF is not the cleanup oracle.
}
