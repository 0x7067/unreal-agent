package lifecycleproof

import (
	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"testing"
)

func TestRemoteWorkloadsCannotUseGenericTerminalStatus(t *testing.T) {
	for _, kind := range []operation.RemoteJobPlanType{durablejob.PlanType, extension.PlanType, reminder.TaskType, "unknown.future"} {
		op := remote(t, kind, 1)
		uncertain := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
		if err := Reconcile(uncertain, nil, map[operation.ID]operation.Operation{op.ID: op}, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{kind: 1}); err == nil {
			t.Fatalf("%s terminal status and arbitrary joined claim bypassed typed proof", kind)
		}
	}
}
func TestEchoHistoricalProofRequiresExactHandlerAttestation(t *testing.T) {
	d := extension.Descriptor{Name: extension.Name, Generation: 1, SchemaVersion: 1, ImplementationVersion: 1, Implementation: "A"}
	spec, err := extension.NewSpec(d, "old", "owner")
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "old-echo", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	state.TerminalResult = "A:old"
	step, err := operation.UpdateRemoteJob(op, state, operation.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	op = *step.Operation
	uncertain := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
	canonical := map[operation.ID]operation.Operation{op.ID: op}
	joined := map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{extension.PlanType: extension.PlanVersion}
	if err = Reconcile(uncertain, nil, canonical, joined); err != nil {
		t.Fatal(err)
	}
	if err = Reconcile(uncertain, nil, canonical, nil); err == nil {
		t.Fatal("historical output proves no handler join")
	}
	if err = Reconcile(uncertain, nil, canonical, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{extension.PlanType: 99}); err == nil {
		t.Fatal("wrong handler version accepted")
	}
	state.TerminalResult = "B:old"
	step, err = operation.UpdateRemoteJob(op, state, operation.StatusCompleted)
	if err != nil {
		t.Fatal(err)
	}
	canonical[op.ID] = *step.Operation
	if err = Reconcile(uncertain, nil, canonical, joined); err == nil {
		t.Fatal("selected B result substituted for committed A identity")
	}
}
