# Supervisor I/O isolation follow-up — upstream-first

Status: planned, not implemented. This follows the residual risks in
[the supervisor async audit](supervisor-async-audit.md), not a redesign of the
harness or a claim that all remaining I/O is already asynchronous.

## Philosophy and comparison baseline

Use upstream's async-first architecture as the design constraint, not merely as
an API compatibility target. Keep the smallest reviewable difference:

- Translators and the context builder perform no I/O. Operations stay serializable
  and versioned; dispatch follows commitment. Execution belongs in owned runtimes.
- Actors retain state ownership. Workers perform blocking I/O on immutable inputs
  and return typed completion events; they do not mutate actor state.
- Preserve `RemoteJobHandler`, `inbox.Writer`, operation-manager contracts and
  existing session reads. No new framework, dependency, durable queue, or wire
  protocol is justified by this plan alone.
- Before changing any shared interface or invariant, compare with `upstream/main`
  and raise the divergence with the user. Do not silently broaden acknowledgment,
  cancellation, delivery, or recovery promises as part of a scheduling refactor.

Planning compared local `b4c3f82` with the locally available `upstream/main`,
`a5f3fd13032737142916523ae4344c392292f9d5`. The double-check also queried live
upstream main with `git ls-remote`; it matched this revision. No source merge or
checkout was performed. Recheck before implementation, recording both revisions
and relevant diffs.

Before introducing helpers, compare existing upstream primitives and ownership
patterns. Reuse them where their semantics actually fit; do not weaken the secure
artifact writer to force reuse or add new dispatch contracts without review.

Existing local divergence matters: upstream remote output preparation only bounds
inline output (and has a TODO for file capture). Local `UpdateRemoteJob` performs
artifact writes/fsync and has optional output-directory/path fields, still on
remote-job version 2. The public handler signatures are unchanged. Preserve reads
of upstream/older version-2 records without these fields. This is backward-read
compatibility, not a promise of downgrade safety or artifact portability between
hosts. Never manufacture a complete artifact from an already truncated preview.
The audit's description of these artifact changes as “uncommitted” is historical;
the current code contains them. Inbox Freeze/Thaw and TaskGraph are existing local
extensions, not reasons to expand the shared API further here.

## Scope and implementation sequence

### 0. Lock down contracts and failing regressions

Inspect `cmd/internal/subagent/supervisor.go`, `harness/taskgraph/runtime.go`,
`harness/operation/remote_job{,_output,_artifact_unix}.go`, and existing lifecycle,
artifact, inbox and request-mailbox tests. Add private deterministic blocking and
failure seams, not production-global hooks or sleep-based scheduling proofs.
Inventory all direct/indirect `UpdateRemoteJob` callers: manager failure handling
also calls `FailRemoteJob`, and external handler admission/cancellation callbacks
execute synchronously on the manager loop. The two built-in handlers are not the
whole archive-I/O surface.

Record and review these decisions before their affected implementation chunk:

- Queue count/byte limits, capacity reservation and overload behavior for frames,
  parent messages and full archive payloads. A full channel must not block the
  shared actor. An unbounded mailbox is not a finite-memory solution.
- Maximum single-payload size and JSONL parsing/admission behavior, not just queue
  limits. Subagent stdout and one-shot runner initial/later stdin use unbounded
  whole-line reads before queue admission. Resident-worker `workerproto.MaxFrame`
  does not bound this separate protocol. New limits or EOF/newline semantics need
  explicit compatibility review; do not import worker framing rules silently.
- Cancellation before writer acquisition versus after bytes may have been sent.
  Existing cancellation wording can imply more than is proved. Stronger promises
  or changed result semantics require explicit review.
- Clean-exit batching currently completes later pending messages before the
  restarted child proves persistence; the first becomes the new run owner.
  Characterize this exception. Preserve versus correct it is an explicit semantic
  decision, not an accidental consequence of queuing.
- Parent `Submit` errors are currently ignored. Automatic retries, new failure
  reporting, or durable delivery guarantees require a separate contract decision.
