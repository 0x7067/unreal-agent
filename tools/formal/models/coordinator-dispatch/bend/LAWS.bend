import Base
import ./model.bend as M
import ./product.bend as P
import ./certificate.bend as C
import ./witnesses.bend as W

law commit_before_every_add:
  for +events: List<&2, M.Event>
  for +s: M.State
  {M.traceCommit(events, s) == True{} : Bool}

law no_terminal_or_stale_add:
  for +events: List<&2, M.Event>
  for +s: M.State
  {M.traceCheckpoint(events, s) == True{} : Bool}

law no_add_before_status_commit:
  {M.audit(M.Add{}, M.Append{}) == M.NoAdd{} : M.Audit}

law stale_replay_snapshot_not_dispatchable:
  for latest: M.Status
  {M.audit(M.Add{}, M.Replay{Some{latest}}) == M.NoAdd{} : M.Audit}

law terminal_recovery_not_added:
  {M.run([M.Crash{}, M.Advance{}, M.Advance{}, M.Advance{}, M.Add{}], M.Idle{M.Terminal{}}) == M.Idle{M.Terminal{}} : M.State}

law awaiting_recovery_uses_latest:
  {M.run([M.Crash{}, M.Advance{}, M.Advance{}, M.Advance{}], M.Idle{M.Awaiting{}}) == M.Adding{M.AwaitingLive{}} : M.State}

law visible_status_error_recoverable:
  {M.run([M.ErrorVisible{}, M.Add{}, M.Crash{}, M.Advance{}, M.Advance{}, M.Advance{}], M.Append{}) == M.Adding{M.ReadyLive{}} : M.State}

law invisible_status_error_retranslates:
  {M.run([M.ErrorInvisible{}, M.Add{}, M.Crash{}, M.Advance{}], M.Append{}) == M.Translate{} : M.State}

# Local audit counterexamples, NOT reachability proofs for TLA mutation traces.
law skip_commit_audit_rejected:
  {M.commitSafe(M.Added{False{}, M.Ready{}, M.Ready{}}) == False{} : Bool}

law skip_overlay_audit_rejected:
  {M.checkpointSafe(M.Added{True{}, M.Ready{}, M.Awaiting{}}) == False{} : Bool}

law skip_terminal_guard_audit_rejected:
  {M.checkpointSafe(M.Added{True{}, M.Terminal{}, M.Terminal{}}) == False{} : Bool}

# Rich two-operation independent-field transition model. Unlike the original
# typestate laws above, these start from Init and establish a relational
# reachable-state invariant, not safety from every representable state.

law product_initial_invariant:
  C.Invariant(P.initial())

law product_invariant_preserved:
  for +e: P.Boundary
  for +s: P.Core
  for inv: C.Invariant(s)
  C.Invariant(P.next(P.fixed(), e, s))

law product_invariant_implies_safety:
  for +s: P.Core
  for inv: C.Invariant(s)
  {P.safe(s) == True{} : Bool}

law product_invariant_relates_partial_cursor:
  for +s: P.Core
  for inv: C.Invariant(s)
  {P.consistent(s) == True{} : Bool}

law product_every_prefix_safe:
  for +events: List<&2, P.Boundary>
  {P.allSafe(events, P.initial()) == True{} : Bool}

law product_arbitrary_crash_trace_safe:
  for +events: List<&2, P.Boundary>
  for +crashes: Nat
  {P.safe(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes}))) == True{} : Bool}

law product_bounded_crash_trace_safe:
  for +events: List<&2, P.Boundary>
  for +max: Nat
  {P.safe(P.core(P.boundedRun(events, max, P.fixed(), P.State{P.initial(), 0n}))) == True{} : Bool}

# Real mutant runs from the same independent Init, not fabricated audit values.
law product_skip_commit_execution_violates:
  {P.safe(P.core(P.run(W.skipCommitEvents(), W.skipCommitConfig(), P.State{P.initial(), 0n}))) == False{} : Bool}

law product_skip_terminal_execution_violates:
  {P.safe(P.core(P.run(W.skipTerminalEvents(), W.skipTerminalConfig(), P.State{P.initial(), 0n}))) == False{} : Bool}

law product_skip_overlay_execution_violates:
  {P.safe(P.core(P.run(W.skipOverlayEvents(), W.skipOverlayConfig(), P.State{P.initial(), 0n}))) == False{} : Bool}

# The same event lists with the correct protocol are safe (nonvacuous controls).
law product_fixed_commit_execution_safe:
  {P.safe(P.core(P.run(W.skipCommitEvents(), P.fixed(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_fixed_terminal_execution_safe:
  {P.safe(P.core(P.run(W.skipTerminalEvents(), P.fixed(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_fixed_overlay_execution_safe:
  {P.safe(P.core(P.run(W.skipOverlayEvents(), P.fixed(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_skip_commit_records_bad_commit:
  {P.badCommit(P.core(P.run(W.skipCommitEvents(), W.skipCommitConfig(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_skip_terminal_records_bad_checkpoint:
  {P.badCheckpoint(P.core(P.run(W.skipTerminalEvents(), W.skipTerminalConfig(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_skip_overlay_records_bad_checkpoint:
  {P.badCheckpoint(P.core(P.run(W.skipOverlayEvents(), W.skipOverlayConfig(), P.State{P.initial(), 0n}))) == True{} : Bool}

law product_two_operation_recovery:
  {P.core(P.run(W.twoOperationRecoveryEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.twoOperationRecoveryExpected() : P.Core}

law product_visible_status_error_recovery:
  {P.core(P.run(W.visibleStatusRecoveryEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.visibleStatusRecoveryExpected() : P.Core}

law product_invisible_status_error_recovery:
  {P.core(P.run(W.invisibleStatusRecoveryEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.invisibleStatusRecoveryExpected() : P.Core}

law product_visible_update_error_recovery:
  {P.core(P.run(W.visibleUpdateRecoveryEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.visibleUpdateRecoveryExpected() : P.Core}

law product_invisible_update_error_recovery:
  {P.core(P.run(W.invisibleUpdateRecoveryEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.invisibleUpdateRecoveryExpected() : P.Core}

law product_partial_overlay_crash_recovery:
  {P.core(P.run(W.partialOverlayCrashEvents(), P.fixed(), P.State{P.initial(), 0n})) == W.partialOverlayCrashExpected() : P.Core}

# Reachable in the unrestricted data type, but NOT in the inductive invariant:
# Add genuinely observes an independently false committed field.
law product_uncommitted_add_is_not_tautologically_safe:
  {P.badCommit(P.next(P.fixed(), P.Tick{}, P.Core{P.Add{}, P.One{}, False{}, P.ready(), P.ready(), True{}, P.noneAdded(), False{}, False{}})) == True{} : Bool}
