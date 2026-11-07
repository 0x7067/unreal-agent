# Handoff / inbox: checked Bend models

These are **handwritten abstract protocol models, not a Go refinement proof**.
The original ten public laws and their three implementation slices are retained
unchanged. Added models close global persistence uniqueness and FIFO **within a
composed abstraction**, and add reload metadata/history properties and executed
negative mutants. Neither the unbounded Bend development nor its finite
witnesses establishes equivalence to the richer retained Lean/TLA systems.

## Check and inventory

From the repository root:

```sh
~/.bend/bin/bend version
BEND_NO_TELEMETRY=1 ~/.bend/bin/bend docs/formal/handoff/bend/PROOF.bend --check-only
python3 tools/formal/prove.py --check-structure
```

Bend reports **`bend 2.0.27`**. The component proof check exits **0** with
**`All terms check.`** There are **43 public laws**, each with its paired
`def Laws.<name>` in `PROOF.bend`: the original **10**, plus **33** additions.
The public set contains **19 all-trace laws**, **4 other universally quantified
laws** (arbitrary state/step or symbolic fresh admission), and **20 witness
laws**. Eight witness laws compute failures across six distinct mutants; the
remaining twelve are positive concrete or symbolic executions. Different laws
can inspect the same execution: these counts are not distinct test schedules.
All 12 local `.bend` sources are import-reachable from the paired proof entry.

There are no unsafe annotations, foreign definitions, invented axioms,
postulates, or unfinished proofs. Only local modules and bundled `Base` are
imported. This compiler does not support `--verdict`; the evidence above is not
an independent BendTT-kernel verdict. The repository gate separately audits
imports/trust bypasses and checks every component plus root `PROOF.bend`.

## What “all traces” means

`run` folds a finite list of commands; disabled commands stutter. Preservation
lemmas accept **arbitrary invariant-satisfying states**, then structural
induction on the event list proves the invariant for every finite execution.
Input and generation identities are natural numbers, not machine integers;
there is no input-count, generation-count, queue-size, or trace-length bound.
Thus every finite prefix of an infinite execution is safe. No fairness,
eventual delivery, successful restart, or filesystem durability is proved.

## Retained original ten laws: separate slices

These sources and original laws have not been weakened or replaced:

* **`inbox.bend`**: arbitrary-ID inbox, reverse first-admission history, actual
  FIFO, transferred freeze FIFO, and delivered sequence. `inbox_all_traces`
  proves `reverse(seen) = delivered ++ handed ++ queue`, unique first-admission
  history, and the open/frozen phase constraints. `inbox_fifo_conservation`
  publishes the sequence equality. `inbox_dedup_survives_thaw` is a symbolic
  duplicate execution. Freeze transfers the FIFO; `ProcessHanded` drains it;
  Thaw requires the transferred queue empty and preserves dedup history.
* **`generation.bend`**: independent live-process list/current pointer,
  reload phases, retirement/failure latches, and published ready/stopped status.
  `I1_all_traces` proves the list is exactly the current owner's singleton or
  empty; `stopped_publication_all_traces` proves no owner implies stopped.
  `replacement_reachable` and `abandoned_retirement_publishes_stopped` are
  computed positive witnesses. A retired worker remains live until `Exit`.
  Launch requires no live process, no owner, proven retirement, no failure,
  and the appropriate reload phase.
* **`journal.bend`**: independent accepted/persisted/acknowledged reverse
  histories. `I2_ack_implies_persisted`, `I2_no_loss`, and
  `I2_journal_monotone` prove acknowledgement/persistence accounting and
  monotone acceptance. This projection intentionally permits arbitrary
  `Persist{id}` and duplicate histories. **It does not claim I3 or I4.**

The composed results below do not silently strengthen these separate slices.
In particular, the inbox laws still do not extract a standalone Boolean
`unique(delivered)` theorem. The new uniqueness law concerns the composed
**session-store append history**.

## Composed host / inbox / journal / store: `composed.bend`

