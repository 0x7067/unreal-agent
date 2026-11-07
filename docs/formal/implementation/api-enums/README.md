# Generated API enum `Valid`: 476 source-linked contracts

This component establishes a **source-linked restricted-semantics contract** for
all current generated `Valid` methods in two files:

| Source | Methods |
| --- | ---: |
| `internal/openaiapi/types.gen.go` | 279 |
| `internal/anthropicapi/types.gen.go` | 197 |
| Total | **476** |

Every method has its own universally quantified law: for every `x: List Nat`,
the extracted source semantics returns exactly the independent policy result.
The policy is **true iff the raw input bytes equal one of that receiver type's
declared enum constants**. There is no normalization, Unicode assumption,
sampled test domain, string-length bound, or restriction to declared inputs.
`List Nat` includes all Go byte strings and also lists with elements above 255;
the theorem holds on that larger domain too.

This does **not** finish all Go proof obligations. It is not a Go compiler proof,
a full-package refinement proof, a proof of generated client behavior, or a proof
of unrelated methods, initialization, imports, JSON parsing, allocation, runtime,
I/O, dependencies or concurrency.

## Independent policy and source artifacts

* `policy.json` is the **frozen, independently declaration-derived** inventory of
  receiver types, constant names and decoded raw string bytes. It was initially
  obtained from `const` declarations, not cases or return values in `Valid`.
* `POLICY.bend` implements that frozen inventory as disjunction-based byte
  membership. Each checker invocation checks its correspondence to `policy.json`.
* `SOURCE.bend` contains the actual extracted method expressions: switches become
  nested `Branch` nodes, guards become raw-byte equality (or disjunction for a
  multi-value case), and returns retain their actual Boolean values. It imports
  the shared interpreter in `../enums/semantics.bend`.
* `source_ast.json` is a list of 476 records containing `id`, `file`, `name`,
  `receiver`, `source_sha256`, `law`, `classification`, `ast`, `dependencies`,
  and `module_metadata_sha256`.
  `source_sha256` hashes the complete corresponding generated source file.
  `dependencies` inventories every production `.go` file in that package with its
  own SHA256, so additions, removals and edits to sibling files require freshness
  review too. `module_metadata_sha256` fingerprints the canonical root `go.mod`
  after the shared module/workspace/overlay environment guard accepts it.
  The classification is `source-linked-restricted-semantics`, not whole-program
  refinement. AST node kinds are `bool`, `eq`, `or`, and `if`; `eq.bytes` is the
  decoded byte sequence, `if.args` is `[guard, yes, no]`, and `bool.value` defaults
  to false when omitted.
* `LAWS.bend` states the 476 contracts and the two generic identities.
  `PROOF.bend` provides the actual evidence. `helpers.bend` defines total
  structural recursion over a list of constants. The generic inductive theorem
  converts conditional chains to disjunction membership using a case split on
  an arbitrary Boolean, not exhaustive testing of selected strings. Each of the
  476 proof terms instantiates that theorem with its frozen declaration list;
  conversion checking requires its actual SOURCE expression to agree.

All helper files are reachable from the paired law/proof entries. There are no
unsafe definitions, foreign bodies, axioms, proof holes or new trust exceptions.
The helpers and imported interpreter are checked by Bend's totality checker.

## Fail-closed frontend

`tools/formal/apienums/extract.go` uses Go's actual `go/parser` AST, plus a
restricted synthetic-package `go/types` check for named types, constants,
signatures and switch comparability. It inventories every `Valid` declaration
in both package directories; an unexpected `Valid` outside the designated
`types.gen.go` files fails rather than being ignored. The Python checker also
requires the exact 279/197 method counts and unique method IDs.

Admitted receiver types are **defined, nongeneric types directly over the
predeclared `string` type**, not aliases or pointers. Signatures must have one
named value receiver, no arguments and one unnamed predeclared `bool` result.
The method must consist of exactly one switch on that exact receiver binding,
without an initializer, with an explicit default. Each case must contain exactly
one return of predeclared `true` or `false`. Case expressions must be identifiers
resolved to constants of exactly the named receiver type. Multiple values in a
case are interpreted as disjunction; branch order and default behavior are
preserved. The Go type checker rejects duplicate case values and invalid or
nonconstant declarations.

