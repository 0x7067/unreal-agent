# M0.4: late-join baseline and proposed local attach v1

**Status: characterization + reviewed extraction contract, not a shipped attach
service.** Baseline `d24507b7789652736a3f9e3d89e37abcc22d046c` plus M0 tests.
[Roadmap](../../goal.md), [M0.1](durable-submission-lifecycle.md),
[M0.2](durable-lifecycle-boundaries.md), [M0.3](durable-recovery-matrix.md),
[verification](../verification/durable-client-m04-20261027.md).

M1 starts with **one explicitly started host, one session, one active local
client**. It must not reuse frontend-owned worker startup as an attach path.
Detaching a client is not stopping the durable agent. Nothing in M0.4 changes
production ownership, observer, protocol, UI, or replay semantics.

## What can be reconstructed today?

| View component | Authority / current path | Late-join limitation and required representation |
| --- | --- | --- |
| Committed conversation, turn identity, model answer/known usage, provider display | Ordered typed [`sessionstore.Item`](../../harness/sessionstore/sessionstore.go): fork, input, turn, model response, tool-call status. [`itemjson.go`](../../harness/sessionstore/itemjson.go) preserves typed payloads. | Canonical history prefix, not an atomic full-agent snapshot. Sequence identifies a position in **one session**, not a global cursor. |
| Effective model/compaction settings | Persisted control inputs replayed by [`workerruntime`](../../cmd/internal/workerruntime/run.go), coordinator/context builder. | Reconstruct from recorded controls; do not reinterpret client defaults. Selected provider/workspace belong to trusted host startup, not client override. |
| Ingress-accepted but not session-persisted inputs | Host-owned [`ingress.Journal`](../../harness/ingress/journal.go); [`Host.Submit`](../../cmd/internal/workerhost/host.go). | History alone omits these. Pending ingress and uncertainty need an explicit snapshot/reconciliation surface. Successful Submit proves ingress acceptance, not provider delivery or effect success. |
| Latest operation state | `SaveOperation` / `ResumeState.Operations` in [`localfile/store.go`](../../harness/sessionstore/localfile/store.go). | **No Item notification for operation-only updates.** Tool-call history can contain an older operation snapshot than recovery state. A history-only client cannot advertise current checkpoints. |
| Structural control execution | Versioned control-operation record, initialized separately from model history. | `InitializeControlOperation` likewise is not an Item notification. Control results/authority are not inferred from transcript length. |
| Worker generation/build, stopped/reload/error status | Host memory, `OnStatus`, generation lifecycle in [`workerhost`](../../cmd/internal/workerhost/host.go). | No independent host incarnation or reconnectable status log. Never present unavailable as running/idle. |
| Live resources/subagents | [`ActivitySnapshot`](../../cmd/internal/workerhost/diagnostics.go), sampled from one worker generation. | Live-only, independently timed; an unavailable sample is **not** an empty fleet, successful cancellation or drain. A report is not process exit. |
| Draft, focus, viewport/scroll, selected details, optimistic UI presentation | Client-local [`TUI model`](../../cmd/unreal-agent-tui/ui.go), [`submissions.go`](../../cmd/unreal-agent-tui/submissions.go). | Preserve locally if desired; not authoritative agent state. Accepted/persisted/delivered presentation needs stable input identity reconciliation. |

Existing startup history replay is safe because **no coordinator writer runs
during that replay**; the worker installs its observer before starting the loop.
That is not an independent-client snapshot/watch primitive for an already-running
worker. The store's observers are synchronous, local to that Store instance, and
must be serialized with mutations. Opening another Store does not subscribe to
filesystem writes. Paging currently decodes the entire log on each `Items` call;
small pages are **not a whole-store memory bound**.

The current host validates contiguous worker Items and suppresses replay already
seen by the host. The TUI ignores sequences at/below its watermark but does not
itself reject forward gaps; it relies on ordered host delivery. Private
[`workerproto v2`](../../cmd/internal/workerproto/protocol.go) has 32-MiB JSONL
frames and startup/activity/reload controls, but no public attach, authorization,
resume cursor or resync handshake. Its host
[`eventMailbox`](../../cmd/internal/workerhost/events.go) can grow unbounded behind
a stalled callback. Worker output writes are synchronous. A socket writer must
not be installed as a blocking callback.

## Executable characterization (not production transport)

[`late_join_test.go`](../../cmd/internal/workerhost/late_join_test.go) supplies a
**test-only** subscription and typed-history view. Only the loopback provider is
fake in the integration scenario: ingress, host, child worker, coordinator,
provider adapter, item framing/replay and session files are current production
paths. There is no fake session store or fake host lifecycle.

`TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog`:

1. Submit one stable-ID input, block its provider response and install a bounded
   test subscription **before** reading history in two-item pages.
