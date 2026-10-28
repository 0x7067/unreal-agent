package reminder_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// Persistence is real localfile append+sync, not an in-memory Save substitute.
// A notification is a canonical tool result, never an external delivery receipt.
func TestM44TimerActualStoreCommitFenceAndRestart(t *testing.T) {
	ctx := t.Context()
	dir := t.TempDir()
	store, e := localfile.New(dir)
	if e != nil {
		t.Fatal(e)
	}
	const sid session.ID = "timer-store"
	if _, e = store.Create(ctx, sid); e != nil {
		t.Fatal(e)
	}
	if e = store.AppendTurn(ctx, sid, session.Turn{ID: "turn", Type: session.TurnRegular}); e != nil {
		t.Fatal(e)
	}
	clock := &reminder.FakeClock{Current: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	due := clock.Now().Add(time.Hour)
	spec, e := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: string(sid), DueTime: due, Ownership: reminder.Ownership{Mode: "background"}, Message: "one canonical notification"})
	if e != nil {
		t.Fatal(e)
	}
	op := operation.Operation{ID: "timer-op", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady, State: spec.State}
	if e = store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "DurableReminder", Arguments: `{}`}}}}}); e != nil {
		t.Fatal(e)
	}
	if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); e != nil {
		t.Fatal(e)
	}
	reopen := func() operation.Operation {
		t.Helper()
		s, e := localfile.New(dir)
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.Resume(ctx, sid)
		if e != nil {
			t.Fatal(e)
		}
		if len(r.Operations) != 1 {
			t.Fatalf("recover operations=%d", len(r.Operations))
		}
		return r.Operations[0]
	}
	next := func(h *reminder.Handler) operation.Operation {
		t.Helper()
		select {
		case op := <-h.RemoteJobUpdates():
			return op
		default:
			t.Fatal("missing update")
			return operation.Operation{}
		}
	}
	empty := func(h *reminder.Handler) {
		t.Helper()
		select {
		case op := <-h.RemoteJobUpdates():
			t.Fatalf("unexpected update %s", op.Status)
		default:
		}
	}
	poll := func(h *reminder.Handler) {
		t.Helper()
		if e := h.Poll(); e != nil {
			t.Fatal(e)
		}
	}
	handler := func(saved operation.Operation) *reminder.Handler {
		t.Helper()
		h := reminder.NewHandler(reminder.Config{Now: clock.Now})
		t.Cleanup(func() {
			_ = h.Close()
			if e := h.AwaitStopped(context.Background()); e != nil {
				t.Error(e)
			}
		})
		if e := h.AddRemoteJob(saved); e != nil {
			t.Fatal(e)
		}
		return h
	}
	save := func(op operation.Operation) {
		t.Helper()
		if e := store.SaveOperation(ctx, sid, op); e != nil {
			t.Fatal(e)
		}
		got := reopen()
		if got.Status != op.Status || !bytes.Equal(got.State, op.State) {
			t.Fatal("saved checkpoint differs from recovered state")
		}
	}
	commit := func(h *reminder.Handler, op operation.Operation) {
		t.Helper()
		save(op)
		if e := h.NotifyCheckpointCommitted(op.ID, reopen().State); e != nil {
			t.Fatal(e)
		}
	}
	h := handler(reopen())
	scheduled := next(h)
	save(scheduled)
	_ = h.Close()
	// Before-deadline restart and backward wall-clock movement preserve absolute due.
	h = handler(reopen())
	restored := next(h)
	_, cp, e := reminder.DecodeTask(restored)
	if e != nil || !cp.Timers[0].Plan.DueTime.Equal(due) {
		t.Fatalf("deadline rebased: %v", e)
	}
	clock.Set(clock.Now().Add(-time.Hour))
	poll(h)
	empty(h)
	clock.Set(due.Add(time.Second))
	poll(h)
	pending := next(h)
	if pending.Status != operation.StatusAwaiting {
		t.Fatal("notification emitted before commit")
	}
	poll(h)
	empty(h)
	// Crash after pending proposal, before Save: disk still contains Scheduled.
	_ = h.Close()
	h = handler(reopen())
	next(h)
	poll(h)
	pending = next(h)
	if e := h.NotifyCheckpointCommitted(pending.ID, scheduled.State); e != nil {
		t.Fatal(e)
	}
	poll(h)
	empty(h)
	// Save pending but crash before acknowledgment: recovery must re-save it.
	save(pending)
	_ = h.Close()
	h = handler(reopen())
	recoveredPending := next(h)
	poll(h)
	empty(h)
	commit(h, recoveredPending)
	poll(h)
	terminal := next(h)
	_, cp, e = reminder.DecodeTask(terminal)
	if e != nil || cp.Timers[0].State != reminder.NotificationPending {
		t.Fatalf("terminal falsely claims external inbox acceptance: %v", e)
	}
	// Crash before terminal commit re-proposes the SAME logical result.
	original, _ := operation.DecodeRemoteJobState(terminal)
	_ = h.Close()
	h = handler(reopen())
	again := next(h)
	commit(h, again)
	poll(h)
	terminal = next(h)
	replayed, _ := operation.DecodeRemoteJobState(terminal)
	if replayed.TerminalResult != original.TerminalResult {
		t.Fatal("logical notification changed across uncommitted terminal crash")
	}
	// Commit terminal without a client/consumer seeing it. Restart is inert.
	save(terminal)
	_ = h.Close()
	h = handler(reopen())
	poll(h)
	empty(h)
	if !reminder.TaskTerminalProof(reopen()) {
		t.Fatal("missing durable terminal proof")
	}
	if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{terminal}}); e != nil {
		t.Fatal(e)
	}
	page, e := store.Items(ctx, sid, 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, item := range page.Items {
		if status, ok := item.Data.(sessionstore.ToolCallStatus); ok {
			for _, op := range status.Operations {
				if op.Status == operation.StatusCompleted {
					n++
					s, _ := operation.DecodeRemoteJobState(op)
					if s.TerminalResult != original.TerminalResult {
						t.Fatal("client history result changed")
					}
				}
			}
		}
	}
	if n != 1 {
		t.Fatalf("canonical completed logical notification count=%d", n)
	}
}

