# Conversation

The left pane shows the intro, your messages, agent replies rendered as markdown, and reasoning that F3 can hide. You scroll it with the mouse wheel or PgUp/PgDn.

## Sub-features

- `conv.send`: Enter sends the composer text, which then appears in the conversation.
- `conv.reply`: the agent reply renders as markdown (headings, bold).
- `conv.scroll`: the wheel or PgUp/PgDn moves the conversation without touching the composer.

## How to get to it (user POV)

- Type in the composer and press Enter.
- Scroll with the wheel over the left pane, or with PgUp/PgDn.

## Driving it with tui.sh

Preconditions: baseline launch with `$S start $R 4 40` (few tool calls, a long reply).

- Send: `$S keys $R "hello" Enter`, wait 3s, then `$S snap $R conv-send`. The screen shows `hello` and `Running 4 commands.`
- Reply: after the tools finish, the screen shows `Result 39` with no `##` (the heading was rendered).
- Scroll: run `$S snap $R conv-before`, then `$S wheel $R 40 20 up` five times, then `$S snap $R conv-after`. Columns 1–117 differ, and the composer row is unchanged.

## Gotchas

- The wheel only scrolls when the pointer is over the conversation (columns 2–117 at 160 wide).
- At the bottom, wheel-down is dropped on purpose, so scroll up first.
