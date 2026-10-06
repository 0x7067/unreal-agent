import Base
import ./model.bend as M
import ./product.bend as P
import ./certificate.bend as C
import ./witnesses.bend as W

law exclusive_file_prevents_respawn:
  for +attempts: List<&2, Bool>
  {M.shellSpawns(attempts, True{}) == 0n : Nat}

law at_most_once_shell_spawn:
  for +attempts: List<&2, Bool>
  {Nat.is_le(M.shellSpawns(attempts, False{}), 1n) == True{} : Bool}

law stored_terminal_never_rewritten:
  for +events: List<&2, M.Event>
  for +location: M.Location
  {M.terminalWrites(events, M.Stored{location}) == 0n : Nat}

law single_remote_terminal_checkpoint:
  for +events: List<&2, M.Event>
  for +s: M.Remote
  {Nat.is_le(M.terminalWrites(events, s), 1n) == True{} : Bool}

law legacy_spawn_retries_twice:
  {M.legacySpawns([True{}, False{}, True{}]) == 2n : Nat}

law fixed_spawn_retries_once:
  {M.shellSpawns([True{}, False{}, True{}], False{}) == 1n : Nat}

law close_skips_retained_terminal:
  {M.close(M.UnknownQueued{}) == M.UnknownQueued{} : M.Remote}

law legacy_duplicate_queue_writes_twice:
  {M.cancelThenCloseWrites(False{}) == 2n : Nat}

law queued_terminal_lost_on_crash:
  {M.terminalWrites([M.CancelFails{}, M.Crash{}, M.Resume{}, M.HandlerCloses{}, M.Persist{}], M.Active{}) == 1n : Nat}


law fixed_cancel_then_close_writes_once:
  {M.cancelThenCloseWrites(True{}) == 1n : Nat}


# Full shared-FIFO product. The crash counter is unbounded and not used as fuel.
# The finite cover constrains independent fields by reachable-state relations;
# Persist remains unconditional and can violate safety outside this invariant.
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

law product_every_prefix_safe:
  for +events: List<&2, P.Boundary>
  {P.allSafe(events, P.initial()) == True{} : Bool}

law product_arbitrary_crash_trace_safe:
  for +events: List<&2, P.Boundary>
  for +crashes: Nat
  {P.safe(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes}))) == True{} : Bool}

law product_invariant_queue_accounting:
  for +s: P.Core
  for inv: C.Invariant(s)
  {P.queueAccounting(s) == True{} : Bool}

law product_invariant_durable_history:
  for +s: P.Core
  for inv: C.Invariant(s)
  {P.durableHistory(s) == True{} : Bool}

law product_invariant_queue_bound:
  for +s: P.Core
  for inv: C.Invariant(s)
  {P.queueBound(s) == True{} : Bool}

law product_at_most_once_spawn:
  for +events: List<&2, P.Boundary>
  for +crashes: Nat
  {Nat.is_le(P.spawnCount(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes}))), 1n) == True{} : Bool}

# o ranges over BOTH Shell and Remote, not just the old remote projection.
law product_single_terminal_checkpoint:
  for +o: P.Op
  for +events: List<&2, P.Boundary>
  for +crashes: Nat
  {Nat.is_le(P.terminalWrites(o, P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes}))), 1n) == True{} : Bool}

law product_trace_queue_accounting:
  for +events: List<&2, P.Boundary>
  for +crashes: Nat
  {P.queueAccounting(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes}))) == True{} : Bool}

# Every intermediate independent-field state of the failing execution.
law legacy_product_spawn_execution:
  {W.spawnTrace() == [
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, False{}, False{}, True{}, [], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 0n, 0n, 0n}, 0n},
    P.State{P.Core{True{}, P.Process{}, P.Running{}, True{}, True{}, False{}, False{}, True{}, [P.Update{P.Shell{}, P.Checkpoint{False{}, P.Process{}}}], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 1n, 0n, 0n}, 0n},
    P.State{P.Core{False{}, P.NoPhase{}, P.NoActive{}, True{}, False{}, False{}, False{}, False{}, [], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 1n, 0n, 0n}, 1n},
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, True{}, True{}, False{}, False{}, True{}, [], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 1n, 0n, 0n}, 1n},
    P.State{P.Core{True{}, P.Process{}, P.Running{}, True{}, True{}, False{}, False{}, True{}, [P.Update{P.Shell{}, P.Checkpoint{False{}, P.Process{}}}], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 2n, 0n, 0n}, 1n}
  ] : List<&2, P.State>}

# Every intermediate independent-field state of the failing execution.
law legacy_product_close_execution:
  {W.closeTrace() == [
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, False{}, False{}, True{}, [], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 0n, 0n, 0n}, 0n},
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, True{}, True{}, True{}, [P.Update{P.Remote{}, P.Checkpoint{True{}, P.NoPhase{}}}], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 0n, 0n, 0n}, 0n},
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, True{}, True{}, False{}, [P.Update{P.Remote{}, P.Checkpoint{True{}, P.NoPhase{}}}, P.Update{P.Remote{}, P.Checkpoint{True{}, P.NoPhase{}}}], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 0n, 0n, 0n}, 0n},
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, True{}, True{}, False{}, [P.Update{P.Remote{}, P.Checkpoint{True{}, P.NoPhase{}}}], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{True{}, P.NoPhase{}}, 0n, 0n, 1n}, 0n},
    P.State{P.Core{True{}, P.CreateErr{}, P.Creating{}, False{}, True{}, True{}, True{}, False{}, [], P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{True{}, P.NoPhase{}}, 0n, 0n, 2n}, 0n}
  ] : List<&2, P.State>}


law legacy_product_spawn_violates_safety:
  {P.safe(W.finalSpawn(W.legacySpawn())) == False{} : Bool}

law legacy_product_spawn_count_two:
  {P.spawnCount(W.finalSpawn(W.legacySpawn())) == 2n : Nat}

law legacy_product_close_violates_safety:
  {P.safe(W.finalClose(W.legacyClose())) == False{} : Bool}

law legacy_product_close_writes_two:
  {P.terminalWrites(P.Remote{}, W.finalClose(W.legacyClose())) == 2n : Nat}

law fixed_product_same_spawn_trace_safe:
  {P.safe(W.finalSpawn(P.fixed())) == True{} : Bool}

law fixed_product_same_close_trace_safe:
  {P.terminalWrites(P.Remote{}, W.finalClose(P.fixed())) == 1n : Nat}

law product_shared_fifo_interleaving:
  {W.afterInterleaving() == P.Core{True{}, P.PhaseDone{}, P.NoActive{}, True{}, True{}, True{}, True{}, True{},
    [P.Update{P.Shell{}, P.Checkpoint{False{}, P.Process{}}},
     P.Update{P.Remote{}, P.Checkpoint{True{}, P.NoPhase{}}},
     P.Update{P.Shell{}, P.Checkpoint{True{}, P.PhaseDone{}}}],
    P.Checkpoint{False{}, P.CreateErr{}}, P.Checkpoint{False{}, P.NoPhase{}}, 1n, 0n, 0n} : P.Core}

law product_restored_process_never_respawns:
  {P.spawnCount(W.storedProcessResume()) == 1n : Nat}

law product_restored_process_writes_terminal:
  {P.terminalWrites(P.Shell{}, W.storedProcessResume()) == 1n : Nat}

law product_dropped_remote_terminal_regenerated_once:
  {P.terminalWrites(P.Remote{}, W.droppedRemoteTerminal()) == 1n : Nat}
