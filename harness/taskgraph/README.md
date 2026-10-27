# TaskGraph runtime safety and input policy

`graph.go` remains the pure reducer. `runtime.go` owns graph state on one actor;
`scans.go` performs filesystem work on a bounded worker (one active scan per
runtime, including cancellation/drain). Commands may still execute concurrently
up to the graph's configured concurrency.

## Input policy

Declare explicit workspace-relative read and write paths when possible. `null`
claims conservatively cover the entire workspace, `[]` covers nothing, and `.`
explicitly covers the workspace. A claimed directory includes all its descendants,
**including git-ignored files**. Only `.git` entries are excluded. Symlinks below
the trusted canonical workspace root, including symlink ancestors and internal
aliases, are unsupported; they fail validation rather than being skipped or
followed. Nonregular files are also unsupported. Whole-workspace and cross-task
read/write conflicts produce plan warnings at start and in status. Warnings do
not relax claims. Existing reducer errors for cyclic ownership remain errors.

The content hash format is unchanged. Ancestors of explicit paths establish
identity and mode, not ownership of unrelated siblings: changing a sibling must
not invalidate an explicit file claim. Directories actually included by a claim
retain strict membership and mutation checking. Read-only input hashes exclude
declared writable subtrees.

## Scans, cancellation and freshness

Admission reserves a task with the existing `running` reducer status **before**
scanning. A checkpointed `input_validation` attempt with `command_dispatched:false`
proves the command has not been dispatched; only this explicit marker permits
safe prelaunch resume. Older Ready/preparation child checkpoints remain unknown
launches and retain their reservations, as before. Dispatch changes the phase and
persists the conservative boundary before adding the child to its manager.
`command_dispatched:true` therefore means dispatched/may have started, not proof
that a shell instruction has run. Legacy child checkpoints are likewise treated
conservatively as potentially dispatched.

A terminal child remains `running` until output validation drains. Cancellation
moves active reservations to `canceling`; the actor acknowledges the request
without waiting for the scan. The task and graph cannot finish cancellation or
admit a conflicting child until that worker and the relevant children drain.
For scans of completed/accepted candidates, the runtime-wide admission barrier
also prevents replacement work while an observation drains, even if a reducer
control has reset/released the candidate. Revision of an active reservation is
still rejected by the reducer.

Workers consume immutable snapshots and return observations, never reducer
mutations. The actor checks runtime cancellation, worker identity, graph epoch,
generation, staleness and current status before applying any result. A successful result
already buffered when runtime cancellation occurs is drained without being
applied; shutdown cannot dispatch, accept, admit work, or start another scan.
Mutating controls invalidate the epoch and cancel in-flight observations; stale results cannot dispatch commands
or accept candidates. Accepted inputs are rechecked before admission, output
completion and acceptance; failure invalidates their descendant closure through
the reducer. Acceptance requests are asynchronous and serialized, with a bounded
queue of 64 waiting requests. Canceling an acceptance operation does not cancel
or accept its task candidate.

**Status is a pure snapshot.** It does not scan, recheck freshness, invalidate,
or advance scheduling. It can describe an externally changed candidate until
an actual admission/completion/acceptance check observes that change. No polling
freshness scans are added.

`Runtime.Wait` joins the request mailbox, scan workers, all nested managers and
lifecycle-aware child handlers. Shutdown stops **all** nested graph contexts
before joining any manager, handler, or scan, so a blocked scan cannot keep a
sibling execution alive. Cancellation is checked between filesystem steps
and bounded file reads. A kernel filesystem call (or a deliberately uncooperative
handler/test gate) cannot itself be forcibly interrupted: status stays responsive
while scans run, but ownership and `Wait` persist until actual drain. Filesystem
hashes are observations, not locks against external workspace writers; mutation
after the last observation is the same unavoidable external TOCTOU boundary as
before. Declared ownership does not sandbox an arbitrary command's writes.
Synchronous manager/handler calls are not moved to this worker; this change
specifically removes filesystem fingerprinting from the runtime actor.

## Diagnostics and compatibility

Version-1 checkpoints retain their existing graph, child and fingerprint fields;
new attempt fields are additive. Old checkpoints without them remain readable
and retain conservative recovery behavior. Inconsistent explicit prelaunch
markers are rejected. Pending acceptance controls were not persisted previously
and still must be resubmitted after a crash; they cannot silently accept on resume.

Attempts expose `phase` (`input_validation`, `execution`, `output_validation`,
`awaiting_acceptance`), `command_dispatched`, `failure_category`, `failure_reason`,
shell `exit_code` when known, and start/phase/finish timestamps. Failure reasons
include filesystem paths when available. Prelaunch unsupported symlinks are
input-validation failures with no dispatch; shell nonzero is an execution
failure; read-only mutation is `input_mutation`; changed acceptance inputs or
outputs are `result_mutation`. Status returns attempts, plan warnings, blockers
and a scan-in-progress flag. When scanning, status also exposes a `scan` object:
`graph`, `task_id`, `generation`, `kind` (`input_validation`,
`output_validation`, `accept`, or final `recheck`), current `phase` (including
`accepted_recheck` while checking accepted inputs), `started_at`, `elapsed_ms`,
`phase_started_at`, and `phase_elapsed_ms`. Worker phase changes publish immutable
atomic progress snapshots, not changes to actor-owned attempts. These timings
measure filesystem observation, not command execution. Inbox notifications include
phase, dispatch and failure information. Historical attempts remain tagged with
their generation.

## Verification

Focused deterministic worker-gate regressions live in `scans_test.go` (no sleeps
are required to put the actor behind a slow scan). They cover responsiveness,
cancel acknowledgment/ownership/drain, Wait joins, prelaunch resume, diagnostics,
status purity, scan phase/timing visibility, superseded acceptance, successful
queued results discarded on shutdown, child-completion/advance suppression, and
stopping sibling contexts before a blocked scan join. Checkpoint tests reject
noninitial Ready shell/agent state behind an explicit prelaunch marker and retain
conservative recovery for genuinely old v1 data without diagnostic fields.
`fingerprint_test.go` covers cancellation and explicit-claim ancestor metadata. The existing exhaustive reducer checker,
lifecycle, checkpoint and remote-artifact tests remain applicable:

```
go test -race ./harness/taskgraph -run 'TestRuntime|TestFingerprint|TestCheckpoint|TestPlanWarnings' -count=10
go test -race ./harness/taskgraph -count=1
go test ./harness/taskgraph -run TestGraphExhaustiveSafety -count=1 -v
UNREAL_TASKGRAPH_EVALUATION=1 go test ./harness/taskgraph -run TestSchedulingEvaluation -count=1 -v
```

The reducer model checker does not model runtime workers or filesystem races;
its bounded exhaustive safety result complements, rather than replaces, these
runtime regressions. `graph.go` and its public reducer event set are unchanged.
