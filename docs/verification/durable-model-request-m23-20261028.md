# M2.3 — durable bounded regular-model request recovery

**Accepted and installed 2026-10-28. M2.3 is complete; M2.4–M4 are not.**
The [reviewed contract and limitations](../plans/durable-model-request-recovery.md)
cover regular requests only. Compaction recovery is separate future work.

## Implemented and tested contract

- Strict canonical `inbox.ModelRequest` v1 records persist logical RequestID,
  unique attempt TurnID, attempt/bound and input cursors **before turn/Respond**.
  Coordinator enables regular requests with bound **2**; malformed/future/causally
  inconsistent records refuse. Bare charge crash gaps still consume an attempt.
- Restart records interruption and preserves the logical ID/bound. Exhaustion
  makes no third provider request and invents no response or usage. It consumes
  only the original request cursor and stays available for newer external input.
- Committed responses replay without another request. Failed or non-StopComplete
  responses cannot dispatch tools, including legacy recorded statuses. The pure
  context builder strips partial calls/opaque continuation state. TUI shows
  incomplete/exhausted notices, not partial pending-tool cards.
- Explicit abort/hard stop cannot autoretry. Independent review found the gap
  `interrupted → committed StopHard → crash before stopped record`; fixed by
  honoring stopped markers for both started and interrupted attempts, with an
  exact regression. Supersession preserves pending input; late output is dropped
  and only actually returned nonzero/structured usage retained once as inert.
- Completed legacy history reads. Uncharged legacy unanswered regular/compaction
  turns refuse rather than invent a budget. Forks do not acquire parent request
  authority. Private worker **4**, session format **2**; old private workers reject
  before retirement, and old canonical readers reject the unknown control mode.

Implementation: `harness/coordinator/request_recovery.go`, integration in
`loop.go`, `compaction.go`, `foreground_abort.go`; strict
`harness/inbox/model_request.go`; pure `contextbuilder/builder.go`; worker protocol
and TUI notices. No mandatory adapter/manager interface or public attach protocol
change, translator/context-builder I/O, generic operation replay or TaskGraph
reducer change. Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` was compared
without fetching; this request-ledger/complete-only divergence was surfaced
before source edits.

## Real and focused evidence

- `workerhost.TestM23OwnedInterruptedModelRecovery`: actual host/runtime/provider,
  two partial tool-looking SSE process cuts, exact owned SIGKILL/Wait/output/owner
  joins, charge-before-observed-HTTP ordering, stable logical ID/new turns,
  exactly two requests before exhaustion, zero uncommitted tool status/effect or
  invented response/usage, then a new logical request on the same live runtime.
- `workerhost.TestM23CommittedResponseReplay`: exact owned kill after canonical
  response commit; recovery makes no extra request/response, preserves the prefix
  and allows only its own scoped stop/drain tail. HTTP handler/dispatcher joins
  and both owner leases checked. Missing killed-worker drain receipt remains an
  honest diagnostic, not manufactured execution-cleanup proof.
- `coordinator/request_recovery_test.go`: repeated restart exhaustion, charge-only
  gap, legacy/corrupt refusal without mutation, new work, completed replay,
  incomplete tools, exact known late usage, invalid transitions and hard-stop gap.
- `contextbuilder/partial_response_test.go`, inbox codec tests and TUI notice tests.
  Exact inbox safety skill passed before/after input validation edits (184/46/14
  states plus frozen-admission case); actor unchanged and skill record updated.
- Focused race process package passed (8.490s); complete coordinator race passed
  (1.909s); CLI package race passed (25.004s); real headless disconnect/reconnect
  passed; wire replay and real late-join snapshot/watch regressions passed.

Two full candidates were **rejected**, not accepted: first stopped early after
CLI oracle failures; second exposed protocol-only fake worker advertising 3,
new snapshot-head counts, and complete synthetic wire-response fixtures missing
StopComplete. Exact charge/order/identity checks were added, intended-complete
fixtures made explicit, and guards retained. Both rejected units, the failed
focused handoff unit, and proof units were stopped and their recorded cgroups
verified absent. Their logs are included, not disguised as passing evidence.

## Required current-source acceptance

Scratch `/tmp/unreal-agent-m23-current-validation.OOQs3K`; unit
`unreal-agent-m23-current-validation-20261028T030333Z.service`:
**make check, make build, make test** passed, race tests in both modules.
Exact package union **51 = 48 passing + 3 no-test**; before/after/current manifests
match. Unit success/exit 0/MainPID 0; observed cgroup empty, exact unit stopped,
recorded cgroup absent. Acceptance is based on scenario assertions, package and
manifest reconciliation—not merely a successful exit.

## Built-byte private frontend proof and atomic installation

- Lifecycle `/tmp/u23.zE71KW`, unit
  `unreal-agent-m23-lifecycle-tmux-20261028T025354Z.service`: actual model/Bash frontend
  replacement, draft preservation, scoped abort/shutdown, offline reconciliation,
  exact TTY restoration and owned joins. **All 13 flags true.**
- Reconnect `/tmp/u23.xDm6sC`, unit
  `unreal-agent-m23-reconnect-tmux-20261028T025354Z.service`: both pre-admission and
  accepted/lost-ack cuts, exact identity/input/sentinel counts and restored view;
  **all 4 cleanup flags true in each case**.

Final rebuild bytes and driver hashes exactly matched these successful proofs;
subsequent corrections were test-only, so the byte-identical proof was reused,
not falsely described as a fresh run. Both proof units exited 0/MainPID 0,
observed cgroups empty, exact units stopped/cgroups absent. All workspaces,
providers, HTTP handlers, private tmux servers, frontends and hosts were owned
fixtures. User frontend and production session/worker were not touched.

Both binaries installed via temporary mode-0755 files and atomic rename into
`~/.local/bin`; byte `cmp` against `bin/` passed for each:

| Binary | Built/installed SHA-256 |
| --- | --- |
| runner | `7d3a41c341c85053defeb675078d727c2c3b9fa913425a913177aeb189d499dd` |
| TUI | `208ed550ff745b346771f3ccba0f06f9e748fc783de83639f07d228a66ea521b` |

[Evidence archive](durable-model-request-m23-20261028-evidence.tar.gz), SHA-256
`fafbcde8b4e3ff58254e1452d7494f16b289977c2da75bbb68c7b4d9dd523b60`.
Includes logs/manifests, point-in-time source, rejected candidates and private
proofs; Go-test temporary process state was deleted, not claimed as archived raw
state.

**Duplicate provider billing is possible:** a charged attempt is intent, not
proof of HTTP dispatch; an interrupted provider may have billed work whose
response never committed. Cache affinity is not idempotency. No exactly-once
execution/billing, unknown-usage estimate, power-loss or storage-loss guarantee.
Continue M2.4 compaction and every later roadmap slice.
