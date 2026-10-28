package inbox

// Exhaustive model check of the inbox actor (local.go run loop).
//
// The actor is a single goroutine whose only nondeterminism is which ready
// channel case the environment offers next. A test that performs one channel
// operation at a time and waits for it to complete therefore explores exactly
// the interleavings the real scheduler could produce: each operation is one
// indivisible rendezvous with the actor, and no two operations are ever
// outstanding at once. This file drives the REAL Inbox through its real
// channels that way and checks, at every step, the promises the inbox exists
// to keep:
//
//  1. At-most-once admission: an input ID is delivered on Output or
//     transferred by Freeze at most once over the actor's lifetime, across
//     redeliveries, Freeze/Thaw cycles, and seen IDs restored after a crash
//     (New's seenIDs argument).
//  2. FIFO: Output delivers admissions in admission order, and Freeze
//     transfers the remaining FIFO in the same order.
//  3. No loss on freeze: a committed Freeze hands over exactly the inputs
//     admitted so far and not yet delivered.
//  4. Frozen blocks admission: while frozen, a submission is not received
//     (checked separately in TestInboxFrozenBlocksAdmission with a live
//     blocked send, because the explorer never offers what must block).
//
// Exploration is breadth-first over actor states (frozen, seen set, queue),
// each expanded once from a shortest event sequence, so no state is ever
// pruned by a truncated subtree. At every state the explorer keeps an oracle
// mirror derived only from the events it issued; any disagreement with what
// the actor actually delivers or transfers — including divergence that would
// otherwise hang — fails with the full event trace. Duplicate redelivery is
// verified at every state by a probe (redeliver a seen ID, freeze, compare),
// and cancellation is verified at every state on a fresh actor.
//
// Bounds (a pass covers these, not all imaginable workloads):
//   - ID alphabet a-d, initial seen IDs nil / {a} / {a,b} (crash recovery);
//   - actor states whose shortest event sequence is at most mcMaxDepth long
//     (with alphabet a-d every reachable state is, with room to spare);
//   - distinct states capped at mcMaxStates;
//   - inputs are external-kind inputs with no payload; control-kind inputs
//     take the identical admission path (kind is not consulted);
//   - Submit-side prelude (validation, payload cloning, caller-context
//     checks) is upstream of the actor and not exercised here;
//   - cancellation drops whatever is still queued (documented volatile-inbox
//     semantics); the check asserts the Output channel closes, not delivery.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

const (
	mcWatchdog        = 2 * time.Second
	mcMaxDepth        = 12
	mcMaxStates       = 50_000
	mcFrozenBlockWait = 150 * time.Millisecond
)

var mcAlphabet = []ID{"a", "b", "c", "d"}

type mcEvent struct {
	kind string // admit | deliver | freeze | thaw | cancel
	id   ID
}

func (e mcEvent) String() string {
	if e.id == "" {
		return e.kind
	}
	return fmt.Sprintf("%s(%s)", e.kind, e.id)
}

type mcOracle struct {
	seen   map[ID]struct{}
	queue  []ID
	frozen bool
	counts map[ID]int // admissions observed on Output or via Freeze
	cancel context.CancelFunc
}

func mcNewOracle(seed []ID) *mcOracle {
	o := &mcOracle{seen: map[ID]struct{}{}, counts: map[ID]int{}}
	for _, id := range seed {
		o.seen[id] = struct{}{}
	}
	return o
}

