package sessionstore

import (
	"context"
	"encoding/json/v2"
	"errors"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"

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

// lifecycleItemsFunc deliberately implements only Items, not writable Store.
type lifecycleItemsFunc func(context.Context, session.ID, Sequence, int) (Page, error)

func (f lifecycleItemsFunc) Items(ctx context.Context, id session.ID, after Sequence, limit int) (Page, error) {
	return f(ctx, id, after, limit)
}

func TestReadLifecycleItemsOnlyOrderedPagesAndForkReset(t *testing.T) {
	ctx := t.Context()
	pages := []Page{
		{Items: []Item{
			ownerItem(1, "parent-owner", operation.StatusCompleted),
			{Sequence: 2, Kind: ItemInput, Data: lifecycleControl(t, "parent-abort", inbox.AbortForeground, inbox.ForegroundAbort{Version: 1, TargetTurnID: "turn", Scope: "host/1"})},
		}, NextAfter: 2, More: true},
		{Items: []Item{
			{Sequence: 3, Kind: ItemFork, Data: Fork{ParentID: "parent"}},
			{Sequence: 4, Kind: ItemInput, Data: lifecycleControl(t, "shutdown", inbox.ShutdownSession, inbox.ShutdownSessionRequest{Version: 1, Scope: "host/2"})},
		}, NextAfter: 4, More: true},
		{Items: []Item{
			{Sequence: 5, Kind: ItemInput, Data: lifecycleControl(t, "result", inbox.SessionShutdownFinished, inbox.SessionShutdownResult{Version: 1, InputID: "shutdown", Scope: "host/2", Phase: "completed"})},
		}, NextAfter: 5},
	}
	calls := 0
	source := lifecycleItemsFunc(func(gotCtx context.Context, id session.ID, after Sequence, limit int) (Page, error) {
		if calls >= len(pages) {
			t.Fatal("read beyond final page")
		}
		wantAfter := BeforeFirst
		if calls > 0 {
			wantAfter = pages[calls-1].NextAfter
		}
		if gotCtx != ctx || id != "child" || after != wantAfter || limit != 512 {
			t.Fatalf("unexpected Items arguments: id=%s after=%d limit=%d", id, after, limit)
		}
		page := pages[calls]
		calls++
		return page, nil
	})
	h, err := ReadLifecycle(ctx, source, "child")
	if err != nil {
		t.Fatal(err)
	}
	if calls != len(pages) {
		t.Fatalf("read %d pages, want %d", calls, len(pages))
	}
	if len(h.RemoteOperations) != 0 || len(h.RetainedRemoteOperations) != 0 || len(h.Aborts) != 0 || h.LastRemoteSequence != 0 || h.LastDrainSequence != 0 || h.UnclassifiedRemoteSequence != 0 {
		t.Fatalf("fork inherited parent lifecycle history: %+v", h)
	}
	if len(h.Shutdowns) != 1 || h.ShutdownResults["shutdown"].Phase != "completed" {
		t.Fatalf("lost ordered child intent/outcome across pages: %+v", h)
	}
	if err := h.ValidateResume(); err != nil {
		t.Fatal(err)
	}
}

func TestReadLifecycleItemsOnlyErrors(t *testing.T) {
	readErr := errors.New("history read failed")
	tests := []struct {
		name string
		page Page
		err  error
		want string
	}{
		{name: "read failure", err: readErr, want: readErr.Error()},
		{name: "nonadvancing cursor", page: Page{NextAfter: 1, More: true}, want: "non-advancing lifecycle history page"},
		{name: "regressing cursor", page: Page{NextAfter: 0, More: true}, want: "non-advancing lifecycle history page"},
		{name: "invalid input", page: Page{Items: []Item{{Sequence: 2, Kind: ItemInput, Data: "not an input"}}}, want: "invalid typed canonical input"},
		{name: "invalid tool status", page: Page{Items: []Item{{Sequence: 2, Kind: ItemToolCallStatus, Data: "not a tool status"}}}, want: "invalid typed canonical tool status"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			source := lifecycleItemsFunc(func(_ context.Context, _ session.ID, after Sequence, _ int) (Page, error) {
				calls++
				if calls == 1 {
					return Page{Items: []Item{ownerItem(1, "owner", operation.StatusAwaiting)}, NextAfter: 1, More: true}, nil
				}
				if calls != 2 || after != 1 {
					t.Fatalf("continued after error or lost cursor: calls=%d after=%d", calls, after)
				}
				return tc.page, tc.err
			})
			h, err := ReadLifecycle(t.Context(), source, "s")
			if err == nil || err.Error() != tc.want {
				t.Fatalf("got error %v, want %q", err, tc.want)
			}
			if tc.err != nil && !errors.Is(err, tc.err) {
				t.Fatalf("read error identity lost: %v", err)
			}
			if h.RemoteOperations["owner"].ID != "owner" {
				t.Fatal("lost history from successful page before error")
			}
		})
	}
}
