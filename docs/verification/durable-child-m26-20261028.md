# M2.6 accepted — durable independent background children

Accepted 2026-10-28 for the narrow [contract](../plans/durable-child-recovery.md).
**16/27 slices complete. M2.7 and every M3/M4 slice remain; this is not roadmap
completion.** Private worker protocol 6, session format 2. No user frontend or
production session/worker was touched. No commits or fetches were made.

## Required current-source validation and installation

Scratch `/tmp/unreal-agent-m26-validation.Kd0lOX`, named bounded user unit
`unreal-agent-m26-validation-20261028T051117Z.service`:

- `make check`, `make build`, `make test`: PASS. Both Go modules tested with race.
- Exact `go list` package union: **52 = 49 passing + 3 no-test**, no duplicate,
  omitted or unexpected package rows. Both module lists rechecked at acceptance.
- Full tracked/untracked relevant-source manifests **before = after = current**;
  phase logs and `acceptance.json` retained. Documentation/evidence outside the
  manifest is excluded deliberately, not silently counted as source validation.
- Observed success, exit 0, MainPID 0; exact recorded cgroup absent before stop.
  Exact named unit stopped and recorded cgroup still absent.
- Both binaries installed by same-directory temporary file + atomic rename to
  `~/.local/bin`, followed by **individual cmp** with each built binary.

| Binary | Built and installed SHA-256 |
|---|---|
| unreal-agent-runner | `0e2e820dfd2d81a0bcad8a11e6602faee4a25041483789343ae35d5e6a8685da` |
| unreal-agent-tui | `55521b44f2bf0a04dfb934b532f9a9c553c3d51e3e5daf209bfef3416aa15d27` |

## Actual process and authority evidence

Race-focused package run: `/tmp/unreal-agent-m26-focused.MtlUo9`, unit
`unreal-agent-m26-focused-20261028T050948Z.service`, PASS. Six affected packages;
actual child process tests also reran in the full suite.

`TestIndependentChildSurvivesParentSIGKILL` passes four cuts:

1. Waiting for the same blocked independent child after real parent SIGKILL.
2. Child canonical answer committed while the parent is dead.
3. Parent operation head saved complete before the final ToolCallStatus. This
   window is constructed with real canonical APIs, not claimed as a live hook.
4. Authenticated current-capability child stop, explicit uncertain parent state,
   no invented answer/relaunch, and fail-closed no-mutation replacement.

Assertions include stable child/session/link/input/result identity, one child
provider request and one logical parent result, rotated parent capability,
model-issued stale stop with no child mutation, ordinary foreground abort refusal,
terminal retained-child rebinding, and no false parent global drain receipt.

`TestRealOwnerLaunchSurvivesParentProcessGroupKill` launches through the real
owner inside an owned disposable parent process group, kills that exact group,
then reattaches to the same child host. This catches process-group inheritance
that the externally launched canonical fixture alone would miss. Its cleanup
uses authenticated shutdown, a kernel pidfd bound to that host incarnation,
canonical child effect-drain evidence, and the containing exact unit drain. It
**does not** pretend to recover the killed parent's original Cmd.Wait capability.
The other child fixtures retain their actual launch Cmd.Wait and socket-reader
joins, as do the owned parent Hosts.

Strict codec, owner-fence, lost/malformed reservation, capacity, conflicting
intent, stale authority, answer-attribution, scan/text bounds and terminal-control
classification tests pass. Task/default lifetime and messages are explicitly
unsupported. Legacy remote owners and unresolved structural/lifecycle controls
remain fenced. Child process/transport loss is not cleanup proof.

The successful focused unit was observed exit 0/MainPID 0, then stopped with its
recorded exact cgroup absent. Earlier rejected diagnostic candidates were stopped
and drained; their failures are not acceptance evidence. One early foreground
multi-cut diagnostic was tool-canceled. Its surviving **private** child was later
shut down through its authenticated endpoint; canonical child drain and the exact
host pidfd exit were observed (`additional/unreal-agent-m26-canceled-fixture-cleanup.log`).
Its lost original Cmd.Wait is not claimed. All subsequent process proofs ran under
bounded named user units with observed exact cgroup drain.

## Fresh-byte owned private frontend replacement

Read and followed `.claude/skills/verify-unreal-agent-tui/SKILL.md`.

- Lifecycle `/tmp/u26.JlAcEY`, unit
  `unreal-agent-m26-lifecycle-tmux-20261028T051259Z.service`: PASS, **13/13 cleanup
  assertions true**, actual frontend replacement during model/Bash work,
  canonical abort/shutdown, exact TTY restoration, host return 0 and owned joins.
- Reconnect `/tmp/u26.oCaCBF`, unit
  `unreal-agent-m26-reconnect-tmux-20261028T051259Z.service`: PASS, both admission
  cuts, **4/4 cleanup assertions each**, one stable canonical external input,
  two legitimate provider calls, restored private frontend view and no errors.
- Built binaries and both drivers hashed before proof and individually checked
  unchanged afterward. These are fresh runs, not reused M2.4/M2.5 evidence.
- Both units observed success/exit 0/MainPID 0, exact recorded cgroups absent,
  then exact units stopped and cgroups absent. No user tmux frontend was replaced.

## Archive and qualification

[Evidence archive](durable-child-m26-20261028-evidence.tar.gz), SHA-256:
`f31c01f9abf35980c11fdee0e46b30a2f7539f7668552b3ec5bb15bdae8c6bc9`.
Includes validation manifests/logs/install hashes, focused test log, private proof
screens/state/assertions/joins, input hashes and exact unit drain observations;
socket objects are excluded. Deleted raw Go fixture directories are **not** claimed
as archived. The local upstream comparison and all remaining limitations are in
the contract. No generic remote replay, exactly-once execution/billing, arbitrary
process-tree sandbox, or universal cleanup/recovery claim is made.