- Artifact durability: the existing writer syncs the file but not directory
  entries and creates the final name before completing its write. Define the
  required crash boundary; review any directory-sync/atomic-publication hardening
  explicitly rather than equating current file sync with proven crash durability.
  The existing-file reuse path checks bytes but does not wait for or sync a first
  writer: a duplicate can report success before that writer syncs, or see partial
  bytes. Decide persistence on reuse, concurrent duplicate behavior, and recovery
  from failed/partial final-name files without weakening no-clobber protections.
  These decisions block claims of durable asynchronous artifact publication.
- Artifact disk/inode retention, including partial files and successful writes
  whose checkpoints are subsequently fenced out as stale/canceled. Memory/worker
  budgets do not bound disk usage. Retention/quotas/garbage collection affecting
  session-referenced artifacts require separate review; no deletion is authorized.

Finite memory, lossless admission, unlimited producer progress and an indefinitely
stalled consumer cannot all be promised. Prefer admission/capacity reservation
before accepting more work. Accepted stdout-derived parent messages must not be
silently dropped. Any spool or unavailable-artifact overload fallback needs review;
it is not preapproved by this plan. Cancellation/completion capacity must remain
available under load.

The initial bounds apply to new owned workers/queues, not end-to-end process
memory. Count queued, active, encoded/cloned, snapshot and duplicate-verification
buffers, plus retained unacknowledged inputs; preview rune limits do not bound full
payload bytes. Existing inbox backlog/deduplication and manager update queues are
uncapped residual risks. Moving backlog into them is not a bounded-memory proof;
bounding those shared components would require its own contract review.

### 1. Isolate child stdin, one owner per generation

Use a private generation-scoped writer with immutable encoded frames. The initial
request is frame zero; subsequent messages and ordered controls use the same FIFO.
No goroutine per write and no second writer for the initial request.

The actor owns pending messages, operations, restart decisions and generations.
Completions identify child, generation and frame. Fence stale state mutations;
workers never retain mutable `child` state as their authority.

Keep these boundaries distinct: request accepted, frame queued, bytes written,
and input persisted by the child. Ordinary message completion still requires the
matching persisted `sessionstore.Item`; write success is not that acknowledgment.
Role matters independently of plan type: a SendMessage to an idle child, or the
first replayed message, owns the resumed process and completes on its exit rather
than merely input persistence. Preserve this distinction in cancellation too.
Preserve stable input IDs and order across clean-exit replay, including frames
that never reached stdin. Failed/stopping exits must not gain automatic replay.

Synchronize queued cancellation with writer acquisition. Removing an actor's
pending entry alone cannot retract a queued frame. After a partial JSONL write,
do not append another frame as if the pipe were intact: stop that generation and
apply the reviewed replay policy. Owner hard-stop/escalation must remain possible
without waiting behind a blocked data frame. Prove process exit before reporting
run cancellation as complete.

### 2. Isolate parent inbox submission, retaining delivery barriers

Use a private serial parent-delivery owner and preserve actor-observed FIFO and
`subagent:<sessionID>:<turnID>:<callID>` identities. Local inbox admission is
volatile deduplication, not session persistence or model consumption.

A frozen/custom blocked inbox must not stop unrelated supervisor state work or
process reaping. Keep an explicit ordered continuation: a child's final result
cannot overtake its preceding parent submissions. Preserve the current ordering
boundary without making the actor wait for `Submit`.

Fence stale completions from new live state, but do not discard an accepted
old-generation parent message just because the child resumed. Resolve its delivery
and barrier using its original identity. Submission errors follow the contract
chosen in step 0, not an invented success acknowledgment.

### 3. Isolate archive preparation in subagent and TaskGraph handlers

Prefer private owned preparation workers executing the existing `UpdateRemoteJob`
preparation/encoding step off the shared actor. Capture immutable full payloads
before truncation; return a prepared checkpoint or error to the owning actor.
Only the actor installs/publishes that checkpoint, after required file persistence.
Prefer keeping checkpoint marshaling on the actor and offloading only preparation
of immutable encoded bytes. If encoding moves too, deep-snapshot graph nodes, task
dependency/read/write slices (preserving nil versus explicit empty claims), the
attempts map and pointed-to attempts, child operations, and all slice-backed raw
JSON (`State`, `Idempotency`, remote handle/plan/subscription/input). A shallow copy
or `&g.checkpoint` is not immutable. Workers must never traverse live graph state.

