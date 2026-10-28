# M1.3 / M1.4 verification — 2026-10-27

**Completed:** stable-ID explicit retry, monotonic acknowledgment reconciliation,
cursor delta / bounded snapshot fallback, and private restart-safe local view.
The full durable-session roadmap is **not** complete. M1.5/M1.6 remain next.

## Accepted implementation

- [Behavior/protocol/storage contract](../plans/durable-tui-reconnect.md)
- Transport: `cmd/internal/attachproto/{protocol,view}.go`,
  `cmd/internal/attachclient/client.go`, `cmd/internal/resident/{publisher,server}.go`.
- TUI: `attach.go`, `attach_retry.go`, `attach_localstate.go`,
  `attach_localview.go`, `submissions.go`, `ui.go` in `cmd/unreal-agent-tui`.
- Fault tests: `resident/submission_fault_test.go` runs actual host + subprocess
  runtime + production provider adapter. Only the loopback provider is faked;
  a real Unix proxy injects transport faults. Three causal cuts are crossed with
  fresh, valid, expired and invalid cursors: 12 cases. Same ID/payload gives one
  canonical input/turn/model response and one provider request; changed payload
  conflicts. The provider remains gated until retry/conflict checks complete.
- Cursor tests: bounded 256 checkpoint ring, exact delta base/range, explicit
  fallback, strict old-host compatibility, exact duplicate suppression and
  conflicting/evicted duplicate refusal.
- TUI tests: stable identity before network dispatch, failed save sends nothing,
  explicit retry/one echo, stale source fencing, capacity not acceptance,
  persistence wins late error, no automatic local-state dispatch, conflicting
  local payload cannot replace a durable echo, private DTO/file limits and lock
  fencing, selection/details/scroll/draft roundtrip.

## Real tmux proof

[Driver recipe](../../.claude/skills/verify-unreal-agent-tui/features/durable-reconnect.md)
and [retained artifacts](durable-reconnect-m13-m14-20261027/).
Final scratch run: `/tmp/u14p.pxSsQo`. It used its own private tmux server, loopback
provider, socket proxy, explicit hosts and four actual frontends. No user frontend
or production worker was signaled/restarted.

| Case | Host PID | Frontend PIDs | External inputs | Legitimate provider calls |
|---|---:|---|---:|---:|
| Drop before admission | 3666696 | 3666731 → 3667051 | 1 | 2 (tool + answer) |
| Lose accepted reply | 3667346 | 3667380 → 3667710 | 1 | 2 (tool + answer) |

The first case initially makes zero provider calls. F5 does not resubmit;
`/retry` sends the exact original ID and payload. The second case suppresses the
acknowledgment and canonical updates, then holds a reconnect delta snapshot at
**H=3, base H=1**, while tool and answer commit. Snapshot plus queued live updates
converge without duplicates. `/retry` after canonical persistence sends no RPC.
Both cases show one logical echo, preserve expanded tool details and unsent draft,
and compare the same visible manual-scroll paragraph markers across replacement.
The independent final snapshot excludes the draft/preferences. Every proxy,
provider, host and frontend is joined/absent in both `cleanup.json` files.

Several earlier driver attempts failed assertions (Escape sequencing, intentional
watcher signal exit classification, and clearing a retained `/retry` command).
They are not passing evidence. Temporary key tracing was removed and no diagnostic
binary was installed. Final proof passes uninstrumented source. Cancellation of
the final read-only CLI watcher is not host shutdown/drain proof.

## Required validation and installation

Commands accepted on final current Go inputs:
```
make test
make check
make build
go test -race ./cmd/internal/attachclient ./cmd/internal/attachproto -count=3 -timeout=40s
go test -race ./cmd/internal/resident -run '^TestSubmissionFaultStableIdentity$' -count=1 -timeout=40s
go -C cmd/unreal-agent-tui test -race . -run '^TestAttach' -v -count=3 -timeout=85s
```

All passed. `make test` result set exactly matches **43 root + 5 TUI packages**.
Before/after/finish source manifests match. The final validation user service
`unreal-agent-m14-final-20261027T222449Z.service` completed with status 0, was stopped,
and its exact observed cgroup is absent. Final proof service
`unreal-agent-m14-tmux-20261027T222602Z.service` likewise completed status 0,
was stopped, and its exact cgroup is absent. Both drain logs are archived.

Both binaries were atomically replaced in `~/.local/bin` and byte-compared:
- runner `9fb3fa0a347c526c268b4d2dc0c5a4bd4c652a1c71609cf1516211bacc45d900`
- TUI `490a5fa897340c43d8e05aff59e19161e36c2dbadab98200a0fb7bfc53501a1c`

Installed runner help, TUI attach help and TUI version smoke checks passed. HEAD
remains `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commit was created.

## Limits

One active same-UID local client; no remote listener, automatic resend, public
abort/shutdown controls, live operation checkpoints/activity or new crash replay.
A new process always refreshes canonical history; only an in-process applied view
can be a delta base. View autosave can lose its last unsaved second on sudden
frontend death; stable retry identity is synchronously saved before dispatch.
Tests establish the stated transport/process/write boundaries, not full arbitrary
power-loss durability, universal recovery or exactly-once external effects.
