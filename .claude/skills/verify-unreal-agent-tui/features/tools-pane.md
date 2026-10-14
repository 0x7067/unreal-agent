# Tools pane

The right pane lists one card per tool call, newest first, with a status dot. Tab focuses it for keyboard navigation, and Enter opens the selected card's details.

## Sub-features

- `tools.list`: cards appear newest first as tool calls arrive.
- `tools.keys`: with the pane focused, Up/Down move the selection and Enter opens details.

## How to get to it (user POV)

- It's visible on the right at 90 or more columns. Below 90, Tab replaces the conversation with it.
- Tab focuses it, and Tab again returns to the composer.

## Driving it with tui.sh

Preconditions: baseline launch (`120` tool calls), then `$S keys $R "run" Enter` and a 4s wait.

- List: `$S snap $R tools-list`. Row 3 shows `Bash echo 119` and lower rows count down.
- Keys: send `$S keys $R Tab`, then `Down` 3 times, then `Enter`, then `$S snap $R tools-keys`. The screen shows `Tool details` with `echo 116` in the title area.

## Gotchas

- Plain captures don't show the selection highlight. Prove selection by opening details.
- Cards stay `●` while a call is still running, so wait for the run to settle before asserting on status.
