package recoverygate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/coordinator"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type m21CountManager struct {
	*operation.LocalOperationManager
	adds atomic.Int64
}

func (m *m21CountManager) Add(op operation.Operation) error {
	m.adds.Add(1)
	return m.LocalOperationManager.Add(op)
}

type m21Model struct{ calls atomic.Int64 }

func (m *m21Model) Respond(context.Context, llm.Request, llm.RequestOptions) (llm.Response, error) {
	m.calls.Add(1)
	return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "recovered"}}}}, nil
}

// The process fixture counts canonical completion, not manager admission. This
// separate real-coordinator/real-manager harness directly counts Add calls.
func TestRecoveryDispatchCounterAndTerminalExclusion(t *testing.T) {
	ctx, stop := context.WithTimeout(t.Context(), 5*time.Second)
	defer stop()
	store, err := localfile.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sid := session.ID("m21-counted")
	if _, err := store.Create(ctx, sid); err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewValueSpec([]byte(`"Message sent to parent."`))
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "original-value-id", Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), Status: operation.StatusReady}
	if err := store.AppendTurn(ctx, sid, session.Turn{ID: "original-turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: "original-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "original-call", Name: tool.SendMessageName, Arguments: `{"to":"parent","message":"hello"}`}}}}}); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: "original-turn", CallID: "original-call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
		t.Fatal(err)
	}
	for generation := 0; generation < 2; generation++ {
		restored, err := store.Resume(ctx, sid)
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(restored.Operations); err != nil {
			t.Fatal(err)
		}
		owner, cancel := context.WithCancel(ctx)
		manager := &m21CountManager{LocalOperationManager: operation.NewLocalOperationManager(owner)}
		inputs, err := inbox.New(owner, restored.InputIDs)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		model := &m21Model{}
		registry := tool.NewRegistry(tool.StaticTranslators{SendMessage: toolsubagent.NewSendToParent()}, tool.SendMessageName)
		builder := contextbuilder.NewBuilder()
		builder.SetModel(llm.Model{ID: "fake"})
		c := coordinator.New(coordinator.Dependencies{SessionID: sid, Restored: restored, Sessions: store, Inbox: inputs, Operations: manager, Tools: registry, ContextBuilder: builder, LLM: model})
		done := make(chan error, 1)
		go func() { done <- c.Run(owner) }()
		// TryQuiesce is the causal barrier. A timer only schedules retries; timeout
		// never substitutes for retirement, Run join, or manager drain evidence.
		ticker := time.NewTicker(time.Millisecond)
		var retired error
		for {
			retired = c.TryQuiesce(ctx)
			if !errors.Is(retired, coordinator.ErrBusy) {
				break
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				retired = ctx.Err()
			}
			if ctx.Err() != nil {
				break
			}
		}
		ticker.Stop()
		if retired != nil {
			cancel()
			<-done
			t.Fatal(retired)
		}
		if err := <-done; !errors.Is(err, coordinator.ErrQuiesced) {
			cancel()
			t.Fatalf("coordinator join: %v", err)
		}
		cancel()
		if err := manager.AwaitStopped(ctx); err != nil {
			t.Fatal(err)
		}
		want := int64(1 - generation)
		if manager.adds.Load() != want || model.calls.Load() != want {
			t.Fatalf("generation %d actual admissions/model calls=%d/%d want %d", generation, manager.adds.Load(), model.calls.Load(), want)
		}
		page, err := store.Items(ctx, sid, sessionstore.BeforeFirst, 100)
		if err != nil || page.More {
			t.Fatalf("canonical verification page: %v more=%v", err, page.More)
		}
		completed := 0
		for _, item := range page.Items {
			if status, ok := item.Data.(sessionstore.ToolCallStatus); ok {
				for _, saved := range status.Operations {
					if saved.ID != op.ID {
						t.Fatalf("operation identity changed: %s", saved.ID)
					}
					if saved.Status == operation.StatusCompleted {
						completed++
					}
				}
			}
		}
		if completed != 1 {
			t.Fatalf("canonical terminal original operation count=%d", completed)
		}
	}
}
