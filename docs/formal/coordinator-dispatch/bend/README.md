# Coordinator dispatch — independent two-operation transition model

**Checked Bend model theorems, not a Go refinement proof.** The original
strengthened one-operation slice and every existing law are retained unchanged.
The additional `product_*` laws establish genuine reachable-state safety for a
richer independent-field, two-operation port of `../CoordinatorDispatch.tla`.
No theorem assumes a committed bit or constructs an Add audit with `True`.

## Check and reproduce

Use Bend **2.0.27**, from the repository root:

```sh
~/.bend/bin/bend docs/formal/coordinator-dispatch/bend/PROOF.bend --check-only
BEND=~/.bend/bin/bend docs/formal/coordinator-dispatch/bend/check.sh
python3 docs/formal/coordinator-dispatch/bend/generate_certificate.py
~/.bend/bin/bend docs/formal/coordinator-dispatch/bend/PROOF.bend --check-only
```

The installed binary in this sandbox is `~/.bend/bin/bend` (its `--help`
identifies 2.0.27); there is no `bend2.0.27` executable here. Regeneration is
optional: all generated `.bend` files are committed proof inputs and are
reachable from the component imports. `LAWS.bend` directly states the public
obligations; `PROOF.bend` imports it directly, delegates finite cases to
`closure.bend`, and proves arbitrary trace induction itself. The new `product`, `certificate`, `closure`, and `witnesses` helpers also each
check independently with `--check-only`. As required by repository law/proof
separation, standalone `LAWS.bend` intentionally contains open declarations;
check it through `PROOF.bend`, which discharges all of them. The unchanged old
`model.bend` is checked through its scoped import (its unscoped `Event` name
collides with Base when checked as a standalone root).

`check.sh` first verifies generated artifact freshness without writing, then
checks the full proof, then copies the model to a temporary directory and
turns on each faulty default separately. Each real mutant must be rejected
at a `closure.edgeTick` transition equality—not merely fail parsing or importing.
The three positive mutant-execution laws in the normal proof additionally
check concrete bad runs from Init. Python replacements avoid platform-specific
`sed -i` behavior in these regression controls.

## New independent-field model

`product.bend` has an unrestricted record, **not a dispatch-safe typestate**:

* `Core` independently stores `pc`, `cursor`, `committed`, `latest`, `local`,
  `localStatus`, `added`, `badCommit`, and `badCheckpoint`.
* Each operation has its own local and durable status: Absent, Ready, Awaiting,
  or Terminal. `added` is an independent pair of flags.
* Cursor One/Two/End represents TLA cursor 1/2/3. Installation, replay and latest
  overlay each take two separate steps, with a separate completion boundary.
* Append status atomically updates commitment and both initial durable Ready
  snapshots, but dispatch is later. Local updates precede journal append and
  return. Errors can be invisible or visible and stop the invocation.
* Crash is enabled at every boundary, including partial install/replay/overlay
  and stopped invocations. It resets local state/status/added, not durable
  state or ghost violation flags. `State.crashes` is a separate unbounded Nat.
* All three TLA mutation toggles are executable independent config fields.
  They are disabled by `fixed()`; they are not assumptions baked into Add.
* Add sets `badCommit |= !committed` and
  `badCheckpoint |= terminal(local[cursor]) || latest[cursor] != local[cursor]`.
  It **does not guard on these conditions**. Representable bad states really
  produce bad Add events (`product_uncommitted_add_is_not_tautologically_safe`).

The ordinary actions of the TLA model are mutually exclusive by pc/cursor.
`Tick` executes **one** such action, never a multi-action macro:

| Bend boundary / phase | TLA action |
| --- | --- |
| Tick / Translate | Translate |
| Tick / Install One or Two | Install |
| Tick / Install End | ApplyStatus |
| Tick / AppendStatus | AppendStatus |
| Tick / StatusReturn | StatusReturn |
| Tick / Guard One or Two | Guard |
| Tick / Add One or Two | Add |
| Tick / Guard End | DispatchDone |
| Tick / AppendUpdate | AppendUpdate |
| Tick / UpdateReturn | UpdateReturn |
| Tick / Replay One or Two | ReplayOperation |
| Tick / Replay End | ReplayStatus |
| Tick / Overlay One or Two | Overlay |
| Tick / Overlay End | ScheduleRestored |
| StatusInvisible / StatusVisible | StatusError(FALSE/TRUE) |
| UpdateOne/TwoAwaiting/Terminal | LocalUpdate(op, next) |
| UpdateInvisible / UpdateVisible | UpdateError(FALSE/TRUE) |
| Crash | Crash |
| Stutter, or disabled event | `[Next]_vars` stuttering |

`boundedStep` and `boundedRun` implement `crashes < MaxCrashes` exactly for an
arbitrary supplied Nat bound. `step` and `run` additionally permit unbounded
crashes. **Both** versions have safety proofs for arbitrary finite event lists;
trace length is never bounded by a crash counter or enumeration depth.

## Inductive invariant and proof, not tautological safety

