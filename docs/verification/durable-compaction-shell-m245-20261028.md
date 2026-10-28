# M2.4 and M2.5 — bounded summaries and conservative shell recovery

**Accepted, documented, proved and installed 2026-10-28.** These two slices are
complete for their exact reviewed cells; M2.6–M4 remain required, not complete.
Contracts: [compaction](../plans/durable-compaction-recovery.md),
[shell recovery](../plans/durable-shell-recovery.md).

## M2.4 contract and evidence

- Separate strict `compaction_request` v1, nested request v1, immutable originating
  manual input ID and failure baseline. Regular request v1/limit 2 unchanged.
  Persist policy and charge before turn/Respond. Default summary failure limit 3;
  changed runtime configuration cannot replenish persisted budget. Private worker
  protocol **5**, session format **2**; old readers reject the new control.
- Interrupted summaries retry as summaries, including manual-only and bare-charge
  gaps. Unusable attempts count once; exhaustion disables automatic compaction
  without delivering queued ordinary input, inventing a summary or losing history.
  New explicit manual IDs authorize bounded overrides, not automatic re-enable.
- `workerhost.TestM24OwnedAutomaticSummaryRecoveryAndCommittedAdoption`: actual
  host/runtime/provider SIGKILL before summary completion, successful bounded
  retry, observed committed usable summary plus ordinary request, second SIGKILL,
  canonical reconstruction. Exactly one committed/adopted usable summary; no new
  summary after commit; pending input occurs once per legitimate ordinary attempt.
- `TestM24OwnedManualOnlyLossesExhaustAndRemainDisabled`: three actual process
  losses, same logical request/origin and bound, no fourth summary, disable survives
  another restart, same resident runtime accepts new input using original history.
  All owned child/reader/dispatcher/HTTP-handler/lease joins are checked. Unknown
  killed-worker drain is a diagnostic, never fabricated proof.
- Coordinator cases cover mixed committed failures/crashes, five store fault
  boundaries, impossible reconstruction refusal, partial-tool fencing, manual
  coalescing/supersession/consumed and fork-inherited origins, hard-stop gaps,
  newer manual input surviving regular retry, and old policy/history reads.
- Controlled canonical summary fixture commits directly without applying it to
  the old builder, then reconstructs/adopts once. It is explicitly not an internal
  live hook: current live adoption occurs before its store call returns. The real
  process cut follows observed commit/live adoption and loses that memory.

## M2.5 contract and evidence

- Only exact shell-v3/v4 awaiting read_out/read_out_tail/read_err/read_err_tail
  cells recover: known exit, no PGID/result/error, strict data and phase buffers.
  They perform capture reads, never ProcessStart. Ready/preparation/process and
  canceling remain uncertain and require ownership reconciliation. PID absence,
  PGID observations and fresh-manager receipts cannot discharge old ownership.
- `workerhost.TestM25ControlledShellRecovery`: ten controlled durable-boundary
  cases, actual original sentinel shell, ready commit before dispatch, latest
  checkpoint SaveOperation, actual manager drain before owned producer SIGKILL,
  scanner/Wait/lease joins, then actual runtime/provider. Both versions and all
  read phases recover available output once; missing capture fails safely with
  recorded exit/capture paths; ambiguous and cancellation cells refuse unchanged.
- `TestM25LiveOldExecutorRemainsUncertain`: separately fixture-owned executor
  remains alive while replacement refuses twice. No second sentinel, provider
  call or canonical mutation. Cleanup only through original manager capability,
  actual AwaitStopped/reader join and local receipt; no PID-only signaling.
- Known-exit output-failure projection is pure/nonmutating. Current owned capture
  artifacts are read; immutable content identity and detached-daemon/global drain
  are not promised. Shared translator projection divergence was surfaced; gate is
  private and absent from local upstream. Mandatory interfaces/formats unchanged.

## Required current-source acceptance

`/tmp/unreal-agent-m245-current-validation.Pij0EV`, unit
`unreal-agent-m245-current-validation-20261028T040240Z.service`:

- **make check, make build, make test PASS**, race in both modules.
- Exact union **51 packages = 48 passing + 3 no-test**; no omissions/duplicates.
- Before/after/current source manifests match; acceptance.json reconciles logs.
- Unit success/exit 0/MainPID 0; observed cgroup already absent; exact unit stopped,
  recorded cgroup absent. No validation service intentionally left running.

Rejected initial candidate `/tmp/unreal-agent-m245-validation.GezMDZ` failed the
old quiescence race assertion counting internal request charges as user input.
It was drained, not accepted. Test-only correction now verifies exact external
identity/payload plus the typed charge's bound/cursors/turn; 100 race repetitions
passed. The final full pipeline passed. Normal Go cache reuse was allowed for
unchanged package inputs, not substituted for the required pipeline.

## Private frontend proof and installation

Fresh built-byte private tmux proofs ran before that **test-only** correction:

- Lifecycle `/tmp/u245.YqGJDT`, unit
  `unreal-agent-m245-lifecycle-tmux-20261028T035833Z.service`: all **13** cleanup and
  model/Bash frontend-replacement/draft/TTY/abort/shutdown flags true.
- Reconnect `/tmp/u245.HkjamC`, unit
  `unreal-agent-m245-reconnect-tmux-20261028T035833Z.service`: pre-admission and
  accepted cuts, all **4** cleanup flags true each, stable original input and view.

Both services exit 0/MainPID 0; owned joins completed and exact cgroups absent.
Final built binaries and driver hashes match those proofs byte-for-byte; manifests
prove only `harness/coordinator/quiescence_test.go` changed between proof and final
validation. Proofs were reused explicitly, not claimed rerun. All state/endpoints
were fixture-owned; no user frontend or production session was touched.

Both binaries atomically installed with mode 0755 to `~/.local/bin`, individual
byte cmp PASS; built/installed SHA-256:

- Runner: `0bf067d3e5ce0ee8d5aaa8fe99bc5e57a276758c64d3d1647a67ceea886f8f1e`
- TUI: `c3a2ae66b1d7b7f8ff977d30855612610dddd5086278d8bed8f9bb5de2e58a40`

## Evidence and remaining work

[Evidence archive](durable-compaction-shell-m245-20261028-evidence.tar.gz), SHA-256
`a90309715d38415b96a4c2676d1125c6b2a81ea97bc0055e7c4ac8d7658a99e3`:
full and rejected logs/manifests/package lists, acceptance/install/drain records,
focused logs, assertion source and private frontend evidence. Go temporary process
fixtures were deleted normally; no claim of retained raw process state. Inbox
skill exact command passed before/after final source; working-tree blobs recorded.
No commit requested or made; preexisting dirty/untracked work preserved.

Scoped helper structural snapshot 04:07:55Z shows process/EOF/reader/owner/terminal
joins and zero nonterminal operations for shell proof helper 0db9fd1a (generation 1),
final summary-edge helper 57c5ea2e (generation 1), and read-only child audit 4ba07629
(generation 2). No helper intentionally retained for these completed slices.

**Continue M2.6, M2.7 and every M3/M4 slice.** Legacy remote-job/child ownership is
still fenced. The child audit found an enqueue-before-launch acknowledgement gap
and in-memory-only control authority; same SessionID alone is not reattachment.
