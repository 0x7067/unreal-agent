# Agent-supervision handoff verification — 2026-10-27

Baseline `385e908`; refreshed upstream `a5f3fd1`. This verifies the completed
[handoff](../plans/agent-supervision-diagnostics-handoff.md), not only its earlier
internal activity projection. No commit or push was made.

## Integrated validation

Passed on the integrated source:

- `make test check build`: race suites in both modules, vet in both modules,
  formatting check and both binaries.
- Full root TaskGraph package, including `TestGraphExhaustiveSafety`: **280.594s**
  in the initial full race run; reused unchanged successful package evidence in
  the final full run. The reducer was not edited.
- Full workerhost race package: **107.846s**. This includes transport/lifecycle
  regression coverage, not just activity unit tests.
- Full TUI race package: **14.722s**, including the actual durable worker control
  round trip.
- Inbox admission/freeze skill command, `-count=1 -v`: **244** exhaustive actor
  states across three recovery seeds; structural-control envelope cases pass.
- `python3 -m unittest discover -s benchmarks/harness-diagnostics -p 'test_*.py' -v`:
  **15 passed**. These are evaluation-fixture tests, not a new live-model score.
- `git diff --check`.

The first full run found a stale CLI tool-list expectation that omitted the newly
registered AgentControl. The expectation now explicitly requires ViewImage,
AgentControl and TaskGraph, and the final full run passed. Earlier runtime bugs
found during integration (invalid self-cleanup transition and initializing a new
control through generic SaveOperation) were fixed with real regression tests,
not bypassed by weakening the store or declaring enqueue to be completion.

## Boundary and integration evidence

Representative executable regressions:

| Boundary | Tests |
| --- | --- |
| Report versus actual task/background drain | `TestTaskReportCleanupAndExplicitBackground`; independent fake gates and real child process helpers |
| Root control of a graph worker before candidate completion | `TestRootControlsGraphChildBeforeCompletion`; reads the real child's persisted feedback input before accepting delivery |
| UI durable control without an LLM-control turn | `TestAgentsUIRealDurableStopWithoutModelControlTurn`; input → versioned initializer → terminal checkpoint → outcome |
| Crash after terminal checkpoint but before outcome | `TestStructuralControlRealStoreInitializationAndCrashReplay`; no reexecution or fabricated model turns |
| Localfile authorization/replay/fork/failure recovery | `control_operation_internal_test.go`; prior owned invocation required; generic unknown SaveOperation still fails |
| Slow stdin, parent consumer and archive work | `TestBlockedInputAndQueuedCancellationRemainResponsive`, `TestBlockedParentDeliveryDoesNotBlockReapingOrControl`, `TestPreparationConcurrencyButOrderedPublication` |
| Cancel before dispatch | `TestPreparationBlockedAdmissionStopAndNoLateLaunch` plus existing TaskGraph queued-scan cancellation regressions |
| Quiet work and timeout without false drain | `TestQuietShellReachesObservableBudgetAndActualDrain`, `TestBudgetWarningCancellationRetainsUninterruptibleOwnership` |
| Shared account and emergency controls at capacity | `TestSharedAccountAcrossManagersRetainsRemoteUntilTerminal`, `TestSaturatedWorkAdmitsOnlyBoundedInspectionAndStopControls` |
| Later blocker after routine completed history | `TestActivityOverflowRetainsQuietNestedBlockerAfterTerminalHistory` |
| Status independent of model/blocked child I/O | `TestActivityQueryResponsiveWithoutModelOrBlockedChildIO`, `TestSnapshotCancellationInterruptsBlockedTransportWithoutWriterGoroutine` |
| Replay/privacy/unknown execution phase | TUI diagnostics tests; runner heartbeat summary tests; coordinator bounded/redacted heartbeat test |
| Scope versus necessary validation | `TestSupervisionPreservesScopeAndVerificationPolicy` and contextbuilder prompt-delivery tests |

A direct reproduction command for the complete UI/worker/store path is:

