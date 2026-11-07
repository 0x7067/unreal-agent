# Source-linked restricted-semantics proofs: string-enum predicates

**Classification: source-linked restricted-semantics proof, not full Go
refinement, whole-package verification, or verified compilation.** This component
checks the deep embedding extracted from **actual selected Go function ASTs**
against an independently declared policy, for every input byte sequence. It does
not replace Go build/type checking, tests, race detection, or source review.

## Reproduction

From the repository root:

```sh
python3 tools/formal/gosource/check.py
python3 tools/formal/gosource/test.py
~/.bend/bin/bend docs/formal/implementation/enums/PROOF.bend --check-only
```

Use Bend **2.0.27** (the root runner checks the pinned version). The component is
imported by the root law/proof indexes. `check.py --root PATH` supports isolated
source trees; extraction always uses the reviewed extractor in this checkout.
The proof avenue is restricted to the canonical root-module Go context:
`github.com/unreallabsai/unreal-agent`, no active workspace, no effective
`GOFLAGS` (in particular no overlay), and no root-module/subpath require or
replace overrides. The guard queries the **actual** Go environment first,
including persisted flags and automatically discovered ancestor workspaces,
and rejects unsupported contexts rather than quietly ignoring them. Only then
do subprocesses pin `GOTOOLCHAIN=local`, `GOWORK=off`, and empty `GOFLAGS`, with
timeouts: no automatic toolchain fetch is permitted. Go itself parses `go.mod`
via `go mod edit -json`; its bytes are included in manifest provenance. Relevant
nested `go.mod` files on actual target/dependency ancestry are rejected.
Unrelated nested modules such as the TUI are not excluded from repository
coverage and do not interfere with this root-module-only slice. `go run tools/formal/gosource/extract.go` from the root emits
the parsed AST manifest to stdout; it does not rewrite any file. **That raw
extractor output is a diagnostic, not a verification gate**: module/environment
checks, freshness comparison, and independent Bend proofs are required through
`check.py` and the root runner. No claim is made for differently resolved
workspace/overlay/submodule builds.

`check.py` always runs that extractor and compares **both** generated
`SOURCE.bend` and `source_ast.json`. It rejects drift; a digest is not used as a
shortcut for extraction. The manifest records target/dependency provenance,
source/module hashes, extracted AST, projected input names and law bindings.
Hashing module metadata is provenance, not a substitute for the explicit
resolution restrictions above.

After reviewing a deliberate implementation change, `check.py --refresh`
updates **only** these two generated source artifacts. It never changes
`POLICY.bend`, `LAWS.bend`, or `PROOF.bend`. A changed policy needs explicit human
review under the root formal policy. Refresh is not proof repair: an
implementation that violates the existing policy fails Bend after refresh.

## Exact target scope

| Actual Go source | Function/method | Independent policy |
| --- | --- | --- |
| `harness/operation/local_manager.go` | `localOperationFinished(Status)` | completed, failed, canceled |
| same | `localPrimitiveCompleted(PrimitiveEventType)` | IO create/read completed, process exited, primitive failed/canceled, compute completed |
| `harness/operation/remote_job_handler.go` | `validRemoteJobStatusTransition(Status, Status)` | ready/awaiting accept awaiting, canceling, completed, failed, canceled; canceling accepts canceling, failed, canceled; other current states reject |
| `harness/coordinator/loop.go` | `operationIsTerminal(Status)` | completed, failed, canceled |
| `harness/taskgraph/runtime.go` | `terminal(Status)` | completed, failed, canceled, unsupported (`""`) |
| same | `shellOutputPhase(ShellPhase)` | read out, out tail, err, err tail |
| `cmd/internal/subagent/activity.go` | `activityTerminal(Status)` | completed, failed, canceled |
| `harness/sessionstore/localfile/state.go` | `terminalOperationStatus(Status)` | completed, failed, canceled |
| `cmd/internal/workerruntime/run.go` | `terminal(Status)` | completed, failed, canceled, unsupported (`""`) |
| `cmd/internal/workerruntime/workload_registry.go` | `terminalOperation(Operation)` | value projection `op.Status`: completed, failed, canceled |
| `harness/llm/model.go` | `(ReasoningEffort).Valid()` | low, medium, high, xhigh, max |

`attachproto.RuntimeStatus.Terminal` is **not included**. In particular this
component makes no fail-closed claim about that function's unknown-state
acceptance. All claims here concern the selected predicates only, not callers,
state transitions of the surrounding actors, or effects following a `true`.

## Representation and checked proof

