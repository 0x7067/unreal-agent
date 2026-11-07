# Operation checkpoints — executable product and inductive safety

This directory now contains a **handwritten executable port of the action
boundary of `../OperationCheckpoint.tla`**, plus the original smaller proof
slices. It proves the fixed Bend product's at-most-once shell spawn and
**both shell and remote terminal checkpoint uniqueness over arbitrary finite
traces**, with an unbounded crash counter and a shared FIFO. This is not a
machine-checked translation/equivalence theorem from TLA+, or a Go refinement
proof. The original TLA/config/evidence files remain intact.

## Check and reproduce

From repository root, with Bend **2.0.27**:

```sh
~/.bend/bin/bend docs/formal/operations/bend/PROOF.bend --check-only
~/.bend/bin/bend docs/formal/operations/bend/PROOF.bend
# Both print: All terms check.

docs/formal/operations/bend/check.sh
# Check deterministic certificate freshness, positive proof, and rejection of
# each legacy toggle when substituted into the fixed-product closure proof.
# Override the compiler with BEND=/absolute/path/to/bend.

python3 docs/formal/operations/bend/generate_certificate.py
# Regenerate certificate.bend / closure.bend after an intentional model change;
# generated output still has to pass the Bend proof checker.
```

Public laws remain in `LAWS.bend`; `PROOF.bend` imports it as `Laws` and fills
all paired declarations. Every `.bend` source here is reachable from that proof
entry point. No unsafe recursion, foreign proof bodies, axioms or proof holes
are used. Python and the generator are **not** proof assumptions: Bend checks
all concrete successor equalities against `product.bend`.

## Product and action correspondence

`product.bend` has independent fields corresponding to all TLA variables:

* `State.crashes : Nat` and `Core.up`;
* volatile shell `phase` / primitive `active`;
* durable filesystem `errFile`;
* independent manager `rmap`, `rterm`, `runknown`, `hopen`;
* one unrestricted `List<Update>` FIFO shared by both operations;
* independent durable shell and remote `Checkpoint{term, phase}` records;
* independent Nat history counters `spawns`, `shellWrites`, `remoteWrites`.

The record constructors do **not** identify queued, stored and volatile
terminal flags. They admit inconsistent states and arbitrarily large queues
and counters. In particular, `Persist` unconditionally saves the FIFO head
and increments its operation's counter if terminal: it has **no terminal
checkpoint deduplication guard**. Safety must follow from the transition
invariant, not from counter saturation or making duplicate writes unrepresentable.

| TLA action | Bend boundary / behavior |
| --- | --- |
| `CreateErrDone` | Same up/phase/active guards; exclusive existing-file failure emits terminal, otherwise appends process progress and increments spawn count |
| `ProcessExit` | Requires up and running primitive; appends shell terminal |
| `CancelFails` | Requires up, retained map entry and not terminal; emits remote terminal, retains unknown outcome |
| `RemoteFinishes` | Requires up, retained entry and open handler; emits only if not already terminal, then releases entry |
| `HandlerCloses` | Requires up/open handler; closes it, conditionally emits remote terminal according to `closeSkipsTerm` |
| `Persist` | Requires up/nonempty shared FIFO; saves and removes exactly its head |
| `Crash` | Requires up; drops FIFO and all manager fields, preserves store/file/histories, increments crash count |
| `Resume` | Requires down; reopens handler, readmits only stored nonterminal operations; stored process shell becomes queued terminal instead of respawning |
| Stuttering | Explicit `Stutter`; disabled boundaries also stutter |

Both original mutation toggles are executable `Config` fields. The one
intentional parameter change is **unbounded crashes**: the `crashes < MaxCrashes`
guard is omitted. `crashes` is still a real, incrementing, independent Nat
field, not finite trace fuel. The proof accepts every initial crash count;
the TLA initial value is zero. Any finite `MaxCrashes` schedule fits this
unbounded-crash boundary, but no formal TLA-to-Bend simulation is claimed.
Constructor types encode typed phases/operations/checkpoints; there is no
separate theorem about the TLA model's untyped universe or `TypeOK` expression.

## Why this is an inductive proof, not bounded trace testing

The executable core has **127 fixed-configuration reachable records** after
projecting away only the independent crash counter. `certificate.bend`
enumerates concrete records, and defines the relational invariant

```text
Invariant(s) = exists i : Index, decode(i) = s
```

This is a closed finite *cover*, not a new lifecycle datatype used by the
reducer. `closure.bend` checks all **1,143 event/state successor equalities**
(127 records times nine boundaries), and checks safety/accounting on every
record. The proof does not assume the generator found all reachable states:
initial membership plus checked closure suffices, even if the cover contains
extra states. Missing a needed successor would fail a concrete equality.

`PROOF.bend` then proves:

1. `product_initial_invariant`: initialization belongs to the cover.
2. `product_invariant_preserved`: **every** boundary preserves it for any core
   satisfying it, by transporting the concrete closure proof through equality.
3. `product_invariant_implies_safety`: all three actual Nat counters are at
   most one. Separate laws expose `product_at_most_once_spawn` and
   `product_single_terminal_checkpoint`; the latter quantifies over **both**
   operation constructors.
4. Structural induction on an **arbitrary-length event list**, carrying the
   invariant through each step. `product_every_prefix_safe` covers every
   prefix, while `product_arbitrary_crash_trace_safe` and the separate count
   laws cover the full `State` executor including unbounded crash history.

There is no bound on trace length or number of crashes. The FIFO is not
capacity-limited: length at most three is a **proved reachable consequence**,
not a guard. `product_invariant_queue_accounting` proves for each operation

