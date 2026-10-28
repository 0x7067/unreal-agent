# Handoff: agent supervision, monitoring, and actionable diagnostics

Date: 2026-10-27. Baseline: `385e908` (`harness: keep narrow tasks scoped and
taskgraph scans responsive`). The five implementation slices below are now implemented. See the completion
record for exact boundaries, verification, compatibility and operational rollout.

## Goal

Make it easy for the user and parent agent to answer:

1. What work exists, who spawned it, and why is it necessary for this request?
2. Is it queued, validating inputs, executing, awaiting review, or draining?
3. What exactly is preventing completion, and what can be stopped safely?
4. Has a result merely been reported, or has all owned execution actually ended?

Do this without turning every small request into an orchestration project,
blocking the coordinator/UI, weakening ownership, or hiding unfinished work.

## Incident record: facts versus interpretation

### A. A tag request became an unnecessary validation project

The request was to tag a release. The parent added a full race-test/check/build
gate and TaskGraph instead of completing the metadata operation directly.
Both validation tasks declared whole-workspace reads; one also wrote binaries.
Those declarations conflicted, so the nominal parallel plan was not safely
parallelizable as declared.

Input fingerprinting encountered an unsupported symlink under Git-ignored
`.fix-work/`. This was a **pre-execution workspace failure**, not a failing test.
The original actor performed filesystem scans synchronously, delaying status and
cancellation. Inspection later found approximately 14 GiB in `.fix-work/`.
Broad claims can therefore be costly even before reaching an invalid path.

### B. A reviewer finished its report, but its Agent operation did not finish

A read-only agent named `scan-safety-review` was spawned during implementation.
An initial discovery shell contained:

```sh
pwd; ls -a
find .. -name AGENTS.md -print
find / -iname '*taskgraph*reducer*safety*' 2>/dev/null | head -20
```

The skill's repository path was already available. A filesystem-wide search was
unnecessary. `head -20` limited matching output, **not search duration or the
amount of filesystem traversal**. The initial shell remained active for about
40 minutes; its `find /` portion had been active for about 34 minutes when found.

The reviewer emitted final-answer text several times, including that no review
blockers remained. Its child session still contained an awaiting shell operation.
The parent's Agent operation consequently remained awaiting as well. Messages
asking it to finish did not cancel that shell. Asking a model to stop is not an
execution-control acknowledgment.

The parent reported implementation completion and later committed/pushed while
this auxiliary work remained pending. The UI continued to show **Waiting for
tools**. Diagnosis required correlating session records, the Agent's child
session handle, shell operation state, and the live process group.

The UI was not inventing a pending tool: its status was accurate but insufficiently
specific. `cmd/unreal-agent-tui/ui.go` derives this label from any awaiting tool
card when a model response is not active. It does not identify the nested blocker.

The unnecessary scan's verified process group was terminated. The reviewer
process exited and its Agent operation became completed. No source change was
needed for that cleanup. **Never reuse incident PID/PGID values**; process
identity must be revalidated before any intervention.

### C. TaskGraph-managed agents were not reachable by the parent's SendMessage

Workers sent progress under generated `tg-...` names. Root-level attempts to
reply using those names returned `unknown subagent`, even while a graph worker
was active. Review feedback had to wait for candidate completion and then be
supplied through reject/revise.

Code inspection is consistent with supervisor-local name resolution: worker
setup supplies fresh supervisors through the TaskGraph handler factory, while
`Supervisor.addMessage` looks in that supervisor's local `children` map.
The exact cross-scope routing contract needs an integration test; do not assume
that a displayed name is a globally addressable handle.

### D. Installed and running binaries were different generations

Both binaries were rebuilt and atomically installed, but the existing session
continued on its worker snapshot executable. Installation does not reload an
active worker or frontend. This is expected lifecycle behavior, but users need
to see which implementation is actually running, not just which is installed.

## Baseline already implemented; do not redo or overstate

Commit `385e908` includes:

- Scope-first model instructions and prompt-delivery tests. Necessary code-change
  verification, repository policy, and safety checks remain required.
- A single bounded, cancellable TaskGraph scan worker; pure status snapshots;
  phase/timing metadata; failure categories; conservative `command_dispatched`;
  whole-workspace/conflicting-claim warnings; preserved runtime updates in tool
  results.
