# Supervised worker reload

The terminal frontend owns terminal state and a durable ingress journal. Its
resident worker owns the provider, coordinator, tools, task graphs, subagents and
session writer. `/reload` replaces only that worker, not frontend/UI code.

## User contract

- `/reload` is a local command, never a model prompt. There is no automatic
  source watcher or automatic workspace build.
- Default replacement comes from the original explicit frontend executable
  path. `-worker-executable /absolute/trusted/unreal-agent-tui` selects a
  replacement explicitly. The executable must implement `worker-probe`,
  `worker` and embedded `agent-runner` modes. Selecting an executable trusts
  its code; the protocol probe is a compatibility check, not a sandbox.
- Build a replacement separately (`make build` for local development), then
  request reload. The supervisor stages a private executable copy, probes its
  protocol/session-format/build identity and leaves the old worker running if
  preflight fails. Installed aliases/symlinks are resolved at each explicit
  selection, not guessed from the current workspace or PATH.
- Busy work blocks reload. This includes model requests, compaction, inputs not
  yet persisted by the worker, tool calls, Bash, subagents and **any unfinished
  TaskGraph**, including completed candidates awaiting acceptance. Busy refusal
  does not cancel work. Finish/accept/cancel it through normal controls, then
  explicitly retry. No forced reload is provided.
- A successful reload retains the frontend PID, terminal, transcript, draft,
  scroll and session identity. UI code updates still require a frontend restart.
- `-session ID` explicitly resumes an existing session after a frontend restart.
  Unresolved nonterminal operations refuse worker resume rather than being
  blindly executed again. Inspect/recover uncertain effects separately; do not
  delete locks or edit operation state to pretend they completed.

## Delivery and failure boundaries

There are three distinct stages: durable ingress acceptance, session persistence,
and model delivery. A successful frontend submission guarantees the first, not the
other two. Matching persisted input events reconcile the UI's local echo.

Ingress records are synced before success, identified by stable input ID, and
reconciled against replayed session history before retry. Conflicting values
under an existing ID are rejected. Host inputs use their JSON wire-normalized
representation (insignificant JSON whitespace is not a different value); the
underlying journal retains byte-exact identities. Private pipe frames require a
terminating newline and are bounded to 32 MiB. Acknowledged identity tombstones
are retained; journal compaction is not currently implemented. Admissions are
bounded (4 MiB encoded journal record, including base64 overhead). An uncertain
write/sync failure fails closed until recovery by reopening authoritative data.

During handoff, new messages remain durably queued in the frontend. Old-generation
pipe delivery is fenced. The coordinator atomically fences its inbox, drains
already admitted inputs, and makes its own final idle decision. Successful
retirement cancels/joins owned runtime infrastructure, closes provider/logs and
releases the session lease before acknowledging quiescence. The supervisor still
waits for **process exit** before launching the replacement. Ordered event replay
reconciles history without duplicating already-rendered items.

Preflight failure leaves the old worker unchanged. A timeout never authorizes a
second writer; the retirement outcome may remain unconfirmed. After the old
worker exits, a replacement startup failure leaves an explicitly stopped session.
There is no automatic downgrade after the candidate might have modified history.
An unexpected worker death is reported, not automatically treated as a safe
reload. Durable pending messages remain available for explicit resume.

## Ownership and platform scope

Linux and macOS use kernel-held advisory flock leases for frontend/ingress and
worker/session ownership. Lock files are persistent separate inodes; **never
unlink them as a stale-lock fix**. Kernel process death releases the lock. These leases are advisory: resident
workers and the public runner enforce them; direct store API callers must
participate rather than opening concurrent writers.
Directory traversal and lock opening reject unsafe aliases/types/hardlinks;
callers resolve trusted platform aliases first. Session namespaces require
current-user ownership and effective privacy through a checked private ancestor,
with shared renameable ancestors rejected. Caller permissions must remain stable;
root/same-UID mutation is outside the threat model. Ingress roots themselves are
private. Unsupported lock platforms fail closed.

This is quiescent replacement, not live-operation migration. Provider calls and
arbitrary shell/network effects cannot be given blanket exactly-once semantics.
Direct owned child processes and actor/readers are joined. Process Wait is a
direct-child boundary, not a process-tree/cgroup sandbox: detached grandchildren
and subprocesses outliving their owning command are not generally contained. Cancellation-ignoring
callbacks/providers or blocked output can prevent graceful cleanup. A timeout is
reported as incomplete cleanup, never successful reload.

The public one-shot runner keeps its original exit-when-idle behavior and now
honors the same exclusive session lease. The private worker protocol is versioned
separately; its probe does not read preferences/auth, load dotenv, open a session,
or contact a model.
