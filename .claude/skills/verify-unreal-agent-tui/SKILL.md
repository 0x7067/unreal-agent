---
name: verify-unreal-agent-tui
description: Drive the real unreal-agent-tui binary in tmux against a fake model and keep screen captures as evidence. Use to prove a TUI change works (conversation, Tools pane, details, mouse and keys), not just its tests.
---

# Verify unreal-agent-tui

Runs `bin/unreal-agent-tui` in a detached tmux session. A local fake OpenAI Responses server (`scripts/fake_model.py`) answers every model call, so runs are deterministic and need no credentials. Every state directory is under the run directory, so your real sessions and config stay untouched.

## Launch

```
make build
S=.claude/skills/verify-unreal-agent-tui/scripts/tui.sh
R=/tmp/verify-tui-$(date +%s)
$S start $R 120 3 160 50   # tool calls per turn, markdown paragraphs per reply, cols, rows
```

`start` returns once the composer placeholder `Send a message` is on screen. It exits 1 and points at `$R/stderr.txt` if the TUI doesn't come up within 10s.

## Doctor

`$S doctor $R` checks that the tmux session and the fake model are up and prints the commit and build time of `bin/unreal-agent-tui`. Rebuild with `make build` if the build time is older than your change.

## Drive

- `$S keys $R "text" Enter` types and sends a message. The fake model replies with the tool calls, then with the markdown paragraphs. Keys use tmux `send-keys` names (`Tab`, `Down`, `Escape`, `F3`, `PageUp`). Send `Escape` in its own `keys` call and wait about 0.3s. If another key follows in the same call, the terminal reads ESC plus that key as Alt+key and the Escape is lost.
- `$S wheel $R COL ROW up|down` sends one SGR wheel notch. `$S click $R COL ROW` sends a left press and release. Both COL and ROW are 1-based screen cells.
- Wait about 4s after sending a message for 120 tool calls to settle before taking a snapshot.

See [features/README.md](features/README.md) for per-feature recipes.

## Evidence

`$S snap $R NAME` writes the plain-text screen to `$R/evidence/NAME.txt` and prints it. Capture before and after every action, and compare the relevant column range with `diff <(cut -cA-B before) <(cut -cA-B after)`. Plain text loses colors, so prove focus or selection changes through text (a hint line, a pane title, a details pane), not through highlighting.

## Cleanup

`$S stop $R` kills only this run's tmux session and fake model, then deletes the workspace, state and config directories. `$R/evidence` and `$R/stderr.txt` are kept.

## Upkeep

When a recipe stops matching the app, fix it with the `maintain-verification-skill` guidance in the same branch as the change.