### Representation and transition boundary

Admissions allocate unbounded journal positions `0, 1, ... accepted-1`, in first
admission order. These are ordering keys, not application payload IDs. The TLA
model uses ascending keys `1,2` instead. The host has an independent `G.State`;
store, journal, inbox, and dispatch fields participate in the **same execution**:

| Field | Meaning |
| --- | --- |
| `accepted` | durable journal's admitted prefix; Submit extends it |
| `acked` | acknowledged prefix; pending positions are `acked <= i < accepted` |
| `sent` | current generation's mark-before-write cursor |
| `head`, `seen` | interval FIFO `[head, seen)` and dedup prefix `[0, seen)` |
| `store` | independent reverse history of successful session appends |
| `broken` | failed input-frame write latch; no further writes in this generation |
| `host` | live/current owner, reload phase, retirement, failure and publication |

An interval FIFO is a compressed representation of an ascending, gap-free
sequence, **not an arbitrary payload list**. The dispatcher selects
`max(sent, acked)` and requires it to be below `accepted`. Already-seen frames
are dropped; a new frame extends the inbox. The symbolic public theorem
`composed_fresh_write_is_FIFO_tail` proves that in any invariant-satisfying,
healthy state, a genuinely fresh selected frame is **exactly `seen`**, so the
interval extension invents no missing frames. Persist consumes the head and
prepends that actual position to the independent store history; the store is
not computed from the counter.

Host write and ordered pipe receipt/inbox admission are **one atomic action**.
No separate pipe backlog, blocked decoder, or frozen-submission object is
represented. Successful writes cannot overtake one another. Reload phases
exclude writes behind the delivery fence; retired/no-owner states cannot
persist. Submit remains possible while no worker exists.

`Ack` advances only when `acked < head`, i.e. after persistence, and can lag by
arbitrarily many steps. Acknowledgements form a prefix here; Go/TLA permit an
arbitrary persisted subset. `Quiesce` refuses and aborts the attempt when a sent
position remains unacknowledged, or when the input pipe is broken. Replacement
requires the lifecycle launch checks, resets the per-generation cursor, and
initializes **both the inbox head and dedup prefix from `length(store)`**. Thus
an already-persisted, still-pending replay cannot cause another store append.

### Explicit failure semantics

* `Write{Delivered}` marks the position and admits/dedups its frame.
* `Write{FailedLost}` **still marks** the attempted position, sets `broken`,
  and does not admit the frame.
* `Write{FailedDelivered}` marks it and sets `broken`, but allows admission
  despite the reported error. The inbox can still persist that admitted frame.
* No subsequent input-frame write is retried on the broken pipe. Crash clears
  ownership, publishes stopped, and latches lifecycle failure; it cannot enable
  automatic replacement. Unacknowledged inputs remain durably pending, which
  is safety/no-loss accounting, **not eventual service**.
* `Persist` represents an atomic successful store append. `PersistFail`
  leaves the store and FIFO untouched, and does not acknowledge anything.
  An ambiguous partially durable filesystem write is **outside this model**;
  no exactly-once theorem about arbitrary filesystem failures is claimed.

### Inductive invariant and extracted theorems

`composed-proof.bend` proves preservation of:

```
acked <= head <= seen <= accepted
not broken => sent <= seen
store = [head-1, ..., 1, 0]
```

The last equality is about the **independently recorded append history**.
`arithmetic.bend` proves the natural-number bounds, membership of canonical
prefixes, uniqueness and strict ordering needed to extract:

* **`composed_invariant_all_traces`**: the full bundle, all finite executions.
* **`composed_store_exact_admission_prefix`**: store is exactly the completed
  admission prefix, not merely an ordered subsequence.
* **`I3_composed_persist_at_most_once`**: `unique(store) = True`. This is genuine
  global uniqueness across arbitrary repeated replacements, not per-inbox
  uniqueness or a predicate that merely permits a future append.
