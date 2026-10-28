# M0.3: recovery by durable version and execution phase

Baseline: `d24507b7789652736a3f9e3d89e37abcc22d046c`, plus the M0.1–M0.3
characterization tests. This records **historical baseline behavior**, not current replay permission.
Later reviewed cells below supersede only their exact baseline cells. [Roadmap](../../goal.md), [submission boundaries](durable-submission-lifecycle.md),
[lifecycle boundaries](durable-lifecycle-boundaries.md), and
[validation record](../verification/durable-recovery-m03-20261027.md).

## Reviewed overrides after M0.3

This original matrix is retained as evidence of the starting behavior, not a
claim that every row below is still enabled or unchanged.

- **Accepted M2.1:** [exclusive narrow entry recovery](durable-recovery-entry.md)
  allows exact value-v1 ready and approved image-v1 ready state; every other
  unresolved operation/ownership fence remains independent.
- **Accepted M2.2:** [image-v1 ready/awaiting, valid no-result state](durable-image-recovery.md)
  rereads current source. Changed source may change the result; missing/corrupt
  source fails, committed results never reread. Skill/shell/remote are not enabled.
- **Accepted M2.3:** [versioned bounded regular-request ledger](durable-model-request-recovery.md)
  supersedes unbounded unanswered-turn recovery only for charged regular requests.
  Limit 2, explicit interruption/exhaustion, no partial/failed tool authority;
  uncharged legacy unanswered regular/compaction turns refuse. Completed legacy
  history remains readable. [Full validation, installed byte checks and private frontend proof](../verification/durable-model-request-m23-20261028.md) passed.

- **Accepted M2.4:** [separate bounded compaction policy](durable-compaction-recovery.md)
  preserves summary kind, manual origin and global failure budget across losses;
  exhaustion preserves ordinary input and original conversation.
- **Accepted M2.5:** [strict shell-v3/v4 recorded-exit output-read cells](durable-shell-recovery.md)
  never restart the command. Ready/preparation/process/canceling remain uncertain
  and fenced. Neither terminal tickets nor PID observations prove global cleanup.
  [Combined full validation/install/private-frontend evidence](../verification/durable-compaction-shell-m245-20261028.md).

- **Accepted M2.7:** [opt-in command-only taskgraph run/control2](durable-graph-recovery.md), strict checkpoint2 and coordinator store-ack-before-nested-dispatch. Accepted generations and unaccepted candidates retain meaning; proven terminal children settle without replay; unknown-dispatched/live/agent/PID-only cells stay refused. [Current validation/install/process/private frontend proof](../verification/durable-graph-m27-20261028.md).
- **Accepted M2.6:** [opt-in subagent.run/v2, explicit background lifetime](durable-child-recovery.md), stable child/input/result identity and fresh parent Hub authority. Retained-owner reconciliation is permission, not global cleanup; legacy/task/default/message/unknown-owner cells stay refused. [Current validation/install/private frontend proof](../verification/durable-child-m26-20261028.md).

## Read this first: capability is not permission

**At the M0.3 baseline, the resident worker rejected every restored nonterminal
operation before history transfer or provider setup.** This includes read-only and pure operations.
See [`workerruntime/run.go`](../../cmd/internal/workerruntime/run.go),
`TestResumeRefusesUnknownOperationBeforeProviderSetup` in
[`run_test.go`](../../cmd/internal/workerruntime/run_test.go), and the new
[`TestResumeRefusesShellIntentAcrossUncommittedExecution`](../../cmd/internal/workerruntime/recovery_boundary_test.go).
The coordinator and individual handlers have lower-layer resume capabilities;
those were **not** an enabled resident-worker recovery path at that baseline. In particular,
`TestCoordinatorRunDispatchesRestoredNonTerminalOperations` in
[`loop_test.go`](../../harness/coordinator/loop_test.go) does not supersede the
worker's guard.

Classifications used below:

- **Local retry:** reconstruction can continue without repeating an already
  committed logical history/result. This does not promise exactly-once external
  execution, identical rebuilt requests, or absence of duplicate provider billing.
- **Effect-safe reread / pure recomputation:** no new mutating external effect,
  but repeatable content can require immutable sources/artifacts.
- **Interrupted / unknown:** durable state cannot establish the external outcome;
  do not redispatch blindly. A live-manager deduplication key is not a durable
  cross-crash idempotency key.
- **Reattach:** requires independently owned work, durable executor identity,
  authority and observable output/drain. **No generic reattach path is established
  for these current local shell, model, or subagent processes.**
- **Blocked:** unsupported versions, unresolved resident-worker operations, or
  cells lacking the evidence needed to enable automatic replay. Source-supported
  lower-layer behavior is labeled separately from executable crash evidence.

