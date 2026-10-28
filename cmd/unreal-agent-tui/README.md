# Unreal Agent

A minimal terminal client for Harness. Run it from your workspace:

```sh
brew install unreallabsai/tap/unreal-agent
unreal-agent
```

`uat` and `unreal-agent-tui` launch the same TUI. To build from source instead:

```sh
go -C cmd/unreal-agent-tui install .
unreal-agent-tui
unreal-agent-tui -provider openai -model gpt-6-astra -theme turbo-vision
unreal-agent-tui -help
```

On the first launch without a provider flag or `UNREAL_HARNESS_LLM_PROVIDER`,
Unreal Agent discovers an existing Codex subscription login and asks whether to
use it. The dialog has model and reasoning selectors: Tab or Up/Down selects a
field, Left/Right changes its value, and Enter confirms. The initial defaults
are `gpt-6.1-sol`, high reasoning effort, and the Turbo Vision theme.
The model selector uses the Codex catalog in `settings.json`, including custom models.

Accepting saves the provider, model, and reasoning choice in
`$XDG_CONFIG_HOME/unreal-agent/preferences.json` (default
`~/.config/unreal-agent/preferences.json`). Later launches reuse that choice
without the dialog. Use `unreal-agent-tui -setup` to reopen it and change the choice.
Declining exits and shows example launch commands. If credentials are unavailable
or invalid, it shows sign-in instructions and examples for other providers.

Explicit flags override `UNREAL_HARNESS_LLM_PROVIDER`, `UNREAL_HARNESS_LLM_MODEL`,
and `UNREAL_HARNESS_LLM_REASONING_EFFORT`, which override saved choices.
Overrides apply to the current launch; accepting the setup dialog updates the saved
choice. An explicit provider skips the subscription dialog unless `-setup` is used;
`-provider openai-codex` uses saved model and reasoning settings when available.
Other providers use their registered default model when available and medium
reasoning effort. Providers without a default model require `-model`.

Turbo Vision is the default theme. Use `-theme lite` for the original Lite palette.
[Theme examples](themes/).

Workspace skills are loaded from `.harness/skills/*/SKILL.md`.