func (o *mcOracle) key() string {
	ids := make([]string, 0, len(o.seen))
	for id := range o.seen {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	queue := make([]string, len(o.queue))
	for i, id := range o.queue {
		queue[i] = string(id)
	}
	return fmt.Sprintf("frozen=%t;seen=%s;queue=%s", o.frozen, strings.Join(ids, ","), strings.Join(queue, ","))
}

func (o *mcOracle) record(id ID) error {
	o.counts[id]++
	if o.counts[id] > 1 {
		return fmt.Errorf("at-most-once admission broken: input %s admitted twice (delivered or transferred more than once)", id)
	}
	return nil
}

// mcApply performs one event against the live actor and updates the oracle.
// Every observable must match the oracle exactly or the divergence is
// returned with the trace.
func mcApply(inputs *Inbox, ev mcEvent, o *mcOracle, trace []mcEvent) error {
	where := fmt.Sprintf("after %v", trace)
	switch ev.kind {
	case "admit":
		if o.frozen {
			return fmt.Errorf("internal: admission offered while oracle says frozen %s", where)
		}
		request := submission{input: Input{ID: ev.id, Kind: InputExternal}, admitted: make(chan struct{})}
		select {
		case inputs.submissions <- request:
			select {
			case <-request.admitted:
			case <-time.After(mcWatchdog):
				return fmt.Errorf("missing definitive admission acknowledgment %s", where)
			}
			if _, dup := o.seen[ev.id]; !dup {
				o.seen[ev.id] = struct{}{}
				o.queue = append(o.queue, ev.id)
			}
			if inputs.HasPending() != (len(o.queue) != 0) {
				return fmt.Errorf("queued ownership disagrees with admitted FIFO %s", where)
			}
			return nil
		case <-time.After(mcWatchdog):
			return fmt.Errorf("send of %s was not received within %v; admission should be enabled %s", ev.id, mcWatchdog, where)
		}
	case "deliver":
		if o.frozen || len(o.queue) == 0 {
			return fmt.Errorf("internal: deliver offered but oracle expects none available %s", where)
		}
		select {
		case got := <-inputs.output:
			if got.ID != o.queue[0] || got.Kind != InputExternal {
				return fmt.Errorf("FIFO broken: delivered %q (kind %q), expected %q %s", got.ID, got.Kind, o.queue[0], where)
			}
			if err := o.record(got.ID); err != nil {
				return fmt.Errorf("%v %s", err, where)
			}
			o.queue = o.queue[1:]
			return nil
		case <-time.After(mcWatchdog):
			return fmt.Errorf("expected delivery of %s, actor blocked (dedup/FIFO divergence?) %s", o.queue[0], where)
		}
	case "freeze":
		res, err := mcChangeAdmission(inputs, true)
		if err != nil {
			return fmt.Errorf("freeze failed: %v %s", err, where)
		}
		got := make([]string, len(res.inputs))
		for i, in := range res.inputs {
			got[i] = string(in.ID)
		}
		want := make([]string, len(o.queue))
		for i, id := range o.queue {
			want[i] = string(id)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			return fmt.Errorf("freeze transferred [%s], expected the remaining FIFO [%s] %s", strings.Join(got, ","), strings.Join(want, ","), where)
		}
		for _, in := range res.inputs {
			if in.Kind != InputExternal {
				return fmt.Errorf("freeze transferred input %q with kind %q, expected external %s", in.ID, in.Kind, where)
			}
			if err := o.record(in.ID); err != nil {
				return fmt.Errorf("%v %s", err, where)
			}
		}
		o.queue = nil
		o.frozen = true
		return nil
	case "thaw":
		res, err := mcChangeAdmission(inputs, false)
		if err != nil {
			return fmt.Errorf("thaw failed: %v %s", err, where)
		}
		if len(res.inputs) != 0 {
			return fmt.Errorf("thaw transferred inputs %v, expected none %s", res.inputs, where)
		}
		o.frozen = false
		return nil
	case "cancel":
		o.cancel()
		// Cancellation races a pending delivery: the actor's select may send on
		// Output or take ctx.Done while both are ready, so drain whatever is
		// still delivered (still subject to at-most-once) until Output closes.
		deadline := time.After(mcWatchdog)
		for {
			select {
			case got, ok := <-inputs.output:
				if !ok {
					return nil
				}
				if err := o.record(got.ID); err != nil {
					return fmt.Errorf("%v %s", err, where)
				}
			case <-deadline:
				return fmt.Errorf("output channel did not close after cancellation %s", where)
			}
		}
	}
	return fmt.Errorf("internal: unknown event %s", ev.kind)
}

// mcChangeAdmission issues a Freeze (freeze=true) or Thaw (freeze=false)
// request exactly like the production changeAdmission does, one operation at
// a time.
func mcChangeAdmission(inputs *Inbox, freeze bool) (admissionResult, error) {
	request := admissionRequest{ctx: context.Background(), freeze: freeze, result: make(chan admissionResult, 1)}
	select {
	case inputs.controls <- request:
	case <-time.After(mcWatchdog):
		return admissionResult{}, fmt.Errorf("control request was not received within %v", mcWatchdog)
	}
	select {
	case result := <-request.result:
		return result, result.err
	case <-time.After(mcWatchdog):
		return admissionResult{}, fmt.Errorf("control result was not sent within %v", mcWatchdog)
	}
}

// mcAdvance is the pure oracle transition for one event; it only computes
// state keys and enablement, never touching an actor.
func mcAdvance(o *mcOracle, ev mcEvent) *mcOracle {
	n := &mcOracle{seen: map[ID]struct{}{}, frozen: o.frozen}
	for id := range o.seen {
		n.seen[id] = struct{}{}
	}
	n.queue = append([]ID(nil), o.queue...)
	switch ev.kind {
	case "admit":
		if _, dup := n.seen[ev.id]; !dup {
			n.seen[ev.id] = struct{}{}
			n.queue = append(n.queue, ev.id)
		}
	case "deliver":
		n.queue = n.queue[1:]
	case "freeze":
		n.queue = nil
		n.frozen = true
	case "thaw":
		n.frozen = false
	}
	return n
}

// mcProbeDup verifies, against the live actor at this state, that
// redelivering an already-seen input ID is dropped: the duplicate send must
// be received, change nothing, and a following freeze must hand over exactly
// the FIFO that was there before the redelivery. A dedup regression (input
// recorded wrongly, or seen state lost across Freeze/Thaw) makes the transfer
// differ from the oracle queue and fails here.
func mcProbeDup(inputs *Inbox, o *mcOracle, trace []mcEvent) error {
	if o.frozen || len(o.seen) == 0 {
		return nil
	}
	var id ID
	for seen := range o.seen {
		id = seen
		break
	}
	if err := mcApply(inputs, mcEvent{kind: "admit", id: id}, o, trace); err != nil {
		return fmt.Errorf("duplicate redelivery of %s: %v", id, err)
	}
	if err := mcApply(inputs, mcEvent{kind: "freeze"}, o, trace); err != nil {
		return fmt.Errorf("freeze after redelivering %s: %v", id, err)
	}
	return mcApply(inputs, mcEvent{kind: "thaw"}, o, trace)
}

// mcRun explores actor states breadth-first, each expanded once from a
// shortest event sequence. Every dequeued state is re-driven against a fresh
// real Inbox: the whole prefix replays with assertions on every observable,
// the remaining FIFO is checked by an epilogue freeze, and a duplicate
// redelivery probe checks dedup at that state. A state whose shortest
// sequence exceeds mcMaxDepth is out of scope (a bound, not a claim), and a
// state is never pruned by a truncated subtree because expansion order
// guarantees minimal prefixes.
func mcRun(seed []ID, visited map[string]bool) error {
	type node struct {
		prefix []mcEvent
		o      *mcOracle
	}
	root := mcNewOracle(seed)
	queue := []node{{nil, root}}
	visited[root.key()] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		ctx, cancel := context.WithCancel(context.Background())
		inputs, err := New(ctx, seed)
		if err != nil {
			cancel()
			return fmt.Errorf("seed %v rejected: %v", seed, err)
		}
		replay := mcNewOracle(seed)
		replay.cancel = cancel
		for i, ev := range cur.prefix {
			if err := mcApply(inputs, ev, replay, cur.prefix[:i]); err != nil {
				cancel()
				return err
			}
		}
		if replay.key() != cur.o.key() {
			cancel()
			return fmt.Errorf("internal: oracle transition mismatch on %v", cur.prefix)
		}
		// Epilogue: a committed freeze must hand over exactly the remaining
		// FIFO, then a duplicate redelivery must change nothing.
		if n := len(cur.prefix); n == 0 || cur.prefix[n-1].kind != "cancel" {
			if err := mcApply(inputs, mcEvent{kind: "freeze"}, replay, cur.prefix); err != nil {
				cancel()
				return fmt.Errorf("epilogue: %v", err)
			}
			if err := mcApply(inputs, mcEvent{kind: "thaw"}, replay, cur.prefix); err != nil {
				cancel()
				return fmt.Errorf("epilogue thaw: %v", err)
			}
			if err := mcProbeDup(inputs, replay, cur.prefix); err != nil {
				cancel()
				return fmt.Errorf("dedup probe: %v", err)
			}
		}
		cancel()
		// A cancel child has the same state key as its parent (the oracle
		// does not model terminality), so it can never be enqueued; verify
		// cancellation at every state on its own fresh actor instead.
		if err := mcVerifyCancel(seed, cur.prefix, cur.o); err != nil {
			return fmt.Errorf("cancellation: %v", err)
		}
		if len(cur.prefix) >= mcMaxDepth {
			continue
		}
		enabled := []mcEvent{}
		if !cur.o.frozen {
			for _, id := range mcAlphabet {
				enabled = append(enabled, mcEvent{kind: "admit", id: id})
			}
			if len(cur.o.queue) > 0 {
				enabled = append(enabled, mcEvent{kind: "deliver"})
			}
		}
		enabled = append(enabled, mcEvent{kind: "freeze"})
		if cur.o.frozen {
			enabled = append(enabled, mcEvent{kind: "thaw"})
		}
		enabled = append(enabled, mcEvent{kind: "cancel"})
		for _, ev := range enabled {
			child := mcAdvance(cur.o, ev)
			key := child.key()
			if visited[key] {
				continue
			}
			if len(visited) >= mcMaxStates {
				return fmt.Errorf("state cap of %d exceeded: tighten mcMaxDepth or the alphabet", mcMaxStates)
			}
			visited[key] = true
			queue = append(queue, node{prefix: append(append([]mcEvent(nil), cur.prefix...), ev), o: child})
		}
	}
	return nil
}

