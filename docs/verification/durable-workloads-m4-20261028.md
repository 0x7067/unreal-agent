# M4.1–M4.5 accepted: trusted extensions and durable workloads

**Status: accepted, 2026-10-28. All 27 defined goal.md slices earned.** This is
acceptance of the stated classified scenarios, not universal replay, arbitrary
plugin safety, power-loss durability, or exactly-once external delivery.

## Current frozen validation and installation

- Full unit: `unreal-agent-m4-validation-20261028T111602Z.service`.
- Raw evidence: `/tmp/unreal-agent-m4-validation.zHSp0I`.
- Focused boundary stage: `go test -race ./cmd/internal/workerruntime
  ./cmd/internal/residentcli -run '^TestResumeRefuses|^TestM4' -count=1 -v`.
- Required commands: **`make check`, `make build`, `make test`**, allPASS0.
  `make test` ran `go test -race ./...` and
  `go -C cmd/unreal-agent-tui test -race ./...`.
- Exact fresh package union: **57 =52 root +5 TUI;54 passing +3 no-test**.
  Before/after/current `go list` equality, no duplicates/missing/extra packages.
- **653 tracked+untracked relevant source files**; before/after/current manifests
  byte-identical. Manifest SHA256:
  `017f78bb07f654fbce153eba3132903995ee997e993d63e5db68996f237de212`.
  Scope: `cmd harness internal go.mod go.sum Makefile docs/formal`, as in prior
  accepted checkpoints. UI driver/base and binary inputs have separate exact
  before/after/current hashes. Documentation outside that scope is not a build input.
- Parent validation: `validation-result.json`, actual success0/MainPID0, original
  exact cgroup absent before and after stopping the exact named unit. A process
  exit alone was not used as semantic acceptance.
- Both binaries atomically replaced through individual same-directory temporary
  files in `~/.local/bin`; **individual runner cmp PASS, individual TUI cmp PASS**.

| Binary | Installed SHA256 (identical to bin/) |
| --- | --- |
| runner | `b7d7be82195527995d745a572836ec92c009177946007e93a7bd7b6b152787c8` |
| TUI | `8d39575dfdec0b2deabe27d0f13c0f3bc68417f3533296b489f08b66c91d36b8` |

## Per-slice semantic acceptance

### M4.1 — replace one named implementation

Implementation: [compiled registry/handler](../../harness/extension/extension.go),
[pure translator](../../harness/tool/trustedextension/echo.go),
[trusted startup/watcher](../../cmd/internal/workerruntime/trusted_extension_config.go).

`TestM41CanonicalCommitBeforePinnedGenerationDispatch` uses the production
translator, actual localfile canonical status+operation transaction, operation
manager, and handler. It asserts inert translation and zero execution before
commit, independently reopens the committed canonical file inside BeforeExecute,
blocks A, installs/selects B, dispatches B, and verifies B output while A remains
blocked, then A output on A. Registry tests assert failed schema/version installs,
selected/in-use removal, immutable generations, joined cancellation and bounds.
The real UI additionally executes `TrustedEcho` and receives `A:UI_ECHO_INPUT`.
No shared `tool.Registry`/`tool.Context` API change, Go plugin or workspace loader.

### M4.2 — pending and completed old-generation recovery

[Real OS worker tests](../../cmd/internal/residentcli/extension_recovery_test.go),
[immutable fixtures/provenance](../../cmd/internal/residentcli/testdata/m4-extension/PROVENANCE.md).

`TestM42RealWorkerPinnedExtensionRecovery` runs all 6 cases through production
TestMain worker dispatch, bootstrap, canonical history, recovery gates and provider:

1. Committed unfinished A with A+B installed and B selected produces **A:seed**.
2. Missing A refuses visibly, provider0, canonical bytes unchanged.
3. Completed A remains **A:seed** with only B available, without old code lookup.
4. Future plan99 refuses, provider0, canonical unchanged.
5. Future schema99 refuses, provider0, canonical unchanged.
6. Future adapter/envelope99 refuses, provider0, canonical unchanged.

The pending fixture represents the synced intent-before-dispatch crash window;
it is not a claim that arbitrary side-effecting extension code is exactly-once.
The compiled A/B handlers are pure; selected generation cannot reinterpret an old
plan. Initial schema1 records without exact turn metadata can recover through
canonical ownership but cannot acquire optional foreground cancellation authority.
No migration is supplied for unsupported versions. The model-visible schema does
not expose selection; persisted plans, not current selection, reconstruct meaning.

### M4.3 — independent executor reattachment

