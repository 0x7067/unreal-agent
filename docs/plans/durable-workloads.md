# Durable trusted workloads (M4.1–M4.5)

North star: **the durable session is the agent; the process running it is replaceable.**

This is an additive, explicitly configured implementation. The shared `tool.Registry`
and `tool.Context` APIs and native tool selection are unchanged. Against local
upstream `a5f3fd13032737142916523ae4344c392292f9d5` (no fetch), these are local
extensions: upstream has no resident/recovery/Hub/compaction/taskgraph extensions,
trusted extension registry, independent executor, durable timers, or TLS executor.
The graph reducer is not changed. No inbox production changes are part of M4.

## Commit and authority boundaries

Translators synchronously produce inert operation specifications. The coordinator
commits the complete tool status and initial operation before manager dispatch.
Registry selection and turn/time metadata reads are pure; configuration filesystem
I/O, clock polling, network requests and execution occur outside translators and
context construction. The optional coordinator lifecycle policy is also pure. It
receives the exact abort target turn; unknown kinds/versions and missing ownership
or cleanup proof remain refusals. Handler joins are not external-job exit proofs.

Worker protocol remains 7 and session-store format remains 2. New operation plans
have their own strict version1. No migration, automatic rollback, Go plugin,
workspace code discovery or automatic workspace build is provided.

## TrustedEcho: fixed schema, pinned implementation

`UNREAL_AGENT_EXTENSION_CONFIG` is an explicitly trusted absolute private0600 file:

```json
{"Version":1,"Known":[{"Name":"Echo","Generation":1,"SchemaVersion":1,"ImplementationVersion":1,"Implementation":"A"},{"Name":"Echo","Generation":2,"SchemaVersion":1,"ImplementationVersion":1,"Implementation":"B"}],"Selected":2}
```

The only implementations are compiled A/B; the model receives fixed `TrustedEcho`
arguments `{ "input": "text" }`, not an implementation selector. Each plan records
its complete immutable descriptor, input and exact foreground turn. A running A
holds its generation lease when the operator installs/selects B. Install does not
select; incompatible installs, selected/in-use removal and generation reuse fail.
The watcher validates the entire candidate before mutation and joins on shutdown.
Selection need not be reconstructed for replay: it was not model-visible, and every
recorded call pins its meaning. Existing terminal output is validated without old
code lookup. Pending A requires the exact supported A descriptor, never selected B.
The initial schema1 fixture without an owner can recover via canonical tool-call
ownership but cannot obtain optional foreground cancellation authority.

Bounds: 16 lifetime generation identities, 32 admitted calls per handler lifetime,
4096 UTF-8 input bytes; fixed output envelope. Restart does not turn retained terminal
history into new execution. Unsupported envelope/plan/schema/implementation versions
fail before provider access and do not rewrite the canonical file.

## RunTrustedJob: independent execution, not a remote local shell

`UNREAL_AGENT_JOB_CONFIG` is an explicit private0600 config. Unix-only example:

```json
{"Version":1,"EnvironmentID":"trusted-tests","Socket":"/ABS/PRIVATE/executor/executor.sock","Jobs":["tests"]}
```

The separately supervised runner subcommand `job-executor --config ABS_PRIVATE0600`
loads trusted named job definitions, not model commands:

```json
{"Version":1,"Directory":"/ABS/PRIVATE/executor","EnvironmentID":"trusted-tests","Jobs":{"tests":{"Path":"/ABS/TRUSTED/test-command","Args":[],"Dir":"/ABS/TRUSTED/project","Env":["PATH=/usr/bin:/bin"]}}}
```

Supply a scoped 24–1024 byte `UNREAL_AGENT_EXECUTOR_TOKEN` only to the explicitly
trusted host/executor startup environment; never to argv, plans or checkpoints.
Workers consume it into memory and remove it from ordinary child environment
inheritance. Subagents do not automatically inherit executor config/credentials.
Executor jobs receive only the administrator-configured environment, not process
credentials. This is **not a sandbox against arbitrary same-UID code or /proc
inspection**; such code and the startup environment remain trusted.