The coordinator persists top-level operation intent atomically with tool-call
status **before** manager admission. Intermediate manager/handler updates are
asynchronous: emitting or receiving a checkpoint does not itself prove that
`SaveOperation` committed it. Sources: [`coordinator/loop.go`](../../harness/coordinator/loop.go),
[`operation/local_manager.go`](../../harness/operation/local_manager.go),
[`localfile/store.go`](../../harness/sessionstore/localfile/store.go).

## Supported formats and compatibility boundaries

| Persisted boundary | Supported now | Explicit limits / evidence |
| --- | --- | --- |
| Localfile session | Header **v2** | v1 is explicitly rejected, not migrated; other header versions fail. `TestStoreRejectsLegacySessionResume` in [`store_test.go`](../../harness/sessionstore/localfile/store_test.go); `TestDecodeLogRejectsInvalidRecords` in [`codec_internal_test.go`](../../harness/sessionstore/localfile/codec_internal_test.go). |
| Model / compaction | Regular and compaction turns/responses inside session v2; no separate persisted request-attempt version | `Turn` stores ID, previous-turn ID and type, not request body or a provider dispatch identity. [`session.go`](../../harness/session/session.go), [`sessionstore.go`](../../harness/sessionstore/sessionstore.go). |
| Relevant inbox controls | `settings`, `compact`, `compaction_policy`, `compaction_disabled`; no independent numeric control version | Strict modes/parameters, not a migration protocol. `TestCompactionControlsRoundTripAndBounds` in [`compaction_control_test.go`](../../harness/inbox/compaction_control_test.go). Structural control-operation record is separately **v1** within session v2; it is not a model checkpoint version. |
| Shell / Bash | `shell` **v3**, **v4** | v4 permits a serialized wall budget; v3 with nonzero budget is rejected. [`shell.go`](../../harness/operation/shell.go), [`tool/bash`](../../harness/tool/bash/bash.go), [`budget_test.go`](../../harness/tool/bash/budget_test.go). Older/newer shell versions are not current supported decoders. |
| Image, skill, pure value | `view_image` **v1**, `skill_use` **v1**, `value` **v1** | Exact type/version checks in [`image.go`](../../harness/operation/image.go), [`skill_use.go`](../../harness/operation/skill_use.go), [`value.go`](../../harness/operation/value.go). |
| Remote jobs | `remote_job` envelope **v2** | Plan has its own exact type/version; opaque handle has handler-specific semantics. Unsupported envelope and missing/duplicate handler fail; envelope version alone grants no replay contract. [`remote_job.go`](../../harness/operation/remote_job.go), [`remote_job_handler.go`](../../harness/operation/remote_job_handler.go). |
| Built-in remote plans | **v1** each: `subagent.run`, `subagent.message`, `subagent.control`, `taskgraph.run`, `taskgraph.control` | These are all built-in plan types, not all possible extension handlers. Subagent `RunHandle` has no version field; historical SessionID-only handles are read. Graph owner handle checkpoint **v1**, nested graph **v1**; unknown versions reject. [`subagent.go`](../../harness/tool/subagent/subagent.go), [`control.go`](../../harness/tool/subagent/control.go), [`taskgraph/protocol.go`](../../harness/taskgraph/protocol.go). |

Backward-read support is specific, not universal. Legacy graph-v1 attempts lacking
phase diagnostics conservatively retain dispatch uncertainty. Additive JSON fields
do not prove older binaries preserve new semantics: **no two-build downgrade or
forward-compatibility proof** was added here. Unsupported state remains blocked;
serializable is not synonymous with resumable.

## Model requests and compaction (session v2)

Sources: [`coordinator/loop.go`](../../harness/coordinator/loop.go),
[`coordinator/compaction.go`](../../harness/coordinator/compaction.go),
[`contextbuilder/compaction.go`](../../harness/contextbuilder/compaction.go).
Model rows assume no unresolved nonterminal operation triggers the worker fence.

