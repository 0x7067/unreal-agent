# Follow-up simplification pass

Baseline/reload work was committed and pushed first as `2243def`. This subsequent
pass implements two bounded opportunities from the
[alignment plan](../plans/upstream-alignment.md), plus deletion of a CLI forwarding
wrapper. Larger policy changes are not bundled into mechanical cleanup.

## Changes and preserved boundaries

1. **Items-only lifecycle reads.** `sessionstore.ReadLifecycle` accepts only the
   `Items` capability it uses. The canonical readonly adapter and test fake no
   longer embed a nil writable Store. The Store interface still matches upstream;
   lease requirements, ordered paging/fork reset, canonical decoding, diagnostic
   bounds and fail-closed ownership decisions are unchanged.
2. **One pure activity formatter.** Runner and worker capture their own detached
   snapshots and call `activitysummary.Format` with an explicit clock. The worker
   still owns only its hub/account, not an extra operation manager. Two formatting
   implementations become one: 169 production lines become 164, including both
   capture adapters. No modes, configuration framework or harness API was added.
   Both outputs now use the runner's 4096-byte UTF-8-safe bound (the worker used
   2048 runes), normalized 128-byte labels and explicit omission/truncation markers.
   Cancellation/report ranking, envelope-scoped resources and report-versus-drain
   caveats remain. Control advice is absent when the fleet capability is absent.
   Diagnostic wording changes; structured activity protocols do not.
3. **Delete the CLI View shim.** CLI and existing tests call the already-shared
   `attachproto.View` directly. The nine-line wrapper is gone; transport, frame
   publication, duplicate handling, rejection and uncertainty semantics are intact.

Pure translators/context construction, persist-before-dispatch, failed/incomplete-
call safety, usable-summary validation and explicit graph acceptance remain
unchanged. Neither a summary nor a canonical-history read grants cleanup proof.

## Review decisions: no extra abstraction for its own sake

A full CLI migration to `attachclient.Client` was reviewed but rejected for this
pass. CLI stdout streams actual snapshot envelopes, including a valid prefix on
later failure. SDK Dial instead returns a complete projected snapshot and drops
some received envelopes. Reconstructing frames invents metadata; faithfully
preserving streaming output and Submit/Next cancellation/error precedence would
need additional API/state machinery. The existing shared protocol validator is
retained without the forwarding wrapper. SDK authentication/capability checks and
snapshot budgets were not weakened. Upstream `a5f3fd1` has none of these resident
attach packages to reuse.

Durable request/history policy, execution assembly and management-admission
classification remain explicit backlog, not silently changed defaults. Any later
split must preserve recorded replay budgets and unknown-version refusal, partial-
setup cleanup and join order, and bounded non-launching cancellation capacity.

## Verification

Both TaskGraph candidates were explicitly accepted after parent review and fresh
independent tests, not merely on helper completion.

Parent checks passed:

```sh
go test -race ./harness/sessionstore ./harness/sessionstore/localfile \
  ./cmd/internal/recoverygate ./cmd/internal/lifecycleproof \
  ./cmd/internal/lifecycleguard -count=1 -timeout=120s

go test -race ./cmd/internal/activitysummary ./cmd/internal/agentrunner \
  ./cmd/internal/workerruntime -count=1 -timeout=3m

go test -race ./cmd/internal/residentcli ./cmd/internal/attachproto \
  ./cmd/internal/attachclient -count=1 -timeout=3m
```

Coverage includes genuine Items-only implementations, pagination/cursor errors,
fork reset, readonly cancellation/reader failures, deterministic zero/future ages,
UTF-8/control/bidi bounds, omissions, redaction, ranking, input nonmutation and
unavailable observations. Existing runner live heartbeat/replay, worker blocked-
child snapshot and CLI lost-reply/real-process tests were retained and passed.
No assertions were skipped or weakened.

Full two-module `make test`: **PASS**, including a final exact-source rerun
after deleting the CLI shim. Unchanged packages legitimately used Go’s test
cache; the independent affected-package race commands above used `-count=1`.
`make check` and `make build`: **PASS** on the final source.
Atomic binary reinstall and SHA-256 comparison: **PASS**. Both 0755 binaries
were staged in `~/.local/bin` and renamed over their targets on that filesystem;
installed bytes/hashes match the builds below.

Across all three cleanups, production Go code has a net reduction of 11 lines;
the main benefit is one formatter and narrower read authority, not a large
line-count claim. Regression tests account for the larger overall patch.

All three task-scoped helpers have process wait, output EOF, reader and owner-exit
cleanup evidence; none remains running. Both graph tasks were explicitly accepted.

No frontend UI source or wire protocol was changed in this pass. No user's
frontend was restarted or killed.

Final built binary hashes:

| Binary | SHA-256 |
| --- | --- |
| `unreal-agent-runner` | `0e4a4dc27abe93ccf8296b6b309838e84d6aeb77acb7f0d2568df823a68ba7ed` |
| `unreal-agent-tui` | `2249625099a1ed55b3d7d6f74c04dea1e3000d418bcf0f6685f97278fd25523f` |

Final Go source/module/prompt manifest: 587 files, SHA-256
`84804b5581a91a2dbe0e979348da3519dfa5e7a5b74506972c97f05406f9802c`.
The ephemeral local manifest is `/tmp/ua-simplification-source-final.sha256`.