// mcVerifyCancel replays the prefix on a fresh actor and cancels the inbox,
// asserting that Output closes and that anything still delivered in the race
// window is delivered at most once.
func mcVerifyCancel(seed []ID, prefix []mcEvent, o *mcOracle) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputs, err := New(ctx, seed)
	if err != nil {
		return fmt.Errorf("seed %v rejected: %v", seed, err)
	}
	replay := mcNewOracle(seed)
	replay.cancel = cancel
	for i, ev := range prefix {
		if err := mcApply(inputs, ev, replay, prefix[:i]); err != nil {
			return err
		}
	}
	return mcApply(inputs, mcEvent{kind: "cancel"}, replay, prefix)
}

func TestInboxExhaustiveAdmissionSafety(t *testing.T) {
	for _, seed := range [][]ID{nil, {"a"}, {"a", "b"}} {
		t.Run(fmt.Sprintf("seed=%v", seed), func(t *testing.T) {
			visited := map[string]bool{}
			if err := mcRun(seed, visited); err != nil {
				t.Fatal(err)
			}
			t.Logf("seed %v: %d distinct actor states explored exhaustively", seed, len(visited))
		})
	}
}

// TestInboxFrozenBlocksAdmission verifies with a genuinely blocked send that
// a frozen actor does not receive submissions; the explorer never offers one.
func TestInboxFrozenBlocksAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inputs, err := New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := mcNewOracle(nil)
	o.cancel = cancel
	if err := mcApply(inputs, mcEvent{kind: "admit", id: "a"}, o, nil); err != nil {
		t.Fatal(err)
	}
	if err := mcApply(inputs, mcEvent{kind: "freeze"}, o, nil); err != nil {
		t.Fatal(err) // freezes hand over [a]; the oracle queue is now empty
	}
	select {
	case inputs.submissions <- submission{input: Input{ID: "b", Kind: InputExternal}, admitted: make(chan struct{})}:
		t.Fatal("submission was received while frozen")
	case <-time.After(mcFrozenBlockWait):
	}
	if err := mcApply(inputs, mcEvent{kind: "thaw"}, o, nil); err != nil {
		t.Fatal(err)
	}
	// Redelivering a is dropped (dedup survives the freeze/thaw cycle); only b
	// may be delivered. If a were wrongly re-admitted, the FIFO check fires.
	if err := mcApply(inputs, mcEvent{kind: "admit", id: "a"}, o, nil); err != nil {
		t.Fatal(err)
	}
	if err := mcApply(inputs, mcEvent{kind: "admit", id: "b"}, o, nil); err != nil {
		t.Fatal(err)
	}
	if err := mcApply(inputs, mcEvent{kind: "deliver"}, o, nil); err != nil {
		t.Fatal(err)
	}
}
