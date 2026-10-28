package taskgraph

import (
	"encoding/json/v2"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func recoveryOperation(t *testing.T, p any, cp *DurableCheckpoint) operation.Operation {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	typ := RunPlanType
	if _, ok := p.(DurableControlPlan); ok {
		typ = ControlPlanType
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: DurablePlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "run", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusAwaiting}
	if cp != nil {
		if cp.Execution.Workspace == "" {
			cp.Execution = DurableExecution{Workspace: "/workspace", Shell: "/bin/sh", BaseDirectory: "/operations"}
		}
		state, _ := operation.DecodeRemoteJobState(op)
		state.Handle, err = json.Marshal(cp)
		if err != nil {
			t.Fatal(err)
		}
		op.State, err = json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
	}
	return op
}
func recoveryTask(id string) Task {
	return Task{ID: id, Command: "true", Reads: []string{}, Writes: []string{}, Acceptance: "checked"}
}
func recoveryChild(t *testing.T, id string, done bool) operation.Operation {
	t.Helper()
	spec, err := operation.NewShellSpec(operation.ShellInput{Command: "true", Shell: "/bin/sh", Directory: "/workspace"}, "/operations", 4096)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: graphChildID("run", id, 1), Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
	if done {
		s, _ := operation.DecodeShellState(op)
		s.Result = &operation.ShellResult{}
		s.OutTruncated = false
		s.ErrTruncated = false
		op.State, err = json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		op.Status = operation.StatusCompleted
	}
	return op
}
func TestDurableRecoveryMixed(t *testing.T) {
	tasks := []Task{recoveryTask("A"), recoveryTask("B"), recoveryTask("C"), recoveryTask("D")}
	tasks[3].DependsOn = []string{"B", "C"}
	p := DurableRunPlan{Version: 1, ParentID: "parent", Run: RunPlan{Name: "graph", Tasks: tasks}}
	g, err := NewGraph("graph", tasks, 4)
	if err != nil {
		t.Fatal(err)
	}
	cp := DurableCheckpoint{Version: 2, ParentID: "parent", Scope: "scope", State: checkpoint{Version: 1, Graph: g, Attempts: map[string]*attempt{}}}
	for _, id := range []string{"A", "B", "C"} {
		if err := cp.State.Graph.Start(id); err != nil {
			t.Fatal(err)
		}
		cp.State.Attempts[id] = &attempt{Generation: 1, ChildOperation: recoveryChild(t, id, true), CommandDispatched: true, Phase: "execution"}
	}
	for _, id := range []string{"A", "B"} {
		if err := cp.State.Graph.Complete(id, 1, "ok", nil); err != nil {
			t.Fatal(err)
		}
		cp.State.Attempts[id].Phase = "awaiting_acceptance"
		cp.State.Attempts[id].OutputFingerprint = "fingerprint"
	}
	if err := cp.State.Graph.Accept("A", 1, "verified"); err != nil {
		t.Fatal(err)
	}
	op := recoveryOperation(t, p, &cp)
	if err := ValidateDurableRecovery(op, "parent"); err != nil {
		t.Fatal(err)
	}
	_, decoded, err := DecodeDurableRun(op)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.State.Graph.Find("D").Status != Pending {
		t.Fatal("dependent advanced")
	}
	if decoded.State.Graph.Find("A").Task.Reads == nil {
		t.Fatal("empty claims lost")
	}
	if err := ValidateDurableRecovery(op, "other"); err == nil {
		t.Fatal("parent mismatch admitted")
	}
	for _, phase := range []operation.ShellPhase{operation.ShellPhaseProcess, operation.ShellPhaseReadOut, "future"} {
		t.Run(string(phase), func(t *testing.T) {
			a := cp.State.Attempts["C"]
			s, _ := operation.DecodeShellState(a.ChildOperation)
			s.Phase = phase
			s.ProcessGroupID = 7654321
			a.ChildOperation.State, _ = json.Marshal(s)
			if err := ValidateDurableRecovery(recoveryOperation(t, p, &cp), "parent"); err == nil {
				t.Fatal("uncertain PID/phase admitted")
			}
		})
	}
}
func TestDurableRecoveryFirstLaunchAndRefusal(t *testing.T) {
	p := DurableRunPlan{Version: 1, ParentID: "parent", Run: RunPlan{Name: "graph", Tasks: []Task{recoveryTask("A")}}}
	op := recoveryOperation(t, p, nil)
	if err := ValidateDurableRecovery(op, "parent"); err != nil {
		t.Fatal(err)
	}
	op.Status = operation.StatusCompleted
	if err := ValidateDurableRecovery(op, "parent"); err == nil {
		t.Fatal("unproven terminal admitted")
	}
	g, _ := NewGraph(p.Run.Name, p.Run.Tasks, 0)
	_ = g.Start("A")
	cp := DurableCheckpoint{Version: 2, ParentID: "parent", Scope: "scope", State: checkpoint{Version: 1, Graph: g, Attempts: map[string]*attempt{"A": {Generation: 1, Phase: "input_validation", ChildOperation: recoveryChild(t, "A", false)}}}}
	if err := ValidateDurableRecovery(recoveryOperation(t, p, &cp), "parent"); err != nil {
		t.Fatal(err)
	}
	cp.State.Attempts["A"].Phase = "execution"
	cp.State.Attempts["A"].CommandDispatched = true
	if err := ValidateDurableRecovery(recoveryOperation(t, p, &cp), "parent"); err == nil {
		t.Fatal("dispatched ready admitted")
	}
	for _, mutate := range []func(*DurableCheckpoint){func(c *DurableCheckpoint) { c.Version = 99 }, func(c *DurableCheckpoint) { c.ParentID = "other" }, func(c *DurableCheckpoint) { c.Scope = "" }, func(c *DurableCheckpoint) { c.State.Version = 2 }, func(c *DurableCheckpoint) { c.State.Graph.Nodes[0].Generation = 0 }} {
		c := cp
		mutate(&c)
		if _, _, err := DecodeDurableRun(recoveryOperation(t, p, &c)); err == nil {
			t.Fatal("malformed checkpoint admitted")
		}
	}
}
func TestDurableStrictCodecs(t *testing.T) {
	p := DurableRunPlan{Version: 1, ParentID: "parent", Run: RunPlan{Name: "graph", Tasks: []Task{recoveryTask("A")}}}
	for _, change := range []func(*operation.RemoteJobState){func(s *operation.RemoteJobState) { s.Plan.Version = 1 }, func(s *operation.RemoteJobState) {
		s.Plan.Data = []byte(`{"version":1,"parent_id":"parent","run":{"name":"graph","tasks":[]},"unknown":true}`)
	}, func(s *operation.RemoteJobState) { s.Handle = []byte(`null`) }, func(s *operation.RemoteJobState) { s.ResultBytes = -1 }} {
		op := recoveryOperation(t, p, nil)
		s, _ := operation.DecodeRemoteJobState(op)
		change(&s)
		op.State, _ = json.Marshal(s)
		if _, _, err := DecodeDurableRun(op); err == nil {
			t.Fatal("invalid envelope admitted")
		}
	}
	c := DurableControlPlan{Version: 1, ParentID: "parent", Scope: "scope", Control: ControlPlan{Name: "graph", Action: "revise", Tasks: []Task{recoveryTask("B")}}}
	c.Control.Tasks[0].DependsOn = []string{"existing"}
	if _, err := DecodeDurableControl(recoveryOperation(t, c, nil)); err != nil {
		t.Fatal(err)
	}
	c.Control.Tasks[0].Prompt = "agent"
	if _, err := DecodeDurableControl(recoveryOperation(t, c, nil)); err == nil {
		t.Fatal("agent revision admitted")
	}
}

func TestDurableRecoveryRejectsNestedUnknownMembers(t *testing.T) {
	p := DurableRunPlan{Version: 1, ParentID: "parent", Run: RunPlan{Name: "graph", Tasks: []Task{recoveryTask("A")}}}
	g, _ := NewGraph("graph", p.Run.Tasks, 0)
	_ = g.Start("A")
	cp := DurableCheckpoint{Version: 2, ParentID: "parent", Scope: "scope", State: checkpoint{Version: 1, Graph: g, Attempts: map[string]*attempt{"A": {Generation: 1, Phase: "execution", CommandDispatched: true, ChildOperation: recoveryChild(t, "A", true)}}}}
	for _, target := range []string{"child", "checkpoint", "plan", "envelope"} {
		t.Run(target, func(t *testing.T) {
			op := recoveryOperation(t, p, &cp)
			s, _ := operation.DecodeRemoteJobState(op)
			add := func(b []byte) []byte { return append(append([]byte{}, b[:len(b)-1]...), []byte(`,"unknown":true}`)...) }
			switch target {
			case "child":
				a := cp.State.Attempts["A"]
				old := a.ChildOperation.State
				a.ChildOperation.State = add(old)
				op = recoveryOperation(t, p, &cp)
				a.ChildOperation.State = old
			case "checkpoint":
				s.Handle = add(s.Handle)
				op.State, _ = json.Marshal(s)
			case "plan":
				s.Plan.Data = add(s.Plan.Data)
				op.State, _ = json.Marshal(s)
			case "envelope":
				op.State = add(op.State)
			}
			if err := ValidateDurableRecovery(op, "parent"); err == nil {
				t.Fatal("unknown member admitted")
			}
		})
	}
}
