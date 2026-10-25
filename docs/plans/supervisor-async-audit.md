# Supervisor asynchronous-behavior audit

Planned residual I/O work: [upstream-first follow-up](supervisor-io-follow-up.md).
This link is a plan, not evidence that the residual risks below are resolved.

## Baseline and contract

The comparison baseline is commit `8828795`, before the worker-host feature in
this worktree. The baseline TUI owned the inbox and coordinator in-process.
Worker hosting must not turn message admission, result observation, or ordinary
tool scheduling into a wait for a model response or tool completion.

There are three separate acknowledgments:

1. Local acceptance: the frontend has queued an immutable input and echoed it.
2. Durable ingress admission: `Host.Submit` has synchronized that input to disk.
3. Session persistence: a typed worker item proves the worker stored the input.

None implies that the model has consumed the input or that its tools have finished.
Local queue entries explicitly remain unconfirmed until the appropriate later
acknowledgment; local acceptance is not a crash-durability promise.

## Findings and corrections

| Path | Comparison with baseline | Required behavior |
| --- | --- | --- |
| Frontend result decoding | New capability-only registry installed unavailable placeholders, incorrectly marking successful worker tools as failed. | Use inert typed result translators. Execution runtimes stay exclusively in the worker. Test actual framed events, running/success/error/canceled states, and replay. |
| Repeated message submission | The existing single-flight Enter gate now waits for disk synchronization rather than the old in-memory inbox handoff. | Queue immutable submissions in Enter order before scheduling commands. A single sender reports results per input ID; later Enter presses must not wait for earlier durability acknowledgments. |
| Admission/lifecycle lock | Host submission held the shared host mutex across journal write/fsync, blocking unrelated lifecycle and observation work. | Register in-flight admission under the mutex, perform disk I/O outside it, and join registered admissions before releasing journal ownership. Shutdown cancellation must not wait to acquire a disk-I/O-held lifecycle lock. |
| Worker stdout consumption | Journal acknowledgment and callbacks ran directly in the stdout reader, allowing slow observation to fill the pipe and stall worker persistence/coordinator progress. | Decode into an ordered mailbox with one delivery owner. Ready/quiesce and terminal barriers remain behind preceding items. Slow observation must not prevent decoding or independent worker/model progress. |
| Exit transcript drain | A proposed one-second post-exit stdout cutoff could discard committed frames. A negative overlay regression reproduced loss of the second of two persisted items. | Process exit and ordered output drain are separate barriers. Do not close a live decoder merely because a callback is slow. |
| Inherited stdin shutdown | In-process `os.Pipe` tests masked inherited blocking stdin that cannot accept read deadlines. | Prepare a private pollable input descriptor on supported platforms, join its reader, restore flags, and preserve the caller's descriptor. |
| Graceful session shutdown | Signal cancellation bypassed the baseline hard-stop protocol: processes exited but Bash/TaskGraph/subagent operations remained durably awaiting, making the session unresumable. A negative overlay reproduced all three cases repeatedly. | Request hard stop while the coordinator can still persist proven cancellation outcomes, then cancel/join infrastructure. Verify terminal restored operations and actual same-session restart; forced/unconfirmed termination stays fail-closed. |
| Abandoned successful retirement status | The original consumer suppressed stopped publication for a clean retired owner after Reload had returned, leaving stale ready status. | Publish stopped for an owning, non-closing generation before clearing current/closing its process-done barrier. Keep host finish conditional on non-retirement or failure: clean retirement leaves Done open, preserves durable admission, and permits later explicit reload recovery. The [formal models](../formal/handoff/tla/README.md) retain historical defect witnesses and a corrected-status candidate reflecting the shipped repair; they do not prove Go refinement. |
| Worker exit ownership | Waiting for stdout EOF before `Wait` could prevent escalation; clean stdout EOF did not prove process exit. | Join the process independently, cancel on unexpected EOF, isolate worker/probe process groups, and retain sufficient frontend deadline headroom for reaping. |

## Preserved asynchronous paths

- The inbox remains an actor-owned FIFO, independent of coordinator consumption.
- The coordinator launches model requests asynchronously. External input can
  steer a new turn while tools remain running.
- Tool progress, completion, graph candidates, and subagent messages remain
  independent events; no frontend wait for a complete task was introduced.
- The frontend event mailbox buffers callbacks instead of synchronously calling
  `Program.Send` from persistence observers.
- Reload runs outside the UI update loop. Probe validation, temporary delivery
  fencing, idle refusal, old-process joining, and lease handoff are deliberate
  ownership barriers, not normal tool-execution joins.
- Shutdown joins writers, operations, graph runtimes, subagents, and active model
  calls before releasing their owned state. A deadline is not proof of cleanup.

## Costs and boundaries not hidden by this change

- Durable admission still requires disk I/O. Journal acknowledgment and admission
  serialize on the journal itself; the frontend queue and stdout decoder must
  isolate that cost rather than falsely acknowledge durability early.
- Lossless volatile queues can grow when consumers stall. They are not an
  unlimited-memory guarantee, and queued-but-unpersisted input is not durable.
- Deliberately detached foreign processes and caller-supplied uninterruptible
  readers are outside ordinary owned-process cleanup. An incomplete drain or
  callback must produce an honest deadline/retained-ownership outcome rather
  than silently lose output and claim completion.
- Subagent actor stdin writes and parent inbox submission were already
  synchronous before this feature. They can cause actor-local head-of-line
  blocking; they are not newly introduced whole-task joins.
- Separate uncommitted remote-output artifact changes add synchronous archive
  writes/fsync inside subagent/taskgraph actors. This is a residual actor-local
  blocking risk, distinct from worker supervision. Moving archive persistence
  outside the shared actor requires preserving durability-before-publication
  and per-operation ordering; this audit does not claim that work was done.

## Validation targets

Use race-enabled root and TUI suites. The targeted regressions must demonstrate:

- successful framed results do not appear failed and decoding has no execution
  side effects;
- blocked first submission does not reject later Enter presses, even if command
  execution order differs, and backend admission remains FIFO;
- a blocked observer does not prevent a second real model request, including
  output larger than the pipe buffer;
- final frames survive worker exit while an earlier callback remains blocked;
- abandoned successful retirement publishes the correct generation/build stopped
  status before the process-done barrier, keeps host Done open, and permits
  durable submission and later explicit reload recovery;
- slow durable admission does not prevent shutdown cancellation or deadline
  reporting;
- shutdown deadlines retain ownership until a successful retry joins remaining
  work, with no callbacks after successful closure;
- idle/model/Bash/TaskGraph/subagent shutdown reaps owned workers and releases
  their leases; successful graceful shutdown also preserves terminal operation evidence
  and permits same-session restart; reload and input-replay/deduplication guarantees
  remain intact.
