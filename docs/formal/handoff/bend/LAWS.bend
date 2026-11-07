import Base
import ./inbox.bend as I
import ./generation.bend as G
import ./journal.bend as J
import ./lists.bend as L
import ./composed.bend as C
import ./arithmetic.bend as A
import ./mutants.bend as M
import ./reload.bend as R


law inbox_all_traces:
  for +events: List<&2, I.Event>
  I.Safe(I.run(events, I.initial()))

law inbox_fifo_conservation:
  for +events: List<&2, I.Event>
  {I.order(I.run(events, I.initial())) == I.accounted(I.run(events, I.initial())) : List<&2, Nat>}

law inbox_dedup_survives_thaw:
  for +id: Nat
  {I.run([I.Admit{id}, I.Freeze{}, I.ProcessHanded{}, I.Thaw{}, I.Admit{id}, I.Emit{}], I.initial()) == I.Inbox{I.Open{}, [id], [], [], [id]} : I.Box}


law I1_all_traces:
  for +events: List<&2, G.Event>
  G.Own(G.run(events, G.initial()))

law stopped_publication_all_traces:
  for +events: List<&2, G.Event>
  G.StatusSafe(G.run(events, G.initial()))

law I2_ack_implies_persisted:
  for +id: Nat
  for +events: List<&2, J.Event>
  J.Safe(id, J.run(events, J.initial()))

law I2_no_loss:
  for +id: Nat
  for +events: List<&2, J.Event>
  {J.accounted(id, J.run(events, J.initial())) == True{} : Bool}

law I2_journal_monotone:
  for +id: Nat
  for +s: J.State
  for +event: J.Event
  for present: {L.contains(id, J.accepted(s)) == True{} : Bool}
  {L.contains(id, J.accepted(J.step(s, event))) == True{} : Bool}

law replacement_reachable:
  {G.run([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Exit{}, G.Launch{1n}], G.initial()) == G.Host{[1n], Some{1n}, G.Idle{}, False{}, False{}, G.Ready{}} : G.State}

law abandoned_retirement_publishes_stopped:
  {G.run([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Abort{}, G.Exit{}], G.initial()) == G.Host{[], None{}, G.Idle{}, True{}, False{}, G.Stopped{}} : G.State}

# Composed model: the same store is read on replacement and written by the
# interval FIFO; these are not conclusions obtained from disconnected slices.
# Admission keys 0,1,... are unbounded journal positions (TLA uses 1,2).

law composed_invariant_all_traces:
  for +events: List<&2, C.Event>
  C.Inv(C.run(events, C.initial()))

law I3_composed_persist_at_most_once:
  for +events: List<&2, C.Event>
  {L.unique(C.store(C.run(events, C.initial()))) == True{} : Bool}

law I4_composed_FIFO_across_replacement:
  for +events: List<&2, C.Event>
  {A.descending(C.store(C.run(events, C.initial()))) == True{} : Bool}

law I2_composed_no_loss:
  for +id: Nat
  for +events: List<&2, C.Event>
  {C.accounted(id, C.run(events, C.initial())) == True{} : Bool}

law composed_store_failure_stutters:
  for +s: C.State
  {C.step(s, C.PersistFail{}) == s : C.State}

law composed_ownership_all_traces:
  for +events: List<&2, C.Event>
  G.Own(C.host(C.run(events, C.initial())))

law composed_stopped_publication_all_traces:
  for +events: List<&2, C.Event>
  G.StatusSafe(C.host(C.run(events, C.initial())))

law seeded_without_guard_all_traces:
  for +events: List<&2, C.Event>
  C.Inv(M.run(events, M.NoGuard{}, C.initial()))

law corrected_replacement_delivery:
  {C.run([C.Submit{}, C.Write{C.Delivered{}}, C.Persist{}, C.Ack{}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}, C.Control{G.Exit{}}, C.Control{G.Launch{1n}}, C.Submit{}, C.Write{C.Delivered{}}, C.Persist{}], C.initial()) == C.System{2n, 1n, 2n, 2n, 2n, [1n, 0n], False{}, G.Host{[1n], Some{1n}, G.Idle{}, False{}, False{}, G.Ready{}}} : C.State}

law restored_seed_replay_drops_duplicate:
  {C.store(M.run(M.replayTrace(), M.NoGuard{}, C.initial())) == [0n] : List<&2, Nat>}

law missing_seed_and_guard_executes_duplicate:
  {C.store(M.run(M.replayTrace(), M.NoSeedNoGuard{}, C.initial())) == [0n, 0n] : List<&2, Nat>}

law missing_seed_and_guard_violates_uniqueness:
  {L.unique(C.store(M.run(M.replayTrace(), M.NoSeedNoGuard{}, C.initial()))) == False{} : Bool}

