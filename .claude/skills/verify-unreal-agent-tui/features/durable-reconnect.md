# Stable retry, cursor gap, and frontend-local view restoration

Use `scripts/durable_reconnect.py REPO RUN` with a **new existing private 0700
short absolute** run directory, after `make build`. It creates its own tmux
server via `TMUX_TMPDIR`, two fake loopback-provider scenarios, and only owned
host/frontend/proxy processes. It uses the common `tui.sh keys` / `snap` actions.
Do not use the common standalone `start/stop` path or touch any user frontend.

Example:
```
R=$(mktemp -d /tmp/u14p.XXXXXX)
python3 -I .claude/skills/verify-unreal-agent-tui/scripts/durable_reconnect.py "$PWD" "$R"
```
For supervision across a tool deadline, run it in an owned named user service
with a bounded runtime and control-group cleanup; record and drain that exact unit.

Required proof (not interchangeable):
- Pre-admission drop yields disconnected/uncertain, one echo, and zero provider
  calls. F5 sends nothing. `/retry` sends exactly the original ID and payload.
- Accepted reply and preceding canonical updates are suppressed. Reconnect
  captures H=3; the proxy withholds its snapshot while the complete tool/answer
  commits. Delta plus queued live events converges. Retrying the already
  persisted input causes no RPC and no second echo.
- Both scenarios execute one external input and two **legitimate** provider calls
  (tool selection, then answer), never reconnect calls.
- A typed unsent draft, manual scroll content, selected/expanded tool result
  survive actual frontend process replacement; independent canonical snapshot
  excludes the draft and preferences.
- Preserve screenshots, saved-view DTO, final independent snapshot, identity /
  request counters, and cleanup JSON. Assert every created owner joined.

Escape is sent separately, then allowed at least 0.35 seconds to settle before
another key. Seeing the draft in expanded details does **not** prove Escape has
closed details; verify the Conversation title before Ctrl+C. Clear any retained
local `/retry` command before typing the new draft. The final CLI watcher is
intentionally interrupted after its complete snapshot; signal/closed-socket
exit is not host failure or drain proof. Judge the complete snapshot separately.

The script leaves proof data/state for inspection. After archiving evidence,
remove only this run's explicitly created state/workspace/client directories.
