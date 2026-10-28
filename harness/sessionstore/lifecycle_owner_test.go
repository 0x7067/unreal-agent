package sessionstore

import (
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func ownerHistory(t *testing.T, items ...Item) LifecycleHistory {
	t.Helper()
	h, err := ReadLifecycle(t.Context(), lifecycleItems{items: items}, "owner-session")
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func ownerItem(seq Sequence, id operation.ID, status operation.Status) Item {
	return Item{Sequence: seq, Kind: ItemToolCallStatus, Data: ToolCallStatus{Operations: []operation.Operation{{ID: id, Type: operation.TypeRemoteJob, Status: status}}}}
}
func requireOwnerFence(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrLifecycleRecoveryRequired) {
		t.Fatalf("want lifecycle recovery fence, got %v", err)
	}
}

func TestLifecycleOwnerClassificationRequiresEveryRetainedIdentity(t *testing.T) {
	h := ownerHistory(t, ownerItem(1, "live", operation.StatusAwaiting), ownerItem(2, "terminal", operation.StatusCompleted))
	if len(h.RemoteOperations) != 2 || len(h.RetainedRemoteOperations) != 2 || h.LastRemoteSequence != 2 || h.UnclassifiedRemoteSequence != 0 {
		t.Fatalf("lost canonical ownership: %+v", h)
	}
	requireOwnerFence(t, h.ValidateResume())
	for _, classified := range []map[operation.ID]bool{nil, {}, {"live": true}, {"terminal": true}, {"live": true, "terminal": false}, {"live": true, "unrelated": true}} {
		requireOwnerFence(t, h.ValidateResumeWithOwners(classified))
	}
	for _, classified := range []map[operation.ID]bool{{"live": true, "terminal": true}, {"live": true, "terminal": true, "unrelated": true}} {
		if err := h.ValidateResumeWithOwners(classified); err != nil {
			t.Fatal(err)
		}
	}
	// Classification grants a narrow permission; it cannot mutate the default fence.
	requireOwnerFence(t, h.ValidateResume())
}

func TestLifecycleOwnerInvokeToolRemainsUnclassified(t *testing.T) {
	invoke := lifecycleControl(t, "invoke", inbox.InvokeTool, inbox.ToolInvocation{Call: llm.ToolCall{CallID: "call", Name: "AgentControl", Arguments: `{"action":"list"}`}})
	h := ownerHistory(t, Item{Sequence: 1, Kind: ItemInput, Data: invoke}, ownerItem(2, "child", operation.StatusAwaiting))
	if h.UnclassifiedRemoteSequence != 1 || h.LastRemoteSequence != 2 {
		t.Fatalf("invoke sequence lost: %+v", h)
	}
	requireOwnerFence(t, h.ValidateResumeWithOwners(map[operation.ID]bool{"child": true, "call": true}))
	// Even a caller-supplied identity cannot classify a control without an owner.
	h = ownerHistory(t, Item{Sequence: 1, Kind: ItemInput, Data: invoke})
	requireOwnerFence(t, h.ValidateResumeWithOwners(map[operation.ID]bool{"call": true}))
}

func TestLifecycleOwnersDoNotOverridePendingLifecycle(t *testing.T) {
	cases := []struct {
		name   string
		inputs []inbox.Input
	}{
		{"pending abort", []inbox.Input{lifecycleControl(t, "abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, TargetTurnID: "turn", Scope: "host/1"})}},
		{"uncertain abort", []inbox.Input{lifecycleControl(t, "abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, TargetTurnID: "turn", Scope: "host/1"}), lifecycleControl(t, "result", inbox.ForegroundAbortFinished, inbox.ForegroundAbortResult{Version: 1, InputID: "abort", TargetTurnID: "turn", Status: "uncertain"})}},
		{"pending shutdown", []inbox.Input{lifecycleControl(t, "shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/1"})}},
		{"uncertain shutdown", []inbox.Input{lifecycleControl(t, "shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/1"}), lifecycleControl(t, "result", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/1", Phase: "uncertain"})}},
		{"work drained shutdown", []inbox.Input{lifecycleControl(t, "shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/1"}), lifecycleControl(t, "result", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/1", Phase: "work_drained"})}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			items := []Item{ownerItem(1, "child", operation.StatusCompleted)}
			for i, input := range tc.inputs {
				items = append(items, Item{Sequence: Sequence(i + 2), Kind: ItemInput, Data: input})
			}
			h := ownerHistory(t, items...)
			requireOwnerFence(t, h.ValidateResumeWithOwners(map[operation.ID]bool{"child": true}))
		})
	}
}

func TestLifecycleOwnerDrainAndForkResetOnlyTheirScope(t *testing.T) {
	invoke := lifecycleControl(t, "invoke", inbox.InvokeTool, inbox.ToolInvocation{Call: llm.ToolCall{CallID: "call", Name: "AgentControl", Arguments: `{"action":"list"}`}})
	receipt := lifecycleControl(t, "drain", inbox.ExecutionDrained, inbox.ExecutionDrainReceipt{Version: 1, Scope: "host/1"})
	items := []Item{ownerItem(1, "old", operation.StatusCompleted), {Sequence: 2, Kind: ItemInput, Data: invoke}, {Sequence: 3, Kind: ItemInput, Data: receipt}}
	h := ownerHistory(t, items...)
	if len(h.RetainedRemoteOperations) != 0 || len(h.RemoteOperations) != 1 || h.RemoteOperations["old"].ID != "old" || h.LastDrainSequence != 3 {
		t.Fatalf("drain erased history or retained ownership: %+v", h)
	}
	if err := h.ValidateResumeWithOwners(nil); err != nil {
		t.Fatal(err)
	}
	if err := h.ValidateResume(); err != nil {
		t.Fatal(err)
	}
	items = append(items, ownerItem(4, "new", operation.StatusAwaiting))
	h = ownerHistory(t, items...)
	if len(h.RemoteOperations) != 2 || len(h.RetainedRemoteOperations) != 1 {
		t.Fatalf("wrong drain boundary: %+v", h)
	}
	requireOwnerFence(t, h.ValidateResumeWithOwners(map[operation.ID]bool{"old": true}))
	if err := h.ValidateResumeWithOwners(map[operation.ID]bool{"new": true}); err != nil {
		t.Fatal(err)
	}
	items = append(items, Item{Sequence: 5, Kind: ItemFork, Data: Fork{ParentID: "parent"}})
	h = ownerHistory(t, items...)
	if len(h.RemoteOperations) != 0 || len(h.RetainedRemoteOperations) != 0 || h.UnclassifiedRemoteSequence != 0 || h.LastRemoteSequence != 0 || h.LastDrainSequence != 0 {
		t.Fatalf("fork inherited owner authority: %+v", h)
	}
	if err := h.ValidateResumeWithOwners(nil); err != nil {
		t.Fatal(err)
	}
	// A new owner after the fork must be fenced independently.
	h = ownerHistory(t, append(items, ownerItem(6, "fork-child", operation.StatusAwaiting))...)
	requireOwnerFence(t, h.ValidateResumeWithOwners(map[operation.ID]bool{"new": true}))
}
