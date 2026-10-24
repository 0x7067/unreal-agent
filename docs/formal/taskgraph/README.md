# Task-graph checkpoint audit with TLA+/TLC

## Findings and fixes

These are **malformed-checkpoint validation bugs**, not demonstrated failures of
normal `Start`/`newChild` execution. A corrupted or externally constructed
checkpoint could pass validation despite violating the scheduler's assumptions.
Existing reducer exploration starts with legal graphs and follows legal events;
it therefore does not exercise this restore boundary.

### 1. Restoring a graph bypassed scheduling safety checks

`Graph.Validate` checked declarations and individual nodes, but omitted these
cross-node properties that `Start` normally enforces:

| Property | Minimal previously accepted invalid checkpoint |
| --- | --- |
| Claim exclusion | A writes `shared`, B reads `shared`, both Running. Also affects Canceling and Completed reservations. |
| Concurrency bound | Two disjoint Running/Canceling tasks with concurrency 1. |
| Prerequisite gating | B depends on A; A Pending, B nonstale Running, Canceling, or Completed. |

`runtimeState.add` trusted this validation before restoring children. The fix
checks reservations, active count, and prerequisite acceptance in `Validate`.
Completed candidates hold claims but do not occupy worker slots. Stale active
attempts still hold claims and slots, but may legitimately outlive prerequisite
acceptance after invalidation; regression tests preserve that exception.

### 2. Duplicate child IDs misrouted terminal updates

`validateAttempts` validated each child separately but allowed two current
attempts to share a child operation ID. `runtimeState.child` routes an update to
the **first** matching current attempt, even if that node is already Completed.
For two disjoint Running tasks with terminal persisted children:

```text
checkpoint:     A Running,   B Running, child(A).ID = child(B).ID
restore A:      A Completed, B Running
restore B:      A Completed, B Running  # update matches A again, then returns
```

B is left Running with no newly dispatched child and no recovery blocker. The
fix rejects duplicate IDs among current-generation attempts before handler
setup or restoration. Obsolete attempts do not reserve IDs.

## Models and code correspondence

- `Checkpoint.tla` abstracts `Graph.Validate` and `validateAttempts`, called by
  `runtimeState.add`. It enumerates arbitrary supplied checkpoints, then performs
  a `Restore` action constrained by either the legacy or corrected validator.
  Safety predicates independently name claim exclusion, concurrency, dependency
  gating, and current-child identity. The corrected validation specification
  explicitly incorporates those predicates; this checks the finite restore
  contract, **not an automatically extracted proof of the Go implementation**.
- `ChildRouting.tla` operationally models the node-order restore loop and the
  first-match/return behavior of `runtimeState.child`. Both children are already
  terminal; after two update deliveries, both tasks must be Completed unless the
  checkpoint was rejected. The legacy model gives the four-state trace above;
  the fixed validator rejects colliding IDs without changing update routing.
- `legacy-*.cfg` each request one expected invariant violation. `fixed.cfg`
  checks all restore invariants. `routing-*.cfg` check the operational consequence.

### Bounds and assumptions

The checkpoint model covers two tasks, all seven task statuses, both stale bits,
an optional B-to-A dependency, optional one-way writer/reader conflict, concurrency
1 or 2, and two current child IDs. There are **6,272 initial assignments**. Task
declarations are valid/canonical; graph/checkpoint versions and generations are
valid; acceptance evidence, required children, and fingerprints are well-formed.
It omits optional current attempts on Pending/Failed/Canceled nodes and obsolete
attempts; Go tests additionally cover obsolete-ID reuse. The routing model has
four initial ID assignments, two independent Running tasks, concurrency 2, empty
claims, and successful fingerprint checks.

This is bounded safety checking, not an unbounded theorem or a proof of liveness,
process reattachment, filesystem correctness, or the entire harness. The separate
Go state explorers remain valuable for reducer/actor transitions.

## Reproduce

Requires Java and a local TLA+ tools jar. The audit used the jar from:

```sh
curl -fL -o /tmp/tla2tools.jar \
  https://github.com/tlaplus/tlaplus/releases/download/v1.8.0/tla2tools.jar
# SHA-256 of the jar used in this audit:
# d15655eeaff4b3f7b3d65c46b838bc0f35d131b2ef4b1e473689b62c98d4726f
TLA2TOOLS_JAR=/tmp/tla2tools.jar docs/formal/taskgraph/check.sh
```

The script requires TLC exit code 12 plus the named invariant violation for each
legacy run, and successful exhaustive checking for each fixed run. Parser errors
or other tool failures are not counted as expected counterexamples. It prints
counterexamples and summaries; temporary TLC states/logs are created outside the
repository and removed on exit. No jar or generated model-checker output is
vendored.

Observed results:

- Legacy restore: all four expected invariant violations found.
- Fixed restore: **7,748 distinct states**, no invariant violation.
- Legacy routing: after both terminal updates, A Completed / B Running.
- Fixed routing: **12 distinct states**, no invariant violation.

## Go regression evidence

```sh
go test ./harness/taskgraph -run '^TestCheckpoint' -v
go test -race ./...
go -C cmd/unreal-agent-tui test -race ./...
```

Race-enabled tests passed for both Go modules, including the existing exhaustive
reducer explorer. The new checkpoint tests also passed ten race-enabled repetitions.
Session-lease tests require a private temporary-directory ancestor; the initial
run used a group-writable `GOTMPDIR` and failed those unrelated permission checks.
After changing that test directory to mode 0700, the complete root suite passed.

Tests were added and run before the production fixes. The original implementation
failed with `Validate() = <nil>` for invalid restored graphs, accepted duplicate
current child IDs, and restored the duplicate-ID fixture as `awaiting`, invoking
handler setup once instead of rejecting it. All new tests pass after the fixes:

- `checkpoint_validation_internal_test.go`: all reserved-status pairs, concurrency
  mixtures, prerequisite statuses, stale-invalidation exception, legal shared
  reads, Completed slot exemption, and validation nonmutation. Its independent
  oracle checks **1,568 arbitrary two-task checkpoints**, using no production
  `active`/`reserved`/`conflicts` helpers to compute expected outcomes.
- `checkpoint_identity_internal_test.go`: distinct IDs, duplicate current IDs,
  obsolete duplicate IDs in both traversal orders, and real runtime restore
  rejection before `NewAgentHandlers` can run.
