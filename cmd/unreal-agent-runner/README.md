# unreal-agent-runner

Run an AI agent from a prompt or JSON request. It writes events to stdout as
JSONL and exits when the task finishes.

Install the prebuilt runner and TUI with Homebrew:

```sh
brew install unreallabsai/tap/unreal-agent
```

Or install with Go 1.27+:

```sh
go install github.com/unreallabsai/unreal-agent/cmd/unreal-agent-runner@latest
go install github.com/unreallabsai/unreal-agent/cmd/unreal-review@latest
```

The fork release archive and the Homebrew cask also install `unreal-review`.
`unreal-agent-runner review` runs that binary with the remaining arguments
and returns its exit code. It looks next to `unreal-agent-runner`, then on
`PATH`. `unreal-agent-runner review help` prints unreal-review's usage, and
`unreal-agent-runner -h` names the alias.

Review uses the runner's provider selection (`providers.Select`). With
`UNREAL_HARNESS_LLM_PROVIDER` unset, the provider is `openai`. If the only
credential is `OPENROUTER_API_KEY`, set `UNREAL_HARNESS_LLM_PROVIDER=openrouter`.
A missing key reports `UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set`
for that default, or `UNREAL_HARNESS_LLM_API_KEY` and the selected provider's
own variable. It no longer says only to set `OPENROUTER_API_KEY`.

Set an OpenAI API key and run a prompt in the current directory:

```sh
export OPENAI_API_KEY="..."
unreal-agent-runner -p 'Inspect this project and explain how to run its tests.'
```

Or run from source at the repository root:

```sh
go run ./cmd/unreal-agent-runner -p 'Inspect this project and explain how to run its tests.'
```

Choose a workspace and save the output:

```sh
unreal-agent-runner -workspace ./my-project -p 'Summarize this project.' > run.jsonl
```

Sessions: `${XDG_STATE_HOME:-$HOME/.local/state}/unreal-agent/sessions`
(override with `-session-directory`).

You can also pass a JSON request as an argument or through stdin:

```sh
unreal-agent-runner '{"prompt":"Summarize this project."}'
unreal-agent-runner < request.json
```

OpenAI is the default provider. Set `UNREAL_HARNESS_LLM_PROVIDER` to `openai`,
`openai-codex`, `openrouter`, `fireworks`, or `ollama`, and
`UNREAL_HARNESS_LLM_MODEL` to choose a model.

Run `unreal-agent-runner -h` for options and the JSON request fields.

## Environment and workspace trust

Workspace `.env` files are **loaded automatically**, matching upstream. Use
`-load-dotenv=false` to skip loading:

```sh
unreal-agent-runner -workspace ./project -load-dotenv=false -p 'Summarize this project.'
```

Review the workspace's `.env` before running: it can redirect credentials through
endpoint/proxy settings and change provider/model and config/state paths.
Variables fill unset environment entries, except `SANDBOX_EGRESS_PROXY`, which
can replace its inherited value and is copied to `HTTPS_PROXY`. Changes are scoped
to the run. Flags are parsed before loading; help never loads the file.
The TUI does not load `.env` by default, but offers an explicit `-load-dotenv`
option and skips loading for help/version. Subagents always inherit the parent's
environment and never reload workspace `.env`, even with `-load-dotenv` present.
The library's `LoadDotEnv(path)` remains available for explicit trusted-file use.

Tool wall-time warnings, automatic cancellation and the concurrent-operation
admission bound are **disabled by default**, matching upstream. Set
`UNREAL_HARNESS_OPERATION_WARN_AFTER`, `UNREAL_HARNESS_OPERATION_TIMEOUT` (durations),
or `UNREAL_HARNESS_MAX_OPERATIONS` (integer) to opt into limits. Zero disables each
limit. Explicit per-call Bash thresholds remain supported. A timeout requests
cancellation; it does not establish that owned work has drained.

## Model compaction settings

Runner and TUI load built-in model settings, merged with
`${XDG_CONFIG_HOME:-$HOME/.config}/unreal-agent/settings.json` by provider and model
ID. Listed models keep their explicit `compaction_threshold`. An omitted or zero
threshold on a listed model defaults to integer `context_window / 2`.

