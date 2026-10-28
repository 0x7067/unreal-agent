# M1.6 — lifecycle controls (complete for classified local ownership)

M1.6 is complete, with [current-source full validation, atomic installation and
actual owned private-tmux/process evidence](../verification/durable-lifecycle-m16-20261028.md).
Public negotiated controls are installed: attach protocol 1, private worker
protocol 3, canonical session format 2. This is the design/work log; the linked
record is acceptance evidence. M2–M4 remain unfinished. Never infer owner drain
from EOF, terminal operation status, or legacy `Host.Close` success alone.

## Reviewed divergence and invariants

Local `upstream/main` is `a5f3fd13032737142916523ae4344c392292f9d5`
(not fetched). Its hard stop also cancels all operations and retires the loop.
Scoped nonretiring abort, local cleanup receipts, and proven-drain host shutdown
are **new shared semantics**, explicitly raised with the user. No translator or
context-builder I/O is introduced. Admission and full inert operation specs must
still commit before dispatch. Unknown ownership remains unsupported.

Distinct actions:

1. **Detach:** close only the frontend/socket; leave host and work alive.
2. **Foreground abort:** bind an intent to the original runtime scope and current
   turn. Cancel only classified foreground owners, await their actual local
   callback/effect/output completion, discard all late model output/tool calls,
   consume old tool/heartbeat continuations, retain new external inputs, and keep
   the coordinator/background owner context alive.
3. **Deliberate session shutdown:** durably fence ingress, stop all supported
   owned execution, join every owner, commit a work-drained outcome, verify worker
   proof plus process/output exit, and only then finalize/release host authority.
   Accepted intent is not completed cleanup. Deadline/cancellation cancels a
   caller wait, not the independent cleanup or its ownership fence.

## Internal implementation now present

- `harness/inbox/{foreground_abort,owned_lifecycle}.go`: strict version-1 typed
  abort, shutdown, outcome and execution-drain controls. No public arbitrary
  `InputControl` endpoint is opened.
- `harness/coordinator/foreground_abort.go`: nonretiring abort. It joins **all**
  outstanding local model callbacks, including canceled/superseded calls. A late
  response cannot dispatch even complete tool calls. Known returned usage can be
  preserved; provider-side cancellation/billing is not proven. Original input ID
  and target cannot be repurposed to cancel a later turn.
- Initial foreground policy supports classified root Bash/model work and
  acknowledged, explicitly background `Agent` branches in the proof-required runtime. Root TaskGraph,
  unknown/legacy/task-lifetime remote ownership, untranslated work, unsupported
  operations and compaction refuse before cancellation. This is deliberately
  conservative, not a general ownership graph or graph-abort implementation.
- `harness/operation/lifecycle.go`: optional live `LocalCleanupVerifier` and
  `AwaitStopped`. The mandatory manager interface is unchanged. Live proof uses
  at most 256 receipts recorded only after the actual owned drain barrier, never
  terminal status. Evicted/missing/remote/unknown/unsupported IDs fail closed.
  `AwaitStopped` joins the actor/forwarders and local primitive effect/output
  completion; remote admission attempts remain structured uncertainty even if
  their reported status was terminal. This is not a sandbox or proof against
  arbitrary detached OS daemons; primitive sender epilogues are not exposed joins.
- `harness/sessionstore/lifecycle.go`: ordered intent/outcome validation and
  startup fence. A work-drained shutdown is **not** host finalization. Pending or
  uncertain intents refuse automatic restart. This does not yet implement M2
  recovery/reconciliation policy.
- `cmd/internal/lifecycleguard`: both journal-only lifecycle intents and canonical
  intents must fence new writers. A one-shot runner takes the same authority
  lease as a resident frontend before taking the session writer lease. It keeps
  authority through all cleanup, then releases session lease before authority.
  Existing journals are inspected without creating unrelated journals. Private
  workers borrow their parent's authority instead of acquiring it twice.
- `agentrunner` and `workerruntime` validate canonical lifecycle history under the
  writer lease before provider/tool execution; `workerhost` refuses undelivered
  lifecycle replay at startup. The journal's pending intent remains durable until
  the canonical input exists, covering the pre-canonical crash window.
- Opt-in `subagent.Config.RequireCleanupProof`: a fresh per-launch UUID scope is
  passed via private `-cleanup-proof-scope`. A trusted runner seals/joins model
  calls, drains local effects, joins tools/graphs/supervisors/hub/input owners,
  then writes a canonical `ExecutionDrained` receipt while its observer and lease
  are still held. Stale sequence/generation/scope receipts cannot satisfy a new
  launch. A launched child exiting without its required receipt fails `Wait` and
  cannot be silently relaunched. Scope is correlation, not same-UID sandboxing.
  Only direct generic unproven-remote IDs for known current managed handlers can
  be reconciled against independent successful strict owner joins. Actual unknown,
  unsupported or wrapped independent errors remain unproven.

