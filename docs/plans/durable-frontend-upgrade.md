# Compatible frontend replacement (M3.1)

The resident host/worker and durable session do not belong to the TUI process.
Use the attach frontend for upgradeable interactive work. Local draft/view state
belongs to the explicit client-state directory; reuse that directory on reattach.

## Explicit install/reattach workflow

1. Verify a trusted candidate's provenance and compatibility against a private
   fixture. Do not execute workspace-selected binaries or infer compatibility from
   a filename. Keep the accepted old pair and a known-good client-state directory.
2. Build/check/test the candidate using repository policy. Atomically install
   **both** runner and TUI using same-directory temporary files and renames, then
   individually `cmp` each installed file with its accepted build. Replacing files
   does not replace the running worker's immutable staged executable.
3. On the owned attach frontend, Ctrl+C detaches (it is not session shutdown).
   The frontend persists draft, focus, selection/scroll and pending-input state.
   Do not send a new input identity merely because an acknowledgment was lost.
4. Launch the new TUI explicitly against the same runtime/session directories,
   session ID and client-state directory, e.g.:

   ```sh
   unreal-agent-tui attach -runtime-directory "$PRIVATE_RUNTIME" \
     -session-directory "$PRIVATE_STATE" -session "$SESSION_ID" \
     -client-state-directory "$PRIVATE_CLIENT_STATE"
   ```

5. Observe the same host incarnation, worker generation/build, durable input and
   active work. A new frontend connection token is expected. An incompatible
   client must fail without stopping or mutating the session. Reattach a compatible
   client; do not force an execution replay or shut down the worker to upgrade UI.

This is frontend-only replacement. It does not authorize active runtime/host
migration, generic forced reload, lease takeover or a schema downgrade.

## Executable proof and bounds

`durable_upgrade.py` derives the existing trusted lifecycle fixture with an exact,
checked transformation, records its expanded source/hash, and selects explicit
frontend paths A/B rather than duplicating the fixture. Three distinct executable
hashes are required. A and B in the accepted proof are the same accepted source
with different Go link build IDs: this proves **compatible build replacement**, not
an untested feature release. The negative binary is an actual TUI compiled with a
private Go overlay changing only attach protocol 1 to 999; source files are unchanged.

The proof uses an owned private tmux server, fake loopback provider and one private
host/session. It replaces A with B during a blocked model, and B with B during Bash,
restoring draft/view and observing one Bash effect. It additionally observes the
exact worker PID through all threads of its known owned host, a bound pidfd, the
same session-lease inode/descriptor, and failed exclusive lease probes. Worker exit
is observed only after the owning host's actual wait. The incompatible frontend's
wrapper retains its actual child wait and TTY receipt; its nonzero exit does not
interrupt A, the host, draft or provider request. The current rejection diagnostic
is `EOF`, not an invented negotiated version explanation.

Thread/descriptor observations are bounded to 256 each and Linux-specific. No PID
is guessed or used as replacement authority. This is not a host-transfer proof.
All fixture processes belong to bounded named systemd units and private tmux;
never run this recipe against a user frontend or production session.
