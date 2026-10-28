package workerruntime

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func lifecycleFixture(t *testing.T) (sessionstore.Store, workerproto.Bootstrap) {
	t.Helper()
	b := bootstrap(t)
	b.ControlScope = "private-host/1"
	store, err := localfile.New(b.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), b.SessionID); err != nil {
		t.Fatal(err)
	}
	return store, b
}

func TestRuntimeFinalizerRequiresIndependentEvidenceAndNeverHostCompletion(t *testing.T) {
	store, b := lifecycleFixture(t)
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: b.ControlScope}})
	if err != nil {
		t.Fatal(err)
	}
	input := inbox.Input{ID: "stop-one", Kind: inbox.InputControl, Payload: payload}
	if err := store.AppendInput(t.Context(), b.SessionID, input); err != nil {
		t.Fatal(err)
	}
	// These flags are injected unit-test evidence; real joins are independently
	// exercised by the process integration tests, not inferred from this test.
	proof := &runtimeCleanupEvidence{managerSeen: true, ownersSeen: true, modelsSeen: true}
	if err := finalizeRuntimeLifecycle(store, b, proof); err != nil {
		t.Fatal(err)
	}
	history, err := sessionstore.ReadLifecycle(t.Context(), store, b.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if history.ShutdownResults[input.ID].Phase != "work_drained" || history.LastDrainSequence == 0 {
		t.Fatalf("missing scoped work receipt: %+v", history)
	}
	if !errors.Is(history.ValidateResume(), sessionstore.ErrLifecycleRecoveryRequired) {
		t.Fatal("runtime incorrectly authorized host authority release")
	}
}
func TestRuntimeFinalizerNoReceiptOnMissingOrFailedBarrier(t *testing.T) {
	for _, tc := range []struct {
		name  string
		proof *runtimeCleanupEvidence
	}{
		{"missing", nil}, {"manager-not-joined", &runtimeCleanupEvidence{ownersSeen: true, modelsSeen: true}},
		{"owners-not-joined", &runtimeCleanupEvidence{managerSeen: true, modelsSeen: true}},
		{"model-not-joined", &runtimeCleanupEvidence{managerSeen: true, ownersSeen: true}},
		{"manager-unknown", &runtimeCleanupEvidence{managerSeen: true, ownersSeen: true, modelsSeen: true, managerErr: &operation.CleanupUncertainError{UnknownRemoteIDs: []operation.ID{"unknown"}}}},
		{"owner-failure", &runtimeCleanupEvidence{managerSeen: true, ownersSeen: true, modelsSeen: true, ownersErr: errors.New("unproven child")}},
		{"model-close-failure", &runtimeCleanupEvidence{managerSeen: true, ownersSeen: true, modelsSeen: true, modelsErr: errors.New("close")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, b := lifecycleFixture(t)
			if err := finalizeRuntimeLifecycle(store, b, tc.proof); err == nil {
				t.Fatal("missing barrier accepted")
			}
			history, err := sessionstore.ReadLifecycle(t.Context(), store, b.SessionID)
			if err != nil {
				t.Fatal(err)
			}
			if history.LastDrainSequence != 0 {
				t.Fatal("false cleanup receipt persisted")
			}
		})
	}
}