An unlisted provider/model pair returns zero-value model settings, matching
upstream: no inferred context window or implicit compaction threshold. Configure
unlisted models explicitly to enable automatic compaction at an appropriate size.
For example, a custom OpenRouter model with a 32,000-token context window:

```json
{"providers":{"openrouter":{"info":{"id":"openrouter","name":"OpenRouter"},"models":[{"id":"vendor/custom-model","name":"Custom model","context_window":32000,"compaction_threshold":16000}]}}}
```

## Subagents

Add `"extra_allowed_tools":["Agent"]` to a request to give the agent the
`Agent` and `SendMessage` tools:

```sh
unreal-agent-runner '{"prompt":"Review each package in parallel.","extra_allowed_tools":["Agent"]}'
```

Each subagent is another runner process with its own session, started with
`-inbox-stdin -subagent`. Agents talk through their inboxes: the parent writes
inbox inputs as JSONL to the subagent's stdin, and the subagent's
`SendMessage` calls to `"parent"` arrive in the parent's inbox. An `Agent`
call runs until its subagent is idle and returns the subagent's final message.
Messaging a finished subagent resumes its session. Subagents cannot start
subagents of their own.

## Task graphs

`TaskGraph` is available to the parent agent in the runner and TUI. Command tasks
use the enabled Bash capability; prompt tasks require Agent permission. For the
runner, enable prompt tasks with `"extra_allowed_tools":["Agent"]` as above.
The TUI enables subagents by default (`-subagents=false` disables them).
Child agents cannot start task graphs.

Ask the coordinating agent to overlap substantial independent work, declare its
dependencies and file ownership, and inspect completed candidates against their
acceptance conditions. Use direct tools for tiny or serial work: graph and worker
coordination adds overhead. Scheduling prioritizes the remaining critical path
using optional `estimated_seconds`; the goal is time to an accepted result, not
the number of agents. General goal speedup and model quality are not established
by this scheduling mechanism alone.

The following JSON is a **TaskGraph tool call**, not a runner request. It shows
two independent writers and a dependent integration task in a disposable
workspace. These small commands illustrate the protocol, not a useful speedup.

```json
{
  "action": "start",
  "name": "combine-notes",
  "concurrency": 4,
  "tasks": [
    {
      "id": "left",
      "command": "printf 'left\\n' > graph-left.txt",
      "reads": [],
      "writes": ["graph-left.txt"],
      "acceptance": "graph-left.txt contains exactly one line: left"
    },
    {
      "id": "right",
      "command": "printf 'right\\n' > graph-right.txt",
      "reads": [],
      "writes": ["graph-right.txt"],
      "acceptance": "graph-right.txt contains exactly one line: right"
    },
    {
      "id": "combine",
      "command": "cat graph-left.txt graph-right.txt > graph-combined.txt",
      "depends_on": ["left", "right"],
      "reads": ["graph-left.txt", "graph-right.txt"],
      "writes": ["graph-combined.txt"],
      "acceptance": "graph-combined.txt contains exactly two lines: left then right"
    }
  ]
}
```

Each task needs a unique stable `id`, exactly one `command` or `prompt`, and
nonempty `acceptance` conditions. Dependencies must form an acyclic graph.
Concurrency defaults to 4 and accepts 1–16; one graph can be active per handler.
Declare workspace-relative `reads` and `writes`, including all generated files
and build artifacts. Omitted or `null` claims conservatively cover the whole
workspace (`.`); `reads: []` declares no reads and `writes: []` declares no writes.
Reader/writer conflicts wait for ownership to clear, and a completed candidate
keeps its claims until the parent accepts or rejects it; readers can overlap.

Acceptance evidence covers a task's claimed paths, so graphs that would
overwrite accepted results are rejected at `start` and `revise`: a task may not
write paths claimed by any of its prerequisites, direct or transitive, and two
unrelated tasks may not write into each other's claims (including overlapping
writes). Unknown `writes` overlap every claim. Workflows that edit one artifact
in sequence should use a single task or disjoint output paths. Narrow claims
must still include every relevant input and generated output.

Completion produces a candidate, not acceptance. Progress arrives in the inbox
while independent tasks continue. Inspect the graph's JSON status, including
each node's result, status, generation and evidence, and any blockers:

