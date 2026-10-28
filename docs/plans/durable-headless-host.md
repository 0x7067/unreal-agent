# Headless host and local attach — implemented M1.1

This is the **implemented subset** of the broader
[M0.4 attach design](durable-client-attach-contract.md), not a claim that its
entire proposed protocol is shipped. Standalone TUI still owns its own worker;
[explicit TUI attach is now available in M1.2](durable-tui-attach.md). Existing
one-shot runner arguments keep their behavior.

## Explicit startup and client commands

On Linux, select an existing private, user-owned **0700** runtime directory.
`XDG_RUNTIME_DIR` normally supplies one. Its absolute, symlink-free path must
leave room for a 70-byte endpoint basename (103 bytes total). Do not use a shared
`/tmp` directory directly, change another directory's permissions, or select an
untrusted state/workspace directory. There is no automatic temporary-directory
fallback and no client-triggered host startup.

```sh
# Terminal / explicit supervisor A. Configuration and credentials here are trusted.
export OPENAI_API_KEY='...'
unreal-agent-runner host \
  -runtime-directory "$XDG_RUNTIME_DIR" \
  -session-directory "$HOME/.local/state/unreal-agent/sessions" \
  -workspace "$PWD" -session durable-demo -model gpt-5.4

# Terminal B. Reuse this globally unique ID only for the SAME logical input.
unreal-agent-runner attach \
  -runtime-directory "$XDG_RUNTIME_DIR" \
  -session-directory "$HOME/.local/state/unreal-agent/sessions" \
  -session durable-demo -submit \
  -input-id 1e1c7c67-3f1a-4e95-a337-e5a3338e17d1 -message 'Inspect this project.'

# Ctrl-C in B disconnects only B. Later, attach again without submitting:
unreal-agent-runner attach \
  -runtime-directory "$XDG_RUNTIME_DIR" \
  -session-directory "$HOME/.local/state/unreal-agent/sessions" \
  -session durable-demo
```

Use `host -h` / `attach -h` for flags. Host also accepts a trusted explicit
`-provider`, `-base-url` and absolute `-worker-executable`; the default executable
is itself, including private worker/probe and embedded child-runner dispatch.
Host does not load workspace dotenv files or TUI preferences. Provider credentials
and environment are inherited from the explicitly launching host/supervisor.
Attach reads only namespace-path defaults, never provider configuration, and
rejects provider/endpoint/model/worker/workspace overrides.

Host stays in the foreground and announces a JSONL `host_ready` record after
worker readiness, ownership and endpoint publication. It does not daemonize.
If a workspace path has symlink components, pass its physical absolute path.
The worker may start/resume authorized work before endpoint readiness; readiness
is not an input acknowledgment or a transaction covering all startup effects.
SIGINT/SIGTERM to **host** requests shutdown, using a fresh 30-second Close budget.
An incomplete close is an error, not proof of joined processes or effects;
external forced process termination can orphan work. This slice provides no
shutdown RPC, service installation or automatic restart/adoption.

## Boundary and identity

- Public local attach protocol **v1** is distinct from private worker protocol
  **v2** and session store **v2**. No session format or crash-replay rule changed.
- One explicitly launched host/session and **one active client**. Another active
  client gets `busy`; a pending-handshake slot is independently bounded. A failed
  handshake or client disconnect never closes the host, worker or their leases.
- Unix peers must have the same effective UID, checked through kernel credentials
  in both directions on Linux/macOS. Other platforms fail closed. This trusts the
  local user; it is not isolation from a compromised same-user process. Runtime
  ancestors are checked without following symlinks. The root is pre-existing,
  user-owned and exactly 0700; socket and persistent endpoint-lock files are 0600.
- Endpoint name is full SHA-256 of canonical state directory + NUL + session ID.
  Workerhost ownership is acquired before binding or stale-socket removal.
  A persistent endpoint flock, owner/type/link/inode checks and refused-connect
  evidence guard cleanup; symlinks, ordinary files and live sockets are not
  removed. Lock files intentionally remain after clean shutdown.
- Each host gets a random incarnation; each connection gets a separate random
  token. Protocol, session, correlation ID and connection identity are validated
  before submissions. Only external inputs can be submitted, not controls.

## Snapshot, updates and outcomes

The server advertises only `fresh_snapshot`, `history_watch`, `submit_external`.
Client/server messages are bounded, strict JSON objects terminated by newlines.
Unknown members, malformed/partial frames and unsupported requests fail closed.

The initial request is `attach`. The server captures published revision **R**,
committed prefix **H**, pending/status/model and subscription under one mutex.
It sends `hello`, `snapshot`, zero or more `snapshot_pending`, ordered
`snapshot_item` records through **H**, then `snapshot_end`, all at that same R/H.
Read-only filesystem replay does not silently expand H to newer persisted data.
Queued live `item`, `pending`, `status`, `pending_unavailable` updates follow at
contiguous revisions; new items advance the history head contiguously. Replies
carry current R/H but do not advance it. The client validates identity, ordering,
R/H and correlated submission replies before emitting them. A partial snapshot
is not a completed current view.

`accepted` means **confirmed durable ingress admission**, not session persistence,
model delivery or effect success. Retries use the same input ID **and payload**;
conflicts are rejected. The optional workerhost admission callback publishes
confirmed new/recovered pending inputs without changing journal interfaces. It
may race after session publication; persisted IDs suppress phantom pending.
A full pending mirror still permits known-ID retries for journal deduplication
or conflict checking. New identities are rejected rather than silently hidden.
Transport loss before the correlated reply means **uncertain**: reconnect and
retry the same stable ID, never invent a replacement ID just to retry. Host
errors and uncertainty are reported on stderr; successful view/reply events are
JSONL on stdout. Ctrl-C exits the client, not the agent.

## Deliberate limits

- 1 MiB per frame **including newline**; 256 KiB submitted JSON payload;
  128-byte submission/correlation IDs.
- Per-client queue: 256 frames / 8 MiB, plus at most the writer's in-flight frame.
  Pending publication mirror: 64 inputs / 8 MiB. Recovered overflow or uncertain
  ingress marks pending state unavailable, rather than pretending it is complete.
- Snapshot: 256 MiB / 30 seconds; handshake and individual writes: 5 seconds.
- Queue overflow, oversize history or partial transport disconnects the client;
  reconnect with a **fresh complete snapshot**. Nonzero `After` is rejected.
  No retained live-cursor replay, chunking or explicit gap-recovery RPC yet.
- No operation/activity view, attachments, control/reload RPC, client takeover,
  multi-session registry, autostart, heartbeat, runtime update/adoption or new
  operation replay. Operation/activity availability fields are explicitly false.
- These bounds cover this bridge, **not whole-host memory**: the existing host
  volatile mailbox and retained seen-ID index/history are not proven bounded.
- The existing runtime refuses resume with any restored nonterminal operation.
  Durable history is not permission to rerun external effects blindly.

[Verification evidence](../verification/durable-host-m11-20261027.md) records the
Linux process/race checks and required repository validation. Darwin credentials
have a platform implementation, but were not runtime-tested in this environment.