| Latest durable phase | Classification and current resume behavior | Durable identity/output and evidence |
| --- | --- | --- |
| Input committed; regular turn not committed | **Local retry.** Pending input can build a fresh request. Build/turn-write failure stops before provider call. | Stable input ID; no provider-attempt identity. `TestCoordinatorRunDoesNotCallModelWhenTurnStoreFails` [M1], `TestCoordinatorResumesUnansweredInput` [M2]. |
| Regular turn committed, truly before dispatch | **Locally retryable**, but the durable record does not distinguish this from the next row. Replay leaves input pending and schedules a **new turn ID** with current builder/settings. | Turn ID is history identity, not a provider idempotency key. [M2]; real restarted-worker request count [B2]. |
| Provider dispatched / midstream / external response generated, but no response commit | **Unknown external outcome, non-idempotent provider attempt.** Current model-only restart may issue another provider request. No reattach, saved live token deltas, cross-crash attempt budget or dispatch marker. | All three windows can have the same turn-only record. [B2] demonstrates interrupted real-worker model request/restart; adapter stream tests [M7] demonstrate only in-process retry. Exact provider-completed-before-commit process-kill cell is **untested**. |
| Provider Go error, no response commit | **Unknown external outcome.** Run returns error; later replay can treat original input as unanswered. | No durable failed-attempt accounting; terminal response failures are a different row. Source-supported, not a process-crash retry-budget proof. |
| Successful regular response committed, tool status not yet committed | **Local history replay.** Input is answered; pure translation may reconstruct unscheduled calls. Status/operation intent must commit before dispatch. | Complete response and call IDs are durable; no repeat request solely for that answered input. Ordering/failure tests [M1], recovery tests [M3]. |
| Regular response committed with `Failure != nil`, empty stop | **Durable terminal failure.** Replay returns typed failure without unsolicited retry; new external input can permit continuation. Failed output/tools are suppressed. | Complete failure/known usage survives; legacy associated operations are not revived. [M6]. |
| Refusal committed (`StopRefused`, including failure metadata), or nonfailed terminal partial response | **Durable terminal answer/partial output**, not a resumable stream. Refusal text/usage survives; follow-up input can continue. Incomplete terminal responses are not adapter-retried. | Do not claim all incomplete-response tools are filtered: failed responses, unfinished individual calls and compaction tools have distinct rules. [M6], [M7]. |
| Terminal operation saved, tool-result/history continuation not fully saved | **Local reconciliation, no terminal redispatch.** Restore missing tool-call result/status, then continue model if needed. Settled work is excluded from resume operations. | Operation ID, terminal state and tool-call ownership. Completed/failed/canceled cases with localfile reopen and a second no-work resume [M3]. |
| Compaction policy committed, turn not committed | **Local retry.** Recorded positive failure limit overrides changed runtime default. Turn-write failure causes no provider call. | Policy control is durable; no attempt dispatch record. [M4], [M5]. |
| Compaction turn committed, no summary response | **Unknown provider outcome; original context preserved.** With pending input, replay reevaluates compaction eligibility and may submit a new summary request/turn. | Pending ordinary input remains undelivered. No durable interrupted-attempt budget. Turn-only/reopen tests [M4]. |
| Usable summary response committed, builder not yet reconstructed | **Local deterministic reconstruction.** Adopt committed summary rather than request it again. Preserve staged/pending ordinary work. | Summary response plus compaction-turn boundary; only complete, nonfailed, nonblank assistant text with eligible history is usable. Last nonblank assistant message is selected, independent of phase label. [M4], [M8]. |
| Unusable summary response committed | **Bounded logical retry**, not provider idempotency. Preserve original context; reconstruct consecutive unusable-summary count. At limit, persist disable notice and continue ordinary work. | Persisted policy, committed unsuccessful responses, disable control. Default limit 3; interrupted/uncommitted attempts do not consume that durable count. [M5]. |
| Failure limit reached, disable-control append fails | **Local checkpoint recovery.** Stop; on replay derive failure count and commit disable control before extra summary submission. | Policy/response records suffice to reconstruct missing disable notice. [M5]. |
| Automatic compaction disabled; later manual compact | Ordinary pending work proceeds without automatic compaction. Manual compact can bypass disabled automatic mode; successful summary clears failures/re-enables it. | Replayed disable/policy controls plus successful summary. [M5]. |
| Manual compact input committed, no turn | **Local one-shot scheduling**, if history eligible. No eligible history means no summarizer call. | Pending manual control. `TestManualCompactionWithoutHistoryDoesNotCallModel` [M6]. |
| Manual-only compaction turn committed, no summary and no pending ordinary input | **Source-inferred gap, untested.** Turn replay consumes manual trigger; `NeedsCompaction` alone does not schedule work, so automatic retry is not established. | Do not generalize pending-input compaction recovery tests to this window. Keep it explicit for future recovery policy. |

Provider retry inside one `Respond` is distinct from coordinator restart:
adapter `MaxAttempts` defaults to 5; failed stream generations are discarded.
Request/cache affinity is **not** an idempotency key. A complete terminal response
can durably contain partial text and known usage, but live deltas and failed
intermediate generations are not session checkpoints. Persistence failure ends
that coordinator Run; recovery constructs a new coordinator from committed state.

## Local operations: status and phase