`semantics.bend` defines an inductive Boolean expression AST and a total
interpreter. The extractor translates actual pure `return` / `if` / `switch`
bodies to this deep embedding, preserving switch priority as nested branches.
Boolean operations, negation, and comparisons with typed enum string constants
are represented as AST constructors, **not pre-evaluated truth tables**.

Strings are `List<&2, Nat>`, not an enumeration of recognized statuses and not
Unicode scalar sequences. Actual Go strings embed byte-for-byte in the
0..255 subset. Constants are emitted as literal numeric byte lists. The public
laws quantify over the larger domain of *every* Nat list (and both lists for
remote transitions), so in particular cover every finite Go byte string,
including invalid UTF-8, NULs, empty strings, and unrecognized values. This is
not finite sampled string testing.

`POLICY.bend` contains separately specified byte constants and allowed-status
expressions; it does **not** import generated source definitions. Public
`LAWS.bend` obligations bind all eleven extracted functions to that independent
policy. `PROOF.bend` uses generic Boolean identities (conditional Boolean
identity, associativity, and merging equal branches), valid for arbitrary
comparison results, to normalize the source interpreter terms. Structural
byte-equality reflexivity is proved inductively over arbitrary lists/Naturals.
Public interpreter-preservation equations cover every AST constructor and
byte-equality recursion. Direct source safety laws also prove that completed,
failed, canceled, and unsupported current remote states cannot revive for any
updated byte string; show ready-to-awaiting reachability and canceling-to-completed
rejection; and distinguish taskgraph/coordinator empty-status treatment. Every `.bend` file is reachable through the component
proof and root index. No unsafe recursion, axioms, foreign bodies, or holes are
used.

## Recognized subset and rejection boundary

The extractor uses Go's **`go/parser` AST**, `go/token`, and `go/types` /
`go/constant` string-literal validation, not regular-expression source matching.
It resolves explicit package/import bindings, direct named string types, and
explicitly typed literal constants from actual declarations. It does not
attempt to type-check or verify entire packages; ordinary Go checks still do
that. Only the targeted function bodies are translated. Surrounding unrelated
functions can of course contain calls, side effects, pointers, or platform code.

Accepted bodies have one unnamed Boolean result, at most two explicitly named
value inputs, no generic parameters, and only:

- Boolean literals, parentheses, `!`, `&&`, `||`;
- `==` / `!=` between a recognized input and an explicitly typed string-enum
  literal constant (either operand order);
- pure, total `return` / `if` / `else` and input-tagged `switch` with a total
  default, no switch init, fallthrough, or trailing unreachable statements.

The sole record projection is the explicitly selected `Operation` **value**
parameter's actual named `Status` field; its actual declaration must resolve to
a direct named string type. Other fields are not modeled, dereferenced, or
claimed verified.

Unsupported syntax is rejected, not guessed or replaced with an unconstrained
model. This includes calls/conversions, assignments, pointer inputs or pointer
projections, loops, defer/go, address-taking, dynamic field access, constant
expressions instead of string literals, aliases, recursive/indirect enum types,
implicit/inherited constant expressions, dot imports, binding shadowing relevant
to the subset, and non-total control flow. Build constraints, platform-selected
filenames, and Go compiler directives on target/declaration files are rejected;
this is deliberately not Go build selection. Source/dependency/artifact
symlinks and nonlocal paths are rejected. Files with unsupported unrelated
functions are not thereby certified.

## Trust and limits

The trusted boundary includes the extractor and generator, their restricted Go
semantics interpretation, parser/literal checker, Python/Go runtimes, host
filesystem, repository/checker integrity, and Bend 2.0.27 plus its bundled Base
library and host runtime. Parsing/translation itself has **not** been proved
correct against the full Go specification or compiler. Pure total accepted
expressions have no observable difference between Go short-circuit evaluation
and the Boolean interpreter; that correspondence is part of the explicitly
trusted restricted-semantics interpretation. The projection assumes a valid
value of the surrounding well-typed Go record, not memory corruption or races.

Mutation regressions modify copies of the **actual Go implementations** for all
eleven targets. Merely refreshing provenance hashes still fails source
extraction comparison. Explicitly regenerating a violating source AST still
fails the unchanged policy proof for all eleven. Additional negatives cover
unsupported syntax, types, build tags and symlink inputs. These regressions are
checker tests, not the universal proof; the Bend laws are the proof avenue.
They do not establish that an adversary allowed to edit policy/checker/CI cannot
remove a gate. Root review/branch-protection requirements remain necessary.