```json
{"action":"status","name":"combine-notes"}
```

After reading `graph-left.txt` and checking its acceptance condition, use the
**current generation from status**. For example, if it is still generation 1:

```json
{"action":"accept","name":"combine-notes","task_id":"left","generation":1,"evidence":"Inspected graph-left.txt: exactly one line, left."}
```

Accept `right` after its own check to unlock `combine`, then inspect and accept
the combined output. The parent judges whether the evidence satisfies the
conditions; the runtime requires evidence and checks generation and declared
file freshness, but does not evaluate the meaning of the evidence. It fingerprints
declared inputs and outputs and rechecks accepted evidence before downstream
admission and graph completion, including after checkpoint restoration. Relevant
changes invalidate affected tasks and their descendants; declare every relevant
input so a successful check cannot silently justify a changed result.

Use `reject` with `task_id`, current `generation` and an `evidence` reason for an
unmet condition. `retry` takes a failed or canceled `task_id`; `invalidate` takes
a `task_id` and supersedes its evidence and descendants. `revise` supplies full
task declarations in `tasks` to atomically add or replace tasks by ID. It rejects
invalid graphs and revisions affecting running or canceling tasks until those
attempts terminate. `cancel` takes an optional `task_id`; omit it to cancel the
whole graph. Reservations remain held until execution actually terminates.

Ownership is cooperative scheduling, not a filesystem sandbox. Parent direct
tools are outside these reservations and must respect active task ownership.
Declared symlink paths are refused. Checkpoints preserve graph state and
freshness evidence, but do not prove process reattachment: a restored running
shell process or active child agent with an unknown outcome retains its
reservation and may leave the graph blocked.

## Docker

The `unrea1labs/unreal-agent` image supports Linux on AMD64 and ARM64. Run it
with a project mounted as the workspace:

```sh
docker run --rm -i --user "$(id -u):$(id -g)" \
  -e OPENAI_API_KEY -v "$PWD:/workspace" \
  -v unreal-agent-state:/state \
  unrea1labs/unreal-agent:latest -p 'Summarize this project.'
```

Each release also publishes its Git tag (for example, `v0.1.0`) for version pinning.

## Independent headless host and lifecycle controls

`unreal-agent-runner host -runtime-directory "$PRIVATE_RUNTIME"
-session-directory "$SESSION_DIRECTORY" -session "$SESSION_ID"
-workspace "$ABSOLUTE_WORKSPACE"` owns a stable session independently of any
attached frontend. The runtime directory must already be private (0700). Provider
selection belongs to this explicit host startup, not attach or reconnect.
`unreal-agent-runner attach` watches/submits external inputs over the same-UID
local transport; it never starts a replacement host automatically.

The public attach protocol remains version 1. New typed lifecycle admission is
negotiated through `foreground_abort.v1` and `session_shutdown.v1` capabilities;
only exact-scope abort/shutdown intents are admitted, never results, drain
receipts, or arbitrary controls. SDK `SubmitLifecycle` requires its existing
single `Next` reader and acknowledges ingress only. Matching canonical outcomes
are authoritative. TUI `/abort`, `/shutdown` and `/retry-control ID` expose these
separately from ordinary detach and external-input retry.

The private worker protocol is version **3** (old private workers are explicitly
rejected); the canonical session format remains version **2**. A new process gets
a fresh generation-bound control scope. Strong owned supervisors require fresh
per-launch child cleanup receipts. A full committed explicit-background progress
report can acknowledge its Run ticket while its child remains owned/running;
that terminal ticket is not task completion or cleanup. Shutdown still joins the
child and requires its receipt. Default/task and legacy unproved supervisors do
not gain this early acknowledgement behavior.

A cleanup deadline or worker EOF cannot release shutdown authority. The shared
parent authority lease fences both resident and one-shot writers; journal-only,
pending/uncertain canonical intents and unproven retained remote ownership refuse
automatic restart before providers/tools initialize. This is not generic remote
recovery, a sandbox, proof of provider-side billing cancellation, or proof against
arbitrary escaped daemons. See the [M1.6 work log](../../docs/plans/durable-lifecycle-controls.md)
for implementation and verification status; later roadmap recovery is not implied.
