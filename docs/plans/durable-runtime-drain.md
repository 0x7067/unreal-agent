# Scheduled runtime replacement

Current `/reload` and attached `/upgrade` use one host-owned graceful handoff:
stage/probe once, queue new ordinary input, submit upstream `StopWhenIdle`, finish
current work, join owners/process/output, verify cleanup and writer-lease release,
then launch the staged replacement. There is no one-second polling loop or
`TryQuiesce` RPC. See the [reload contract](hot-reload.md).

Busy work continues naturally. Scoped graph acceptance/rejection/cancellation and
helper controls remain available for the retiring generation. Graph results are
never implicitly accepted; background reports do not release native ownership.

The client request has a bounded wait, not a guaranteed replacement deadline.
Cancellation before admission can stop scheduling; after admission, timeout,
disconnect and `/cancel-upgrade` end only the wait. The host owns completion and
reports the eventual generation through ordinary status/reconnect. No provider or
shell operation is interrupted/replayed to upgrade, and no candidate is selected
by the model or an attached frontend.

This behavior is advertised through `runtime_replacement_v2` and
`runtime_drain_v2`; public frame/payload versions remain 1. The old
`runtime_drain_v1`/idle-only capability is not advertised with changed semantics.
The private worker protocol is 8. Preflight, cleanup uncertainty and candidate
startup failure remain fail-closed, with no automatic downgrade.

The [M3.3 verification](../verification/durable-runtime-drain-m33-20261028.md)
is historical evidence for the earlier polling policy and cancel-before-idle
contract, **not** verification of this implementation. Current validation must
include scheduled model/tool drain, explicit graph intervention, retained helpers,
canceled waits, exact old process/output joins, canonical receipts and lease release.