The constant evaluator handles Go string escapes, raw strings, constant
expressions, constant references, inherited constant specifications and inferred
named constant types according to `go/types`; it does not infer enum membership
from the function body. All constants are checked together with the receiver
type declarations and methods. A declaration outside this isolated environment
that cannot resolve there is an explicit unsupported-input failure. Aliases,
indirect named underlying types, build constraints, compiler directives (other
than the inert-for-compilation `go:generate`), dot imports, reserved-name import
bindings, predeclared Boolean/string name shadowing and symlinks are deliberately
rejected. Reserved-name checks scan **every production file in the package**,
not just `types.gen.go`; `var true = false` in a sibling file therefore fails
even with `--refresh`. Calls, assignments,
fallthrough, receiver conversions, alternative tags, arbitrary statements,
named results, pointer receivers and unsupported expressions are not translated.

The proof generator is not a general program equivalence solver. A semantically
equivalent AST refactoring may need an additional reviewed proof identity;
failing a proof is preferable to silently widening the frontend or weakening
the contract.

## Commands and policy workflow

```sh
python3 tools/formal/apienums/check.py
python3 tools/formal/apienums/check.py --root /absolute/mutation/tree
python3 tools/formal/apienums/check.py --refresh
python3 tools/formal/apienums/test.py -v
~/.bend/bin/bend docs/formal/implementation/api-enums/PROOF.bend --check-only
```

Every check first calls the shared `gosource` environment/source-context guards:
the root module path must be canonical, workspace/overlay source substitution is
rejected, and nested `go.mod` files on API source paths are forbidden. The raw
Go extractor is a frontend, not a standalone verification command.

Every check re-extracts the actual AST and compares the full current declared
constant set against the frozen policy. New, removed, renamed, retyped or
changed-valued constants fail even if the method body was not edited.

`--refresh` writes **only `SOURCE.bend` and `source_ast.json`**. It never writes
policy, laws or proofs. A declaration-policy mismatch is checked *before* any
refresh writes and still fails with `--refresh`. There is intentionally no
policy regeneration command: policy changes require explicit human review and
separate deliberate edits to the frozen policy and corresponding laws/proofs.
For source-only changes, review the exact AST delta, explicitly refresh source,
then run Bend against the unchanged policy and laws. Changing a member return to
false, changing default to true, or removing a member case is caught by freshness
before refresh and fails the Bend proof after refresh.

The stdlib suite exercises these three real-Bend negative cases, the positive
476-method proof, independent declaration mutations, type mutations, raw NUL and
non-UTF-8 bytes, syntax rejection, same-file/sibling-file predeclared-name shadowing, compiler
directives, module/workspace/overlay rejection, root-module metadata and new
sibling dependency freshness, inventory rejection and symlink
rejection.
Bend-dependent checks skip only when the binary is unavailable; CI installs the
pinned Bend version. Go subprocesses set `GOTOOLCHAIN=local`, `GOPROXY=off` and
`GOSUMDB=off` and have bounded timeouts; the checker does not download tools.

## Explicit trust boundary

The trusted computing base includes the Go parser and constant/type checker,
this frontend's inventory/filtering/AST translation, the restricted Go-to-Bend
semantics correspondence, Python's artifact comparison and frozen-policy gate,
and Bend 2.0.27 with its bundled Base library and runtime. The synthetic package
is intentionally not a proof or complete type check of either production
package. Proof review must validate that the admitted Go constructs really have
the stated byte-string, switch and Boolean semantics. Source freshness hashes
and regression tests support that bridge but do not themselves prove it.
Repository reviewers must also protect deliberate edits to frozen policy,
laws, proof terms, checker and CI; no in-repository checker prevents an authorized
human from changing all of them together.
