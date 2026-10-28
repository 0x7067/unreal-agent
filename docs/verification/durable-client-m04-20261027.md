# M0.4 late-join/client-contract verification — 2026-10-27

## Result and scope

**Completed M0.4**: executable current-history reconstruction driver,
[view inventory and proposed local attach v1 contract](../plans/durable-client-attach-contract.md).
[Test source](../../cmd/internal/workerhost/late_join_test.go).
This is a characterization slice: no production socket service, independent host
command, public protocol, UI change or automatic crash replay was introduced.

### Observed scenarios

- Real host, subprocess worker, coordinator, durable ingress/store and provider
  adapter; only loopback provider is fake. Subscribe before reading a prefix in
  two-item pages. Assert **H=3**, settings/external-input/turn kinds and input ID.
- An **after-read/before-return** snapshot hook releases the blocked provider and
  waits for its committed answer callback. The answer is at **H+1**, before a
  caller could perform an unsafe subscribe-after-snapshot-return. The bounded
  backlog plus snapshot equals the complete fresh canonical typed history.
- Injected exact duplicate has no effect. Commit another input/answer while no
  test subscriber exists; a later fresh snapshot recovers it without model replay.
- Stall a one-frame test subscriber: overflow forces explicit resync, frees its
  slot/backlog and does not block callbacks, ingress or the third answer. A new
  subscriber reconstructs exact canonical history. Each stable input ID appears
  once; three inputs/answers produce **three provider calls**. The same worker
  object/generation remains responsive; explicit Close/fixture cleanup joins it.
- Unit driver rejects conflicting duplicate, forward gap, wrong session, queue
  frame/byte overflow and oversize frame; view item/byte bounds preserve accepted
  prefix. Fixture limits are not claimed as production or whole-process bounds.
- Real leased store gap: operation-only checkpoint changes recovery state without
  changing Item history or observer count. Existing ahead-of-head Items cursor
  returns empty success; test-only validation explicitly rejects it.

Only the **test subscription** detaches/overflows. This is not a real TUI/socket
client lifecycle proof, an atomic full-agent snapshot, OS peer authentication,
or an operation/ingress/status watch implementation. The contract keeps these
capabilities and required bounds explicitly proposed for M1.

## Review and upstream comparison

Read-only inventory/security review and final code/contract review completed.
Review identified two evidence issues: H=3 was initially only nonempty, and an
answer released after snapshot return did not distinguish read-then-register.
Both were corrected before the final-source focused/full validations: exact H=3
prefix assertions and the after-read/before-return commit hook above. No remaining
blocking findings were reported; parent inspected final changes and accepted the
history-only evidence without claiming full-agent atomicity.

Concrete proposed decisions: same-UID Unix peer credentials in a private runtime
namespace; one active client and bounded handshake; no attach-time bootstrap/auth
loading; independent host and client lifetime; distinct durable/live/worker/client
identities; serialized published view **R,H**, not unrelated filesystem head;
explicit unknown/unavailable components; queue/frame/chunk/disk/time limits and
client-only overflow; stable input-ID admission reconciliation. These are design
rules, not passing production transport tests.

Compared against locally available `upstream/main`
`a5f3fd13032737142916523ae4344c392292f9d5` (not freshly fetched).
Items/observer/sequence/paging contracts match upstream; optional structural
control initializer and worker host/protocol/runtime are preexisting local
additions. M0.4 changes **tests/docs only**. Future socket/view/API extensions and
acknowledgment ordering are surfaced in the contract for comparison before M1
implementation, not silently added to shared interfaces here.

## Commands and current evidence

```sh
go test -race ./cmd/internal/workerhost \
  -run '^(TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog|TestLateJoinDriverRejectsGapsConflictsAndBounds|TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint)$' \
  -v -count=3 -timeout=70s
make test
make check
make build
```

- Focused: **three tests passed three consecutive race-enabled runs**; four unit
  subcases per run also passed. Final focused package duration **8.055s**.
- Full `make test`: **exit 0**, both Go modules, **41 passing / 3 no-test packages**.
  All 44 packages matched `go list ./...` from both modules; no FAIL/panic/make-error
  markers. Workerhost **118.616s**, TUI **13.396s**. Unchanged packages reused valid
  race-test cache, including the unchanged exhaustive TaskGraph checker previously
  validated in M0.1/M0.2. No expensive unchanged checker was gratuitously rerun.
- `make check`: **exit 0**, vet both modules and gofmt check.
- `make build`: **exit 0**, runner and TUI built with `-trimpath`.
- **Both binaries atomically reinstalled individually** in `~/.local/bin` with
  same-directory temporary file + rename; `cmp`/SHA-256 matched. Installed runner
  `-h` and TUI `--version` exited 0. No frontend restart: no production/UI source
  changed and installed production binary hashes are unchanged.

Source HEAD: `d24507b7789652736a3f9e3d89e37abcc22d046c`.
Tracked source diff and complete tracked/untracked relevant-source manifest
(`cmd`, `harness`, `internal`, module files, Makefile, formal specs) were unchanged
across full validation. Manifest SHA-256: `b974d3c142e16ead59c56fb24e74e52e59390be01e32749071d595f8119d286b`.
Final focused and full-suite inputs share this exact test hash:

