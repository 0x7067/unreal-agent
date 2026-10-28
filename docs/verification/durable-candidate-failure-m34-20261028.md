# M3.4 accepted: compatibility refusal and stopped candidate failure

[Contract/limits](../plans/durable-candidate-failure.md). [Prior source audit](durable-candidate-failure-m34-audit.md)
is a historical candidate audit, not this acceptance. HEAD remains
`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commits or fetch. Preexisting dirty
work is retained. `harness/taskgraph/graph.go` blob remains
`2315afc35788160fc81bf84657f2600bd61c9e86`. No inbox production changes.

## Observed semantic assertions

- `cmd/internal/resident/runtime_failure_test.go`,
  `TestRuntimeCandidateFailureAfterRetirementRecoversDurableInput`: real worker
  Run/manager join, bounded private old-process exit gate, no candidate before
  Host actually waits for A, durable `private-fenced-stable` input admission during
  the fence, provider0; compatible B then fails startup. Client receives typed
  `RuntimeError`/`stopped`, not success or transport uncertainty. A is actually
  gone, stale repeat replacement is refused, no automatic restart. Actual old
  Serve joins; correct private `state/ingress` journal reopens with original ID
  and body. An explicitly healthy new host/incarnation recovers canonical count1,
  provider1; original-ID retry deduplicates and pending clears after actual joins.
- `cmd/internal/workerhost/m34_format_test.go`,
  `TestM34HistoricalStoreReadsAndStartupRefusalPreserveSource`: immutable local
  upstream v2 reads preserve ID/state/history bytes. Actual worker startup refuses
  legacy-v1, future header999, unknown historical unfinished operation and future
  shell checkpoint999; all preserve canonical bytes and provider0. Fixture
  provenance is in `testdata/m34/PROVENANCE.md`. Readability is not replay approval.
- Full race suite also passes preflight/bad-probe, late retirement, malformed B,
  frozen delivery and `TestReplacementFailureAfterWriteNeverDowngrades`. The last
  is a real current-format append followed by failure, NOT a new-format migration.
- Found/fixed a real resident race: canceling client readers immediately on
  `host.Done()` could destroy the correlated stopped reply. Opt-in failed hosts
  now retain the latched stopped owner until explicit operator stop; no retry,
  downgrade or new worker is launched. Default/error and successful shutdown
  policies otherwise remain unchanged.

## Current required validation and installation

`/tmp/unreal-agent-m34-validation.gZat0u`, named bounded user unit
`unreal-agent-m34-validation-20261028T084837Z.service`:
`make check`, `make build`, `make test` PASS; race in both modules. Fresh exact
package union **54 = 51 passing + 3 no-test**, no missing/extra/duplicate rows,
FAIL/panic/race. Tracked+untracked relevant source manifests before/after/current
are identical, SHA256
`b07d78628914f26b5001ec46f7e79c934339ff224ef557fb75a0a8f0c6ecf96d`.
`acceptance.json`, fresh package lists, logs and manifests are archived.

Both binaries atomically installed via private same-directory temporary files and
rename, then **individual cmp**:

- runner `108383ff36c32a9ada9162ed670d182aaa2200c0c529579b4cdfa5c9ecb4543e`
- TUI `53a07353efa2128e2e3cef1b14fdb8f06cae7eac35b0821a69ee52e38cf9114a`

Focused race proof `/tmp/unreal-agent-m34-focused.fkcXql`, `unreal-agent-m34-focused-20261028T084743Z.service`: both packages PASS.

## Actual private tmux frontend / runtime recovery proof

Driver `.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_failure.py`;
private A/B runners and compatible-probe/startup-failing artifact built in
`/tmp/unreal-agent-m34-build.5VjzIc`. Negative is a private Go overlay of worker
Run only, never production source; probe remains protocol7-compatible.

Accepted `/tmp/unreal-agent-m34-ui.MQA8ig`, `/tmp/u34.argi46`, unit
`unreal-agent-m34-failure-tmux-20261028T085737Z.service`: actual `/builds` ready,
A retirement/old pidfd exit, B startup failure observed in owned stderr, visible
stopped UI, no automatic fallback/provider work; owner remains alive, session
lease free but never used as takeover authority. Owned frontend detach actually
waits/restores TTY. Explicit owned failed-host termination actually waits nonzero;
only then operator installs healthy slot and starts a fresh resident/incarnation.
New frontend attaches; full model/Bash A→B→C replacement, draft/view restoration,
canonical abort/shutdown, offline read-only reconciliation completes. All **13/13**
cleanup assertions true, provider2, sentinel1, no late tool, all process/reader/
TTY/socket/thread waits. Driver/executable before/after hashes identical. The TUI
binary is identical to the accepted M3.3 race-proved TUI; no reader/client/UI source
changed here. This is M3.4 explicit recovery, not M3.5 active host transfer.

Full/focused/build/accepted-tmux units: observed success0/MainPID0/exact recorded
cgroup absent, then stopped each exact unit and observed the same cgroup absent.
No user frontend, production session, or protected output file was touched.

Rejected runs were not relabeled: initial helper long socket path, then incorrect
legacy diagnostic expectation, the real resident stopped-reply race, invalid
zero-generation retry expectation, and test opening `state` rather than real
`state/ingress` were corrected before accepted focused/full evidence. First tmux
`/tmp/unreal-agent-m34-ui.SpEMk1` correctly observed stopped but inherited a detach
assertion requiring an already-started model: exit1/9-of13 scenario cleanup,
actual owned resources joined/exact cgroup absent, then exact unit stopped.
Corrected prelude detach asserts provider0 and actual TTY/process join; final
proof has13/13. Old focused units were also exactly stopped/drained.

Archive `durable-candidate-failure-m34-20261028-evidence.tar.gz` (regular bounded
owned metadata/source/log/canonical fixtures, no executable/socket) SHA256:
`8f662a12a75b9c5410e3392091013ccd5a51337e277a5ee468cab5521bc9accc`.
**21/27 accepted. Continue M3.5 and M4.1–M4.5; foundations are still unwired.**