All nonterminal rows below remain **blocked by the resident-worker fence**, even
when a reducer can safely continue in isolation. Terminal states are
`completed/failed/canceled`; they are reconciled, not executed again [M3].
`canceling` means cancellation requested, **not** proof of process/primitive drain.

| Type/version and persisted phase | Lower-layer behavior / classification | Durable state required; evidence and limits |
| --- | --- | --- |
| Shell v3/v4, `ready` | Initializes capture directory/files and can launch command. **Not cross-crash safe merely because status says ready**: updates may lag a real completed effect. | Input command/shell/directory, base artifact directory, operation ID, output limit; v4 additionally budget. New boundary test [B1] proves same ready state at three distinct effect boundaries. |
| Shell v3/v4, awaiting `create_directory/create_out/create_err` | Re-dispatches capture setup. These are preparation phases, not proof a command never launched if the durable checkpoint lags. No universal idempotency claim about artifact creation. | Same artifact namespace and unambiguous launch boundary would be needed for safe enabled recovery. Reducer phase tests [O1]; nested checkpoint caveat below. |
| Shell v3/v4, awaiting `process`, PGID = 0 | **Unknown process-start outcome.** Reducer fails without redispatch. | Missing recorded start is not proof of nonexecution. `TestShellActorFailsUnknownProcessOutcomeWithoutRestarting` [O1]. |
| Shell v3/v4, awaiting `process`, PGID recorded but no exit | **Interrupted.** Reducer fails without restarting and preserves PGID; not process reattachment or successful drain. | Recorded PGID is observational state, not a durable control capability. `TestShellActorPreservesRecordedProcessWhenFailingRecovery` [O1]; actual death [B2]. |
| Shell v3/v4, awaiting `read_out/read_out_tail/read_err/read_err_tail`, exit recorded | **Effect-safe output reread**, assuming surviving unchanged capture artifacts. Rebuilds bounded inline/tail output without rerunning command. Missing exit is rejected. | Recorded exit/signal status, artifact paths/sizes, output limit and truncation flags. `TestShellActorResumesOutputReadWithRecordedExitStatus`, `…PersistsSignalExitStatusBeforeReading`, `…RereadsAndReplacesInline`, `…RequiresRecordedExitStatus`, `TestShellRereadsAndReplacesTail` [O1]. No physical-crash proof for every read/tail phase or v4 budget combination. |
| Shell v3/v4, `canceling` | Terminal cancellation reducer path; live primitive/process ownership must separately drain. Not permission to kill a process found only by recycled PID. | PGID may remain recorded. Cancellation/ownership tests [O1], [B2]; nil host Close alone does not prove graceful execution cleanup. |
| Image v1, ready or awaiting read/compute | **Effect-safe reread/recompute**, not reattachment. Only path/config/result are serialized; chunks, read cursor and compute-in-progress state are in memory. Restored awaiting image rereads from the beginning, even after prior compute dispatch. | Requires same source contents for repeatable result; no source digest/snapshot identity. Compute is cancellable/owned, not durable CPU execution. `TestViewImageDispatchesComputeAndRecoveryRereads`, `…CancellationDoesNotDispatchCompute`, `…ComputeFailuresTerminateWithoutRedispatch` [O2]. |
| Image v1, canceling or terminal | Cancellation does not launch compute; terminal result is reused rather than recomputed. | Result includes encoded image and metadata/error. [O2]; no process-crash matrix over all image read/compute windows. |
| Skill v1, ready or awaiting partial read | **Effect-safe reread continuation**, not immutable snapshot recovery. Resume read offset is `len(Content)`; subsequent bytes append. | Path and saved bytes; changing source can splice different versions. `TestSkillUseOperationResumesPartialRead` [O3]. No content identity fence or changed-source crash test. |
| Skill v1, canceling or terminal | Cancellation is terminal; completed content is reused. | Content/terminal error and status. [`skill_use.go`](../../harness/operation/skill_use.go), [O3]. |
| Value v1, ready/awaiting/canceling | **Pure, idempotent local recomputation:** complete/cancel from saved JSON with no primitive dispatch. Still blocked by worker if nonterminal. | Valid inline JSON is the complete result. `TestAdvanceValueCompletesWithoutPrimitiveDispatch`, `TestDecodeValueRejectsOtherTypesAndVersions` [O4]. |
| Any unsupported/malformed type/version/state | **Blocked, fail explicitly**, never substitute an unversioned default decoder. Resident nonterminal fence can refuse before type-specific decoding. | Exact versioned state and handler registration are necessary, not sufficient, for recovery. [O1–O5], graph validation [R4]. |

## Remote jobs, subagents and TaskGraph

