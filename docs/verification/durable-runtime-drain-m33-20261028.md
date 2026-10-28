# M3.3 — bounded natural-drain runtime replacement

**Accepted/installed: 20/27; continue M3.4–M4.5. Not roadmap completion.**
[Policy](../plans/durable-runtime-drain.md). HEAD remains
`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`, no commit; inherited dirty/untracked
tree preserved. No inbox production edits and reducer remains
`2315afc35788160fc81bf84657f2600bd61c9e86`.

## Implementation/test boundary

Separate opt-in `runtime_drain_v1` capability: `attachproto.RuntimeRequest`
action `drain`, strict version1/expected generation; `queued` is nonterminal.
`resident.drainRuntime` retries only existing Host `ErrBusy`, once/second, at most
46 attempts/45 seconds, generation checked each attempt. No session work is
canceled, checkpointed, replayed or implicitly accepted to upgrade. Other errors
are terminal/uncertain under the existing stopped/no-rollback Host policy.
Candidate is host-selected and rechecked at handoff, never supplied by clients.

TUI `/upgrade` requests natural drain, `/cancel-upgrade` targets the current
connection-owned request ID. Cancel admission does not claim cleanup; original
request settlement is separate. Existing `/reload` stays idle-only. Queued footer
explicitly says current work continues. `attachclient.RuntimeControl` requires
separate capability before write and never auto-retries. Saved lifecycle controls
retain old scope; reconnect before fresh lifecycle controls after generation swap.

Tests: `cmd/internal/resident/runtime_drain_test.go` real resident+worker, held
provider request completes naturally once then generation advances; canceling
upgrade settles canceled at original generation without canceling the provider.
Focused race unit `unreal-agent-m33-focused-20261028T075014Z.service` passed5.312s,
actual fixture joins and exact unit cgroup absence/stop recorded. Strict capability/
queued state tests in `attachproto/runtime_test.go`, `attachclient/runtime_test.go`;
TUI pending-ID/cancel admission, capability/connection and queued rendering tests.
Existing current Host graph-candidate quiescence tests preserve completed-but-
unaccepted candidates and block replacement; this loop introduces no graph control.

A real race frontend exposed a latent TUI bug: synthetic lifecycle admission
`workerEvent` replies scheduled another socket `Next`. `workerEvent.local` now
prevents local command completions from advancing the reader pump. Both actual
admission construction and no-second-reader behavior have regression assertions in
`attach_lifecycle_test.go`. Bounded disconnect diagnostics now expose protocol
failure rather than hiding it behind generic disconnected status. No store,
coordinator, manager or inbox API was broadened.

## Full acceptance / installation

`/tmp/unreal-agent-m33-validation.l4CUXG`,
`unreal-agent-m33-validation-20261028T082228Z.service`, GOFLAGS=-p=2,
RuntimeMax20min/TimeoutStop10s/KillModecontrol-group. `make check`, `make build`,
`make test` PASS, race both modules; **54 = 51 passing + 3 no-test**, exact fresh
both-module package union/no duplicates/FAIL/race/panic. Before/after/current
tracked+untracked relevant-source manifests byte equal, SHA-256
`3e7b13a04bc51fac5b766f0767ac1ea273fedb48c0d7757f86795a86ffa11894`.
Observed successful exit0/MainPID0/exact recorded cgroup absent, exact stop/absent.
The additional two packages are **unwired, unaccepted M4.3/M4.4 foundations**;
their compilation/tests do not mark those roadmap capabilities complete.

Both binaries atomically installed via same-directory temporary files + rename,
then individually cmp checked; hashes/receipts in archive `validation/install.txt`.
Only private owned frontends were replaced. Production session/worker
`33b63c18-e05a-487f-a0a9-2f5411dac58a` and user frontend untouched; no protected
tool-output files read.

## Actual two-build normal and race frontend proofs

