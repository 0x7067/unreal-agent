# Tool details

The details pane shows one tool call's arguments, status and output. Esc returns to the Tools pane with that card still selected.

## Sub-features

- `details.open`: shows the selected call's name, status and result output.
- `details.back`: Esc closes details and refocuses the Tools pane.

## How to get to it (user POV)

- Tools pane focused, select a card, press Enter.

## Driving it with tui.sh

Preconditions: baseline launch, `$S keys $R "run" Enter`, a 4s wait, then `$S keys $R Tab Enter`.

- Open: `$S snap $R details-open`. The screen shows `Tool details`, `Bash`, `Completed` and `Result` with the output `119`.
- Back: `$S keys $R Escape`, then `$S snap $R details-back`. The Tools pane is back and the help line shows `Enter view`.

## Gotchas

- Opening details replaces the conversation and the Tools pane until you press Esc.
