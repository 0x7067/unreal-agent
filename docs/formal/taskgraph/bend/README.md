# Taskgraph checkpoint safety — Bend2 proof slice

These are hand-written executable abstractions and checked proofs, **not a
Go refinement proof**. The original `../Checkpoint.tla`, `../ChildRouting.tla`
and configs remain retained supplemental evidence; Bend is the project proof authority.

## Check

From the repository root, using Bend **2.0.27**:

```sh
~/.bend/bin/bend docs/formal/taskgraph/bend/PROOF.bend --check-only
```

Expected: `All terms check.` `LAWS.bend` deliberately declares the public
obligations; `PROOF.bend` imports it and supplies every proof. Check the proof
entrypoint, not the declarations alone. `model.bend` has no external calls,
unchecked recursion, or assumed facts.

## Preserved finite abstraction

`Checkpoint` represents exactly two tasks, A and B. All seven statuses are
retained. Each task has a stale bit and a current-child ID; the two possible IDs
are represented by `Bool`. `dependent` means B depends on A; `conflicting`
means the one-way claim conflict from the TLA specification; `capTwo=False`
means concurrency 1 and `True` means concurrency 2.

| TLA predicate / config | Bend definition and evidence |
| --- | --- |
| `LegacyValid` | `legacy`: stale implies active; accepted B requires accepted, nonstale A when dependent |
| `ClaimExclusion` | `claims`: running, canceling **and completed** reserve claims |
| `ConcurrencyBound` | `concurrency`: only running/canceling consume active capacity |
| `DependencyGating` | `dependencies`: nonstale active, completed, or accepted B requires prerequisites |
| `UniqueChildren` | `identity`: reserved or accepted nodes need distinct current-child IDs |
| `fixed.cfg` / `SafeRestore` | `restored_cross_node_safety` plus four separate extracted properties |
| Four `legacy-*.cfg` files | Four computed `(LegacyValid, named property) = (True, False)` witnesses |
| `routing-fixed.cfg` | `terminal_children_retired`, exhaustively split over both IDs |
| `routing-legacy.cfg` | `legacy_routing_loses_second_child`: two deliveries target the first duplicate ID; the second node remains running |

`restore` computes `legacy AND cross`, rather than returning a literal safety
answer. The acceptance proofs extract actual conjuncts from successful
validation. The four legacy witnesses are supplied checkpoints, not assumed
reachable reducer states, matching the intent of `Checkpoint.tla`'s `Init`.

## Quantifiers and proof strength

* **Universal over this finite two-task universe:** successful validation
  implies each cross-node property, for every represented checkpoint. These
  are symbolic implication proofs using Boolean conjunction elimination, not
  just tests of selected snapshots.
* **Inductive, unbounded history length:** `every_restore_attempt_safe` uses
  structural induction on an arbitrary finite list of supplied checkpoints.
  Every accepted attempt satisfies cross-node safety. This is a history of
  independent admission checks, **not** a simulation of arbitrary reducer
  events or a proof that an accepted graph stays safe after mutation.
* **Bounded routing proof:** all four ID assignments of `ChildRouting.tla`
  are checked. Duplicate IDs are rejected; the two unique-ID assignments
  actually execute both first-match delivery steps and complete both nodes.
  Rejection satisfies the done-implies-completed statement vacuously.
* **Concrete negative examples:** the legacy counterexamples and duplicate-ID
  routing execution are normalization proofs of particular cases, not
  universal counterexample-search procedures.

## Not preserved / not claimed

There is no arbitrary-size DAG proof, no declaration canonicalization, version
or generation validation, evidence/fingerprint checking, shell-child existence,
optional or obsolete attempt validation, or implementation equivalence. These
were already assumptions of the TLA checkpoint abstraction. Claims are a
single conflict bit, not paths/read-write sets. The child-routing proof fixes
both children terminal, current generation, successful fingerprints, and two
deliveries, exactly as the TLA scenario does. There is no liveness, fairness,
I/O, crash persistence, runtime ordering, or Go-code execution theorem.

`TypeOK` is represented by the datatypes rather than a separate theorem about
TLA's untyped variables. The Bend history does not model the TLA `restored`
marker as mutable state: it proves the corresponding acceptance implication
at each boundary. No claim is made that the Bend and TLC reachable-state sets
are identical.
