# Preserved M3.4 source fixtures

These bytes are copied verbatim, without fetch or regeneration, from the local
commit `a5f3fd13032737142916523ae4344c392292f9d5`:

- `upstream-a5f3-golden-v2.session.jsonl` ←
  `harness/sessionstore/localfile/testdata/golden-session.session.jsonl`.
- `upstream-a5f3-legacy-v1.session.jsonl` ←
  `harness/sessionstore/localfile/testdata/legacy-session.session.jsonl`.

The v2 store remains readable; its unfinished synthetic `test` operation is NOT
runtime replay permission. V1 is explicitly unsupported, not silently migrated.
Tests derive unsupported future versions in private copies only and assert
startup refusal preserves every canonical byte and makes no provider call.
No migration or new persisted format is introduced by M3.4.
