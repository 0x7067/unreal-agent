# Supervised worker reload

The frontend/host owns terminal state and durable ingress; the resident worker
owns the coordinator, provider and local execution. `/reload` gracefully replaces
only that worker, not frontend/UI code. The coordinator uses ordinary
`StopWhenIdle`; there is no `TryQuiesce` API or separate retirement channel.

## User contract

- Build a replacement separately. Reload never builds or discovers workspace code.
  The original explicit executable is the default; `-worker-executable` selects
  another trusted absolute path. Selecting code is a trust decision, not a sandbox.
- The host stages a private immutable copy and probes protocol/session/build
  compatibility **once**, before changing the running worker. Bad candidates leave
  it unchanged. Installed symlinks/aliases resolve on explicit selection.
- Busy reload is **scheduled**: current model, compaction, shell and graph work
  finishes naturally. New ordinary submissions remain durably accepted but queued
  for the replacement. No work is killed, retried or implicitly accepted to reload.
- Graphs awaiting acceptance and retained native helpers can prevent drain. Use
  `/graph status NAME`, `/graph accept NAME TASK GENERATION EVIDENCE`,
  `/graph reject NAME TASK GENERATION EVIDENCE`, or `/graph cancel NAME` explicitly.
  Acceptance requires your own validation. `/agents message HANDLE@GEN TEXT` and
  `/agents stop HANDLE@GEN` remain available. These controls target the exact old
  worker scope and existing owners; they cannot launch/revise/retry arbitrary work.
- `/upgrade` on an enabled attached host uses the same scheduled handoff as
  `/reload`, not repeated idle checks/probes.
- Canceling before admission can leave the worker unchanged. After admission,
  canceling a wait, `/cancel-upgrade`, disconnecting or reaching the client timeout
  does **not** undo the host-owned retirement/replacement. Inspect current status
  or reconnect; do not infer that a new reload is needed. Internal callers can
  await the original result with `Host.AwaitReload`.
- Terminal state, transcript, draft, scroll and session identity survive worker
  replacement. Frontend/UI code updates still need a frontend restart. `-session`
  resumes explicitly; unknown interrupted effects are not blindly redispatched.

## Ordered delivery and shutdown

```text
stage/probe -> host delivery cutoff -> ordinary StopWhenIdle
  -> natural completion (including retained native owners)
  -> worker cleanup receipt -> process/output joins -> writer lease probe
  -> launch candidate -> reconcile history -> deliver queued ordinary input
```

The host serializes the stop after inputs already forwarded to the old worker.
Only validated, current-scoped graph/helper interventions can pass the cutoff.
The terminal drain reply closes intervention delivery under the same gate used
for admission. An admitted but unpersisted old-scope intervention stays in ingress
and is durably refused by the replacement, never retargeted to it.

Internal results and child messages continue flowing. Authoritative supervisor
ownership tracks pending admissions, process/output/preparation/finalization work
and parent deliveries; completed background reports are not owner-drain proof.
The coordinator's optional pending-work signal affects graceful idle stopping;
ordinary embedders need only `Run(context.Context) error`. Independently owned
jobs/durable children retain their existing classified ownership policy.

Retirement requires a fresh scope-bound canonical execution-drain receipt,
successful actual worker exit, joined output/callbacks, and a successful acquire/
release of the writer lease. Completed structural control checkpoints are read
from canonical storage, including terminal operations absent from the resume
projection. Missing proof or held leases fail closed, including subsequent retries.
No timeout, EOF, status snapshot or reported tool completion substitutes for joins.

Replacement startup failure leaves a stopped session, with durable inputs retained
and no automatic downgrade. Unexpected worker death is not successful reload.
Explicit whole-host shutdown is different: it may cancel work and fences launches.

## Compatibility and retained limits

Private worker protocol **8** uses `drain`/`drained`. Older peers are rejected
before retirement. Public attach still uses version-1 frames, with separately
advertised `runtime_replacement_v2`, `runtime_drain_v2`, `host_retire_v2` and
`reload_intervention.v1` capabilities. Old runtime capabilities do not authorize
the changed cancellation/replacement semantics.

Session header format remains 2, with fork-only controls/checkpoints: this is not
upstream-readable history. Historical unscoped AgentControl inputs remain readable
but do not gain access to the scoped retirement intervention lane.

Ingress acceptance, session persistence and model delivery remain distinct. Stable
input IDs, durable deduplication and replay reconcile the same messages; a failed
write/sync is uncertain, not success. Private frames remain newline-terminated and
bounded to 32 MiB; ingress records remain bounded to 4 MiB. Journal compaction is
not implemented.

Linux/macOS kernel-held advisory leases enforce participating writer ownership.
Never unlink lock files. Same-UID/root mutation, arbitrary detached grandchildren,
power-loss certification and exactly-once provider/third-party effects are outside
the contract. Cancellation-ignoring callbacks can prevent cleanup indefinitely;
that remains visible uncertainty, not permission to start another writer.

See [alignment plan and remaining work](upstream-alignment.md),
[scheduled runtime replacement](durable-runtime-drain.md) and historical
verification reports for their original, narrower tested contracts.
