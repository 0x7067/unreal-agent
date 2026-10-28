# M2.4 — durable compaction request recovery

**Complete for the reviewed cells, accepted and installed 2026-10-28.**
[Required current-source validation, exact package/manifests, process proofs,
private frontend replacement and installed byte checks](../verification/durable-compaction-shell-m245-20261028.md).
This contract does not enable unclassified work or complete M2–M4.

## Reviewed narrow extension

Keep the M2.3 regular `model_request` v1/limit-2 contract unchanged. Add a separate
strict, versioned `compaction_request` control, so an older reader rejects it
rather than ignoring a retry counter. Top-level v1 data: nested v1
ModelRequestRecord (TurnType compaction), FailuresBefore and ManualInputID.
A nonempty ManualInputID proves which canonical Compact input authorized the
manual request; an empty one denotes automatic compaction. Bump private worker
protocol to 5, retain session format 2 and backward reads of existing completed
summaries and compaction policy/disabled controls. Legacy unanswered summaries
without the new attempt policy remain refused; no invented old budget.

Preserve pure context building and summary validation. No operation-manager or
adapter interface extension, translator I/O or generic recovery permission.
Compare local upstream and surface the concrete divergence before source edits.

## Accounting and intent rules

- Persist existing SetCompactionPolicy before the first new compaction charge.
  Recorded policy wins over changed runtime dependencies. Default remains 3.
- Charge before turn/Respond, including a charge-before-turn crash gap. A logical
  retry retains RequestID/new TurnID and immutable origin metadata. For automatic
  work the charge bound is the remaining existing consecutive-failure budget;
  a new explicit manual request gets a fresh bounded policy attempt group even
  when automatic compaction is disabled, as an authorized override, not an
  automatic re-enable. The upper codec bound must be explicit, never clamped.
- Persist interruption once; each unusable charged summary attempt consumes the
  existing consecutive-failure budget once. Interrupted transport/crash, and
  superseded/stopped still-started compaction attempts are unusable; settling an
  already-interrupted attempt must not count twice. A committed unusable/partial
  summary is already counted by existing applyCompactionResponse, also once.
- A pending interrupted summary is rebuilt as that summary, even if the ordinary
  scheduling heuristic would no longer trigger. Original manual-only intent
  survives turn replay even without pending user input. Never turn a retry into
  a different request kind silently.
- Track pending Compact input identity on the actor. A new manual input after
  the original turn is a new explicit intent; it can supersede the old attempt
  group without silently reusing/resetting the old logical identity. A bare
  charge crash must consume/settle only its own originating manual identity so
  exhaustion cannot reuse the same unconsumed Compact input forever. Forks read
  parent context but do not acquire parent manual/request authority.
- Exhaustion does not deliver ordinary pending input or invent a provider
  response/summary. Preserve the working history; persist disable when the
  existing consecutive-failure limit is reached. Ordinary queued work proceeds
  on original context with automatic compaction disabled. Manual-only exhausted
  work goes idle; a later explicit manual request remains possible. Explicit
  whole-session stop still must not autorestart its original input work.
- Validate/adopt only committed usable summaries, once on replay. Successful
  summary resets the existing failure count and automatic-disable state exactly
  as before. Missing/unusable/partial summary cannot erase original history or
  dispatch its complete-looking tool calls. Late usage-only receipts are inert
  and do not count a second summary failure or adopt opaque output.

## Required boundary tests before acceptance

1. Automatic summary interrupted before completion, then successful retry:
   stable logical identity/bound, fresh turn, one usable adopted summary, pending
   external input still delivered exactly once to the ordinary request.
2. Manual-only summary interruption with no pending ordinary input: original
   intent retries within its persisted bound despite the consumed turn trigger.
3. Repeated actual runtime losses exhaust the persisted compaction bound, do not
   reset failure/disabled state after restart and do not make a fourth summary.
   Full original context/new accepted input remains usable after disable.
4. Charge-before-turn gaps, old-policy/new-dependency reads, manual override while
   disabled, new manual ID vs original ID, fork ownership and hard-stop gaps.
5. Kill after usable summary commit; next rebuild adopts only that committed
   summary once without another summary request. Be explicit that the current
   actor applies pure local state before its store call returns: do not claim an
   uninstrumented internal "before live adoption" hook. Durable-commit/rebuild
   evidence must state the actual tested boundary.
6. Partial/failed/unusable summaries preserve history/input, known usage only,
   zero partial-tool dispatch, and existing policy/disabled/completed-history
   fixtures still read. Unknown version/malformed causal metadata refuse before
   provider work or canonical mutation.

Required acceptance remains both-module make check/build/test, matching current
source manifests/package union, actual owned process/unit/cgroup drain, atomic
both-binary install+cmp and new-built private frontend replacement proof. Continue
M2.5–M4 afterward; this is not broad model/operation migration permission.

## Review clarifications

Independent read-only design review confirmed the needed origin/cursor fences.
Multiple Compact inputs accepted before a turn coalesce deterministically to the
latest pending ID, matching the prior boolean scheduling behavior. A newer manual
ID after an interrupted attempt supersedes that group explicitly; it is not an
automatic reset. A committed unusable manual-only summary remains one-shot/idle;
the bounded group retries interrupted attempts, not recorded failed outcomes.
Local upstream has no compaction.go or compaction controls; this private extension
and protocol-5 boundary were surfaced before edits. M2.5 now has its own narrow [shell policy](durable-shell-recovery.md); neither slice is accepted solely from focused tests.
