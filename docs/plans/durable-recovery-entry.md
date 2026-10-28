# M2.1 — exclusive, narrowly classified recovery entry

**Complete.** [Current-source full validation, actual process/admission evidence,
private frontend replacement and atomic installation](../verification/durable-recovery-entry-m21-20261028.md)
are accepted. Installed binaries include these narrow cells; M2.2–M4 are unfinished.

## Enabled source cells and unchanged fences

The pure `cmd/internal/recoverygate.Validate` classifier replaces the worker's
blanket nonterminal refusal and also fences standalone runner resume before
provider construction. It does no I/O or dispatch and does not mutate inputs.
Both entrypoints already hold their writer/parent authority as appropriate;
canonical lifecycle, journal-only lifecycle and unproven remote-owner guards
remain independent prerequisites, not errors the classifier may discharge.

| Type/version/status/state | Decision |
| --- | --- |
| Completed/failed/canceled | Inert history, never permission to redispatch; other history decoders retain their supported-version checks |
| `value` v1, ready, valid state and original nonempty ID | Pure initial intent may finish; its recorded tool/result implementation must independently be compatible |
| `view_image` v1, ready, valid state with no result | Initial read may repeat using the current source; this does not pin old file contents |
| Every other nonterminal type/version/status/state | Refuse with deterministic bounded ID/type/version/status diagnostic |

In particular ready Bash does **not** prove never dispatched. Shell v3/v4,
image awaiting/canceling, skill offsets, unknown/future operations and all
nonterminal remotes stay refused. No legacy replay is inferred from terminal
remote tickets; missing actual cleanup remains fenced. Unfinished model and
compaction request identity/retry accounting are later M2.3/M2.4 work, not claims
of this initial entry slice. No wire/session schema, mandatory manager API,
translator or context-builder I/O changes are made.

Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` was compared without
fetching. Its pure value/read-only reducers and terminal dispatch exclusion are
preserved. This new private-entrypoint allow-list and standalone tightening were
raised with the user. Pure value is valid for subagent-to-parent results; the
resident root's parent-side remote translator cannot consume that result type.
Therefore the real resident intent fixture uses **ViewImage**, not an invented
value/tool pairing. This initial image cell overlaps M2.2 deliberately; later
image phases remain disabled until their own adverse-boundary evidence.

## Focused evidence and its limits

`workerhost.TestM21OwnedProcessLossAndRestart` uses a controlled owned producer
process with real durable APIs. It holds both leases and ingress, signals only
after successful committed boundaries, and is killed by the parent through its
exact `Cmd`, then joined with an actual SIGKILL `WaitStatus`. This is **not** an
instrumented kill inside the runtime decision loop. It establishes durable
journal-only FIFO input, session-persisted/unacknowledged input, and canonical
initial image intent before any operation dispatch. Actual workerhost/runtime
restart reconciles original identities, returns a real image, acknowledges
journal input and excludes competing ownership. Second restart preserves the
committed prefix and may append only its fresh scoped execution-drain receipt at
the causal quiescence barrier. Cleanup controls are not external input duplicates.

The process test counts canonical completions, **not** precommit manager calls.
`recoverygate.TestRecoveryDispatchCounterAndTerminalExclusion` separately wraps
an actual local manager in an actual coordinator recovery harness: original
pure value ID is admitted exactly once in recovered generation one, zero times
after terminal replay, with one then zero model calls. It checks the canonical
terminal original operation remains present once. This uses the legitimate
subagent-to-parent translator and performs no remote message delivery.

`agentrunner.TestRecoveryGateStandaloneRefusesShellBeforeProviders` proves a
standalone runner cannot bypass the gate, construct providers, mutate canonical
history or produce its Bash sentinel. Both authority leases release after refusal.
The prior shell intent/effect ambiguity characterization still refuses before
provider construction or environment access, with richer policy diagnostics.

## Acceptance and next work

The required both-module full pipeline passed on unchanged source, exact 51
packages, with actual unit/cgroup drain. Both current-binary private tmux proofs
passed, and both binaries were atomically reinstalled and byte-compared. The
linked verification record contains paths, hashes and corrected failures.

Continue M2.2–M4. Image awaiting/canceling, skill, Bash and remote recovery remain
disabled until their own reviewed matrix cells and adverse-boundary evidence.