The model selects one configured job name. Committed operation identity deterministically
pins environment, session-derived controller, job, attempt and fingerprint. Reservation
is synced before launch. A lost submit acknowledgment can retry only that identity;
reattachment uses inspect/output offsets rather than launching again. Durable output
reference is the exact executor handle plus retained-byte offset, bounded to64KiB.
Stale handle/controller/attempt/environment cannot cancel a different execution.
Cancel acknowledges actual process/output join, not request admission. Completion
racing cancellation is diagnostic, not a fabricated cancellation success.

Close joins local handlers/requests without canceling independently owned jobs.
Unavailable transport retries at most three times by default, then parks an explicit
nonterminal unknown/waiting checkpoint; restart does not reset the exhausted budget.
There is no local fallback. Service restart with an unjoined running reservation
reports unknown and does not blindly rerun. Keeping the independent executor alive
is the demonstrated recovery boundary. Jobs are bounded32; output is bounded64KiB.

## Explicit remote environment over TLS1.3

Unix-only is the default. Network mode requires explicit TCP address, server
certificate identity, private CA file and controller allowlist. Worker config:

```json
{"Version":1,"EnvironmentID":"trusted-tests","TLSAddress":"EXECUTOR_HOST:PORT","TLSServerName":"executor.example","CAFile":"/ABS/PRIVATE/ca.pem","Jobs":["tests"]}
```

Socket and TLS configuration cannot be mixed. Executor config adds `TLSAddress`,
`CertFile`, `KeyFile`, and `ControllerIDs`. Controller is `session-` plus the first
32 hexadecimal SHA256 characters of the session ID. Network exposure refuses an
empty controller allowlist. Certificate/key/CA/config files are absolute, bounded,
owned private0600 regular files with link refusal and opened-file identity checking.

The explicitly selected address is the only route. Verified TLS1.3 and server name,
explicit roots, no proxy/redirect/cleartext fallback, bounded requests/handshakes and
32 admitted connections protect the transport boundary. Authorization is a scoped
bearer header; HTTPS encryption is established by the dedicated TLS dialer even
though the internal fixed HTTP protocol URL is not a routing authority. Listener
shutdown joins Serve, connections and handlers before service shutdown. Certificate
rotation/revocation, multi-tenant isolation and cloud credential provisioning are
operator responsibilities; this is not a generic remote-execution marketplace.

## DurableReminder: canonical logical notification

`UNREAL_AGENT_DURABLE_TIMERS=1` explicitly adds the fixed tool:

```json
{"due_at":"2026-11-01T12:00:00Z","message":"review results","background":true}
```

The plan pins parent session, absolute UTC deadline, explicit ownership and message.
Foreground plans pin the current turn; an abort targets only that exact owner.
Background timers survive ordinary foreground abort. Deadline policy is at most30
future days; overdue work is allowed. Stable timer/notification IDs derive from
committed operation identity. Limits are128 lifetime tasks per handler and16 overdue
transitions/notification completions per poll, ordered by due time then stable ID.

A due transition first emits `NotificationPending`. Only the actual successful
canonical SaveOperation callback for the exact checkpoint authorizes completion.
The terminal tool result `{stableNotificationID,message}` is the logical notification.
Its checkpoint deliberately remains `NotificationPending`: it does not claim durable
inbox acceptance or external delivery. Restart after terminal commit is inert;
fresh client snapshots reconcile one stored notification. Explicit cancellation has
its own terminal canceled checkpoint. Fake-clock tests cover backward movement,
deadline/restart windows and bounded overdue batching without scheduling sleeps.

## Evidence and acceptance

See the M4 verification report for exact current full-suite source/package manifests,
real OS worker/executor tests, owned tmux/frontend/TTY replacement, installed hashes,
unit/cgroup drain receipts, rejected runs and tested limits. Component tests alone
are not slice acceptance. No process-kill proof is a power-loss claim, and no logical
notification promises exactly-once third-party sends.