The `remote_job/v2` envelope serializes plan `{Type,Version,Data}`, handle,
next-inspection time, outstanding input/subscription state, terminal result/error
and artifact paths/byte counts/truncation flags. A custom handler must define its
own recovery policy. There is **no envelope-level idempotency or reattachment**.
Handler routing checks exact plan type/version and immutable identity; manager
and handler admission deduplication applies only during their current lifetime
[O5]. Result sidecars must survive at their recorded paths; serialization alone
does not make them relocatable or immune to storage loss [O6].

**Nested intent is not durably acknowledged before launch.** TaskGraph calls
`save` before child manager `Add`, but `save` enqueues a parent update; it does not
wait for session-store acknowledgement. Subagent preparation similarly enqueues
its handle update, then invokes launch continuation. See
[`taskgraph/scans.go`](../../harness/taskgraph/scans.go),
[`subagent/io.go`](../../cmd/internal/subagent/io.go),
[`subagent/supervisor.go`](../../cmd/internal/subagent/supervisor.go) and the
explicit enqueue-boundary caveat in [`subagent/README.md`](../../cmd/internal/subagent/README.md).
A durable explicit-prelaunch checkpoint can therefore lag a real launch. Tests
restoring a chosen checkpoint establish reducer behavior, **not crash-safe
exactly-once nested dispatch**. This is a baseline gap, not an invariant changed
by M0.3. The top-level coordinator commit-before-dispatch rule remains intact.

Every nonterminal owner below is also blocked by the resident-worker fence.

| Plan/version and phase | Lower-layer recovery behavior / classification | Durable identity/output and limits |
| --- | --- | --- |
| `subagent.run/v1`, no handle / prelaunch | Allocates child session and enqueues handle before launch; **unknown launch outcome** across parent crash because there is no persistence acknowledgement/launch discriminator. | `RunHandle{SessionID,Handle,Generation}`; SessionID is durable history identity, not proof old runner is absent. Preparation ordering/cancellation [R1], not parent-crash effect counting. |
| `subagent.run/v1`, handle recorded, active/interrupted | Supervisor can start **another process** against recorded child session with restart prompt and fresh capability/generation. **Not reattachment**, not an enabled safe parent-crash path. | No durable proof old process drained. Session/history may survive while old process/terminal result status is uncertain. Historical-handle read/fencing [R1]; successful interrupted-child process restart is untested here. |
| `subagent.run/v1`, child report/final outcome before parent commit | A report is not drain. Normal supervisor lifecycle waits/reaps/readers and delivers terminal owner update; a lost parent result commit remains uncertain. | Child history, terminal envelope reply/error, parent tool-call ownership. Clean exchange [R1], generic terminal-result reconciliation [M3]. No parent-crash-after-child-answer proof. |
| `subagent.message/v1`, queued/written/not acknowledged | **Unknown delivery across crash.** Live supervisor assigns random input ID; pending IDs/queues are not durable handle dedup keys. Written bytes are not persisted input or model consumption. | Current target registry and matching child input item. Clean-exit reconciliation preserves unacknowledged IDs/order within same supervisor; failed/stopping exits do not replay. No durable cross-crash exactly-once messaging. [R1]. |
| `subagent.message/v1`, persisted acknowledgment / resumed idle child | Matching persisted child input establishes delivery barrier. Idle-child message can own resumed runner and finish with its reply. New supervisor cannot target an unrestored named child automatically. | Child session/input ID and result; capability/generation are scoped, not names. [R1]; parent-crash acknowledgment/result windows untested. |
| `subagent.control/v1`, list/status/message/stop | `list/status` are observations; `message` has the uncertainty above; `stop` usually means cancellation requested, not drained. Old/cross-Hub handle is rejected; a new Hub does not revive old authority. | Exact opaque handle + generation; outcome can record delivery, state, drain confirmation and snapshot. No generic durable control transaction/replay. [R1]. |
| `taskgraph.run/v1`, checkpoint v1/graph v1, pending / explicit `input_validation` and `!CommandDispatched` | Revalidate then dispatch, or cancel without launching. **Only a lower-layer checkpoint interpretation**: nested enqueue gap above prevents a crash-safety claim from this flag alone. | Owner operation ID, declarations, generation, claims and validation fingerprints. Explicit-prelaunch/invalid-state tests [R2], identity checks [R4]. |
| Same, running/execution or legacy phase `""`, uncertain shell launch | Legacy empty phase is conservatively dispatched. Non-output-collection shell phases retain unknown-outcome reservations; **no blind relaunch**. Proven process absence can resolve reservation as canceled/failed-unknown, not success. | Child ID derived from owner/task/generation, child operation/PGID, claims retained. [R2]; process observation is not kill authority. |
| Same, running shell `read_out/read_out_tail/read_err/read_err_tail`, recorded exit | Can resume output collection, **not command execution**, subject to shell artifact requirements. | Recorded shell child and exit/artifacts. [O1] plus source restore routing in [`runtime.go`](../../harness/taskgraph/runtime.go). No full parent-process-kill proof per tail phase. |
| Same, active agent child | **Unknown process outcome; retain reservation.** No proven runner reattachment. | Recorded child operation/session is insufficient authority/liveness proof. Source-supported restore refusal; active-agent restoration process test is **untested**. |
| Same, child terminal, `output_validation` | Validate outputs and produce candidate without reexecuting terminal child. | Child terminal result, before/after fingerprints, claims and generation. [R2–R4]; combined parent-crash scenario remains untested. |
| Same, completed / `awaiting_acceptance` | **Completed candidate, not accepted.** Claims remain until explicitly decided. Coordinator/graph completion cannot be inferred from a child exit or report. | Exact current generation + evidence + fresh scan needed for acceptance. Candidate/claims tests [R3] are not physical-crash restoration proof. |
| Same, accepted | Preserve acceptance only while relevant inputs/outputs/prerequisites remain fresh. Resume rechecks even an all-accepted graph before terminal success; changes invalidate generations and can make work runnable again. | Checkpointed fingerprints, acceptance evidence, graph declarations and ownership. [R3]. This policy is not an exactly-once effect guarantee after invalidation/retry. |
| Same, canceling/canceled/failed | Request cancellation, keep owned work/claims until drain/uncertainty is resolved. Cancellation timeout is not termination. | Persisted attempt/status plus live ownership observations. [R2], [R4]. |
| `taskgraph.control/v1`, status/revise/accept/reject/invalidate/retry/cancel | Acts on current runtime's named graph; does not reconstruct graph from control handle. Status is pure snapshot. Acceptance requires fresh completed generation, evidence and scan. **No cross-crash control idempotency/atomicity guarantee.** | Graph must exist; mutation may precede owner-checkpoint/control-result commits. Transient pending scans/queue not a durable transaction. Fresh-generation tests [R3]; mutation/owner/result crash window untested. |

