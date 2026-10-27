package operation

import (
	"context"
	"encoding/json/jsontext"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
)

func managementTestOperation(t *testing.T, id ID, planType RemoteJobPlanType, version RemoteJobPlanVersion, data string) Operation {
	t.Helper()
	spec, err := NewRemoteJobSpec(RemoteJobPlan{Type: planType, Version: version, Data: jsontext.Value(data)})
	if err != nil {
		t.Fatal(err)
	}
	return Operation{ID: id, Type: spec.Type, Version: spec.Version, Status: StatusReady, State: spec.State, MaxOutputLength: spec.MaxOutputLength}
}

func TestManagementClassificationNarrowVersionedSet(t *testing.T) {
	for _, test := range []struct {
		plan    RemoteJobPlanType
		version RemoteJobPlanVersion
		data    string
		allowed bool
	}{
		{"subagent.control", 1, `{"Action":"list"}`, true},
		{"subagent.control", 1, `{"Action":"status"}`, true},
		{"subagent.control", 1, `{"action":"stop"}`, true},
		{"taskgraph.control", 1, `{"action":"status"}`, true},
		{"taskgraph.control", 1, `{"Action":"cancel"}`, true},
		{"subagent.control", 1, `{"Action":"message"}`, false},
		{"subagent.control", 1, `{"Action":"run"}`, false},
		{"taskgraph.control", 1, `{"action":"retry"}`, false},
		{"taskgraph.control", 1, `{"action":"revise"}`, false},
		{"taskgraph.run", 1, `{"action":"status"}`, false},
		{"other.control", 1, `{"action":"stop"}`, false},
		{"subagent.control", 2, `{"Action":"list"}`, false},
		{"subagent.control", 1, `{"action":"stop","Action":"list"}`, false},
		{"subagent.control", 1, `{"ACTION":"list"}`, false},
		{"subagent.control", 1, `{"Action":null}`, false},
		{"subagent.control", 1, `{"Action":123}`, false},
		{"subagent.control", 1, `[]`, false},
	} {
		op := managementTestOperation(t, "classify", test.plan, test.version, test.data)
		if got := isManagementOperation(op); got != test.allowed {
			t.Errorf("%s v%d %s: got %t", test.plan, test.version, test.data, got)
		}
	}
}

type managementRemote struct {
	*budgetRemote
	planType RemoteJobPlanType
}

func (h *managementRemote) RemoteJobPlanType() RemoteJobPlanType { return h.planType }

func TestSaturatedWorkAdmitsOnlyBoundedInspectionAndStopControls(t *testing.T) {
	a := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 1})
	ctx, cancel := context.WithCancel(t.Context())
	makeHandler := func() *budgetRemote {
		return &budgetRemote{updates: make(chan Operation), added: make(chan Operation, 8), canceled: make(chan ID, 8)}
	}
	work := makeHandler()
	agent := &managementRemote{budgetRemote: makeHandler(), planType: "subagent.control"}
	graph := &managementRemote{budgetRemote: makeHandler(), planType: "taskgraph.control"}
	m := NewLocalOperationManagerWithOptions(ctx, LocalOperationManagerOptions{ResourceAccount: a}, work, agent, graph)
	defer func() {
		cancel()
		for range m.Updates() {
		}
	}()
	running := managementTestOperation(t, "executing-child", "budget-test", 1, `{}`)
	if err := m.Add(running); err != nil {
		t.Fatal(err)
	}
	launched := []Operation{<-work.added}
	handlers := []*budgetRemote{work}
	for _, test := range []struct {
		id      ID
		plan    RemoteJobPlanType
		data    string
		handler *budgetRemote
	}{
		{"list", "subagent.control", `{"Action":"list"}`, agent.budgetRemote},
		{"inspect", "subagent.control", `{"Action":"status"}`, agent.budgetRemote},
		{"stop", "subagent.control", `{"Action":"stop"}`, agent.budgetRemote},
		{"graph-status", "taskgraph.control", `{"action":"status"}`, graph.budgetRemote},
		{"graph-cancel", "taskgraph.control", `{"action":"cancel"}`, graph.budgetRemote},
	} {
		op := managementTestOperation(t, test.id, test.plan, 1, test.data)
		if err := m.Add(op); err != nil {
			t.Fatal(err)
		}
		select {
		case dispatched := <-test.handler.added:
			launched = append(launched, dispatched)
			handlers = append(handlers, test.handler)
		case <-time.After(5 * time.Second):
			t.Fatalf("saturation blocked management %s", test.id)
		}
	}
	snapshot := a.Snapshot()
	if snapshot.WorkOwned != 1 || snapshot.ManagementOwned != 5 || snapshot.Owned != 6 {
		t.Fatalf("management altered executing ownership: %+v", snapshot)
	}
	for _, test := range []struct {
		plan RemoteJobPlanType
		data string
	}{
		{"subagent.control", `{"Action":"message"}`},
		{"taskgraph.control", `{"action":"retry"}`},
		{"unknown.control", `{"action":"status"}`},
		{"subagent.run", `{"Action":"status"}`},
	} {
		op := managementTestOperation(t, ID("rejected-"+string(test.plan)), test.plan, 1, test.data)
		if err := m.Add(op); err != nil {
			t.Fatal(err)
		}
		select {
		case failed := <-m.Updates():
			state, err := DecodeRemoteJobState(failed)
			if err != nil {
				t.Fatal(err)
			}
			if failed.Status != StatusFailed || !strings.Contains(state.TerminalError, "capacity exhausted") {
				t.Fatalf("work escaped full account: %+v", failed)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("capacity rejection blocked")
		}
	}
	for i, op := range launched {
		op.Status = StatusCanceled
		handlers[i].updates <- op
	}
	for range launched {
		select {
		case op := <-m.Updates():
			if op.Status != StatusCanceled {
				t.Fatal(op.Status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("terminal control not drained")
		}
	}
	if err := m.Add(running); err != nil {
		t.Fatal(err)
	}
	if a.Snapshot().Owned != 0 {
		t.Fatal("drained controls or work retained capacity")
	}
}
