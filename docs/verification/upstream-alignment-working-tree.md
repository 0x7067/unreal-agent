# Upstream alignment / scheduled reload — working-tree verification

This records the implementation described in
[the alignment plan](../plans/upstream-alignment.md) and
[the reload contract](../plans/hot-reload.md). Verification was recorded before
committing the implementation.

Scope: baseline/reload implementation committed as `2243def`. Build hashes and
installation statements below describe that verification point, not subsequent
builds. Later reader/formatter work is covered by the
[simplification-pass report](simplification-pass.md).

## Scope and compatibility

- Settings, Messages/Responses retry implementations and the base preamble compare
  byte-for-byte with `upstream/main`. Unknown models have zero settings; long server
  retry hints are subject to upstream replacement/capping semantics.
- Coordination guidance is selected by registered capabilities. Baseline tools do
  not acquire graph/helper instructions.
- Reload uses canonical `StopWhenIdle`, authoritative retained-owner/queued-input
  checks and host-owned completion. A canceled client wait does not revoke admitted
  retirement. Post-cutoff ordinary input is durable but cannot reach the old worker.
- Scoped interventions can finish existing graph/helper work, not launch arbitrary
  new work. Failed cleanup/receipt/join/lease proof cannot authorize another attempt
  to replace an unproven worker.
- Private worker protocol is **8**; public frame/payload versions remain **1** with
  explicit v2 runtime capabilities and separate `reload_intervention.v1`. Session
  format remains **2**. These changes do not make extended fork histories upstream-
  readable. The durable request ledger/replay policy is intentionally unchanged.

## Automated verification

Final root race-suite result: **PASS** — `go test -race ./... -timeout=10m`.
All packages passed; unchanged packages legitimately used Go’s test cache. The
final workerhost suite ran for 237.743s, resident for 57.986s, and all changed core
packages were revalidated after the final source changes.

Completed checks:

- `make check` (vet both modules), `git diff --check`, documentation links.
- TUI module: `go -C cmd/unreal-agent-tui test -race ./... -timeout=5m`.
- Settings/providers, prompt scope/compaction and runner policy regression tests.
- Inbox, coordinator, local session store, worker runtime and recovery gate race
  suites after the final producer/admission race fix.
- Public attach codecs/client and resident runtime/intervention actual-process race
  tests. Cancellation after admission retains host ownership; scoped graph controls
  preserve generation/evidence rules.
- Supervisor full race suite and retained-work tests repeated ten times; bounded
  diagnostics and completed background reports do not certify process cleanup.
- Host adversarial cases cover stale/missing/replayed receipts, held writer leases,
  process/output/callback joins and fail-closed retries. Eleven adversarial tests
  passed five race repetitions; the late-input cutoff case passed twenty.
- Actual-process model/shell/helper and graph-awaiting-acceptance drain tests,
  including explicit stop of a retained background helper after its final report.
- Inbox admission model checker: **184 / 46 / 14 states**; frozen admission also
  passes. An isolated Go-overlay deduplication mutant failed all three expected
  seeds. The production source was never mutated and temporary overlays removed.

Earlier broad runs were rejected: the initial 120-second package budget was too
short for real/race subprocesses and the exhaustive graph checker. A subsequent
full run exposed stale protocol-7/cutoff/tail-record test expectations; these now
assert protocol 8 (including explicit rejection of 7), the irreversible process
cutoff and exact ordered canonical StopWhenIdle/ExecutionDrained records. Its
stalled-watcher test also exhausted its shared 24-second budget across publication
and nine malformed-client handshakes. The overall budget is now 90 seconds; each
attack retains its original 2-second deadline and all rejection/bounds assertions.
Its dial helper now reports failures through the actual subtest, not its parent.
No failing assertions were skipped. The exhaustive graph race check passed in
356.952 seconds; its reducer implementation was not changed.

## Final-build isolated live TUI proof

A deterministic loopback provider, temporary workspaces/config/state, and two
private tmux sessions exercised the built production frontends/workers. No real
credentials or user's frontend were used or restarted.

Evidence: `/tmp/ua-alignment-live-5k083ccj` (ephemeral local directory), containing
provider requests, screen captures, per-case results and empty stderr logs.
Driver: `/tmp/ua-alignment-live-proof.py`, SHA-256
`f45f1ea18bb9c9d466b5ce86971993f9d4af5eaf8842ee857a8ea00826b53cdc`.

| Case | Frontend | Old worker | Replacement | Provider requests | Result |
| --- | ---: | ---: | ---: | ---: | --- |
| Held model response | 1321488 | 1321550 | 1321776 | 2 | Scheduled stop; queued input only in replacement |
| Graph awaiting acceptance | 1321983 | 1322088 | 1322400 | 4 | `/graph status` and explicit accept; command/effect exactly once |

Both checks observed old-worker pidfd exit before replacement, with the frontend
unchanged across handoff. Both owned frontends and all four workers subsequently
exited; isolated tmux sessions were cleaned up. Background-helper retention is
covered separately by actual-process tests, not inferred from this graph proof.

Built by `make build`:

| Binary | SHA-256 |
| --- | --- |
| `unreal-agent-runner` | `5423fe48a11e08c49238a8bfec1d09cc82d59f661eb6ae6b70ef6edbe37d2026` |
| `unreal-agent-tui` | `4b7625e50e609e18bcca1edae57175ae6142341a739b45d1a1833a35b4891939` |

Installation verification: **PASS**. Both binaries were staged on the destination
filesystem, chmodded 0755 and atomically renamed over `~/.local/bin` targets.
Built and installed bytes/SHA-256 hashes match the table above. Existing user frontends must
be restarted by their owner to load UI/capability changes; none was killed here.

Go source/module/prompt manifest: 586 files, SHA-256
`bda332ff1765addd97031ba993464d0d97096f0ef60d511b4889e9f7f339dd90`.
The local manifest is `/tmp/ua-alignment-source-final.sha256`.
