# M0.1 — One submission from acceptance through replay

Scope: characterize current behavior for [goal slice M0.1](../../goal.md#m01--trace-one-submission-through-acceptance-and-replay).
Status: **complete**, with [full verification evidence](../verification/durable-submission-m01-20261027.md).
No production protocol, interface, recovery policy, or acknowledgment semantics
are changed. This is not the detachable-host implementation.

## Executable scenario

[`TestRealWorkerSubmissionAcceptancePersistenceDeliveryAndReplay`](../../cmd/internal/workerhost/submission_lifecycle_test.go)
uses the production host, ingress journal, subprocess worker, coordinator,
session files, provider adapter, and history replay. Only the HTTP provider is a
loopback fake. An existing delivery fence and a provider response gate establish
causal boundaries without quiet-period sleeps.

1. Hold worker delivery. Submit a stable ID with raw JSON whitespace, retry its
   wire-normalized equivalent, and reject a changed payload with `ErrConflict`.
   Observe one confirmed pending journal identity, no persisted external input,
   no turn, and no provider request. `Submit` success cannot mean model delivery.
2. Release delivery. The provider receives the input exactly once in its request.
   While its answer is gated, observe one persisted input and turn, no committed
   response, and no pending ingress entry after the host observes persistence.
   Retry the same submission while the request is active.
3. Release the answer and observe one committed response. Close the host cleanly;
   start a fresh host and worker against the same files. Compare the complete
   replayed typed history, including content and order, with persisted items.
4. Retry the acknowledged identity, reject a conflict again, and complete an idle
   worker reload as a delivery/lifecycle barrier. Assert one input, one turn, one
   response, one total provider request, and no pending retry.

The restart here is **after a completed turn and proven clean shutdown**. This
scenario does not demonstrate recovery of an interrupted effect or prove
exactly-once execution across a crash.

## State inventory

Paths below refer to the current implementation, not a requirement that future
hosts preserve the same process layout. A session directory is abbreviated `D`
and the session ID `S`.

| State | Current owner / source symbols | Authority and reconstruction | Failure boundary |
| --- | --- | --- | --- |
| Draft, scroll, selection, expanded details | TUI `model`, composer and conversation state | Client-local volatile memory; not canonical session history | Frontend loss loses this state; no durable view snapshot yet |
| Input ID, local echo, queued submission | TUI `startSubmission`, `submissionQueue.enqueue` / `.run` | UUID and JSON payload in frontend memory; echo correlated by ID | Enqueue is not durable acceptance; no persistent client retry outbox |
| Ingress-accepted identity and payload | `Host.Submit`, `Journal.Submit` / `.append` | `D/ingress/S/journal.jsonl`; complete supported records recover identities and order | Success follows record write and file sync; failure may be uncertain |
| Pending versus acknowledged ingress | `Journal.Pending`, `.Acknowledge` | Submit/acknowledge records in the same journal; acknowledged tombstones retained | An acknowledge means session persistence was observed, not that a model answered |
| Live-worker delivery attempt | `Host.dispatch`, `process.sent` | Host memory, scoped to worker generation; reconstructed by pending-ingress reconciliation | Marked sent before pipe write; a failed write is ambiguous and not blindly retried live |
| Inbox FIFO and seen-ID set | Worker `serve`, `Inbox.Submit` / `.run` | Worker memory; resume seeds seen IDs from session history | Inbox admission is volatile and is not a persistence acknowledgment |
| Persisted external input | Coordinator `handleInboxInput`, `storeItemInSessionStore`; `Store.AppendInput` | `D/S.session.jsonl`, typed `ItemInput` | Successful store append precedes notification; speculative context updates are not authority |
| Observed session persistence | Store observer, worker item emitter, `process.consume`, TUI `applyInput` | Derived from contiguous committed item events; reconstructible by history replay | Host syncs journal acknowledgment before forwarding a new item to UI; pipe loss can delay observation |
| Prepared model context | Coordinator and context builder | Volatile working state reconstructed from supported history | No I/O in the builder; context preparation is not provider delivery |
| Model-request intent / turn | Coordinator `buildModelRequest`, `requestModelResponse`, session `ItemTurn` | Turn persisted before calling the adapter | A turn is local intent, not a durable provider receipt |
| In-flight provider execution | Joined client, provider adapter, external provider | Runtime/provider state; loopback request observed directly only in this test | No blanket persisted delivery acknowledgment or provider idempotency guarantee |
| Completed response | Coordinator `processModelResponse`, `handleModelResponse`; `AppendModelResponse` | Session `ItemModelResponse`; includes committed response output | Incomplete/unrecorded output is not a completed response; committing it closes this test's turn |
| Internal input-delivery accounting | Coordinator `currentTurnInputs`, `deliveredInputs` | Reconstructed when loading turns/responses from session history | Response application advances delivered input accounting; not a separate external receipt |
| Session/host ownership | Host parent lease and worker session lease | Kernel-held advisory leases; current process/generation authority | A timeout/EOF does not prove exit, cleanup, or permission to create a second writer |

### Source entry points

- [TUI submission and reconciliation](../../cmd/unreal-agent-tui/submissions.go),
  [submission sender](../../cmd/unreal-agent-tui/submission_queue.go),
  [frontend lifetime](../../cmd/unreal-agent-tui/run.go).
- [Host admission, dispatch, and item consumption](../../cmd/internal/workerhost/host.go).
- [Journal identity and sync behavior](../../harness/ingress/journal.go) and
  [storage setup](../../harness/ingress/storage_unix.go).
- [Worker startup, history transfer, observer, and inbox](../../cmd/internal/workerruntime/run.go).
- [Inbox ID deduplication](../../harness/inbox/local.go).
- [Coordinator request, persistence, and replay accounting](../../harness/coordinator/loop.go).
- [Typed store contract](../../harness/sessionstore/sessionstore.go),
  [local file appends and observer ordering](../../harness/sessionstore/localfile/store.go),
  [history reconstruction](../../harness/sessionstore/localfile/state.go).

## Identity and acknowledgment contract

- Host submission marshals/unmarshals the input before journal admission.
  Insignificant raw-JSON whitespace is normalized for the wire path; this is not
  general semantic JSON canonicalization (for example, arbitrary object-key
  reorderings are not promised equivalent).
- Direct journal identity is the same ID, kind, and **byte-exact payload**. Changed
  kind/payload conflicts. Identical retries are idempotent even after reopening
  the journal, including identities already acknowledged. No journal compaction
  currently removes these tombstones.
- Inbox deduplication compares IDs only. Direct `Store.AppendInput` does not add
  external-input identity/conflict enforcement. The end-to-end guarantee requires
  the host/ingress/inbox path; direct store callers must not infer that guarantee.
- Raw-JSON normalization can reuse the caller's payload backing buffer. A direct
  caller should supply a fresh/cloned payload rather than assume a whitespace-
  bearing raw slice is unchanged and reusable after `Host.Submit`. The scenario
  supplies fresh payloads, as a wire retry does; fixing caller-buffer ownership is
  a separately scoped follow-up, not an acknowledgment-contract change here.
- `Journal.Acknowledge` is driven by a persisted input event. If the host misses
  that event, startup history replay acknowledges matching journal identities
  before dispatch resumes. There is no atomic transaction joining the two logs
  and the provider; replay is the reconciliation mechanism.
- A committed completed response reconstructs delivery accounting on resume.
  An input/turn **without** a committed response can lead to another model call
  on explicit restart. Provider retries can also make additional requests. A
  stable input ID is not an external provider idempotency key or a billing promise.

## Reconstructible history versus client state

Worker startup currently acquires ownership, resumes the store, emits history,
and installs an observer before coordinator writers start. Host per-worker
sequence checks reject gaps; host-wide sequence accounting suppresses duplicate
UI callbacks during worker replacement while still reconciling journal inputs.
This controlled startup handoff is not yet a general snapshot/watch attach API.

Canonical committed inputs, turns, and responses reconstruct the conversation.
Drafts, local echoes without observed persistence, pending UI sender state,
scroll, selection, and client event cursors are frontend-local and volatile.
Saved provider preferences do not constitute a persistent draft or retry outbox.
The frontend still owns the host lifecycle, and existing event queues do not
establish bounded multi-client reconnect support.

## Known gaps carried forward

- **Ambiguous errors versus UI wording:** ingress write/sync failure poisons the
  journal with `ErrUncertain` until reopen. A complete record may have reached
  storage even though the caller received an error. TUI `finishSubmission` treats
  any returned error as failed, and the label says "not persisted". That label is
  not authoritative evidence of absence. Later acknowledgment/reconnect work must
  preserve an explicit uncertain state; no UI behavior is changed in this slice.
- **Payload ownership:** host normalization can reuse raw payload bytes as noted
  above. Persistent client retry state must own immutable input values, not reuse
  a buffer whose original whitespace-bearing length no longer matches its bytes.
- **Recovery coverage:** unexpected worker death is reported, not automatically
  replaced; unresolved nonterminal operations still refuse worker resume. Shell,
  subagent, graph, and interrupted model recovery remain later slices.
- **Bounds:** input record/frame limits exist, but current frontend submission and
  event mailboxes include unbounded in-memory queues. This test is not a bounded
  memory, overload, or multi-client authorization proof.

## Durability and compatibility limits

Ingress success follows record fsync; storage setup syncs new file/directory
entries. Initial session publication syncs a temporary file, renames, and syncs
its directory; subsequent history appends sync the session file. Incomplete final
records and complete corrupt/unsupported records have different recovery rules;
complete corruption is not silently discarded to force a successful resume.

These are OS/filesystem sync contracts, conditional on the actual device and
filesystem. Clean process restart (or process-kill tests elsewhere) does not prove
power-loss durability, media-loss survival, or exactly-once external effects.

The locally available upstream comparison baseline is
`a5f3fd13032737142916523ae4344c392292f9d5` (`upstream/main`); it was not fetched anew
for this characterization. Ingress and supervised worker lifecycle are existing
local additions relative to that baseline. This slice changes only tests and
documentation, not a shared interface, storage format, or invariant.

## Verification evidence

Completion commands/results are recorded under M0.1 in [goal.md](../../goal.md).
The test above is the combined end-to-end oracle. Related existing focused tests
cover individual failure/recovery contracts:

- `harness/ingress`: `TestIdentityOrderReopenAndClones`,
  `TestStickyAmbiguousWrites`, `TestStickyUncertainAcknowledgement`.
- `harness/sessionstore/localfile`:
  `TestStoreNotifiesObserversSynchronouslyAfterItemPersistence`,
  `TestStoreDoesNotNotifyObserverWhenItemPersistenceFails`,
  `TestStoreResumeRestoresPersistedInputIDs`.
- `harness/coordinator`: `TestCoordinatorSubmissionBoundarySurvivesRecovery`.
- `cmd/internal/workerhost`: `TestWireNormalizationAndConflictingIdentity`,
  `TestDurableInputSurvivesWorkerDeathBeforePersistence`.

### Execution record

- Focused final test: `go test -race ./cmd/internal/workerhost -run
  '^TestRealWorkerSubmissionAcceptancePersistenceDeliveryAndReplay$' -count=3 -v`
  — passed all three runs.
- `make check` — passed vet in both modules and the repository formatting gate.
- `make build` — passed; both resulting binaries were atomically installed into
  `~/.local/bin` and SHA-256 compared with their build outputs. Runner `-h` and
  TUI `--version` worked. Runner `--version` is unsupported and is not used as
  installation evidence.
- `make test` — two attempts were canceled before root-suite completion. The
  captured first attempt reports 28 root packages passing, including workerhost,
  coordinator, ingress, and localfile. It did not reach the TUI invocation.
- `go -C cmd/unreal-agent-tui test -race ./... -timeout=80s` — all TUI packages
  passed separately. No TUI implementation changed, so no new UI proof is claimed.
- `go test -race ./harness/taskgraph -v -timeout=80s` — diagnostic run timed out
  during the existing exhaustive check. Its `chain-with-shared-reader` subtest
  passed after exploring 10,876 states; the next case was unfinished at timeout.
- `go test -race ./harness/taskgraph ./harness/tool/... ./internal/... -skip
  '^TestGraphExhaustiveSafety$' -timeout=80s` — passed. This excludes the expensive
  checker only from this shard; it is not a substitute for validating its cases.
- `go test -race ./harness/taskgraph -run
  '^TestGraphExhaustiveSafety$/^(nil-and-empty-claims|directory-prefix-claims)$'
  -v -timeout=105s` — passed both cases, exploring 947 and 8,223 states.

- `go test -race ./harness/taskgraph -run
  '^TestGraphExhaustiveSafety$/^independent-cap$' -v -timeout=105s` — timed out
  while still exploring states; no safety assertion failure was reported.

- `GOMAXPROCS=2 go test -race ./harness/taskgraph -run
  '^TestGraphExhaustiveSafety$/^independent-cap$' -v -timeout=105s` — also timed
  out while exploring states; reducing runtime parallelism did not clear the gate.

**Gate after the initial attempts:** the unchanged `independent-cap` exhaustive check needed a longer
run than those command-sized attempts allowed. At that point M0.1 remained
unaccepted. The other root-package shards and three exhaustive shapes had passed,
but the full `make test` had not. The authorized extended run below subsequently
cleared this gate without changing the tests.

A larger per-call timeout cannot exceed the host's base policy. The authorized
remedy below used explicit task-scoped external supervision, not an unowned
detached process. No checker state bounds, cases, implementation, or assertions
were changed.


### Authorized extended run

The user authorized completing the long validation run. To avoid restarting the
active frontend or changing its global budget, `make test` ran under the
local user-service supervisor (now completed):

- Unit: `unreal-agent-m01-validation-20261027181954.service`.
- Command: `/usr/bin/make test`, with the repository as its working directory
  and the invoking trusted `PATH`.
- Budget: `RuntimeMaxSec=20min`, `TimeoutStopSec=10s`, `KillMode=control-group`.
  This is intentional task-scoped ownership, not an unowned detached process.
- Evidence directory: `/tmp/unreal-agent-m01-validation.z4uOtd`, containing the
  command log, source baseline, test-file hash, and service identity.
- Observed terminal phase: **completed**. Full `make test` passed both race
  suites, with 41 passing and 3 no-test packages. The TaskGraph package passed in
  263.642s; total service wall duration was 4min 38.941s.

HEAD, tracked source diff, and new test-file hash matched the starting baseline.
Installed binaries still matched the previously accepted build outputs. The
service was inactive/dead with no main process, automatically collected, and its
cgroup was absent. No retained validation job remains.

The [repository-local verification record](../verification/durable-submission-m01-20261027.md)
preserves the complete output, coverage checks, and drain evidence. **M0.1 is
accepted as complete**; no checker changes or reduced coverage were involved.