## Evidence index (exact tests, not implied crash coverage)

### New boundary evidence

- **[B1]** [`recovery_boundary_test.go`](../../cmd/internal/workerruntime/recovery_boundary_test.go),
  `TestResumeRefusesShellIntentAcrossUncommittedExecution`: shell **v3 and v4**,
  each at (1) intent committed only, (2) real process start observed then canceled
  behind an effect gate, (3) real effect and successful terminal manager result,
  deliberately **not committed**. A lossy persistence driver drops updates; this
  is a controlled crash-window simulation, **not a SIGKILL test**. Driver drains
  before fresh-store recovery. All cases restore the same ready intent; effects
  are respectively **0, 0, 1**. Worker refuses before credentials/provider/history
  transfer, leaves session bytes unchanged, preserves effect count and releases
  its lease. It does not prove that a ready operation may safely auto-retry.
- **[B2]** Actual Linux process death and model restart are in
  [`lifecycle_boundaries_linux_test.go`](../../cmd/internal/workerhost/lifecycle_boundaries_linux_test.go):
  `TestRealFrontendProcessLossExposesInterruptedShell`,
  `TestRealWorkerUnexpectedDeathRetainsHostOwnershipAndRecoveryBoundary`, and
  `TestRealWorkerCloseDeadlineRetainsHostLeaseAfterWorkerExit`.
  [M0.2 evidence](../verification/durable-lifecycle-m02-20261027.md) records
  pidfd identity/exit, one sentinel effect without replay, surviving orphan shell
  under frontend loss, retained host ownership, and additional model request on
  explicit interrupted model-only restart. Linux fixture is not a live-TUI attach
  proof or all-operation physical-crash test.

### Model / history evidence

- **[M1]** [`loop_test.go`](../../harness/coordinator/loop_test.go):
  `TestCoordinatorRunDoesNotCallModelWhenTurnStoreFails`,
  `TestCoordinatorHandlesModelResponseBeforeSchedulingToolCalls`,
  `TestCoordinatorDoesNotScheduleToolCallsWhenModelResponseStoreFails`,
  `TestCoordinatorRunPersistsToolCallBeforeDispatch`.
- **[M2]** [`recovery_test.go`](../../harness/coordinator/recovery_test.go):
  `TestCoordinatorResumesUnansweredInput` (input/turn/response/compaction phases).
  Synthetic durable-phase fixtures, not provider-completed process-kill tests.
