# Tools pane

The right pane lists one card per tool call, newest first, with a status dot. Tab focuses it for keyboard navigation, and Enter opens the selected card's details.

## Sub-features

- `tools.list`: cards appear newest first as tool calls arrive.
- `tools.keys`: with the pane focused, Up/Down move the selection, the list follows it, and Enter opens details.
- `tools.wheel`: the wheel over the pane scrolls the list one card per notch and leaves focus in the composer.
- `tools.click`: clicking a card focuses the pane and opens that card's details.

## How to get to it (user POV)

- It's visible on the right at 90 or more columns. Below 90, Tab replaces the conversation with it.
- Tab focuses it, and Tab again returns to the composer.
- The mouse works over it without focusing it first.

## Driving it with tui.sh

Preconditions: baseline launch (`120` tool calls), then `$S keys $R "run" Enter` and a 4s wait.

- List: `$S snap $R tools-list`. Row 3 shows `Bash echo 119` and lower rows count down.
- Keys: send `$S keys $R Tab`, then `Down` 3 times, then `Enter`, then `$S snap $R tools-keys`. The screen shows `Tool details` with `Arguments  echo 116`.
- Wheel: run `$S snap $R tools-before`, send `$S wheel $R 140 10 down` five times, then `$S snap $R tools-after`. Row 3 moves from `echo 119` to `echo 114`, and the last line still starts `Enter send`, so the composer kept focus.
- Click: find the row with `grep -n "echo 110 " $R/evidence/tools-after.txt`, run `$S click $R 130 ROW`, then `$S snap $R tools-click`. The screen shows `Arguments  echo 110`. `$S keys $R Escape` brings back the list with the help line `↑/↓ tools · Enter view`, and Enter reopens `echo 110`.

## Gotchas

- Plain captures don't show the selection highlight. Prove selection by opening details.
- Cards stay `●` while a call is still running, so wait for the run to settle before asserting on status.