Serialize transitions per operation, and separate preparation concurrency from
publication order. Preserve emission FIFO across operation IDs on each existing
handler queue, including TaskGraph before/after control snapshots. Prepared later
updates wait behind earlier emissions on that handler. This intentionally retains
ordered-publication head-of-line waiting; actor responsiveness does not mean every
unrelated final result can bypass a stalled earlier archive.

Define dependency boundaries precisely: preparation completion, actor installation,
handler enqueue, downstream receipt and session persistence are distinct. A graph
control containing a mutated checkpoint, and terminal/release continuations, must
wait for the corresponding run checkpoint to be prepared and installed/enqueued.
Existing separate run/control forwarders do not guarantee downstream observation
order. Stronger receipt/persistence barriers require separate contract review.

Launch/resume is a continuation too: supervisor `addRun` currently enqueues its
handle update before `start`, and TaskGraph `advance` calls `save` before nested
`manager.Add(child)`. Async preparation must not let child startup/dispatch overtake
that checkpoint identity/preparation/enqueue boundary. Gate launch on preparation
success and recheck cancellation/generation before dispatch; cancellation while
waiting must not start the deferred child. This preserves current enqueue order,
not a durable-commit acknowledgment (the existing handler has no such handshake).
Do not describe these nested paths as having stronger persistence proof than they
actually have. Review stronger commit-before-side-effect semantics separately.

Fence by operation identity and transition sequence as well as child/attempt
generation. A resumed child must not cause an old owner's required terminal update
to be discarded. Never let late preparation resurrect terminal work, regress
cancellation, replace a newer checkpoint, or publish after closure. Explicit
continuations, not merely goroutines around helper calls, enforce these rules.

Bound active workers and retained bytes; allow independent operations to make
progress within that budget. One global serial writer merely relocates archive
head-of-line blocking; an unbounded goroutine per update is not acceptable.
Preserve no-directory inline behavior, immutable replay artifacts, stale-path
clearing, bounded honest save-failure previews, and path/permission protections.

Built-in-handler isolation does not fix manager-side failure preparation or
arbitrary synchronous handlers. Retain these as named residual risks until an
explicitly reviewed manager-path chunk addresses them; do not claim all archive
I/O is isolated after this step. Include oversized manager-generated errors and
blocking handler callbacks in the inventory/regression characterization.

Do not move archive writes into the manager's shared update loop: that blocks a
larger actor, and the existing helper may already have discarded the full text.
Changing the helper contract to emit raw checkpoints, introducing dispatch types,
or changing shared interfaces/storage semantics requires upstream comparison and
user review first. Extract a small shared private mechanism only if both handler
implementations actually need it; avoid a general asynchronous-I/O framework.

TaskGraph notification delivery (`runtime.notify`) has a separate
submission-before-checkpoint ordering rule. Do not turn it into fire-and-forget
work while moving archive preparation. Preserve the pure reducer and current
nested-context cancellation/join ordering. A notification outbox is separate work.

### 4. Integrate lifecycle, then validate and ship

Register all workers before launch and stop registration before joining. Shutdown
requests cancellation of submissions, unblocks mechanisms it owns (including
owned stdin writes), and joins writers, preparation, delivery, stdout readers and
actual process waits. Callback/filesystem return is still required for a full
join. Workers must not need a completion send to an exited actor to terminate.

Preserve graceful retirement ordering: request `StopHard` while coordinator, inbox,
manager and update consumers remain live; consume and persist actual terminal
operation updates before canceling that infrastructure, then join all owners
before releasing leases. The watchdog/escalation fallback is not proof of terminal
state and must not synthesize successful persistence. This preserves the existing
worker shutdown contract rather than inventing a new one.