```text
terminalWrites(op) + countTerminalEntries(op, sharedFIFO) <= 1.
```

`product_invariant_durable_history` also checks that each stored terminal bit
agrees with its committed terminal count being one. This excludes the error
where a stale nonterminal checkpoint overwrites an already durable terminal.
`product_trace_queue_accounting` transports accounting through arbitrary full
executions. Accounting alone is **not** asserted sufficient for preservation:
the stronger finite-cover invariant supplies phase, store, manager and FIFO
ordering relationships. This approach checks genuine transition equations and
write/spawn counts, rather than an audit function that labels each action safe.

## Actual legacy failing executions

The following are executions of `product.bend`, not the old projected Boolean
attempt stream or two-event queue counter. `witnesses.bend` replays all states;
`legacy_product_spawn_execution` and `legacy_product_close_execution` check
**the entire lists of independent-field states**, including crash counts.
Additional laws compute the violating count as **2** and safety as **False**.
All listed boundaries are enabled and change state.

### `ExclusiveErrCreate = false`, `CloseSkipsTerm = true`

`S/F/process` abbreviates a nonterminal shell process checkpoint. Stored shell
remains `F/create_err`; both terminal-write counts are zero throughout.
Remote store remains `F/none`; its local fields are initially
`(rmap,rterm,runknown,hopen)=(T,F,F,T)`, reset to `(F,F,F,F)` on crash and
restored to `(T,F,F,T)` on resume.

| State after | up / crashes | phase / active | errFile | shared FIFO | spawns |
| --- | --- | --- | --- | --- | --- |
| Init | T / 0 | create_err / create | F | `[]` | 0 |
| CreateErrDone | T / 0 | process / process | T | `[S/F/process]` | 1 |
| Crash | F / 1 | none / none | T | `[]` | 1 |
| Resume | T / 1 | create_err / create | T | `[]` | 1 |
| CreateErrDone | T / 1 | process / process | T | `[S/F/process]` | **2** |

The lagging durable phase permits a retry; reusable err-file creation permits
the second launch. With both fixes enabled the same retry instead emits a
shell terminal unknown-outcome checkpoint and does not increment spawns.

### `ExclusiveErrCreate = true`, `CloseSkipsTerm = false`

`R/T/none` is a remote terminal checkpoint. The shell fields/store remain at
Init, spawns and shell writes remain zero, runtime stays up with crashes zero,
and `rmap` remains true throughout.

| State after | rterm / runknown / hopen | shared FIFO | remote store | remote writes |
| --- | --- | --- | --- | --- |
| Init | F / F / T | `[]` | F/none | 0 |
| CancelFails | T / T / T | `[R/T/none]` | F/none | 0 |
| HandlerCloses | T / T / F | `[R/T/none, R/T/none]` | F/none | 0 |
| Persist | T / T / F | `[R/T/none]` | T/none | 1 |
| Persist | T / T / F | `[]` | T/none | **2** |

Legacy close duplicates a queued terminal; unconditional persistence writes
both. With both fixes enabled the same schedule produces one durable write.
The local `check.sh` additionally substitutes each legacy toggle into `fixed`
in a temporary copy and requires failure at the corresponding closure lemma,
not a parser error. The legacy TLA configs/check script remain supplemental.

Other checked witnesses exercise all three shared-FIFO positions in an
interleaving, stored-process shell rejection on resume, and a remote terminal
lost on crash then regenerated and committed once.

## Original slices preserved

`model.bend` and **all original laws** remain unchanged in meaning:
`shellSpawns` proves exclusive-file launch bounds for arbitrary attempt lists;
`terminalWrites` on the old `Remote` datatype proves the remote projection's
zero-future-write and at-most-one-write facts by trace induction;
`cancelThenCloseWrites` retains its legacy/fixed bounded queue witnesses.
Those projections are not used as axioms for the product proof. The full
product and old slices are not claimed machine-equivalent.

## Exact remaining gaps and assumptions

* No checked translation/equivalence theorem against `OperationCheckpoint.tla`,
  no Go refinement or executable-core extraction, no model/implementation
  behavioral equivalence. The action correspondence above is manually reviewed.
* The port inherits the TLA scope: **one shell and one remote operation**, the
  shell initially at `create_err`, and one remote handler. It does not generalize
  safety to arbitrary operation sets/IDs, multiple handlers or concurrent
  managers. It has no nonterminal remote updates, separate pending/channel
  buffers, journal failures or partial store writes.
* Create completion, FIFO append and launch remain one atomic action, just as
  in the TLA model; it does not expose actual filesystem/process/network/actor
  callback microsteps. The durable file fuse assumes persistence and no external
  deletion/replacement. External processes can outlive crashes, but their real
  effects and operating-system exclusive-create guarantees are not proved.
* Crash loses all modeled volatile state atomically and preserves store/file;
  resume uses the modeled durable checkpoint. No storage corruption, failed
  persistence, admission validation or unsupported-version behavior is modeled.
* Only safety is proved. There is no fairness, progress, eventual completion,
  exactly-once successful execution, liveness, or coinductive trace-equivalence
  theorem. Arbitrary finite-prefix safety is the stated invariant result.
* The finite certificate is small for this product, but needs regeneration and
  checked closure when the boundary changes. Its Python generator is not a
  verified TLA translator. The Bend compiler/Base/runtime remain trusted as
  documented by the project proof policy.

The former gaps of a missing shared-FIFO product, missing shell terminal
uniqueness, and missing full-boundary legacy execution witnesses are addressed
here; the implementation and environmental gaps above remain open.
