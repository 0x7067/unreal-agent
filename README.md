# Unreal Agent

An async-first agent harness from Unreal Labs.

- [harness/](harness/) — the library.
- [cmd/](cmd/) — executables that use the library.
  - [cmd/unreal-agent-runner](cmd/unreal-agent-runner) - cloud agent runner.
  - [cmd/unreal-agent-tui](cmd/unreal-agent-tui) - cozy TUI.
- [benchmarks/](benchmarks/) — benchmark runners.

Architectural direction: [a durable, reconnectable, replaceable harness](goal.md)
(target and incremental roadmap; not a statement of current guarantees).
Current simplifications and remaining work:
[upstream alignment](docs/plans/upstream-alignment.md).
The [M1.5 adversarial-client verification](docs/verification/durable-adversarial-m15-20261027.md)
records stalled-client isolation, measured publication bounds, and authentication
test limits. The [M1.6 lifecycle verification](docs/verification/durable-lifecycle-m16-20261028.md)
records distinct detach, scoped foreground abort, proven-drain shutdown and their
conservative ownership limits. The [M2.1 recovery entry](docs/verification/durable-recovery-entry-m21-20261028.md)
enables only exact initial-ready pure/read-only cells under exclusive ownership;
broader interrupted model, Bash, subagent and graph recovery is not yet enabled.