Process exit and ordered stdout/delivery drain are separate proofs. The current
subagent reader calls `command.Wait` after stdout handling; review independence
from blocked observers. Do not simply call `Wait` concurrently with reads from
`StdoutPipe`, which can close before draining. If needed, use explicitly owned
pipes with separately tested drain/reaping ownership, following existing workerhost
lessons rather than duplicating a new supervision framework.

A deadline reports incomplete cleanup and retained ownership, never success.
Cancellation-ignoring callbacks/fsync can still prevent a full join and retain
their reserved worker/byte capacity. Do not spawn unlimited replacements for
stalled workers or release ownership early. A successful retry of `Wait` must
prove no subsequent updates or callbacks. Keep reload busy
until pending owned work/publication is resolved; do not release leases early.
Coordinator idleness cannot discover arbitrary private queues: keep unresolved
work represented by a visible nonterminal operation/barrier or an explicitly
reviewed equivalent. Do not make the coordinator falsely idle by publishing a
terminal update before the associated delivery/preparation continuation resolves.

## Acceptance evidence

Use deterministic gated regressions, including real helper processes where pipe
capacity, EOF, signals and process exit matter:

1. Block an initial request larger than pipe capacity, then a later stdin frame:
   unrelated children, updates and cancellation continue; initial-first FIFO and
   framing remain intact; no delivery completion merely from queue/write success.
2. Cancel queued/acquired/partially written messages; prove truthful outcomes,
   no trailing corrupt JSON, and exact stable-ID replay on clean exit only.
3. Freeze/block parent admission: supervisor and process reaping continue while
   the affected final result remains behind preceding parent deliveries. Release
   the gate and verify FIFO, deduplication and old-generation message preservation.
4. Block archive A: independent actor work and cancellation remain responsive;
   available worker capacity serves unrelated operations. Same-operation updates
   remain ordered, no path appears before persistence, and late completion cannot
   overwrite a newer terminal decision or graph checkpoint. Gate two different
   control IDs to prove handler FIFO and before/after cancellation snapshots. Gate
   a run checkpoint and dependent control response separately; verify the declared
   enqueue boundary without claiming downstream durable acknowledgment. Mutate,
   retry and cancel graph state while preparation is gated; immutable snapshots
   retain their own transition and old-owner terminal updates survive restart.
   Gate initial handle/attempt preparation: no child `Start`/nested manager dispatch
   occurs before the specified enqueue boundary; cancellation or preparation
   failure while gated prevents the deferred launch. Also arbitrate cancellation
   against a completion already prepared but waiting to publish, not just an
   actively writing worker.
5. Inject write/sync/close failures, partial existing artifacts and concurrent
   identical writes; verify honest fallback, crash/replay behavior and the chosen
   directory/publication durability contract. In particular, gate the first writer
   after complete bytes but before sync, attempt identical reuse, then fail that
   sync: reuse must not claim a persistence guarantee it has not proved. Retain
   symlink/hardlink/privacy tests and unsupported-platform honest fallback.
6. Exercise upstream version-2 fixtures without new fields, old truncated previews,
   no-directory configuration, changed payloads, and UTF-8 size/preview accounting.
7. Saturate count/byte budgets: assert worker/goroutine/descriptor/retained-payload
   limits, documented overload handling and usable cancellation/completion paths.
   Test near/over-limit, malformed, fragmented and indefinitely unterminated JSONL
   frames and EOF boundaries under the reviewed protocol contract. Distinguish
   bounded new queues from residual parser/downstream allocations; demonstrate
   allocation limits before enqueueing only where an agreed parser limit applies.
   Exercise subagent stdout, one-shot initial JSONL request and later stdin inbox
   input separately, recording each path's final-non-newline compatibility. Tests
   of malformed/oversized resident-worker frames do not cover these readers.
8. Exercise EOF-before-exit, exit-before-drain, shutdown deadline then successful
   join retry, no post-close callbacks, terminal restored operation evidence and
   actual same-session restart. Extend stop/EOF/caller-cancel/output-failure tests
   to active subagents and new writers/delivery/preparation owners. Inspect latest
   persisted tool-call snapshots as well as restored outstanding operations; prove
   cancellation did not replay effects. Gate the real terminal update and its
   session-store commit separately: until commit, coordinator/inbox/manager/update
   consumers remain live and the lease stays held; after commit, infrastructure
   cancellation and joins precede lease release. Keep forced/watchdog cases
   separate. Existing worker-runtime snapshot assertions cover shell/graph; extend
   them to subagents and new owners instead of relying on empty outstanding state.
