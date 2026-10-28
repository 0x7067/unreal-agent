package localfile

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

func controlInput(t *testing.T, id inbox.ID) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Call: llm.ToolCall{Name: "AgentControl", CallID: "call-1", Arguments: `{"Action":"list"}`}}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
}

func controlResult(t *testing.T, id inbox.ID) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ToolInvocationFinished, Parameters: inbox.ToolInvocationResult{InputID: id, CallID: "call-1", Status: "completed"}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: "result-1", Kind: inbox.InputControl, Payload: payload}
}

func controlOperation(t *testing.T, id inbox.ID, planType operation.RemoteJobPlanType, version operation.RemoteJobPlanVersion) operation.Operation {
	t.Helper()
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: planType, Version: version, Data: []byte(`{"Action":"list"}`)})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: controlOperationID(id), Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
}

func TestControlOperationDurableResumeUntilExplicitResult(t *testing.T) {
	directory := t.TempDir()
	store, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), "session-1"); err != nil {
		t.Fatal(err)
	}
	input := controlInput(t, "control-1")
	value := controlOperation(t, input.ID, "subagent.control", 1)
	if err := store.SaveOperation(t.Context(), "session-1", value); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unknown SaveOperation = %v", err)
	}
	if err := store.AppendInput(t.Context(), "session-1", input); err != nil {
		t.Fatal(err)
	}
	// A crash after input persistence leaves the original input visible for translation.
	resume, err := store.Resume(t.Context(), "session-1")
	if err != nil || len(resume.Operations) != 0 || !reflect.DeepEqual(resume.InputIDs, []inbox.ID{input.ID}) {
		t.Fatalf("input-only resume = %#v, %v", resume, err)
	}
	page, err := store.Items(t.Context(), "session-1", 0, 10)
	if err != nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0].Data, input) {
		t.Fatalf("input-only history = %#v, %v", page, err)
	}
	if err := store.InitializeControlOperation(t.Context(), "session-1", input.ID, value); err != nil {
		t.Fatal(err)
	}
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCompleted} {
		value.Status = status
		if status != operation.StatusReady {
			if err := store.SaveOperation(t.Context(), "session-1", value); err != nil {
				t.Fatal(err)
			}
		}
		reopened, err := New(directory)
		if err != nil {
			t.Fatal(err)
		}
		resume, err := reopened.Resume(t.Context(), "session-1")
		if err != nil || !reflect.DeepEqual(resume.Operations, []operation.Operation{value}) {
			t.Fatalf("%s resume = %#v, %v", status, resume.Operations, err)
		}
	}
	encoded, err := os.ReadFile(store.sessionPath("session-1"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"type":"control_operation_v1","data":{"Version":1`)) || !bytes.Contains(encoded, []byte(`"Version":2,"Session"`)) {
		t.Fatalf("missing explicit record/header versions: %s", encoded)
	}
	if err := store.AppendInput(t.Context(), "session-1", controlResult(t, input.ID)); err != nil {
		t.Fatal(err)
	}
	// A duplicate input must not restore consumed authority.
	if err := store.AppendInput(t.Context(), "session-1", input); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(directory)
	if err != nil {
		t.Fatal(err)
	}
	resume, err = reopened.Resume(t.Context(), "session-1")
	if err != nil || len(resume.Operations) != 0 {
		t.Fatalf("finished resume = %#v, %v", resume.Operations, err)
	}
	if err := reopened.InitializeControlOperation(t.Context(), "session-1", input.ID, value); err == nil {
		t.Fatal("finished control reinitialized")
	}
}

func TestControlOperationRejectsInvalidAuthorizationAndRecords(t *testing.T) {
	input := controlInput(t, "control-1")
	valid := controlOperation(t, input.ID, "subagent.control", 1)
	tests := []struct {
		name      string
		inputs    []inbox.Input
		record    controlOperationRecord
		duplicate bool
	}{
		{name: "missing prior input", record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: valid}},
		{name: "external input", inputs: []inbox.Input{{ID: input.ID, Kind: inbox.InputExternal}}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: valid}},
		{name: "finished input", inputs: []inbox.Input{input, controlResult(t, input.ID)}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: valid}},
		{name: "missing input ID", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 1, Operation: valid}},
		{name: "invalid record version", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 2, InputID: input.ID, Operation: valid}},
		{name: "missing record version", inputs: []inbox.Input{input}, record: controlOperationRecord{InputID: input.ID, Operation: valid}},
		{name: "wrong ID", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: controlOperation(t, "other", "subagent.control", 1)}},
		{name: "noncontrol plan", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: controlOperation(t, input.ID, "subagent.run", 1)}},
		{name: "wrong plan version", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: controlOperation(t, input.ID, "subagent.control", 2)}},
		{name: "duplicate", inputs: []inbox.Input{input}, record: controlOperationRecord{Version: 1, InputID: input.ID, Operation: valid}, duplicate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newStoredState("session-1", stateCreatedAt)
			for _, input := range test.inputs {
				if err := state.appendInput(input, stateUpdatedAt); err != nil {
					t.Fatal(err)
				}
			}
			encoded, err := encodeInitialLog(state.Snapshot.Session, state.Items)
			if err != nil {
				t.Fatal(err)
			}
			line, err := encodeRecord(recordControlOperation, test.record)
			if err != nil {
				t.Fatal(err)
			}
			encoded = append(encoded, line...)
			if test.duplicate {
				encoded = append(encoded, line...)
			}
			if _, _, err := decodeLog(encoded); err == nil {
				t.Fatal("accepted invalid control record")
			}
		})
	}
	// Only ready checkpoints can initialize, even with a valid input.
	state := newSessionHead("session-1", stateCreatedAt)
	if _, err := state.appendInput(input, stateUpdatedAt); err != nil {
		t.Fatal(err)
	}
	terminal := valid
	terminal.Status = operation.StatusCompleted
	if err := state.initializeControlOperation(input.ID, terminal); err == nil {
		t.Fatal("initialized terminal operation")
	}
	if err := state.initializeControlOperation(input.ID, valid); err != nil {
		t.Fatalf("invalid attempt mutated authorization: %v", err)
	}
	if err := state.initializeControlOperation(input.ID, valid); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("duplicate initialization = %v", err)
	}
}

func TestForkDoesNotInheritControlAuthorityOrOperations(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), "parent"); err != nil {
		t.Fatal(err)
	}
	input := controlInput(t, "control-1")
	value := controlOperation(t, input.ID, "subagent.control", 1)
	if err := store.AppendInput(t.Context(), "parent", input); err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeControlOperation(t.Context(), "parent", input.ID, value); err != nil {
		t.Fatal(err)
	}
	uninitialized := controlInput(t, "control-2")
	if err := store.AppendInput(t.Context(), "parent", uninitialized); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendTurn(t.Context(), "parent", session.Turn{ID: "turn-1", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Fork(t.Context(), "child", "parent", "turn-1"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []inbox.ID{input.ID, uninitialized.ID} {
		if err := store.InitializeControlOperation(t.Context(), "child", id, controlOperation(t, id, "subagent.control", 1)); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("inherited initialization = %v", err)
		}
	}
	resume, err := store.Resume(t.Context(), "child")
	if err != nil || len(resume.Operations) != 0 {
		t.Fatalf("fork resume = %#v, %v", resume.Operations, err)
	}
	// New controls in a fork still work.
	fresh := controlInput(t, "control-3")
	if err := store.AppendInput(t.Context(), "child", fresh); err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeControlOperation(t.Context(), "child", fresh.ID, controlOperation(t, fresh.ID, "subagent.control", 1)); err != nil {
		t.Fatal(err)
	}
	resume, err = store.Resume(t.Context(), "child")
	if err != nil || len(resume.Operations) != 1 {
		t.Fatalf("fresh fork resume = %#v, %v", resume.Operations, err)
	}
}

func TestFailedControlInitializationEvictsMutatedCachedHead(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), "session-1"); err != nil {
		t.Fatal(err)
	}
	input := controlInput(t, "control-1")
	value := controlOperation(t, input.ID, "subagent.control", 1)
	if err := store.AppendInput(t.Context(), "session-1", input); err != nil {
		t.Fatal(err)
	}
	path := store.sessionPath("session-1")
	backup := path + ".backup"
	if err := os.Rename(path, backup); err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeControlOperation(t.Context(), "session-1", input.ID, value); err == nil {
		t.Fatal("missing log append succeeded")
	}
	if _, _, ok := store.getCachedWriteState("session-1"); ok {
		t.Fatal("failed append retained mutated cache")
	}
	if err := os.Rename(backup, path); err != nil {
		t.Fatal(err)
	}
	if err := store.InitializeControlOperation(t.Context(), "session-1", input.ID, value); err != nil {
		t.Fatalf("retry after failed append: %v", err)
	}
	resume, err := store.Resume(t.Context(), "session-1")
	if err != nil || !reflect.DeepEqual(resume.Operations, []operation.Operation{value}) {
		t.Fatalf("retried resume = %#v, %v", resume.Operations, err)
	}
}

func TestOldFormatTwoSessionAcceptsNewControls(t *testing.T) {
	encoded, err := os.ReadFile("testdata/golden-session.session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.sessionPath("golden-session"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	input := controlInput(t, "new-control")
	if err := store.AppendInput(t.Context(), "golden-session", input); err != nil {
		t.Fatal(err)
	}
	value := controlOperation(t, input.ID, "subagent.control", 1)
	if err := store.InitializeControlOperation(t.Context(), "golden-session", input.ID, value); err != nil {
		t.Fatal(err)
	}
	state, _, err := store.readState(t.Context(), "golden-session")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.controlOperationInputs[value.ID]; !ok {
		t.Fatal("new control missing from old session")
	}
}

func TestCanonicalControlOperationsSurviveOutcomeProjection(t *testing.T) {
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), "s"); err != nil {
		t.Fatal(err)
	}
	input := controlInput(t, "input")
	if err = store.AppendInput(t.Context(), "s", input); err != nil {
		t.Fatal(err)
	}
	op := controlOperation(t, input.ID, "subagent.control", 1)
	if err = store.InitializeControlOperation(t.Context(), "s", input.ID, op); err != nil {
		t.Fatal(err)
	}
	state, _ := operation.DecodeRemoteJobState(op)
	state.TerminalResult = `{"Action":"list","State":"snapshot","DrainConfirmed":false}`
	step, err := operation.UpdateRemoteJob(op, state, operation.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveOperation(t.Context(), "s", *step.Operation); err != nil {
		t.Fatal(err)
	}
	if err = store.AppendInput(t.Context(), "s", controlResult(t, input.ID)); err != nil {
		t.Fatal(err)
	}
	resumed, err := store.Resume(t.Context(), "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Operations) != 0 {
		t.Fatal("completed control incorrectly resumed")
	}
	canonical, err := store.ControlOperations(t.Context(), "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(canonical) != 1 || !reflect.DeepEqual(canonical[0], *step.Operation) {
		t.Fatalf("canonical checkpoint missing: %+v", canonical)
	}
	canonical[0].State[0] = '!'
	fresh, err := store.ControlOperations(t.Context(), "s")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fresh[0], *step.Operation) {
		t.Fatal("reader aliases canonical storage")
	}
}
