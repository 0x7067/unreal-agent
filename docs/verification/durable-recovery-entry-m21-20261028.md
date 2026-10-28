# M2.1 — exclusive recovery of accepted input and classified initial intent

**Accepted 2026-10-28. M2.1 is complete; M2.2–M4 remain unfinished.**
The [recovery entry contract](../plans/durable-recovery-entry.md) defines the exact
cells, limits and reviewed divergence. HEAD remains
`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; no commit was requested or made and
preexisting dirty/untracked work was preserved. No production session/frontend
was accessed, replaced or stopped.

## What is enabled

A pure, deterministic and bounded-diagnostic `recoverygate.Validate` is used by
both worker-runtime and standalone entrypoints under their existing ownership.
It does no I/O or dispatch. Valid `value` v1 ready intent and `view_image` v1
initial-ready intent with no result may recover under their existing reducers.
Committed completed/failed/canceled history remains inert, not permission to
redispatch; other history decoders retain their own version checks. Every other
nonterminal type/version/status/state remains refused. Image awaiting/canceling,
skill offsets, arbitrary ready Bash, and remotes are **not enabled** by this slice.

An unfinished initial image may read the **current** source; it is not an immutable
snapshot of the old file. Broader image interruption/missing-source cases are M2.2.
Pure value is a legitimate subagent-to-parent operation; root parent-side remote
translators cannot consume that result type. The resident process test therefore
uses a real ViewImage rather than an invented tool/value pairing.

Canonical and journal-only lifecycle fences, unproven remote ownership, exclusive
leases and unsupported-state refusal remain independent prerequisites. Standalone
resume cannot bypass the new gate before provider construction. No mandatory
manager, wire or session schema changes, translator/context-builder I/O, or generic
model/compaction retry semantics are introduced. Local upstream
`a5f3fd13032737142916523ae4344c392292f9d5` was compared without fetching; its pure
reducers and terminal dispatch exclusion were preserved. The new private gate,
standalone tightening and initial-image overlap with M2.2 were raised with user.

## Actual process and dispatch evidence

`go test -race ./cmd/internal/workerhost -run '^TestM21' -count=1 -timeout=60s -v`
passed all three recovery cases in the owned bounded unit
`unreal-agent-m21-restart-20261028T013940Z.service`:
`/tmp/unreal-agent-m21-restart.CApVHc` (package 26.091s).

A controlled producer process uses actual durable ingress/session APIs and holds
both leases and ingress. Each committed marker follows confirmed durable APIs.
The parent kills only that exact owned `Cmd`, joins `Wait` and checks its SIGKILL
status. **This is not an instrumented kill inside a runtime-internal decision
window.** It establishes these exact durable states before actual recovery:

1. Two stable-ID external inputs synced to ingress, no canonical input yet.
2. First input canonical but not journal-acknowledged; second remains pending.
3. Original canonical ViewImage tool call with ready v1 intent, never dispatched.

Actual workerhost/runtime recovery preserves FIFO, payloads and original IDs,
returns the real PNG result under the original operation ID, acknowledges input,
and rejects competing hosts. All three measured runs used one recovered provider
request. Input batching may legitimately use one or two; the test cross-checks
actual response counts rather than promising coalescing. Second clean restart
makes no extra provider call or operation result and preserves the committed prefix;
a causal quiescence barrier may append only its fresh exact scoped drain receipt.
Old leases are tested free only after actual producer Wait or host join.

Canonical completion counting is **not** a direct admission counter. A separate
actual coordinator/local-manager harness,
`TestRecoveryDispatchCounterAndTerminalExclusion`, directly counts `Add`: one
original pure-value admission and one model call in the recovered run, zero of
both after terminal replay, original canonical completion present once. It passed
three consecutive focused race runs and the final full suite. The legitimate
send-to-parent translator is used without any remote-message-delivery claim.

`TestRecoveryGateStandaloneRefusesShellBeforeProviders` proves no provider setup,
canonical mutation, output, Bash sentinel or retained lease on refusal. Existing
shell intent/effect ambiguity tests still refuse before providers/environment.
The first full candidate exposed a matrix test still treating all images as
unsupported and old crash tests expecting the `unresolved status` phrase. The
matrix was corrected and that diagnostic phrase preserved; actual refusal and
unchanged provider counts were never relaxed. Failed owned units were stopped and
their exact cgroups verified absent, not accepted as successful validation.

## Required current-source full validation

Scratch `/tmp/unreal-agent-m21-validation.Un0vCj`;
unit `unreal-agent-m21-validation-20261028T015008Z.service`.

`make check`, `make build`, **`make test` (race, both Go modules)** all passed:
**48 passing packages + 3 no-test packages = exactly all 51 packages** from both
module package lists. Before/after/current acceptance manifests match. Unit was
success/exit 0/MainPID 0; no processes remained in its observed cgroup. The exact
unit was stopped and its exact recorded cgroup was absent. Logs, manifests and
point-in-time changed implementation files are archived below.

## Current-binary owned private frontend replacement

Both proofs reran on the newly built bytes, with explicit private tmux servers,
HOME/state/workspaces and owned fake providers/processes:

- Lifecycle proof `/tmp/u21.mmgjcv`, unit
  `unreal-agent-m21-lifecycle-tmux-20261028T015459Z.service`: real model/Bash detach,
  three frontends/two replacements with draft preservation, scoped aborts,
  canonical shutdown, actual host exit 0, read-only offline reconciliation and
  exact TTY restoration. All 13 cleanup/result flags true.
- Reconnect proof `/tmp/u21.pxE5vH`, unit
  `unreal-agent-m21-reconnect-tmux-20261028T015459Z.service`: both pre-admission and
  accepted/lost-ack scenarios, original identity/payload once canonically, exactly
  two legitimate provider calls per scenario, sentinel once, draft/details/manual
  scroll restoration and gap handoff. All four cleanup flags true in each case.

Both unit exit codes were 0/MainPID 0; exact observed cgroups were empty, then
absent after stopping exact units. These test UI replacement, not new generic
runtime migration. The M1.6 retained-background process proof and limitations
remain separately documented; no remote-billing cancellation claim is added.

## Atomic installation

Both binaries were installed through temporary files in `~/.local/bin`, mode
0755 and atomic rename, then byte-`cmp`ed against `bin/`. Both passed. The owned
frontend replacements above satisfy frontend verification without touching any
user frontend or production worker/session.

| Binary | Built/installed SHA-256 |
| --- | --- |
| runner | `9ba78574364d2f9d7d312eedaec17b39a5f3b5c38191911ffc251abfc16411de` |
| TUI | `33ddcb8add0a68c93adf3f8721105e068518c5553a71878c37c0702e878fa014` |

[Full evidence archive](durable-recovery-entry-m21-20261028-evidence.tar.gz)
(SHA-256 `971bfb7b7fd606e978f7d12fb5c29fb0e39af1bd61ffb18c5bfdb046037e6dda`).
Process-test temporary fixture state is deleted by Go test; its source/assertions
and verbose results are retained, not misrepresented as archived raw fixture data.

Next: M2.2 phase-specific read-only image recovery, then every later slice in
`goal.md`. This checkpoint does not complete M2 or the roadmap.
