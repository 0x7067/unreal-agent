# Bend 2 proof policy and verification status

**Bend 2 is the authoritative proof language. The entire Go application is NOT
proved.** `make prove` checks protocol model theorems and source-linked contracts
for a restricted, explicitly trusted Go subset. These are distinct claims.

## Canonical commands

```sh
make install-bend       # checksum-pinned Bend 2.0.27 Linux x64 installer
make proof-test         # positive/negative gates, actual-source mutations, inventory tests
make proof-check        # structure + source/coverage freshness, local Go required, no Bend needed
make prove              # source-bound stable-tree gate, all component + root Bend proofs
make proof-regressions  # full-model actual mutations of all five checkpoint fix toggles
make proof-report       # live Go AST named-declaration inventory and remaining obligations
make test check build   # Go/race/vet/format/build checks remain required
make prove-complete     # refuses success while whole-implementation obligations remain
```

Use a trusted **Bend 2.0.27** on other platforms and pass
`BEND=/absolute/path/to/bend`. Go extraction uses the installed local toolchain
with downloads disabled; it requires the canonical root-module interpretation,
no active workspace, no effective GOFLAGS/source overlay, no root/subpath module
overrides, and no nested module along participating source/dependency ancestry.
The existing separate TUI and API-generator modules are not those participating
root-module imports and remain inventoried as unproved code.

Run `bend guide` before writing Bend. Checking uses `--check-only`, never project
`main` or a foreign body. Compiler/Base/runtime remain trusted; reproducible
pinning is not a proof of compiler soundness. The pinned release does not support
`--verdict`; no independent kernel replay is claimed.

## Three evidence classes

1. **Protocol model theorem:** a Bend proof about the stated hand-written
   transition model. Concrete mutation traces do not establish Go equivalence.
2. **Source-linked restricted-semantics contract:** actual supported Go AST is
   re-extracted on every gate and its modeled evaluation is proved equal to an
   independent policy for every input. Go byte strings are represented by
   arbitrary `List Nat` values, including every byte string, not sampled or
   finite enum inputs. The restricted frontend/correspondence is trusted.
3. **Whole-implementation refinement:** a proof connecting the application,
   heap/concurrency/initialization and external interfaces to its full specified
   behavior. **There is no such proof in this repository.**

Tests, source hashes and inventory rows are not interchangeable with these
proofs. Do not label a package fully proved because some of its predicates have
source-linked contracts.

## What is checked now

| Area | Checked development | Remaining distinction |
| --- | --- | --- |
| Review | [109 existing laws](review/LAWS.bend), including termination-checked grouping; [witness rules](review/spec/README.md) | Most review behavior is still a handwritten model/test correspondence, not extraction |
| Handoff / inbox | [laws and exact scope](handoff/bend/README.md): composed store-prefix FIFO, global dedup, pending/persisted accounting; separate lifecycle metadata/probe/single-flight/artifact safety; original inbox conservation | Indexed FIFO/prefix acks/atomic receipt and separately composed slices are not the shipped Go pipe/actor/drain implementation |
| Task graph | [finite checkpoint and routing proofs](taskgraph/bend/README.md) | Two-task abstraction, not arbitrary DAG reducer refinement |
| Coordinator dispatch | [independent two-op product](coordinator-dispatch/bend/README.md): 212-state inductive invariant, 2,332 checked transition edges, arbitrary traces/crashes, three actual mutant executions | Manual TLA correspondence; journal atomicity and Go mapping remain assumptions |
| Operations | [shared-FIFO shell/remote product](operations/bend/README.md): 127-state invariant, 1,143 checked transitions, both terminal-write bounds and spawn bound over arbitrary traces/crashes | Not a proof of Go actor/channel implementation or filesystem/process/network semantics |
| Safety classifiers | [11 actual source predicates](implementation/enums/README.md), exact independent policies, explicit no-revival laws and differing empty-status handling | Restricted pure Go semantics; parser/extractor/bridge trusted |
| API enum validation | [476 actual Valid methods](implementation/api-enums/README.md), frozen declaration-derived policy independent of method bodies | Other generated API/client/JSON behavior unproved |
| Other source | [package ledger](coverage.json), `make proof-report` | Explicitly unresolved, not silently excluded |

The finite certificates above are **inductive invariants**, not bounded trace
search results: Bend checks initial membership, every action at every invariant
member, and safety of each member; structural induction lifts this to every
finite trace. Generated successor guesses are untrusted because each equality
is checked against the independently written reducer. No temporal liveness,
fairness, general TLA translation or whole-Go theorem follows from that alone.

## Source-bound gate and refresh discipline

