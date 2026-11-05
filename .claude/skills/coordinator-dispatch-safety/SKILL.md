---
name: coordinator-dispatch-safety
description: Use when changing coordinator tool scheduling, persistence-error propagation, operation dispatch, history replay, or localfile Resume projection. Checks that tool operations are committed before Add, recovered from the latest checkpoint, and never redispatched after a durable terminal checkpoint; includes bounded TLA+ crash exploration and real-Go journal/recovery checks.
---

# Coordinator durable dispatch safety

**Promise:** the coordinator dispatches only durably initialized tool operations,
using their latest durable checkpoints, and never dispatches a recorded terminal
operation. **Harm if broken:** a tool can run without recoverable authority, run
again after finishing, or resume from stale state and repeat destructive work.
This does **not** promise exactly-once external effects.

## When to use and what to read

Use for changes to these covered paths (even if only error handling changes):

| Source | Covered functions / role |
| --- | --- |
| `harness/coordinator/loop.go` | `processModelResponse`, `handleModelResponse`, `scheduleToolCalls`, `scheduleToolCall`, `toolCallContext.Submit`: response/status commit before manager Add |
| same | `handleOperationUpdate`, `reconcileToolCalls`: separate checkpoint and final-result commits |
| same | `restore`, `loadHistory`, `restoreItem`, `addItemToLocalState`, `dispatchOperationsToManager`, `dispatchOperationToManager`, `operationIsTerminal`; `Run` startup orders restore → schedule → dispatch |
| `harness/sessionstore/localfile/state.go` | `appendToolCallStatus`, `saveOperation`, `storedState.resume`: initialized IDs, canonical latest values, terminal checkpoints pending reconciliation |
| `harness/sessionstore/localfile/store.go` | `AppendToolCallStatus`, `SaveOperation`, `Resume`, `append`, `appendFile`: actual journal boundary (assumed atomic recoverable records in TLA+, exercised through real files in Go) |
| `harness/sessionstore/localfile/codec.go` | log replay/record decoding; exercised on ordinary complete records, not formally verified |
| `harness/sessionstore/sessionstore.go` | `ToolCallStatus`, `ResumeState`, `Store` contracts |

Read `docs/formal/coordinator-dispatch/README.md` for the action-to-source mapping;
read `CoordinatorDispatch.tla` alongside those functions. The Go checks are in
`harness/coordinator/dispatch_modelcheck_internal_test.go`. They use the real
coordinator functions and reopened localfile store, not a hand-written simulator
of the Go function. There is no general Go implementation proof here.

## Run

From the repository root, one command:

```sh
.claude/skills/coordinator-dispatch-safety/scripts/verify.sh
```

Typically seconds with a warm Go cache: four TLC runs plus focused Go checks.
Needs Bash, Java, Go **1.27.x** (module minimum 1.27.0; validated with 1.27.3),
`sha256sum`, `realpath`; Python 3 is needed for the mutation self-test below.
Validated with Java 21.0.13. The checked-in script downloads nothing.

TLC defaults to the existing `.tools/tlaplus/tla2tools.jar` (not tracked by git).
For a fresh machine, obtain the reproducible release artifact:

```sh
mkdir -p .tools/tlaplus
curl -fL https://github.com/tlaplus/tlaplus/releases/download/v1.8.0/tla2tools.jar \
  -o .tools/tlaplus/tla2tools.jar
```

The runner **verifies SHA-256 before executing**:
`d15655eeaff4b3f7b3d65c46b838bc0f35d131b2ef4b1e473689b62c98d4726f`
(TLC reports `2026.10.24.013619`, rev `2ef2af8`). To use that identical artifact
elsewhere, set `TLA2TOOLS_JAR`. Do not silently replace the pin after a failure.

**Read every TLA+ result with the pstack `/statechart` skill**, including the
passing fixed configuration and all three expected-negative configurations.
Use that skill's current instructions, not a copied invocation. It explains each
violated rule as a shortest plain-language path, rechecks the other rules, and
charts the passing state graph when within its chart budget. The fixed two-crash
model has 510 states, over its current 500-state chart budget; lack of a chart
is not a failed verification. See the README's statechart compatibility note.
If `/statechart` is unavailable, obtain that skill or explicitly report the gap;
do not claim its report was run. Neither these scripts nor this skill depend on
its release-specific command paths or options.

## Expected results and interpreting failure

* `fixed.cfg`: **no invariant violation**, 510 distinct states, queue exhausted,
  depth 31 (866 generated). This is the production-design model.
* `skip-status-commit.cfg`: expected `CommitBeforeAdd` violation. Translation
  creates two local operations; the status write is skipped; `Add` runs anyway.
  `/statechart` also exposes `NoTerminalOrStaleAdd` here because no canonical
  checkpoint exists. This is a deliberate broken model, not a product finding.
* `skip-terminal-guard.cfg`: expected `NoTerminalOrStaleAdd` violation. Save a
  terminal update; crash before returning from the save; replay initial Ready
  history, overlay Terminal; the missing guard calls `Add(Terminal)`.
* `skip-latest-overlay.cfg`: expected `NoTerminalOrStaleAdd` violation. Save an
  Awaiting checkpoint; crash; replay Ready; omit the overlay; `Add(Ready)` uses
  stale state. Terminal checkpoints can likewise be revived as Ready.