- Generation/epoch checks, scan-drain ownership, strict prelaunch checkpoints,
  old-v1 checkpoint recovery, and final freshness checks for all-accepted restore.
- A reproduced shutdown race fix: a previously successful queued scan result
  cannot dispatch after observed runtime cancellation. Nested contexts stop
  before joins. Deterministic regressions complement reducer model checking.
- An ancestor-observation fix allowing unrelated sibling writes without weakening
  checks on directories actually claimed. Fingerprint hash encoding is unchanged.

Full `make test` (race detection, both modules), `make check build`, installation
comparison, and the synthetic scheduling evaluation passed before commit.
The synthetic evaluation is **not** evidence about model delegation quality.
Subsequent exploratory live-model evidence is recorded in
[harness-diagnostics.md](harness-diagnostics.md); it does not establish an
improvement claim for the completed supervision implementation.

Those baseline changes did **not** provide a general agent activity inspector, parent-routed
nested-agent controls, automatic auxiliary cleanup, or a nested blocker UI.
They move filesystem scans off the TaskGraph actor, not every synchronous
manager/supervisor callback. Symlinks are still rejected; ignored files remain
included in declared directories. A blocked kernel syscall cannot be forcibly
interrupted, and ownership/Wait must remain truthful until drain.

References:

- [Current TaskGraph guarantees and limitations](../../harness/taskgraph/README.md)
- [Scope guidance and manual behavioral evaluation](harness-diagnostics.md)
- [Existing asynchronous supervisor audit](supervisor-async-audit.md)
- [Upstream-first supervisor I/O follow-up](supervisor-io-follow-up.md)
- [Worker reload design](hot-reload.md)

## Design principles

### Keep result, acceptance, and lifecycle separate

These are different facts, not one overloaded status:

| Fact | What it proves | What it does not prove |
| --- | --- | --- |
| Spawn/control request admitted | Request is queued at its stated durability boundary | Child started, input consumed, or action completed |
| Child report available | A model emitted candidate/final text | Tool drain, process exit, or correctness |
| Candidate accepted | Reviewer accepted a current generation with evidence | Unrelated auxiliary work ended |
| Cancellation requested | Targeted shutdown has been requested | Process/tree terminated or ownership released |
| Execution drained | Owned operations, processes, readers, and output delivery reached their required barriers | Semantic acceptance of the result |

Retain existing conservative semantics: an Agent operation must not become
terminal success merely because final-answer text appeared. Instead expose a
report-available event independently, and show what still prevents lifecycle
completion. Likewise, do not make the UI idle by forgetting an awaiting card.

The parent may legitimately answer a new user input while background work runs.
Do not globally ban final messages while tools exist. Distinguish **this request
is satisfied**, **intentionally retained background work**, and **auxiliary work
that should have been cleaned up**.

### Make controls structural, not conversational

`SendMessage("please stop")` is steering, not cancellation. Inspection and stop
must work even if the model is unavailable, rate-limited, or already done speaking.
Timeout, transport failure, stale identity, and unconfirmed drain must remain
explicit outcomes. A timeout must never be reported as successful cleanup.

### Diagnose before introducing more orchestration

Use direct tools for tiny/serial work. Before spawning, record a short purpose,
parent task, expected result, owned resources, and intended lifetime. Do not add
mandatory confirmation rounds for ordinary authorized spawning. Reserve explicit
approval for consequential ambiguity or actions requiring authorization.

## Implementation slices and acceptance requirements

### 1. P0: authoritative activity snapshots and blocker chains

Add a structured, bounded activity view shared by supervision, model-facing
inspection, and UI presentation. Prefer projections of typed operation/session
events plus explicitly labeled live observations; do not have the UI rediscover
process ownership by scraping `/proc` or parsing human tool text.

Useful fields, with unknown/unavailable values represented honestly:

- Stable agent handle, display name, parent handle, owner operation/call ID,
  child session ID, supervisor scope, and attempt/process generation.
- Graph/task/generation when applicable; task purpose and lifetime policy.
- Execution phase, report availability, active descendant counts, and the
  **oldest/relevant blocking descendant** with its causal path.