The runner and TUI expose `TaskGraph` to the coordinating agent for work with
useful overlap: a named dependency graph schedules ready commands or enabled
subagents within declared file ownership. Completed results require explicit
parent acceptance before dependents run. See the [task graph usage and
limits](cmd/unreal-agent-runner/README.md#task-graphs).

## Configuration safety

The runner automatically loads workspace `.env` files, matching upstream; use
`-load-dotenv=false` to skip loading. The TUI does not load them by default,
also matching upstream; `-load-dotenv` enables its optional loader. Review workspace
configuration before running: endpoint/proxy settings can redirect credentials.
Help/version never load `.env`; subagents inherit the parent environment without
reloading it. Tool wall-time thresholds and the concurrent-operation admission
bound are disabled by default; environment settings can enable explicit limits.
See [environment trust and model compaction settings](cmd/unreal-agent-runner/README.md#environment-and-workspace-trust).
Unlisted provider/model settings use upstream's zero-value compaction threshold;
configure their context window/threshold explicitly.

## Headless sessions and reconnectable TUI (M1.1–M1.6)

`unreal-agent-runner host` explicitly starts a foreground session host;
`unreal-agent-runner attach` submits/watches through an authenticated local Unix
socket. Disconnecting the client does not stop its worker. This first slice has
one session and one active client, with fresh snapshots only.
`unreal-agent-tui attach` connects without starting or owning its worker; standalone
TUI startup is preserved. See [TUI attach and limits](docs/plans/durable-tui-attach.md).
See [commands, protocol and limits](docs/plans/durable-headless-host.md).
F5 / `/reconnect`, explicit same-ID `/retry`, and private draft/view restoration
are described in [durable reconnect](docs/plans/durable-tui-reconnect.md).

## Glossary

- **Input**: an event with a caller-supplied globally unique ID that remains
  stable across redeliveries.
- **Inbox**: session-scoped, in-memory deduplication of external, control, and
  crash inputs.
- **Session**: append-only persisted history that can be forked.
- **LLM turn**: the coordinator-managed sequence around one logical LLM request.
- **Tool**: a capability described by a schema and bound to a translator.
- **Tool call**: a model-produced request to use a tool.
- **Tool translator**: validates a tool call and translates it into one or more
  operations. It runs synchronously on the coordinator's event loop and must not
  perform I/O or suspend the loop.
- **Tool call status**: the translation outcome: a validation error or references
  to submitted operations. Operation execution state is tracked separately;
  the translator formats these into a model-facing result.
- **Operation**: a serializable description of work produced by a tool translator
  for asynchronous execution. Implementations are encouraged to use the available
  [primitives](harness/primitives/).

## Components

| Component | Responsibility |
| --- | --- |
| Session inbox | Volatile, session-scoped input idempotency. |
| Coordinator | Persist accepted inputs, run LLM turns, resolve tool translators through the registry, and dispatch committed operations. |
| Session store | Persist canonical session history and operation state; support recovery and forks; atomically record tool-call status with operations. |
| Context builder | Statefully assemble model input in memory. Return the model input together with a record of anything omitted, truncated, or compacted. Perform no I/O and accept no persistence dependencies. |
| LLM Adapter | Send prepared model input to a provider and return a normalized completed response. Own authentication, cancellation, and provider errors. |
| Tool registry | Own the fixed Bash, ViewImage, and skill-use definitions and their translators; expose the host-selected set. |
| Tool translator | Validate a tool call and produce its status and operations. Format a recorded call status and prepared operation output into model results. Perform no I/O. |
| Operation manager | Actor runtime for durable operations. The local implementation is swappable. |

## Extending the harness

Harness components are composable, and alternative implementations of their interfaces are encouraged.

We intend to preserve these invariants:

- Session-store items are serializable, and the storage format is versioned.
- We'll do our best to maintain backwards compatibility for sessions.
  An unsupported session version will always cause an explicit error on resume.
- Operations are versioned and always serializable.

For example, a proxy operations manager can send serialized operations to a
local operations manager running in a process inside a remote sandbox, allowing
tools to execute there.

## Compaction and terminal model failures

Automatic compaction requires eligible committed history. A summary replaces
that history only when the response completes successfully with nonblank
assistant text; empty, whitespace-only, non-assistant, refused, truncated, and
failed summaries preserve the original builder and pending ordinary input.
Consecutive unusable summaries are limited to **three** by default. Embedders
can set `coordinator.Dependencies.CompactionFailureLimit` to a positive value;
zero selects the default and negative values are rejected. The policy is
recorded before the first summary request and takes precedence over dependency
settings on subsequent resumes. Adapter/transport retries are separate from this
coordinator-level summary bound.

At the bound, the coordinator persists a `compaction_disabled` control notice
and continues ordinary work using the original context. The notice appears in
the session event stream and the TUI. Enter **`/compact`** in the TUI to request
one manual attempt, including when automatic compaction is disabled. This queues
behind an active request and does nothing when no history is eligible. A usable
manual summary resets the failure count and re-enables automatic compaction;
an unusable one preserves context and does not start an unbounded retry loop.
Programmatic clients can submit an inbox control with `{"Mode":"compact"}`.

The additive control modes `compact`, `compaction_policy` (with
`{"Parameters":{"FailureLimit":3}}`), and `compaction_disabled` do not change the
session file version. New binaries read older histories. **Older binaries reject
these new controls as unsupported** and therefore cannot resume histories that
contain them; use a new binary rather than dropping these records or attempting
a downgrade. Response records derive the retry count; the policy and disable
notice make recovery decisions durable, including a crash between the last
unusable response and its disable notice. Duplicate responses are ignored.

A regular response with `Failure` and no stop reason is durably recorded,
including its usage, before the coordinator returns `*coordinator.ModelFailureError`.
The runner consequently exits nonzero. Failed output is diagnostic only, except
that assistant messages from a `StopRefused` response remain in conversation
history even when the adapter also sets `Failure` (as MessagesAPI does).
Refusals remain ordinary responses and allow subsequent user follow-ups. Tool
calls from any response carrying `Failure` are excluded from context and never
scheduled, either live or during replay (including legacy recorded operations
from such calls). Unsuccessful summary responses follow the bounded compaction
policy rather than terminating the run.
Resuming a terminal model failure without new external input surfaces the last
failure without another model request. New recorded or queued external input
permits a new turn; settings, heartbeats, and stop controls alone do not retry it.
Already-completed operations are not dispatched again.

### Reloading the agent worker

The TUI uses a supervised resident worker. Use `/reload` to replace worker code
at an idle safe point while keeping the terminal and conversation open. Build
changes separately first; no workspace code is built automatically. Use
`-worker-executable /absolute/trusted/unreal-agent-tui` to select a replacement
explicitly, or `-session ID` to resume after a frontend restart.

Reload is scheduled while model/tool/subagent work finishes naturally. Graphs
awaiting acceptance and retained helpers require explicit intervention; reload
never accepts results or kills work implicitly. New ordinary inputs are durably
queued; scoped `/graph` and `/agents` controls remain available. Canceling a client
wait does not undo an admitted reload. Accepted,
session-persisted and model-delivered are distinct states. UI-code updates still
require a frontend restart. See [reload lifecycle and limits](docs/plans/hot-reload.md).

[Phase-specific image recovery (M2.2): policy, process tests and installed verification](docs/verification/durable-image-recovery-m22-20261028.md). Historical slice checkpoint; later classified capabilities are linked below.

[Bounded durable regular-model request recovery (M2.3), incomplete-tool fencing and installed verification](docs/verification/durable-model-request-m23-20261028.md). Compaction and later capabilities have their own classified evidence.

Classified recovery and replacement checkpoints: [M2.7 classified command graph recovery](docs/verification/durable-graph-m27-20261028.md). [Opt-in contract and limits](docs/plans/durable-graph-recovery.md). [Frontend upgrade proof](docs/verification/durable-frontend-upgrade-m31-20261028.md). [Idle runtime replacement](docs/verification/durable-runtime-upgrade-m32-20261028.md). [Natural-drain upgrade](docs/verification/durable-runtime-drain-m33-20261028.md). [Candidate failure and compatibility](docs/verification/durable-candidate-failure-m34-20261028.md). [Controlled host replacement](docs/verification/durable-host-replacement-m35-20261028.md). [Trusted extensions, independent jobs, durable timers and verified-TLS remote work](docs/plans/durable-workloads.md). [Final M4 validation, real worker/client/private-tmux evidence and installed pair](docs/verification/durable-workloads-m4-20261028.md). [Roadmap](goal.md): **27/27 defined slices accepted**. Unknown ownership/versions remain refused; this is not arbitrary plugin replay, a sandbox, power-loss certification or exactly-once third-party delivery.
