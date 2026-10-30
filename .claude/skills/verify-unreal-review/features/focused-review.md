# Focused review

`run --strategy focused` uses four independent discovery passes followed by a verification/consolidation pass. Candidate findings stay private until the verifier accepts them. The public findings JSONL retains its root review id, source fingerprint, one summary, and aggregate cost. `single` remains the default.

## Sub-features

- `focused-invalid` rejects a strategy other than `single` or `focused` before reviewing a repository.
- `focused-empty` completes an empty range without starting any discovery or verifier session.
- `focused-success` the compiled CLI records cost across all child requests and one complete root report.
- `focused-complete` a second invocation refuses an already-complete root file without new model calls.
- `focused-fresh` starts a new review id and independent child sessions with `--fresh`.
- `focused-resume` reuses completed stages with the same settings and retains already-recorded cost rather than charging completed stages twice.
- `focused-live` (skippable) runs a real focused review and retains its findings and model cost as evidence.

## How to get to it (user POV)

- `unreal-review run --strategy focused --out findings.jsonl`.
- Resume with the same `--strategy focused`, model, thinking level, workspace, source selection, and output file.
- `unreal-review run --strategy focused --fresh --out findings.jsonl` starts over.

## Driving it with verify-unreal-review

Preconditions:

- Launch and doctor have succeeded, and a fixture repo is available.
- Dummy key/model values are sufficient for invalid-strategy and empty-range paths.
- Real reviews require a real `OPENROUTER_API_KEY` and an explicitly selected model, and spend credit. Do not run them just to test flags. `cli.sh` pins `UNREAL_HARNESS_LLM_PROVIDER=openrouter` and drops `UNREAL_HARNESS_LLM_API_KEY` and `UNREAL_HARNESS_LLM_BASE_URL`, so these steps hit the OpenRouter loopback stub.

- **Invalid strategy.** Run `OPENROUTER_API_KEY=dummy scripts/cli.sh --name focused-invalid -- run --strategy nope --model x --workspace "$VERIFY_SCRATCH" --out "$VERIFY_SCRATCH/unused.jsonl"`. Exit `1`, stderr contains `strategy "nope": want single or focused`, and there is no findings file.
- **Empty range.** Run `OPENROUTER_API_KEY=dummy scripts/cli.sh --name focused-empty -- run --strategy focused --model x --workspace "$VERIFY_FIXTURE" --from HEAD --to HEAD --out "$VERIFY_SCRATCH/focused-empty.jsonl"`. Exit `0`, stderr contains `cost: USD 0.000000`. Copy the JSONL into evidence and confirm a complete run, the empty-diff SHA, and `No material issues: the selected range has no changes.`
- **Offline provider workflow.** `make review-canary` builds this checkout and execs the binary with an isolated HOME against scripted loopback providers. It proves an invalid strategy exits nonzero and writes no checkpoint, and that a focused empty range completes with zero recorded cost and the empty-diff summary; the large-diff canary also completes end to end under `--strategy focused`. Discovery-before-verification ordering and `--fresh` session isolation have no dedicated assertion; prove them live via the Resume bullet or add tests before citing them as covered. These paths test CLI/harness/checkpoint integration, not model reasoning or benchmark quality.
- **Live review.** Run `scripts/cli.sh --name focused-live -- run --strategy focused --model "$UNREAL_HARNESS_LLM_MODEL" --workspace "$VERIFY_FIXTURE" --from "$BASE_SHA" --to "$HEAD_SHA" --out "$VERIFY_SCRATCH/focused-live.jsonl"`. Confirm complete status and recorded cost, inspect the actual findings and summary, and copy the JSONL into evidence. Trace any confirmed finding back to code. Do not interpret an empty result as proof that real defects will be found.
- **Resume.** After cancellation of a real focused invocation, run the identical command with the same `--out`. Inspect that the root id is unchanged, cost is cumulative, and previously completed discovery sessions have no repeated calls. Missing adapter state or changed settings fails rather than silently rebuilding a different plan.

## Gotchas

- The focused strategy costs more calls and may take longer. `--timeout` covers the whole invocation, not each child.
- Notes are currently eligible for GitHub inline comments. They are not automatically hidden.
- Candidate reports are not publishable reviews. Render only the root public findings file.
- For working-tree reviews, place `--out` outside the reviewed workspace, which is better, or Git-ignore `<out>`, `<out>.work`, `<out>.context`, `<out>.context.*`, `.<out>.tmp-*`, and `.<out>.work.tmp-*` in `--out`'s directory before starting. Untracked generated artifacts otherwise change the diff fingerprint.
- Stage manifests/sessions live under HOME in the agent adapter. Preserve them for resume. `--fresh` changes the root id and isolates old state.
- Costs durably captured in completed/failed stage records can be reconciled after parent checkpoint interruption. A hard process kill while an underlying provider request is still in flight can still leave unknown billed usage, as with the single harness.
- Large diffs are automatically decomposed and coverage-gated. Combining that path with `--strategy focused` runs focused discovery inside each bounded task and can multiply cost substantially.
- Fake-provider success demonstrates pipeline integration, not improved defect recall.