[Executor](../../cmd/internal/jobexecutor/executor.go),
[adapter](../../cmd/internal/durablejob/handler.go),
[fixed tool](../../cmd/internal/workerruntime/trusted_job.go),
[production CLI](../../cmd/internal/residentcli/executor.go).

`TestM43WorkerRestartReattachesIndependentExecutor` runs an actual separate
production executor CLI and actual worker A/B. The private proxy drops a submit
response **after launch**, holds transport unavailable, then A is killed through
Host authority and actually joined. Executor/job survive. B resumes the same
committed operation, one effect, `firstsecond`, terminal executor proof and input
retry dedup. Adapter tests separately cover lost-response retry, strict/null
checkpoint decoding, plan-byte immutability, output offset/clamping, stale
cancellation authority, actual cancellation joins and bounded unavailable parking.

### M4.4 — timer commit, restart, abort and client reconciliation

[Task handler](../../harness/reminder/task.go),
[actual-store fake-clock tests](../../harness/reminder/store_integration_test.go),
[real worker/client proof](../../cmd/internal/residentcli/timer_recovery_test.go),
[commit receipt bridge](../../cmd/internal/workerruntime/durable_graph.go).

`TestM44TimerActualStoreCommitFenceAndRestart` and
`TestM44TimerActualStoreStableBoundedBatches` use real append/sync/readback, fake
clock movement, restart before/after deadline and pending/terminal commit windows,
exact checkpoint receipts, stable deadline/ID ordering and16-item batches without
scheduling sleeps. A notification cannot precede the actual canonical save.

`TestM44RealWorkerTimerRestartAbortAndClientReconciliation` seeds exact owned FG/BG
plans, observes scheduled checkpoints, kills/joins A, starts B before deadline,
scopes foreground abort to the exact turn, requires FG canceled/BG retained,
commits exactly one BG notification, closes before any client exists, then starts
two fresh resident/client snapshots with one unchanged notification and no provider
increase. Its real deadline is8sec with condition waits; fake-clock coverage is
separate. Live UI invokes the real `DurableReminder` translator/coordinator/handler
and projects `UI_TIMER_NOTIFICATION` with stable notification ID.

Notification semantics are **one canonical logical tool result**, not an external
send. Terminal checkpoint intentionally retains NotificationPending; it does not
claim inbox acceptance or client delivery. No exactly-once third-party promise.

### M4.5 — explicitly authorized remote environment

[TLS transport](../../cmd/internal/jobexecutor/tls.go),
[connection bound](../../cmd/internal/jobexecutor/network_limit.go),
[trusted runtime config](../../cmd/internal/durablejob/config.go),
[real network/worker scenario](../../cmd/internal/residentcli/remote_recovery_test.go).

`TestM45VerifiedRemoteRecoveryAndNoLocalFallback` uses real verified TLS1.3,
independently owned service/job and actual worker A/B. Post-launch acknowledgments
are dropped; worker A actually dies; endpoint closes while the job survives;
unavailable network errors explicitly; endpoint returns and identical submit
reattaches, never reruns locally. Environment/controller/job/attempt/fingerprint
and output handle+offset remain stable, effect1. Wrong token, wrong environment
and a new unauthorized controller are rejected. Native-shell result proves the
startup executor token was not inherited. Canonical history contains no token or
transport/CA path. TLS tests also reject wrong roots/name, stale handles and bad
configuration, and join active TLS readers/handlers on endpoint Close.

Network is opt-in; Unix-only default unchanged. Network listener requires explicit
controller allowlist. Scoped bearer header over verified TLS, explicit roots/name,
no proxy, redirect, insecure verification or local fallback. Jobs receive only
explicitly trusted administrator environment. Same-UID arbitrary code is **not a
sandboxed security boundary**, and operator job definitions/environment identities
must remain stable for recorded calls. Output reference is exact handle+offset,
not a claimed general artifact service.

## Current actual private tmux/frontend/runtime proof

Driver: [durable_workloads.py](../../.claude/skills/verify-unreal-agent-tui/scripts/durable_workloads.py)
composes the accepted private lifecycle fixture, no production/frontend discovery.

- Unit: `unreal-agent-m4-workloads-tmux-20261028T112349Z.service`.
- Artifacts: `/tmp/unreal-agent-m4-ui.kfSHvp`, private root `/tmp/u4.1pZBuK`.
- Current binary/driver/base before/after/current hashes equal; executable hashes
  match both individually compared installed binaries above.
- Live Echo and Reminder tools:4 provider requests, exact A output, one stable
  logical timer notification. Remote prelude:2 requests, separately supervised
  production TLS executor CLI, exact private worker lease holder identified across
  all owned host threads and bound by pidfd, frontend detached/waited first,
  worker SIGKILL observed exited, actual old host wait1, executor/job survive.
