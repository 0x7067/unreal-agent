---
name: taskgraph-reducer-safety
description: Use when a change touches harness/taskgraph/graph.go (the task DAG reducer) or the events the taskgraph runtime feeds it — scheduling, file claims, acceptance, cancellation, revision, or checkpoint validation. Runs an exhaustive model check of every event interleaving and fails with a step-by-step counterexample when a safety promise breaks.
---

# Taskgraph reducer safety

`harness/taskgraph/graph.go` is a pure reducer that decides when tasks in a
dependency DAG may run. The runtime (`harness/taskgraph/runtime.go`,
`Runtime.loop`) drives it from a single goroutine through its public methods, so
every possible runtime interleaving is a sequence of those events. The model
check explores **all** such sequences (BFS over reachable states, memoized) and
asserts the reducer's safety promises at every state:

1. **Claim exclusion** — no two tasks that hold claims (Running, Canceling, or
   Completed awaiting acceptance) have overlapping read/write claims. Harm if
   broken: two processes write the same files concurrently and corrupt work.
   Overlap is recomputed inside the test, independent of `conflicts()`.
2. **Dependency gating** — a live, non-stale task has all dependencies Accepted
   and fresh. Harm if broken: work runs on inputs a parent never accepted.
3. **Concurrency bound** — Running plus Canceling never exceeds
   `Graph.Concurrency`.
4. **Accepted closure** — an Accepted task's dependencies are Accepted and
   fresh (canceling or invalidating a prerequisite must invalidate descendants).
5. **Checkpoint validity** — every reachable state passes `Graph.Validate()`,
   the same check the runtime applies when resuming a persisted checkpoint, so
   no crash can strand the graph in a state the next run would reject.
6. **Ready/Start agreement** — the batch from `Ready()` can be started in order,
   which `runtimeState.advance` assumes.

## Run it

```
go test ./harness/taskgraph -run TestGraphExhaustiveSafety -count=1 -v
```

~19 s, ~55,000 states over four seed graphs, no network, no credentials. It
also runs as part of the normal package tests. The checker is
`harness/taskgraph/graph_modelcheck_internal_test.go` (package `taskgraph`, so
it can reach unexported helpers; its invariants are deliberately independent
re-implementations).

## Read a failure

- **`safety violation ... trace:`** — a reachable sequence of real reducer
  events broke a promise. The trace names real methods (`Start(A)`,
  `Cancel(X)`...). First decide: is the new behavior an intended change to the
  reducer's promises, or a bug? Do not weaken a property to make a failing
  check pass; report bugs with the trace, and only re-derive a property when
  the product promise itself changed.
- **`state cap of 400000 exceeded`** — the state space outgrew the bounds
  (below), usually because a change made the reducer less idempotent. This is
  not a safety finding; investigate what keeps generating distinct states, then
  tighten `mcGenerationCap` or the shapes and say so in the skill's bounds.
- A pass covers the seed graphs and bounds below — not every conceivable graph.

## Bounds

- Seed graphs in `mcShapes()`: a chain with an outside reader of a shared
  claim, three independent tasks under the concurrency cap, nil-vs-empty
  claims, and directory-prefix claims. Result text is fixed (`"ok"`/error);
  state identity keeps only whether acceptance evidence is nonblank.
- `mcGenerationCap = 3`: any successor in which some task's generation would
  exceed 3 is not explored (each task reset at most twice per path). The filter
  must be a *post*-filter: `Cancel` on a non-active task resets its descendants
  without raising its own generation, so a per-task gate cannot bound the space.
- `Revise` is explored via fixed single-task upsert proposals per shape. Adding
  brand-new tasks through `Revise`, calls with stale generations, and blank
  evidence (rejected before mutation) are not enumerated; they cannot produce
  states the enumerated events miss.

## Prove the check still bites (spot-check)

After changing the checker or the reducer, break the reducer on purpose and
confirm a counterexample appears, then restore. Each mutation below is one
unique line in `graph.go` (verify with `grep -c`); restore with
`git checkout -- harness/taskgraph/graph.go` afterwards:

| Break | Edit the unique line | Expect |
| --- | --- | --- |
| Start ignores claims | `if reserved(other) && conflicts(n.Task, other.Task) {` → prefix `false &&` | claim exclusion, 2-step trace |
| eligible ignores deps | `if p == nil \|\| p.Status != Accepted \|\| p.Stale {` (inside `eligible`) → `if p == nil {` | dependency gating, 1-step trace |
| Start ignores cap | `if count >= g.Concurrency {` → prefix `false &&` | concurrency bound |
| Cancel keeps descendants | `for descendant := range g.affected(map[string]bool{id: true}) {` → iterate an empty map | dependency gating first; accepted closure appears in the same break once the shorter gating traces are exhausted (disable the gating assertion temporarily to see it) |
| Canceled keeps stale | `n.Stale = false` inside `func (g *Graph) Canceled` → `n.Stale = true` | checkpoint validity |

Known result, not a bug: removing `Accept`'s own prerequisite re-check alone
(`if p.Status != Accepted || p.Stale {` inside `Accept`) produces **no**
reachable violation — the descendant resets in `Invalidate`/`Cancel`/`Revise`
already guarantee the closure, so that check is defense-in-depth against other
code changing.

## Coverage record

Verified at commit **8828795** (`graph.go` byte-identical to `HEAD`; the
working tree also carries unrelated uncommitted changes to `runtime.go` that do
not touch the reducer). Covers in `harness/taskgraph/graph.go`: `NewGraph`,
`Validate`, `Find`, `validate`, `normalizeClaims`, `eligible`, `Ready`,
`Start`, `Complete`, `Accept`, `Reject`, `Revise`, `Invalidate`, `Retry`,
`Cancel`, `Canceled`, `AllAccepted`, `affected`, `reset`, `reserved`,
`conflicts`, `intersects`, `overlaps`, `claims`. The runtime's event set (the
argument that reducer-level exploration covers every interleaving) is
`advance`, `recheck`, `child`, `control`, `cancelGraph`, `probe` in
`runtime.go`.

When a change touches these functions, rerun the check, update this record,
and move the sync commit forward **only after** the check passes (or its new
failure is understood and reported) at the new commit. When a change adds
claim semantics (new claim kinds, new statuses, new transitions), add a seed
graph or events to `mcShapes()`/`mcSuccessors` — the shapes exist to exercise
`overlaps` (equal, prefix, case-insensitive, `"."`), nil-vs-empty claims,
completed candidates holding claims, and cancel/revise/invalidate cascades.
Keep the whole test under ~30 s.

## Upkeep

Follow the `maintain-formal-verification-skill` guidance in the same branch as
the change that invalidated a recipe here.