- **[M3]** Same file: `TestCoordinatorResumesSavedTerminalOperation`;
  [`recovery_sequences_test.go`](../../harness/coordinator/recovery_sequences_test.go):
  `TestCoordinatorRetriesRecoveryPersistenceFailures`,
  `TestCoordinatorRetriesInitialToolStatusPersistenceFailure`;
  [`loop_test.go`](../../harness/coordinator/loop_test.go):
  `TestCoordinatorReconcilesToolCallsFromPersistedOperationUpdates`.
- **[M4]** [`compaction_test.go`](../../harness/coordinator/compaction_test.go):
  `TestCompactionReconstructionMatchesLocalFileReplay`,
  `TestCompactionResponseStatusOnLiveAndReplay`,
  `TestCompactionPersistenceFailureStopsAndRetriesOnResume`;
  [`compaction_scheduling_test.go`](../../harness/coordinator/compaction_scheduling_test.go):
  `TestCompactionCrashContinuation`, `TestCompactionTurnPersistenceFailureDoesNotSubmit`.
- **[M5]** [`safety_test.go`](../../harness/coordinator/safety_test.go):
  `TestCompactionBoundNoticeReplayAndManualRecovery`,
  `TestCompactionConfiguredLimitAndDuplicateResponse`,
  `TestCompactionControlPersistenceFailuresRecoverWithoutExtraSummary`.
- **[M6]** Same file: `TestRegularFailureDurableTypedAndNoToolScheduling`,
  `TestFailedReplayRequiresNewInputAndIgnoresLegacyOperations`,
  `TestRefusalWithFailureLiveReplayAndFollowup`,
  `TestDuplicateSuccessfulResponseDoesNotReviveCompletedTool`,
  `TestManualCompactionWithoutHistoryDoesNotCallModel`.
- **[M7]** [`responsesapi/stream_test.go`](../../harness/llm/responsesapi/stream_test.go):
  `TestResponsesStreamRetryDiscardsPartialAttempt`, `TestResponsesTerminalStatuses`,
  `TestResponsesPreservesTerminalAfterDisconnect`;
  [`exchange_retry_test.go`](../../harness/llm/responsesapi/exchange_retry_test.go):
  `TestExchangeIncompleteResponsesAreNotRetried`;
  [`response_test.go`](../../harness/llm/responsesapi/response_test.go):
  `TestResponseIgnoresUnfinishedToolCall`;
  [`messagesapi/exchange_test.go`](../../harness/llm/messagesapi/exchange_test.go):
  `TestAdapterRetriesInterruptedStreamWithoutLeakingPartialCalls`,
  `TestAdapterStreamFailureDoesNotReturnPartialOutput`.
- **[M8]** [`contextbuilder/reconstruction_test.go`](../../harness/contextbuilder/reconstruction_test.go):
  `TestCompactionUsesLastAssistantMessage`, `TestCompactionSelectionStaysStableAcrossStagedArrivals`;
  [`contextbuilder/safety_test.go`](../../harness/contextbuilder/safety_test.go):
  `TestUnusableCompactionPreservesOriginal`.

### Local / remote operation evidence

- **[O1]** [`shell_test.go`](../../harness/operation/shell_test.go): exact shell tests
  named in the matrix; also `TestShellActorResumesEveryReplayablePhase`,
  `TestShellActorValidatesPersistedStateAndIdentity`,
  `TestShellActorCancellationPreservesRecordedProcess`,
  `TestShellActorCancellationStopsActiveProcess`.
- **[O2]** [`image_actor_test.go`](../../harness/operation/image_actor_test.go):
  exact image tests named above. These are actor/primitive tests, not process death.
- **[O3]** [`skill_use_test.go`](../../harness/operation/skill_use_test.go):
  `TestSkillUseOperationResumesPartialRead`, `TestSkillUseOperationReportsReadFailure`.
- **[O4]** [`value_test.go`](../../harness/operation/value_test.go): exact value tests
  above and `TestLocalOperationManagerDeliversCanceledValue`.
- **[O5]** [`remote_job_test.go`](../../harness/operation/remote_job_test.go):
  `TestDecodeRemoteJobStateReportsUnsupportedEnvelope`;
  [`remote_job_handler_test.go`](../../harness/operation/remote_job_handler_test.go):
  `TestLocalOperationManagerSelectsRemoteJobHandler`,
  `TestLocalOperationManagerAddsRemoteJobOnce`,
  `TestLocalOperationManagerValidatesRemoteJobUpdateIdentity`.
