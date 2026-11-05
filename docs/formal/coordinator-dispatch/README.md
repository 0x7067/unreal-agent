# Coordinator commit-before-dispatch / recovery safety

This small, bounded TLA+ model checks the coordinator's **manager `Add` boundary**,
not execution of a tool's external effects. It models one already-journaled,
successful model response containing one tool call that produces **two operations**,
and **at most two crashes/restarts**. Statuses are `Ready`, `Awaiting`, and
`Terminal` (completed/failed/canceled); `Absent` means no local/durable operation.

## Reproduce

From the repository root:

```sh
docs/formal/coordinator-dispatch/check.sh
```

For full TLC variable traces and state directories, retained in a printed temporary
location:

```sh
KEEP_TLC_LOGS=1 docs/formal/coordinator-dispatch/check.sh
```

The combined Go history checks and this model are run by:

```sh
.claude/skills/coordinator-dispatch-safety/scripts/verify.sh
```

`check.sh` itself only runs TLC; it does not run the Go checks. It needs Bash,
Java, `sha256sum`, `realpath`, and ordinary shell utilities. It downloads nothing.
`TLA2TOOLS_JAR` can select an existing jar; otherwise the repository's
`.tools/tlaplus/tla2tools.jar` is used. **Both choices must match the pinned hash**
before any Java invocation:

```text
TLC2 Version 2026.10.24.013619 (rev: 2ef2af8)
SHA-256 d15655eeaff4b3f7b3d65c46b838bc0f35d131b2ef4b1e473689b62c98d4726f
```

Recorded run: Eclipse Adoptium Java 21.0.13, one worker, seed 1, 256 MiB maximum
heap, no symmetry or state constraints. Four short JVM invocations take about
five seconds on the development host. Exact timings are not a portability gate.
The script requires a clean fixed-model completion and TLC exit **12** with the
specific expected invariant for each negative config; parser errors, other exits,
and missing diagnostics fail the script.

## Properties and results

Two sticky ghost flags record events without disabling bad transitions:

* **`CommitBeforeAdd`**: no manager `Add` without a committed tool-call status.
  That single status record includes **both** initial operation values.
* **`NoTerminalOrStaleAdd`**: no `Add` of a terminal local value, or of a local
  status different from its latest durable checkpoint. This intentionally detects
  stale `Ready` versus `Awaiting`, not just stale `Ready` versus `Terminal`.
  “Different” here means **status**, not payload bytes: payload and idempotency data
  are outside this model.

`TypeOK` is also checked. No fairness or liveness claim is made; idle/stopped
states may stutter (`CHECK_DEADLOCK FALSE`).

| Configuration | Result | Generated | Distinct | BFS depth |
| --- | --- | ---: | ---: | ---: |
| `fixed.cfg` | both safety properties pass | 866 | 510 | 31 |
| `skip-status-commit.cfg` | `CommitBeforeAdd` fails | 39 | 27 | 9 |
| `skip-terminal-guard.cfg` | `NoTerminalOrStaleAdd` fails | 478 | 298 | 23 |
| `skip-latest-overlay.cfg` | `NoTerminalOrStaleAdd` fails | 466 | 290 | 23 |

The fixed run exhausted its queue (zero states remaining). Negative runs stop
at the first shortest counterexample, so their counts are not full state spaces.

### Concise counterexamples

The script prints the full TLC action-name sequence (and source locations).
These are the same sequences with repeated operation steps grouped. `Init` is
state 1; numbers below are TLC state numbers, not source line numbers.

**Skip status commit, states 1–9:**

```text
1 Init
2 Translate                 (pure pair of Ready operations)
3–4 Install(1), Install(2)   (local only)
5 ApplyStatus               (local call.status only)
6 AppendStatus              (mutant skips durable commit)
7 StatusReturn
8 Guard(1)
9 Add(1)                    committed=FALSE; badUncommittedAdd=TRUE
```

**Remove terminal guard, states 1–23:**

```text
1–12 normal translation, local application, commit, Add(1), Add(2), DispatchDone
13 LocalUpdate(1,"Terminal") (local terminal; durable value still Ready)
14 AppendUpdate             (latest[1]=Terminal)
15 Crash                    (before UpdateReturn; crashes=1)
16–17 ReplayOperation(1), ReplayOperation(2)  (history embeds Ready, Ready)
18 ReplayStatus
19–20 Overlay(1), Overlay(2) (local[1]=Terminal, matching latest)
21 ScheduleRestored         (does not retranslate the saved call)
22 Guard(1)                 (mutant fails to skip terminal)
23 Add(1)                   local=latest=Terminal; badCheckpointAdd=TRUE
```

