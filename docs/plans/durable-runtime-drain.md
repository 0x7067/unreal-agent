# M3.3 — explicit natural-drain replacement policy

Opt-in resident runtime replacement adds a separate `runtime_drain_v1`
capability. A connected supported frontend may request `/upgrade`; `/reload`
remains immediate idle-only. `/cancel-upgrade` targets the exact current
connection-owned request, not the session or its operations. No executable path
is accepted. Payload version1 remains strict; legacy clients never receive
unsolicited queued runtime replies.

The approved policy is **finish naturally, then replace at proven quiescence**.
No model/read/shell/graph operation is checkpointed, interrupted, killed, retried,
or implicitly accepted to upgrade. Busy or persistence-unconfirmed work is
reported queued/deferred. Completed graph candidates remain unaccepted and keep
blocking the unchanged Host quiescence proof. Every replacement attempt uses
existing immutable staging, quiescence, input-delivery fence, retirement, actual
process wait and session lease release. The trusted installed candidate is
rechecked at the eligible handoff; a prior `/builds` is observation, not a frozen
selection promise.

One connection-owned request lasts at most45 seconds and attempts at most46
existing Host reloads, at one-second busy intervals. Ordinary input admission is
independent; sustained new work may prevent reaching idle. Generation is checked
before every attempt. Cancellation/deadline/disconnect ends only the upgrade;
late retirement is still handled by the unchanged Host fences, and no ambiguous
outcome is relabeled successful. Cancel acknowledgment is admission only; the
original request must settle. There is no forced reload or automatic retry.

The UI shows `Upgrade queued · waiting for natural idle drain; current work
continues`. Saved lifecycle controls are not rebound across runtime generations;
reconnect is required before fresh lifecycle controls. Runtime request IDs are
not durable input receipts. Source scopes/ownership and session/operation formats
are unchanged. Local upstream `a5f3fd13032737142916523ae4344c392292f9d5`
(no fetch) has no resident/attach layer: this is a surfaced local optional
transport policy, not a change to shared manager/store APIs, translator purity,
context building or coordinator commit-before-dispatch.

Accepted M3.3 evidence: [verification](../verification/durable-runtime-drain-m33-20261028.md).
M3.4 candidate/state failure and M3.5 resident-host transfer remain separate slices.