func TestM44TimerActualStoreStableBoundedBatches(t *testing.T) {
	ctx := t.Context()
	store, e := localfile.New(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	const sid session.ID = "timer-batch"
	if _, e = store.Create(ctx, sid); e != nil {
		t.Fatal(e)
	}
	if e = store.AppendTurn(ctx, sid, session.Turn{ID: "turn", Type: session.TurnRegular}); e != nil {
		t.Fatal(e)
	}
	clock := &reminder.FakeClock{Current: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	h := reminder.NewHandler(reminder.Config{Now: clock.Now})
	defer h.Close()
	next := func() operation.Operation {
		t.Helper()
		select {
		case op := <-h.RemoteJobUpdates():
			return op
		default:
			t.Fatal("missing batch update")
			return operation.Operation{}
		}
	}
	empty := func() {
		t.Helper()
		select {
		case op := <-h.RemoteJobUpdates():
			t.Fatalf("batch exceeded16: %s", op.ID)
		default:
		}
	}
	calls := []llm.Item{}
	ops := []operation.Operation{}
	// Reverse admission order; equal deadlines use lexical operation IDs.
	for i := 19; i >= 0; i-- {
		id := fmt.Sprintf("timer-%02d", i)
		spec, e := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: string(sid), DueTime: clock.Now().Add(time.Hour), Ownership: reminder.Ownership{Mode: "background"}, Message: id})
		if e != nil {
			t.Fatal(e)
		}
		op := operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
		ops = append(ops, op)
		calls = append(calls, llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: id, Name: "DurableReminder", Arguments: `{}`}})
	}
	if e = store.AppendModelResponse(ctx, sid, sessionstore.ModelResponse{TurnID: "turn", Response: llm.Response{Stop: llm.StopComplete, Output: calls}}); e != nil {
		t.Fatal(e)
	}
	for _, op := range ops {
		if e = store.AppendToolCallStatus(ctx, sid, sessionstore.ToolCallStatus{TurnID: "turn", CallID: string(op.ID), Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); e != nil {
			t.Fatal(e)
		}
		if e = h.AddRemoteJob(op); e != nil {
			t.Fatal(e)
		}
		scheduled := next()
		if e = store.SaveOperation(ctx, sid, scheduled); e != nil {
			t.Fatal(e)
		}
	}
	clock.Advance(2 * time.Hour)
	pending := []operation.Operation{}
	for _, size := range []int{16, 4} {
		if e = h.Poll(); e != nil {
			t.Fatal(e)
		}
		for i := 0; i < size; i++ {
			op := next()
			want := fmt.Sprintf("timer-%02d", len(pending))
			if string(op.ID) != want || op.Status != operation.StatusAwaiting {
				t.Fatalf("due order/status: %+v want %s", op, want)
			}
			if e = store.SaveOperation(ctx, sid, op); e != nil {
				t.Fatal(e)
			}
			pending = append(pending, op)
		}
		empty()
	}
	for _, op := range pending {
		if e = h.NotifyCheckpointCommitted(op.ID, op.State); e != nil {
			t.Fatal(e)
		}
	}
	completed := 0
	for _, size := range []int{16, 4} {
		if e = h.Poll(); e != nil {
			t.Fatal(e)
		}
		for i := 0; i < size; i++ {
			op := next()
			want := fmt.Sprintf("timer-%02d", completed)
			if string(op.ID) != want || op.Status != operation.StatusCompleted {
				t.Fatalf("completion order/status: %+v want %s", op, want)
			}
			if e = store.SaveOperation(ctx, sid, op); e != nil {
				t.Fatal(e)
			}
			completed++
		}
		empty()
	}
	if completed != 20 {
		t.Fatal("batch work lost")
	}
}
