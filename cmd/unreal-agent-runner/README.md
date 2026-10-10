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
```

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
Overlapping writers and reader/writer conflicts wait for ownership to clear;
readers can overlap.

Current limitation: a conflicting writer can change a completed candidate before
the parent accepts it. A later task can also invalidate an accepted prerequisite
by changing its declared inputs or outputs. Such graphs can repeatedly invalidate
their results instead of reaching completion. Shared mutable artifact workflows
need further ownership and evidence design; the measured evaluation covers
compatible, explicit claims. Narrow claims must still include every relevant
input and generated output.

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
