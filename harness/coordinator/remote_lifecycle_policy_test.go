package coordinator

import (
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"testing"
)

type policyRegistry struct {
	tool.Registry
	mode    string
	handled bool
	proof   error
	target  string
}

func (p *policyRegistry) ClassifyRemoteForeground(_ operation.Operation, target string) (string, bool, error) {
	p.target = target
	return p.mode, p.handled, nil
}
func (p *policyRegistry) RemoteCleanupProof(operation.Operation) error { return p.proof }

type policyManager struct{ operation.Manager }

func (policyManager) LocalCleanupProof([]operation.ID) error {
	return errors.New("no local proof for remote work")
}
func TestOptionalRemoteLifecyclePolicyRemainsFenced(t *testing.T) {
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: "test.pure.policy", Version: 1, Data: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "owned", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusAwaiting}
	p := &policyRegistry{mode: "foreground", handled: true}
	c := New(Dependencies{Tools: p, Operations: policyManager{}}).(*coordinator)
	c.state.currentTurnID = session.TurnID("exact-current")
	c.state.operations[op.ID] = op
	targets, err := c.foregroundAbortTargets()
	if err != nil || len(targets) != 1 || targets[0] != op.ID || p.target != "exact-current" {
		t.Fatalf("foreground %v %v target=%q", targets, err, p.target)
	}
	p.mode = "terminal"
	if _, err = c.foregroundAbortTargets(); err == nil {
		t.Fatal("nonterminal falsely classified drained")
	}
	op.Status = operation.StatusCompleted
	c.state.operations[op.ID] = op
	p.proof = errors.New("executor wait unconfirmed")
	if _, err = c.foregroundAbortTargets(); err == nil {
		t.Fatal("terminal status substituted for executor proof")
	}
	if err = c.abortCleanupProof([]operation.ID{op.ID}); err == nil {
		t.Fatal("unproved remote cleanup")
	}
	p.proof = nil
	if err = c.abortCleanupProof([]operation.ID{op.ID}); err != nil {
		t.Fatal(err)
	}
	if targets, err = c.foregroundAbortTargets(); err != nil || len(targets) != 0 {
		t.Fatal(targets, err)
	}
	op.Status = operation.StatusAwaiting
	c.state.operations[op.ID] = op
	p.mode = "background"
	if targets, err = c.foregroundAbortTargets(); err != nil || len(targets) != 0 {
		t.Fatal(targets, err)
	}
	p.handled = false
	if _, err = c.foregroundAbortTargets(); err == nil {
		t.Fatal("unknown remote plan lost native refusal")
	}
	p.handled = true
	p.mode = "invented"
	if _, err = c.foregroundAbortTargets(); err == nil {
		t.Fatal("unknown policy mode accepted")
	}
}