- Explicit fresh host with new incarnation, same canonical operation
  `7f4af5b4-afc3-4fba-9a09-cc7c1d625fd3`, same remote job identity, effect1,
  `firstsecond`; executor actual wait0 and provider threads joined. Token absent
  from canonical history and captured client frames.
- Full baseline then passes model/Bash/frontend replacement, foreground cancellation,
  shutdown, offline read-only reconciliation, sentinel1/no late tool.
- **8 total provider requests** =4 auxiliary +2 remote +2 baseline.
- **4 actual frontend child waits/zero exit, exact TTY restoration, no forced
  cleanup**, pidfd wrapper exit receipts. **13/13 baseline cleanup checks true**.
- Unit actual success0/MainPID0; exact original cgroup absent, exact unit stopped,
  same cgroup absent afterward. Private host/runtime/frontend/executor/tmux/socket
  and reader owners joined. No user frontend or production session was targeted.

## Rejected candidates and safeguards retained

Earlier focused runs exposed fixture issues (duplicate model response for one turn,
wrong executor Snapshot fields, external input object instead of string, terminal
status incorrectly dropping operation IDs, JSON lowercase input tag, timer field
case). They were rejected rather than counted as evidence.

Two full candidates were rejected: one exposed a completed-history fixture error;
the next exposed configuration getenv occurring before native unsafe-resume refusal.
The final implementation restores a pure native preflight **before environment
lookup**, then still requires all exact configured ownership/recovery gates before
provider construction or dispatch. All original shell3/4 crash-window and unknown
operation environment-refusal tests pass. No native/unknown ownership rule was
removed to make M4 pass.

UI candidates rejected inherited provider-count assumptions and a welcome-banner
wait that stopped working after history scrolled. The accepted driver checks a
fresh snapshot plus actual composer detach hint; it does not weaken protocol,
canonical identity, effect, wait, TTY or cleanup assertions. A successful smaller
UI run preceded the final enriched proof, but is not substituted for it.

Rejected full artifact roots: `/tmp/unreal-agent-m4-validation.zLLMDg`
(canceled after known completed-history fixture failure) and
`/tmp/unreal-agent-m4-validation.5KwTyo` (native environment-ordering regression).
The boundary compile-only candidate `/tmp/unreal-agent-m4-boundary-focused.S07LsS`
was rejected for a missing import and drained. UI rejected roots include
`/tmp/unreal-agent-m4-ui.xb77IR` (provider-count assumption) and
`/tmp/unreal-agent-m4-ui.DY7PjK` (scrolled welcome-banner waiter). Earlier smaller
successful UI roots `LJh2hX`/`5q4fY2` are historical, not final enriched evidence.

Known earlier owned M4 focused/integration/full/UI units were inspected for actual
termination and exact cgroup absence and stopped by their recorded names. Final
full and UI receipts are archived. Tests in all successful final scopes are parent
validated candidates, not helper claims. No graph reducer/inbox production changes,
commits, fetch, protected tool-output reads, or production worker/frontend actions.

## Upstream, repository and limits

Compared against **local** `a5f3fd13032737142916523ae4344c392292f9d5`, no fetch.
Shared Registry/Context interfaces remain identical. New opt-in registry composition,
pure lifecycle classification, exact turn/time metadata, trusted executor/TLS and
timer receipt hooks are local additions; upstream has none of the resident/recovery/
Hub/compaction/taskgraph extensions. Divergence and transport authorization choices
were surfaced before acceptance.

HEAD remains `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commits. Huge preexisting
dirty/untracked work preserved. `harness/taskgraph/graph.go` remains blob
`2315afc35788160fc81bf84657f2600bd61c9e86`. Worker protocol7/session2 unchanged.
Never touched user frontend or production session/worker
`33b63c18-e05a-487f-a0a9-2f5411dac58a`; never read its protected tool-output files.

Limits: compiled Echo A/B only;16 generation identities/32 calls per handler;
128 timers/16 overdue transitions and completions per poll;32 executor jobs and
connections,64KiB retained output. Executor must survive the worker: service restart
with a running reservation is unknown, never blind rerun. Bounded retry exhaustion
parks pending state. No cloud provisioning, arbitrary plugin migration, external
notification exactly-once, or storage power-loss certification. See
[configuration and semantics](../plans/durable-workloads.md).

Evidence archive: `durable-workloads-m4-20261028-evidence.tar.gz`.
Archive SHA256: **6d3aac7a0622113556636c72dd6446abc6379954eaab7fa21b0720719f026927**.
