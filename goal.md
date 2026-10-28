# Goal: a durable, reconnectable, replaceable agent harness

Status: **27/27 defined slices accepted 2026-10-28**. This file remains an
architectural target; the per-slice evidence and explicit limits, not blanket
roadmap prose, define the guarantees actually earned.

## North star

**The durable session is the agent; the process running it is replaceable.**

An agent should survive client disconnection, runtime failure, and compatible
code updates without losing committed history, accepted inputs, task identity,
or ownership relationships. Clients should attach to the agent rather than own
its lifetime. Interrupted work must recover according to explicit semantics,
not through blind replay.

Inspired by [Pi Durable](https://earendil.com/posts/pi-durable/), especially its
checkpointed execution, attachable clients, and replaceable extensions. Adopt
these principles in this Go harness; a rewrite or adoption of Pi's framework is
not required.

## What the goal means

- **Durable:** accepted inputs, committed execution intent, checkpoints, and
  outcomes survive process restart within a documented storage failure model.
  An accepted submission is not necessarily session-persisted, model-delivered,
  or successfully executed. Preserve those distinctions.
- **Reconnectable:** detaching or replacing a UI does not stop the agent. A client
  receives a consistent current view and subsequent ordered updates, with an
  explicit recovery path for gaps. Retrying a submission with the same identity
  must not create a second logical submission.
- **Replaceable:** compatible frontend, runtime, and extension implementations
  can change without abandoning the session. Running work either drains on its
  original implementation, reattaches to an independently owned executor, or
  recovers from a checkpoint under its declared policy.

"Fully" does not mean every effect is reversible, every operation is replayable,
or arbitrary running code can migrate uninterrupted. Unknown outcomes must stay
visible. Process-crash recovery is not a promise to survive storage loss; broader
failure guarantees need explicit storage/backup contracts and tests.

## Current baseline and gaps

This table records the starting baseline; completed slice statuses below track
the shipped increments.

Initial source inspection: commit `d24507b7789652736a3f9e3d89e37abcc22d046c`.
These observations are a baseline, not permanent implementation instructions.

| Area | Existing foundation | Gap to close |
| --- | --- | --- |
| Input | Durable ingress and stable identities | Move ingress ownership outside the UI; reconnectable submission acknowledgments |
| History | Versioned, append-only session history and operations | Inventory recovery behavior and state needed for a consistent live client view |
| Execution | Commit operations before dispatch; operation checkpoints | Explicit recovery decisions for every supported task/operation type and phase |
| Ownership | Session leases, worker generations, subagent/graph ownership | Preserve durable identity across restart without reusing stale control authority |
| Reload | Worker probing, staging, and quiescent replacement | Independent client lifetime; later recovery-aware runtime replacement |
| UI | History replay and session resume | Attach/detach protocol, reconnect cursors, and preservation of local view state |

Relevant boundaries inspected:

- `cmd/unreal-agent-tui/run.go` starts the worker host in the frontend's lifecycle.
- `cmd/internal/workerhost/host.go` owns ingress, frontend/session-host ownership,
  worker delivery, and generation lifecycle; callbacks currently feed the UI.
- `cmd/internal/workerruntime/run.go` refuses resume with unresolved nonterminal
  operations. Do not remove this guard without a validated recovery path.
- `harness/operation/operation.go` already defines serializable, versioned state.
  Manager admission is at-most-once per ID **during that manager's lifetime**,
  not a blanket cross-crash execution guarantee.
- `harness/operation/shell.go` has phase-specific resume behavior. A shell process
  with no recorded exit is interrupted/unknown, not safe to launch again.

Existing [worker reload guarantees](docs/plans/hot-reload.md) remain authoritative
until explicitly superseded. The
[supervisor I/O follow-up](docs/plans/supervisor-io-follow-up.md) and
[async audit](docs/plans/supervisor-async-audit.md) identify related blocking-I/O,
resource-bound, and cleanup concerns; this goal does not mark those as resolved.

## Architecture direction

```text
TUI / future clients
        |
versioned attach, submit, watch, and control protocol
        |
resident host independent of client lifetime
        |
durable session history, ingress, task state, ownership
        |
replaceable execution runtime
        |
local or remote execution environments
```

Start with the smallest extraction of existing host responsibilities, not a new
framework. Transport, storage backend, multi-session hosting, and executor
isolation are design decisions to justify per milestone, not prerequisites to
all progress. The first transport should be local and authenticated/authorized
for its intended user; network access and multiplayer are separate scope.

### Recovery contract

For each operation/task type, version, and relevant phase, define one of:

- **Retry-safe:** repeating the interrupted work is safe under stated conditions.
- **Idempotent:** retry uses a persisted, stable key honored by the external system.
- **Reattachable:** reconnect to a separately owned job using durable identity and
  verified authority; do not infer liveness or ownership from a PID alone.
- **Non-replayable:** preserve available output and record interruption or uncertain
  effects. Require reconciliation or an explicit new action rather than retry.

Classify model requests, compaction, Bash, image/skill operations, subagents, and
TaskGraph work. Model retries may cost money and produce a different response;
any policy must handle partial output and retry limits explicitly. An intent
checkpoint cannot by itself close the crash window between an external effect
and recording its result.

### Compatibility and implementation replacement

Persist implementation/task names, supported versions, and serializable state,
not callbacks, goroutines, channels, or credentials. Define compatible reads,
explicit migrations, and unsupported-version failures before changing formats.
Old work must not silently acquire different semantics after an upgrade.

For later extension replacement, existing calls retain their implementation
generation; new calls use the selected compatible generation. In Go, prefer
explicit registries or subprocess boundaries over requiring Go plugins. Treat
extension loading as execution of trusted code. Changing the fixed tool-registry
contract is a separate, reviewed architectural change.

## Non-negotiable constraints

- Preserve the [README architecture](README.md#components): translators and context
  building perform no I/O; commit intent before operation dispatch.
- One authoritative writer per session. Fence stale generations and prove the old
  owner's exit/release before starting a replacement writer. Timeouts are not
  proof of termination or permission to take ownership.
- Preserve backward reads of supported sessions; reject unsupported state
  explicitly. No automatic downgrade after new code may have changed history.
- Never conflate input deduplication with exactly-once external effects. Do not
  automatically replay arbitrary shell, network, deployment, or payment actions.
- Preserve TaskGraph validation, explicit parent acceptance, and ownership rules;
  a completed candidate is not accepted merely because recovery found it.
- Distinguish foreground/background ownership, detach, abort, and host shutdown.
  Cleanup completes only with evidence that owned work has drained; uncertain
  cleanup must remain explicit across restart.
- Bound queues, frames, replay buffers, retained output, and retry behavior. A
  slow/disconnected client must not block execution or cause unbounded memory.
- Keep trusted executable selection explicit; no automatic workspace builds or
  execution of workspace-provided replacement code.
- Compare shared interface/invariant changes with `upstream/main` and surface
  divergence before implementing it. This goal authorizes the direction, not
  unreviewed changes to acknowledgment, replay, cancellation, or trust semantics.

## Milestones as verifiable vertical slices

Slices are **planned** unless explicitly marked complete with evidence below.
The initial goal and source map above are planning inputs, not by themselves
evidence that a slice has passed.

A slice follows one user/recovery scenario through every affected layer: client
or test driver, host, persistence, runtime, and executor where applicable. Ship
only the layers necessary for that scenario. Contract decisions, implementation,
diagnostics, and focused tests belong in the same slice; do not defer verification
to a later milestone. M0 is characterization-only: its deliverable is an executable
baseline plus the contract decisions needed for extraction, not new runtime
behavior.

Each slice declares prerequisites, what becomes demonstrable, and an acceptance
oracle. Slice dependencies allow useful overlap; finishing an unrelated milestone
is not an implicit prerequisite. New recovery/extension behavior remains disabled
for unclassified types, phases, or versions. Unknown cases retain conservative
refusal rather than inheriting a generic retry policy.

### M0 — Characterize contracts and choose the first extraction

**Status: complete.** M0.1–M0.4 have linked executable baselines and verification
records. The [reviewed first-extraction contract](docs/plans/durable-client-attach-contract.md)
constrains M1; no production attach or new automatic crash replay is claimed.

#### M0.1 — Trace one submission through acceptance and replay

- **Status:** **complete** — characterization and full repository validation passed.
- **Implementation/test links:** [state and failure-boundary inventory](docs/plans/durable-submission-lifecycle.md),
  [real-worker submission/replay test](cmd/internal/workerhost/submission_lifecycle_test.go).
- **Commands/results:** focused race test passed three consecutive runs;
  `make check`, `make build`, and the full `make test` passed on unchanged source
  inputs. The full race suite covered both modules, including the exhaustive
  TaskGraph checker: 41 passing packages, 3 no-test packages. Both binaries were
  atomically installed and SHA-256 verified. The authorized 20-minute supervised
  run finished in 4min 38.941s and its service/cgroup drained.
  [Verification record and complete output](docs/verification/durable-submission-m01-20261027.md).
- **Remaining limitations:** clean completed-turn replay only, not interrupted
  effect recovery, detach support, or power-loss durability. The inventory records
  ambiguous-error UI wording and raw-payload buffer ownership as follow-up gaps.

- **Depends on:** the initial source map above.
- **Deliver:** a state-ownership inventory for a single submitted turn, identifying
  ingress acceptance, session persistence, model delivery, committed history, and
  client-local state. Pair it with a characterization test using stable input IDs.
- **Verify:** submit, retry the same ID/payload, replay the session, then submit a
  conflicting payload under that ID. Observe one logical input/model delivery,
  explicit conflict rejection, and no claim that ingress acknowledgment alone
  proves model delivery. Capture current behavior at each acknowledgment boundary.
- **Evidence:** executable test and inventory linking each state to its current
  owner, authoritative storage or reconstruction source, and failure boundary.

#### M0.2 — Characterize disconnect, worker death, and deliberate shutdown

- **Status:** **complete** — current lifecycle boundaries characterized and verified.
- **Implementation/test links:** [current-versus-target lifecycle matrix](docs/plans/durable-lifecycle-boundaries.md),
  [Linux real-process scenarios](cmd/internal/workerhost/lifecycle_boundaries_linux_test.go).
- **Commands/results:** five new scenarios passed three consecutive race-enabled
  runs. Full `make test` exited 0 (both modules, 41 passing / 3 no-test packages);
  `make check` and `make build` passed. Both binaries were atomically installed and
  hash-verified; validation service and scoped review helper drained.
  [Complete evidence](docs/verification/durable-lifecycle-m02-20261027.md).
  No production lifecycle semantics changed.
- **Remaining limitations:** Linux pidfd process observations, not a live terminal
  detach proof. Frontend loss can leave a shell alive despite released leases;
  interrupted model-only restart can issue another provider request. Existing
  fail-closed shell resume and cleanup uncertainty are preserved.

- **Depends on:** M0.1.
- **Deliver:** deterministic lifecycle scenarios while a fake provider and an owned
  shell are blocked. Record which frontend/host/worker exit currently stops what,
  which leases remain held, and how unresolved work appears on resume. Specify
  the intended distinction between detach, abort, and stop without enabling it.
- **Verify:** independently exercise frontend loss, unexpected worker exit, and
  explicit shutdown. Check actual child exit/lease state, persisted outcomes, and
  resume refusal where cleanup is uncertain; do not equate pipe EOF with cleanup.
- **Evidence:** lifecycle tests plus a current-versus-target behavior table.

#### M0.3 — Classify a crash at each execution boundary

- **Status:** **complete** — version/phase baseline classified and verified;
  automatic interrupted-operation replay remains disabled.
- **Implementation/test links:** [recovery matrix and explicit blocked cells](docs/plans/durable-recovery-matrix.md),
  [real-primitive, lossy-persistence boundary test](cmd/internal/workerruntime/recovery_boundary_test.go).
- **Commands/results:** six scenarios (shell v3/v4 at intent, dispatched-before-effect,
  effect-before-result-commit) passed three consecutive race-enabled runs. Same
  restored ready intent corresponded to effects **0 / 0 / 1**; resume refused
  without extra effects, provider setup, history transfer or session mutation.
  Full `make test` passed both modules (41 passing / 3 no-test packages);
  `make check` and `make build` passed. Both binaries were atomically reinstalled
  and hash-verified; scoped helpers and validation service drained.
  [Complete validation record](docs/verification/durable-recovery-m03-20261027.md).
- **Remaining limitations:** checkpoint-loss simulation complements M0.2 physical
  process death; it is not every-version/every-phase crash coverage. Nested
  subagent/TaskGraph checkpoint enqueue is not a durable acknowledgement before
  launch; provider attempt identity, immutable source recovery, manual-only
  interrupted compaction, control atomicity and active-agent reattachment remain
  gaps. No production interface/invariant or recovery guard changed.

- **Depends on:** M0.1.
- **Deliver:** a recovery matrix covering model requests, compaction, Bash,
  image/skill operations, remote jobs, subagents, and TaskGraph by supported
  version/phase. Identify safe retry, idempotency, reattachment, or interruption,
  including unsupported cases and required durable identity/output.
- **Verify:** representative tests stop after intent commit, after dispatch, and
  after an effect but before its result commit. Count external effects and
  compare restored state with the matrix. Mark untested cells explicitly; every
  cell enabled later must gain its own evidence before its guard is relaxed.
- **Evidence:** matrix with source/test links and explicit unknowns, not an
  assumption that every serializable operation can resume.

#### M0.4 — Prove a bounded late-join client contract against existing events

- **Status:** **complete** — bounded history-only join and current gaps verified;
  local protocol/security extraction contract reviewed, not implemented.
- **Implementation/test links:** [view inventory and proposed attach v1 contract](docs/plans/durable-client-attach-contract.md),
  [real-worker late-join driver and gap tests](cmd/internal/workerhost/late_join_test.go).
- **Commands/results:** three focused tests passed three consecutive race-enabled
  runs. Snapshot H=3 plus an answer committed in an after-read/before-return hook
  converged exactly with canonical typed history; duplicates, no-subscriber commit,
  late join and bounded backlog/resync preserved three stable inputs/answers and
  three provider calls on the same responsive worker. Unit tests reject gaps,
  conflicts, wrong cursors and frame/byte/view overflows. Full `make test` passed
  both modules (41 passing / 3 no-test packages); `make check`, `make build` passed.
  Both binaries atomically reinstalled/hash-verified; service and review helpers
  drained. [Full evidence](docs/verification/durable-client-m04-20261027.md).
- **Remaining limitations:** test-only subscription, not actual socket/TUI detach.
  Item history omits operation-only updates and accepted pending ingress/live
  status; current store accepts ahead cursors and decodes full logs despite paging.
  Proposed peer authentication, published full-view barrier, bounds/chunks and
  independent host lifetime still require M1 implementation/evidence. No production
  contract or conservative recovery guard changed; upstream comparison recorded.

- **Depends on:** M0.1 and M0.2.
- **Deliver:** a test driver that reconstructs a view from current history/events,
  with a commit deliberately placed at the snapshot/watch boundary. Specify the
  proposed versioned local protocol, peer authorization, cursors/gap recovery,
  queue/frame limits, single-client or multi-client policy, and host discovery.
- **Verify:** the driver converges to canonical state without dropping or
  duplicating the boundary commit. Demonstrate current missing state or transport
  capabilities as explicit gaps; test a duplicate update, late join, and bounded
  consumer backlog without claiming that a production attach service exists.
- **Evidence:** executable reconstruction test, view-state inventory, and reviewed
  protocol/security decisions. Compare affected contracts with `upstream/main`.

**Milestone exit:** all four scenario baselines are recorded, untested recovery
cells remain blocked, and M1 has a bounded reviewed extraction contract. No new
automatic crash replay is enabled by M0.

### M1 — Independent resident host and reconnectable TUI

**Status: complete.** M1.1–M1.6 have linked current-source validation and owned
process/frontend evidence. Recovery-aware execution remains the next milestone.

#### M1.1 — Start one headless session and reconnect a local test client

- **Depends on:** M0.2 and M0.4.
- **Deliver:** an explicitly started resident host extracted from existing
  ownership code, plus the smallest authorized local attach/submit/watch path.
  Restrict this first slice to one session and the reviewed client policy.
- **Verify:** a client submits a blocked fake-model turn, disconnects, and attaches
  again while the same host/worker continues. Completing the model produces one
  committed answer. A competing host cannot acquire the same session, and a
  failed client connection does not retire the worker or release host ownership.
- **Evidence:** real-process host/client integration test with identity, lease,
  input, and answer assertions; no terminal frontend is required yet.

- **Status (2026-10-27): completed.** Explicit runner `host` and authenticated
  local `attach`/submit/watch; optional confirmed-ingress admission observation.
  [Implemented commands/protocol/limits](docs/plans/durable-headless-host.md),
  [real-process test](cmd/internal/residentcli/run_test.go),
  [verification and full logs](docs/verification/durable-host-m11-20261027.md).
- **Commands/results:** focused race/process checks repeated three times;
  `make test` (42 root + 5 TUI packages), `make check`, `make build` passed on
  final inputs; both binaries atomically reinstalled and byte-verified.
- **Remaining limits:** one session/active client, fresh snapshots only; no TUI
  attach, operation/activity view, control/reload RPC or new crash replay. The
  conservative unresolved-operation resume guard remains in force.

#### M1.2 — Attach the TUI to an already-running turn

- **Depends on:** M1.1.
- **Deliver:** a TUI attach path rendering canonical history, queued inputs, live
  work, and output from the host rather than owning worker lifetime. Keep the
  previous startup path working until its replacement is verified.
- **Verify:** start work headlessly, attach the TUI, detach it, allow the answer to
  finish, and attach a fresh TUI. Both views converge on the same session and
  answer without a second model request. Include a late join during tool output.
- **Evidence:** integration tests plus a fake-model tmux proof using the TUI skill.

- **Status (2026-10-27): completed.** Explicit `unreal-agent-tui attach` uses
  validated snapshots/live history without starting or owning its worker;
  standalone startup remains. [Usage/limits](docs/plans/durable-tui-attach.md),
  [integration tests](cmd/unreal-agent-tui/attach_test.go),
  [real tmux proof and validation](docs/verification/durable-tui-m12-20261027.md).
- **Commands/results:** client and TUI focused race tests repeated three times;
  `make test` (43 root + 5 TUI packages), `make check`, `make build` passed;
  both binaries atomically reinstalled. Five actual frontend processes proved
  detach/replacement across blocked model and shell-output cases, same worker
  generation, exact request/answer counts and ordered final views.
- **Remaining limits:** fresh snapshots/manual reconnect, committed tool history
  only (not live operation checkpoints/activity), no controls or persisted local
  view state. Automatic/UI stable-ID retry and richer acknowledgment display
  remain M1.3; existing unresolved-operation resume guard is unchanged.

#### M1.3 — Lose a submission acknowledgment without duplicating the turn

- **Depends on:** M1.1 and M0.1.
- **Deliver:** stable client submission identities, reconnect retry, and visible
  pending/accepted/persisted acknowledgment reconciliation through durable ingress.
- **Verify:** sever transport before admission, after journal sync but before
  acknowledgment, and after session persistence. Retry the same input each time;
  observe one logical submission and no loss after proven acceptance. Conflicting
  payloads are rejected, and capacity rejection is not displayed as acceptance.
- **Evidence:** fault-injected client-to-journal-to-model tests, plus a TUI check
  that local echoes reconcile instead of duplicating transcript messages.

- **Status (2026-10-27): completed.** Stable original input ID/payload survive
  lost acknowledgments and explicit F5 + `/retry`; canonical persistence wins
  late replies, conflicts/rejections are not acceptance, one echo per input.
  [Implementation](docs/plans/durable-tui-reconnect.md),
  [12 fault/cursor cases](cmd/internal/resident/submission_fault_test.go),
  [verification](docs/verification/durable-reconnect-m13-m14-20261027.md).
- **Commands/results:** focused client/protocol and TUI race checks repeated three
  times; real transport-to-journal-to-model fault cuts; real tmux retry and one-echo
  proof; `make test` (43 root + 5 TUI), `make check`, `make build` passed on final
  inputs; both binaries atomically reinstalled and byte-compared.
- **Remaining limits:** no automatic resend, public lifecycle controls or expanded
  recovery permission; explicit retry uses the retained external payload only.

#### M1.4 — Reconnect across an event gap and restore local view state

- **Depends on:** M1.2 and M1.3.
- **Deliver:** cursor-based reconnect with an explicit bounded snapshot fallback,
  plus client-local draft, selection, expansion, and scroll preservation. Do not
  write local view preferences into canonical execution history.
- **Verify:** disconnect during commits, reconnect with a valid, stale, and invalid
  cursor, and inject duplicate updates. The view converges without missing or
  duplicated items; manual scroll and draft survive supported client replacement.
  A corrupted local snapshot cannot change session state or execute an input.
- **Evidence:** snapshot/watch race tests, local-state compatibility tests, and a
  TUI detach/reattach demonstration.

- **Status (2026-10-27): completed.** Bounded cursor metadata and canonical deltas
  with explicit full-snapshot fallback; exact duplicate suppression; private
  versioned client draft/retry/selection/expanded-detail/manual-scroll storage,
  separately fenced against concurrent view writers and never auto-dispatched.
  [Implementation/limits](docs/plans/durable-tui-reconnect.md),
  [verification](docs/verification/durable-reconnect-m13-m14-20261027.md).
- **Commands/results:** full required validation above; exact source manifests;
  actual held H=3 snapshot while answer/tool commits, four actual frontends,
  restored draft/detail/identical scroll markers, same host identities and one
  canonical input each. Corrupt/unsupported state refuses without execution.
- **Remaining limits:** fresh authoritative snapshot across process replacement,
  delta only from an applied in-process view; bounded autosave window for view
  preferences, no control/activity bridge or relaxed unresolved-operation guard.

#### M1.5 — Keep execution healthy under a stalled or unauthorized client

- **Status:** complete for authenticated local transport; [implementation, measured bounds, validation/install and private-tmux evidence](docs/verification/durable-adversarial-m15-20261027.md). Actual foreign-UID process testing remains unavailable under eUID 1000; deterministic UID policy and real same-UID kernel transport were tested.

- **Depends on:** M1.3 and M1.4.
- **Deliver:** enforced peer authorization and the chosen concurrent-client policy,
  bounded frames/queues, and explicit overload/disconnect/resnapshot behavior.
- **Verify:** stall a watcher while a tool emits output; a healthy client can still
  submit/control and the turn finishes. Assert configured queue/byte bounds and
  no silent loss of accepted inputs. Reject unauthorized peers, oversized frames,
  incompatible protocol versions, and disallowed competing clients before mutation.
- **Evidence:** adversarial transport tests with measured bounds and clear client
  errors; no assumption that an output cap proves memory boundedness.

#### M1.6 — Detach without stopping; stop without orphaning owned work

- **Status:** complete for the classified local ownership policy; [semantics and limits](docs/plans/durable-lifecycle-controls.md), [full validation, actual process/private-tmux proofs and atomic installation](docs/verification/durable-lifecycle-m16-20261028.md). Unknown/unsupported ownership remains refused; M2–M4 are not complete.

- **Depends on:** M1.2 and M1.5.
- **Deliver:** distinct user actions for detach, foreground abort, and deliberate
  host/session shutdown, with the reviewed background ownership semantics.
- **Verify:** detach during Bash/model work leaves it alive; foreground abort does
  not silently kill retained background work. Explicit stop drains owned work and
  releases ownership only after proof. A cleanup timeout remains visibly uncertain
  and cannot authorize a competing writer; repeat control requests are safe.
- **Evidence:** real-process lifecycle tests, terminal restoration/TUI proof, and
  documented commands/defaults so ordinary UI exit is not mistaken for shutdown.

**Milestone exit:** a TUI can be disconnected/replaced during a model call or Bash
operation and reattach without stopping work, losing accepted input, or creating
duplicates. Client failures and explicit shutdown have tested distinct semantics.

### M2 — Recovery-aware execution

#### M2.1 — Restart with accepted input and undispatched intent

- **Status:** complete for the exact initial-ready cells; [contract and limits](docs/plans/durable-recovery-entry.md), [full validation, process/admission counters, private-tmux replacement and installation](docs/verification/durable-recovery-entry-m21-20261028.md). All other unresolved states remain refused.

- **Depends on:** M0.3, M1.3, and M1.6.
- **Deliver:** a recovery entry path that reconciles durable ingress and committed
  intent under exclusive ownership, dispatching only matrix-approved initial
  states. Unsupported/unresolved cases retain refusal with useful diagnostics.
- **Verify:** restart after input sync, session persistence, and operation-intent
  commit but before dispatch. Preserve FIFO and input/operation IDs; dispatch each
  eligible item once in the recovered run. Repeated recovery cannot resurrect a
  terminal operation or allow two runtimes to own the session.
- **Evidence:** process-restart tests with dispatch counters and lease assertions.

#### M2.2 — Recover a supported read-only operation

- **Status:** complete; [reviewed ready/awaiting image-v1 cells and changed-source semantics](docs/plans/durable-image-recovery.md); [six real process cases, required both-module validation, installed byte checks and private frontend proof](docs/verification/durable-image-recovery-m22-20261028.md). Other operation/model recovery is not implied.

- **Depends on:** M2.1.
- **Deliver:** the first narrowly classified safe operation/phase (for example an
  approved image read), with interrupted-output handling and explicit semantics
  if its external source changed. Expand image/skill coverage only per the matrix.
- **Verify:** crash before read, after read but before result commit, and after
  result commit. The unfinished safe read may repeat; committed results do not.
  Missing/changed files and unknown versions produce the documented outcome,
  never an invented successful result or a blanket replay rule for skill use.
- **Evidence:** end-to-end operation/tool-result recovery tests for every enabled
  version/phase and fixtures proving existing completed history still reads.

#### M2.3 — Recover an interrupted model request without replaying partial tools

- **Status:** complete; [durable two-attempt regular request policy and strict partial-tool fences](docs/plans/durable-model-request-recovery.md); [real provider/process cuts, hard-stop gap regression, both-module validation, installed byte checks and private frontend proof](docs/verification/durable-model-request-m23-20261028.md). Compaction remains separate M2.4 work.

- **Depends on:** M2.1.
- **Deliver:** persisted request identity/attempt accounting and an explicit bounded
  interrupted-request policy, with partial output distinguished from completion.
- **Verify:** crash mid-answer and after response commit. Retry only when policy
  permits, record interruption and usage that is actually known, and never execute
  tool calls from an incomplete/failed response. Exhausted retries stop visibly;
  a completed response is replayed from history rather than requested again.
- **Evidence:** fake-provider restart tests counting requests, committed responses,
  dispatched tools, and retry exhaustion. Document possible duplicate billing.

#### M2.4 — Recover compaction without losing the working conversation

- **Status:** complete for the reviewed bounded summary policy; [contract](docs/plans/durable-compaction-recovery.md), [coordinator/process fault cases, both-module validation, installed byte checks and private frontend proof](docs/verification/durable-compaction-shell-m245-20261028.md). Unknown/legacy unanswered summaries remain refused.

- **Depends on:** M2.3.
- **Deliver:** recovery of compaction intent/outcome with existing failure limits,
  summary validation, and pending-input semantics preserved.
- **Verify:** interrupt before summary completion and after summary commit but
  before adoption. Replay adopts only a committed usable summary once; failed or
  partial summaries preserve original history and queued input. Repeated crashes
  cannot reset the retry bound or silently re-enable disabled compaction.
- **Evidence:** coordinator/context reconstruction tests across these boundaries,
  including backward reads of the existing compaction controls.

#### M2.5 — Resume after an interrupted shell without rerunning its effects

- **Status:** complete for the reviewed output-read/uncertainty cells; [contract and limits](docs/plans/durable-shell-recovery.md), [real shell sentinel/live-old-executor cases, both-module validation, installation and frontend proof](docs/verification/durable-compaction-shell-m245-20261028.md). No PID-only reattachment or generic shell replay.

- **Depends on:** M2.1.
- **Deliver:** matrix-approved Bash recovery and a model/client-visible interrupted
  or uncertain outcome. Keep execution uncertainty distinct from proven cleanup;
  require reconciliation where an old process may still be running.
- **Verify:** run a shell that appends a sentinel, then interrupt before its exit
  result commits. Recovery never appends a second sentinel by automatic replay,
  preserves available output, and reports uncertainty. Pre-launch, recorded-exit,
  and cancellation-phase cases each follow their own reviewed matrix rule.
- **Evidence:** process/artifact integration tests; no PID-only reattachment or
  assertion that a missing result proves the process is dead.

#### M2.6 — Recover a parent waiting for a subagent

- **Status:** complete for explicit independently owned background children; [contract and limits](docs/plans/durable-child-recovery.md), [real parent/child cuts, exact validation, install and private frontend replacement](docs/verification/durable-child-m26-20261028.md). Legacy/task-default/message/unknown-owner cells remain unsupported; no global drain is inferred from a retained report.

- **Depends on:** M2.3 and M2.5.
- **Deliver:** durable parent/child session linkage and result-delivery identity,
  with fresh control authority on recovery and no duplicate child creation.
- **Verify:** restart after child creation, after child answer commit, and before
  parent result persistence. Reconcile the same child and deliver one logical
  result. Stale messages/control capabilities cannot mutate the resumed attempt;
  child interruption and foreground/background abort remain explicit.
- **Evidence:** parent/child process recovery tests counting child identities and
  parent-visible results, including uncertain child cleanup.

#### M2.7 — Recover a graph with running work and an unaccepted candidate

- **Status:** complete; [strict command-only v2 recovery and real store acknowledgment before nested dispatch](docs/plans/durable-graph-recovery.md); [current both-module validation, process/reducer proofs, installation and private frontend replacement](docs/verification/durable-graph-m27-20261028.md). Unknown-dispatched/live/agent/PID-only cells stay unsupported.

- **Depends on:** M2.5 and M2.6; M2.2 for graph nodes using that operation.
- **Deliver:** graph checkpoint recovery that validates state, preserves dependencies,
  ownership claims, and explicit acceptance, and applies node-specific policies.
- **Verify:** restart with one committed accepted node, one completed-but-unaccepted
  node, and one interrupted node. Accepted work is not rerun; the candidate remains
  unaccepted and its dependents blocked; interrupted work follows its policy.
  Invalid checkpoints and stale acceptance/control attempts fail explicitly.
- **Evidence:** reducer/runtime/coordinator recovery tests following the TaskGraph
  safety skill, plus a parent-visible graph recovery scenario.

**Milestone exit:** classified work recovers under tested policies at intent,
dispatch, and unrecorded-result boundaries. Terminal work is not redispatched,
unsafe effects are not replayed, and unresolved work remains diagnosable. Any
uncovered matrix cell stays unsupported; do not declare universal recovery.

### M3 — Compatible runtime and frontend replacement

#### M3.1 — Upgrade the frontend during an active turn

- **Status:** complete; [explicit compatible install/reattach workflow](docs/plans/durable-frontend-upgrade.md); [two actual builds, incompatible real TUI, exact worker/lease/draft/view/effect and owned-join proof](docs/verification/durable-frontend-upgrade-m31-20261028.md). Runtime/host replacement remains separate.

- **Depends on:** M1.4 and M1.6; no M2 completion is required.
- **Deliver:** an explicit install/reattach workflow for a compatible new frontend,
  preserving local view state without touching the active worker.
- **Verify:** replace frontend A with B while a model or Bash operation is blocked.
  Worker identity remains unchanged, the operation completes once, and B restores
  the view/draft. An incompatible frontend fails without stopping the session.
- **Evidence:** two-build process/TUI proof with observed build and worker identities.

#### M3.2 — Upgrade an idle runtime behind attached clients

- **Depends on:** M1.6 and M0.3.
- **Deliver:** existing probe/stage/fence/retire replacement through the resident
  host, exposing candidate/running build identity and status to attached clients.
- **Verify:** preflight rejection leaves the old worker unchanged. Successful
  replacement waits for old-process exit and ownership release; inputs admitted
  during handoff survive. Delayed exit, canceled reload, and stale generation
  events cannot start a second writer or produce false success.
- **Evidence:** host/client integration tests with two compatible builds and lease,
  input identity, status ordering, and process-exit assertions.

**Status: complete (M3.2).** [Implementation/policy](docs/plans/durable-runtime-upgrade.md),
[verification](docs/verification/durable-runtime-upgrade-m32-20261028.md).
Real opt-in host/client/TUI bridge, two compatible runner artifacts, actual
protocol-999 preflight rejection, old-process exit/held-lease observations,
active-model refusal and full private frontend replacement proof. Current
`make check`, `make build`, both-module race `make test`: exact 52 packages;
tracked+untracked source before/after/current equal, both binaries atomically
installed and individually cmp checked. Named units/actual joins/exact drain
recorded. Idle only: queued work, state/startup failure and host transfer remain
M3.3–M3.5; no forced reload, migration or active shell replay is claimed.

#### M3.3 — Request replacement while recoverable work is active

- **Depends on:** M3.2 and the M2 slices for every enabled work type.
- **Deliver:** an explicit replacement request that drains or checkpoints only
  approved work under a documented policy. Non-replayable active work blocks or
  defers replacement; no generic forced reload is introduced.
- **Verify:** request an upgrade during a safe read/model request and during an
  unsafe shell. The first follows its approved recovery/drain policy; the shell
  is not killed or replayed just to upgrade. Completed candidates stay unaccepted,
  and cancellation of the upgrade does not cancel unrelated session work.
- **Evidence:** two-build upgrade tests with effect/request counters and a client
  view explaining whether replacement is queued, blocked, or completed.

**Status: complete (M3.3).** [Policy](docs/plans/durable-runtime-drain.md),
[verification](docs/verification/durable-runtime-drain-m33-20261028.md).
Explicit separate capability, `/upgrade` queued natural drain and exact
`/cancel-upgrade`; no forced checkpoint/replay, unsafe shell kill, or candidate
acceptance. Real normal and race private two-build proofs both pass13/13 cleanup,
three actual provider requests and one Bash effect; actual joins and exact unit
drain. Fixed synthetic lifecycle admission pumping a second socket reader, with
regressions/current race proof. `make check/build/test` both modules pass exact54
packages, source before/after/current equal, both binaries atomically installed
and individually cmp checked. Limits: 45second bounded natural drain, installed
candidate rechecked at handoff, lifecycle reconnect after runtime generation;
M3.4 state/candidate failure and M3.5 host replacement remain separate.

#### M3.4 — Reject incompatible state and survive candidate startup failure

- **Depends on:** M3.2 and M2.1.
- **Deliver:** session/checkpoint compatibility gates and explicit stopped/recovery
  behavior after retirement. Add migrations only for formats actually introduced,
  with preserved source fixtures and a defined crash-safe commit boundary.
- **Verify:** unsupported state is rejected without mutation; supported old sessions
  read correctly. If B fails after A exits, accepted input remains durable and
  status is stopped, not successful. If B has committed new-format state, no
  automatic downgrade starts A. Interrupted migrations recover or refuse safely.
- **Evidence:** old/new format fixtures and candidate-failure tests across preflight,
  retirement, startup, and first-write boundaries.

**Status: complete (M3.4).** [Contract](docs/plans/durable-candidate-failure.md),
[verification](docs/verification/durable-candidate-failure-m34-20261028.md).
Real resident failed-B stopped reply and accepted fenced-input recovery exactly
once; immutable historical reads and unsupported startup byte preservation;
current-format first-write/no-fallback test. No new format/migration introduced.
Full check/build/test race both, exact54 packages, source manifests equal; both
binaries atomically installed/individually cmp; actual owned private tmux proof
13/13 cleanup and exact unit drains. Explicit recovery, not active host transfer.

#### M3.5 — Replace the resident host without overlapping owners

- **Depends on:** M3.2 and M3.4.
- **Deliver:** a controlled host replacement workflow with reconnecting clients,
  explicit ingress/session lease transfer, and a chosen worker drain strategy.
  First support idle/drained replacement; active-worker transfer is not implied.
- **Verify:** replace host A with B while clients reconnect and an acknowledged
  pending input exists. Exactly one host owns ingress and one worker owns the
  session. Delayed exit, startup failure, and lock contention cannot bypass fencing;
  timed-out cleanup stays unconfirmed rather than authorizing takeover.
- **Evidence:** two-host real-process tests, accepted-input recovery assertions,
  and a documented explicit recovery procedure when replacement cannot complete.

**Status: complete (M3.5).** [Controlled workflow/limits](docs/plans/durable-host-replacement.md),
[verification](docs/verification/durable-host-replacement-m35-20261028.md).
Explicit idle retirement seals/joins admissions, actual worker wait and retained
ingress; incarnation/generation-fenced CLI, no automatic supervisor/fallback.
Real two-host delayed-exit contention/startup-failure/pending-input recovery and
actual private tmux frontend proof13/13 cleanup. Full check/build/test race both,
exact55 packages, equal source manifests; atomic both installs/individual cmp,
exact unit drains. Active-worker transfer remains unsupported.

**Milestone exit:** compatible UI, runtime, and host code each have a demonstrated
replacement path. Supported replacements preserve session/input identities and
ownership; unsupported active work remains blocked rather than silently migrated.

### M4 — Replaceable extensions and broader durable workloads

These are separately valuable capabilities, not prerequisites for declaring M1
or a classified subset of M2 complete. Keep network access opt-in and authorized.

#### M4.1 — Replace one named implementation between two calls

- **Status:** accepted 2026-10-28. [Implementation/contract](docs/plans/durable-workloads.md#trustedecho-fixed-schema-pinned-implementation); [current proof](docs/verification/durable-workloads-m4-20261028.md#m41--replace-one-named-implementation). [Tool→actual canonical commit→manager A/B scenario](harness/tool/trustedextension/integration_test.go), registry failure/bounds tests and live Echo UI pass. **Commands/results:** make check/build/test PASS; exact 57-package race union, identical source manifests, installed pair and actual owned UI waits in the report. **Remaining limits:** compiled named Echo A/B only, immutable16 lifetime generation identities/32 calls per handler; no arbitrary plugins or workspace loading.


- **Depends on:** M3.4 and review of the tool-registry/upstream contract change.
- **Deliver:** one trusted named/versioned extension with explicit implementation
  generations, wired from tool selection through operation execution and results.
- **Verify:** block a call on implementation A, install compatible B, then issue a
  new call. A finishes on A and the new call uses B. Failed installation leaves A
  selected; removal/incompatible schema changes fail explicitly rather than
  changing a recorded call's meaning. Bound retained generations/instances.
- **Evidence:** an end-to-end tool scenario asserting generation selection and
  outputs; no Go plugin loading or workspace auto-execution is required.

#### M4.2 — Restart with a pending old-generation extension call

- **Status:** accepted 2026-10-28. [Recovery/fixtures/commands/results](docs/verification/durable-workloads-m4-20261028.md#m42--pending-and-completed-old-generation-recovery), [real OS-worker tests](cmd/internal/residentcli/extension_recovery_test.go), [literal fixtures/provenance](cmd/internal/residentcli/testdata/m4-extension/PROVENANCE.md). B-selected pendingA runsA; missingA and future plan/schema/envelope refuse provider0 with unchanged bytes; completedA replays without A code. Full current 57-package race validation/install PASS. **Remaining limits:** intent-before-dispatch crash fixture, pure compiled implementations, no unsupported-state migration or arbitrary side-effect exactly-once; ownerless initial records cannot acquire foreground cancellation authority.


- **Depends on:** M4.1 and the relevant M2 recovery policy.
- **Deliver:** persisted implementation identity/checkpoint version and an explicit
  compatibility path for pending calls; record selection changes where replay
  needs them to reconstruct what the model saw.
- **Verify:** restart with B installed and an unfinished A call. Resume only with
  a supported implementation or approved migration; missing/incompatible A state
  fails visibly without silently running B. Completed A output replays unchanged,
  and unavailable old code does not trigger blind retry or fallback.
- **Evidence:** old/new extension fixtures and crash tests with pending/completed
  calls and explicit unsupported-version diagnostics.

#### M4.3 — Reattach to one independently owned execution job

- **Status:** accepted 2026-10-28. [Executor/adapter/tool contract](docs/plans/durable-workloads.md#runtrustedjob-independent-execution-not-a-remote-local-shell), [actual independent CLI/worker fault proof](cmd/internal/residentcli/job_recovery_test.go), [commands/results](docs/verification/durable-workloads-m4-20261028.md#m43--independent-executor-reattachment). Postlaunch lost acknowledgment, worker actual kill/join, samejob/effect1/output, strict stale cancellation and bounded unavailable parking PASS under current full 57-package race validation/install. **Remaining limits:** independent executor must survive; lost executor ownership parks unknown, never blind rerun;32 jobs/64KiB output.


- **Depends on:** M2.1, M2.5, and M1.5.
- **Deliver:** one concrete separately owned job adapter with durable job identity,
  verified control authority, reconnectable output, and defined submit/cancel
  reconciliation. Make submission idempotent through the executor's contract.
- **Verify:** interrupt after job submission but before local acknowledgment;
  recovery finds the same job without launching another. Output resumes without
  false completion, stale authority cannot cancel a new attempt, and executor
  unavailability remains an explicit waiting/unknown outcome with bounded retries.
- **Evidence:** fault-injected job service/host integration tests counting actual
  executions and observing output, exit, and cancellation acknowledgments.

#### M4.4 — Deliver one durable timer across restart and abort

- **Status:** accepted 2026-10-28. [Timer/receipt/delivery contract](docs/plans/durable-workloads.md#durablereminder-canonical-logical-notification), [actual-store fake-clock tests](harness/reminder/store_integration_test.go), [real worker FG/BG/restart/client test](cmd/internal/residentcli/timer_recovery_test.go), [current commands/UI/results](docs/verification/durable-workloads-m4-20261028.md#m44--timer-commit-restart-abort-and-client-reconciliation). Deadline/restart/clock movement, committed-before-notification, explicit foreground cancellation/background retention and two fresh client reconciliations PASS; live tool translation also verified. Full 57-package race validation/install PASS. **Remaining limits:** canonical logical notification only, no exactly-once third-party delivery;128 tasks/16-item overdue and completion batches.


- **Depends on:** M2.1 and M1.6.
- **Deliver:** a persisted reminder/wait task with stable identity, due-time policy,
  foreground/background ownership, and one logical notification on completion.
- **Verify:** restart before/after its deadline and after notification commit but
  before client delivery. Reconcile one notification, honor explicit cancellation,
  and preserve background tasks through ordinary foreground abort. Use a fake
  clock to test clock movement and overdue scheduling without sleeps.
- **Evidence:** task-to-client restart tests with bounded overdue batching and
  stated external-delivery semantics; do not promise exactly-once third-party sends.

#### M4.5 — Run and recover one operation in a remote environment

- **Status:** accepted 2026-10-28. [Explicit TLS/authorization review and contract](docs/plans/durable-workloads.md#explicit-remote-environment-over-tls13), [network-loss/worker test](cmd/internal/residentcli/remote_recovery_test.go), [current real UI/runtime replacement and commands/results](docs/verification/durable-workloads-m4-20261028.md#m45--explicitly-authorized-remote-environment). Verified TLS1.3/scoped controller/token/environment refusal, network loss/duplicate submit/effect1/outputreference, no local fallback/credential persistence, actual UI detach/runtime death/new host reattach PASS; full 57-package race validation, atomic both install/individual cmp PASS. **Remaining limits:** controlled loopback remote environment, administrator-trusted immutable named jobs,32 connections/64KiB retained output; no cloud provisioning, sandbox or general artifact service.


- **Depends on:** M4.3 and authorization/transport review for the chosen environment.
- **Deliver:** a single useful remote job (for example a test run) using explicit
  trusted environment selection, scoped credentials, and the reattachment contract.
- **Verify:** submit from a local client, disconnect both UI and runtime transport,
  and reattach to the same remote execution. Preserve environment/job identity,
  output/artifact references, and cancellation authority. Reject unauthorized or
  mismatched environments; remote loss must not silently rerun work locally or
  expose credentials in persisted checkpoints/client snapshots.
- **Evidence:** controlled local-host/remote-executor integration scenario with
  network-loss, duplicate-submit, output bounds, and unavailable-executor checks.

**Milestone exit:** each implemented capability has its own end-to-end evidence.
Replacing implementations preserves old-call meaning; jobs, timers, and remote
work recover only within their explicitly supported contracts. This does not
promise all extensions or external systems have identical durability semantics.

## Verification and progress discipline

### Definition of done for every slice

- [x] Prerequisites have current evidence; affected interface/invariant changes
      were compared with local upstream `a5f3fd13032737142916523ae4344c392292f9d5`
      (no fetch) and divergence surfaced for review.
- [x] The stated scenario passes through the real affected layers, with fake
      providers/clocks/executors only at explicit external boundaries.
- [x] Success and the listed failure paths have deterministic assertions for
      identity, persisted state, effect count, ownership, and user-visible status
      as applicable. A zero exit code alone is not the acceptance oracle.
- [x] New behavior has bounded resources, explicit unsupported cases, compatible
      session reads, and no broadened acknowledgment or exactly-once claims.
- [x] Required repository validation passes. Code changes are rebuilt/reinstalled;
      TUI changes have the required fake-model tmux proof. Documentation-only
      slices do not require binary builds.
- [x] This roadmap links the implementation, tests, exact verification commands,
      observed results, and remaining limitations before a slice is marked done.

Use focused lifecycle/fault-injection tests around durable writes and external
effects. A process-kill test alone does not establish power-loss durability;
record the tested storage/failure boundary. Never remove conservative resume
guards merely to make a demo pass.

Record completion under the relevant slice using: `Status`, `implementation/test
links`, `commands/results`, and `remaining limitations`. Revalidate evidence after
relevant changes; a plan, worker claim, or old test result is not current proof.

**Current accepted roadmap: 27/27 defined slices.** M0.1–M0.4, M1.1–M1.6,
M2.1–M2.7, M3.1–M3.5 and M4.1–M4.5 are earned. [Final current M4 evidence,
validation/installation, private actual frontend/runtime/executor waits and limits](docs/verification/durable-workloads-m4-20261028.md).
M3.4/M3.5 were not treated as roadmap completion. All five M4 capabilities are now
wired and individually evidenced, not accepted from isolated foundation tests.
This closes the 27 stated vertical slices, not unsupported ownership, arbitrary
plugins, automatic cloud execution, power-loss certification or exactly-once
third-party delivery. The durable session remains the authority; its process is
replaceable only at the explicitly tested/classified boundaries.