Type `@` in a message to search workspace filenames. Matching is case-insensitive
and fuzzy, using [sahilm/fuzzy](https://github.com/sahilm/fuzzy).
Filename matches rank ahead of directory matches;
include `/` to search a path. For example, `@uig` can find
`cmd/unreal-agent-tui/ui.go`. Up/Down selects
a result, Enter or Tab replaces the `@` query with its workspace-relative path,
and Esc dismisses the picker. Paths containing spaces are quoted. The next Enter
sends the message.
The picker scans files when opened, including hidden and untracked files, respects
workspace and nested `.gitignore` files, and skips version-control metadata
directories and directory symlinks. Scanning and searching run in process;
they do not launch Git, a shell, or other subprocesses.
File references insert paths into the message; file contents are not attached.

## Bash output

Bash tool details show stdout and stderr as literal terminal text, preserving
ANSI colors, indentation, and blank lines rather than interpreting Markdown.
For stable viewport layout, each tab displays as four spaces (not terminal tab
stops); raw captures retain the original tabs. Exit codes and stream headings
are styled separately; full capture paths point to unabridged output when the
display is shortened. Commands run with piped output, not a PTY,
so programs often need an explicit option such as `--color=always` to emit colors.
Terminal controls and hyperlinks are removed for safety.

## Environment

Workspace `.env` files are **not loaded by default**, including for help/version.
The inherited process environment remains trusted, including credentials and
`UNREAL_HARNESS_LLM_BASE_URL`. To trust a workspace's environment explicitly:

```sh
unreal-agent-tui -load-dotenv
```

Migration from older versions: export configuration from a trusted shell or add
`-load-dotenv` only for reviewed workspaces. Opt-in can change credential destinations
(endpoint/proxy), provider, model, and config/state paths. Variables fill unset
environment entries, except `SANDBOX_EGRESS_PROXY`, which can replace its inherited
value and is copied to `HTTPS_PROXY`. CLI flags still take precedence. Loading is
scoped to the run and skipped for help/version even when opt-in is supplied.
Subagents inherit the parent environment and never reload workspace `.env`, even
if their runner receives `-load-dotenv`.

## Model compaction settings

The TUI shares the runner's [model settings and fallback](../unreal-agent-runner/README.md#model-compaction-settings).
Unlisted provider/model pairs use `DefaultCompactionThreshold = 64000` tokens,
including model changes in the TUI. Listed settings retain their configured
threshold, or `context_window / 2` when omitted/zero. Configure smaller-context
models explicitly; the fallback is not a claim about the actual context window.

## Subagents

The `Agent` and `SendMessage` tools are on by default; `-subagents=false` disables them.
Subagents run as child processes started with `unreal-agent-tui agent-runner`.
Their stderr goes to `subagents.log` in the session's operations directory, not the terminal.
Child sessions are stored alongside the parent's session.

## ChatGPT subscription

Sign in with Codex using file credentials:

```sh
codex -c 'cli_auth_credentials_store="file"' login
unreal-agent-tui
```

Credentials come from `$CODEX_HOME/auth.json` or `~/.codex/auth.json`;
override with `OPENAI_CODEX_AUTH_FILE`. Alternatively use
`OPENAI_CODEX_ACCESS_TOKEN` and `OPENAI_CODEX_ACCOUNT_ID`.
Renew credentials with Codex, then restart the agent.

## Sending messages

Enter immediately shows your message with **Submitting · not yet persisted**.
**Queued · not yet persisted** means the retained frontend accepted it into
durable ingress, not that the worker persisted it to session history or delivered
it to the model. The status clears only when the session store
observes the matching input ID; this does not imply a model response.
**Send failed · not persisted** retains the message and restores its text in the
composer. If the run ends first, **Run ended · persistence unconfirmed** keeps
the unresolved message visible without claiming delivery. The visual pending states are
process-local; durable ingress survives frontend/worker replacement and is
reconciled against session history when the same session is resumed.

Sending reveals the latest message; background acknowledgments preserve manual
scroll. `/compact` requests manual compaction and uses the same submission
acknowledgment states.

## Explicit worker reload

The terminal UI stays in place while a resident worker owns session/model/tool
execution. Type `/reload` on its own and press Enter to explicitly replace the
worker. This is a local UI command: it is never sent to the model or recorded as
chat/session input. Reload preserves the transcript, tool/details panes, manual
scroll, and any draft typed while replacement is in progress.

By default `/reload` stages the current executable path again, so a trusted
installed binary can be upgraded without restarting the frontend. To choose a
separate trusted candidate explicitly:

```sh
unreal-agent-tui -worker-executable /absolute/path/to/new/unreal-agent-tui
```

The flag is a trusted executable path, not a shell command. It selects the reload
candidate only; initial startup uses the frontend's own binary. It is never read
from workspace settings or environment variables. Candidates must support the
private `worker-probe`, `worker`, and embedded `agent-runner` subcommands. The
probe checks protocol/session compatibility without consulting auth, config,
workspace dotenv, or opening a terminal. Worker identity/generation is shown in
the transcript header; reload progress, busy rejection, and errors appear in the
status row.

Reload is allowed only at a proven idle boundary. Active model calls, graph/tool
work, or input not yet proven session-persisted block replacement rather than
canceling work. Probe failures leave the old worker running. If replacement fails
after old-worker retirement, the session stays stopped; there is no automatic
binary downgrade. New text still receives conservative session-persistence
status. In this locally supervised mode (not `attach`), Ctrl+C shutdown,
unlike reload, may cancel active work and waits
for bounded worker cleanup (shells that ignore SIGTERM take 5s; a stuck worker is
killed after 20s). When the stop takes more than a second, stderr says so; a
second Ctrl+C kills the worker's process group at once instead of waiting, which
may leave detached shell commands running.

The Session ID is shown in the transcript header. To resume its history and
pending durable ingress after restarting the frontend:

```sh
unreal-agent-tui -session YOUR-SESSION-ID
```

Only a safely terminal session can resume; interrupted nonterminal model/tool
work is refused rather than silently rerun. Use the same trusted workspace,
provider, and state directory. Worker replacements launch descendants from the
replacement binary, not the original frontend's executable.

## Attach to a durable headless session

The independent-host attach path is different from the locally supervised mode
above. Start an explicitly chosen host with `unreal-agent-runner host`, then:

```sh
unreal-agent-tui attach -runtime-directory "$PRIVATE_RUNTIME" \
  -session-directory "$SESSION_DIRECTORY" -session "$SESSION_ID"
```

Runtime/state paths are explicit private absolute paths. Attach does not start a
worker, initialize a provider, or load workspace credentials. **Ctrl+C detaches
only this frontend**; model calls, Bash and host-owned branches keep running.
F5 or `/reconnect` obtains a fresh view without resubmitting inputs. `/retry ID`
retries only an original external input, not a control or arbitrary command.

Negotiated lifecycle commands:

- **`/abort`** saves a UUID, exact host-generation scope and original canonical
  turn target before sending. Supported foreground model/Bash work is canceled
  and independently drained without retiring the session or canceling retained
  explicit-background branches. Unsupported ownership/compaction is refused.
- **`/shutdown`** deliberately seals ingress and drains all supported owned
  execution. Admission is **not** cleanup. The host retains writer authority
  through fresh worker/child proof, actual process/output joins and final
  canonical completion; a caller timeout does not authorize takeover.
- **`/retry-control ID`** uses the saved original control, never the newest turn
  or scope. Stale controls are not rebound. After endpoint loss it performs
  read-only canonical reconciliation; EOF or a missing socket alone is unknown.

Older hosts without the capability receive no lifecycle RPC. Control identities
are stored separately from the external outbox in private local-state version 2
(backward reads of version 1 remain supported). Local view state is not execution
state. Restarting attach after a deliberate shutdown can report matching
canonical outcomes without starting a host or provider. See the
[lifecycle implementation/proof work log](../../docs/plans/durable-lifecycle-controls.md)
for current verification status and conservative limits.