2. Capture history through **H = 3** (settings input, external input, turn).
3. In a snapshot after-read/before-return hook, release the provider and wait
   for its committed answer callback. The real answer commits at **H+1**,
   deliberately **before the snapshot helper returns**: a caller registering
   only after that return would miss it. Assert H=3 and settings/input/turn kinds
   and input identity, not merely a nonempty prefix.
4. Apply the backlog with continuity/duplicate checks. Compare the **entire typed
   Item sequence/content** with a fresh canonical read, not just message counts.
   Inject an exact duplicate answer and assert no second append.
5. Remove the test subscriber, commit another input/answer, late-join again and
   recover those records from disk without asking the model again.
6. Give a nonconsuming subscriber one frame. Further commits force explicit
   resync and clear/release only that subscription, while all host callbacks and
   a third input/answer progress. Reconnect from a fresh snapshot and compare
   canonical history again; require each input ID once and three answers/calls.
7. Query the same worker generation and verify responsiveness and unchanged
   worker object; explicitly close/join the host during test cleanup.

Ordering:

```text
install bounded test subscription
            |
read snapshot prefix ... H
            |
after-read hook: release --> durable H+1 --> host callback queues H+1
            |
snapshot helper returns; then consume queued updates
            |
contiguous prefix H+1 == canonical typed history
```

The test does **not** claim an unprotected read-then-register gap is safe:
registration happens first and the hook forces a commit before a caller could
register after snapshot return. Delayed callbacks for records already read
are checked exact duplicates. It does not provide production socket detach,
peer credentials, reconnect status atomicity or TUI state restoration.

`TestLateJoinDriverRejectsGapsConflictsAndBounds` checks exact duplicate no-op,
conflicting same-sequence content, forward gap, wrong session, frame-count/byte
backlogs, oversize frame, view item-count/byte limits, and unchanged accepted
prefix after rejection. Fixture defaults: 1-MiB per Item encoding, live backlog
16 frames/64 KiB (one frame for the stalled client), view **64 Items/256 KiB**.
The driver consumes one bounded batch at a time; batch, view and marshalling
scratch are separate allocations. These are logical fixture bounds, **not a
claim about bounded total host, provider, store decoding or TUI memory**.

`TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint` holds the real session
lease, writes tool intent and then a valid newer shell checkpoint. A fresh store
sees the changed operation while typed history and observer count stay unchanged.
It also shows `Items(after > head)` currently succeeds with an empty page; the
fixture cursor validator explicitly rejects ahead/wrong-session cursors. No shell
is launched in this store-only gap test. It characterizes missing state rather
than silently adding an operation-watch API.

## Proposed attach v1: decisions for M1

All rules below are **proposed and unimplemented**. Protocol v1 is separate from
private worker v2 and session format v2. No version constants are changed here.
Concrete starting limits below are part of the proposed contract, not assertions
that current production buffers enforce them.

### 1. Local identity, authorization and discovery

- Filesystem Unix-domain socket only; no TCP, abstract socket, remote transport,
  browser endpoint or workspace-discovered executable. Unsupported platforms
  fail explicitly; no unauthenticated fallback.
- Both peers validate **OS peer credentials against their own effective UID**
  (Linux `SO_PEERCRED`, macOS equivalent). Socket pathname, client-supplied UID,
  PID, process name and session ID are not authentication. Same-UID trust does
  not defend against a compromised process of that user; root remains privileged.
- Runtime root: a trusted absolute `$XDG_RUNTIME_DIR` or an explicit trusted
  startup option. Validate ownership, directory type and permissions, rejecting
  symlink traversal and writable-by-other ancestors; final application directory
  **0700**, socket **0600**, process umask **077**. No implicit `/tmp`/workspace
  fallback. Use directory-relative no-follow operations and check inode identity,
  not a vulnerable check-then-open pathname sequence. Existing lease paths can
  have different validation rules; do not silently change them in M0.4.
- Endpoint key: SHA-256 of a domain-separated v1 encoding of canonical store
  namespace + session ID. Use a fixed digest basename, never raw ID/path
  interpolation; reject OS socket path length overflow explicitly. Host startup
  validates the state namespace and acquires existing **exclusive host and ingress
  ownership** before binding/advertising. The worker still owns its session lease.
- Discovery finds only a protocol endpoint, session/namespace identity and random
  host incarnation. Connect/handshake is authoritative; a manifest is not a live
  process proof. Publish any descriptor atomically, owner-readable only; no keys,
  environment, provider auth or executable-selection payload in discovery.
- A new host may remove a stale socket **only while exclusively owning the host
  namespace**, after owner/type checks, proven absence of a listener, and an
  inode-identity recheck. Timeout, permission failure, incompatible handshake or
  PID absence alone never authorizes unlink/takeover. Never unlink lock files.
  If absence cannot be established, fail closed. Do not rename/copy an active
  state namespace or automatically spawn a replacement from an attach command.