Focused race tests cover strict decoding/admission safety, callback joins and
late tools, old-target retries, real Bash drain without loop retirement,
independent live cleanup proof, retained authority after worker lease release,
journal-only intent fencing, canonical work-drained fencing before providers,
child receipt sequence/generation/scope, actual missing-receipt child exit, and
model sealing/joining. The full pipeline for this internal layer passed (49 exact packages, check/build,
matching source manifests). Both binaries were atomically reinstalled and
byte-verified. The actual private-tmux replacement proof passed both existing
reconnect/fault scenarios; both owned units exited zero and their exact cgroups
are absent. [Core checkpoint evidence](../verification/durable-lifecycle-core-m16-20261027.md)
is historical validation of this foundation, **not final M1.6 completion**.

## Public wiring and focused evidence (accepted in linked final validation)

- Private worker protocol 3 carries the actual incarnation/generation scope; old
  private versions are rejected before stronger semantics are used. Canonical
  session format 2 and public attach protocol 1 remain unchanged. Replayed
  pre-ready drain records cannot satisfy the current process proof.
- Every strong runtime supervisor (including dynamic factories) requires fresh
  child proof. Runner stdin uses a private pollable inherited-pipe descriptor so
  cleanup interrupts and joins the reader before restoring flags. Independent
  model/manager/tool/graph/supervisor/hub/input joins authorize receipts, not the
  absence of an execution error.
- `lifecycleproof.Reconcile` is a pure, fail-closed merger. Exact canonical known
  handler plans and successful actual owner joins may discharge only generic
  unproven admission IDs. Unknown/unsupported evidence and independent wrapped
  errors are retained; terminal ticket status does not establish cleanup.
- In the strong runtime only, an explicit-background Run ticket may be
  acknowledged by a full committed assistant progress report, including one
  dispatching tools. Its child and structural owner identity remain retained.
  This is neither final report nor task/cleanup completion; diagnostics cannot
  call the owner drained until actual child joins and its fresh receipt. Legacy
  and default/task behavior is unchanged. This new divergence was raised with
  the user; the real retained-background process proof is complete in the linked evidence.
- Runtime finalization appends exact `work_drained` outcomes plus fresh execution
  proof only after independently observed barriers, under its lease/observer.
  It never appends host `completed`. The host independently seals/joins ingress,
  waits the exact worker/output/callback join and fresh proof, takes the session
  lease under retained parent authority and commits final completion before
  releasing ownership. No client wait context cancels that controller. Unknown
  exit leaves a diagnostic fenced host; journal/canonical guards survive death.
- Public capability-negotiated `lifecycle` requests admit only strict bounded
  typed abort/shutdown intents, bound to original scope/target. Result, receipt,
  foreign-scope, external and hard-stop injections actually disconnect without
  mutation. SDK reuses its single reader and stable cloned identity.
- TUI source has explicit `/abort`, `/shutdown`, `/retry-control ID`; Ctrl+C
  remains detach. Version-2 private local state saves original control identities
  separately before network, backward-reads version 1 and never rebinds retries.
  Offline reconciliation opens only existing anchored private canonical files
  and uses the shared bounded versioned read-only codec, without mkdir/leases,
  provider initialization, execution-state cache or session metadata writes.

Focused current-process evidence includes real Bash deliberate shutdown, stable
same-ID retries and wrong-scope/conflicting-identity rejection; a timed-out wait
and Close retain parent authority after the actual worker session lease is free,
blocking **actual** competing standalone and resident processes until the owned
publication barrier is released and normal completion is proven. Real SDK tests
exercise model/Bash detach, target-bound abort, deliberate shutdown, post-endpoint
read-only reconciliation and forged outcome/receipt rejection. The linked acceptance record adds the required unchanged-source full pipeline,
actual private tmux proofs, atomic install and exact service/cgroup drain.

## Acceptance and remaining roadmap

All M1.6 critical-path items passed: retained-background process survival/drain,
strong acknowledgment gates, actual control lost-ack/new-turn isolation, exact
scope rotation and pre-ready replay rejection, required both-module pipeline,
current-binary lifecycle/reconnect tmux proofs, and atomic installation of both
binaries with byte comparison. The [verification record](../verification/durable-lifecycle-m16-20261028.md)
contains actual paths, hashes, failures corrected before acceptance and cleanup.

Continue with M2.1 and the rest of `goal.md`. The unresolved-operation and
unknown/unsupported-owner guards stay conservative; no generic crash replay,
model retry accounting, remote reattachment or timer recovery is implied here.