```sh
go -C cmd/unreal-agent-tui test -race \
  -run '^TestAgentsUIRealDurableStopWithoutModelControlTurn$' -count=1
```

These deterministic fixtures do not search `/`, need model credentials, or infer
process drain from report text. Fake model calls are test drivers, not behavioral
evaluation evidence.

## Real terminal proof

The repository TUI verification skill was followed in isolated tmux sessions,
with fake HTTP model providers and separate workspace/state/config directories.
No real frontend or user session was restarted.

Final **root-built frontend** evidence:

```text
/tmp/unreal-agent-handoff-completion-20261027-160504/root-ui-final/evidence/
  list.txt             report-pending.txt    inspect.txt
  collapsed.txt        narrow.txt            stop-result.txt
  drained.txt          stale-result.txt      builds.txt
```

Observed states, with ephemeral handles abbreviated:

```text
HANDLE@1 · Executing · reviewer · shell → sleep … [ready]
message · HANDLE@1 · delivered · delivery barrier: persisted
Review reported; cleanup pending · reviewer · 1 pending
Report available: true (not proof of drain)
Process joined: false · output EOF: false · reader joined: false
stop · HANDLE@1 · cancellation requested · drain NOT confirmed
HANDLE@1 · Execution drained; terminal result enqueued
Resources (in_process_operation_envelopes):
  work 0 / 64 · management 0 / 8 · total 0 owned
Error: unknown, stale-generation or cross-session agent handle
```

The `[ready]` is explicitly a last persisted child observation, not a claim that
the shell has or has not started. The quiet real shell is independently owned by
the fixture. The final text does not erase its blocker. Detail view, collapsed
Tools and 60-column layout retain the diagnostic and control affordances.
`/builds` identifies the actual executable digest and the checked candidate;
installation is not presented as reloading a frontend.

Parent driver attempts with incorrect focus, an incorrect expectation that
repeated stop should fail, and an overly specific error-text assertion are
retained beside the final evidence. Corrected assertions match actual semantics:
stop of a drained target is idempotent; wrong generation fails explicitly. No
production changes or suppressed stderr were used to make those attempts pass.
The final run's stderr is empty. Test/model/tmux/helper processes were drained;
only evidence directories remain.

## Build and installation

`make build` rebuilt both executables after the final Go change. Each installed
file was staged in `~/.local/bin` and atomically renamed, then byte-compared
against `bin/`:

| Binary | SHA-256 of both built and installed file |
| --- | --- |
| `unreal-agent-runner` | `cbc5c590b502412c2d5c80d6043e3cd8cdd16866689551f3ff067ac61b561b14` |
| `unreal-agent-tui` | `5d21765371e3df4d0bcdf5df9c26e18038f742a7c282009632a40b6ae36408b2` |

Installed `unreal-agent-tui worker-probe` returned private protocol **2**, session
version **2**. Local development labels are `dev+unknown+unknown`; the binary
digest, rather than those labels alone, distinguishes implementations.

**Operational boundary:** the existing active frontend was intentionally not
killed. Restart it for the new UI/protocol; worker `/reload` alone cannot replace
frontend code, and incompatible versions fail explicitly. There is no claim that
installation hot-patched the currently executing session.

## Limits that remain explicit

- Child activity is bounded last-observed persisted evidence, not omniscient
  cross-process manager state. Oversized/unretained details are unavailable with
  omission counters, not proof of idle.
- Accounting is shared for in-process operation envelopes, including graph
  managers and remote-agent owner envelopes. Child-private internals have their
  own propagated policy/account; this is not an OS-wide process-tree quota.
- SIGINT, warning and timeout do not prove termination of uncooperative execution
  or a blocked syscall. Ownership is retained until the actual barriers drain.
- Private queues/frame limits do not make arbitrary external callbacks or all
  existing inbox/manager queues preemptible or bounded.
- Known-path and validation-reuse instructions are tested for delivery; neither
  these fixtures nor the historical exploratory model runs prove universal
  compliance. Arbitrary shell commands are not cached on command text alone.
