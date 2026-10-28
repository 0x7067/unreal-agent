# Controlled idle/drained host replacement (M3.5)

A trusted launching supervisor, not a frontend, owns host process replacement.
The first supported strategy is **natural idle quiescence → worker retirement and
actual wait → admission seal/join → retained ingress owner → explicit host stop
and actual process wait → B acquires both OS lease fences → fresh client attach**.
Active-worker transfer, forced checkpointing, killing work to upgrade and automatic
host spawning/fallback are unsupported.

## Explicit workflow

1. Keep the original trusted host process handle/startup receipt and independently
   verified trusted A/B executables. Preserve session/runtime/state directories.
   Detach the ordinary frontend; this is an exclusive control-client workflow.
2. On an opt-in `host --runtime-replacement`, run the trusted installed runner:
   ```sh
   unreal-agent-runner host-retire --runtime-directory "$RUNTIME" \
     --session-directory "$STATE" --session "$SESSION" \
     --expected-incarnation "$OWNED_OLD_INCARNATION" --expected-generation "$GEN"
   ```
   Both identity fences are mandatory. A wrong incarnation cannot retire another
   owner. Missing capability or active/unpersisted work refuses; no hidden retry.
3. Require success and the correlated **`drained`** receipt, not merely a stopped
   footer, cancellation admission, EOF, idle observation, or a timeout. It means
   real quiescence/worker wait and pre-seal admission joins, NOT old-host exit.
   Original acknowledged pending ingress remains durable; new admission is sealed.
4. Explicitly terminate only the original supervisor-owned host and **actually wait
   for that exact process**. Do not infer transfer authority from its numeric PID,
   elapsed time or an available session lease. Its ingress/frontend lease stays
   held until cleanup. If wait is unconfirmed, leave B unstarted.
5. Explicitly start trusted B with the same stable session/state/runtime namespace.
   Normal startup must acquire the host/ingress and worker/session leases; do not
   unlink locks, steal descriptors, copy journals, or bypass recovery validation.
   Attach a fresh frontend/client; old-incarnation control/cursor authority is not
   rebound. B recovers supported original-ID input from its durable journal.

## Failure recovery and compatibility

`blocked`: let work finish, then explicitly retry retirement. Request cancellation
or timeout is unconfirmed, not permission to launch B. If retirement may have
committed, inspect and use explicit owned host shutdown/wait; do not assume a live
worker was transferred. Unknown/unsupported checkpoints continue to refuse startup.
A failed B startup leaves durable input and stopped service; fix its trusted
configuration/state compatibility and explicitly start a supported B. No automatic
A downgrade occurs. Avoid concurrent controlling clients; the normal single-client
policy still applies. All signals in verification target only private owned
subprocess handles; no user frontend or production owner is touched.

## Shared contract review

Local upstream `a5f3fd13032737142916523ae4344c392292f9d5` (no fetch) has the existing
quiesce/actual-wait/lease replacement invariant but not this resident protocol.
`Host.Reload` and new `RetireForHostReplacement` share the same fenced handoff;
retire skips candidate probing/launch, then seals/joins admissions. Distinct
`host_retire_v1` gates the new `retire-host` runtime variant; attach envelope and
runtime payload stay v1, no persisted-format migration. `drained` is explicitly
terminal without a running build/generation. The CLI neither signals nor starts a
host, and ordinary/default non-opt-in behavior is unchanged. This is an explicit
local additive divergence, not generic active-owner migration.
