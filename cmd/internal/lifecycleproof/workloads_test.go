package lifecycleproof

import (
	"context"
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"testing"
	"time"
)

func TestTimerTerminalProofRequiresExactHandlerJoin(t *testing.T) {
	spec, err := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: "parent", DueTime: time.Now().UTC().Add(time.Minute), Ownership: reminder.Ownership{Mode: "background"}, Message: "notification"})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "timer", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
	h := reminder.NewHandler(reminder.Config{})
	if err = h.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	<-h.RemoteJobUpdates()
	if err = h.CancelRemoteJob(op.ID, "explicit"); err != nil {
		t.Fatal(err)
	}
	op = <-h.RemoteJobUpdates()
	if err = h.Close(); err != nil {
		t.Fatal(err)
	}
	if err = h.AwaitStopped(context.Background()); err != nil {
		t.Fatal(err)
	}
	uncertain := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
	canonical := map[operation.ID]operation.Operation{op.ID: op}
	joined := map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{reminder.TaskType: reminder.TaskVersion}
	if err = Reconcile(uncertain, nil, canonical, joined); err != nil {
		t.Fatal(err)
	}
	if err = Reconcile(uncertain, nil, canonical, nil); err == nil {
		t.Fatal("terminal checkpoint substituted for actual handler join")
	}
	if err = Reconcile(uncertain, errors.New("owner still running"), canonical, joined); err == nil {
		t.Fatal("owner join failure ignored")
	}
	op.Status = operation.StatusAwaiting
	canonical[op.ID] = op
	if err = Reconcile(uncertain, nil, canonical, joined); err == nil {
		t.Fatal("nonterminal timer treated as drained")
	}
}
