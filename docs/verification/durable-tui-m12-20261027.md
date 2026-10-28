# M1.2 TUI attach verification — 2026-10-27

## Result

**Completed M1.2:** explicit `unreal-agent-tui attach` connects to an already-running
headless host, renders a complete fresh snapshot and ordered live history, and
detaches without stopping or replacing its worker. Standalone startup is retained.
[Commands and limits](../plans/durable-tui-attach.md).

Implementation/test links:
- [TUI attach](../../cmd/unreal-agent-tui/attach.go), [UI changes](../../cmd/unreal-agent-tui/ui.go),
  [integration/unit tests](../../cmd/unreal-agent-tui/attach_test.go).
- [Reusable transport client and tests](../../cmd/internal/attachclient/),
  [shared pure validator](../../cmd/internal/attachproto/view.go),
  [JSONL compatibility wrapper](../../cmd/internal/residentcli/view.go).
- [Reproducible tmux recipe](../../.claude/skills/verify-unreal-agent-tui/features/durable-attach.md),
  [driver](../../.claude/skills/verify-unreal-agent-tui/scripts/durable_attach.py).
- [Full validation log](durable-tui-m12-20261027/validation.log),
  [source/dependency/formal-input manifest](durable-tui-m12-20261027/source.sha256),
  [actual tmux reports](durable-tui-m12-20261027/tmux-proof.log).

## Real tmux proof on final-source binaries

The verification skill's `tui.sh keys` and `snap` drive actual 160×50 terminals.
Only the loopback provider is fake. Headless host, worker, JSONL client and TUI
are real executables. Every test namespace/workspace/config is isolated. Attach
runs with poisoned provider environment, invalid preference/settings files and a
workspace `.env`; no provider bootstrap is needed or performed by the frontend.

### Model-only: answer finishes with no TUI

- Host PID **3182934**, incarnation
  `798738c7750db66ee73db4c85482890686df71e5ba6a3746cd3f7315f84b90f0`.
- Headless seed submits one stable input; provider blocks before answering.
  TUI PID **3182976** joins the already-running turn and displays its input and
  Working state. Ctrl+C exits that frontend only; the host remains alive.
- Release the provider while no TUI exists. The answer becomes readable in
  canonical history. Fresh TUI PID **3183121** renders it once.
- Independent final JSONL snapshot confirms the same PID/incarnation, worker
  generation **1**, **H=4**, **one provider call and one answer**.

[Running capture](durable-tui-m12-20261027/model/01-attached-running.txt) →
[fresh answer capture](durable-tui-m12-20261027/model/02-fresh-answer.txt).
[Structured result](durable-tui-m12-20261027/model/result.json).

### Late join during shell output: live completion and another replacement

- Host PID **3183245**, incarnation
  `d30d92111db653d9e24ef8f0b222bc329a0267fa36ead1e6efc9a90ea21423a6`.
- A headless model turn starts Bash. It prints `M12 PARTIAL OUTPUT`, then creates
  its start marker and blocks on a release file. TUI PID **3183297** late-joins
  during this active operation. `Tab Enter` opens details and proves **Awaiting
  result** in text; no color/dot interpretation is counted as status proof.
- Detach, then TUI PID **3183511** joins while the shell is still blocked. Release
  the shell: the UI receives terminal committed tool output and the final answer
  live. Tool details contain both `M12 PARTIAL OUTPUT` and `M12 FINAL OUTPUT`.
- Detach again. Fresh TUI PID **3183656** reconstructs the final answer once.
  Independent JSONL snapshot confirms generation **1**, **H=8**, unchanged host
  identity and **two expected provider calls** (tool request + final answer),
  with **no additional requests caused by either reconnect**.

[Late-join capture](durable-tui-m12-20261027/tool/01-attached-running.txt) →
[pending details](durable-tui-m12-20261027/tool/01b-pending-tool-details.txt) →
[second frontend still running](durable-tui-m12-20261027/tool/02-reattached-tool-running.txt) →
[live answer](durable-tui-m12-20261027/tool/03-live-answer.txt) →
[terminal output](durable-tui-m12-20261027/tool/04-tool-output.txt) →
[fresh final view](durable-tui-m12-20261027/tool/05-fresh-answer.txt).
[Structured result](durable-tui-m12-20261027/tool/result.json).

