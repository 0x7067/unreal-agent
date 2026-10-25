# Worker reload/handoff safety audit with TLA+/TLC

Bounded model checks of the resident-worker handoff and inbox Freeze/Thaw
contract. These are **abstract models, not a verified refinement of Go**.
`Variant = "fixed"` retains the historical guarded baseline, including its
abandoned-retirement status defect. It is not the corrected-status variant.
`Variant = "statusOnAbandonedRetire"` is the corrected-status candidate model
reflecting the shipped status repair; neither variant proves Go refinement.

## Historical source finding and shipped status repair

In the original `cmd/internal/workerhost/host.go` path, `Reload` could return on
cancellation after receiving `quiesced` but before `p.done` closed. The worker
could then retire successfully. In `consume`, a retired, failure-free owner set
`h.stopped = true`, but the original
`reportStopped := owner && !closing && (!p.retired.Load() || failure != nil)`
was false. No stopped status was emitted, leaving a historical stale ready
status after `h.current` became nil. This clean-retirement path did not call
`h.finish`; later inputs could still be durably admitted without a dispatcher
target, and a later explicit successful reload could recover the host.

The shipped repair separates status publication from terminal host completion:
`reportStopped` becomes `owner && !closing`, while `finish` remains conditional
on `!retired || failure != nil`. The stopped callback carries generation/build
identity and occurs before clearing `current` and closing `p.done`. A successful
clean retirement therefore publishes stopped without latching failure or closing
`Done`; durable `Submit` and later explicit reload recovery remain available.
Refusing new admissions would be a separate semantic change, not this repair.

The retained historical-baseline `handoff-defect-status.cfg` finds:

```
Begin, Stage(ok), Fence, Quiesce, QuiesceAccept, Abort, OldExit
owner = 0, live = {}, stopped = TRUE, published = "ready"
```

`handoff-defect-input.cfg` also admits input 1, leaving `pending = {1}` with no
stopped status. These are regression illustrations of the **original defect**,
not claims that current code still exhibits it. `handoff-corrected-status.cfg`
changes only the model's abandoned-exit publication and checks I1-I10. Its
corrected-status candidate model reflects the shipped repair, not a proof of the
Go callback/barrier ordering, open `Done`, or complete recovery. The model has no
`Done` channel and collapses publication and ownership/barrier changes into one
atomic exit step. Blocking callbacks and their interleavings remain outside it.

These results are formal-model evidence only. Go regressions and their execution
results must be reported separately; no test names or successful Go runs are
inferred from TLC. Old failing-test transcripts and sandbox-specific lease
restrictions are not current validation evidence.

## Model/code correspondence

Source locations are navigation references; they are not mechanically linked.

| Action/fields | Code/contract abstracted |
| --- | --- |
| `Init` | host already started, gen 1 ready (`host.go` Start/launch); initial failure out of scope |
| `Submit`, `pending` | `Host.Submit` -> durable ingress; two new IDs in journal order |
| `Write`, `sent` | dispatch, delivery fence, mark-before-write and once-per-generation ID handling |
| `Admit`, `queue`, `seen` | worker pipe decode and inbox FIFO/dedup contract |
| `Persist`, `Ack` | coordinator append and host item-frame journal acknowledgment; separate steps |
| `Begin`, `inflight` | `h.reload.CompareAndSwap(false, true)` |
| `Stage`, `Reject`, `probe`, `cand` | `stage.go`: immutable copy, `worker-probe`, compatible `Identity.Validate`; internals collapsed to ok/bad |
| `Fence`, `GuardBusy` | delivery token plus pending-ID/`p.sent` guard |
| `Quiesce`, `QuiesceAccept` | control frame, freeze/drain FIFO, accepted retirement; control cannot overtake earlier input frames |
| `Abort` | refusal/cancellation; only an already accepted retirement can be abandoned in this model |
| `OldExit`, `stopped`, `published` | ordered consumer and process wait barrier; clean-retirement latch and status behavior |
| `Launch` | old process gone, safeStopped, probed staged replacement; dedup seed from persisted IDs |
| `Crash`, `failed` | between-reload failure latch; no automatic replacement |

`InboxFence.tla` models the inbox contract: open actor admission/dedup and FIFO
output, `Freeze` transfers the admitted FIFO to its caller, `ProcessHanded`
processes it, and `Thaw` requires processing to finish and preserves dedup.
This is a **caller contract**, not a claim that the inbox API enforces the caller's
processing. The separate inbox implementation was not re-audited here.

## Properties and expected outcomes

All HOLDS verdicts mean exhaustive safety checking **within this abstraction and
its bounds**, not production correctness, liveness or filesystem durability.

| Property | Meaning | Evidence/config |
| --- | --- | --- |
| I1 | at most one live generation and its unique delivery owner | HOLDS, `handoff-fixed` |
| I2 | every admitted ID is pending or persisted | HOLDS, `handoff-fixed` |
| I3 | no ID appears twice in persisted sequence | HOLDS, `handoff-fixed` |
| I4 | strictly increasing persisted IDs (admission order) | HOLDS, `handoff-fixed` |
| I5 | replacement implies old process exit | HOLDS, `handoff-fixed`; early-launch variant violates I5 |
| I6 | quiesce requires good probe and staged candidate | HOLDS, `handoff-fixed`; no-probe variant violates I6 |
| I7 | at most one in-flight reload | HOLDS, `handoff-fixed`; no-cas variant violates I7 |
| I8 | unused candidate exists only during active attempt | HOLDS, `handoff-fixed`; no-discard variant violates I8 |
| I9 | published ready implies a live owner | Historical baseline VIOLATED, `handoff-defect-status` |
| I10 | pending input with no owner implies published stopped | Historical baseline VIOLATED, `handoff-defect-input` |
| I1-I10 | all above, corrected-status candidate reflecting shipped repair | HOLDS, `handoff-corrected-status` |
| NoLoss | every seen/admitted ID is delivered, queued or transferred | HOLDS, `inbox-fixed`; thaw-abandons violates NoLoss |
| NoDuplicateDelivery, FIFO | dedup survives thaw; increasing delivered IDs | HOLDS, `inbox-fixed`; thaw-resets-dedup violates NoDuplicateDelivery |

