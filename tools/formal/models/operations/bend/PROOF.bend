import Base
import ./model.bend as M
import ./LAWS.bend as Laws
import ./product.bend as P
import ./certificate.bend as C
import ./closure.bend as G
import ./witnesses.bend as W

def Laws.exclusive_file_prevents_respawn(attempts):
  match attempts:
    case Nil{}: {==}
    case attempt <> rest:
      match attempt:
        case False{}: Laws.exclusive_file_prevents_respawn(rest)
        case True{}: Laws.exclusive_file_prevents_respawn(rest)

def Laws.at_most_once_shell_spawn(attempts):
  match attempts:
    case Nil{}: {==}
    case attempt <> rest:
      match attempt:
        case False{}: Laws.at_most_once_shell_spawn(rest)
        case True{}:
          %Equal.sym(Nat, M.shellSpawns(rest, True{}), 0n, Laws.exclusive_file_prevents_respawn(rest)) : {Nat.is_le(1n+_, 1n) == True{} : Bool}
          {==}

def Laws.stored_terminal_never_rewritten(events, location):
  match events:
    case Nil{}: {==}
    case e <> rest:
      match e:
        case M.CancelFails{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Retained{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
        case M.RemoteFinishes{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
        case M.HandlerCloses{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Retained{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
        case M.Persist{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Retained{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
        case M.Crash{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Down{})
        case M.Resume{}:
          match location:
            case M.Retained{}: Laws.stored_terminal_never_rewritten(rest, M.Retained{})
            case M.Released{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})
            case M.Down{}: Laws.stored_terminal_never_rewritten(rest, M.Released{})

def Laws.single_remote_terminal_checkpoint(events, s):
  match events:
    case Nil{}: {==}
    case e <> rest:
      match e:
        case M.CancelFails{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.UnknownQueued{})
            case M.UnknownQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.UnknownQueued{})
            case M.ReleasedQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Retained{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Retained{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
        case M.RemoteFinishes{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.UnknownQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.ReleasedQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
        case M.HandlerCloses{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.UnknownQueued{})
            case M.UnknownQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.UnknownQueued{})
            case M.ReleasedQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Retained{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Retained{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
        case M.Persist{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.Active{})
            case M.UnknownQueued{}:
              %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Retained{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Retained{})) : {Nat.is_le(1n+_, 1n) == True{} : Bool}
              {==}
            case M.ReleasedQueued{}:
              %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(1n+_, 1n) == True{} : Bool}
              {==}
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Retained{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Retained{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
        case M.Crash{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
            case M.UnknownQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
            case M.ReleasedQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Down{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Down{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.DownUnstored{})
        case M.Resume{}:
          match s:
            case M.Active{}:
              Laws.single_remote_terminal_checkpoint(rest, M.Active{})
            case M.UnknownQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.UnknownQueued{})
            case M.ReleasedQueued{}:
              Laws.single_remote_terminal_checkpoint(rest, M.ReleasedQueued{})
            case M.Stored{location}:
              match location:
                case M.Retained{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Retained{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Retained{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Released{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
                case M.Down{}:
                  %Equal.sym(Nat, M.terminalWrites(rest, M.Stored{M.Released{}}), 0n, Laws.stored_terminal_never_rewritten(rest, M.Released{})) : {Nat.is_le(_, 1n) == True{} : Bool}
                  {==}
            case M.DownUnstored{}:
              Laws.single_remote_terminal_checkpoint(rest, M.Active{})

def Laws.legacy_spawn_retries_twice(): {==}
def Laws.fixed_spawn_retries_once(): {==}
def Laws.close_skips_retained_terminal(): {==}
def Laws.legacy_duplicate_queue_writes_twice(): {==}
def Laws.queued_terminal_lost_on_crash(): {==}

def Laws.fixed_cancel_then_close_writes_once(): {==}


def Laws.product_initial_invariant(): (C.S0{}, {==})

def Laws.product_invariant_preserved(e, s, inv):
  match inv:
    case (i, eq):
      %eq : C.Invariant(P.next(P.fixed(), e, _))
      G.closure(e, i)

def Laws.product_invariant_implies_safety(s, inv):
  match inv:
    case (i, eq):
      %eq : {P.safe(_) == True{} : Bool}
      G.indexSafe(i)

def prefixSafety(events: List<&2, P.Boundary>, +s: P.Core, inv: C.Invariant(s)) -> {P.allSafe(events, s) == True{} : Bool}:
  match events:
    case Nil{}: Laws.product_invariant_implies_safety(s, inv)
    case +e <> rest:
      match inv:
        case (+i, eq):
          %eq : {Bool.and(P.safe(_), P.allSafe(rest, P.next(P.fixed(), e, _))) == True{} : Bool}
          %Equal.sym(Bool, P.safe(C.decode(i)), True{}, G.indexSafe(i)) : {Bool.and(_, P.allSafe(rest, P.next(P.fixed(), e, C.decode(i)))) == True{} : Bool}
          prefixSafety(rest, P.next(P.fixed(), e, C.decode(i)), G.closure(e, i))

def Laws.product_every_prefix_safe(events):
  prefixSafety(events, P.initial(), Laws.product_initial_invariant())

def stateInvariant(events: List<&2, P.Boundary>, +s: P.State, inv: C.Invariant(P.core(s))) -> C.Invariant(P.core(P.run(events, P.fixed(), s))):
  match events:
    case Nil{}: inv
    case +e <> rest:
      match s:
        case P.State{+x, n}:
          stateInvariant(rest, P.step(P.fixed(), e, P.State{x, n}), Laws.product_invariant_preserved(e, x, inv))

def Laws.product_arbitrary_crash_trace_safe(events, crashes):
  Laws.product_invariant_implies_safety(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes})), stateInvariant(events, P.State{P.initial(), crashes}, Laws.product_initial_invariant()))

def Laws.product_invariant_queue_accounting(s, inv):
  match inv:
    case (i, eq):
      %eq : {P.queueAccounting(_) == True{} : Bool}
      G.indexqueueAccounting(i)

def Laws.product_invariant_durable_history(s, inv):
  match inv:
    case (i, eq):
      %eq : {P.durableHistory(_) == True{} : Bool}
      G.indexdurableHistory(i)

def Laws.product_invariant_queue_bound(s, inv):
  match inv:
    case (i, eq):
      %eq : {P.queueBound(_) == True{} : Bool}
      G.indexqueueBound(i)

def invariantSpawns(+s: P.Core, inv: C.Invariant(s)) -> {Nat.is_le(P.spawnCount(s), 1n) == True{} : Bool}:
  match inv:
    case (i, eq):
      %eq : {Nat.is_le(P.spawnCount(_), 1n) == True{} : Bool}
      G.indexSpawns(i)

def invariantWrites(+o: P.Op, +s: P.Core, inv: C.Invariant(s)) -> {Nat.is_le(P.terminalWrites(o, s), 1n) == True{} : Bool}:
  match inv:
    case (i, eq):
      %eq : {Nat.is_le(P.terminalWrites(o, _), 1n) == True{} : Bool}
      G.indexWrites(o, i)

def Laws.product_at_most_once_spawn(events, crashes):
  invariantSpawns(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes})), stateInvariant(events, P.State{P.initial(), crashes}, Laws.product_initial_invariant()))

def Laws.product_single_terminal_checkpoint(o, events, crashes):
  invariantWrites(o, P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes})), stateInvariant(events, P.State{P.initial(), crashes}, Laws.product_initial_invariant()))

def Laws.product_trace_queue_accounting(events, crashes):
  Laws.product_invariant_queue_accounting(P.core(P.run(events, P.fixed(), P.State{P.initial(), crashes})), stateInvariant(events, P.State{P.initial(), crashes}, Laws.product_initial_invariant()))

def Laws.legacy_product_spawn_execution(): {==}

def Laws.legacy_product_close_execution(): {==}

def Laws.legacy_product_spawn_violates_safety(): {==}

def Laws.legacy_product_spawn_count_two(): {==}

def Laws.legacy_product_close_violates_safety(): {==}

def Laws.legacy_product_close_writes_two(): {==}

def Laws.fixed_product_same_spawn_trace_safe(): {==}

def Laws.fixed_product_same_close_trace_safe(): {==}

def Laws.product_shared_fifo_interleaving(): {==}

def Laws.product_restored_process_never_respawns(): {==}

def Laws.product_restored_process_writes_terminal(): {==}

def Laws.product_dropped_remote_terminal_regenerated_once(): {==}