* Go checks: **500 histories** (25 pairs of Ready/Awaiting/Completed/Failed/
  Canceled checkpoints × two save orders × ten ordered pairs of restart cuts).
  Each history checks the actual disk projection, exact per-ID Add multiplicity,
  latest serialized state, partial sibling completion, reconciliation, and final
  reopen. Cuts are before either save, before reconciliation, or after it;
  equal cuts mean two consecutive restarts. Initial/final validation reopens
  supplement the two selected cuts. Six commit/error scenarios cover successful
  writes and old-prefix/complete-record-with-error outcomes for response/status
  commits, then retry with a fresh coordinator and store. Existing single-loop
  regression tests also cover initial persistence failure and partial completion.

`CommitBeforeAdd` is an event property: the invariant uses a sticky observer,
not an action precondition that would forbid the unsafe transition. The second
invariant similarly observes terminal/stale Add; its TLA+ checkpoint comparison
is **status only**, while the Go checks compare the complete operation value.

On failure:

1. Preserve the named failing subtest/trace. `KEEP_TLC_LOGS=1` retains model logs.
   A nonzero exit, tool/parser error, missing jar, timeout, or unexpected state
   cap is **not** a safety counterexample and is not an expected-negative pass.
2. Tell the path in code's terms. For example: first status initializes op-0
   and op-1 → op-0 completes and its checkpoint saves → crash before result
   reconciliation → Resume must retain that terminal checkpoint → restore must
   overlay it → only op-1 may be Added.
3. Compare each modeled action with source. A record may become recoverable
   even when append returns an error; that invocation must stop. Local mutation
   before commit is intentional, not itself a finding. Only a **fresh** recovery
   may use surviving complete records.
4. Reproduce a fixed-model violation against the actual Go code before calling
   it a product defect. Keep real findings as findings: do not weaken the
   property/model or fix product code without the user's request.
5. If a behavior change is intentional, update the source mapping, model,
   real-code oracle, bounds and mutants together. Do not merely change the
   expected outcome. Run the ordinary check and mutation self-test again.

## Prove it still catches bad real code

```sh
.claude/skills/coordinator-dispatch-safety/scripts/verify.sh --self-test
```

This runs the normal recipe, compiles **four scratch changes to production Go**
using Go source overlays outside the repository, demands a named behavioral
failure for each, discards them, then reruns the clean Go baseline. Compilation
errors do not count. Working product files are never edited or reset.

| Scratch production change | Required failure |
| --- | --- |
| Skip `scheduleToolCall`'s status persistence | `Add before matching committed initialization` |
| Remove `dispatchOperationToManager`'s terminal guard | `unsafe Add after crash` |
| Omit `restore`'s latest-operation overlay | `unsafe Add after crash` (even Ready with changed payload is detected) |
| Remove `resume`'s unrecorded-terminal retention | `recovery lost or changed checkpoint` |

All four were observed failing on these real-code changes; the unchanged baseline
passed afterwards. If a source anchor moves, the self-test refuses to guess:
review the changed function and update its uniquely scoped anchor in
`scripts/mutations.py`.

## Bounds and exclusions

TLA+ explores **one already-persisted successful model response, one tool call,
two operations, at most two crashes**, Ready → Awaiting → Terminal or Ready →
Terminal. Each local install, commit, dispatch guard/Add, replay and checkpoint
overlay is separate; crashes interrupt restore too. Completed/Failed/Canceled
collapse to Terminal. Errors may leave an old or complete journal prefix, but
stop the live invocation. The model covers the interval **before** terminal
result reconciliation; the Go cases additionally cover reconciliation and
terminal projection removal. No liveness/fairness or deadlock claim is made.

Assume one coherent store snapshot and exclusive session writer ownership,
pure translators, valid journal records, and manager checkpoints that do not
regress from terminal. Neither model nor bounded tests prove OS/fsync/torn-write
behavior, arbitrary payloads or histories, all Go schedules, multi-call/turn
interaction, manager internals, process exit, lifecycle/structural tools, forks,
request-budget recovery, or exactly-once effects across a lost checkpoint.
`StatusUnsupported` is not treated as terminal and is outside the status alphabet.
Nonterminal Add on restart is permitted. Existing
`docs/formal/operations` addresses the adjacent effect/checkpoint-lag question.

## Coverage and synchronization record

Last confirmed against commit **`57b053059fe9dab05fab796605d2894df314c4c2`**.
No covered product source was changed during creation. Recorded source blobs:

| Path | Git blob |
| --- | --- |
| `harness/coordinator/loop.go` | `e21b669cf8369728bfb9d6ba5b79ba9dc6e9742b` |
| `harness/sessionstore/localfile/state.go` | `7b3b38068564e95f9d31a67e51d3054940ebf392` |
| `harness/sessionstore/localfile/store.go` | `ae65e558b371c5dd00a4e2af90efed5685b2be6c` |
| `harness/sessionstore/localfile/codec.go` | `92d4adfe560da448b57a05582a67bf4f4b9ad0f5` |
| `harness/sessionstore/sessionstore.go` | `b06dfcba1e72065ca12ddbbd704f0f02919e506d` |

Start a relevant change by comparing those files against the sync commit,
including working-tree changes. A passing unchanged model cannot establish that
it still describes edited code. **Advance the sync commit only after reviewing
all covered changes, confirming source/action correspondence at the new commit,
and rerunning both recipes.** Update hashes with `git hash-object` only after
that review. Uncommitted validation should be labeled as such, not described as
having occurred in a future commit.

Use `/maintain-formal-verification-skill` to restore synchronization when later
changes bypass this recipe.
