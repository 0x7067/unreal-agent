package subagent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func TestHubExternalRotationAndAuthority(t *testing.T) {
	h := supervisionHub(t)
	other := supervisionHub(t)
	var calls atomic.Int32
	callback := func(ctx context.Context, p ControlPlan) (any, error) {
		calls.Add(1)
		return map[string]string{"State": "observed"}, nil
	}
	a := AgentActivity{ChildActivity: ChildActivity{Name: "same", SessionID: "child", Generation: 42}, Purpose: "review", Phase: "running"}
	first, gen, err := h.BindExternal(a, callback)
	if err != nil || gen != 1 {
		t.Fatalf("bind: %s %d %v", first, gen, err)
	}
	second, _, err := other.BindExternal(a, callback)
	if err != nil || first == second {
		t.Fatal("binding did not rotate", err)
	}
	for i, p := range []ControlPlan{{Action: "stop", Handle: second, Generation: 1}, {Action: "stop", Handle: first, Generation: 2}, {Action: "stop", Handle: "same", Generation: 1}} {
		id := operation.ID(string(rune('a' + i)))
		op := supervisionOperation(t, string(id), ControlPlanType, p)
		if err := h.control.AddRemoteJob(op); err != nil {
			t.Fatal(err)
		}
		if got := controlAwait(t, h, id, true); got.Status != operation.StatusFailed {
			t.Fatal("accepted invalid authority")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid authority invoked callback")
	}
	for i, action := range []string{"status", "message", "stop"} {
		id := operation.ID(string(rune('x' + i)))
		p := ControlPlan{Action: action, Handle: first, Generation: gen}
		if action == "message" {
			p.Message = "hello"
		}
		op := supervisionOperation(t, string(id), ControlPlanType, p)
		if err := h.control.AddRemoteJob(op); err != nil {
			t.Fatal(err)
		}
		if got := controlAwait(t, h, id, true); got.Status != operation.StatusCompleted {
			t.Fatal("callback failed", got)
		}
	}
	list := supervisionOperation(t, "external-list", ControlPlanType, ControlPlan{Action: "list"})
	if err := h.control.AddRemoteJob(list); err != nil {
		t.Fatal(err)
	}
	listed := controlAwait(t, h, list.ID, true)
	remote, err := operation.DecodeRemoteJobState(listed)
	if err != nil || !strings.Contains(remote.TerminalResult, first) {
		t.Fatal("list omitted external target", err, remote.TerminalResult)
	}
	if calls.Load() != 3 {
		t.Fatal("callback dispatch count", calls.Load())
	}
	if got := h.Snapshot().Agents[0]; got.Generation != 42 || got.Handle != first || got.ChildActivity.Handle != first || got.ScopeID == "" {
		t.Fatal("identity mismatch", got)
	}
}

func TestHubExternalBoundedSnapshot(t *testing.T) {
	h := supervisionHub(t)
	var calls atomic.Int32
	callback := func(context.Context, ControlPlan) (any, error) { calls.Add(1); return nil, nil }
	a := AgentActivity{ChildActivity: ChildActivity{Name: strings.Repeat("x", 1000), Operations: []OperationActivity{{ID: "secret", Executable: "secret"}}, OwnedResources: []string{"/private/path"}}, Purpose: strings.Repeat("p", 1000), ScopeID: "foreign", Phase: "running"}
	handle, _, err := h.BindExternal(a, callback)
	if err != nil {
		t.Fatal(err)
	}
	a.Name = "mutated"
	first := h.Snapshot()
	if len(first.Agents) != 1 || len(first.Agents[0].Name) > activityIdentityBytes || len(first.Agents[0].Purpose) > activityIdentityBytes || first.Agents[0].Operations != nil || first.Agents[0].OwnedResources != nil || first.Agents[0].ScopeID == "foreign" {
		t.Fatal("unbounded/unredacted snapshot", first)
	}
	first.Agents[0].Name = "mutated"
	if h.Snapshot().Agents[0].Name == "mutated" || calls.Load() != 0 {
		t.Fatal("snapshot mutation or callback")
	}
	if err := h.UpdateExternal(handle, AgentActivity{ChildActivity: ChildActivity{Name: "updated"}, Phase: "report_available", ScopeID: "foreign", Handle: "forged"}); err != nil {
		t.Fatal(err)
	}
	if got := h.Snapshot().Agents[0]; got.Name != "updated" || got.Handle != handle || got.ScopeID == "foreign" {
		t.Fatal("update changed authority", got)
	}
	for i := 1; i < MaxFleetAgents; i++ {
		if _, _, err := h.BindExternal(AgentActivity{}, callback); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := h.BindExternal(AgentActivity{}, callback); err == nil {
		t.Fatal("capacity not enforced")
	}
	if got := h.Snapshot(); len(got.Agents) != MaxFleetAgents || got.OmittedAgents != 1 {
		t.Fatal("wrong capacity observation", len(got.Agents), got.OmittedAgents)
	}
}

func TestHubExternalWaitAndDeadRoot(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := NewHub(ctx)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	handle, _, err := h.BindExternal(AgentActivity{Phase: "running"}, func(ctx context.Context, p ControlPlan) (any, error) {
		calls.Add(1)
		close(entered)
		<-release
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	op := supervisionOperation(t, "blocked", ControlPlanType, ControlPlan{Action: "stop", Handle: handle, Generation: 1})
	if err := h.control.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("callback not entered")
	}
	cancel()
	wait, stop := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if err := h.Wait(wait); err == nil {
		t.Fatal("Wait did not join callback")
	}
	stop()
	close(release)
	wait, stop = context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := h.Wait(wait); err != nil {
		t.Fatal(err)
	}
	// Hub shutdown neither manufactures independent child drain nor runs controls.
	if h.Snapshot().Agents[0].Phase != "running" || h.Snapshot().Agents[0].OwnerTerminalEnqueued {
		t.Fatal("claimed child drain")
	}
	h.control.execute(op)
	if calls.Load() != 1 {
		t.Fatal("dead root invoked external callback")
	}
	if _, _, err := h.BindExternal(AgentActivity{}, func(context.Context, ControlPlan) (any, error) { return nil, nil }); err == nil {
		t.Fatal("dead root bound target")
	}
}
