# Bend proof gate

From the repository root (Python 3.10+, standard library only):

```sh
python3 tools/formal/prove.py
python3 tools/formal/prove.py --check-structure
python3 -m unittest discover -s tools/formal -p 'test_*.py'
```

The runner resolves its default project root from its own location, so invoking
it by absolute path also works from another working directory.

`--root DIR` selects a project fixture/root, `--bend PATH` selects an executable,
and `--timeout SECONDS` sets a finite positive per-invocation deadline (120 by
default). Binary selection is explicit `--bend`, then `BEND`, then `bend` on
`PATH`, then `~/.bend/bin/bend`. An explicitly missing binary is an error, not a
reason to try another version. No installation or download is performed.

The executable must report exactly **bend 2.0.27** via `bend version`.
The gate invokes every discovered component `PROOF.bend` separately, then root
`PROOF.bend`, always with **`--check-only`**. Every invocation must exit zero and
report exactly **`All terms check.`**, without other stdout. Unsafe/foreign
summaries, missing summaries, errors, and unknown stderr fail even at exit zero.
Only Bend's precisely shaped update notice is tolerated on stderr.
`BEND_NO_TELEMETRY=1` is forced to suppress Bend's implicit update-network check;
no remote package imports are allowed.

The offline structural guard:

- Discovers root `*.bend` and all `docs/formal/**/*.bend` sources, including
  untracked additions, and recursively includes any other local sources they
  import. Vendor/cache worktrees outside this maintained seed set are not
  scanned unless imported. All discovered sources must be reachable from root
  `PROOF.bend`; unused scratch `.bend` files must be removed or relocated.
- Discovers every component `LAWS.bend`/`PROOF.bend` pair under `docs/formal`,
  requires both siblings, and requires direct imports in the corresponding root
  indexes. Each proof must directly import its sibling laws.
- Requires nonempty component law sets and a `def Alias.law_name(...)` in each
  paired proof for every law, using that proof's explicit law-import alias.
  Root-local laws are checked likewise. Delegation to imported helper proofs is
  permitted. Law declarations outside paired `LAWS.bend` files are rejected.
- Accepts only top-level `import Base` and local unquoted `./...bend` or
  `../...bend` imports. Local imports may not escape the root, traverse symlinks,
  refer to missing/nonregular files, or use duplicate paths/aliases. Symlinked
  formal directories and source files are rejected.
- Masks `#` comments and quoted literals before inspecting declarations. Rejects
  unsafe decorators (including `@unsafe`), legacy unsafe `def name?` syntax,
  foreign markers, and nested/foreign `.c` or `.js` imports, even in sources not
  otherwise used by a proof. Unsupported lexical/header conventions fail closed.
- Rechecks the source inventory/content after proof checking to catch ordinary
  concurrent edits. This is **not** a sandbox against an adversarial filesystem
  or a malicious compiler executable.

These are deliberately restrictive **lexical and structural** guards, not a
complete Bend parser. The pinned actual compiler remains responsible for syntax,
types, complete law implementations, and termination. The regression suite tests
mock failures offline, and, when installed, also tests the real pinned compiler:
a law lacking its implementing definition fails with `Error: 1 TODO found.`

`--check-structure` neither locates nor executes Bend and explicitly reports
**proofs NOT checked**. A successful full gate checks the indexed Bend protocol models and source-linked
restricted-semantics contracts. The source contracts require the separate
`make proof-source` freshness check; use canonical `make prove`, not this runner
alone, for the combined gate. Source extraction/Go-subset correspondence remains
explicitly trusted. Neither mode establishes general Go/model equivalence,
complete implementation coverage, or a codebase-wide correctness proof.
Coverage governance is maintained in `docs/formal/coverage.json` and the live
named-declaration report is `make proof-report`.


`verify.py` snapshots all participating project Go/module and formal control/artifact
inputs, runs source checks before/after the compiler, then rejects a changed tree.
This detects ordinary coupled Go/SOURCE edits and module changes during checking;
it is not an adversarial filesystem sandbox.