- Spawn/admission/start timestamps, phase age, last observed progress, and source
  of progress evidence. Output silence alone is not proof of a stall.
- Safe command summary, workspace, operation ID, artifact references, and
  redacted process identity where appropriate. Commands can contain secrets.
- Cancellation request/acknowledgment/drain state and last diagnostic error.
- Actual worker/frontend build identity and reload state; installed candidate
  identity is separate from the executing generation.

Inspection must be snapshot-only, responsive during slow I/O and blocked child
operations, and safe during startup/exit/replay. No filesystem traversal,
revalidation, handler join, or scheduling side effect in a status request.
Bound the response size and include omitted-node counts/artifact references.

**Acceptance:** the reproduced reviewer incident can be explained from one
activity query as `parent -> reviewer -> shell -> find /`, with elapsed time,
report availability, and reason it is still awaiting. No raw JSONL/process-table
forensics are required for normal diagnosis.

### 2. P0: scoped inspect/message/stop handles for direct and graph agents

Choose a small control interface after reviewing upstream/shared contracts.
Possible shape: agent list/status/stop plus a graph-task message route. The implemented interface is `AgentControl` (`subagent.control/v1`). Do not
silently treat all names as global.

- Return an opaque, scoped handle on spawn/admission and attach it to progress.
- Make direct and TaskGraph-managed children inspectable/control-addressable by
  their authorized owner. Preserve isolation between supervisors and sessions.
- Include generation in routing so a delayed message/cancel cannot affect a
  replacement child with the same display name. Names are labels, not authority.
- Distinguish request admission, durable delivery where promised, processing
  acknowledgment, and completed stop/drain. Surface unknown/stale handles clearly.
- Support targeted cancellation of an operation or owned subtree, not just
  session-wide shutdown. Do not kill siblings, a whole shared supervisor, or the
  parent when stopping one auxiliary reviewer.
- Reuse operation-manager cancellation and checkpoint-aware shutdown. Avoid
  teaching the model to kill guessed PIDs as the normal control plane.

**Acceptance:** a parent can send review feedback and stop a TaskGraph child
before candidate completion, without reject/revise solely as a messaging workaround.
The same API remains responsive when the child's shell or input consumer blocks.
Stale-generation and cross-session requests fail safely.

### 3. P0: explicit task-scoped lifetimes and completion cleanup

Design an explicit lifetime policy for auxiliary reviewers versus intentional
long-lived services. Do not infer permission to terminate a database/server from
the fact that a model sent final-answer text.

A practical completion path for task-scoped helpers:

1. Emit `report_available` without falsely completing the Agent operation.
2. Inventory outstanding owned work at the task boundary.
3. Keep necessary work visible; cancel unnecessary task-scoped work through the
   structural control path; retain intentionally backgrounded work explicitly.
4. Acknowledge cancellation promptly, then join processes/readers/output delivery
   and persist proven outcomes before releasing ownership.
5. If cleanup cannot finish, show a retained, attributable blocker and the next
   safe action. Do not synthesize success or silently forget the resource.

Define whether the parent or child owns the cleanup decision; make repeated
completion/cancel requests idempotent. Distinguish a reusable idle agent from a
finished ephemeral helper. Budgeted escalation may be appropriate for owned
process groups, but must preserve process identity, output, and session safety.

**Acceptance:** a reviewer which emits a final report while an unnecessary scan
runs cannot quietly leave the parent appearing finished with an unexplained tool
pending. It either drains/cancels according to its declared policy or explicitly
reports retained work. Intentional background tasks are not indiscriminately killed.

### 4. P1: task-aware, bounded discovery and delegation budgets

A prompt is guidance, not enforcement. Read-only work can still consume arbitrary
I/O, wall time, process slots, and context. File claims do not sandbox commands.

- Resolve known repository instruction/skill locations before exploratory search.
- Default discovery to explicit workspace roots. Broad/outside-workspace searches
  need a concrete purpose, not an accidental fallback from a known path.
- Distinguish output caps from wall-time/traversal/process budgets. Pipelines such
  as `find / ... | head` do not provide a general search-time bound.
- Offer configurable operation budgets and task-level resource accounting;
  distinguish a warning threshold, requested cancellation, and confirmed drain.
- Account for parent, direct child, and graph-child resources together rather
  than assuming each supervisor's local limit bounds the whole session.
