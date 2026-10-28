# M2.3 — durable bounded regular-model requests

**Complete.** [Current-source both-module acceptance, installed byte checks,
private frontend proofs and archived evidence](../verification/durable-model-request-m23-20261028.md)
passed on 2026-10-28. M2.4–M4 remain unfinished.

## Reviewed upstream divergence and compatibility

Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` was compared without
fetching: its request path commits a turn before Respond, but has no durable
logical identity/attempt bound. Before implementation the new typed control,
two-attempt policy, billing uncertainty, complete-only tool fence and private
worker protocol change were surfaced with the user.

`inbox.ModelRequest` is a strict versioned canonical control with a bounded
`ModelRequestRecord`: logical RequestID, unique per-attempt TurnID, turn type,
charged Attempt/Limit and input/external-input cursors. Actions are started,
interrupted, exhausted, stopped and superseded. Codec v1 validates shape;
coordinator additionally enables **regular, limit 2 only**, validates causal
transitions/cursors and refuses unsupported compaction policy. New controls are
not optional turn metadata: old canonical readers reject their unknown mode.
Private worker protocol **4**, session format **2**, rejects old candidate workers
before retirement. No mandatory adapter/manager interface, translator I/O,
context-builder I/O, public attach protocol or operation schema change.

Completed legacy history remains readable. A legacy unanswered regular or
compaction turn has no supported persisted retry policy and refuses explicitly,
without provider dispatch or canonical mutation. A fork reads parent history but
does not acquire/retry the parent's request authority. Recovery of compaction is
separate M2.4 work; existing bounded controls/completed summaries still read.

## Commit and retry contract

1. Persist and validate the complete attempt charge **before** its turn and
   before Respond. Pure precommit ledger validation uses an isolated copy.
2. Charge-before-turn and turn-before-dispatch crash gaps consume the charge,
   even if no actual HTTP request happened. It is intent, not proof of billing.
3. On restart record interruption exactly once. A permitted retry retains the
   logical RequestID, uses a fresh TurnID and the next persisted attempt number.
   Restart cannot reset the bound. CacheKey remains session affinity, not a
   provider idempotency/exactly-once key. Transport retries are separate.
4. Exhaustion is a visible canonical control, not an invented provider response
   or token bill. Consume only the original request's input cursor; stay idle
   for newer accepted/new external input rather than exiting before journal
   dispatch. Fresh work gets a distinct logical ID and bound.
5. A committed response, including a recorded incomplete/failed outcome, is read
   from history rather than requested again. Only failure-free **StopComplete**
   responses may introduce executable tool calls. Complete-looking JSON in an
   incomplete/failed response is not authority; legacy statuses from those turns
   are also rejected. The pure builder strips partial calls/opaque continuation
   state, retaining appropriate assistant text and actually known token usage.
6. Explicit hard stop/foreground abort do not autoretry. A durable hard-stop
   input is authoritative even if a crash precedes its typed stopped settlement.
   Supersession preserves pending input; late output is discarded and only
   actually returned nonzero usage is retained once, marked locally interrupted.
   Missing usage is unknown, never estimated. Abort/drain/ownership proofs remain
   separate; a stopped request control does not prove remote billing canceled.

**Duplicate billing is possible.** A provider may have accepted/billed a request
whose result never committed, and may bill its retry. There is no exactly-once
provider execution or billing claim, nor power-loss/storage-loss guarantee.

## Evidence and remaining acceptance

- `harness/inbox/model_request*.go`: strict typed record, unknown members/version,
  identity/action/type/cursor/bound tests; exact inbox safety skill passed before
  and after edits. Actor unchanged.
- `harness/coordinator/request_recovery*.go`: pure causal validation, restart
  bound/identity, bare charge gap, legacy/corrupt refusal with unchanged history,
  new work after exhaustion, completed replay, incomplete tools and exact known
  late usage. Independent static review found the interrupted→hard-stop crash
  gap; fixed with started/interrupted stop markers and an exact regression.
- `cmd/internal/workerhost/model_recovery_test.go`: actual Host/runtime/provider,
  two partial-SSE process kills with exact SIGKILL/owned Wait/output/owner joins,
  charge-before-observed-HTTP ordering, stable logical IDs/new turns, no invented
  response/usage/tool status/sentinel, no third HTTP at exhaustion, new input on
  the same runtime, and actual kill after response commit with no redispatch.
  HTTP handler and host dispatcher joins plus both leases are checked. A killed
  worker's missing drain receipt is reported honestly, not manufactured.
- `harness/contextbuilder/partial_response_test.go`: all incomplete/empty/unknown
  stops strip executable/opaque state; complete outputs and refusal behavior
  retained. Synthetic intended-complete fixtures now declare StopComplete.
- TUI: canonical exhaustion/incomplete notices, no partial pending-tool cards;
  unit tests in `model_request_test.go`. Actual byte-matched private frontend proofs passed; see verification.
- Focused race process tests passed (8.490s), full coordinator nonrace passed,
  focused TUI notices passed; required full acceptance is recorded in the linked verification.

After required acceptance, continue M2.4 and every later slice. M2.3 does not
complete M2 or the 27-slice roadmap.
