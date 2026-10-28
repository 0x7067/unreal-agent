# M2.2 — narrowly supported image recovery

**Complete.** [Current-source full validation, installation, owned private frontend
replacement and archived evidence](../verification/durable-image-recovery-m22-20261028.md)
passed on 2026-10-28. M2.3–M4 remain unfinished.

## Reviewed cells

Only `view_image` **version 1**, status **ready or awaiting**, valid configuration
and **no recorded result** may recover. Transient source chunks/read cursor and
compute state are not serialized and are intentionally discarded. The resumed
operation rereads the **current file** from the beginning and computes a new
bounded result. It may differ if source changed; no old-content identity, inode
or digest promise is invented. Missing or corrupt source produces a visible
failed image result, not a success reconstructed from uncommitted old bytes.

A committed completed/failed result stays inert and is rendered from its
versioned state without rereading its source. Unknown versions, canceling,
malformed or result-bearing nonterminal state remain refused. Skill offsets,
Bash, remote jobs and generic model/compaction restart do not inherit permission.
Existing canonical/journal lifecycle fences, unproven remote ownership and
exclusive old-owner drain/lease rules still apply independently.

Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` was compared without
fetching: its v1 image handler already supports ready/awaiting restart with fresh
read state. That pure state machine, image schema, bounds and tool translator are
unchanged. Only the new private entrypoint allow-list is expanded; this policy
and changed-source semantics were surfaced with the user before implementation.
No translator or context-builder I/O, session-format, mandatory manager or wire
changes are introduced.

## Focused boundary evidence

`workerhost.TestM22OwnedImageRecovery` has six owned process cases:

1. Initial-ready before read, then source changes from 2x3 to 5x7: returns 5x7.
2. Actual old read/compute complete but final update deliberately uncommitted,
   canonical awaiting state, then source changes: returns 5x7, not old 2x3.
3. Same actual uncommitted read, then source removed: failed result, no image.
4. Same actual uncommitted read, then source corrupted: failed result, no image.
5. Actual completed v1 result committed before crash, source removed: reuses 2x3
   content; the existing completed-history format remains readable.
6. Future image version 2: refusal before provider requests or canonical mutation.

The producer is a **controlled durable-boundary fixture**, not a claimed runtime
internal fault hook. Awaiting cases use the actual local operation manager and
real read/image computation to prove the original read completed. The parent
kills only its exact owned Cmd, verifies SIGKILL WaitStatus, joins it and checks
leases free before recovery. Actual workerhost/runtime then restores the original
operation ID and canonical tool call. Real provider wire requests contain an
image on success, an error and no image on source failure. Exactly one terminal
original operation and one model continuation are observed. A second actual
runtime makes no extra request/result or alteration to the committed prefix;
only its own exact stop/drain tail is allowed. Both owner leases are released
only after actual joins. No PID-based reattachment is used.

`recoverygate` tests exhaust the enabled type/version/status matrix, malformed
state and result-bearing nonterminal refusals. The M2.1 real coordinator/manager
admission-counter and terminal-exclusion test remains applicable; canonical
completion is not mislabeled as direct admission instrumentation.

Acceptance is recorded in the linked verification: both-module make check/build/test,
exact 51-package union and matching manifests, actual unit/cgroup drain, new-built
private frontend replacement, atomic installation and byte comparison of both
binaries. Continue M2.3–M4; this is not roadmap completion.