**Omit latest overlay, states 1–23:** same sequence, but state 13 is
`LocalUpdate(1,"Awaiting")`, state 14 commits `Awaiting`, and states 19–20 leave
replayed `Ready` unchanged. State 23 adds `Ready` while `latest[1]=Awaiting`.
The terminal guard cannot catch stale nonterminal snapshots. A saved `Terminal`
would likewise be redispatched as stale `Ready`; the first BFS trace happens to
use `Awaiting`.

## Source mapping and atomicity

Authoritative sources are `harness/coordinator/loop.go` and
`harness/sessionstore/localfile/{state,store}.go`. Function names, rather than
line numbers, keep the mapping usable when surrounding code changes.

| Model action | Source operations represented |
| --- | --- |
| `Translate` | `scheduleToolCall`: translator `Translate` / context `Submit` returns serializable initial Ready operations; **no I/O and no coordinator operation-map installation** |
| `Install` (one operation per step) | `scheduleToolCall` calls `addOperationToLocalState` for each translated operation |
| `ApplyStatus` | `addItemToLocalState(ItemToolCallStatus)` applies local call status and waiting references, **before** persistence |
| `AppendStatus` | `storeItemInSessionStore` → `AppendToolCallStatus`; `state.go` validates references; `store.go` appends a single `recordItem` containing the item and all its operations |
| `StatusReturn` | successful return through `scheduleToolCalls` / `handleModelResponse`; only afterwards can `processModelResponse` invoke dispatch for the returned operations |
| `Guard`, `Add` | separate `dispatchOperationToManager` terminal check and manager `Operations.Add` invocation; state/idempotency clones are pure and abstracted |
| `LocalUpdate`, `AppendUpdate`, `UpdateReturn` | separate local application in `handleOperationUpdate`, journal `SaveOperation` (`recordOperation`), and successful return |
| `Crash` | lose all coordinator state at any modeled boundary, including midway through restore, after commit but before return, or after a persistence error |
| `ReplayOperation`, `ReplayStatus` | `restore` → `loadHistory` → `restoreItem` / `addItemToLocalState`, rebuilding the initial embedded Ready snapshots and saved call status |
| `Overlay` (one operation per step) | `restore`'s **subsequent** `Restored.Operations` loop installs the latest checkpoints over historical Ready snapshots |
| `ScheduleRestored` | `Run` schedules unprocessed calls (`call.status == nil`) only **after restore**, then dispatches the resulting local operation map |
| `StatusError`, `UpdateError` | failed journal calls return an error through the coordinator and stop this invocation; `stopped` has no path to `Add` except a fresh crash/restart/restore |

No translation, local application, durable append, and dispatch are merged into
one “atomic scheduling” action. Local operation installation, replay, and overlay
are each split per operation, so crashes can interrupt any of them. Applying the
initial local status also repeats identical Ready map writes in real code; those
idempotent writes and pure construction/cloning are abstracted as stuttering.
The program counter only records control flow; advancing it alongside an action
is not another durable/source operation. `Translate`'s pure context is represented
by its control-flow stage rather than a redundant buffer variable.

The important localfile resume case is before the final tool-result status has
been journaled: history still lists the initial nonterminal Ready snapshots.
`storedState.resume` marks their IDs pending, then includes the **latest terminal
values** in `ResumeState.Operations` because they remain unrecorded in history.
`restore` must overlay those values after history replay, and dispatch must skip
them. The model deliberately remains in this pre-reconciliation interval; it does
not model a later final status that removes the call.

## Assumptions and limits

* **Journal abstraction, not filesystem verification:** one complete recoverable
  record atomically publishes either both initial operations and their call status,
  or one latest operation update. `store.append` encodes a record;
  `appendFile` truncates an incomplete tail, writes, syncs, and closes; the cached
  write head is updated after append succeeds. We abstract encoding, replay,
  checksums, caching, OS/filesystem durability and incomplete-tail handling.
  Crashes before/after the durable transition give old/complete records. A failed
  append may leave either an old journal or a complete recoverable record
  (`visible=FALSE/TRUE`), but **both error branches stop the loop**. No dispatch
  occurs merely because local mutation happened before an error. Corruption that
  cannot be read must fail resume; it is not modeled as a valid recovered record.
