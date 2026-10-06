# Palettes

`turbo-vision` is the default theme. `-theme lite` selects the original Lite
palette from `lite.json`. Each JSON file is a complete theme.

Adapted from [Dracula Classic / Alucard](https://draculatheme.com/spec),
[Catppuccin Mocha / Latte](https://catppuccin.com/palette/), and
[GitHub Primer](https://github.com/primer/github-vscode-theme).
Muted colors are adjusted where needed for the TUI.

`turbo-vision` uses the DOS/EGA colors from [Borland Turbo Vision's palette](https://github.com/magiblot/tvision/blob/master/include/tvision/app.h):
blue panes and status row, cyan user messages and double-line borders,
and yellow headings and hotkeys. Select it with `unreal-agent-tui -model MODEL -theme turbo-vision`.

Optional colors support contrasting panels: `surface_foreground`, `surface_muted`,
`surface_hint`, `surface_accent`, `surface_key`, `status_foreground`, `status_accent`,
`status_error`, `user_foreground`, `selection_foreground`, and `code_background`.
They default to the corresponding foreground, muted, hint, accent, error, or surface
colors; status text defaults to muted and shortcut keys to hint.
An optional `delimiter` color enables frames around the conversation, tools, and composer.

The interface displays messages and tool results immediately, with static status
indicators. The working timer updates once per second.