* **`I4_composed_FIFO_across_replacement`**: the reverse append history is
  strictly descending, hence chronological persistence is strictly ascending
  in admission order, across replacements.
* **`I2_composed_no_loss`**: every identity is either unacknowledged or in the
  actual store. For accepted positions this is exactly pending-or-persisted.
  `composed_journal_admission_monotone` separately proves that every transition
  preserves the accepted prefix, including crash, abort, failure and launch.
* **`composed_ownership_all_traces`** and
  **`composed_stopped_publication_all_traces`** lift ownership and corrected
  publication to these **same** host/inbox/journal/store executions. No owner
  implies stopped, so I10 follows even when Submit occurs without a worker.
* **`composed_store_failure_stutters`** proves failed appends leave all state
  unchanged, for arbitrary states, not just a concrete failed-write example.

`mutants.bend` also executes a **seeded NoGuard** model and proves its composed
invariant for all traces. This is a stronger replay stress test of the seed
within this abstraction, **not a recommendation to remove the Go busy guard**.
The ordinary guarded, prefix-ack schedule cannot retire a generation while its
already-persisted sent input is still unacknowledged; that is why the explicit
seed replay witness uses NoGuard.

## Reload metadata/history: `reload.bend`

This is a **separate proved lifecycle slice**, not extra fields silently
assumed to be synchronized with `C.State`. It adds independently stored
inflight count, unused candidate presence, compatibility result, and a
cumulative audit of the actual old live-process list at replacement launch.
`reload-proof.bend` proves these fields agree with the lifecycle phases by
induction; they are not defined as projections of those phases at runtime.

* `reload_metadata_all_traces` proves the full metadata/history invariant.
* **I5** `I5_actual_launch_history_all_traces`: every actual replacement launch
  observed an empty old live-process list. Retirement does not remove the live
  process; Exit does. The cumulative audit remains true for the whole trace.
* **I6** `I6_probed_candidate_all_traces`: admitted retirement/waiting for exit
  has an existing compatible candidate. Probe failure cleans up and returns
  without retiring the running worker.
* **I7** `I7_single_flight_all_traces`: independent inflight count is at most
  one, including arbitrary interleavings of repeated Begin requests.
* **I8** `I8_unused_artifact_cleanup_all_traces`: no unused staged candidate
  remains when the attempt is idle. Launch transfers candidate ownership and
  clears unused-candidate metadata; this is **not a filesystem deletion proof**.

Staging/probing and bad-candidate cleanup are atomic here. There are no artifact
bytes, immutable file paths, file-copy failures, identity serialization, or OS
unlink semantics. These properties port the logical I5–I8 obligations, not
those implementation details.

## Executed witnesses and counterfactuals

All witness laws reduce actual `run`/`mutantRun` computations in Bend. They do
not assume a supplied bad state or supplied failing predicate.

| Execution | Checked observation |
| --- | --- |
| Correct guarded reload then second-generation delivery | Exact composed state, store `[1,0]`, current generation 1 |
| Persisted unacknowledged replay, seed retained, busy guard bypassed | Store remains `[0]`; replay is dropped |
| **NoSeedNoGuard** | Actual store `[0,0]`; uniqueness false; two-input replay also makes FIFO false |
| FailedLost followed by attempted retry/persist | Sent mark retained, broken true, store empty |
| FailedDelivered followed by attempted retry/persist | Sent mark retained, broken true, actual store `[0]` |
| Unacknowledged sent input at reload | Guard aborts; original worker stays live and not retired |
| Failed-pipe crash then attempted reload | Failure latched, stopped, no replacement |
| Abandoned retirement then Submit and explicit reload recovery | Replacement persists the pending position |
| **NoCAS** | Two Begin commands produce inflight count 2 |
| **NoDiscard** | Aborting a staged attempt leaves an unused candidate while idle |
| **NoProbe** | Worker retires with compatible=false |
| **EarlyLaunch** | Actual launch audit sees old process still live and becomes false |
| **OldStatus** | Abandoned exit leaves no owner but stale ready publication |
| Corrected status, same abandonment schedule | Stopped publication; also covered by composed all-trace theorem |

