# Durable local reconnect and retry (M1.3 / M1.4)

The session, ingress journal and host own execution. A frontend's view file is
not canonical history, a worker checkpoint, or authority to dispatch anything.
This extends the local optional attach-v1 protocol only; the upstream coordinator,
inbox contract and conservative unresolved-operation recovery guard are unchanged.

## Commands and acknowledgment states

Start the host explicitly as described in [headless hosting](durable-headless-host.md),
then `unreal-agent-tui attach -runtime-directory ... -session-directory ... -session ID`.
Use F5 or `/reconnect` after transport loss. Reconnect sends no input. Use
`/retry ID` (or `/retry` for the oldest retained input) to retry the original
normalized external payload and ID, not a new message with a new UUID.

The local echo distinguishes submitting, durable ingress acceptance, canonical
session persistence, definite rejection, and uncertain delivery. Canonical
persistence wins over late acknowledgments/errors. A capacity rejection is never
acceptance. A pending host payload wins over conflicting local retry data; the
conflicting retry is disabled rather than changing the visible accepted input.
A persisted input does not get another local echo or RPC when `/retry` is used.

Each new attached submission first synchronously saves its stable ID and original
payload locally, before it enters the sender FIFO. Save failure retains the draft
and sends nothing. Lost acknowledgment retains the original logical submission;
it never restores a draft that encourages generating a new ID. The journal,
not this local file, supplies accepted-input durability and deduplication.

## Cursor protocol and race boundary

An optional `Cursor` contains host incarnation, published revision R and canonical
history head H. The host keeps **256 fixed-size R/H checkpoints**, not an unbounded
event log. A current or retained exact checkpoint selects a canonical delta
H+1..captured-H. A complete current model/status/pending mirror accompanies the
delta: intermediate transient status/pending events are coalesced, not replayed.
Selection of the base, capture of the published head, and watcher subscription
happen under one publisher mutex. Concurrent commits after that capture are live
queued updates, never silently folded into the fixed snapshot.

Expired, invalid/ahead, or foreign-incarnation cursors receive an explicit full
snapshot fallback code. Old strict v1 hosts that close on the optional field get
one fresh reconnect attempt (`cursor_unsupported`). Fresh older clients remain
compatible: optional fields are omitted unless requested. A delta's base must
exactly match the client's requested checkpoint; clones cannot carry a base.
The existing snapshot limit remains 256 MiB / 30 seconds, frames 1 MiB, pending
mirror 64 inputs / 8 MiB, and output queue 256 frames / 8 MiB.

The frontend advances its cursor only after applying a validated canonical update.
New connection epochs fence stale queued reader results. One reader owns each
connection; reconnect never creates two readers on the new stream. Exact duplicate
live updates within a 256-fingerprint window are skipped. Changed or evicted
revisions cause explicit discontinuity/resnapshot, never silent acceptance.

## Private local state

`-client-state-directory DIR` selects the separate private view directory. Default:
a `clients` sibling of the selected session directory (also the normal XDG state
location). Files are namespaced by SHA-256 of the canonical session-directory path,
NUL, and session ID. A private stable advisory lock fences concurrent writers of
the same view slot, including disconnected frontends. Close the previous frontend
before replacing it, or deliberately use a different client-state directory.
This lock confers no host/session authority and never replaces the host leases.

The version-1 DTO contains draft, unconfirmed stable external input identities,
manual scroll/anchor, selected tool identity, expanded details/pane state and text
selection. It contains **no canonical history, credentials, connection token,
worker state, operation checkpoints, or executable control messages**. A new
frontend always obtains fresh authoritative history before restoring preferences;
it cannot use an untrusted local history cache as a cursor base. In-process F5
reconnect can use the already applied canonical view as its delta base.

Paths are walked descriptor-relatively without following symlinks; the final
directory must be same-UID 0700 and files same-UID regular 0600. JSON is strict and
versioned, with bounded text/offset/ID/count/aggregate/file sizes. Saving uses a
private temporary, file sync, atomic rename and directory sync. New ancestor
entries are synced. Unsupported/corrupt/unsafe local state refuses startup with
repair/new-directory guidance, without submitting an input. A valid retained
input is merely visible retry data: loading it never submits. Explicit `/retry`
can send only an external text input, never a control envelope. Drafts/prompts
are sensitive; do not place the directory in a shared or untrusted workspace.

View changes checkpoint about once per second and on graceful exit; the stable
submission identity is saved synchronously before dispatch. A sudden frontend
kill can lose the most recent unsaved view changes, not an already saved retry
identity. Scroll uses a rendered text anchor where it is still present, otherwise
a clamped offset. Text selection is presentation only and can be clamped when
layout/content changes. A failed/corrupt view never authorizes execution recovery.

## Scope

One active authenticated client per host/session is still the selected policy.
This work adds no remote listener, automatic resend, host auto-start, public
control/reload API, activity/operation checkpoint view, or crash replay permission.
Ordinary Ctrl+C detaches only the frontend. Deliberate abort/shutdown controls and
stalled-client adversarial verification are the next M1 slices, not implied here.
