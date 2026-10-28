# M2.7 accepted — classified command graph recovery

Accepted 2026-10-28 for the [opt-in contract](../plans/durable-graph-recovery.md).
**17/27 slices complete. Every M3/M4 slice remains; this is not roadmap completion.**
Private worker protocol 7, session format 2. No user frontend or production
session/worker was touched; no commits or fetches were made.

## Current-source acceptance and installation

Scratch `/tmp/unreal-agent-m27-validation.KM5J6c`, bounded user unit
`unreal-agent-m27-validation-20261028T062155Z.service`, `GOFLAGS=-p=2`:

- `make check`, `make build`, `make test`: PASS; race in both modules.
- Exact package union **52 = 49 passing + 3 no-test**, no duplicates, omissions,
  failures, race warnings or panics. Both module lists rechecked at acceptance.
- Tracked/untracked relevant-source manifests **before = after = current**.
  Manifest SHA-256 `d545f14222ae080e931fcc51f16c9ee3f66357c01daa65865a15b659e23e7400`.
- `acceptance.json`, phase logs, source manifests, package lists and install hashes
  archived. Documentation/skills/evidence are deliberately outside the source manifest.
- Observed success/exit0/MainPID0 and exact recorded cgroup absent, then exact unit
  stopped and recorded cgroup again absent. Full-suite time budget was 20 minutes,
  not an artificial foreground test timeout. Package parallelism was bounded.
- Both binaries installed with same-directory temporary files + atomic renames;
  **individual `cmp`** passed for runner and TUI.

| Binary | Built and installed SHA-256 |
|---|---|
| runner | `da02ec395a76169b314129cce72f10070115a84077b7e3367daa934d81a250ca` |
| TUI | `c4be8a2974302894245e683e7a62c46a81e5acaed0e3f7743e161a4874ab352b` |

## Semantic/process evidence

`harness/taskgraph/recovery_test.go` and `dispatch_commit_test.go` cover strict
command-only plans/checkpoints, execution/parent/child identity, accepted/candidate
retention, stale scopes/generations, prelaunch first execution, unknown dispatch
refusal, no PID-only settlement, exact/duplicate acknowledgments and cancel-before-
acknowledgment. Serialized saves in the dispatch tests are explicitly simulated;
these are not claimed as coordinator/store integration proofs.

`workerruntime/m27_contract_test.go` exercises the actual store decorator and real
runtime/nested manager: failed saves never release a command; successful duplicate
saves launch once; actual shell receipt and nested joins are observed. Other tests
cover trusted parent/config binding, mixed graph/subagent ownership, terminal inert
controls versus unfinished refusal, and exact strict graph2 lifecycle attestation.
Pure translator tests prove command-only v2 wrapping/result projection without
mutating persisted operations or changing legacy translation.

`workerhost.TestM27DurableGraphWorkerRecovery` runs in the current full race suite.
Actual A/B/C shell receipts establish accepted A, completed unaccepted B and a
controlled saved C output-validation boundary. This is **a controlled canonical
fixture, not a live internal crash hook**. A real first worker recovers it; stale
scope and stale generation do not mutate candidates. That worker is actually killed
and joined while its loopback provider request is blocked. The replacement has a
new durable control scope; A/B/C do not rerun, B stays unaccepted with D blocked, and
fresh explicit B/C/D accepts finish the graph. Each command counter is exactly `x`;
there is one canonical graph completion and actual parent/nested joins.

A separate non-race scenario passed in `/tmp/unreal-agent-m27-process.zZWEEm`.
Ten isolated race repetitions passed in `/tmp/unreal-agent-m27-isolated.1Px1pW/graph.log`
after fixing a proof synchronization assumption (fresh provider admission is not
fresh checkpoint persistence). These repetitions predate the optional-store-capability
fix below; the final full race suite validates the current implementation.