- **[O6]** [`remote_job_artifact_test.go`](../../harness/operation/remote_job_artifact_test.go):
  `TestRemoteArtifactsRecoveryAndReplay`, `TestRemoteArtifactReplayFromOriginalIsExclusive`.
  Artifact reread/serialization/exclusive replay, not power-loss or relocation proof.

### Subagent / graph evidence

- **[R1]** [`supervision_test.go`](../../cmd/internal/subagent/supervision_test.go):
  `TestPreparationBlockedAdmissionStopAndNoLateLaunch`,
  `TestPreparationConcurrencyButOrderedPublication`,
  `TestRestartHandleFencingAndOldSessionHandleRead` (historical handle/fencing,
  not successful interrupted-child restart),
  `TestGraphScopedControlsAndPersistedMessageBarrier`,
  `TestTaskReportCleanupAndExplicitBackground`,
  `TestBlockedParentDeliveryDoesNotBlockReapingOrControl`;
  [`lifecycle_test.go`](../../cmd/internal/subagent/lifecycle_test.go):
  `TestSupervisorWaitJoinsChildReadersAndCommandWait`;
  [`agentrunner/subagent_test.go`](../../cmd/internal/agentrunner/subagent_test.go):
  `TestRunMainSubagentsExchangeInboxMessages` (clean start/message/final/follow-up).
- **[R2]** [`runtime_test.go`](../../harness/taskgraph/runtime_test.go):
  `TestRuntimeResumeUnknownProcessRetainsReservation`;
  [`runtime_regression_test.go`](../../harness/taskgraph/runtime_regression_test.go):
  `TestRuntimeUnprovenDispatchRetainsReservationOnResume`;
  [`scans_test.go`](../../harness/taskgraph/scans_test.go):
  `TestRuntimeResumeExplicitPrelaunchCheckpoint`,
  `TestRuntimeLegacyV1WithoutDiagnosticFieldsRetainsUnknownLaunch`,
  `TestCheckpointPrelaunchRejectsNoninitialChildState`.
- **[R3]** [`runtime_test.go`](../../harness/taskgraph/runtime_test.go):
  `TestRuntimeRequiresExplicitAcceptance`, `TestRuntimeResumePendingAndAccepted`,
  `TestRuntimeFreshnessBeforeAcceptance`;
  [`graph_test.go`](../../harness/taskgraph/graph_test.go):
  `TestCompletedCandidateKeepsClaimsUntilDecided`;
  [`scans_test.go`](../../harness/taskgraph/scans_test.go):
  `TestRuntimeResumeAllAcceptedRechecksBeforeTerminal`,
  `TestRuntimeAcceptScanCannotAcceptRevisedGeneration`.
- **[R4]** [`runtime_test.go`](../../harness/taskgraph/runtime_test.go):
  `TestRuntimeUnsupportedCheckpoint`;
  [`checkpoint_identity_internal_test.go`](../../harness/taskgraph/checkpoint_identity_internal_test.go):
  `TestCheckpointDuplicateRestoreRejectedBeforeHandlerSetup`;
  [`checkpoint_validation_internal_test.go`](../../harness/taskgraph/checkpoint_validation_internal_test.go).
  Existing exhaustive graph checker is unchanged and its valid cached race-test
  evidence is reused by full repository validation.

## Explicitly blocked work / what this slice does not prove

- No change to production dispatch, replay, lease, cancellation or compatibility
  semantics. **No worker guard is relaxed.** No extension migration, executor
  daemon or attach service has been implemented by writing this matrix.
- No process-kill coverage for every version/phase combination. In particular:
  image/skill mutation during recovery, manual-only interrupted compaction,
  model completion before response commit, subagent parent death around child
  launch/result delivery, active graph-agent recovery, a restored mixed graph
  with accepted + unaccepted + interrupted nodes, and graph-control mutation
  atomicity remain explicitly unproven. Existing synthetic fixtures do not fill
  these cells automatically.
- Future enabled replay needs a persisted phase that cannot lag unsafe dispatch,
  operation/task identity and version, effect-specific idempotency or an executor
  reattachment contract, durable output/source identity, and proven ownership/drain.
  Each newly enabled cell needs its own adverse-boundary test before guard changes.
- Session JSONL append/sync, incomplete-tail recovery and append-error tests are
  not an exhaustive ambiguous-I/O or power-loss proof. No universal storage-loss,
  host-loss, remote-side effect reversal or exactly-once provider billing claim.
- Work was compared with the locally available `upstream/main` at
  `a5f3fd13032737142916523ae4344c392292f9d5` (not freshly fetched). This slice
  changes tests/docs only; no shared interface/invariant diverges further. The
  nested enqueue-vs-persist gap is a preexisting baseline limitation, surfaced
  here rather than silently redesigning upstream boundaries.
