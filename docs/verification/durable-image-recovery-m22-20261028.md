# M2.2 — phase-specific read-only image recovery

**Accepted 2026-10-28. M2.2 is complete; M2.3–M4 remain unfinished.**
The [reviewed policy and six process cases](../plans/durable-image-recovery.md)
define the narrow image-v1 ready/awaiting, no-result permission. Recovery rereads
current source; changed bytes may change the result. Missing/corrupt source
fails visibly. Completed results remain inert even after source removal. Unknown
versions and result-bearing nonterminal states refuse before providers. This is
not skill, shell, model, remote job or generic operation replay permission.

## Implementation and focused evidence

- `cmd/internal/recoverygate/gate.go`, `gate_test.go`: exact type/version/status
  allow-list and malformed/result-bearing/future-version negative cases.
- `cmd/internal/workerhost/image_recovery_test.go`: six actual process-boundary
  cases, real read/compute, owned SIGKILL+Wait, actual workerhost/runtime, original
  operation identity, provider wire content, one terminal result/continuation,
  second restart exclusion and both leases free after actual joins.
- `go test -race ./cmd/internal/workerhost -run '^TestM22' -count=1 -timeout=120s -v`
  passed all six cases, package 23.272s. Scratch
  `/tmp/unreal-agent-m22-image.2cwtUi`; exact unit
  `unreal-agent-m22-image-20261028T020020Z.service` stopped/cgroup absent.
- Focused gate tests passed (1.034s), `/tmp/unreal-agent-m22-gate.log`.

The durable producer is an explicit controlled boundary fixture, not an internal
runtime crash hook. It uses the real local manager to prove the old image read
completed before deliberately withholding the final update. Committed-result
history uses the existing v1 format. Test temporary state is deleted by Go test;
assertion logs and source are archived, not claimed as raw fixture state.

## Current-source required acceptance

`/tmp/unreal-agent-m22-validation.wfPUc5`, unit
`unreal-agent-m22-validation-20261028T020444Z.service`:
`make check`, `make build`, `make test` passed, including race tests in both Go
modules. Exact package union: **51 = 48 passing + 3 no-test**. Before/after/current
source manifests match. Result success, exit 0, MainPID 0; observed cgroup empty,
then exact unit stopped and its recorded cgroup absent.

New-built-byte owned private tmux proofs also passed:

- Lifecycle `/tmp/u22.W6Fceo`,
  `unreal-agent-m22-lifecycle-tmux-20261028T021001Z.service`: real model/Bash frontend
  replacement, draft preservation, abort/shutdown, offline reconciliation, exact
  TTY restoration, all **13** result/cleanup flags true.
- Reconnect `/tmp/u22.OXZIKU`,
  `unreal-agent-m22-reconnect-tmux-20261028T021001Z.service`: before-admission and
  accepted/lost-ack cuts, one logical input/sentinel, stable identity/restored
  view, all **4** cleanup flags true in each case.

Both proofs: binary hashes matched new builds, exit 0/MainPID 0, actual owned
process/server/host joins, empty observed cgroups, exact units stopped and
recorded cgroups absent. No user frontend or production session was touched.

Both binaries atomically installed to `~/.local/bin`, mode 0755, and byte `cmp`
against `bin/` passed:

| Binary | Built/installed SHA-256 |
| --- | --- |
| runner | `509cc714e8af7c515bed4c21a22f3fd0a028d3087bbe3ac9e840a592f2b1677a` |
| TUI | `76edcce5f18b71c6dd9ac1c2b0388a28b3fc2918b9f1f28941ac44499024117e` |

[Evidence archive](durable-image-recovery-m22-20261028-evidence.tar.gz), SHA-256
`0fce71f85f79a75817ba4e17329b3d595ea044f7f75b97b6d16d56ceb6891a3f`.
Source comparison used local upstream
`a5f3fd13032737142916523ae4344c392292f9d5`, without fetching. Existing image handler,
codec, translator and pure context builder were unchanged. Crash recovery is
not a power-loss/storage-loss or source-content-identity guarantee.

Continue M2.3 durable bounded model attempts, then all later roadmap slices.
