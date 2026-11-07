import Base
import ./model.bend as M
import ./LAWS.bend as Laws
import ./product.bend as P
import ./certificate.bend as C
import ./closure.bend as G

def eventCommit(e: M.Event, s: M.State) -> {M.commitSafe(M.audit(e, s)) == True{} : Bool}:
  match e:
    case M.Add{}:
      match s:
        case M.Adding{latest}: {==}
        case M.Translate{}: {==}
        case M.Install{}: {==}
        case M.Apply{}: {==}
        case M.Append{}: {==}
        case M.StatusReturn{_}: {==}
        case M.Guard{_}: {==}
        case M.Idle{_}: {==}
        case M.UpdatePending{_, _}: {==}
        case M.UpdateReturn{_}: {==}
        case M.Stopped{_}: {==}
        case M.Replay{_}: {==}
        case M.Overlay{_}: {==}
    case M.Advance{}: {==}
    case M.ToAwaiting{}: {==}
    case M.ToTerminal{}: {==}
    case M.ErrorInvisible{}: {==}
    case M.ErrorVisible{}: {==}
    case M.Crash{}: {==}

def eventCheckpoint(e: M.Event, s: M.State) -> {M.checkpointSafe(M.audit(e, s)) == True{} : Bool}:
  match e:
    case M.Add{}:
      match s:
        case M.Adding{latest}:
          match latest:
            case M.ReadyLive{}: {==}
            case M.AwaitingLive{}: {==}
        case M.Translate{}: {==}
        case M.Install{}: {==}
        case M.Apply{}: {==}
        case M.Append{}: {==}
        case M.StatusReturn{_}: {==}
        case M.Guard{_}: {==}
        case M.Idle{_}: {==}
        case M.UpdatePending{_, _}: {==}
        case M.UpdateReturn{_}: {==}
        case M.Stopped{_}: {==}
        case M.Replay{_}: {==}
        case M.Overlay{_}: {==}
    case M.Advance{}: {==}
    case M.ToAwaiting{}: {==}
    case M.ToTerminal{}: {==}
    case M.ErrorInvisible{}: {==}
    case M.ErrorVisible{}: {==}
    case M.Crash{}: {==}

def Laws.commit_before_every_add(events, s):
  match events:
    case Nil{}: {==}
    case +e <> rest:
      %Equal.sym(Bool, M.commitSafe(M.audit(e, s)), True{}, eventCommit(e, s)) : {Bool.and(_, M.traceCommit(rest, M.step(e, s))) == True{} : Bool}
      Laws.commit_before_every_add(rest, M.step(e, s))

def Laws.no_terminal_or_stale_add(events, s):
  match events:
    case Nil{}: {==}
    case +e <> rest:
      %Equal.sym(Bool, M.checkpointSafe(M.audit(e, s)), True{}, eventCheckpoint(e, s)) : {Bool.and(_, M.traceCheckpoint(rest, M.step(e, s))) == True{} : Bool}
      Laws.no_terminal_or_stale_add(rest, M.step(e, s))

def Laws.no_add_before_status_commit(): {==}
def Laws.stale_replay_snapshot_not_dispatchable(latest): {==}
def Laws.terminal_recovery_not_added(): {==}
def Laws.awaiting_recovery_uses_latest(): {==}
def Laws.visible_status_error_recoverable(): {==}
def Laws.invisible_status_error_retranslates(): {==}

def Laws.skip_commit_audit_rejected(): {==}
def Laws.skip_overlay_audit_rejected(): {==}
def Laws.skip_terminal_guard_audit_rejected(): {==}


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

def Laws.product_invariant_relates_partial_cursor(s, inv):
  match inv:
    case (i, eq):
      %eq : {P.consistent(_) == True{} : Bool}
      G.indexConsistent(i)

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

def boundedInvariant(events: List<&2, P.Boundary>, +max: Nat, +s: P.State, inv: C.Invariant(P.core(s))) -> C.Invariant(P.core(P.boundedRun(events, max, P.fixed(), s))):
  match events:
    case Nil{}: inv
    case +e <> rest:
      match s:
        case P.State{+x, +n}:
          boundedInvariant(rest, max, P.boundedStep(max, P.fixed(), e, P.State{x, n}), Laws.product_invariant_preserved(P.budgetEvent(Nat.is_lt(n, max), e), x, inv))

def Laws.product_bounded_crash_trace_safe(events, max):
  Laws.product_invariant_implies_safety(P.core(P.boundedRun(events, max, P.fixed(), P.State{P.initial(), 0n})), boundedInvariant(events, max, P.State{P.initial(), 0n}, Laws.product_initial_invariant()))

def Laws.product_skip_commit_execution_violates(): {==}
def Laws.product_skip_terminal_execution_violates(): {==}
def Laws.product_skip_overlay_execution_violates(): {==}
def Laws.product_fixed_commit_execution_safe(): {==}
def Laws.product_fixed_terminal_execution_safe(): {==}
def Laws.product_fixed_overlay_execution_safe(): {==}

def Laws.product_skip_commit_records_bad_commit(): {==}
def Laws.product_skip_terminal_records_bad_checkpoint(): {==}
def Laws.product_skip_overlay_records_bad_checkpoint(): {==}
def Laws.product_two_operation_recovery(): {==}
def Laws.product_visible_status_error_recovery(): {==}
def Laws.product_invisible_status_error_recovery(): {==}
def Laws.product_visible_update_error_recovery(): {==}
def Laws.product_invisible_update_error_recovery(): {==}
def Laws.product_partial_overlay_crash_recovery(): {==}
def Laws.product_uncommitted_add_is_not_tautologically_safe(): {==}
