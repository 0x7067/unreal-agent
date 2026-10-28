# Runtime candidate failure and state compatibility (M3.4)

The opt-in resident retains its ingress owner and control connection after a
latched worker failure. It publishes `stopped`, never successful replacement or
an automatic old-worker fallback. The operator must stop/join that resident,
correct the trusted executable/state problem, and explicitly restart. Existing
non-opt-in failure exit and successful deliberate shutdown remain unchanged.

A replacement request is not delivery of queued input. Input durably admitted
while delivery is fenced survives failure and journal reopen. A healthy new host
uses the same stable ID/body, canonical deduplication, and reviewed recovery gate.
Stale replacement generation is refused; a timeout is never transfer authority.
Resident admission after stopped is unavailable, not a false acceptance.

Session v2 and existing supported operation formats are unchanged. Immutable local
upstream fixtures preserve readable historical bytes; unfinished synthetic/unknown
operations do not gain runtime permission. V1, future headers/checkpoints and
unsupported recovery states refuse before provider dispatch without changing
canonical bytes. No migration, new format or interrupted migration is introduced.
The existing first-write subprocess test appends current-format B history and
fails: the unconditional no-fallback latch still applies. This is NOT a claim to
have tested a hypothetical future-format migration or storage power loss.

Compared locally with upstream `a5f3fd13032737142916523ae4344c392292f9d5`
(no fetch): its fixed compatibility/refusal and commit-before-dispatch boundaries
are preserved. The opt-in resident stopped-control lifetime is an explicit local
extension; upstream has no corresponding resident runtime-control protocol.