law missing_seed_and_guard_violates_FIFO:
  {A.descending(C.store(M.run(M.fifoMutantTrace(), M.NoSeedNoGuard{}, C.initial()))) == False{} : Bool}

law lost_write_is_marked_and_not_retried:
  {C.run([C.Submit{}, C.Write{C.FailedLost{}}, C.Write{C.Delivered{}}, C.Persist{}], C.initial()) == C.System{1n, 0n, 1n, 0n, 0n, [], True{}, G.initial()} : C.State}

law failed_write_can_have_been_admitted:
  {C.run([C.Submit{}, C.Write{C.FailedDelivered{}}, C.Write{C.Delivered{}}, C.Persist{}], C.initial()) == C.System{1n, 0n, 1n, 1n, 1n, [0n], True{}, G.initial()} : C.State}

law abandoned_retirement_explicit_recovery_delivers:
  {C.store(C.run([C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}, C.Control{G.Abort{}}, C.Control{G.Exit{}}, C.Submit{}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Launch{1n}}, C.Write{C.Delivered{}}, C.Persist{}], C.initial())) == [0n] : List<&2, Nat>}

law reload_metadata_all_traces:
  for +events: List<&2, G.Event>
  R.Inv(R.run(events, R.initial()))

law I5_actual_launch_history_all_traces:
  for +events: List<&2, G.Event>
  {R.launchSafe(R.run(events, R.initial())) == True{} : Bool}

law I6_probed_candidate_all_traces:
  for +events: List<&2, G.Event>
  {R.probeSafe(R.run(events, R.initial())) == True{} : Bool}

law I7_single_flight_all_traces:
  for +events: List<&2, G.Event>
  {R.singleFlight(R.run(events, R.initial())) == True{} : Bool}

law I8_unused_artifact_cleanup_all_traces:
  for +events: List<&2, G.Event>
  {R.artifactSafe(R.run(events, R.initial())) == True{} : Bool}

law missing_CAS_reaches_two_reloads:
  {R.singleFlight(R.mutantRun([G.Begin{}, G.Begin{}], R.NoCAS{}, R.initial())) == False{} : Bool}

law missing_discard_leaks_artifact:
  {R.artifactSafe(R.mutantRun([G.Begin{}, G.ProbeOK{}, G.Abort{}], R.NoDiscard{}, R.initial())) == False{} : Bool}

law missing_probe_retires_incompatible:
  {R.probeSafe(R.mutantRun([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}], R.NoProbe{}, R.initial())) == False{} : Bool}

law early_launch_records_live_old_process:
  {R.launchSafe(R.mutantRun([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Launch{1n}], R.EarlyLaunch{}, R.initial())) == False{} : Bool}

law reload_replacement_is_reachable:
  {R.run([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Exit{}, G.Launch{1n}], R.initial()) == R.Attempt{G.Host{[1n], Some{1n}, G.Idle{}, False{}, False{}, G.Ready{}}, R.Tracking{0n, False{}, False{}}, True{}} : R.State}

law composed_store_exact_admission_prefix:
  for +events: List<&2, C.Event>
  {C.store(C.run(events, C.initial())) == A.down(C.head(C.run(events, C.initial()))) : List<&2, Nat>}

law composed_fresh_write_is_FIFO_tail:
  for +s: C.State
  for inv: C.Inv(s)
  for healthy: {C.broken(s) == False{} : Bool}
  for fresh: {Nat.is_lt(Nat.max(C.sent(s), C.acknowledged(s)), C.seen(s)) == False{} : Bool}
  {Nat.max(C.sent(s), C.acknowledged(s)) == C.seen(s) : Nat}

law historical_abandoned_exit_executes_stale_ready:
  {R.statusSafe(R.mutantRun([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Abort{}, G.Exit{}], R.OldStatus{}, R.initial())) == False{} : Bool}

law corrected_abandoned_exit_executes_stopped:
  {R.statusSafe(R.run([G.Begin{}, G.ProbeOK{}, G.Fence{}, G.Quiesce{}, G.Abort{}, G.Exit{}], R.initial())) == True{} : Bool}

law guard_refuses_unacknowledged_delivery:
  {C.run([C.Submit{}, C.Write{C.Delivered{}}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}], C.initial()) == C.System{1n, 0n, 1n, 0n, 1n, [], False{}, G.initial()} : C.State}

law failed_pipe_crash_does_not_launch_replacement:
  {C.host(C.run([C.Submit{}, C.Write{C.FailedLost{}}, C.Control{G.Exit{}}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Launch{1n}}], C.initial())) == G.Host{[], None{}, G.Idle{}, False{}, True{}, G.Stopped{}} : G.State}

law composed_journal_admission_monotone:
  for +s: C.State
  for +event: C.Event
  A.Le(C.accepted(s), C.accepted(C.step(s, event)))