Counterfactuals are not assertions about shipped code:

- `handoff-legacy-no-seed.cfg`: **clean** for I1-I8 with guard intact.
- `handoff-legacy-no-busy-guard.cfg`: **clean** for I1-I8 with seed intact.
- `handoff-legacy-no-seed-no-guard.cfg`: **I3 violated** (`persisted = <<1,1>>`).

Thus the two defenses are redundant for these modeled safety properties. This
does **not** justify removing either production defense: ack failures, startup
replay ordering and other recovery paths are absent or abstracted. An earlier
model claimed no-seed alone failed, but its Abort(sent) could discard a queued
control and then admit post-control inputs before retirement. That was not a
faithful ordered-pipe trace and that claimed production conclusion is withdrawn.

Two intentional counterexamples prevent simple vacuous passes:

- `handoff-reachable.cfg`: violates `NoReplacementDelivery`, witnessing gen 2
  receiving both IDs and `persisted = <<1,2>>` (not just never launching).
- `inbox-reachable.cfg`: violates `NoTransfer`, witnessing a nonempty Freeze
  handoff. These are reachability witnesses, **not liveness** assertions.

## Bounds and limitations

- Two input IDs, each admitted once in ascending order; two generations; at most
  one completed handoff. Failed attempts can repeat, revisiting finite states.
  No proof for arbitrary IDs/generations follows from this bound.
- Six reload phases, two probe outcomes, a coarse candidate-presence bit,
  single owner, two coarse published statuses. `TypeOK` constrains seeded runs;
  the no-cas counterexample intentionally stops at inflight 2.
- Seeded queues/persistence are bounded by dedup; unseeded variants can apply
  each of two IDs once per generation. Expected invariant failures stop their
  searches early: counts for those runs are **not exhaustive state counts**.
- Timeout **before** quiesce acceptance is omitted as an abandoned-retirement
  path. Abort(sent) represents refusal; only Abort(quiesced) sets abandoned.
  This avoids allowing post-control frames into the retiring FIFO. Crash is
  modeled only between attempts. An abandoned exit racing an already-fenced
  second attempt and Close/Kill are outside the represented schedule.
- Persistence and acknowledgment are infallible. `Ack` is nondeterministic and
  may lag even across modeled exit. Production consumes ordered item frames
  before quiesced/exit; this over-approximation may produce spurious schedules.
  In particular, the no-seed/no-guard double apply demonstrates a model dependency,
  not a guaranteed executable production trace.
- Writes are atomic, ordered and successful. Partial writes, encoding/size
  rejection (`workerproto.MaxFrame`), scan errors, cancellation during write,
  and transport failure during reload are omitted. Crash is not a complete
  abstraction of those failures.
- Coordinator busy/final-idle checks, model/tools/subagents/task graphs,
  compaction/forks/migration, journal/session filesystem failures, probe internals,
  launch failure, actual staged-file removal errors, callback blocking, leases
  and escaped process groups are not modeled. `cand` tracks intended ownership,
  not proof that `os.Remove` succeeded.
- Inbox novel IDs arrive in increasing order and one outstanding Submit is
  allowed. Dedup-reset counterfactual NoLoss quantifies current `seen`, not all
  historical admissions; its duplicate-delivery check supplies the failure.
- No fairness, temporal formula, eventual delivery, cancellation progress,
  bounded-memory or production refinement proof is provided.

## Reproduce and validation

Use an installed Java and local TLA+ tools jar; nothing is downloaded or vendored:

```sh
TLA2TOOLS_JAR="$PWD/.tools/tlaplus/tla2tools.jar" docs/formal/handoff/tla/check.sh
```

`check.sh` requires Bash, `realpath`, GNU-compatible `timeout`, Java and standard
text tools. On macOS supply compatible utilities (for example GNU coreutils on
PATH); Linux validation is not a macOS portability check. Each run has a 300s
wall-clock limit, 1 GiB Java heap, one worker and seed 1. Temporary logs/state are
created outside the repo and cleaned on exit. No user cache/data is deleted.

Clean runs require exit 0 and TLC's no-error completion marker. Expected
violations require exit 12 and the **named** invariant, so parse errors, timeout
and unexpected violations fail. Anti-vacuity witnesses use the same strict check.
All 16 configs are enumerated explicitly; add an expectation when adding a config.

Validation with the available TLC jar and Java 21 passed all 16 expected outcomes:

| Exhaustive clean run | Distinct states |
| --- | ---: |
| handoff-fixed | 786 |
| handoff-corrected-status | 786 |
| handoff-legacy-no-seed | 786 |
| handoff-legacy-no-busy-guard | 1,078 |
| inbox-fixed | 44 |

Expected violations: status I9 (129 distinct states before stopping), input I10
(165), early-launch I5 (64), no-seed-no-guard I3 (574), no-discard I8 (22),
no-probe I6 (41), no-cas I7 (9), thaw-abandons NoLoss (22), thaw-resets-dedup
NoDuplicateDelivery (61), replacement-delivery witness (564), Freeze-transfer
witness (12). These partial-search counts are not exhaustive state counts.