Exact exhaustive safety passed before (34.427s) and after (19.872s), the latter in
`/tmp/unreal-agent-m27-safety.3fKjU5`. Reducer blob remains
`2315afc35788160fc81bf84657f2600bd61c9e86`, **unmodified**. The safety skill records
`checkpointCommitted` as an actor event feeding existing modeled transitions;
store/ack/process properties have separate tests, not invented model coverage.

## Fresh owned private tmux frontend replacement

`/tmp/unreal-agent-m27-ui.CjflHH` records before/after unchanged binary+driver hashes.

- Lifecycle `/tmp/u27.24q8KP`, unit
  `unreal-agent-m27-lifecycle-tmux-20261028T061458Z.service`: model and Bash frontend
  replacement preserve worker identity/draft/view; one Bash effect; canonical abort
  and shutdown; host0/offline0; all **13/13 cleanup booleans true**.
- Reconnect `/tmp/u27.Be7VJz`, unit
  `unreal-agent-m27-reconnect-tmux-20261028T061458Z.service`: before/accepted ack cuts
  preserve one logical input/echo and two legitimate provider calls; event-gap/view
  reconstruction; no fixture errors; each cut **4/4 cleanup booleans true**.
- Both named units observed success0/MainPID0 and exact cgroups absent, then stopped.
  Wrappers retained actual frontend/host waits, exact pane pidfds, TTY restoration,
  relay/provider joins and owned tmux-server waits. No user tmux server was used.

## Rejected candidates and fixes

Earlier validation candidates are **not acceptance**. First, the new proof sampled
an obsolete checkpoint before the replacement's fresh scope was saved; it now waits
for durable evidence without weakening identity assertions. Second, the store
wrapper accidentally hid optional `ControlOperationInitializer`, which made the
existing child-stop invocation fail before dispatch. Forwarding that capability
preserves its atomic commit boundary; `m27_store_extension_test.go` tests delegation
and failure. Current isolated child-interruption race proof passed **3 repetitions**
(`/tmp/unreal-agent-m27-child-confirm.W8TdG6`, 20.552s) with actual child joins and
fail-closed replacement. The helper additionally waits for a connected binding,
not merely a published capability. No inbox production changes were made.

A subsequent suite found a one-shot private lost-ack relay startup race: an old
frontend reader can close before server-side release. The test relay now retries
only explicit pre-admission `busy` responses on fresh sockets, retaining its frontend
and all admitted-frame/cut assertions. Five isolated lost-ack race repetitions
passed (11.765s, `/tmp/unreal-agent-m27-resident-repeat.wivzDt`); final full validation
also passed. No production attach/admission behavior was relaxed.

Rejected units were stopped/drained. One obsolete repeated child diagnostic was
structurally canceled after identifying the store-capability failure; its exact
unit cgroup was observed absent, but its canceled fixture's original child Cmd.Wait
is **not** claimed. Accepted process proofs retain real waits. The final full,
child-confirm, resident-repeat, UI and safety units all have observed exact drain.

## Compatibility and limits

Compared with local upstream `a5f3fd13032737142916523ae4344c392292f9d5`, no fetch.
Upstream lacks taskgraph/private recovery/host/Hub layers. These are surfaced local
opt-in divergences, not upstream APIs. Pure translators/context builder, coordinator-
owned writes, version fences and root commit-before-dispatch remain. Nested graph
launch now additionally waits for actual successful coordinator store acknowledgment.
No mandatory shared manager/store interface or reducer semantics were changed.

Legacy graph1, live/unknown-dispatched/PGID-only/agent children and unfinished old
controls remain unsupported. Scope rotation does not autoaccept candidates. The
saved reservation is not a launch receipt. Terminal shell state is not universal
independent-process cleanup proof. This slice makes no universal recovery,
exactly-once shell effects, active-host transfer or extension/remote-job claim.

## Archive

[Evidence archive](durable-graph-m27-20261028-evidence.tar.gz), SHA-256
`7dc7ab86b5b8d6d44ed8eb9aaafa8cf6bee44242d98ff9b0148e92f612ab9fec`.
Only regular files/directories/symlinks were archived; sockets omitted.