```text
80048cf17c3ac92bdbfe0437259b51cf2b4c058348bc80f04ccef70bd298515e  cmd/internal/workerhost/late_join_test.go

fc158a84db86e5ee902ab08bd959ae2b1dd1bcab07e14ee13f2b313d510fc327  bin/unreal-agent-runner
27906ed29bdd600063068e3769fa28dc1435c1072526e17af00782cef60c2dfd  bin/unreal-agent-tui
fc158a84db86e5ee902ab08bd959ae2b1dd1bcab07e14ee13f2b313d510fc327  /home/denguinho/.local/bin/unreal-agent-runner
27906ed29bdd600063068e3769fa28dc1435c1072526e17af00782cef60c2dfd  /home/denguinho/.local/bin/unreal-agent-tui
```

## Supervision and drain

User-authorized supervised validation, task-scoped unit
`unreal-agent-m04-validation-20261027192744.service`: 20-minute runtime budget,
10-second stop bound, control-group cleanup. Unmodified `make test` finished in
about **2min 13s**; wrapper atomically recorded exit **0**. Before releasing unit:

```text
ActiveState=active
SubState=exited
RuntimeMaxUSec=20min
RemainAfterExit=yes
MainPID=0
Result=success
ExecMainStartTimestamp=Tue 2026-10-27 19:27:45 UTC
ExecMainExitTimestamp=Tue 2026-10-27 19:29:58 UTC
ExecMainCode=1
ExecMainStatus=0
ControlGroup=
KillMode=control-group
```

Stopped that exact unit after output/package/source verification:

```text
LoadState=not-found
ActiveState=inactive
SubState=dead
MainPID=0
```

Exact cgroup
`/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-m04-validation-20261027192744.service`
was absent after cleanup. Raw artifacts: `/tmp/unreal-agent-m04-validation.QvOpVh`.
Full make-test log SHA-256: `3270e7094f9e249e629ba8fceca80408ef0a58d9e4c06477e0b2618137d6ca60`.

Scoped inventory helper drained (process wait/EOF/reader/owner exit observed).
Final review handle `23c911b4-ff8a-49c2-a931-228baf59d665`, generation **2**,
returned **DrainConfirmed=true** with all those lifecycle observations. No retained
review helper or validation service remains.

## Full `make test` output

```text
go test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/providers	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerhost	118.616s
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerproto	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/xdgpath	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-runner	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/contextbuilder	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/coordinator	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/executionbudget	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/inbox	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/ingress	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/anthropic	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/fireworks	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/ollama	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openai	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openaicodex	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openrouter	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/internal/retryhint	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/messagesapi	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/responsesapi	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/operation	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/primitives	(cached)
?   	github.com/unreallabsai/unreal-agent/harness/session	[no test files]
ok  	github.com/unreallabsai/unreal-agent/harness/sessionstore	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/settings	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/taskgraph	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/bash	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/taskgraph	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/viewimage	(cached)
ok  	github.com/unreallabsai/unreal-agent/internal/anthropicapi	(cached)
?   	github.com/unreallabsai/unreal-agent/internal/apijson	[no test files]
ok  	github.com/unreallabsai/unreal-agent/internal/jsonoptions	(cached)
ok  	github.com/unreallabsai/unreal-agent/internal/openaiapi	(cached)
go -C cmd/unreal-agent-tui test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui	13.396s
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch	1.023s
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning	(cached)
?   	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/runcontrol	[no test files]
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext	(cached)
```

## Final-source focused race output

```text
=== RUN   TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog
    late_join_test.go:336: snapshot head=3; answer committed before watch consumption; exact history and duplicate convergence
    late_join_test.go:408: no-subscriber commit and overflow recovered from disk; same worker stayed responsive; three inputs/answers/provider calls
--- PASS: TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog (2.19s)
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count
--- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds (0.04s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count (0.01s)
=== RUN   TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint
    late_join_test.go:551: fresh store exposes newer operation while Item history/watch stay unchanged; ahead cursor needs explicit rejection
--- PASS: TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint (0.01s)
=== RUN   TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog
    late_join_test.go:336: snapshot head=3; answer committed before watch consumption; exact history and duplicate convergence
    late_join_test.go:408: no-subscriber commit and overflow recovered from disk; same worker stayed responsive; three inputs/answers/provider calls
--- PASS: TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog (2.20s)
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count
--- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds (0.03s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count (0.01s)
=== RUN   TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint
    late_join_test.go:551: fresh store exposes newer operation while Item history/watch stay unchanged; ahead cursor needs explicit rejection
--- PASS: TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint (0.01s)
=== RUN   TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog
    late_join_test.go:336: snapshot head=3; answer committed before watch consumption; exact history and duplicate convergence
    late_join_test.go:408: no-subscriber commit and overflow recovered from disk; same worker stayed responsive; three inputs/answers/provider calls
--- PASS: TestRealWorkerLateJoinSnapshotWatchAndBoundedBacklog (2.53s)
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame
=== RUN   TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count
--- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds (0.03s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/frame-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/byte-count (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/oversize-frame (0.00s)
    --- PASS: TestLateJoinDriverRejectsGapsConflictsAndBounds/view-item-count (0.01s)
=== RUN   TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint
    late_join_test.go:551: fresh store exposes newer operation while Item history/watch stay unchanged; ahead cursor needs explicit rejection
--- PASS: TestHistoryWatchDoesNotIncludeLatestOperationCheckpoint (0.01s)
PASS
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerhost	8.055s
```

## Check/build output

```text
go vet ./...
go -C cmd/unreal-agent-tui vet ./...
go build -trimpath -o bin/unreal-agent-runner ./cmd/unreal-agent-runner
go -C cmd/unreal-agent-tui build -trimpath -o ../../bin/unreal-agent-tui .
unreal-agent-tui dev (commit unknown, built unknown)
```
