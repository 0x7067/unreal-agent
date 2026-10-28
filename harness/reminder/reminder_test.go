package reminder

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

var epoch = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

func plan(t *testing.T, id string, o Ownership) Plan {
	t.Helper()
	p, err := NewPlan("parent", id, "reminder", epoch, epoch.Add(time.Minute), o, "wake up")
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func add(t *testing.T, c Checkpoint, p Plan) Checkpoint {
	t.Helper()
	tr, err := Add(c, p)
	if err != nil {
		t.Fatal(err)
	}
	return tr.Checkpoint
}
func restart(t *testing.T, c Checkpoint) Checkpoint {
	t.Helper()
	b, err := Encode(c)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func intents(t *testing.T, tr Transition) []NotificationIntent {
	t.Helper()
	got, err := Deliveries(tr.Checkpoint, tr.RequiredCommit)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func mustAdvance(t *testing.T, c Checkpoint, now time.Time) Transition {
	t.Helper()
	tr, err := Reconcile(c, now)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}
func TestFakeClockRestartAndMovement(t *testing.T) {
	p := plan(t, "one", Ownership{Mode: "background"})
	c := add(t, NewCheckpoint(), p)
	saved, _ := Encode(c)
	clock := &FakeClock{Current: epoch.Add(59 * time.Second)}
	tr, err := Poll(restart(t, c), clock)
	if err != nil {
		t.Fatal(err)
	}
	if len(intents(t, tr)) != 0 {
		t.Fatal("fired before deadline")
	}
	clock.Set(epoch.Add(-time.Hour))
	tr, err = Poll(tr.Checkpoint, clock)
	if err != nil {
		t.Fatal(err)
	}
	if len(intents(t, tr)) != 0 {
		t.Fatal("backward clock fired early")
	}
	clock.Set(epoch.Add(24 * time.Hour))
	tr, err = Poll(restart(t, tr.Checkpoint), clock)
	if err != nil {
		t.Fatal(err)
	}
	got := intents(t, tr)
	if len(got) != 1 || got[0].InputID != LogicalInputID(p.TimerID) {
		t.Fatalf("intents: %+v", got)
	}
	ack, err := Acknowledge(tr.Checkpoint, p.TimerID, got[0].InputID)
	if err != nil {
		t.Fatal(err)
	}
	for _, now := range []time.Time{epoch, epoch.Add(48 * time.Hour)} {
		tr = mustAdvance(t, restart(t, ack.Checkpoint), now)
		if len(intents(t, tr)) != 0 {
			t.Fatal("accepted notification fired again")
		}
	}
	after, _ := Encode(c)
	if !bytes.Equal(saved, after) {
		t.Fatal("mutated saved input")
	}
}
func TestNotificationCommitAndUnconfirmedClientDeliveryBoundary(t *testing.T) {
	c := add(t, NewCheckpoint(), plan(t, "boundary", Ownership{Mode: "background"}))
	// This is a pure transition trace, not a claim about a real store/process/UI.
	pending := mustAdvance(t, c, epoch.Add(time.Minute))
	if pending.Checkpoint.Timers[0].State != NotificationPending {
		t.Fatal("must stage pending before enqueue")
	}
	if _, err := Deliveries(pending.Checkpoint, CommitReceipt{}); err == nil {
		t.Fatal("unconfirmed checkpoint permitted enqueue")
	}
	old := mustAdvance(t, c, epoch)
	if _, err := Deliveries(pending.Checkpoint, old.RequiredCommit); err == nil {
		t.Fatal("stale checkpoint receipt accepted")
	}
	// Simulate the persisted checkpoint at notification commit. No receipt of inbox
	// acceptance or client presentation exists; restore must redeliver the same ID.
	committed := restart(t, pending.Checkpoint)
	first := intents(t, pending)
	restored := mustAdvance(t, committed, epoch.Add(2*time.Minute))
	second := intents(t, restored)
	if len(first) != 1 || !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable redelivery: %+v / %+v", first, second)
	}
	if restored.Checkpoint.Timers[0].State != NotificationPending {
		t.Fatal("inferred client delivery from commit")
	}
	if _, err := Acknowledge(committed, first[0].TimerID, "wrong"); err == nil {
		t.Fatal("accepted wrong logical input")
	}
	accepted, err := Acknowledge(committed, first[0].TimerID, first[0].InputID)
	if err != nil {
		t.Fatal(err)
	}
	if got := intents(t, mustAdvance(t, restart(t, accepted.Checkpoint), epoch.Add(time.Hour))); len(got) != 0 {
		t.Fatal("inbox accepted notification redelivered")
	}
}
func TestBoundedOverdueBatchesAcrossRestarts(t *testing.T) {
	c := NewCheckpoint()
	for i := 0; i < MaxTimers; i++ {
		c = add(t, c, plan(t, fmt.Sprint(i), Ownership{Mode: "background"}))
	}
	if _, err := Add(c, plan(t, "overflow", Ownership{Mode: "background"})); err == nil {
		t.Fatal("capacity not enforced")
	}
	all := map[string]bool{}
	for batch := 0; batch < MaxTimers/MaxOverdueBatch; batch++ {
		tr := mustAdvance(t, restart(t, c), epoch.Add(time.Hour))
		got := intents(t, tr)
		if len(got) != MaxOverdueBatch {
			t.Fatalf("batch %d size %d", batch, len(got))
		}
		repeated := mustAdvance(t, restart(t, tr.Checkpoint), epoch.Add(2*time.Hour))
		if !reflect.DeepEqual(got, intents(t, repeated)) {
			t.Fatal("restart changed pending batch or grew beyond bound")
		}
		c = repeated.Checkpoint
		for _, intent := range got {
			if all[intent.InputID] {
				t.Fatal("duplicate acknowledged notification")
			}
			all[intent.InputID] = true
			ack, err := Acknowledge(c, intent.TimerID, intent.InputID)
			if err != nil {
				t.Fatal(err)
			}
			c = ack.Checkpoint
		}
	}
	if len(all) != MaxTimers || len(intents(t, mustAdvance(t, restart(t, c), epoch.Add(3*time.Hour)))) != 0 {
		t.Fatal("did not drain bounded batches exactly once after acknowledgment")
	}
}
func TestForegroundAbortAndExplicitCancellation(t *testing.T) {
	c := NewCheckpoint()
	a := plan(t, "a", Ownership{Mode: "foreground", OwnerTurnID: "turn-a"})
	b := plan(t, "b", Ownership{Mode: "foreground", OwnerTurnID: "turn-b"})
	bg := plan(t, "bg", Ownership{Mode: "background"})
	for _, p := range []Plan{a, b, bg} {
		c = add(t, c, p)
	}
	canceled, err := CancelForeground(c, "turn-a", "ordinary foreground abort")
	if err != nil {
		t.Fatal(err)
	}
	if c.Timers[0].State != Scheduled {
		t.Fatal("cancel mutated prior checkpoint")
	}
	restored := mustAdvance(t, restart(t, canceled.Checkpoint), epoch.Add(time.Hour))
	got := intents(t, restored)
	if len(got) != 2 {
		t.Fatalf("foreground separation: %+v", got)
	}
	for _, i := range got {
		if i.TimerID == a.TimerID {
			t.Fatal("canceled foreground fired")
		}
	}
	canceled, err = Cancel(restored.Checkpoint, bg.TimerID, "explicit background cancel")
	if err != nil {
		t.Fatal(err)
	}
	got = intents(t, mustAdvance(t, restart(t, canceled.Checkpoint), epoch.Add(2*time.Hour)))
	if len(got) != 1 || got[0].TimerID != b.TimerID {
		t.Fatal("explicit cancel did not suppress pending retry")
	}
	scheduled := add(t, NewCheckpoint(), bg)
	canceled, err = Cancel(scheduled, bg.TimerID, "user canceled")
	if err != nil {
		t.Fatal(err)
	}
	if len(intents(t, mustAdvance(t, restart(t, canceled.Checkpoint), epoch.Add(time.Hour)))) != 0 {
		t.Fatal("cancel then restart fired")
	}
}
func TestStrictVersionsIdentityOwnershipAndDuePolicy(t *testing.T) {
	p := plan(t, "strict", Ownership{Mode: "background"})
	raw, err := EncodePlan(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePlan(append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"unexpected":true}`)...)); err == nil {
		t.Fatal("unknown plan field accepted")
	}
	nested := bytes.Replace(raw, []byte(`"mode":"background"`), []byte(`"mode":"background","extra":true`), 1)
	if _, err := DecodePlan(nested); err == nil {
		t.Fatal("unknown nested field accepted")
	}
	duplicate := bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
	if _, err := DecodePlan(duplicate); err == nil {
		t.Fatal("duplicate field accepted")
	}
	p.Version++
	if _, err := EncodePlan(p); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("version fence: %v", err)
	}
	p.Version = PlanVersion
	cases := []func(*Plan){func(p *Plan) { p.TimerID = "forged" }, func(p *Plan) { p.DueTime = p.CreatedAt.Add(MaxDelay + time.Nanosecond) }, func(p *Plan) { p.DueTime = p.CreatedAt.Add(-time.Nanosecond) }, func(p *Plan) { p.DueTime = p.DueTime.In(time.FixedZone("nonUTC", 3600)) }, func(p *Plan) { p.Ownership = Ownership{Mode: "foreground"} }, func(p *Plan) { p.Ownership.OwnerTurnID = "forbidden" }, func(p *Plan) { p.Kind = "unknown" }}
	for i, change := range cases {
		q := p
		change(&q)
		if err := q.Validate(); err == nil {
			t.Fatalf("invalid case %d", i)
		}
	}
	c := add(t, NewCheckpoint(), p)
	raw, _ = Encode(c)
	bad := append(append([]byte{}, raw[:len(raw)-1]...), []byte(`,"unknown":true}`)...)
	if _, err := Decode(bad); err == nil {
		t.Fatal("unknown checkpoint field accepted")
	}
	c.Version++
	if _, err := Encode(c); !errors.Is(err, ErrUnsupported) {
		t.Fatal("checkpoint version fence missing")
	}
	c.Version = CheckpointVersion
	c.Timers[0].State = "future"
	if _, err := Encode(c); err == nil {
		t.Fatal("future timer state accepted")
	}
}
func TestIdentityIdempotenceAndCopying(t *testing.T) {
	p := plan(t, "same", Ownership{Mode: "background"})
	c := add(t, NewCheckpoint(), p)
	tr, err := Add(c, p)
	if err != nil || len(tr.Checkpoint.Timers) != 1 {
		t.Fatal("idempotent add failed")
	}
	p.Message = "different"
	if _, err := Add(c, p); err == nil {
		t.Fatal("changed plan reused ID")
	}
	tr.Checkpoint.Timers[0].Plan.Message = "mutated output"
	if c.Timers[0].Plan.Message != "wake up" {
		t.Fatal("aliased saved input")
	}
	if StableTimerID("ab", "c") == StableTimerID("a", "bc") {
		t.Fatal("ambiguous identity binding")
	}
}