`certificate.bend` describes a **212-element finite relational invariant** on
these independent fields:

```
Invariant(s) = exists i : Index, decode(i) = s
```

The relation constrains the whole record, including partial cursor positions,
local versus durable checkpoints, status application, dispatch flags and the
history ghost flags. It is not the set of all type-correct states.

`closure.bend` checks **2,332 concrete reducer edge equalities**: 212 independent
records times the complete 11-constructor event alphabet. Every case computes
`next(fixed(), event, decode(i))` and supplies another index with a reflexive
record equality. The initial-state membership and the safety of every indexed
state are also checked. The proof therefore cannot silently omit an action,
an error branch, a crash boundary, or a transition to a previously unseen state.

For readability, `product.consistent` also expresses phase/field relationships
without referring to certificate indices. Every invariant member is proved to
satisfy it (`product_invariant_relates_partial_cursor`):

* Uncommitted durable maps are empty; committed maps have no Absent entries.
* Partial installation is a Ready prefix followed by Absent entries.
* Partial replay starts from cleared locals and replays the initial Ready
  prefix **rather than the latest snapshots**.
* Partial overlay replaces the replayed Ready prefix with the latest durable
  checkpoints. Only after both entries are overlaid can scheduling reach Guard.
* Guard/Idle/UpdateReturn have matching independent local/latest maps and an
  applied committed status. Add additionally has a valid nonterminal cursor.
* A pending update can differ from latest only at its selected, added cursor.

The readable predicate is a checked consequence of the finite-cover invariant;
we do **not** separately claim it alone characterizes all reachable states or
is inductive on every unrestricted record satisfying it.

`PROOF.bend` proves initial membership, one-step preservation for any event and
any invariant state, and safety from membership. Structural induction then
proves `product_every_prefix_safe` for **all finite event lists**, plus final
safety with an arbitrary initial crash count and with any MaxCrashes budget.
This is an inductive transition-system proof, not bounded trace enumeration.

The Python generator is **not in the proof trust boundary**. Its graph and
successor guesses are untrusted suggestions. A wrong guess fails equality
checking against the independently written Bend reducer; a missing vertex
fails a closure case. The generator cannot change the safety predicate, hide
an event constructor from Bend's exhaustive matches, or supply a proof hole.
The manual correspondence from that reducer to the TLA source is still a
review obligation, not a machine-checked cross-language theorem.

## Actual mutation and positive execution witnesses

`witnesses.bend` contains explicit event lists, checked by ordinary reduction
from `initial()`. Public laws check each mutant's false safety result **and**
the particular violated ghost flag:

* SkipStatusCommit: **8 steps**, reaching Add without commitment.
* SkipTerminalGuard: **22 steps**, persist Terminal for operation one, crash,
  replay and overlay, then incorrectly Add Terminal.
* SkipLatestOverlay: **22 steps**, persist Awaiting for operation one, crash,
  replay initial Ready, omit overlay, then incorrectly Add stale Ready.

The same three lists with `fixed()` are proved safe. These are execution
counterexamples, unlike the preserved original one-op audit-only controls.
They reproduce the respective invariant violations, not necessarily TLC's
chosen shortest trace ordering.

Additional full-record execution laws check:

* operation one restored/added as Awaiting while operation two is independently
  restored Terminal and skipped; the final added flags are `(True, False)`;
* visible/invisible initial-status append errors followed by recovery;
* visible/invisible operation-update errors followed by recovery (the former
  restores Awaiting for operation two, the latter restores Ready);
* crash after only the first overlay entry, followed by complete replay/overlay
  restoring the independently terminal second operation.

## Preserved original slice

`model.bend` and the preexisting laws retain the one-operation strengthened
protocol: `Guard{latest}` shares local/latest, `Adding{Live}` excludes Terminal,
and `addAudit` constructs commitment from typestate. Its original arbitrary
finite trace proofs, recovery examples and supplied bad-audit checks remain
valid but **are not the justification for the new independent-field model**.
The new public laws are appended rather than replacing/weaking these claims.

## Residual scope / exact-port gaps

* There is no mechanically checked equivalence between the textual TLA model
  and the Bend reducer, nor proof that the generated invariant is the *least*
  reachable set. Exhaustive closure, initial membership and invariant safety
  are checked and suffice for the stated safety theorem.
* PC/cursor/status domains are typed instead of proving TLA's untyped `TypeOK`;
  `Tick` packages the selection among mutually exclusive ordinary actions.
  There is no checked TLA trace translation function.
* The unbounded crash variant is a generalization. The budgeted wrapper retains
  the original bound semantics, but the proof is not a temporal-logic theorem
  about infinite behaviors. There is no fairness or liveness claim.
* Like TLA, this remains a two-operation abstraction with an atomic journal
  record, monotone statuses, no final tool-result record, no record corruption,
  and no external effects. It is not arbitrary-operation-count verification.
* **No Go implementation refinement**, filesystem/journal atomicity theorem,
  manager/provider behavior proof, or durable external exactly-once theorem is
  established. Retain the original TLC suite and Go runtime/fault witnesses.
