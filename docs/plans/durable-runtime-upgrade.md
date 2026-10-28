# Resident runtime upgrade — M3.2

The durable session is the agent; the process running it is replaceable.

`host --runtime-replacement` opts into a private, same-user runtime control
capability. `/builds` checks the host's configured trusted worker executable;
`/reload` requests an **idle** replacement at the current generation. Clients
cannot supply executable paths. The existing Host owns immutable staging/probe,
quiescence, delivery fencing, retirement, actual process wait and lease release.
It remains the only replacement authority; translators/context building stay
pure and coordinator commit-before-dispatch is unchanged.

Attach envelope v1 adds an opt-in `runtime` variant with payload version 1,
correlated request/connection identity, bounded text and strict fields. Legacy
clients never receive it unsolicited; legacy status fields are unchanged. The
capability is absent by default. One connection-owned control and one matching
cancel are bounded; disconnect cancels only that control, not session work.
`cancel_requested` confirms admission, **not** retirement or cleanup. Lost replies
are uncertain and are never automatically retried or treated as input receipts.

The client monotonically advances lifecycle generation from validated current
status/correlated replies. The UI keeps retained controls on their original
scope and requires reconnect before issuing new lifecycle controls after a
runtime generation change; no saved control is silently rebound. View/draft/
selection survive runtime metadata updates. Active work returns `blocked`, not a
forced reload. A stopped replacement is not rolled back automatically. Deferred
active-work policy is M3.3, candidate failure/state compatibility M3.4, and host
replacement M3.5. None is implied by this idle bridge.

Local upstream comparison: `a5f3fd13032737142916523ae4344c392292f9d5`
(no fetch). It has no resident/attach host layer; this private transport extension
is a surfaced local divergence, not a change to shared manager/store interfaces.
Its pure translator/context and committed-operation dispatch invariants remain.

Current proof: [M3.2 verification](../verification/durable-runtime-upgrade-m32-20261028.md).
