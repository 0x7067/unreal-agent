# Command-only durable graphs (M2.7)

North star: **The durable session is the agent; the process running it is replaceable.**

## Opt-in and compatibility

`unreal-agent-runner host --durable-graphs` selects `taskgraph.run/2` and
`taskgraph.control/2`; the default v1 translator/runtime remain unchanged.
Private worker protocol is 7, session format remains 2. The command layer supplies
stable parent session identity and a hash of the fresh bootstrap control scope.
The model cannot provide this scope. Translators and codecs are pure.

A v2 run contains a strict v1 envelope with parent ID and original `RunPlan`;
its optional handle is a strict v2 checkpoint wrapping the existing checkpoint1.
Workspace, shell and operation base directory are pinned. Each command child ID
binds graph operation, task and generation, and current-generation child command
must match its declaration. Initial/revised tasks are command-only; agent children
are not enabled. State is limited to 16 MiB and 128 tasks/attempts. Unknown members,
versions, invalid graphs, contradictory completion and configuration changes fail
explicitly. No migration or blanket legacy-v1 recovery is introduced.

## Classified recovery cells

| Persisted cell | Supported action |
| --- | --- |
| Initial ready/awaiting run, no checkpoint | Validate intent, reserve and save before first launch |
| Current `input_validation`, undispatched, canonical initial ready shell | Revalidate inputs and reserve a safe **first** launch |
| Accepted generation with successful terminal child | Keep generation/evidence/output fingerprint; existing freshness checks still apply |
| Completed unaccepted candidate with successful terminal child | Keep candidate and claims; dependents remain blocked; explicit fresh acceptance required |
| Running/canceling with validated terminal shell result | Finish output validation/cancellation, not command replay |
| Dispatched reservation with ready/unknown child | Refuse: storage acknowledgment does not prove whether launch happened |
| Saved PGID, live child, read-phase/output-read state or agent child | Refuse; no PID-only settlement in durable mode |
| Unfinished persisted control | Refuse; do not restore as a graph-owner checkpoint |
| Terminal control | Inert history only; fresh current-scope controls are separate operations |

The generic recovery gate performs pure syntax/policy checks; the trusted command
reconciler additionally authorizes actual parent and current execution configuration.
Mixed graph/subagent retained ownership is classified per kind. Structural InvokeTool
and pending/uncertain lifecycle fences are preserved. Unsupported owners are not
silently removed from debt.

## Store acknowledgment is not a launch receipt

The existing coordinator remains the sole production session writer. A command-layer
store decorator delegates `SaveOperation` first; only successful durable saves notify
the graph actor. After input validation the live actor marks the child dispatched
and emits an execution reservation **without** adding it to the nested manager.
Only that actor's volatile pending ID/generation map, exact child and input proof,
and current parent/scope authorize one dispatch after the matching acknowledgment.
Duplicate/stale acknowledgments cannot launch. Cancellation before acknowledgment
uses the live actor's positive knowledge that no launch occurred; a replacement actor
has no such knowledge. Thus a persisted reservation is deliberately **not** replayable.

This is distinct from the coordinator's original commit-before-root-dispatch rule:
it extends the fence to nested graph commands without direct runtime store I/O or
mandatory changes to manager/remote-handler interfaces. Storage errors do not release
the reservation. No universal exactly-once shell-effects claim is made.

## Ownership and controls

All reducer changes still occur on the runtime actor. Durable shutdown additionally
awaits actual nested manager stop after draining its updates and propagates errors.
Command-layer lifecycle reconciliation accepts only strict known graph2 operations
with exact joined handler version and successful current owner joins. It does not
convert a persisted terminal child result into proof that an unknown live process
has exited. The recovery codec requires terminal effect receipts; saved PIDs are not
control authority. Old control scopes fail; stale acceptance generations fail under
the existing reducer guard. Cancel requests do not imply confirmed process drain.

## Evidence boundaries

`dispatch_commit_test.go` tests real manager/scans with a simulated serialized save;
`workerruntime/m27_contract_test.go` tests the actual store decorator, injected save
failure, duplicate success, actual shell effects and nested joins. Codec tests are
synthetic fixtures, not process proofs.

`workerhost.TestM27DurableGraphWorkerRecovery` produces A/B/C receipts using actual
shell execution, saves accepted A and unaccepted B, then constructs a **controlled
canonical persistence boundary** for C at output validation with its real terminal
child receipt. It is not a live internal crash hook. Real worker A loads it; a stale
scope and stale generation cannot mutate candidates. The owned worker is killed and
joined during a blocked real provider request; worker B has a fresh scope, preserves
candidates and blocked D, and accepts current B/C/D explicitly. Counters assert A/B/C
never rerun and D runs once. Model replies are selected from durable candidates:
runtime notifications may interrupt HTTP replies, so response delivery is never
mistaken for committed tool dispatch. The provider is only a loopback external fake.

The unchanged reducer blob is `2315afc35788160fc81bf84657f2600bd61c9e86`.
Exact exhaustive safety ran before and after; the skill records the additional
mailbox acknowledgment event and explicitly excludes I/O/ack/process properties
from reducer-model coverage.

## Local upstream comparison

Compared with local upstream `a5f3fd13032737142916523ae4344c392292f9d5`, without
fetch. It has no `harness/taskgraph` or private host/recovery/Hub layers. The v2
codec, actor acknowledgment and trusted owner reconciliation are local divergences,
not upstream contracts. Shared translator/context purity, coordinator-owned writes,
version rejection and commit-before-dispatch are preserved. No shared mandatory
manager/store interface, pure reducer or inbox admission code was changed for M2.7.

Acceptance/build/install/private frontend evidence is recorded separately; this
contract document alone does not mark the slice accepted or the roadmap complete.
