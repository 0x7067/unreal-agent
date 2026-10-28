# Explicit TUI attach — implemented M1.2

The standalone TUI is preserved. An additional **explicit subcommand** connects
it to the [M1.1 foreground host](durable-headless-host.md):

```sh
# Explicit supervisor / terminal A. Provider configuration belongs here.
unreal-agent-runner host \
  -runtime-directory "$XDG_RUNTIME_DIR" \
  -session-directory "$HOME/.local/state/unreal-agent/sessions" \
  -workspace "$PWD" -session durable-demo -model gpt-5.4

# Terminal B. No provider credentials or provider/model arguments needed.
unreal-agent-tui attach \
  -runtime-directory "$XDG_RUNTIME_DIR" \
  -session-directory "$HOME/.local/state/unreal-agent/sessions" \
  -session durable-demo
```

`attach -h` lists its small flag surface. Namespace paths default to the same XDG
locations as the host; explicit overrides must match the running host. Runtime
directory safety, Unix path limits and same-UID authentication are unchanged.
There is no autostart or client takeover; another active client is refused.
The old `unreal-agent-tui -session ID` remains **standalone resume**, not attach,
and cannot take a session lease from an already-running headless host.

## Ownership and view

Attach dispatches before provider defaults, dotenv, preferences/settings,
subscription login, skill discovery or worker startup. It cannot override host
provider/model/endpoint/workspace/worker configuration. Only `-theme` is a
client-local presentation choice. The file picker is disabled because v1 does
not expose the host workspace; attaching never scans the client's workspace for
host execution configuration.

The reusable transport client obtains and validates the complete bounded fresh
snapshot **before** creating the attached UI. Canonical inputs, turns, messages
and tool-status/result records reconstruct the view; confirmed pending external
inputs are labeled queued, not persisted, and reconcile by stable input ID.
Unavailable pending state is visible rather than silently presented as empty.
Subsequent updates use the same connection/incarnation, contiguous revision/head
checks as the JSONL client. Their pure validator is shared in `attachproto`.

The UI's single event command reads directly from the connection—there is no new
unbounded event mailbox. It also consumes correlated submission replies while
the existing serialized submission queue waits for durable admission. Typing a
normal message submits an external input; admission is not session persistence,
model delivery or effect success. Uncertain replies remain explicitly uncertain
and do not restore a draft that would encourage resending under a new ID.

**Ctrl+C detaches only this frontend.** Its cleanup closes the socket and joins
its local submission queue, never calling host Close/Kill. A lost connection
retains the last view, stops local progress animation and displays **host state
unknown** even if a turn/tool was pending. Start a fresh `attach` to reconnect.
There is no automatic retry or persisted local draft/scroll state yet.

## Deliberately unavailable

- v1 exposes **committed tool history**, not current operation checkpoints or
  agent activity. The header labels this limitation. A late join can render a
  pending Bash card and later receive its terminal output and answer through
  canonical history, but does not claim to stream uncommitted shell output.
- Attach-mode slash commands, including `/reload`, `/compact`, `/agents` and
  `/builds`, are blocked locally with the draft retained; no control input is
  sent. Standalone build/reload hints are not advertised in attached mode.
- Fresh snapshots only; no cursor replay, automatic reconnect, local view-state
  persistence, shutdown/control RPC or new operation replay. These remain later
  slices. Input retry after uncertainty is currently explicit via the JSONL
  client with the same ID and payload; automatic/UI retry is M1.3.
- Transport bounds do not establish whole-host or submission-queue memory bounds.
  Same-UID authentication trusts that user, not a compromised same-user process.

[Verification](../verification/durable-tui-m12-20261027.md) includes race tests,
actual tmux captures, host/worker identity, request counts and cleanup evidence.

For explicit same-ID retry, cursor reconnect and private local view preservation, see [M1.3/M1.4](durable-tui-reconnect.md). The original M1.2 evidence below describes its historical limits.