- Deduplicate equivalent validation runs where inputs/evidence are unchanged;
  do not repeatedly run expensive full suites just because another observer asks.

Do not implement a brittle shell-string denylist as a security boundary, silently
exclude files from freshness checks, or add a permission questionnaire to every
read-only command. Strong execution confinement is a separate design if required.

**Acceptance:** the incident's known-path lookup performs no filesystem-root scan;
a deliberately broad quiet command reaches an observable budget state. Budget
expiration does not claim that an uninterruptible syscall was terminated.

### 5. P1: UI and heartbeat diagnostics with actionable controls

Keep a concise summary, with a drill-down into the actual blocker. Examples:

```text
Review reported; cleanup pending · scan-safety-review · 1 tool · 34m
  shell: find / … · no recent observed output
  Actions: inspect · request stop · view output

Validating inputs · graph build/task tests · 12s · command not dispatched
Stopping reviewer · request acknowledged · waiting for owned process drain
Background work retained · preview server · explicitly kept running
```

- Include task/agent identity and pending count in the compact status; use a
  deterministic priority rule when several kinds of work coexist.
- Show report availability separately from running/draining descendants. A final
  answer must not erase the blocker, and a historical card must not remain active
  after an authoritative terminal update.
- Surface nested failures with phase and origin: workspace input validation is
  not `tests failed`; queued/dispatched is not proof that a command started.
- Heartbeats should identify blocker chains, phase ages, last progress evidence,
  and available structural controls. Avoid repeated generic messages inviting
  more broad inspection or more model requests to the same finished reviewer.
- Treat suspected stalls as diagnostics, not proof. Thresholds should be
  configurable; slow expected work and uninterruptible drain need honest labels.
- Show executing versus installed build IDs so reload expectations are explicit.

**Acceptance:** the user can see why waiting persists and target the pending work
without reading session files. Late/duplicate/replayed updates cannot resurrect
finished cards, hide live work, or produce a false idle state.

## Architecture and safety constraints

Follow the README glossary/components and `AGENTS.md`. In particular:

- Compare against current `upstream/main` before changing shared interfaces or
  invariants; raise any fork divergence explicitly. The TaskGraph subsystem at
  this baseline is fork-specific. Upstream comparison must be refreshed, not
  assumed from the incident's cached ref.
- Translators/context building remain I/O-free. Controls become serializable
  operations; coordinator persistence precedes dispatch.
- Graph state remains actor-owned and reducer transitions stay pure. Workers may
  publish immutable progress, never mutate reducer state concurrently.
- Preserve ownership until actual drain, dependency/acceptance freshness,
  generation gating, and conservative unknown-launch recovery.
- Preserve ordered output delivery and distinguish process exit from output EOF
  and durable terminal state. Do not truncate committed output to make stop fast.
- Version persisted/protocol changes and preserve supported old-session reads;
  unsupported versions fail explicitly. Clearly label live-only observations.
- Do not fix actor head-of-line blocking by spawning unbounded goroutines or
  dropping updates. Reuse the ordered-mailbox/durability work in the linked audit
  and I/O follow-up rather than creating incompatible parallel designs.
- Redact diagnostics and bound retention. Do not expose credentials, private
  prompt contents, or provider reasoning payloads in status summaries.
- Rebuild/reinstall after code changes; frontend UI changes need a frontend
  restart. `/reload` updates a worker only at supported lifecycle boundaries.

## Code map

| Area | Start here |
| --- | --- |
| Scope/delegation instructions | `harness/contextbuilder/prompts/preamble.md`, `scope_test.go` |
| Agent/message translation and handle shape | `harness/tool/subagent/subagent.go` |
| Local child registry, stdout events, delivery, exit, cancellation | `cmd/internal/subagent/supervisor.go`; its lifecycle/artifact tests |
| Direct versus graph supervisor construction | `cmd/internal/workerruntime/setup.go`; inspect runner setup too |
| Graph worker creation and immutable scan telemetry | `harness/taskgraph/runtime.go`, `scans.go`, `scans_test.go` |
| Operation ownership/cancellation and process primitives | `harness/operation/local_manager.go`, `harness/primitives` |
| Idle/wait/heartbeat and persisted tool dependencies | `harness/coordinator/loop.go`, `heartbeat_test.go` |
| Worker generation, reload, frontend protocol | `cmd/internal/workerhost`, `cmd/internal/workerruntime`, `cmd/internal/workerproto` |
| Tool cards and generic waiting summary | `cmd/unreal-agent-tui/tools.go`, `ui.go`, their tests |

