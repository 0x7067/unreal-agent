# Durable attach (M1.2)

This recipe starts its **own explicitly launched headless host**, not the usual
standalone TUI. Read [SKILL.md](../SKILL.md); use its same `tui.sh keys` / `snap`
commands and 160×50 captures. Never drive another user's/frontend's session.

```sh
make build
R=$(mktemp -d /tmp/u12proof.XXXXXX)  # pre-existing 0700 short runtime root
python3 -I .claude/skills/verify-unreal-agent-tui/scripts/durable_attach.py "$PWD" "$R"
```

The driver uses only fake loopback credentials, isolated workspace/state/config,
and deliberately invalid attach-side provider settings. It asserts:

1. Model-only: submit headlessly, attach to the blocked turn, Ctrl+C detach,
   finish the answer with no TUI, attach a fresh frontend; one request/answer.
2. Shell: late join while the shell writes output; `Tab Enter` opens pending
   details, separate `Escape` returns. Detach, replace the frontend while still
   blocked, finish the shell, receive the answer live, open completed output,
   detach and start a third frontend. Two expected provider calls, not extra calls
   from reconnect, one answer, same host incarnation/worker generation throughout.
3. Distinct pane PIDs prove actual frontend replacement. JSONL snapshots check
   host PID/incarnation, generation, head and answer cardinality independently.

Evidence is `$R/u12-*-{model,tool}/evidence`: plain-text captures before/after,
`result.json`, `final-snapshot.json`, `detached.json`, `cleanup.json`.
The compact tool list shows a pending dot, so status is proven by opening details
and observing **Awaiting result**, not by interpreting color or the dot alone.
Compare relevant conversation/tool columns using `cut` and `diff` as in SKILL.md.

Cleanup is owned by the driver: Ctrl+C its own tmux sessions; SIGINT and Wait its
own CLI/host children; stop/join its in-process fake server. Any forced/uncertain
join fails the proof. Do not use generic `tui.sh stop` here (there is no separate
fake.pid). The driver retains state/config for inspection; after archiving the
capture evidence, delete **only this run's** workspace/state/config directories.
