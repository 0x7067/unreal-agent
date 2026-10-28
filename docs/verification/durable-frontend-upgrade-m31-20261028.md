# M3.1 accepted — two-build frontend replacement

Accepted 2026-10-28. **18/27 complete; M3.2–M4.5 remain.**
[Workflow and limits](../plans/durable-frontend-upgrade.md).

No Go/production implementation changed for this slice. Existing attach behavior
is exposed as an explicit install/reattach workflow and proved with a new owned
verification driver. Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` has no
resident/attach/client-state layer; this remains a local workflow divergence, not
a shared translator/context/store-interface change. No fetch/commit/user frontend
or production-session access occurred.

## Two actual builds and a negative real TUI

Private build scratch `/tmp/unreal-agent-m31-build.fhkQ1z`, named 8-minute unit
`unreal-agent-m31-build-20261028T063046Z.service`. A/B use identical accepted M2.7
source with different Go link build IDs. No untested feature difference is claimed.
The actual incompatible TUI uses only an archived private source overlay, attach
protocol constant 1 -> 999, not a mock client or edited production source.

| Binary | Go build ID | SHA-256 |
|---|---|---|
| A | `unreal-agent-m31-compatible-a` | `7898b44d7aa66638ccb09a4c04b3fc63c62f0ec0650a6e09bd4f5b54f7549558` |
| B | `unreal-agent-m31-compatible-b` | `2028ed379ab5a2709393d98def401a2072dc17e2b2473f114bee8ba3de35d1e3` |
| incompatible | `unreal-agent-m31-incompatible` | `3c9a973075fd185b210099b38e4074d31e0cfabfb81ad717e9fdcf06f87cd7dc` |

## Real private tmux proof

Scratch `/tmp/unreal-agent-m31-ui.YEZc18`, private run `/tmp/u31.ROiQgS`, unit
`unreal-agent-m31-upgrade-tmux-20261028T063344Z.service`, 4-minute budget.

```sh
python3 -I .claude/skills/verify-unreal-agent-tui/scripts/durable_upgrade.py \
  REPO PRIVATE_RUN FRONTEND_A FRONTEND_B INCOMPATIBLE_REAL_TUI
```

- PASS: A -> B while the model is blocked; same worker/host, restored draft/view.
- PASS: B -> B during actual Bash; same worker; sentinel exactly `x`, no late tool.
- Exact owned worker PID **937409**, bound pidfd and held lease device/inode
  **46/18287073**, observed at four boundaries; exclusive lock probes fail.
- Actual incompatible TUI exits **1**, actual wrapper child-wait receipt, exact
  TTY restoration, no forced cleanup. Its visible diagnostic is `EOF`. A's draft,
  connection/host generation and single blocked provider request remain intact.
- `upgrade-result.json.passed=true`, incompatible join true, exact worker exit
  pidfd observed **after** host join; lifecycle cleanup **13/13 true**. All frontend,
  host, relay, provider and private tmux owners joined under the existing driver.
- Executable and both driver hashes before/after match. Expanded trusted fixture
  source is archived; base driver hash
  `f6466bb29b4132e20df548844fc4d421df6acaddfbf69c37632aae4d7139dbec`.
- Both build and proof units observed success0/MainPID0, exact recorded cgroup absent,
  then exact stop and recorded path still absent.

An initial observer candidate incorrectly read only the main OS thread's children.
It failed without orphaning work; its cleanup joins were observed and exact unit
stopped/drained. The corrected proof enumerates only threads of the known owned
host and asserts the one child/lease, rather than inferring worker identity from a
host PID or weakening the assertion.

## Reused required validation and installed pair

Relevant Go/module/Makefile/formal sources are unchanged. The new current tracked+
untracked source manifest equals M2.7 before/after/current byte-for-byte. Therefore
current accepted `make check`, `make build`, and race `make test` evidence is reused,
not falsely reported as rerun: [M2.7 full 52-package exact union](durable-graph-m27-20261028.md),
49 pass + 3 no-test, accepted source-manifest hash
`d545f14222ae080e931fcc51f16c9ee3f66357c01daa65865a15b659e23e7400`.
The installed runner/TUI pair remains the M2.7 atomic installation, independently
`cmp`-rechecked during M3.1. The private build-ID variants are proof artifacts, not
claimed as installed releases. Driver syntax and actual owned UI proof both pass.

## Archive and remaining scope

[Evidence archive](durable-frontend-upgrade-m31-20261028-evidence.tar.gz), SHA-256
`14c5644be763bc8fc795e639c06f9c5935a64bc9a31b2d1cf5bb4edfaf82dcfc`.
Build commands/IDs/hashes and negative overlay are included; large executable files
and sockets omitted. This is compatible frontend replacement, not runtime/host
replacement or universal compatibility. Continue M3.2 and all remaining slices.