Root [`LAWS.bend`](../../LAWS.bend) indexes all seven component law files; root
[`PROOF.bend`](../../PROOF.bend) imports their implementations. Every component is
also checked separately. Missing implementations, unindexed/orphan sources,
unsafe recursion (including legacy `def name?`), foreign proof bodies, escaping
or symlinked imports, and tainted/malformed compiler results fail the gate.

Canonical `make prove` invokes [`tools/formal/verify.py`](../../tools/formal/verify.py):

1. Snapshot Git-visible Go/module inputs and formal checker/artifact inputs.
2. Run both guarded actual-Go extractors and require source artifact freshness.
3. Type/termination-check every component and root proof with pinned Bend.
4. Repeat source extraction/freshness and compare the full input snapshot.

Thus ordinary concurrent Go edits, even coupled with a source-artifact refresh,
require a retry. This is not a hostile-filesystem sandbox or a proof of a
malicious compiler. Direct `tools/formal/prove.py` is **only** the compiler and
formal-source structure gate; it does not bind Go source freshness on its own.
Use `make prove` for the combined claim.

On source changes, review laws, correspondence, witness tests and residual gaps,
then explicitly update source artifacts where needed:

```sh
python3 tools/formal/gosource/check.py --refresh
python3 tools/formal/apienums/check.py --refresh
python3 docs/formal/check_coverage.py --refresh
make proof-test prove proof-regressions test check build
```

The first two commands update **SOURCE artifacts only**, never independent
policies/laws/proofs. API declaration changes are rejected even with refresh
until the frozen policy is explicitly reviewed. A bad implementation refreshed
into SOURCE still fails its unchanged law. The ledger refresh only acknowledges
reviewed source hashes; it does not promote correctness.

`check_coverage.py` checks package/source freshness, every retained `.lean`/`.tla`
source mapping, and source-contract classification/hash/public-law bindings.
`make proof-report` additionally inventories actual named Go function/method
declarations across modules/build variants. Its unresolved list is available via
`python3 tools/formal/completion.py --json`. Anonymous closures are within their
containing body, not separately counted; initialization and external semantics
are additional obligations, not presumed covered by the named-function count.

## Rules we abide by

- State important behavior as independent laws/policies before implementation.
- Preserve laws when fixing code. Do not weaken/delete laws, invent assumptions,
  or edit the checker to green a failing proof. Deliberate policy changes require
  explicit human approval and rationale.
- No unsafe/foreign trust exceptions or unfinished proof holes. The former
  review allowlist is empty. Bend exit zero with tainted proofs is a failure.
- Keep environment assumptions and abstraction boundaries explicit. In particular,
  source-linked predicate proofs do not establish their callers' orchestration.
- Run source/proof and runtime checks together; never replace runtime evidence
  with a theorem about a disconnected model.

## Legacy migration and enforcement

Every retained Lean/TLA model/invariant source is mapped in `coverage.json`.
Keep the original suites as supplemental evidence until a reviewed equivalence
and property-by-property replacement, including mutants and reachability, exists.
The newer products close significant old slice gaps, but the richer handoff
composition and shipped Go behavior still differ. Do not delete evidence to
make a migration look complete. Bend is the default for new formal work.

CI **`bend-prove`** runs proof regressions, all source-bound laws and the live
completion report on every PR/push to main without path filters. Releases run
those gates before publication. `make check` includes the no-Bend freshness/
structure checks. `make review-prove` is review-only compatibility, not authority
for the rest of the repository.

[CODEOWNERS](../../.github/CODEOWNERS) requests owner review of laws, policies,
checkers, module metadata and workflows. **Branch protection is external:** an
administrator must require `bend-prove` and existing CI, require code-owner
approval, dismiss stale approvals and disallow bypass/force-push. Repository
files cannot stop an authorized person changing policy or a PR editing its own
checker/workflow. Those remote settings have not been changed or verified here.

## What remains before "until the end" is true

Bend has no general Go frontend. The restricted source bridge covers only the
admitted pure classifier bodies, not thousands of mutable/I/O/concurrent bodies.
Additional Boolean facts, hashes or proof-count increases cannot close that gap.

Completion requires:

1. Full behavior contracts for remaining bodies, package initialization, panic,
   arithmetic, heap/interfaces, cancellation, channels/atomics and dependency calls.
2. A sound connected implementation semantics: extend and justify a Go frontend/
   refinement system, or migrate executable cores into Bend and verify every
   remaining Go/FFI boundary. A wholesale runtime-language rewrite is a major
   upstream divergence, not merely moving proof files.
3. A composed simulation from actual worker/coordinator/store/operation behavior
   to the proved protocols, including partial failures and recovery schedules.
4. Explicit environmental contracts for persistence, processes, networks, tool
   execution and untrusted extensions, plus any assumptions left in the TCB.

`make prove-complete` therefore refuses success and reports actual outstanding
implementation obligations. Never make it green by removing unresolved source,
shrinking the specification to the implementation, or changing a claim flag.