9. Gate each new hidden queue/barrier: queued/acquired stdin, pending parent
   delivery/final-result continuation and prepared-but-unpublished archive updates.
   `/reload` remains busy without retirement, generation change or lease release.
   After release, verify retirement only after the specified barriers resolve.
   Retain TaskGraph ownership and completed-but-unaccepted reload regressions.

Existing lifecycle/artifact/inbox/mailbox/reload tests are baseline contract
guards, not proof of the proposed isolation. Joining a blocked callback does not
prove actor responsiveness; synchronous artifact replay does not prove concurrent
durable publication or bounded workers. Record which new gated regressions fail
on the baseline and require evidence from the implemented generation before
marking acceptance items complete. Supervisor lifecycle tests prove joins, inbox
freeze tests prove fencing, artifact tests prove synchronous recovery/replay, and
current reload tests prove refusal for visible busy work/unaccepted graphs.
`TestRuntimeMailboxUnboundedFIFO` deliberately tests uncapped admission. None of
these establishes responsiveness, resource bounds or quiescence visibility of
the new private I/O owners.

Start with focused race tests:

```sh
go test -race ./cmd/internal/subagent ./harness/operation ./harness/taskgraph ./harness/inbox -count=1
go test -race ./harness/coordinator -count=1
go test -race ./cmd/internal/workerhost ./cmd/internal/workerruntime ./cmd/internal/agentrunner -count=1
```

Then run `make test check build` (both Go modules), and applicable TUI fake-model
verification if observable behavior changes. For actual code delivery, atomically
reinstall both built binaries into `~/.local/bin` per `AGENTS.md`; UI code changes
also require a frontend restart. This documentation-only plan does not require a
rebuild/reinstall. Baseline test runs can check existing contracts, but supply no
runtime acceptance evidence for the unimplemented design.

If inbox admission or reducer/runtime-fed events change, apply the corresponding
repository safety skill and extend its explorer where needed. Existing bounded
model checks do not cover these proposed external I/O owners. Run the eventual
implementation checks on Linux and macOS, matching CI; one Linux run is not
platform-complete lifecycle or durability evidence.

## Recorded baseline results (not design acceptance)

The planning pass recorded the following Linux/Go 1.27.3 results. These are
historical observations, not a fresh validation of every later worktree edit;
the documentation/formal review did not inspect or rerun concurrently edited Go
tests. Reproduce them on the implementation generation before relying on them:


- Live upstream main matched the comparison revision above.
- Race suites passed for subagent, operation, inbox, coordinator, workerhost,
  workerruntime, agentrunner and the TUI module (including its subpackages).
- The combined race run used `-count=1 -timeout=240s`. TaskGraph
  `TestGraphExhaustiveSafety` hit that time limit while exploring states; it did
  not report an assertion or race failure. That full race model-check run is
  **not verified**.
- TaskGraph race tests excluding only that exhaustive test passed with
  `go test -race ./harness/taskgraph -skip '^TestGraphExhaustiveSafety$' -count=1 -timeout=120s`.
- The exhaustive reducer check separately passed **without race instrumentation**:
  `go test ./harness/taskgraph -run '^TestGraphExhaustiveSafety$' -count=1 -timeout=180s`.
- `make check` passed (vet in both modules and formatting). Documentation relative
  links and `git diff --check` were checked. No build/reinstall was needed.

These results do not pass any of the new gated design acceptance items, do not
constitute a full root-module race run, and do not establish macOS behavior.

Ship small reviewed chunks in the order above. Stdin and parent-delivery changes
share supervisor ownership and must not be edited concurrently without coordination.
TaskGraph/archive work can proceed independently only after shared ordering and
resource contracts are agreed. Update the original audit with completion evidence
only after implementation is validated; do not erase its historical baseline.
