# M2.6 — independently owned background child recovery

**Accepted, fully validated, proved and installed 2026-10-28.**
[Verification and evidence](../verification/durable-child-m26-20261028.md). This is an opt-in command-layer extension,
not a legacy-child replay permission or a universal remote cleanup contract.

## Supported path and explicit limits

A headless resident host started with `host --durable-subagents` enables the pure
Agent translator's `subagent.run` plan **version 2**. Its strict data envelope is
version 1, bound to the stable parent session. Only explicit
`lifetime: "background"` is supported. Task/default lifetime is rejected before
submission rather than silently retaining a task-scoped service. Child hosts get
basic tools, not nested subagent capability. Graph-agent recovery is not enabled.
The private worker protocol is 6; session format remains 2. An existing compatible
TUI can attach to this configured headless host; the TUI does not silently opt in.

The existing manager and RemoteJobHandler interfaces are unchanged. Translators
and context building remain pure. The coordinator commits the operation before
manager dispatch. All historical v1 child runs remain fail-closed unless the
existing genuine lifecycle cleanup fence already permits their history.

## Identity, reservation, admission and results

- Parent session + operation identity determine a child key/session/store. The
  immutable linkage also hashes effective provider endpoint/model/workspace and
  execution request. Resolve a default endpoint **before** reconciliation and
  setup. A replacement staged executable pathname is not a new logical job;
  the original launch command remains recorded, with no executable substitution
  for an already reserved child.
- A private, strict, exclusive `launch.json` is synced, including its directory,
  **before** execution. Invalid/incomplete markers, or a preexisting child directory
  without a valid marker, remain uncertain. They do not authorize another launch.
  A reservation-before-exec crash can deliberately strand an unsubmitted job.
  This is at-most-one launch permission, not exactly-once execution or billing.
- The independent resident child host owns its own Unix session/process group,
  no parent pipes and no parent cancellation context. It is not independent of a
  containing cgroup's destruction, machine loss, or arbitrary same-UID mutation.
  Closing a parent connection joins its socket reader only, not the child host.
- Initial prompt ingress has one deterministic input ID and payload. Repeating
  admission through the existing durable ingress contract does not create new
  user work. A saved PID, lease absence or transport EOF is never reattachment
  or effect-drain authority.
- The parent uses the first canonical complete, nonfailed, non-tool-call assistant
  answer on a regular turn opened after that initial input. Compaction, partial,
  failed and pre-input responses are not answers. Any foreign external input
  before that selected answer conflicts, even if it precedes initial delivery.
- Handle v1 stores linkage, a distinct stable parent-result delivery identity,
  and canonical answer sequence/turn references, not an answer-sized diagnostic.
  The actual output remains the operation result. Scans cap at 4096 items and
  answer text at 256 KiB. The underlying localfile API still decodes its file;
  these scan limits are not a claim of bounded decoder allocation for arbitrary
  manually enlarged history.

## Fresh authority, uncertainty and lifecycle

Each parent incarnation binds a fresh Hub capability. Old handles, generations,
name-based authority and inherited messages cannot reach the new attempt.
Messages are explicitly unsupported in this adapter. Status is observation, not
model consumption; status can wake one bounded reconnect using the same linkage.
Stop sends an authenticated, current child-host-scoped shutdown intent and reports
**cancellation requested**, never completed drain. Child transport loss is visible
as uncertain; no answer or replacement child is invented.

Recovery reconciles **every retained owner**, including already-reported children,
not just unfinished operation heads. Terminal child adoption binds observation and
fresh control without resubmitting the prompt or rewriting old results. Exact
terminal list/status/stop controls can be classified as inert: they cannot create
a child and are never replayed. Messages, unfinished controls, structural InvokeTool
fences, unknown remote plans, and pending/uncertain lifecycle intents remain refused.

Per-owner resume permission is not global cleanup proof. Retained children keep
`OwnershipProof` unproven even when terminal replay requires no manager admission;
otherwise an empty new manager could falsely issue ExecutionDrained. The first
adapter explicitly refuses ordinary foreground abort while this remote ownership
is not classified by the existing foreground policy. It does not cancel a
background child merely to complete abort. Shutdown/interruption remains explicit,
and restarting after an unavailable child does not silently relaunch it.

The operator must deliberately stop retained services through fresh child/parent
capabilities. Test fixtures authenticate shutdown and independently join their
owned process/reader handles. A cleanup report or stop admission is not a join.

## Upstream comparison and divergence

Compared with the supplied **local** upstream tip
`a5f3fd13032737142916523ae4344c392292f9d5`; no fetch. Its remote plan/handler
boundary remains intact. The private recovery/host/Hub packages, this opt-in v2
translator, retained-owner permission and compaction layer are local divergence.
No upstream generic remote-job interface or pure-builder invariant is expanded.

## Evidence targets

- `cmd/internal/durablechild/integration_test.go`: real child/provider and parent
  worker SIGKILL/replacement at awaiting, committed-child-answer, and saved-parent-
  operation/unrecorded-final-status boundaries; stable child/input/result, rotated
  capability, stale model-issued stop refusal, foreground abort refusal, terminal
  adoption and no invented global receipt. The saved-operation window is a
  **controlled canonical persistence fixture**, not an uninstrumented live hook.
- `launch_process_linux_test.go`: actual child launch inside an owned replaceable
  parent's process group, exact parent-group SIGKILL, same child after reconnect,
  one request. Cleanup uses authenticated shutdown, the specific host's kernel
  pidfd exit event and a canonical child drain receipt, plus containing unit drain;
  it does not claim recovery of the killed parent's Cmd.Wait object.
- `handler_test.go`, `answer_test.go`, `connection_test.go`: exact parent/link,
  unsupported data, bounds, conflicts, fresh/stale authority, lost/malformed
  reservations, canonical answer attribution and bounded scans.
- Lifecycle/gate tests preserve all default, mixed-owner and lifecycle-intent
  fences. Required full validation, manifests, owned frontend replacement,
  installation and exact cgroup observations must be linked before acceptance.
