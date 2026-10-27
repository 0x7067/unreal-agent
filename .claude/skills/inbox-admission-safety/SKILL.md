---
name: inbox-admission-safety
description: Use when a change touches harness/inbox/local.go (the inbox actor run loop: admission, dedup, FIFO output, Freeze/Thaw, shutdown) or harness/inbox/inbox.go's input/control validation. Exhaustively explores every admission/delivery/freeze/thaw/cancel interleaving against the real actor and fails with a step-by-step counterexample when an input could be admitted twice, delivered out of order, or lost at a freeze.
---

# Inbox admission safety

`harness/inbox/local.go` runs the session inbox: one goroutine that dedups
input IDs (`seen`), queues accepted inputs FIFO, delivers them on `Output`,
and fences admission/output behind `Freeze`/`Thaw`. The model check
(`harness/inbox/inbox_modelcheck_internal_test.go`, package `inbox`) drives
the **real** actor through its real channels, one channel operation at a
time — each operation is one indivisible rendezvous, so the exploration
covers exactly the interleavings the Go scheduler could produce — and checks
at every state:

1. **At-most-once admission** — an input ID is delivered on Output or
   transferred by Freeze at most once over the actor's lifetime, across
   redeliveries, Freeze/Thaw cycles, and seen IDs restored after a crash
   (`New`'s `seenIDs`). Harm if broken: a redelivered event (at-least-once
   delivery is the callers' contract) runs twice — a tool executes twice, a
   control input applies twice. Verified at every state by a duplicate
   redelivery probe (redeliver a seen ID, freeze, compare the transfer).
2. **FIFO** — Output delivers admissions in admission order; Freeze
   transfers the remaining FIFO in the same order.
3. **No loss on freeze** — a committed Freeze hands over exactly the inputs
   admitted so far and not yet delivered.
4. **Shutdown** — after cancellation, Output closes; anything still
   delivered in the cancellation race window is delivered at most once
   (cancellation otherwise drops the queue, which is documented
   volatile-inbox semantics).

`TestInboxFrozenBlocksAdmission` additionally verifies with a genuinely
blocked send that a frozen actor receives nothing until Thaw.

## Run it

```
go test ./harness/inbox -run 'TestInboxExhaustiveAdmissionSafety|TestInboxFrozenBlocksAdmission' -count=1 -v
```

Well under a second, ~250 actor states across three crash-recovery seeds
(empty, one, two restored IDs). Part of the normal package tests.

## Read a failure

- **`dedup probe: freeze after redelivering X: freeze transferred [...],
  expected [...]`** — a redelivered input was admitted again; the trace names
  the state's event sequence (e.g. `after [admit(a)]`). Check what changed in
  the `seen` bookkeeping or the Freeze/Thaw handling in `run`.
- **`FIFO broken: delivered "b", expected "a" after [admit(a) admit(b)]`** —
  delivery order or queue popping changed.
- **`freeze transferred [...], expected [...]`** — the transfer lost,
  duplicated, or reordered the FIFO.
- **`send of X was not received within 2s` / `expected delivery of X, actor
  blocked`** — the actor stopped servicing an enabled operation (deadlock or
  enablement divergence); the trace shows where.
- **`state cap of 50000 exceeded`** — the state space outgrew the bounds;
  not a safety finding. See what keeps generating distinct states, then
  tighten the alphabet or `mcMaxDepth`.
- A pass covers the bounds below, not every conceivable workload.

## Bounds

- ID alphabet `a`–`d`, seeds nil/`{a}`/`{a,b}` (restored seen IDs). With this
  alphabet every reachable actor state has a shortest event sequence well
  under `mcMaxDepth = 12` — the enumeration is complete for this protocol;
  184 states for the empty seed matches the combinatorial reachability count
  by hand.
- External-kind inputs without payload; control-kind inputs take the
  identical admission path (the actor does not consult the kind).
- `Submit`'s prelude (validation, payload cloning, caller-context checks)
  runs before the actor and is covered by the package's other tests.
- `Freeze`/`Thaw` have one owner in production; the explorer issues them
  sequentially from that one owner's position. Racing control requests and
  blocked-producer release on shutdown are covered by the package's other
  tests (`freeze_test.go`), not by the exploration.

## Prove the check still bites (spot-check)

After changing the checker or the actor, break the code on purpose and
confirm a counterexample appears, then restore. Each mutation below is one
unique line in `local.go` (verify with `grep -c`); restore from a copy —
**these files may carry uncommitted work, never `git checkout --` them**:

| Break | Edit the unique line | Expect |
| --- | --- | --- |
| Admission not recorded | `seen[input.ID] = struct{}{}` → `_ = input.ID` | dedup probe: transfer `[a]` vs `[]` after `[admit(a)]` |
| Thaw clears dedup | `frozen = request.freeze` → append `; if !request.freeze { seen = map[ID]struct{}{} }` | dedup probe after a freeze/thaw cycle |
| Deliver from tail | `next = queued[0]` → `next = queued[len(queued)-1]` and `queued = queued[1:]` → `queued = queued[:len(queued)-1]` | FIFO broken after `[admit(a) admit(b)]` |
| Freeze drops queue | `queued = nil` → `_ = queued` | freeze transfer mismatch |
| Deliver does not pop | `queued = queued[1:]` → `_ = queued` | FIFO/dedup violation at the next delivery |

## Coverage record

Verified on 2026-10-27 at baseline **385e908** plus the supervision-control
working-tree changes: `inbox.go` blob
`5b666ce31f262a6ca337efaba7551eba32dafea4`, `local.go` blob
`c1a632a45cb8bea9f962aac243947077ca1ad627` (check with
`git hash-object harness/inbox/<file>`; move the record forward only to
content you have re-verified). Covers in `harness/inbox/local.go`: `New`,
`run` (admission, dedup, FIFO output, freeze/thaw control handling,
shutdown), `Freeze`, `Thaw`, `changeAdmission`; in `harness/inbox/inbox.go`
the `Input`/admission contract the explorer exercises. When those change,
rerun the check, update this record, and only then advance it. When the
actor gains a new event (say, a drain or peek control), add it to the
explorer's `enabled` list and to `mcAdvance`, and add a row to the
spot-check table.

## Upkeep

Follow the `maintain-formal-verification-skill` guidance in the same branch as
the change that invalidated a recipe here.