## Test plan and definition of done

Use deterministic fake children/models and gates; do not reproduce this by
actually searching `/` or waiting 40 minutes. Prefer logical/virtual time for age
and budget tests. A child report and child-process exit must be independently
gated in the fixture.

Required integration/regression scenarios:

1. Child emits final report while an owned shell is blocked: report visible,
   Agent still pending, exact descendant shown; requested cleanup drains it.
2. Repeated final/stop messages do not count as process termination. Structural
   cancellation works even when no model turn can run.
3. Root parent routes inspect/message/stop to a graph child with scoped handles;
   same-name children elsewhere and replacement generations are unaffected.
4. Control admission, delivery, acknowledgment, process exit, output drain, and
   persisted terminal result are separate barriers. Slow stdin/output consumers
   do not block unrelated status or cancellation admission.
5. Successful child/scan result queued just before cancellation cannot dispatch
   replacement work, accept stale data, or falsely release a lease.
6. Restart/reload during pending inspection/control/cleanup preserves ownership,
   ordered results, old-session compatibility, and explicit unknown outcomes.
7. Quiet but healthy work gets age/evidence, not a false stall verdict. Expired
   budgets with uncooperative work retain visible ownership rather than success.
8. Intentional long-lived background work survives an unrelated task's finish;
   ephemeral helpers do not leak after confirmed completion/cancellation.
9. UI snapshot/replay, narrow terminal, collapsed tools, and late/duplicate events
   preserve accurate blocker identity and terminal state. Prove UI changes in tmux
   against a fake model using the repository's TUI verification skill.
10. Tag-only, code-change, and policy-required-validation scenarios retain the
    scope distinctions in [the behavioral fixture](harness-diagnostics.md).

Track active descendants after confirmed completion, cancellation acknowledgment
latency separately from drain latency, status latency under blocked I/O, time to
identify a blocker, unnecessary spawn/search/full-suite counts, and phase-claim
accuracy. Set measurable bounds for controlled tests; do not promise universal
latency for arbitrary filesystems. Keep raw evaluation transcripts and failures.
Prompt text tests and synthetic scheduling timings do not prove model behavior.

Run targeted tests per slice, then `make test check build` for integrated code
changes. Run reducer/inbox safety skills when their event sets or invariants are
affected. Do not use an unchanged reducer model as proof of supervisor/process
interleavings; runtime integration tests remain necessary.

Deliver in reviewable slices: activity schema/projection, scoped controls,
lifetime/cleanup policy, then UI/heartbeat and resource-budget refinements.
Do not start with a wholesale supervisor rewrite or claim the work is complete
because the waiting label disappears. Completion means the user can identify
and safely manage real work, and the harness tells the truth about what remains.

## Completion record (2026-10-27)

This replaces the earlier **foundation-only** status. Direct and graph children
now share a session-local Hub; both the model and frontend can inspect and
control them. The user-facing path is implemented, not just the projection API.
The original incident and acceptance requirements above remain as the design
record; limits below narrow factual claims, not the requested implementation scope.

### Completed capabilities and evidence

