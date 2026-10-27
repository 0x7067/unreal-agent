# Session-local supervision core

`NewHub(ctx)` owns a bounded, session-local capability registry. Supply the same
`Config.Hub` to direct and TaskGraph-created supervisors, and register
`Hub.Handlers()` once with the root operation manager. `Hub.Snapshot()` is a
bounded detached in-memory read: it never calls the actor, filesystem, process,
model, inbox or manager. `Hub.Wait(ctx)` joins its control workers, not child
supervisors; each supervisor must also be joined with `Supervisor.Wait(ctx)`.

## Identity, controls and compatibility

- `tool/subagent.ControlPlan` and `ControlOutcome` define the additive
  `subagent.control/v1` protocol. The coordinator must persist the operation before
  dispatch. Both opaque handle and positive attempt generation are mandatory for
  status/message/stop. Display names are labels, never fleet routing authority.
- Handles are random per Hub/attempt, including resumed attempts. Old handles
  cannot reach replacements, same-named children in other supervisors or another
  Hub. Terminal generations remain tombstones. `RunHandle` retains `SessionID`
  and adds `Handle`/`Generation`; SessionID-only historical records remain readable.
  Resuming into a new Hub deliberately does not revive old control capabilities.
- Admission allocates a capability and snapshot before launch. Handle preparation
  and handler enqueue precede launch, with a fresh cancellation check. This is the
  existing **enqueue** boundary, not a downstream persistence acknowledgment.
- Status/list emit typed `snapshot` outcomes. Running-message completion requires
  the matching persisted child `ItemInput`: `Delivery=persisted` does not prove
  model consumption. A message that owns a resumed process instead finishes at
  process/output/delivery drain with `Delivery=process_drained`, not a fabricated
  input-persistence acknowledgment. Ordinary admission, queued, write-started,
  written, canceled-before-write and persisted observations remain distinct.
- Stop is out-of-band process-context interruption, independent of a blocked
  stdin data frame or unavailable model. Its receipt says
  `cancellation_requested`, `DrainConfirmed=false`. A repeated stop after the
  real lifecycle barrier may say `drained`. Canceling a receipt cannot undo an
  already-dispatched structural stop. Controls do not accept arbitrary PIDs.
- Stop targets the owned agent subtree, not siblings or the whole supervisor.
  Arbitrary descendant-operation cancellation is not part of control/v1; no
  unsupported descendant operation is silently treated as a broader stop.
- This implementation changes normal runner escalation from the former 10-second
  forced kill to `exec.Cmd.WaitDelay=0`. SIGINT is a request. An uncooperative
  runner retains ownership indefinitely; a caller's Wait deadline reports
  incomplete cleanup. Killing only the runner would not prove its owned nested
  processes drained. Higher-level watchdog outcomes remain separately unconfirmed.
- Clean-exit replay preserves input IDs and order. The first replayed message owns
  the new process. Later messages now require actual matching input persistence;
  the historical batching exception that completed them before acknowledgment has
  been corrected. Failed/stopping exits do not automatically replay messages.

## Reports, cleanup and diagnostics

A proven report is a successful complete model response containing text and no
ToolCall. Arbitrary commentary/text, refusal, truncation and incomplete responses
are not that boundary. A report is not acceptance or execution drain.

Empty lifetime keeps historical reusable-agent behavior. Explicit `background`
retains work. Explicit `task` initiates structural cleanup on a proven report.
Self-cleanup is a lifecycle flag, **not** an operation cancellation transition:
the owner stays Awaiting until actual drain, then may complete with its report.
An explicit owner cancellation takes priority and follows Canceling -> Canceled.
Parent inbox failure remains a failure, even during report cleanup.

The parent Agent cannot become terminal merely because report text appeared.
Process Wait, ordered stdout consumption, owned stdin writer/reader completion
and preceding parent submissions must all reach their barriers before its terminal
update is prepared/enqueued. Explicitly owned stdout pipes keep process Wait
independent from a blocked output consumer; `StdoutPipe` is not raced with Wait.
Global shutdown stops new admission but continues finite completion/publication
continuations before joining. A successful Wait is not proof that the parent
persisted its terminal update; handler enqueue, downstream receipt and parent
session-store commit remain different boundaries.

Snapshots expose report/process/readers/EOF/cancellation/publication timestamps,
phase start, purpose, lifetime, parent task/resources and graph/task generation.
Progress evidence is a persisted-child-session observation, not an inferred stall.
Operation statuses are last observed persisted snapshots, **not** every private
manager transition. Fleet views do not resurrect historical pending operations
after the owner is actually drained. The supervisor-local diagnostic API retains
historical observations explicitly. Relevant blocker paths select the oldest
retained nonterminal observation; bounded taskgraph checkpoint projection adds
nested child operations (maximum depth four). Large/unavailable checkpoint details
remain unknown, not a guessed blocker. Arbitrary nested supervisors in another
runner process require that process's own telemetry; this is not process-table
scraping or a claim of omniscient cross-process state.

