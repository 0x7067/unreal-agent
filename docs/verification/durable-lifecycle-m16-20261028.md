# M1.6 — distinct detach, foreground abort and proven session shutdown

**Accepted 2026-10-28. M1.6 is complete for the explicitly classified local
ownership cases below. M2–M4 are not complete.** No production frontend or session
was used, restarted, or stopped. No commit was requested or created. The initial
HEAD remains `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; preexisting dirty/untracked
work was preserved.

## Contract and implementation

The [lifecycle work log](../plans/durable-lifecycle-controls.md) records the
reviewed divergence from local `upstream/main`
`a5f3fd13032737142916523ae4344c392292f9d5` (not fetched): scoped nonretiring abort,
independent cleanup receipts, retained-background progress-ticket acknowledgment,
and host-finalized shutdown are new semantics. Translators/context building stay
pure; intent and full operation specifications commit before dispatch.

- Attached Ctrl+C closes only the frontend. Its composer/tools/details/file help
  explicitly says **detach**. `/abort` binds to the original runtime scope and
  observed turn, including its Bash phase. `/shutdown` deliberately drains the
  session. `/retry-control ID` preserves the original ID, scope, target and bytes;
  it is separate from external-input retry/outbox state.
- Public attach protocol remains 1, with negotiated `foreground_abort.v1` and
  `session_shutdown.v1`. Only strict typed intents are admitted. Result/receipt,
  foreign-scope, external-input and arbitrary hard-stop injection is rejected.
  SDK admission is not cleanup completion, and a lost acknowledgment is uncertain.
- Private worker protocol is **3**; session format remains **2**. Fresh
  incarnation/generation and child-launch scopes correlate live cleanup proof.
  Historical pre-ready receipts cannot prove a new process drained.
- Model/Bash abort joins classified foreground callbacks/effects/output only,
  discards late output/tools and retains newer external inputs. It does not cancel
  the coordinator or acknowledged explicit-background Agent owners. Unknown,
  unsupported, compaction, root TaskGraph, legacy/task remote and untranslated
  ownership remain conservatively refused.
- Strong-runtime background progress acknowledgment completes a *ticket*, not its
  retained child or task. Structural control capability and ownership persist.
  Legacy/default/task behavior is unchanged. Actual child receipt plus process,
  reader/output, supervisor, graph, Hub, input and callback joins remain required.
- Shutdown seals ingress under an independently owned controller. A timed-out or
  canceled caller does not cancel cleanup or release authority. Worker
  `work_drained` plus fresh post-ready receipt and exact process/output/callback
  joins precede host `completed`; only then does the host release authority.
  The shared parent lease also blocks one-shot writers after the worker's own
  session lease becomes free. Uncertain host exit stays fenced/diagnostic.
- Pending canonical or journal-only lifecycle work, and remote ownership without
  later trustworthy drain evidence, block automatic resume before execution.
  Read-only offline reconciliation opens only existing anchored private files,
  shares the bounded versioned canonical codec, and writes no session metadata.

Scopes are correlation, not a sandbox against arbitrary same-UID code. A terminal
status or EOF is not cleanup proof. Manager local receipts retain at most 256 live
entries and fail closed if absent/evicted. No arbitrary escaped-daemon cleanup,
provider-side cancellation, exactly-once external effects, or known remote billing
is promised. Primitive sender epilogues are not independently exposed joins.

## Current-source required validation

Scratch: `/tmp/unreal-agent-m16-final-validation.j6ITKO`.
Unit: `unreal-agent-m16-final-validation-20261028T012053Z.service`.

`make check`, `make build`, and `make test` all passed in the bounded named user
unit. Both Go modules were included: **47 passing packages, 3 no-test packages,
exactly all 50 packages** in their `go list ./...` union. Source manifests before,
after, and at acceptance match. Unit status was success/exit 0/MainPID 0; its
observed cgroup had no remaining processes, and after stopping the exact unit its
exact recorded cgroup was absent. Complete logs/manifests are archived below.

The full suite includes real lifecycle SDK/host/runtime/supervisor tests:

- Model and Bash detach/reconnect, target-bound abort and canonical completion;
  actual forged control disconnection without mutation.
- Actual control lost-ack retry cannot abort a newer turn and persists one original
  input. Definitive stable-ID conflict rolls back only when non-admission is known.
- Deliberate stop drains real Bash; a held owned output-publication barrier leaves
  authority held after the worker lease is free. Actual competing standalone and
  resident processes are rejected; releasing the barrier completes shutdown.
- Deliberately reused private test scope plus historical receipt cannot satisfy
  current-process proof before ready; reload rotates the worker scope.
- **Actual retained background runner → Bash** acknowledges its progress ticket,
  leaves root foreground available, survives completed root abort, and produces
  a new probe effect (`x` → `xp`) *after* that completion. Whole shutdown requires
  actual child/global receipts and joins, frees all three leases, and never emits
  the late `z`. Synthetic Hub tests separately verify exact retained capability
  and missing-proof refusal; they are not substituted for this process proof.
- Read-only canonical codec, stale/forged/missing receipts, unsupported/unknown
  cleanup evidence, bounded local control state, backward local-state reads, and
  detached quit hints in all panes.

An earlier full candidate failed an obsolete CLI test expecting only one
post-detach record; the joined runtime now also appends its cleanup receipt. The
assertion was corrected to verify the unchanged client prefix and both exact tail
control types. That run was stopped and its exact cgroup drained; it was not
accepted. The final full pipeline above ran after this and the quit-hint change.

## Actual owned private tmux proofs

Both drivers used the **same newly built bytes** recorded below, explicit private
tmux sockets/foreground servers, private HOME/state/workspaces, owned fake HTTP
providers, and owned host/frontend processes. No user's tmux server was accessed.

### Lifecycle/replacement

`python3 -I .claude/skills/verify-unreal-agent-tui/scripts/durable_lifecycle.py REPO /tmp/u16.gasQlG`

Unit `unreal-agent-m16-lifecycle-tmux-20261028T012149Z.service` passed. Evidence:
`/tmp/u16.gasQlG/lifecycle/evidence`.

Three real frontends, two replacements: model-blocked detach and restored draft,
model abort, Bash-blocked detach and restored draft, Bash abort, explicit shutdown,
actual host wait 0/endpoint removal, disconnected and startup offline canonical
reconciliation. Exactly two legitimate provider requests; Bash sentinel `x` once;
no late tool; canonical bytes unchanged by offline startup. Exact per-frontend
TTY before/after equality, actual frontend child wait 0 and exact wrapper pidfd
exit observations. All **13** cleanup/result flags are true, including HTTP
handlers, relay/watchers and actual foreground tmux-server joins. The blocked HTTP
fixture does not establish remote billing cancellation. Retained-background
survival is the separate real Go process test described above.

Two earlier fixture attempts failed before controls: the Python relay socket
needed the real transport's required mode 0600; then a restored draft correctly
hid the placeholder used as readiness text. The driver was corrected to satisfy
socket policy and check the stable attached heading. No security check was relaxed.
Those failed units were stopped with exact cgroup absence, not counted as proof.

### Reconnect/outbox/view-state regression

`python3 -I .claude/skills/verify-unreal-agent-tui/scripts/durable_reconnect.py REPO /tmp/u16.IPnSDM`

Unit `unreal-agent-m16-reconnect-tmux-20261028T012150Z.service` passed **both**
pre-admission and accepted/lost-ack cuts. Each preserved original payload/ID,
committed one external input and one Bash sentinel with exactly two legitimate
provider calls, and restored draft, expanded tool details and manual scroll.
The accepted cut also committed an answer during the snapshot handoff. All four
cleanup flags are true for each cut. Both proof units exited 0/MainPID 0, were
stopped, and their exact recorded cgroups are absent.

## Atomic installation

Both files were installed to temporary files in `~/.local/bin`, chmod 0755,
atomically renamed over their destinations and byte-compared with `bin/` using
`cmp`. All comparisons passed. Already running production processes were untouched;
the required frontend replacements were the owned private proofs above.

| Binary | Built and installed SHA-256 |
| --- | --- |
| runner | `96d88e2d771a03cb8f3c2f581e8d598c8e076eae1d859df9276ef4a65550911b` |
| TUI | `6ff4fa114584717b39ac88a80d630a93064316ae01d35e1a4e01af251608d86c` |

[Complete logs, manifests, install comparisons, canonical fixtures, screenshots,
TTY/process receipts and unit drain records](durable-lifecycle-m16-20261028-evidence.tar.gz).

Next: **M2.1**, narrowly classified recovery under exclusive ownership. This
checkpoint does not enable generic operation, model, subagent or TaskGraph replay,
implementation hot replacement, or durable timers/workloads from later slices.