This explicitly proves **committed** tool-history rendering, not streaming of
uncommitted operation checkpoints. v1 operation/activity availability remains
false and the attached UI labels this limitation.

## Automated checks

Focused commands, repeated three times with race detection:

```text
go test -race ./cmd/internal/attachclient ./cmd/internal/attachproto \
  ./cmd/internal/residentcli -count=3 -timeout=80s                 PASS
go -C cmd/unreal-agent-tui test -race . -run '^TestAttach' \
  -v -count=3 -timeout=80s                                       PASS
```

Coverage includes atomic/incomplete snapshot handling, confirmed/uncertain/lost/
untrusted reply correlation, cancellation/Close unblocking the submit/read
owners, control rejection before mutation, no provider/environment bootstrap,
queued-to-persisted reconciliation, visible unavailable pending state, blocked
controls/draft retention, uncertain delivery not misreported as rejection,
disconnected status precedence while responding, and real host/worker/provider
integration for both model-only and late shell-output scenarios. Those scenarios
also assert held session leases after detach, expected answer content and exact
provider-call counts. Existing standalone tests ran in the full suite.

Independent read-only review found two first-candidate UI issues: busy state
hid disconnection, and standalone build/reload hints appeared in attach mode.
Both were fixed, covered by tests and re-inspected; no remaining blocker was
reported in that reviewed scope. Early proof-driver failures were fixture issues
(buffered stdout/readiness and compact-card status assumptions), corrected before
the complete final-source proof above; they are not counted as passing evidence.

## Required validation, installation and cleanup

On unchanged final source inputs:

```text
make test    PASS — all 48 packages (43 root + 5 TUI), none omitted/duplicated
make check   PASS — vet both modules and gofmt
make build   PASS — both binaries
source manifest before/after  IDENTICAL
```

Final unit `unreal-agent-m12-final-20261027T212305Z.service` completed with atomic
exit code **0**, `Result=success`, `ExecMainStatus=0`, `MainPID=0`. Initial unit
`unreal-agent-m12-validation-20261027T211743Z.service` also passed, but final
validation was renewed after adding visible incomplete-pending-state handling.
Both used 20-minute budgets/cgroup-wide cleanup, then were explicitly stopped,
observed inactive/dead/unloaded and had their exact observed cgroups absent.
Valid cached package evidence was reused only for unchanged inputs, including
the unchanged current-HEAD TaskGraph checker; no checker was weakened.

Both binaries were atomically installed using same-directory temporary files,
byte-compared with `bin/`, and entrypoint-smoke checked:

```text
279dc7b8468f4207e039fceb49872c79d9c6fdc10327f467289c50b344d72a48  unreal-agent-runner
9092ca02599c970d699cb970ef175d9f8ea42ee10831419135010f047929372a  unreal-agent-tui
```

The tmux proof launched/restarted only its own fresh frontend processes. Its
[model cleanup](durable-tui-m12-20261027/model/cleanup.json) and
[tool cleanup](durable-tui-m12-20261027/tool/cleanup.json) confirm host joins,
provider-thread joins and absent owned tmux sessions. No unrelated frontend was
restarted or signaled. Isolated workspace/state/config were removed only after
archiving captures and canonical snapshot evidence. Scoped implementation and
review helpers were observed drained; no validation service/helper was retained.

HEAD **`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`**, existing M0/M1.1 work and
current model-check/skill changes were preserved. Compared shared/UI boundaries
with available local `upstream/main` **`a5f3fd13032737142916523ae4344c392292f9d5`**
(not freshly fetched). Divergence is the explicit local-client path and shared
pure protocol validator; no execution runtime was moved into the UI. No session/
private-protocol version, journal format, commit-before-dispatch rule, translator/
context-builder purity or unresolved-operation resume guard changed.
No commit was requested or created.

**Remaining:** fresh snapshots/manual reattach only; no persistent local view
state, automatic same-ID UI retry, live operation/activity bridge, controls or
new operation recovery. Linux runtime proof only; not power-loss durability or
arbitrary external-effect recovery proof. **Next: M1.3 — lost-ack retry and visible
submission reconciliation.**