| Slice | Implementation and verification |
| --- | --- |
| P0 activity/blockers | `cmd/internal/subagent/{activity,metadata,hub}.go`: scoped fleet, report/phase/age/progress evidence, cancellation and independent lifecycle timestamps; bounded graph→shell blocker paths and safe executable summaries. `TestNestedGraphBlockerPathAndRedaction`, activity saturation/overflow tests, and TUI `TestActivitySeparatesTextFromDrainAndQuietFromStall`. |
| P0 owner controls | `harness/tool/subagent/control.go`, runner/runtime shared Hub setup: `AgentControl` list/status/message/stop using opaque handle + positive generation. Live root→graph integration inspects, persists feedback in the child's real history, stops the selected child, then drains/cancels its graph. Wrong generations, cross-Hub handles, replacement attempts and historical SessionID-only handles have regressions. A name alone grants no authority. |
| P0 task cleanup/background | `RunPlan` purpose/parent-task/lifetime/owned-resources metadata; graph/task/generation metadata attached before launch. A successful complete text-only model response triggers cleanup for explicit `task` lifetime. The owner stays Awaiting until process/readers/writer/ordered parent delivery drain. Explicit `background` and legacy empty lifetime retain reusable work. `TestTaskReportCleanupAndExplicitBackground` independently gates report and process exit. Explicit cancellation remains Canceling→Canceled; self-cleanup can complete with its report. |
| P1 discovery/budgets | Preamble guidance resolves known paths first, defaults to workspace roots, requires purpose for broader discovery, distinguishes `head` from time/process bounds, and reuses current accepted validation evidence. Prompt-delivery and scope-policy regressions cover this guidance, not universal model compliance. Actual warning/cancellation/accounting are enforced in `executionbudget`, the operation manager and Bash v4. `TestQuietShellReachesObservableBudgetAndActualDrain` runs a quiet real shell; fake uninterruptible work retains its lease past timeout. Shared-account tests cover root/nested managers and graph wiring. |
| P1 UI/heartbeat/builds | Model-independent worker snapshots and `/agents list`, `inspect`, `message`, `stop`; `/builds` distinguishes frontend, running worker, installed candidate and reload/restart. Heartbeats identify pending calls without raw arguments and append bounded snapshot-only activity. Actual UI→host→worker→coordinator→Hub tests verify durable controls with no model-control turn. Replay/late terminal, narrow/collapsed layouts, safe summaries and blocked transport have regressions. |

The blocker view exposes `reviewer → shell → find …`, operation identity, age,
report availability and outstanding join barriers without JSONL or process-table
forensics. It intentionally **does not echo `/` or other argv** from the incident:
only an allowlisted executable basename is retained. Unknown or oversized private
checkpoints remain unavailable, with omission counts, rather than fabricated detail.
The main 32-operation inventory retains terminal tombstones; a bounded overflow
slot preserves a later quiet blocker after ordinary completed history fills it.

### Control durability and responsiveness

- UI message/stop use durable `InputControl/invoke_tool`, not direct Hub mutation.
  Coordinator translates without I/O and initializes a deterministic operation
  **after** input persistence and **before** dispatch. `ToolInvocationFinished`
  records the bounded outcome without waking an LLM.
- Localfile's optional `ControlOperationInitializer` only initializes a supported
  control operation authorized by a locally owned prior invocation. Generic
  `SaveOperation` still refuses unknown operations; no fake model turn is created.
  A terminal checkpoint missing its outcome remains resumable without rerunning
  its side effect. Forks inherit neither control authority nor dispatchable controls.