### 2. Client policy and request authority

- Exactly **one active authenticated client**, one session, one in-flight snapshot
  and **one pending handshake**. Accept/reject without spawning an unbounded
  per-connection goroutine backlog; the handshake also has a deadline.
  Reject a second client with `client_busy`; no implicit takeover. EOF/detach
  releases only the client slot. Host/worker lifetimes and leases are independent.
- Hello negotiates exact protocol v1, session/namespace, host incarnation,
  capabilities and advertised limits **before mutation**. Reject unknown versions,
  wrong session, malformed/unknown request fields and unsupported capabilities.
  Validate request IDs (bounded to 128 bytes) and payloads before admission.
- Each connection receives a fresh connection generation. Queued requests must
  still belong to that active generation immediately before admission. A stale
  connection cannot mutate a successor. Once durable input admission starts,
  disconnect does not undo it; lost replies are reconciled by stable input ID.
- Initial verbs: `attach`, `submit`, `activity_sample`, `detach`, with history/view
  acknowledgments for pacing. Correlation IDs are **not** submission identity.
  One mutation RPC and one diagnostic RPC in flight per client; no unbounded
  request/goroutine backlog. Pipelining beyond negotiated limits is rejected.
- No arbitrary worker bootstrap, credential/environment/provider endpoint,
  executable/shell/workspace override, filesystem read, structural control,
  automatic reload, abort or host shutdown in this first attach surface. Future
  authorized controls must distinguish cancellation request, completion and drain.
  Attach must not load dotenv/preferences/auth or contact the model provider.

### 3. Cursors, publication and snapshot/watch boundary

Keep distinct identities:

| Identity | Meaning / reset |
| --- | --- |
| Durable history cursor `(namespace, session, sequence)` | Immutable Item prefix, durable across a compatible restart; sequence 0 is before first. No cross-session cursor reuse. |
| Live cursor `(host_incarnation, revision)` | Monotonic published view revision in this host only; reset on host restart. Cannot establish durable effect execution. |
| Worker generation | Execution-runtime sample/source identity; changes on replacement, not client detach. Does not revive old process-control capabilities. |
| Connection generation | Admission authority for one connection; not a durable replay cursor. |

Use lossless unsigned sequence/revision decoding; reject overflow, ahead-of-head,
wrong namespace/session/incarnation and unknown cursor versions. Do not interpret
an ahead cursor's empty page as success. Exact duplicates are no-ops; conflicting
content at the same identity is corruption. Gaps require explicit resync, not
advancing the watermark. The v1 record digest is **SHA-256 of deterministic session-v2 Item JSON**:
ordered object keys, authoritative recorded timestamps, and lossless integer/raw
number handling (no float64 round trip for sequences). Fix those encoding rules
in the versioned wire codec and test them before M1 enables it. The fixture only
compares exact current typed-encoder bytes; it does not prove that future codec.
Never hash rendered text or connection-specific framing as record identity. Never persist a resume cursor ahead of applied/verified state.

**Required publication model:** a host-owned serialized view lane incorporates
source events, maintains a contiguous **published** history head and registers
clients. On that lane, register bounded delivery and capture immutable view
revision **R** plus published history head **H**. Replay exactly through H,
then `snapshot_end(H,R)`, then deliver revisions **>R**. Stage client state until
that marker validates the complete advertised prefix/components; don't render a
half-snapshot as current. Pace history reads instead of queueing all history.

H is the history prefix incorporated into the host's view at R, **not an
independent read of a potentially newer filesystem head**. The worker can have
committed records still in transit. A snapshot may be a known published prefix;
it must not falsely claim wall-clock recency. Stable source identities and
serialized application reconcile delayed history, ingress acknowledgments and
status updates. Client submit replies follow the corresponding ingress-state
publication; an acknowledgment cannot outrun its reconstructable view.

Snapshot components explicitly include: history head/prefix, pending ingress
identities/payloads plus confirmed/uncertain state, host status/generation and
effective recorded settings. Latest operation/control view is separately marked
available/unavailable with reason and provenance. **M1 cannot advertise current
operation snapshots using only existing Item callbacks**: that needs a separately
reviewed bridge/API and tests. Optional live activity samples carry generation
and observation time/revision, and are not atomically aligned with durable
history. Unavailable never means empty/idle/drained.

The test's register-before-read method proves the history-only join. Independently
reading history, pending ingress and operation state does **not** implement this
multi-component publication barrier. An added publication lane must preserve
upstream commit-before-dispatch and pure translator/context-builder boundaries.

### 4. Framing, backlog and large history

