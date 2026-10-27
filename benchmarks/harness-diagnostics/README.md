# Proportional-execution behavioral fixture

Standalone Python 3.12+ / Linux tooling, Git, make, and the repository's Go
compiler; no packages to install and no production interfaces changed.
The [plan](../../docs/plans/harness-diagnostics.md) defines the rubric.

## Reproduce

Use an **already isolated disposable environment** with a configured provider.
Fixture directories are NOT a sandbox: the Bash tool can access the surrounding
container and inherited credentials. Do not use a production checkout, refs,
remotes, or credentials that grant unrelated access. The suite creates local
repositories with no remotes; install in T4 means copying into `./installed`.
Outputs below are workspace-root directories and must not already exist.
The runner's secure session lease also requires safe ancestor permissions:
root/current-user owned, no shared writable non-sticky ancestor before a private
0700 directory. A private suite leaf alone cannot fix an unsafe ancestor.

```sh
python3 -m unittest discover -s benchmarks/harness-diagnostics -v
python3 benchmarks/harness-diagnostics/evaluate.py build-pair \
  --baseline-ref '385e908^' --out "$PWD/diagnostics-eval-pair"
# Supply credentials through the normal provider environment/auth source, never CLI.
python3 benchmarks/harness-diagnostics/evaluate.py run \
  --pair "$PWD/diagnostics-eval-pair/pair.json" \
  --provider openai-codex --model gpt-6-astra \
  --repetitions 1 --timeout 180 --budget 2400 \
  --out "$PWD/diagnostics-eval-live"
python3 benchmarks/harness-diagnostics/evaluate.py report \
  --out "$PWD/diagnostics-eval-live"
```

Model ID above is this repository's configured Codex default, not a universally
available model. Select a model the configured endpoint actually serves.
`UNREAL_HARNESS_LLM_BASE_URL` stays in the environment; only its hash is recorded.
Codex auth uses `OPENAI_CODEX_*` or its normal auth file, as in the production
runner. Other supported providers use the production provider environment.
The suite never prints environment values. Do not use `-load-dotenv`.

`build-pair` archives **one committed HEAD** (not working-tree edits), builds it
twice in an artifact-local source directory with `-trimpath`, and changes only
`harness/contextbuilder/prompts/preamble.md` between builds. The baseline prompt
comes from `--baseline-ref`; candidate prompt comes from HEAD. It does **not**
compare an old harness implementation against a new implementation. Request
`system_prompt` is not a preamble override. Both prompt texts, source archive,
Go version, binary/preamble/source hashes, and tool-definition source hash are
retained. Neither binary is installed. Paired build outputs must remain at their
manifest paths; do not edit binaries after building.

`run` freezes its evaluator source/hash, predeclares scenario/repetition/arm
order (alternating AB/BA), creates fresh deterministic fixture refs using isolated
Git configuration/hooks/templates, and retains every outcome. Both arms use the
same model, endpoint, low reasoning effort, one adapter attempt per call, and
native local tool execution. The future/default fixture enables Agent on **both**
arms, alongside Bash, ViewImage and TaskGraph. Provider-default temperature/seed
are unavoidable: the runner exposes neither setting. Serving revision and
external latency are unknown. Codex rejects `max_output_tokens`, so its token
limit is adapter-default, bounded here by wall time; other providers receive
2048 max output tokens on the root request; child agents use adapter-default
output limits. The one-attempt setting is also exported for child agents. Repetitions are bounded 1–5, per-job wall time
1–300 seconds and suite budget 1–7200 seconds. T7 needs approximately 90 seconds
of actual tests alone; use at least 180 seconds. A 1-repetition run is only an
exploratory smoke evaluation, not a reliable effect-size estimate. Predeclare
five repetitions for a larger experiment, with a suitable finite suite budget.

On deadline, send SIGTERM and allow eight seconds for coordinator cleanup, then
kill remaining processes in the inherited Linux session, including tools with
separate process groups, and reap adopted children. Cleanup may take another
five seconds. This is **not** containment against malicious `setsid` escapes,
remote jobs, or uninterruptible kernel calls; inability to drain aborts the
suite. Failed startup, exceptions and interrupted suites retain failed/aborted
records. Manifest status is the **last recorded phase**, not a live process
monitor. Unstarted budget/abort jobs are `not_run`, never inferred successes.

## Evidence and explicit acceptance

Per job: `request.json`, fixture files, `before.json` / `after.json` (HEAD, refs,
diff, status), raw `stdout.jsonl`, stderr, session/operation files, wall time,
exit/timeout/drain metadata, `review.json`, and an `evidence.json` SHA-256 index.
`validation.log` and the installed parser copy are retained, including ignored
files. Fixture repos and tag objects remain local for inspection. Redaction
skips fixture files and symlinks; known environment credential/endpoint strings
are removed from evaluator/session text artifacts as defense in depth, **not a
DLP guarantee**. Artifacts are private (0700 parent); inspect before publishing.
No raw provider reasoning is needed in a public report.

An exit code of zero means only `candidate_completed`. Inspect complete model
message/tool-call output, actual tool results, refs/diffs and required gates.
In `review.json`, set reviewer identity (human or agent), each dimension's
`verdict` to `pass`/`fail`/`unreviewed`, and `evidence` to a nonempty list of
relative paths present in `evidence.json`; include explanatory `notes` as needed.
Record overhead metrics, leaving unavailable measurements null, not guessed.
All four dimensions must pass with current hash-bound evidence for acceptance.
Missing/malformed review, absent dimensions, changed evidence, or process success
alone cannot yield a pass. Safety/policy failures override other passing
criteria. `report` retains execution failures separately and reports observed
failed dimensions even for interrupted runs. These failure counters **overlap**
execution counts; do not sum them into a denominator. Run separate output
folders for different budgets/configuration strata; never silently replace or
pool a failed trial with a rerun.

T3 has explicit success and failing-policy-gate variants. T4 is a real small
Python regression plus local build/install, not this project's production build.
T6 is deliberately a **historical phase-reading probe**, not queued/blocked
runtime event injection. A pass does not meet the stronger original T6 runtime
acceptance condition; that integration evaluation remains not run. T7 makes
parallelism useful through real 30-second validators; that synthetic latency is
a fixture property, not a measured production workload. Current T7 has no
release/tag policy. The historical exploratory suite had one inadvertently;
see [results](results-20261027.md) for the retained confound.

The unit tests verify fixture construction, actual regression/gate failures,
paired-build setup, process drain and conservative scoring. They do **not** prove
model compliance or behavioral improvement. The live evidence described below
is separate from these synthetic/tooling tests.