- Child messages distinguish queued/write-started/written/**persisted ItemInput**.
  Persisted is not model consumption. Stop acknowledges cancellation requested
  until actual drain is observed. Cancellation of an undispatched control does
  not retract a stop already dispatched.
- Supervisor actors no longer perform child stdin writes, parent inbox submission,
  or remote artifact preparation inline. Finite FIFO writers, serial ordered
  delivery, and two preparation workers per handler preserve publication order
  and prelaunch barriers. One blocked archive preparation cannot prevent another
  preparation from progressing; it cannot be bypassed for ordered publication.
- Host live queries have one bounded in-flight slot and cancellation deadlines;
  a canceled late response is ignored. An interrupted partial transport write is
  uncertain and retires that transport rather than claiming a clean retry.

See [`cmd/internal/subagent/README.md`](../../cmd/internal/subagent/README.md)
for exact worker/queue/frame limits and lifecycle contracts.

### Budgets, reuse and explicit limits

Runner/frontend policy defaults were originally **30s warning, 120s cancellation request, 64
work-operation envelopes**, configured by:

Current defaults match upstream: warnings, automatic cancellation and the work
admission bound are disabled. The following values are optional explicit limits,
not defaults:

```text
UNREAL_HARNESS_OPERATION_WARN_AFTER=30s
UNREAL_HARNESS_OPERATION_TIMEOUT=120s
UNREAL_HARNESS_MAX_OPERATIONS=64
```

Explicit zero disables the corresponding threshold/work bound; library zero
configuration retains historical behavior. A configured warning must precede a
nonzero timeout. Bash can request tighter `warn_after_seconds`/`timeout_seconds`,
not extend configured policy. A separate bounded management lane (default eight)
keeps typed list/status/stop/cancel available when work capacity is exhausted;
message/run/retry/revise and malformed/unknown plans do not get this exemption.
Resource views separate work and management ownership and identify their scope.

The account is **`in_process_operation_envelopes`**: root and in-process nested
graph managers share it; direct/graph remote agent envelopes charge the owning
account. Effective policy is forwarded to child runners, whose private internal
managers have separate accounts. Fleet diagnostics aggregate those supervisors,
but this is **not** a single cross-process OS/process-tree/quota account. File
claims and shell strings are not a sandbox. No root search is used in regression
fixtures, and no denylist pretends to confine arbitrary commands.

Equivalent DAG work reuses its existing task identity and current accepted
evidence; status is a pure snapshot, not a request to re-run validation. Guidance
also prevents adding duplicate full-suite gates for a second observer. Arbitrary
shell commands are **not** memoized by command text: without declared inputs and
fresh evidence that would skip necessary verification unsafely. The exploratory
behavioral results are retained separately; no new model-quality win is claimed.

Normal structural/task stop now uses SIGINT with `WaitDelay=0`, rather than
force-killing only the runner after ten seconds. Uncooperative owned execution
remains a visible retained blocker; a caller deadline is not successful cleanup.
A blocked kernel syscall, arbitrary external synchronous handler, manager-side
artifact preparation, and wider existing inbox/manager queues are not magically
made preemptible or bounded by these supervisor-private limits. Snapshot child
operations are **last observed persisted tool-call snapshots**, not every private
manager transition or proof a ready shell started. Output silence is never a
stall verdict. Terminal handler enqueue is not parent-session persistence.

### Compatibility and upstream comparison

Comparison was refreshed against `upstream/main` at `a5f3fd1`; local baseline is
`385e908`. These are additive fork-specific TaskGraph/supervision extensions;
shared `operation.Manager` and `RemoteJobHandler` signatures remain unchanged.

- Private frontend/worker protocol is **2**; incompatible workers fail explicitly.
- Session header remains **2**; new `control_operation_v1` records carry explicit
  `Version:1`. Old supported format-2 sessions still read and accept new controls.
  Old binaries encountering the new record fail unsupported-type, not silently
  drop operations. No downgrade is promised for newly written extensions.
- Control plan is `subagent.control/v1`; old Agent plan fields and SessionID-only
  handles remain readable. Handles are live-session capabilities, not durable
  cross-restart authority; restored work obtains a fresh attempt capability.
- Bash with budgets uses shell **v4**; ordinary calls remain v3, and both versions
  read. Budgets in a v3 state are rejected explicitly.
- Private child stdin/stdout JSONL now has an **8 MiB/frame** ceiling. Oversize
  frames fail explicitly, never truncate accepted frames; final non-newline EOF
  is accepted. The separate one-shot runner parser is unchanged. Later clean-exit
  replay messages now wait for matching persisted input instead of the historical
  premature-completion exception.

### Verification and rollout

Targeted core/model/runtime/budget/coordinator/store/transport/UI race suites and
vet passed, including the real persisted-control crash gap, saturated management
lane, no-late-launch cancellation, and report-before-drain regressions. Inbox's
exhaustive admission/freeze check was rerun (244 explored states across three
recovery seeds). The reducer remains unchanged; its model check is part of the
full race suite, not a proof of supervisor/process interleavings.

Full integrated validation and installation results are recorded in
[the verification record](../verification/agent-supervision-20261027.md).
The evaluation fixture's 15 Python tests also pass; its historical live-model
results are not reclassified as a fresh run of the new implementation.

Tmux/fake-model captures prove list, persisted message result, report with an owned
shell, inspect, collapsed and narrow views, cancellation-request-only outcome,
confirmed drain/zero ownership, stale-handle failure and executing/installed builds.
All verification helpers are stopped and joined; the user's active frontend is
not killed or restarted by verification. Installation alone never changes that
frontend: restart it to use the new UI; `/reload` only changes a compatible worker
at its supported idle boundary.