`.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_drain.py` extends
accepted M3.2/lifecycle drivers using checked transforms. A/B runner artifacts in
`/tmp/unreal-agent-m33-build.tzMeYm`: same runner production source, distinct Go
link build IDs; negative changes only worker-probe protocol7→999 in private Go
overlay. This proves compatible artifact replacement, not an untested new release.
Normal TUI is current `make build`; private race TUI from
`/tmp/unreal-agent-m33-race-build.OEKfVg`. GORACE=atexit_sleep_ms=0 removes only
the instrumentation exit delay; it does not suppress race reports or exit errors.

- Normal `/tmp/unreal-agent-m33-ui.00iPK7`, `/tmp/u33.Bqos4V`, unit
  `unreal-agent-m33-drain-normal-tmux-20261028T082955Z.service`.
- Race `/tmp/unreal-agent-m33-ui.4RDgpx`, `/tmp/u33.75P2pE`, unit
  `unreal-agent-m33-drain-race-tmux-20261028T082955Z.service`.

**Both exit0, 13/13 cleanup true, unchanged executable/driver hashes, no races**;
actual frontend/host/offline child waits/TTY receipts and socket/thread joins.
Both units observed success0/MainPID0/exact cgroup absent, exact stopped/absent.

1. Incompatible candidate leaves A alive with exact exclusive lease and zero
   provider calls; running A/candidate B digests visible.
2. Upgrade queued during actual safe model request; one held request, unchanged A
   pidfd and lease holder. Provider released naturally, output committed once,
   old process exits, B generation2 acquires the same exclusively held lease.
3. Upgrade queued during next blocked model, then canceled: matching original ID
   settles canceled, cancel acknowledgment separate, original model remains alive
   and worker generation/lease unchanged. Canonical user abort is a later action.
4. Upgrade queued while unsafe Bash has written its sentinel; canceled without
   killing/replaying Bash or opening its gate. Later explicit user foreground
   abort, not upgrade, stops it. Sentinel exactly `x`, no late tool.
5. Full frontend A→B→C replacement/draft retention, canonical abort/shutdown,
   offline read-only reconciliation and exact owned cleanup complete. Exactly
   **3 actual provider requests**: 1 safe natural-drain prelude + 2 baseline
   model/Bash calls; baseline counter intentionally excludes that separate prelude.
   Replacement worker pidfd exit observed after actual host join.

The observer scans bounded children across every owned host thread and selects
exactly one **held session lease descriptor**, not an arbitrary child PID: a
queued request legitimately has a transient side-effect-free candidate probe.
Exclusive flock refusal, lease dev/inode and bound old/new pidfds supplement the
actual Host process waits; no PID-only takeover permission is inferred.

## Rejected/superseded evidence

Early completed full runs M7OvgZ/ovqggy were superseded by relevant TUI fixes;
only l4CUXG is current acceptance. Private early wrapper returned0 without
executing the imported fixture (`__name__` fence); rejected by missing semantic
receipts, fixed explicit entrypoint. Queued-status proof exposed hardcoded
reloading footer; fixed with rendering assertion. Later shell phase exposed the
real duplicate-reader race described above, not masked by reconnect/retry.
Diagnostic race stderr is private owned fixture evidence, not protected output.
A post-fix race run reached scenario end but missed final frontend join under
instrumentation's exit delay; not accepted. A normal observation encountered the
transient probe child; observer corrected to exact lease holder, not weakened
writer checks. Both final proofs above are fresh after observer/source fixes.
All rejected/superseded units stopped/exact drained; their success/effect/cleanup
flags were not relabeled acceptance.

## Limits / upstream

Natural drain only, bounded/no guaranteed idle under continuing admission;
unsupported/live work remains queued until natural quiescence or deadline.
No generic forced reload, active shell replay, new migration, host transfer,
remote job capability, timer delivery, power-loss proof or exactly-once external
billing claim. Local upstream `a5f3fd13032737142916523ae4344c392292f9d5`
(no fetch) lacks resident/private/runtime-control layers; optional local divergence
surfaced. Shared pure translators/context builder and coordinator committed-
operation dispatch remain unchanged. M3.4 real failure/compatibility gaps remain
tracked in `durable-candidate-failure-m34-audit.md`.

Evidence archive SHA-256: `9717eca33efa733080b51b8543974b91ec8ebfb0724ab1d198343c1298dcb09f`.
