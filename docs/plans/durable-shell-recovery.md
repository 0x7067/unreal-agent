# M2.5 — interrupted shell reconciliation and output-only recovery

**Complete for the reviewed cells, accepted and installed 2026-10-28.**
[Required current-source validation, exact package/manifests, process proofs,
private frontend replacement and installed byte checks](../verification/durable-compaction-shell-m245-20261028.md).
This contract does not enable unclassified work or complete M2–M4.

## Reviewed exact cells

| Shell-v3/v4 checkpoint | Policy |
| --- | --- |
| Terminal history | Inert; never redispatch. Terminal status alone is not a global execution-drain receipt. |
| ready or awaiting create_directory/create_out/create_err | Refuse with prior-execution-uncertain and ownership-reconciliation-required diagnostic. Persisted state can lag actual dispatch, so even apparent prelaunch is not proven prelaunch. |
| awaiting process (with or without PGID) | Refuse. Missing result or PID is not proof of death; PGID is only an observation, not verified control authority. |
| canceling | Refuse. Cancellation request is not cancellation completion or old-executor drain. |
| awaiting read_out/read_out_tail/read_err/read_err_tail | Output-read-only recovery with known PendingExitCode, PGID zero, no result/error, valid exact phase buffers/sizes/paths and supported format. No ProcessStart dispatch. |
| Any other version, phase, inconsistent or unknown data member | Explicit unsupported refusal, without canonical mutation or provider request. |

`cmd/internal/recoverygate` is pure. Real constructor/decoder validation plus strict
schema/cell checks precede manager admission. No mandatory Manager, remote handler,
operation format or translator/context-builder I/O change. Existing shell-v3 and
budget-v4 semantics remain. Shared Bash result projection now exposes a recorded
exit code if output recovery fails; it does not fabricate complete output or a
cleanup receipt. Local upstream a5f3fd13032737142916523ae4344c392292f9d5 was inspected
without fetching: the gate is a private extension; the small shared translator
projection divergence was surfaced before editing.

## Completion and artifact semantics

Genuine ProcessExited is emitted only after the current primitive's command.Wait,
local process-group handling, capture sync/close and output worker joins. The
canonical read-phase checkpoint therefore records that local completion barrier.
This is not PID reattachment, detached-daemon/global cleanup proof, or authority
for a fresh manager to declare cleanup of arbitrary older work.

Output recovery reads the remaining **current owned capture artifacts**, preserving
already committed buffers. It is not an immutable content-addressed snapshot:
manual same-UID artifact modification is not excluded. Missing artifacts fail;
tail size changes fail under existing reducer validation. Never rerun Bash to
reconstruct missing output. Known exit, capture paths, and failure remain available.

## Focused evidence

- Gate matrix covers versions/statuses/all phases, malformed configurations,
  budget misuse, missing exit, result/error/PGID-bearing state, paths and exact
  phase buffer/truncation shapes; unknown members reject.
- `workerhost.TestM25ControlledShellRecovery`: real manager runs sentinel Bash;
  initial ready intent is committed before dispatch, selected latest state uses
  SaveOperation, final tool status withheld. Actual manager AwaitStopped precedes
  owned producer SIGKILL, scanner/Wait and lease joins. Actual runtime recovers
  v3/v4 read checkpoints including both tails, preserves provider-visible output,
  and yields one logical tool result/provider request/sentinel. Missing capture
  gives a safe error; ready/preparation/process/canceling refuse unchanged.
- This is an explicitly **controlled durable-boundary producer**, not an
  uninstrumented hook inside workerruntime. Cancellation intent is saved before
  the original manager cancellation; no terminal-state cleanup inference.
- `TestM25LiveOldExecutorRemainsUncertain`: a separately fixture-owned original
  manager keeps its effectful shell alive while replacement runtimes refuse twice.
  No provider request, second sentinel, canonical mutation or PID signal. Cleanup
  is performed only with the original manager capability, actual AwaitStopped,
  reader join and retained local cleanup receipt. Fresh-manager receipts cannot
  launder old work.
- `bash.TestRecordedExitSurvivesOutputRecoveryErrorProjection` verifies pure,
  nonmutating known-exit/error/capture-path projection.

This narrow policy fulfills classified reconciliation, not universal effect
recovery. Truly reattachable live shell execution needs the separately owned
idempotent job adapter in M4.3. Unclassified remote ownership remains fenced.
