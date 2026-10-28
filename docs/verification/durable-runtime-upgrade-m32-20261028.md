# M3.2 — idle runtime replacement behind attached clients

**Accepted: 19/27 slices; M3.3–M4.5 remain.** This is not roadmap completion.
HEAD remains `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commit was made.
The large inherited dirty/untracked tree was preserved. No inbox production edits
or reducer edits: `harness/taskgraph/graph.go` remains blob
`2315afc35788160fc81bf84657f2600bd61c9e86`.

## Implementation and tests

- `cmd/internal/attachproto/runtime.go`: strict versioned opt-in request/status
  variant; `runtime_test.go` covers bounds, unsupported versions, nested unknown
  fields, binding/cursor rules and omitted legacy fields.
- `cmd/internal/resident/runtime.go`, `server.go`, `publisher.go`: host-selected
  capability, current connection/generation admission, one bounded pending
  control, matching cancel and client control join. No executable from clients.
- `cmd/internal/attachclient/runtime.go`, `runtime_test.go`: sole-reader correlation,
  progress/terminal events, concurrent input serialization, capability fence,
  typed refusal, close/cancel/uncertainty and monotonic lifecycle scope.
- `cmd/unreal-agent-tui/attach_runtime.go`, `attach_runtime_test.go`, `attach.go`,
  `attach_retry.go`, `submissions.go`: real `/builds`/`/reload` bridge, exact
  advertised/connected command guard, view preservation and lifecycle fences.
- `cmd/internal/resident/runtime_test.go`: real idle worker replacement, stale
  retry refusal, active-model refusal without cancellation, opt-in default.
- Existing current `workerhost` handoff/adversarial tests remain the core oracle
  for admitted-during-handoff inputs, exact-once delivery, canceled late retirement,
  delayed exit, stale status, actual waits and lease fencing. The full suite below
  includes these unchanged core inputs; the new bridge never bypasses that Host.

## Current full validation and installation

`/tmp/unreal-agent-m32-validation.NEoBlK`, unit
`unreal-agent-m32-validation-20261028T072452Z.service`, `GOFLAGS=-p=2`,
RuntimeMax=20min, TimeoutStop=10s, KillMode=control-group. `make check`,
`make build`, `make test` PASS (race in both modules). Exact fresh package union:
**52 = 49 passing + 3 no-test**, no duplicates/FAIL/race/panic. Tracked+untracked
relevant source manifests before/after/current are byte equal, SHA-256
`3d35935442cad6f1228ab4e9cb1ca6cee6a6e55200dddef8694965d007d92fc6`.
Observed success/exit0/MainPID0/exact recorded cgroup absent, then stopped the exact
unit and observed that same cgroup absent. Full evidence includes package lists,
acceptance JSON, phase logs, manifests, upstream audit and drain receipts.

Both binaries were installed with same-directory temporary files and atomic
rename to `~/.local/bin`, then **individually cmp** checked:

- runner `7755ae80c893684f0286fdb6697ea137652668839a10b1d362fc3f5f6bc464fb`
- TUI `8f2552a92cd5d2e79f2456015e22c773f3a7583fc85d6b36d98956674c35b80c`

Only owned private frontends were replaced. The user frontend and production
session/worker `33b63c18-e05a-487f-a0a9-2f5411dac58a` were not touched, and no
protected tool-output files were read.

## Two-build real private tmux proof

Driver `.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_upgrade.py`
uses exact checked transformations of the full owned lifecycle fixture. It
records expanded source/provenance. `/tmp/unreal-agent-m32-build.5svHLC` contains
A/B same runner production source with **distinct Go link build IDs**, not a claim
of an untested feature release. The negative runner changes only worker-probe
protocol7→999 through a private Go overlay. Large binaries are omitted archive.
Build unit `unreal-agent-m32-build-20261028T070705Z.service` completed success0,
MainPID0, exact cgroup absent, explicitly stopped/drained.

Accepted `/tmp/unreal-agent-m32-ui.E2OJd3`, private run `/tmp/u32.5XYkbO`, unit
`unreal-agent-m32-runtime-tmux-20261028T073132Z.service`: proof exit0; all 13/13
cleanup checks true. Executable/driver hashes before/after equal; observed unit
success0/MainPID0/exact cgroup absent, then exact stop/absent.

1. Actual protocol-999 candidate rejected with diagnostic, generation1 and A PID
   **1767128** unchanged/alive, same exclusively held lease, zero provider calls.
2. `/builds` visibly distinguishes executing A from installed B by staged digest.
3. `/reload` completes at generation2 on PID **1767275** only after old pidfd exit;
   the replacement holds the identical session lease inode, verified through
   bounded owned `/proc` descriptors and a failed exclusive flock probe. No
   PID-only assertion is used as permission to replace.
4. Upgrade during an actual blocked model reports `blocked`, same worker/lease,
   one still-blocked provider request, no unrelated cancellation.
5. Full model/Bash frontend A→B→C detach/replacement, retained draft/view,
   canonical abort and shutdown, one Bash sentinel, no late tool, exactly two
   legitimate provider calls, real host/offline child waits, exact TTY restoration
   and all owned thread/socket/process joins complete. Replacement worker pidfd
   exit is observed after actual host join.

Focused real resident race tests passed 9.555s in
`unreal-agent-m32-focused-20261028T070345Z.service`, stopped/exact cgroup absent.

## Rejected runs and synchronization repair

Full `/tmp/unreal-agent-m32-validation.KMxXnb` was rejected: the existing M2.4
manual-only exhaustion test read the exhaustion commit before the separate durable
`CompactionDisabled` commit. The test now waits for both, preserving disable=1,
provider calls=3 and tool authority assertions. Ten isolated race repeats passed
77.841s (`ua-m32-compaction-sync-1793171668.service`); helper observed successful
exit and exact cgroup absence, and parent re-observed that exact path absent.
No compaction production behavior changed. Its logs are in final validation.

Private `/tmp/unreal-agent-m32-ui.AdZPxJ` correctly exposed the legacy attach
command guard; fixed with exact capability/connection tests. Second private
`/tmp/unreal-agent-m32-ui.hYjlZJ` passed actual A→B retirement but the driver
appended model text to intentionally retained `/reload` composer text. Driver now
uses owned Ctrl+A/Ctrl+K before the next command, not a production draft-clearing
change. Both rejected fixtures actually joined/cleaned their owned resources and
had exact unit cgroup absence; their scenario/effect checks were not accepted.
All rejected units stopped/drained. No hidden protected output was used.

## Limits

See [policy](../plans/durable-runtime-upgrade.md). Payload version1/attach1,
worker7/session2; graph durability flags unchanged. Active work is blocked, not
checkpointed or forcibly killed. New UI lifecycle controls require reconnect
following runtime generation advance; retained old controls remain stale.
Cancel acknowledgment is admission only; transport loss closes the local client
connection and produces uncertainty rather than an automatic retry. No host
transfer, arbitrary executable selection, migration, active shell replay,
power-loss durability, or general extension durability is claimed. Local upstream
`a5f3fd13032737142916523ae4344c392292f9d5` lacks this resident/private layer;
shared pure translator/context and commit-before-dispatch semantics are preserved.

Evidence archive SHA-256: `59f942d4b59be5a68e8f9cac229337a1c99ec22eee03cdc19af7a09e6c5caf96`.
