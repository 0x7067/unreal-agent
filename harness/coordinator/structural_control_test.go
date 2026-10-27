package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func invocationInput(t *testing.T, id inbox.ID, arguments string) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Call: llm.ToolCall{CallID: string(id), Name: tool.AgentControlName, Arguments: arguments}}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}

func controlTestCoordinator(t *testing.T) (*coordinator, *fakeStore, *fakeOperationManager) {
	t.Helper()
	store := emptyFakeStore()
	manager := newFakeOperationManager()
	c := newTestCoordinator(store, newTestInbox(t), manager, contextbuilder.NewBuilder(), tool.NewRegistry(tool.StaticTranslators{AgentControl: toolsubagent.NewAgentControl()}, tool.AgentControlName))
	return c, store, manager
}

func TestStructuralControlPersistsBeforeDispatchWithoutModel(t *testing.T) {
	c, store, manager := controlTestCoordinator(t)
	input := invocationInput(t, "stop-input", `{"action":"stop","handle":"opaque","generation":1}`)
	if err := c.handleInboxInput(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if len(store.appendedInputs) != 1 || len(store.savedOperations) != 1 || len(manager.adds) != 1 {
		t.Fatalf("missing commit boundaries: inputs%d checkpoints%d dispatch%d", len(store.appendedInputs), len(store.savedOperations), len(manager.adds))
	}
	if manager.adds[0].ID != structuralOperationID(input.ID) {
		t.Fatal("unstable control identity")
	}
	if c.state.callModel || c.pendingInputs() != 0 || c.isIdle() {
		t.Fatal("control started model or hid owned operation")
	}
	value := manager.adds[0]
	remote, err := operation.DecodeRemoteJobState(value)
	if err != nil {
		t.Fatal(err)
	}
	remote.TerminalResult = `{"Action":"stop","State":"cancellation_requested","DrainConfirmed":false}`
	step, err := operation.UpdateRemoteJob(value, remote, operation.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	if err = c.handleOperationUpdate(t.Context(), *step.Operation); err != nil {
		t.Fatal(err)
	}
	if len(store.appendedInputs) != 2 || !c.isIdle() {
		t.Fatal("terminal checkpoint/result not published")
	}
	result, err := store.appendedInputs[1].DecodeControlMessage()
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != inbox.ToolInvocationFinished || result.Parameters.(inbox.ToolInvocationResult).Status != "completed" {
		t.Fatalf("bad outcome %+v", result)
	}
	// Duplicate delivery cannot redispatch an already acknowledged control.
	if err = c.handleInboxInput(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if len(manager.adds) != 1 {
		t.Fatal("duplicate control dispatched again")
	}
}

func TestStructuralControlStoreFailuresDoNotDispatch(t *testing.T) {
	for _, inputFailure := range []bool{true, false} {
		c, store, manager := controlTestCoordinator(t)
		if inputFailure {
			store.appendInputErr = errors.New("input fsync failed")
		} else {
			store.saveOperationErr = errors.New("operation fsync failed")
		}
		err := c.handleInboxInput(t.Context(), invocationInput(t, "commit-fails", `{"action":"list"}`))
		if err == nil || len(manager.adds) != 0 {
			t.Fatalf("failed persistence dispatched work: %v %+v", err, manager.adds)
		}
	}
}

func TestStructuralControlRestoreCrashGapAndFinishedInput(t *testing.T) {
	input := invocationInput(t, "crash-gap", `{"action":"list"}`)
	for _, checkpoint := range []bool{false, true} {
		c, store, manager := controlTestCoordinator(t)
		store.items = []sessionstore.Item{storedItem(1, sessionstore.ItemInput, input)}
		if checkpoint {
			p, _ := json.Marshal(toolsubagent.ControlPlan{Action: "list"})
			spec, _ := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.ControlPlanType, Version: 1, Data: p})
			c.dependencies.Restored.Operations = []operation.Operation{{ID: structuralOperationID(input.ID), Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady}}
		}
		if err := c.restore(t.Context()); err != nil {
			t.Fatal(err)
		}
		if len(manager.adds) != 0 {
			t.Fatal("restore directly dispatched")
		}
		if err := c.dispatchOperationsToManager(); err != nil {
			t.Fatal(err)
		}
		if len(manager.adds) != 1 || manager.adds[0].ID != structuralOperationID(input.ID) {
			t.Fatal("crash gap lost stable control")
		}
	}
	c, store, manager := controlTestCoordinator(t)
	result, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ToolInvocationFinished, Parameters: inbox.ToolInvocationResult{InputID: input.ID, CallID: string(input.ID), Status: "failed", Error: "stale target"}})
	store.items = []sessionstore.Item{storedItem(1, sessionstore.ItemInput, input), storedItem(2, sessionstore.ItemInput, inbox.Input{ID: "result", Kind: inbox.InputControl, Payload: result})}
	if err := c.restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := c.dispatchOperationsToManager(); err != nil {
		t.Fatal(err)
	}
	if len(manager.adds) != 0 || len(store.savedOperations) != 0 {
		t.Fatal("finished control replayed")
	}
}

func (s *fakeStore) InitializeControlOperation(ctx context.Context, id session.ID, _ inbox.ID, value operation.Operation) error {
	return s.SaveOperation(ctx, id, value)
}

func TestStructuralControlRealStoreInitializationAndCrashReplay(t *testing.T) {
	c, _, manager := controlTestCoordinator(t)
	store, err := localfile.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), c.dependencies.SessionID); err != nil {
		t.Fatal(err)
	}
	c.dependencies.Sessions = store
	input := invocationInput(t, "real-store", `{"action":"list"}`)
	manager.addError = func(value operation.Operation) error {
		restored, err := store.Resume(t.Context(), c.dependencies.SessionID)
		if err != nil {
			return err
		}
		if len(restored.Operations) != 1 || restored.Operations[0].ID != value.ID {
			return errors.New("dispatch overtook committed initialization")
		}
		return nil
	}
	if err = c.handleInboxInput(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	value := manager.adds[0]
	remote, _ := operation.DecodeRemoteJobState(value)
	remote.TerminalResult = `{"Action":"list","State":"snapshot","DrainConfirmed":false}`
	step, _ := operation.UpdateRemoteJob(value, remote, operation.StatusCompleted)
	// Crash after the terminal checkpoint but before its result input.
	if err = store.SaveOperation(t.Context(), c.dependencies.SessionID, *step.Operation); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Resume(t.Context(), c.dependencies.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Operations) != 1 {
		t.Fatal("terminal control lost before outcome commit")
	}
	next, _, nextManager := controlTestCoordinator(t)
	next.dependencies.Sessions = store
	next.dependencies.Restored = restored
	if err = next.restore(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err = next.dispatchOperationsToManager(); err != nil {
		t.Fatal(err)
	}
	if len(nextManager.adds) != 0 {
		t.Fatal("terminal control reexecuted")
	}
	final, err := store.Resume(t.Context(), c.dependencies.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.Operations) != 0 {
		t.Fatal("completed result retained operation")
	}
	page, err := store.Items(t.Context(), c.dependencies.SessionID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("unexpected fabricated model history: %+v", page.Items)
	}
	outcome, _ := page.Items[1].Data.(inbox.Input).DecodeControlMessage()
	if outcome.Mode != inbox.ToolInvocationFinished {
		t.Fatal("result not persisted")
	}
}