The NoSeedNoGuard counterexample schedules keep `acked=0`, so replay frames are
a contiguous interval and the mutant computation faithfully represents those
frames. No general refinement theorem for arbitrary malformed/unseeded interval
states or arbitrary mutant schedules is asserted.

## Property mapping and remaining differences

| Retained obligation | Checked Bend scope |
| --- | --- |
| Lean I1; TLA I1 | Original ownership theorem and same-execution composed ownership theorem |
| Lean I2 ack/persistence and monotone journal | Original laws preserved; composed accounting and admission monotonicity added |
| TLA I2 | Composed accepted input is pending or in actual store, all traces |
| TLA I3 | Actual composed store uniqueness, all traces, restored seeding |
| TLA I4 | Actual composed store admission-prefix/order, all traces across replacement |
| TLA I5 | Separate reload slice's cumulative actual-launch empty-live audit |
| TLA I6 | Separate reload slice's compatible candidate at admitted retirement |
| TLA I7 | Separate reload slice's independent inflight counter, all traces |
| TLA I8 | Separate reload slice's unused-candidate metadata cleanup, all traces |
| Corrected TLA I9/I10 | Composed no-owner-implies-stopped; Submit allowed while stopped |
| InboxFence FIFO/NoLoss | Original exact ordered inbox accounting remains separate |
| Negative/reachability checks | Six executable negative mutants, seeded replay, successful replacement delivery and abandoned recovery witnesses; not the entire TLC configuration sweep |

Important non-equivalences to `Handoff.tla` and shipped Go remain:

1. **Composition is not the entire TLA machine.** Numeric admission positions,
   prefix acknowledgements, interval FIFO, merged write/receipt, and unbounded
   generation IDs differ from TLA's two-input/two-generation arbitrary-set
   acknowledgements and separate Write/Admit/Persist actions. No bisimulation,
   extraction, or Go correspondence theorem is provided.
2. **Drain/freeze boundaries differ.** The composed host persistence guard and
   atomic admission collapse normal quiesce/freeze/retirement into one lifecycle
   action. There is no separately modeled nonempty quiesce drain. The original
   inbox slice proves freeze transfer and drain conservation, but that result
   has **not** been composed with the store theorem. Go's actual freeze,
   `processInputs`, store appends and retirement boundaries are richer.
3. **Status boundaries are atomic.** Exit represents ordered final-frame drain,
   wait/reaping, stopped publication, owner clearing and completion barrier in
   one step. In Go the stopped callback precedes clearing current/closing
   `p.done`; blocking callbacks and their interleavings are not modeled. No
   `Done` channel is present. Clean retirement leaves failure clear and permits
   durable Submit and later explicit recovery; crash latches failure.
4. **Cancellation is selected, not exhaustive.** Abort before retirement and
   abandonment after admitted retirement are represented; late acceptance after
   a timed-out control request, partial frame decoding, failed shutdown,
   readiness/hello delays, close joins and competing dispatch-lock acquisition
   are not distinct transitions.
5. **Metadata slice is separate.** I5–I8's richer independent metadata/history
   has not been incorporated into the same composed data-plane state. The
   data-plane lifecycle represents successful probing and single-flight phases,
   but it does not carry the independent artifact/counter fields.
6. **Environmental effects remain unproved.** Journal/session filesystem crash
   durability, ambiguous partial store writes, process enumeration, networking,
   frontend rendering and artifact immutability/deletion remain outside these
   handwritten models. The richer Lean slot/artifact model and retained TLA
   configurations are still supplemental evidence, not retired equivalents.

Accordingly this development closes **model-level** global I2/I3/I4 for the
stated composition and adds model-level I5–I8 slices. It does not claim all-ten
closure for one full TLA/Go state machine or end-to-end exactly-once delivery.