Shell summaries retain only an exact allowlisted executable token (`find`, `go`,
`git`, etc.). Never arguments, command text, environment assignments, paths,
output, prompts or provider reasoning. This is diagnostic redaction, **not** a
shell security denylist. Purpose/resource metadata is explicitly supplied by the
caller and bounded, not extracted from prompts.

## Finite resource and ordering contracts

- 64 registered supervisors per Hub; 128 lifetime attempt capabilities per Hub,
  including tombstones. Exhaustion rejects new registrations/attempts explicitly;
  `OmittedAgents` records rejected handle starts. No tombstone recycling can make
  an old handle authoritative again.
- Per supervisor: 64 child slots, 256 lifetime admitted operation IDs, 16 MiB
  aggregate admitted operation state, 128 pending ordinary messages. Per Hub:
  256 lifetime control IDs and 16 MiB admitted control state. Exhaustion is an
  admission error, not successful cleanup. Dedup/tombstones last the live scope;
  a new Hub is a new authority epoch.
- **Private runner JSONL compatibility restriction:** a complete stdout or stdin
  frame is limited to 8 MiB (including framing). Oversize input fails explicitly;
  oversize stdout stops that attempt with a protocol failure. Frames are not
  truncated into different JSON. A valid final non-newline stdout frame remains
  supported. This does not change workerproto or bound the separate one-shot
  runner's initial/later stdin parser.
- One FIFO stdin writer per process generation: 32 queued frames and 8 MiB total
  active/queued encoded input. Cancellation and writer acquisition synchronize on
  each frame. After a short/failed write, that writer stops; it never appends a
  later frame to a possibly corrupt JSON prefix. Queued cancellation can retract
  the frame; cancellation after acquisition cannot promise retraction.
- One serial parent delivery owner per supervisor: at most 128 frame batches and
  16 MiB reserved source bytes, plus bounded encoding/decoded-payload overhead.
  Reservations precede actor admission. Saturation backpressures only readers;
  actors and independent process Wait remain responsive. Parent submissions keep
  actor-observed FIFO and stable session/turn/call identities. Submit errors are
  explicit failures, not durable delivery or automatic retries.
- Two artifact-preparation workers per handler (four total per supervisor).
  Preparation may run concurrently, but sequence-numbered actor publication is
  FIFO for each handler. Prepared later checkpoints cannot overtake earlier
  checkpoints or launch continuations. Active, queued and prepared-unpublished
  work share a 32 MiB reservation. Full-budget terminal results retain a bounded
  retry FIFO (bounded by admitted operation IDs, sharing owned immutable reply
  strings), not an unbounded spool or goroutine per event. Child retained replies
  and reader-local frames have an additional 8 MiB/child ceiling; source-frame,
  decoding, encoded, snapshot and artifact-helper copies are additional bounded
  buffers, not a claim of a 32 MiB whole-process memory cap.
- Snapshots retain 32 main operation IDs and one overflow operation, plus 32
  input observations per attempt. Main terminal IDs remain sticky tombstones for
  the attempt. Once the main inventory fills, the extra slot retains a later live
  blocker (including its bounded nested path), so routine completed history does
  not hide a subsequent quiet shell. Snapshot `RelevantOperation` falls back to
  this slot when the main inventory has no live operation. A matching terminal
  update clears its relevance and stays sticky in that slot; only a *distinct
  live ID* may replace the terminal overflow tombstone. `OverflowEvictions` counts
  those replacements. After eviction the old overflow ID is no longer retained
  as a tombstone; sequence/generation fencing still rejects old-frame replay, but
  no unbounded per-ID terminal history is promised. While the overflow slot is
  live, additional distinct operations are omitted rather than displacing that
  blocker. Actual omitted observations/frames are counted; counts are not exact
  omitted-live work. Projection parsing is 1 MiB/frame, operation details 16 KiB, labels 128
  UTF-8 bytes, resources 16 labels. No query scans storage or acquires I/O locks.
  Typed operation IDs too long for exact matching are omitted, never truncated
  into authority. New handler admission rejects oversized owner identities.

Artifact workers call existing `operation.UpdateRemoteJob`; they do not invent
fsync, atomic-publication, directory durability or parent-commit guarantees.
Existing artifact retention/reuse behavior, manager-side artifact preparation and
arbitrary third-party synchronous callbacks are outside this package. Shared
inbox/manager queues are not made bounded by these private limits. A callback or
filesystem call that ignores cancellation still prevents full join, with visible
ownership retained. No filesystem spool or artifact deletion is introduced.

## Evidence

`go test -race ./cmd/internal/subagent -count=2` passes for the current core
candidate, as does `go vet ./cmd/internal/subagent`. Deterministic fake gates and
real helper processes separately prove report-before-drain, task/background
policy, prelaunch cancellation, independent Wait/EOF, blocked stdin and parent
inbox, FIFO preparation/publication, persisted-message acknowledgment, queued
retraction, stale/cross-Hub/replacement handles, historical SessionID-only reads,
capacity, framing compatibility and nested redaction/snapshot ownership. Helpers
are released, canceled and joined in cleanup; no model or filesystem-root scan is
used. These tests do not prove semantic report acceptance or arbitrary OS syscall
termination. Full integrated suites/build/install and UI restart are the parent
integration track, not claimed here.
