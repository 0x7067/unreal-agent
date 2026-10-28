# M0.2 — Disconnect, worker death, and deliberate shutdown

Status: **complete**, with [full verification evidence](../verification/durable-lifecycle-m02-20261027.md).
This implements [goal slice M0.2](../../goal.md#m02--characterize-disconnect-worker-death-and-deliberate-shutdown).
It records current behavior and the target distinction between detach, abort,
and stop. **No production lifecycle, protocol, or recovery behavior changes.**

## Current versus target behavior

| Boundary | Current behavior and evidence | Target / implication |
| --- | --- | --- |
| Normal UI exit | TUI `run` defers `shutdown`, which closes the host and joins submissions; it is not detach. Existing host tests cover clean stop during model, shell, graph, and subagent work. | Client exit should eventually support detach without stopping an independently owned host. Ordinary exit defaults and explicit stop must be documented when that changes. |
| Abrupt frontend process loss during a model request | The thin frontend fixture is SIGKILLed. The real worker exits, the fake provider sees its HTTP transport disconnect, session/host leases become available, and one input/turn with no response remains. | Worker exit/transport loss is not proof a remote provider stopped computation or billing. A reconnectable host must outlive its client instead. |
| Abrupt frontend process loss during Bash | Closing both worker pipes can end the worker while its separately owned FIFO-blocked shell remains alive. Kernel leases are released, but shell intent remains unresolved; explicit resume refuses without another provider request or sentinel effect. | This is **not graceful shutdown** and lease availability is not effect cleanup. Keep unsafe resume blocked; later separation of client and host lifetime removes this particular client-loss trigger. |
| Unexpected worker death during a model request | `Host.Done` reports failure; the worker is observed exited, generation does not advance, reload refuses, and the host/ingress lifetime is not automatically closed. Explicit host Close releases host ownership. A new Start retries the unfinished model turn: one logical input, two persisted turns/observed requests, no response. | No blanket exactly-once request/billing promise. Recovery/retry policy needs to be explicit and bounded in M2. |
| Unexpected worker death during Bash | Direct SIGKILL of the worker leaves the separately owned shell alive. Worker lease becomes available; host lease remains held until Close. Session retains unresolved shell intent and Start refuses before provider execution; the effect remains exactly the fixture's original `x`. | Preserve uncertainty and do not replay the effect. A surviving shell is killed and observed exited by **fixture cleanup**, not falsely credited to runtime shutdown. |
| Cooperative explicit host Close | Existing real-process tests show worker/model/owned shell/graph/subagent cleanup, resumable terminal state, no repeated effects, and lease reacquisition. Runtime tests gate provider cancellation return to prove join before lease release. | Explicit stop may cancel work, but must prove supported cleanup before claiming execution drained. Preserve these contracts while adding detach. |
| Caller Close deadline with a blocked callback | An expired caller deadline returns a deadline error. The worker can already have exited and its lease be available while host/ingress leases remain held because a callback has not drained. Releasing the gate and retrying Close releases those remaining leases. | A caller timeout never grants takeover permission. Distinguish worker exit, callback completion, journal closure, and host ownership release. |
| Forced Close / escalation | Existing SIGSTOP/short-grace test shows nil Close after forced worker exit can coexist with unresolved shell state and resume refusal. Host Close does not return the worker's process/runtime result. | **Nil Close is not sufficient evidence of graceful execution cleanup.** Do not reinterpret it as universal terminal persistence or process-tree containment. |

## New executable scenarios

[Linux process-level tests](../../cmd/internal/workerhost/lifecycle_boundaries_linux_test.go):

### `TestRealFrontendProcessLossExposesInterruptedShell`

Runs model and shell scenarios through a separate **thin frontend process**, the
production Host, actual worker subprocess, journal/store, and provider adapter.
The HTTP provider is loopback fake. The fixture is intentionally not Bubble Tea:
it tests the same host/pipe ownership boundary without claiming terminal UI or
physical terminal-disconnection behavior.

The frontend reports its worker PID before admitting input. The parent pins
worker identity with a pidfd, installs cleanup, then authorizes work via a private
stdin handshake. A shell writes its PID and a single sentinel before blocking on
a named FIFO. No arbitrary quiet-period sleep establishes readiness.

After SIGKILL of the frontend, the parent joins that direct process, independently
observes worker exit, checks both leases, and inspects persisted state. The shell
case directly observes a surviving shell and refuses an actual new worker Start;
no automatic retry/repeated side effect is allowed. The model case inspects one
persisted input, one turn, and no completed response.

**Important distinction:** runtime EOF tests keep a usable output writer. Actual
frontend death removes both pipe peers. Runtime shutdown tries to persist control
and operation updates through synchronous observers; a lost process stdout can
terminate the worker before all owned effects are drained. The process test
establishes the resulting shell/lease/state behavior; it does not claim an
orphan's exact exit signal or direct parent reaping of that worker.

### `TestRealWorkerUnexpectedDeathRetainsHostOwnershipAndRecoveryBoundary`

Runs model and shell scenarios through a live host. Kills the observed worker via
its pidfd without first calling Close. Checks failure reporting, actual exit,
unchanged generation, and refused automatic reload. Checks worker lease available
versus host lease still held, then verifies host Close releases its ownership.

On explicit fresh Start, model-only unfinished work produces a second request
and turn for the same single input. A shell instead leaves nonterminal intent and
refuses startup before another model request. Its sentinel remains `x`, and the
shell remains alive until independently owned fixture cleanup kills/observes it.
The checkpoint may still be ready or awaiting depending on the unrecorded-start
window; neither status authorizes repeating the already-observed effect.

### `TestRealWorkerCloseDeadlineRetainsHostLeaseAfterWorkerExit`

Gates the input `OnItem` callback while a fake model request is active. Calls Close
with an already-expired deadline to avoid a scheduler-dependent short timeout.
This is an error/ownership contract test, not a measurement of shutdown duration.

Even after pidfd-observed worker exit and worker-lease release, host lease and
ingress journal still reject another owner. Only releasing the callback and
retrying Close releases them. This prevents conflating direct child termination
with the rest of host cleanup.

## Observation and fixture ownership

- New process tests are Linux-specific and require pidfds; unsupported kernels or
  restricted environments skip before starting work. Acceptance here requires an
  actual non-skipped run; these scenarios do not establish macOS behavior.
- A pidfd pins the observed process identity and becomes readable on exit,
  including when an orphan is a zombie awaiting its actual parent's reaping.
  `kill(pid, 0)` alone would not distinguish a zombie from running execution.
  The direct frontend child also has an actual `exec.Cmd.Wait` result.
- Fixture shell identity comes from its private workspace and live blocked
  process, not a guessed PID or a resumed control capability. This is not a
  production process-reattachment policy.
- Cleanup uses pidfd-based signaling and observes exit before closing the handle.
  Cooperative host/frontend cleanup is ordered before worker-kill fallbacks when
  shell discovery fails, so an assertion failure does not deliberately orphan an
  untracked shell. The crash assertions themselves still use abrupt termination.
- Test read waits, fixture lifetimes, and cleanup waits are bounded. Provider
  notifications do not block handlers if a test has already failed. Gates reflect
  actual request arrival, committed callbacks, kernel process state, and locks;
  they are not inferred from how long a command stayed quiet.

## Existing evidence reused, not replaced

- [`TestRealWorkerCloseJoinsActiveOwners`](../../cmd/internal/workerhost/integration_test.go):
  explicit Close for idle/model/shell/graph/subagent, actual startup after stop,
  effect counts, process disappearance, and session/frontend/runner leases.
- `TestRealWorkerForcedCloseKeepsUnresolvedResumeFailClosed` in the same file:
  stopped worker, forced exit, unresolved intent, refusal before unsafe replay.
- [`TestShutdownCancelsAndJoinsModelBeforeReleasingLease`](../../cmd/internal/workerruntime/run_test.go):
  stop/EOF/cancel/output-failure cross-product; a cancellation-ignoring callback
  is held until explicitly released, and the provider/session lease remains owned.
- `TestShutdownPersistsTerminalOperationsBeforeResume` in the same file:
  shell/graph cross-product, latest tool-call snapshots (not just an empty resume
  set), canceled-owner terminal persistence, and actual same-session restart.
- `TestResumeRefusesUnknownOperationBeforeProviderSetup` in the same file:
  rejects unresolved state before credential/provider lookup or command launch.
- [TUI shutdown tests](../../cmd/unreal-agent-tui/shutdown_test.go): fake-host
  tests prove notice, second-signal force, and bounded frontend joining behavior;
  they do not by themselves prove real child cleanup.

Full suite execution must include these existing tests, not only the new focused
scenarios. Source inspection is not substituted for passing execution evidence.

## Intended lifecycle contract for later slices

1. **Detach:** disconnect a client, preserving the independent host and foreground
   and background execution. This is a target, **not available in current UI exit**.
2. **Foreground abort:** request cancellation of the current owned foreground
   work; do not silently cancel retained background work. Persist what is known
   and retain uncertainty when cleanup is unconfirmed. Full semantics need the
   M1 contract review, not an inference from these tests.
3. **Explicit session/host stop:** fence admissions, cancel work according to
   ownership, join supported runtimes, and release ownership only at its proven
   boundary. Forced termination must not fabricate terminal operation outcomes.
4. **Unexpected death:** report loss, retain durable identities and effect
   uncertainty, and apply only reviewed recovery policies. Existing unresolved
   operation refusal remains in place until those policies are implemented.

## Limitations and follow-ups

- This is process-crash characterization, not power-loss/storage durability proof.
- No reconnect/attach service, bounded client queues, or retry policy is added.
- Released leases after process death do not imply an external provider or shell
  completed, stopped, or became safe to replay.
- The observed unfinished-model retry is current behavior, not a newly approved
  general retry policy or guarantee of provider cancellation, exactly-once
  execution, billing, or bounded retry across repeated restarts.
- Normal UI exit is mapped from its real defer path plus existing shutdown tests;
  no new live terminal/tmux proof or Bubble Tea behavior change is claimed.
- Source comparison uses the previously recorded local `upstream/main` baseline;
  no shared interface, invariant, or storage-format changes occur in this slice.

## Verification

Focused final scenarios passed three consecutive race-enabled runs, with no
pidfd skips. Full `make test` exited 0 with both race suites (41 passing packages,
3 no-test packages); `make check` and `make build` passed. Both binaries were
atomically reinstalled and hash-verified. Source inputs remained unchanged, and
the exact validation service/helper drained.

The [verification record](../verification/durable-lifecycle-m02-20261027.md)
preserves commands, complete suite output, input hashes, and ownership evidence.
**M0.2 is accepted as characterization, not as implementation of detach/recovery.**
