# unreal-agent-tui verification map

Recipes for proving user-facing TUI behavior with `scripts/tui.sh`. Read [SKILL.md](../SKILL.md) for launch, evidence and cleanup.

## Baseline preconditions

- `make build` has run on the commit under test.
- One run directory per verification: `R=/tmp/verify-tui-$(date +%s)`.
- Launched with `$S start $R 120 3 160 50` unless a recipe says otherwise. At 160 columns the Tools pane sits on the right, covering columns 118–159.
- Never drive a TUI that this run didn't start.

## Driving conventions

- Coordinates are 1-based screen cells. Rows 2 to 49 are pane content at 160×50.
- Snapshot before and after each action. Name the files after the feature ID and step.
- Plain-text captures carry no color. Prove state with text, such as the details title `Tool details` or the hint `Esc back`.

## Proof and skip reporting

- Report the feature ID, the command sent and the before/after snapshot paths.
- If a recipe can't be driven, report the command you tried and the precondition that wasn't met. Don't count a different path as proof.

## Features

- [Conversation](./conversation.md): sending a message, the markdown reply, wheel and PgUp scrolling, hiding reasoning.
- [Tools pane](./tools-pane.md): the tool card list, keyboard navigation, wheel scrolling, and opening details by click or Enter.
- [Tool details](./tool-details.md): result output, scrolling, returning with Esc.

- [Durable attach](./durable-attach.md): explicitly hosted turn, TUI detach/replacement, late tool output and independent identity/request-count checks.

- [Durable reconnect](./durable-reconnect.md): lost-ack same-ID retry, held cursor snapshot during commits, private local view restoration and owned tmux-server cleanup.
