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

## Two-build compatible upgrade (M3.1)

`durable_upgrade.py REPO PRIVATE_SHORT_RUN FRONTEND_A FRONTEND_B BAD_FRONTEND`
uses the owned lifecycle fixture with explicit executables. A/B must have distinct
SHA-256/build identities; the negative artifact is a real TUI built with a private
Go overlay changing only the attach protocol constant to an unsupported version.
No production source modification or workspace auto-execution is involved. Run in
a named bounded user unit, record executable/driver hashes before/after, inspect
`upgrade-result.json` and all 13 lifecycle cleanup booleans, then observe the exact
recorded unit cgroup absent and stop that exact unit. It observes the actual worker
across **all threads** of its owned host (Go may fork from a non-main thread), held
lease identity, pidfd and incompatible frontend child/TTY receipts. This is Linux-
specific; details/build provenance and accepted limits are in
`docs/verification/durable-frontend-upgrade-m31-20261028.md`. Never target a user
frontend or infer active host/runtime transfer from this proof.

## Resident runtime A/B upgrade (M3.2)

`durable_runtime_upgrade.py REPO PRIVATE_SHORT_RUN RUNNER_A RUNNER_B BAD_RUNNER`
uses the full owned lifecycle fixture, enabling only its private host's
`--runtime-replacement` and trusted worker slot. A/B are distinct-link-build-ID
compatible runner artifacts; BAD uses a private worker-protocol7→999 Go overlay.
The driver atomically changes only that private slot, drives real `/builds` and
`/reload`, proves rejected preflight, actual old pidfd exit/current held lease,
active-model refusal, and then completes the full frontend replacement/lifecycle
proof. `/reload` intentionally retains composer state: clear it with owned
Ctrl+A/Ctrl+K before entering the next command. Inspect
`runtime-upgrade-result.json`, all 13 lifecycle cleanup booleans, unchanged input
hashes, actual waits and exact unit cgroup drain. Full policy/limits and artifact
provenance: `docs/verification/durable-runtime-upgrade-m32-20261028.md`.

## Queued natural-drain runtime upgrade (M3.3)

`durable_runtime_drain.py REPO PRIVATE_SHORT_RUN RUNNER_A RUNNER_B BAD_RUNNER`
proves actual A→B upgrade after a safe model finishes naturally, then cancellation
of queued model and unsafe-shell upgrades without canceling that work. It finishes
the full lifecycle/frontend proof. Exactly3 actual provider requests (one separate
safe prelude plus the baseline2), one Bash effect, 13/13 cleanup required. Inspect
`runtime-drain-result.json`, expanded source/provenance, all waits/TTY receipts,
before/after hashes and exact named-unit cgroup drain. Optional **explicit trusted**
`OWNED_M33_TUI` selects a private race TUI; `GORACE=atexit_sleep_ms=0` avoids only
instrumentation's final sleep. Require zero race warnings. Probe children are
legitimate while queued: observer chooses exactly one held lease descriptor,
with flock refusal and bound pidfds, not one arbitrary direct child. The synthetic
local lifecycle reply must never pump a second socket reader. Current commands,
rejected-run caveats and limits: `docs/verification/durable-runtime-drain-m33-20261028.md`.

## Failed compatible runtime candidate / explicit recovery (M3.4)

`durable_runtime_failure.py REPO PRIVATE_SHORT_RUN RUNNER_A HEALTHY_B STARTUP_FAIL_B`
keeps the negative artifact's probe compatible but injects failure in private worker
Run. Actual `/reload` must settle stopped, old pidfd exited, owned host still alive,
no automatic fallback; then explicitly stop/wait that owned host, install healthy
slot and start a fresh incarnation. Includes an extra pre-model frontend detach
(provider0) and recovered frontend (provider1 at its later detach); do not inherit
the baseline helper's fixed prelude count. Full model/Bash/frontend/TTY proof must
still finish13/13, provider2/sentinel1. Inspect `runtime-failure-result.json`, actual
failed-host nonzero wait and healthy-host wait0, unchanged hashes, exact cgroup
drain/stop. Go test separately proves accepted-during-fence input recovery. This
is not active host transfer or a new-format migration. See M3.4 verification.

## Controlled idle/drained host A/B replacement (M3.5)

`durable_host_upgrade.py REPO PRIVATE_SHORT_RUN GATED_A HEALTHY_B STARTUP_FAIL_HOST_B`
uses private worker-after-real-Run and host-before-cleanup gates. Actual frontend
A detaches; wrong-incarnation CLI refuses; explicit protocol retire admits a
stable fenced input then settles drained only after worker wait. Hold old-host
exit: B must refuse ingress contention without changing bytes; release, actually
wait A, fail negative B startup, then explicitly start healthy B. Require one
recovered input request/canonical ID, same exclusive ingress/session lease inodes,
new incarnation, then full frontend/model/Bash proof13/13 with total provider3.
Keep fixture `command` variable unshadowed; intercept recovery prelude only once,
not every later prompt containing history. Inspect `host-upgrade-result.json`,
actual waits/TTY, before/after hashes, exact unit cgroup drain/stop. No active
worker transfer or user frontend replacement. M3.5 verification has exact receipts.

## Trusted workloads and independent TLS job (M4.1–M4.5)

`durable_workloads.py REPO PRIVATE_SHORT_0700_ROOT` composes the owned lifecycle
fixture. Use the current built/installed pair in a bounded named user unit. It
invokes real TrustedEcho and DurableReminder tools, then a private separately
supervised production TLS executor job. Frontend detaches/actually waits before
its exact lease-holding private worker is killed by pidfd; old host actually
waits1, fresh incarnation reattaches the same job/effect/output, then the complete
model/Bash/frontend baseline runs. No user frontend/session discovery or signals.
Require8 provider requests (4 auxiliary +2 remote +2 baseline), one remote effect,
one Bash effect,4 frontend actual waits/exact TTY, executor wait0 and13/13 cleanup.
Long history scrolls the welcome banner away: attachment requires a fresh snapshot
and actual composer detach hint. Inspect `m4-workloads-result.json`, `cleanup.json`,
all TTY/wait receipts, unchanged binary/driver/base hashes, and original exact unit
cgroup absence before/after exact stop. CLI config/CA/key files stay private; never
archive private keys or treat fixture tokens as production credentials. Current
scope, rejected-run history and limits: M4 verification report.
