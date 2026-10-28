# M1.6 internal foundation checkpoint — NOT slice completion

Public lifecycle controls are still disabled. See the [in-progress design and
remaining critical path](../plans/durable-lifecycle-controls.md). This checkpoint
validates the internal abort/cleanup-receipt and common writer-fence layer only;
it does not prove end-to-end retained-background abort/shutdown.

[Archived evidence](durable-lifecycle-core-m16-20261027/):

- Fresh `make test`, `make check`, `make build` all passed on matching
  before/after/finish source manifests. Test packages equal both module lists:
  **49** (one new `cmd/internal/lifecycleguard` package).
- Focused lifecycle/inbox safety/cleanup race tests passed count 3; real Bash
  foreground-abort tests passed count 5. The complete repository race suite also
  includes these tests and the preexisting adversarial/reconnect cases.
- Both binaries were atomically reinstalled to `~/.local/bin`, byte-compared to
  `bin/`, and help-smoked. SHA256:
  - runner `97dbf1ae16ba96f8c00a551ebd22041cf185adaec8d7a187bf6ff643ff83be64`
  - TUI `6036ce2403310aa3bf06c3ffdb675d9014af47711dc7c262415969351449ac21`
- The real private-tmux M1.3/M1.4 driver passed both pre-admission and accepted-ack
  cuts against these binaries, including actual frontend replacement and restored
  draft/selected expanded tool/manual scroll. Canonical snapshots and all-true
  owned cleanup records are archived. This is a reconnect regression proof, not
  a demonstration of not-yet-exposed lifecycle actions.
- `unreal-agent-m16-core-validation-20261027T233447Z.service` and
  `unreal-agent-m16-core-tmux-20261027T234122Z.service` exited zero, were stopped,
  and their exact recorded cgroups were confirmed absent.
- HEAD is unchanged at `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`.
  No commit was requested or made. No user/production frontend was restarted.

Do not promote generic remote uncertainty, process-group cleanup, primitive
sender epilogues, EOF or legacy `Host.Close` to stronger cleanup guarantees.
The runtime/host proof merge, generation-scoped public SDK/TUI controls, actual
retained-background process tests, timeout/takeover tests and final validation
remain required before M1.6 is complete. Subsequent code changes require fresh
relevant final validation and reinstall; these logs attest only this checkpoint.