| Resource | Proposed initial v1 hard limit / response |
| --- | --- |
| Complete JSONL wire frame | **1 MiB** encoded, newline-delimited; reject incomplete trailing frame and unknown/malformed fields. Existing private worker limit remains 32 MiB. |
| Submission payload | **256 KiB** encoded; reject before ingress mutation. Accepted inputs are never dropped because a client queue fills. |
| Replay batch | **256 Items and 1 MiB** encoded, whichever first; at most one batch staged for sending. |
| Outbound live backlog | **256 frames and 8 MiB**, both enforced; at most one additional frame in the socket writer. |
| Retained live replay | **4,096 events and 16 MiB**, both enforced; evict old live events, not committed history. Cursor outside window requires a fresh snapshot. |
| Oversize Item transfer | Bounded chunks **64 KiB** (encoded frame still ≤1 MiB), one record reassembly at a time, digest/offset/length checked; **32 MiB logical record** initial cap. Larger history records return explicit unsupported-size error, not truncation. |
| Snapshot staging | Disk-backed, **256 MiB** maximum per client snapshot; reject `snapshot_too_large` without destroying history. No unbounded in-memory reassembly. Future scalable viewport/replay support is a separate capability. |
| Time / concurrency | **5-second** handshake and per-write deadlines; **30-second** snapshot lifetime; one connection/snapshot and bounded RPC counts above. A deadline closes the client, not the worker. |

Bounds include framing/metadata, not just content bytes. Release queue/staging
references on success, error and disconnect. The serialized host lane must never
wait for socket writes or for the client to drain; a bounded writer consumes
outside it. Retain bounded descriptors for large immutable history records and
stream chunks on demand; do not eagerly enqueue an entire record's chunks or
marshal it into an unbounded scratch buffer. A nonhistory component that cannot
be frozen/versioned within limits is explicitly unavailable or forces resync,
never read later from mutable latest-state under an older revision. Overflow
invalidates only that subscription and requires resnapshot.
Send `resync_required` only if bounded delivery is still possible; otherwise EOF
is the signal to reconnect/resnapshot. Never grow an emergency unbounded queue
to deliver the overflow warning. Repeated failure is surfaced, not an infinite
transparent reconnect loop. Oversize/disk-cap failures need a capability/limit
change, not identical blind retries.

These limits bound **transport/replay/staging**, not all durable history,
coordinator state, provider allocations or ingress identity tombstones. Current
full-log reads, unbounded host mailbox and pending/admission memory are explicit
implementation gaps. M1 must measure the new buffers and avoid cloning an entire
agent view into every queue; M1.5 supplies adversarial resource/peer proofs.
Basic authentication and bounded disconnect behavior are not deferred merely
because that broader hardening slice comes later.

### 5. Submission and recovery semantics

- `submit` carries stable input ID and normalized payload. Retry same identity
  and equivalent content is one logical input; changed content is explicit
  conflict. Do not replace input identity with connection/request IDs.
- Reply states distinguish **ingress accepted**, **session persisted**, **model
  delivered** and terminal result. An uncertain append/reply is not labeled
  definitely rejected/not persisted; reconcile same ID after reconnect.
- History, pending ingress and stable IDs rebuild accepted-but-not-yet-persisted
  presentation. Retrying from a fresh UI must not manufacture a second input.
  Existing ingress failure means the pending list may not be a complete recovery
  view; advertise uncertainty and fence mutation until authorized recovery.
- Host/worker restart, crash replay, migration and process control remain governed
  by [M0.3](durable-recovery-matrix.md). Connecting never relaxes unresolved-op
  refusal, reexecutes a shell or silently changes credentials/configuration.

## Upstream comparison and acceptance boundaries

Compared with locally available `upstream/main`
`a5f3fd13032737142916523ae4344c392292f9d5`, **not freshly fetched**:

- Shared Store Items/observer/sequence/paging contracts are unchanged relative to
  upstream. Current optional `ControlOperationInitializer` is a preexisting local
  interface addition. No new shared method/record/observer behavior is added here.
- Current workerhost/workerproto/workerruntime are local additions against that
  upstream; they are baseline code to preserve, not upstream attach guarantees.
- **Intentional future extension, not silently implemented divergence:** public
  same-UID socket protocol, independent host lifetime, published view revision,
  additional pending/operation snapshot surface, bounded subscriber queues and
  peer credentials. Recompare before implementing shared invariant/interface
  changes and surface any changed commit/ack/authority ordering for review.

M0.4 acceptance is the executable history-boundary/duplicate/backlog proof, the
operation/cursor gap demonstration and this reviewed security/view contract.
It does not mark any M1 transport, peer-authentication, full-agent atomic snapshot,
oversize chunk transfer, live-TUI reconnect or power-loss scenario complete.
**M1.1 is next:** headless one-session host and minimal authenticated client,
with real-process detach/reconnect and exclusive ownership evidence. Each enabled
capability must earn its own tests; proposed limits are not existing guarantees.
