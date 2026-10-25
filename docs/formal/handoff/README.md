# Worker reload / handoff models

Two complementary, **abstract** developments of the resident-worker protocol:

- [`lean/`](lean/) proves I1/I2 for every state reachable in its transition
  system (no trace-length bound).
- [`tla/`](tla/README.md) checks bounded safety properties, counterfactuals and
  explicit reachability witnesses with TLC. Historical baseline configurations
  reproduce an abandoned-retirement status gap; the corrected-status candidate
  model reflects the shipped status repair without proving Go refinement.

Neither development proves refinement of the Go implementation. Successful
elaboration or model checking establishes the stated property of the model,
not equivalence to production scheduling, filesystem or process behavior.

## Lean project and reproduction

Lean is pinned by `lean/lean-toolchain` to **4.35.0**. Core + Std only, no mathlib
or external packages. An installed matching toolchain needs no network for build;
Elan may download it if it is missing. With the tools available in this checkout:

```sh
export ELAN_HOME="$PWD/.tools/elan"
export PATH="$ELAN_HOME/bin:$PATH"
cd docs/formal/handoff/lean
lake build
```

To force source checking without deleting an existing cache, run
`lake env lean Handoff/Model.lean` and `lake env lean Handoff/Invariants.lean`
after building imports. `.gitignore` excludes only `lean/.lake/`; the manifest
and all sources remain visible. No existing artifacts need deletion.

| Source | Contents |
| --- | --- |
| `lean/Handoff/Model.lean` | `St`, `s0`, 24-constructor relation `Step` |
| `lean/Handoff/Invariants.lean` | `Inv`, `Reach`, initialization/preservation, published theorems |
| `lean/Handoff.lean` | root imports |
| `lean/lakefile.toml`, `lean/lean-toolchain`, `lean/lake-manifest.json` | reproducible package configuration |

## Lean model and exact proof scope

`St` includes journal admission order (`journal`), acknowledgments (`acked`),
persisted input IDs (`store`), worker slots (`w`), current owner, reload slots,
delivery token, staged-artifact IDs and lifecycle latches. Worker records include
sent IDs, inbox FIFO, dedup IDs and frozen/broken flags. Rules carry historical
source citations to `workerhost`, `workerruntime`, inbox and coordinator code;
line numbers are navigation hints, not a machine-checked correspondence.

Identities are bounded (`Fin 3` for input/process/artifact/reload slots). Lists
and trace lengths are **not** bounded. Slots may be reused; artifact identities
need not be fresh. There is no exhaustive Lean explorer or `Explore.lean`.

| Property | Kernel-checked result |
| --- | --- |
| I1 | `I1`: every live slot is current. `I1_atMostOne`: any two live slots are equal. `Live` means phase is not `exited`; this is stronger than merely one delivery pointer **in this model**. |
| I2 acknowledgment safety | `I2`: membership in `acked` implies membership in `store` for reachable states. The `ack` rule itself requires persistence. |
| I2 ingress retention | `I2_monotone`: journal membership is preserved by every `Step`, including unreachable source states. Together with acknowledgment safety this accounts for admitted IDs as pending or persisted. It is **not** a filesystem durability or eventual-delivery theorem. |
| I3 no duplicate persistence | Not proved. Needs queue freshness/nodup and persisted IDs included in live-worker `seen`. |
| I4 FIFO across reload | Not proved. Needs a relation between journal order, sent cursor, queue and persisted prefix, including failed-write paths. |
| I5 old exit before replacement | Not proved as a history/state theorem. `launchAwait` explicitly requires old phase `exited` and `current = none`; `launchNoProc` requires the stopped latch. |
| I6 probe before retirement | Not proved. Successful probe is abstracted by `reloadStage`; no identity bytes or executable contents are represented. |
| I7 single-flight | Not proved. `reloadAdmit` requires all active flags false. |
| I8 artifact cleanup | Not proved. Lists of artifact IDs do not model removal errors, fresh paths, or filesystem persistence. |

The development declares no custom axiom and contains no `sorry`/`admit` proof
placeholder. Ordinary Lean kernel checking is used; this is not an independent
kernel replay or a proof of the compiler/runtime.

## Important limitations and remaining obligations

- **Atomic delivery abstraction:** Lean combines host write and inbox admission
  in `sendNew`/`sendDup`. Real pipe admission and actor admission are distinct;
  interleavings during cancellation/freeze are omitted. No refinement argument
  establishing this reduction is provided.
- **Failed writes:** `sendLost` assumes the whole input was lost and permanently
  latches `broken`. Production marks `sent` before `workerproto.Write` and cancels
  on any error, but that error can be encoding/size rejection, a partial write,
  or a pipe failure. It does not itself prove the pipe is permanently dead.
  This abstraction is insufficient to prove production FIFO or replay behavior.
- **Acknowledgments:** `ack` can fire for any persisted pending ID at any time.
  This admits schedules the ordered event consumer cannot produce. I2 remains a
  model invariant; consequences for other guards need separate simulation work.
- **Quiescence:** accepted quiesce atomically drains the queue and retires;
  refusal is nondeterministic. In-flight model/tools/operations and the final
  coordinator idle recheck are absent. Timeout before acceptance is not fully
  represented (`reloadAbortAwait` applies only in `awaiting`).
- **Lifecycle:** slots abstract generations and may be reused indefinitely.
  `hostClose` just sets closing and clears artifact IDs; it does not join a live
  process. Launch failure, callback blocking, escaped writers, leases, frontend
  statuses, and actual filesystem errors are not represented.
- **Artifacts:** `reloadStage` may reuse an existing ID. Abort filtering can
  therefore remove an ID also associated with another generation. Do not use
  this model to claim I8 without strengthening freshness/ownership first.
- **Persistence:** `store` models only journal-derived input records, not the
  session-creation input, all session records, compaction or migration. Journal
  and session writes are infallible model steps, not durability proofs.

The missing Lean results are substantive proof/model obligations, not a claim
that automation alone would establish them. Extending these models to the
planned external I/O owners in [`supervisor-io-follow-up.md`](../../plans/supervisor-io-follow-up.md)
requires explicit new states, ownership rules and evidence.
