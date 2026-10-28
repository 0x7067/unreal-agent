# M1.5 — stalled and adversarial public clients

Status: complete for the bounded authenticated local transport. This is not a
whole-host memory bound, a remote transport proof, or lifecycle-control support.

## Implementation and oracle

- [`adversarial_test.go`](../../cmd/internal/resident/adversarial_test.go) runs the
  real resident host and subprocess worker runtime with the production provider
  adapter. Only the loopback SSE provider is fake. A Bash operation appends one
  `x` to a sentinel, emits partial output, waits for an owned release file, then
  emits final output.
- A raw authenticated client stops reading after its seed input is durably
  accepted. Large external inputs fill its publication path. The stalled peer is
  fenced; a healthy client attaches to the **same host/incarnation/generation**,
  retries the seed identity, and durably submits a **new** input while Bash is
  still blocked. Busy retries are bounded, not an assumption of immediate
  availability. After release, typed canonical history contains every
  `OnAdmission`-confirmed input exactly once, one tool call/result, the answer,
  and precisely one sentinel effect. Later model-call batching can vary.
- Raw protocol-version, session/token, cursor, arbitrary-control, shutdown,
  oversized-payload/frame, and partial-frame attacks must actually disconnect;
  a read timeout is not counted as rejection. They cannot mutate ingress.
- [`peer_policy.go`](../../cmd/internal/attachproto/peer_policy.go) factors the
  existing kernel-UID comparison out of the Linux/Darwin socket credential
  checks. No interface/wire version changed. Tests accept the current eUID and
  reject foreign UID values and a nil peer check. Wire claims never choose UID.

### Independently measured publication bounds

The no-writer publisher test independently sums encoded queued frames. It fills
one limit at a time, then proves the next enqueue fences the client and clears
both queue and accounting:

| Limit | Before overflow | Next frame |
| --- | --- | --- |
| 256 frames | 256 frames / 39,424 bytes | 154 bytes |
| 8 MiB encoded bytes | 32 frames / 8,198,784 bytes | 256,212 bytes |

The real writer includes its in-flight frame in accounting. These tests measure
**publication queue** limits, not Go heap/RSS or all host buffers. Snapshot,
frame, payload, pending-input, handshake and writer limits remain separate.

## Validation, installed binaries, and actual frontend replacement

[Evidence directory](durable-adversarial-m15-20261027/):

- `go test -race ./cmd/internal/attachproto -count=3 -timeout=20s`: passed.
- `go test -race ./cmd/internal/resident -run '^TestAdversarial' -v -count=3
  -timeout=80s`: passed (39.398s).
- Fresh `make test`, `make check`, `make build`: all passed. The test log covers
  exactly the union of both module package lists, **48 packages**. Source
  manifests before/after/finish match. HEAD is still
  `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commit was requested or made.
- Both binaries were atomically replaced in `~/.local/bin` and byte-compared to
  `bin/`; runner help and TUI attach help passed. SHA256:
  - runner: `654ec03ceae527b4a12fca7ccb1e83bd044f9ce4186388e6273a04278f27c53b`
  - TUI: `6a9bdd9431c8efbb100b4a05e0913807bbb23fbbadbcdbcd8d49b83ae89fcd41`
- The existing M1.3/M1.4 driver was rerun against these rebuilt binaries using
  its own private tmux server. Both pre-admission and lost-accepted-reply cuts
  passed: no automatic reconnect submit, stable retry identity, one canonical
  input/effect, two legitimate provider calls, and actual replacement frontends
  restored draft, expanded selected tool and manual scroll. Canonical snapshots,
  screens, result identities, and all-true cleanup records are archived under
  `tmux/`. No user frontend was restarted.
- Full validation unit `unreal-agent-m15-validation-20261027T224032Z.service`
  and proof unit `unreal-agent-m15-tmux-20261027T224717Z.service` exited zero,
  were stopped, and their exact recorded cgroup paths were confirmed absent.

## Limits and next work

An actual **foreign-UID process** could not be launched under this environment's
unprivileged eUID 1000. The deterministic UID-policy tests are not misrepresented
as that experiment. Actual same-UID kernel transport plus forged session/token
attacks ran. Supported peer policy remains same kernel eUID, not wire identity.

The Bash fixture's effect and canonical terminal result are independently
checked. A successful legacy `Host.Close`, socket EOF, or terminal unsupported
status is **not** promoted to general descendant-cleanup proof. Public lifecycle
controls are still rejected. M1.6 must add distinct target-bound foreground abort
and deliberate shutdown with explicit ownership/drain outcomes; ordinary detach
continues to leave host execution alive.