* One successful, already-persisted model response, one tool call, two operations,
  one coordinator owner; no concurrent writer. Model-response commit errors,
  multiple turns, failed turns, structural/lifecycle recovery, forks, cancellation,
  reconciliation, output payloads, manager internals and translator errors are
  outside this model. The separate Go checks exercise more source-level paths.
* Operation IDs 1 and 2 are representative fresh IDs; after a crash before the
  initial commit the translator may create fresh concrete IDs. Renaming them to
  1 and 2 is safe here because **no uncommitted operation has been dispatched**.
  The skip-commit mutant intentionally violates that premise and is rejected.
* The two operations are interchangeable and traversed in one canonical order.
  Both can independently advance Ready → Awaiting → Terminal or Ready → Terminal.
  Updates occur after an `Add` in this invocation, after the synchronous dispatch
  loop finishes. No manager update is processed midway through coordinator
  dispatch, matching its single loop. Repeated status-equal updates are omitted.
* Manager `Add` records an invocation with the successful-continuation path. An
  `Add` error only truncates that path (the source returns immediately); restarting
  is represented by a crash. The safety observation is at invocation, independent
  of whether `Add` succeeds. We do not prove manager deduplication or execution.
* **This is not at-most-once external effects.** A nonterminal operation can be
  added again after recovery, including an operation whose effects happened but
  whose terminal checkpoint was never committed. Safety requires the status
  authorization/checkpoint rules above, not a bound on how often a tool runs.
  This is bounded model checking, not a refinement proof of the Go implementation.


## Real-code validation and maintenance

The project-local skill is
`.claude/skills/coordinator-dispatch-safety/SKILL.md`; it contains the covered
functions, source blob hashes, and synchronization commit
`57b053059fe9dab05fab796605d2894df314c4c2`.

`harness/coordinator/dispatch_modelcheck_internal_test.go` exercises the real
coordinator methods against reopened localfile stores in 500 bounded histories:
25 checkpoint pairs (Ready/Awaiting/Completed/Failed/Canceled), both save orders,
and all ten pairs of two restart cuts around the saves and reconciliation.
Initial/final probe reopens are additional to the chosen cuts. An independent
oracle checks retained terminal checkpoints, full operation snapshots, exact
per-ID Add counts, and removal only after terminal history reconciliation.
Six live commit scenarios cover response/status failures with both old-prefix
and complete-record-with-error outcomes and recovery from each durable prefix.
These generated cases are not a proof over arbitrary Go executions.

Run the skill's scratch-production-code self-test:

```sh
.claude/skills/coordinator-dispatch-safety/scripts/verify.sh --self-test
```

Observed: all four Go mutations compile and fail behaviorally (skip status
commit, remove terminal guard, omit latest overlay, drop unrecorded terminal
retention), then the unchanged baseline passes. The production files are never
edited: Go overlays compile scratch variants and are discarded. Focused checks
also include the existing full-loop initial-commit-error and partial-completion
regressions. Source code changes require rereading the mapping, not just rerunning
an unchanged model. Move the skill's sync commit forward only after that review
and successful revalidation.

## Reading results with /statechart

Read **every** model result with the pstack `/statechart` skill, using its current
instructions rather than copying a release-specific invocation here. That skill
was actually run on all four configurations during creation. The fixed model's
510 states exceed its 500-state full-chart budget; the checks still complete.
The expected skip-commit mutant breaks **both** `CommitBeforeAdd` and
`NoTerminalOrStaleAdd` on its first Add (there is no latest checkpoint at all).
The other two mutants break only `NoTerminalOrStaleAdd`; `TypeOK` holds in all
four. Rechecking every invariant matters: plain TLC stops at its first violation.

Compatibility observed during creation: the then-current upstream `/statechart`
SANY XML reader failed with the verification runner's newer pinned TLC jar before
checking any rule. Running the **unmodified** skill with its own pinned TLC 1.7.4
worked and confirmed the same fixed-state count and shortest counterexamples.
Use that skill's supported toolchain; do not treat its parser error as a safety
finding or silently change this verification runner's pinned jar. The skill's
commands, options, and installed paths deliberately are not copied here.

A retained human-readable summary and the passing one-crash illustration are in
[`STATECHART.md`](STATECHART.md); the verified production-model bound remains two
crashes.
