# Upstream alignment and simplification

Status: **baseline policies and scheduled reload implemented**. The unrelated
extractions and durable-history redesign below remain backlog.
Comparison baseline: `upstream/main` at `a5f3fd1`, fork `main` at `1f276f6`.

## Direction

Keep ordinary execution close to upstream. Put orchestration and durability at
the application/owner boundary. Prefer removing a second mechanism over hiding
it behind an interface. Avoid a general lifecycle framework, a compatibility-mode
matrix, or more environment switches.

Preserve pure translation/context construction, commit-before-dispatch, failed/
incomplete-tool rejection, usable-summary validation, bounded compaction failures,
explicit graph acceptance, exclusive writer ownership and fail-closed recovery.
Input admission, session persistence, model delivery, cancellation, owner joins and
process exit remain distinct facts.

## Implemented

### Upstream baseline

- `harness/settings/settings.go` restores upstream unknown-model zero values;
  runner/TUI no longer invent a 64k threshold or emit fallback warnings. Explicit
  listed-model settings/normalization are unchanged. Fork-only `LookupModel` and
  `DefaultCompactionThreshold` APIs are removed.
- Both providers' `retry.go` match upstream. Server hints again replace local
  backoff and are capped; provider-specific parsing/classification remain distinct.
  This can retry earlier than a long server-requested wait. Removed the unused
  fork-only `retryhint` package rather than adding another policy switch.
- The upstream preamble stays unchanged. Registered TaskGraph, Agent and
  AgentControl capabilities select their own concise guidance through `AddTool`;
  baseline tools do not receive coordination instructions. No new public builder API.

### Scheduled worker reload

See the authoritative [reload contract](hot-reload.md). The direction agreed in
review is now implemented:

```text
stage/probe once -> host delivery cutoff -> ordinary StopWhenIdle
  -> finish current work -> join owners/process/output -> prove lease release
  -> start staged replacement -> deliver queued ordinary input
```

Removed `TryQuiesce`, its coordinator channel/lifecycle synchronization, special
exit sentinels and the separate polling-based `/upgrade` implementation.
`Coordinator` again requires only `Run(context.Context) error`.

The necessary boundary changes are deliberately limited:

- Scoped structural graph/helper interventions can finish existing work during
  drain; no arbitrary start/revise/retry lane. Explicit task-generation/evidence
  checks and worker scopes prevent stale controls reaching a replacement.
- Native supervisors publish authoritative retained work, including process/output,
  preparation/finalization and parent delivery ownership. Optional worker wiring
  wakes ordinary idle-stop checks; bounded diagnostic snapshots grant no authority.
- Inbox admission acknowledges only after queued ownership is published. The idle
  decision reads producer ownership **before** queued input, closing the race where
  a child submits after event slurp but before its owner releases responsibility.
- Canonical structural checkpoints remain readable after terminal controls disappear
  from `Resume`, so cleanup proof does not reconstruct operations from result text.
- The host owns admitted retirement even if the client cancels its wait. Successful
  cleanup, actual joins and a writer-lease probe precede safe-stopped permission;
  failed proof cannot be bypassed by a later reload.

Busy reload now schedules rather than immediately refusing. `/cancel-upgrade`
cancels waiting, not an already admitted retirement. Private worker protocol is 8;
public runtime capabilities are v2, with `reload_intervention.v1` advertised
separately. Old peers cannot silently opt into the new semantics.

## Still deliberately different from upstream

- Failed/incomplete responses cannot dispatch tools or carry executable/opaque
  continuation state into later context. Refusal text retains its semantics.
- Unusable compaction preserves context and queued input; three consecutive
  failures disable automatic attempts by default, with explicit manual retry.
- Ordinary model requests still write the existing durable request ledger. Its
  two-attempt interrupted-request contract, replay validation and charge-before-
  dispatch semantics have **not** been removed or silently weakened.
- Session headers remain format 2 with fork-only controls/checkpoints. Simpler
  new writers would not retroactively make existing histories upstream-readable.
  Scoped graph interventions also require this fork. Unknown versions are refused.

## Deferred opportunities (not part of this change)

| Area | Potential reduction | Required gate |
| --- | --- | --- |
| Durable request/compaction accounting (`harness/coordinator/*recovery.go`) | Separate ordinary execution from explicitly selected durable recovery. | Inventory replay/abort/scheduling dependencies first. Existing histories retain their recorded budgets, manual origins and effect authority; define interrupted plain execution explicitly. |
| History compatibility (`harness/sessionstore/localfile/codec.go`) | Make the extended-history boundary explicit rather than relying on the same format-2 header. | Decide whether format versioning or required capabilities is smaller. Preserve legacy readers/fixtures; never strip controls/checkpoints or silently downgrade. |
| Read-only lifecycle inspection (`harness/sessionstore/lifecycle.go`, `localfile/readonly_lifecycle.go`) | Accept Items-only reading instead of embedding a nil writable Store. | Preserve ordered decoding, lease requirements and diagnostic-only authority. |
| Attach transport (`cmd/internal/residentcli/run.go`, `attachclient/client.go`) | Reuse the existing client rather than a CLI-specific connection state machine. | Preserve exact CLI JSONL/snapshot metadata, single reader, authentication, cursor and stable-ID semantics. Do not merge public attach and private worker protocols. |
| Execution composition (`agentrunner/run.go`, `workerruntime/setup.go`) | Share only identical supervisor/graph assembly and ordered joins. | Preserve partial-setup cleanup, credentials, logs, different defaults and independent cleanup evidence. No universal runner/config framework. |
| Activity summaries (`agentrunner/activity_summary.go`, `workerruntime/diagnostics.go`) | One pure formatter over detached snapshots. | Keep redaction, omission markers, explicit bounds and report-versus-drain distinctions; never turn formatting into idle permission. |
| Management admission (`harness/operation/management_budget.go`) | Let trusted handlers classify their own versioned controls instead of decoding tool protocols in the generic manager. | Unknown/malformed plans remain ordinary work; preserve bounded cancellation capacity and separate classification from authorization. |

No reader/formatter/attach/setup extraction or workload state-machine unification
was undertaken merely because the review identified it. Evaluate later changes by
fewer production paths and a smaller maintained upstream diff, not more wrappers.

Current validation and live-proof evidence is recorded in the
[working-tree verification report](../verification/upstream-alignment-working-tree.md).
