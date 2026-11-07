# Restricted actual-Go AST extraction

See [the component scope and trust boundary](../../../docs/formal/implementation/enums/README.md).

- `go run tools/formal/gosource/extract.go [ROOT]`: read actual Go AST, reject
  unsupported target/declaration syntax, emit diagnostic JSON; no files changed.
  Raw JSON alone is NOT the verification gate: use check.py and Bend.
- `python3 tools/formal/gosource/check.py [--root ROOT]`: always re-extract;
  compare checked-in generated Bend semantics and AST/provenance manifest.
- `python3 tools/formal/gosource/check.py --refresh`: explicit **source-only**
  artifact update. Does not change independent policy/laws/proofs.
- `python3 tools/formal/gosource/test.py`: actual-source mutation regressions in
  temporary copies. Uses `$BEND` or `~/.bend/bin/bend` for post-refresh negative
  proofs; the root proof runner is responsible for the compiler version pin.

The Go program is unproved trusted tooling, not a certified compiler. Its own
package must remain visible as unproved tooling in the source coverage ledger.
No production Go code is changed by these commands. All generated artifacts
are deterministic and checked in under `docs/formal/implementation/enums/`.

Shared checker API (also used by source-linked API-enum proofs):
`check_environment(root)` validates actual effective Go workspace/flags and the
restricted root module graph, returning `{'go.mod': sha256}` provenance;
`check_source_context(root, source_paths)` rejects nested modules along actual
root-relative selected source/dependency paths. Only after both guards succeed
should `canonical_go_environment()` be used for subprocesses. Module digests do
not prove import resolution. Alternate workspace/overlay/module builds remain
outside this checked avenue.
