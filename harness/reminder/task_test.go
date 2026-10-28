package reminder

import (
	"context"
	"fmt"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"testing"
	"time"
)

func timerTask(t *testing.T, id string, now time.Time, background bool) operation.Operation {
	t.Helper()
	owner := Ownership{Mode: "foreground", OwnerTurnID: "turn-1"}
	if background {
		owner = Ownership{Mode: "background"}
	}
	spec, e := NewTaskSpec(TaskPlan{Version: 1, ParentID: "session-1", DueTime: now.Add(time.Hour), Ownership: owner, Message: "remember"})
	if e != nil {
		t.Fatal(e)
	}
	return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength, State: spec.State}
}
func timerNext(t *testing.T, h *Handler) operation.Operation {
	t.Helper()
	select {
	case op := <-h.RemoteJobUpdates():
		return op
	default:
		t.Fatal("missing synchronous update")
		return operation.Operation{}
	}
}
func timerEmpty(t *testing.T, h *Handler) {
	t.Helper()
	select {
	case op := <-h.RemoteJobUpdates():
		t.Fatalf("unexpected update %v", op.Status)
	default:
	}
}
func TestTaskCommitFenceAndRecovery(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &FakeClock{Current: now}
	h := NewHandler(Config{Now: clock.Now})
	defer h.Close()
	op := timerTask(t, "op-1", now, false)
	if e := h.AddRemoteJob(op); e != nil {
		t.Fatal(e)
	}
	scheduled := timerNext(t, h)
	clock.Advance(2 * time.Hour)
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	pending := timerNext(t, h)
	if pending.Status != operation.StatusAwaiting {
		t.Fatal("notification before checkpoint")
	}
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	timerEmpty(t, h)
	if e := h.NotifyCheckpointCommitted(op.ID, scheduled.State); e != nil {
		t.Fatal(e)
	}
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	timerEmpty(t, h)
	if e := h.NotifyCheckpointCommitted(op.ID, pending.State); e != nil {
		t.Fatal(e)
	}
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	terminal := timerNext(t, h)
	if !TaskTerminalProof(terminal) {
		t.Fatal("invalid terminal proof")
	}
	for _, saved := range []operation.Operation{scheduled, pending, terminal} {
		restart := NewHandler(Config{Now: clock.Now})
		if e := restart.AddRemoteJob(saved); e != nil {
			t.Fatal(e)
		}
		if saved.Status == operation.StatusCompleted {
			timerEmpty(t, restart)
		} else {
			restored := timerNext(t, restart)
			_, cp, e := DecodeTask(restored)
			if e != nil {
				t.Fatal(e)
			}
			if cp.Timers[0].State == Scheduled {
				if e := restart.Poll(); e != nil {
					t.Fatal(e)
				}
				restored = timerNext(t, restart)
			}
			if e := restart.NotifyCheckpointCommitted(restored.ID, restored.State); e != nil {
				t.Fatal(e)
			}
			if e := restart.Poll(); e != nil {
				t.Fatal(e)
			}
			got := timerNext(t, restart)
			a, _ := operation.DecodeRemoteJobState(terminal)
			b, _ := operation.DecodeRemoteJobState(got)
			if a.TerminalResult != b.TerminalResult {
				t.Fatal("restart changed logical notification")
			}
		}
		_ = restart.Close()
		if e := restart.AwaitStopped(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
}
func TestTaskClockBatchCancelAndStrictRecovery(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := &FakeClock{Current: now}
	h := NewHandler(Config{Now: clock.Now})
	defer h.Close()
	for i := 0; i < 20; i++ {
		op := timerTask(t, fmt.Sprint(i), now, true)
		if e := h.AddRemoteJob(op); e != nil {
			t.Fatal(e)
		}
		scheduled := timerNext(t, h)
		if i == 0 {
			bad := cloneTask(scheduled)
			bad.State = append(bad.State[:len(bad.State)-1], []byte(`,"unexpected":1}`)...)
			if ValidateTaskRecovery(bad) == nil {
				t.Fatal("unknown field accepted")
			}
		}
	}
	clock.Set(now.Add(-time.Hour))
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	timerEmpty(t, h)
	if e := h.CancelRemoteJob("0", "user canceled"); e != nil {
		t.Fatal(e)
	}
	canceled := timerNext(t, h)
	if canceled.Status != operation.StatusCanceled || !TaskTerminalProof(canceled) {
		t.Fatal("cancel not acknowledged")
	}
	clock.Set(now.Add(2 * time.Hour))
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 16; i++ {
		if timerNext(t, h).Status != operation.StatusAwaiting {
			t.Fatal("premature notification")
		}
	}
	timerEmpty(t, h)
	if e := h.Poll(); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		timerNext(t, h)
	}
	timerEmpty(t, h)
}
