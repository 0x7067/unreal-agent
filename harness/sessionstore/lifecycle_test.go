package sessionstore

import (
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func lifecycleControl(t *testing.T, id inbox.ID, mode inbox.ControlMode, parameters any) inbox.Input {
	t.Helper()
	data, err := json.Marshal(inbox.ControlMessage{Mode: mode, Parameters: parameters})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: id, Kind: inbox.InputControl, Payload: data}
}
func TestLifecycleShutdownNeedsCommittedFinalOutcomeNotEOFOrWorkReceipt(t *testing.T) {
	var h LifecycleHistory
	apply := func(id inbox.ID, mode inbox.ControlMode, value any) {
		t.Helper()
		if err := h.Observe(lifecycleControl(t, id, mode, value)); err != nil {
			t.Fatal(err)
		}
	}
	apply("shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/1"})
	if !errors.Is(h.ValidateResume(), ErrLifecycleRecoveryRequired) {
		t.Fatal("pending intent did not fence resume")
	}
	apply("receipt", inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: "host/1"})
	if h.ValidateResume() == nil {
		t.Fatal("unrelated execution receipt authorized restart")
	}
	apply("work-drained", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/1", Phase: "work_drained"})
	if h.ValidateResume() == nil {
		t.Fatal("work-drained phase authorized competing writer before host finalization")
	}
	if err := h.Observe(lifecycleControl(t, "wrong", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/2", Phase: "completed"})); err == nil {
		t.Fatal("accepted mismatched generation")
	}
	apply("complete", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/1", Phase: "completed"})
	if err := h.ValidateResume(); err != nil {
		t.Fatal(err)
	}
	if err := h.Observe(lifecycleControl(t, "rewrite", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/1", Phase: "uncertain"})); err == nil {
		t.Fatal("rewrote final outcome")
	}
}
func TestLifecycleUncertainAbortRequiresReconciliation(t *testing.T) {
	var h LifecycleHistory
	for _, value := range []inbox.Input{
		lifecycleControl(t, "abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, TargetTurnID: "turn", Scope: "host/1"}),
		lifecycleControl(t, "uncertain", inbox.ForegroundAbortFinished, inbox.ForegroundAbortResult{Version: 1, InputID: "abort", TargetTurnID: "turn", Status: "uncertain"}),
	} {
		if err := h.Observe(value); err != nil {
			t.Fatal(err)
		}
		if h.ValidateResume() == nil {
			t.Fatal("unfinished abort authorized automatic execution")
		}
	}
	if err := h.Observe(lifecycleControl(t, "complete", inbox.ForegroundAbortFinished, inbox.ForegroundAbortResult{Version: 1, InputID: "abort", TargetTurnID: "turn", Status: "completed"})); err != nil {
		t.Fatal(err)
	}
	if err := h.ValidateResume(); err != nil {
		t.Fatal(err)
	}
}
