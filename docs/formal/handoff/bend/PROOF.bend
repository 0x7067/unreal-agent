import Base
import ./lists.bend as L
import ./inbox.bend as I
import ./generation.bend as G
import ./journal.bend as J
import ./LAWS.bend as Laws
import ./composed.bend as C
import ./composed-proof.bend as CP
import ./arithmetic.bend as A
import ./mutants.bend as M
import ./reload.bend as R
import ./reload-proof.bend as RP

# The inductive preservation theorem applies to any state satisfying the
# invariant, independently of how that state was reached.
def admitSafe(+seen: List<&2, Nat>, +q: List<&2, Nat>, +h: List<&2, Nat>, +d: List<&2, Nat>, +id: Nat, dup: Bool, known: {L.contains(id, seen) == dup : Bool}, +safe: I.Safe(I.Inbox{I.Open{}, seen, q, h, d})) -> I.Safe(I.admitOpen(I.Inbox{I.Open{}, seen, q, h, d}, id, dup)):
  match dup safe:
    case True{} Tuple{part, Tuple{uniq, ps}}:
      (part, (uniq, ps))
    case False{} Tuple{part, Tuple{uniq, ps}}:
      (%L.assoc(h, q, [id]) : {L.append(L.reverse(seen), [id]) == L.append(d, _) : List<&2, Nat>}
       %L.assoc(d, L.append(h, q), [id]) : {L.append(L.reverse(seen), [id]) == _ : List<&2, Nat>}
       %part : {L.append(L.reverse(seen), [id]) == L.append(_, [id]) : List<&2, Nat>}
       {==},
       (%Equal.sym(Bool, L.contains(id, seen), False{}, known) : {Bool.and(Bool.not(_), L.unique(seen)) == True{} : Bool}
        uniq,
        ps))

def inboxStep(+s: I.Box, +e: I.Event, +safe: I.Safe(s)) -> I.Safe(I.step(s, e)):
  match s e:
    case I.Inbox{I.Frozen{}, seen, q, h, d} I.Admit{id}:
      safe
    case I.Inbox{I.Open{}, seen, q, h, d} I.Admit{id}:
      admitSafe(seen, q, h, d, id, L.contains(id, seen), {==}, safe)
    case I.Inbox{I.Open{}, seen, x <> q, h, d} I.Emit{}:
      (part, (uniq, ps)) = safe
      (%Equal.sym(List<&2, Nat>, L.append(L.append(d, [x]), L.append(h, q)), L.append(d, x <> L.append(h, q)), L.assoc(d, [x], L.append(h, q))) : {L.reverse(seen) == _ : List<&2, Nat>}
       %Equal.sym(List<&2, Nat>, h, [], ps) : {L.reverse(seen) == L.append(d, x <> L.append(_, q)) : List<&2, Nat>}
       %ps : {L.reverse(seen) == L.append(d, L.append(_, x <> q)) : List<&2, Nat>}
       part,
       (uniq, ps))
    case I.Inbox{I.Open{}, seen, Nil{}, h, d} I.Emit{}:
      safe
    case I.Inbox{I.Frozen{}, seen, q, h, d} I.Emit{}:
      safe
    case I.Inbox{I.Open{}, seen, q, h, d} I.Freeze{}:
      (part, (uniq, ps)) = safe
      (%Equal.sym(List<&2, Nat>, L.append(L.append(h, q), []), L.append(h, q), L.rightNil(L.append(h, q))) : {L.reverse(seen) == L.append(d, _) : List<&2, Nat>}
       part,
       (uniq, {==}))
    case I.Inbox{I.Frozen{}, seen, q, h, d} I.Freeze{}:
      safe
    case I.Inbox{I.Frozen{}, seen, q, x <> h, d} I.ProcessHanded{}:
      (part, (uniq, ps)) = safe
      (%Equal.sym(List<&2, Nat>, L.append(L.append(d, [x]), L.append(h, q)), L.append(d, x <> L.append(h, q)), L.assoc(d, [x], L.append(h, q))) : {L.reverse(seen) == _ : List<&2, Nat>}
       part,
       (uniq, ps))
    case I.Inbox{I.Frozen{}, seen, q, Nil{}, d} I.ProcessHanded{}:
      safe
    case I.Inbox{I.Open{}, seen, q, h, d} I.ProcessHanded{}:
      safe
    case I.Inbox{I.Frozen{}, seen, q, Nil{}, d} I.Thaw{}:
      (part, (uniq, ps)) = safe
      (part, (uniq, {==}))
    case I.Inbox{I.Open{}, seen, q, h, d} I.Thaw{}:
      safe
    case I.Inbox{I.Frozen{}, seen, q, x <> h, d} I.Thaw{}:
      safe

def inboxRun(+es: List<&2, I.Event>, +s: I.Box, +safe: I.Safe(s)) -> I.Safe(I.run(es, s)):
  match es:
    case Nil{}:
      safe
    case e <> rest:
      inboxRun(rest, I.step(s, e), inboxStep(s, e, safe))

def Laws.inbox_all_traces(events):
  inboxRun(events, I.initial(), ({==}, ({==}, {==})))

def inboxPart(+s: I.Box, +safe: I.Safe(s)) -> {I.order(s) == I.accounted(s) : List<&2, Nat>}:
  match s:
    case I.Inbox{p, seen, q, h, d}:
      (part, rest) = safe
      part

def Laws.inbox_fifo_conservation(events):
  inboxPart(I.run(events, I.initial()), Laws.inbox_all_traces(events))

def eqRefl(n: Nat) -> {Nat.is_eq(n, n) == True{} : Bool}:
  match n:
    case 0n:
      {==}
    case 1n+p:
      eqRefl(p)

def Laws.inbox_dedup_survives_thaw(id):
  %Equal.sym(Bool, Nat.is_eq(id, id), True{}, eqRefl(id)) : {I.emit(I.admitOpen(I.Inbox{I.Open{}, [id], [], [], [id]}, id, Bool.or(_, False{}))) == I.Inbox{I.Open{}, [id], [], [], [id]} : I.Box}
  {==}

# Ownership and corrected stopped publication are inductive invariants, not
# conclusions inferred merely from a launch permission boolean.
def genBegin(+s: G.State, safe: G.Safe(s)) -> G.Safe(G.begin(s)):
  match s:
    case G.Host{live, cur, G.Idle{}, retired, False{}, status}:
      safe
    case G.Host{live, cur, G.Idle{}, retired, True{}, status}:
      safe
    case G.Host{live, cur, G.Staging{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Staged{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}:
      safe

def genProbe(+s: G.State, ok: Bool, safe: G.Safe(s)) -> G.Safe(G.probe(s, ok)):
  match s ok:
    case G.Host{live, cur, G.Staging{}, retired, failed, status} True{}:
      safe
    case G.Host{live, cur, G.Staging{}, retired, failed, status} False{}:
      safe
    case G.Host{live, cur, G.Idle{}, retired, failed, status} b:
      safe
    case G.Host{live, cur, G.Staged{}, retired, failed, status} b:
      safe
    case G.Host{live, cur, G.Fenced{}, retired, failed, status} b:
      safe
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status} b:
      safe

def genFence(+s: G.State, safe: G.Safe(s)) -> G.Safe(G.fence(s)):
  match s:
    case G.Host{live, cur, G.Staged{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Idle{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Staging{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Fenced{}, retired, failed, status}:
      safe
    case G.Host{live, cur, G.Awaiting{}, retired, failed, status}:
      safe

def genQuiesce(+s: G.State, safe: G.Safe(s)) -> G.Safe(G.quiesce(s)):
  match s:
    case G.Host{live, Some{pid}, G.Fenced{}, retired, failed, status}:
      safe
    case G.Host{live, None{}, r, retired, failed, status}:
      safe
    case G.Host{live, Some{pid}, G.Idle{}, retired, failed, status}:
      safe
    case G.Host{live, Some{pid}, G.Staging{}, retired, failed, status}:
      safe
    case G.Host{live, Some{pid}, G.Staged{}, retired, failed, status}:
      safe
    case G.Host{live, Some{pid}, G.Awaiting{}, retired, failed, status}:
      safe

def genAbort(+s: G.State, safe: G.Safe(s)) -> G.Safe(G.abort(s)):
  match s:
    case G.Host{live, cur, r, retired, failed, status}:
      safe

def genExit(+s: G.State) -> G.Safe(G.exit(s)):
  match s:
    case G.Host{live, cur, r, retired, failed, status}:
      ({==}, {==})

def genLaunch(+s: G.State, +id: Nat, safe: G.Safe(s)) -> G.Safe(G.launch(s, id)):
  match s:
    case G.Host{Nil{}, None{}, G.Awaiting{}, True{}, False{}, status}:
      ({==}, Unit{})
    case G.Host{Nil{}, None{}, G.Fenced{}, True{}, False{}, status}:
      ({==}, Unit{})
    case G.Host{x <> live, cur, r, retired, failed, status}:
      safe
    case G.Host{Nil{}, Some{pid}, r, retired, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Idle{}, retired, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Staging{}, retired, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Staged{}, retired, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Fenced{}, False{}, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Fenced{}, True{}, True{}, status}:
      safe
    case G.Host{Nil{}, None{}, G.Awaiting{}, False{}, failed, status}:
      safe
    case G.Host{Nil{}, None{}, G.Awaiting{}, True{}, True{}, status}:
      safe

def genStep(+s: G.State, e: G.Event, safe: G.Safe(s)) -> G.Safe(G.step(s, e)):
  match e:
    case G.Begin{}:
      genBegin(s, safe)
    case G.ProbeOK{}:
      genProbe(s, True{}, safe)
    case G.ProbeBad{}:
      genProbe(s, False{}, safe)
    case G.Fence{}:
      genFence(s, safe)
    case G.Quiesce{}:
      genQuiesce(s, safe)
    case G.Abort{}:
      genAbort(s, safe)
    case G.Exit{}:
      genExit(s)
    case G.Launch{id}:
      genLaunch(s, id, safe)

def genRun(+es: List<&2, G.Event>, +s: G.State, safe: G.Safe(s)) -> G.Safe(G.run(es, s)):
  match es:
    case Nil{}:
      safe
    case e <> rest:
      genRun(rest, G.step(s, e), genStep(s, e, safe))

def genOwn(+s: G.State, safe: G.Safe(s)) -> G.Own(s):
  (own, status) = safe
  own

def genStatus(+s: G.State, safe: G.Safe(s)) -> G.StatusSafe(s):
  (own, status) = safe
  status

def Laws.I1_all_traces(events):
  genOwn(G.run(events, G.initial()), genRun(events, G.initial(), ({==}, Unit{})))

def Laws.stopped_publication_all_traces(events):
  genStatus(G.run(events, G.initial()), genRun(events, G.initial(), ({==}, Unit{})))

def Laws.replacement_reachable():
  {==}

def Laws.abandoned_retirement_publishes_stopped():
  {==}

# Pointwise I2 induction over all journal traces. Ack is the only operation
# removing an ID from the pending alternative; it is justified by persistence.
def orRight(a: Bool, b: Bool, present: {b == True{} : Bool}) -> {Bool.or(a, b) == True{} : Bool}:
  match a:
    case True{}:
      {==}
    case False{}:
      present

def falseTrue(b: Bool) -> Data:
  match b:
    case False{}:
      Unit
    case True{}:
      Empty

def absurd(-A: Type, e: Empty) -> A:
  match e:

def impossible(e: {False{} == True{} : Bool}) -> Empty:
  %e : falseTrue(_)
  Unit{}

def eqOf(a: Nat, b: Nat, ok: {Nat.is_eq(a, b) == True{} : Bool}) -> {a == b : Nat}:
  match a b:
    case 0n 0n:
      {==}
    case 0n 1n+q:
      absurd({0n == 1n+q : Nat}, impossible(ok))
    case 1n+p 0n:
      absurd({1n+p == 0n : Nat}, impossible(ok))
    case 1n+p 1n+q:
      %eqOf(p, q, ok) : {1n+p == 1n+_ : Nat}
      {==}

def ackMember(+id: Nat, +x: Nat, +journal: List<&2, Nat>, +store: List<&2, Nat>, +acked: List<&2, Nat>, eq: Bool, known: {Nat.is_eq(id, x) == eq : Bool}, persisted: {L.contains(x, store) == True{} : Bool}, safe: J.Safe(id, J.Ledger{journal, store, acked}), present: {Bool.or(eq, L.contains(id, acked)) == True{} : Bool}) -> {L.contains(id, store) == True{} : Bool}:
  match eq:
    case True{}:
      %Equal.sym(Nat, id, x, eqOf(id, x, known)) : {L.contains(_, store) == True{} : Bool}
      persisted
    case False{}:
      safe(present)

def ackAllowed(+id: Nat, +x: Nat, +journal: List<&2, Nat>, +store: List<&2, Nat>, +acked: List<&2, Nat>, allowed: Bool, known: {L.contains(x, store) == allowed : Bool}, safe: J.Safe(id, J.Ledger{journal, store, acked})) -> J.Safe(id, J.ackIf(J.Ledger{journal, store, acked}, x, allowed)):
  match allowed:
    case True{}:
      present => ackMember(id, x, journal, store, acked, Nat.is_eq(id, x), {==}, known, safe, present)
    case False{}:
      safe

def journalStep(+id: Nat, +s: J.State, +e: J.Event, safe: J.Safe(id, s)) -> J.Safe(id, J.step(s, e)):
  match s e:
    case J.Ledger{journal, store, acked} J.Submit{x}:
      safe
    case J.Ledger{journal, store, acked} J.Persist{x}:
      present => orRight(Nat.is_eq(id, x), L.contains(id, store), safe(present))
    case J.Ledger{journal, store, acked} J.Ack{x}:
      ackAllowed(id, x, journal, store, acked, L.contains(x, store), {==}, safe)
    case J.Ledger{journal, store, acked} J.Other{}:
      safe

def journalRun(+id: Nat, +es: List<&2, J.Event>, +s: J.State, safe: J.Safe(id, s)) -> J.Safe(id, J.run(es, s)):
  match es:
    case Nil{}:
      safe
    case e <> rest:
      journalRun(id, rest, J.step(s, e), journalStep(id, s, e, safe))

def journalInitial(id: Nat) -> J.Safe(id, J.initial()):
  present => absurd({False{} == True{} : Bool}, impossible(present))

def Laws.I2_ack_implies_persisted(id, events):
  journalRun(id, events, J.initial(), journalInitial(id))

def noLoss(+id: Nat, +s: J.State, acked: Bool, known: {L.contains(id, J.acknowledged(s)) == acked : Bool}, safe: J.Safe(id, s)) -> {Bool.or(Bool.not(acked), L.contains(id, J.persisted(s))) == True{} : Bool}:
  match s acked:
    case J.Ledger{journal, store, acks} False{}:
      {==}
    case J.Ledger{journal, store, acks} True{}:
      safe(known)

def Laws.I2_no_loss(id, events):
  noLoss(id, J.run(events, J.initial()), L.contains(id, J.acknowledged(J.run(events, J.initial()))), {==}, Laws.I2_ack_implies_persisted(id, events))

def journalAckMonotone(+id: Nat, +s: J.State, x: Nat, allowed: Bool, present: {L.contains(id, J.accepted(s)) == True{} : Bool}) -> {L.contains(id, J.accepted(J.ackIf(s, x, allowed))) == True{} : Bool}:
  match s allowed:
    case J.Ledger{journal, store, acked} True{}:
      present
    case J.Ledger{journal, store, acked} False{}:
      present

def Laws.I2_journal_monotone(id, s, event, present):
  match s event:
    case J.Ledger{journal, store, acked} J.Submit{x}:
      orRight(Nat.is_eq(id, x), L.contains(id, journal), present)
    case J.Ledger{journal, store, acked} J.Persist{x}:
      present
    case J.Ledger{journal, store, acked} J.Ack{x}:
      journalAckMonotone(id, s, x, L.contains(x, store), present)
    case J.Ledger{journal, store, acked} J.Other{}:
      present


def Laws.composed_invariant_all_traces(events):
  CP.run(events, C.initial(), (Unit{}, (Unit{}, (Unit{}, (Unit{}, {==})))))

def Laws.I3_composed_persist_at_most_once(events):
  CP.unique(C.run(events, C.initial()), Laws.composed_invariant_all_traces(events))

def Laws.I4_composed_FIFO_across_replacement(events):
  CP.fifo(C.run(events, C.initial()), Laws.composed_invariant_all_traces(events))

def Laws.I2_composed_no_loss(id, events):
  CP.noLoss(id, C.run(events, C.initial()), Laws.composed_invariant_all_traces(events))

def Laws.composed_store_failure_stutters(s):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}: {==}

def composedQuiesceHost(+s: C.State, enabled: Bool, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.quiesceIf(s, enabled))):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: genQuiesce(host, safe)
    case C.System{n, a, w, h, t, xs, b, host} False{}: genAbort(host, safe)

def composedQuiesce(+s: C.State, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.quiesce(s))):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      composedQuiesceHost(s, Bool.and(Bool.not(b), Nat.is_le(w, a)), safe)

def composedLaunchHost(+s: C.State, +id: Nat, enabled: Bool, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.launchIf(s, id, enabled))):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: genLaunch(host, id, safe)
    case C.System{n, a, w, h, t, xs, b, host} False{}: safe

def composedLaunch(+s: C.State, +id: Nat, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.launch(s, id))):
  composedLaunchHost(s, id, C.canLaunch(C.host(s)), safe)

def composedControl(+s: C.State, +e: G.Event, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.control(s, e))):
  match e:
    case G.Launch{id}: composedLaunch(s, id, safe)
    case G.Quiesce{}: composedQuiesce(s, safe)
    case G.Begin{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.begin(C.host(s)))), G.begin(C.host(s)), CP.hostReplace(s, G.begin(C.host(s)))) : G.Safe(_)
      genBegin(C.host(s), safe)
    case G.ProbeOK{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.probe(C.host(s), True{}))), G.probe(C.host(s), True{}), CP.hostReplace(s, G.probe(C.host(s), True{}))) : G.Safe(_)
      genProbe(C.host(s), True{}, safe)
    case G.ProbeBad{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.probe(C.host(s), False{}))), G.probe(C.host(s), False{}), CP.hostReplace(s, G.probe(C.host(s), False{}))) : G.Safe(_)
      genProbe(C.host(s), False{}, safe)
    case G.Fence{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.fence(C.host(s)))), G.fence(C.host(s)), CP.hostReplace(s, G.fence(C.host(s)))) : G.Safe(_)
      genFence(C.host(s), safe)
    case G.Abort{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.abort(C.host(s)))), G.abort(C.host(s)), CP.hostReplace(s, G.abort(C.host(s)))) : G.Safe(_)
      genAbort(C.host(s), safe)
    case G.Exit{}:
      %Equal.sym(G.State, C.host(C.replaceHost(s, G.exit(C.host(s)))), G.exit(C.host(s)), CP.hostReplace(s, G.exit(C.host(s)))) : G.Safe(_)
      genExit(C.host(s))

def composedHostStep(+s: C.State, +e: C.Event, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.step(s, e))):
  match e:
    case C.Submit{}:
      %Equal.sym(G.State, C.host(C.submit(s)), C.host(s), CP.hostSubmit(s)) : G.Safe(_)
      safe
    case C.Write{o}:
      %Equal.sym(G.State, C.host(C.write(s, o)), C.host(s), CP.hostWrite(s, o)) : G.Safe(_)
      safe
    case C.Persist{}:
      %Equal.sym(G.State, C.host(C.persist(s)), C.host(s), CP.hostPersist(s)) : G.Safe(_)
      safe
    case C.PersistFail{}: safe
    case C.Ack{}:
      %Equal.sym(G.State, C.host(C.ack(s)), C.host(s), CP.hostAck(s)) : G.Safe(_)
      safe
    case C.Control{e}: composedControl(s, e, safe)

def composedHostRun(+es: List<&2, C.Event>, +s: C.State, safe: G.Safe(C.host(s))) -> G.Safe(C.host(C.run(es, s))):
  match es:
    case Nil{}: safe
    case e <> rest: composedHostRun(rest, C.step(s, e), composedHostStep(s, e, safe))

def Laws.composed_ownership_all_traces(events):
  genOwn(C.host(C.run(events, C.initial())), composedHostRun(events, C.initial(), ({==}, Unit{})))

def Laws.composed_stopped_publication_all_traces(events):
  genStatus(C.host(C.run(events, C.initial())), composedHostRun(events, C.initial(), ({==}, Unit{})))

def unguardedControl(+s: C.State, +e: G.Event, inv: C.Inv(s)) -> C.Inv(M.control(s, M.NoGuard{}, e)):
  match e:
    case G.Quiesce{}: CP.replaceHost(s, G.quiesce(C.host(s)), inv)
    case G.Begin{}: CP.control(s, e, inv)
    case G.ProbeOK{}: CP.control(s, e, inv)
    case G.ProbeBad{}: CP.control(s, e, inv)
    case G.Fence{}: CP.control(s, e, inv)
    case G.Abort{}: CP.control(s, e, inv)
    case G.Exit{}: CP.control(s, e, inv)
    case G.Launch{id}: CP.control(s, e, inv)

def unguardedStep(+s: C.State, +e: C.Event, inv: C.Inv(s)) -> C.Inv(M.step(s, M.NoGuard{}, e)):
  match e:
    case C.Control{ev}: unguardedControl(s, ev, inv)
    case C.Submit{}: CP.step(s, e, inv)
    case C.Write{o}: CP.step(s, e, inv)
    case C.Persist{}: CP.step(s, e, inv)
    case C.PersistFail{}: inv
    case C.Ack{}: CP.step(s, e, inv)

def unguardedRun(+es: List<&2, C.Event>, +s: C.State, inv: C.Inv(s)) -> C.Inv(M.run(es, M.NoGuard{}, s)):
  match es:
    case Nil{}: inv
    case e <> rest: unguardedRun(rest, M.step(s, M.NoGuard{}, e), unguardedStep(s, e, inv))

def Laws.seeded_without_guard_all_traces(events):
  unguardedRun(events, C.initial(), (Unit{}, (Unit{}, (Unit{}, (Unit{}, {==})))))

def Laws.corrected_replacement_delivery(): {==}
def Laws.restored_seed_replay_drops_duplicate(): {==}
def Laws.missing_seed_and_guard_executes_duplicate(): {==}
def Laws.missing_seed_and_guard_violates_uniqueness(): {==}
def Laws.missing_seed_and_guard_violates_FIFO(): {==}
def Laws.lost_write_is_marked_and_not_retried(): {==}
def Laws.failed_write_can_have_been_admitted(): {==}
def Laws.abandoned_retirement_explicit_recovery_delivers(): {==}

def Laws.reload_metadata_all_traces(events):
  RP.run(events, R.initial(), ({==}, {==}))

def Laws.I5_actual_launch_history_all_traces(events):
  RP.launchHistory(R.run(events, R.initial()), Laws.reload_metadata_all_traces(events))

def Laws.I6_probed_candidate_all_traces(events):
  RP.probeSafe(R.run(events, R.initial()), Laws.reload_metadata_all_traces(events))

def Laws.I7_single_flight_all_traces(events):
  RP.singleFlight(R.run(events, R.initial()), Laws.reload_metadata_all_traces(events))

def Laws.I8_unused_artifact_cleanup_all_traces(events):
  RP.artifactSafe(R.run(events, R.initial()), Laws.reload_metadata_all_traces(events))

def Laws.missing_CAS_reaches_two_reloads(): {==}
def Laws.missing_discard_leaks_artifact(): {==}
def Laws.missing_probe_retires_incompatible(): {==}
def Laws.early_launch_records_live_old_process(): {==}
def Laws.reload_replacement_is_reachable(): {==}

def Laws.composed_store_exact_admission_prefix(events):
  CP.prefix(C.run(events, C.initial()), Laws.composed_invariant_all_traces(events))

def Laws.composed_fresh_write_is_FIFO_tail(s, inv, healthy, fresh):
  CP.freshTail(s, healthy, fresh, inv)

def Laws.historical_abandoned_exit_executes_stale_ready(): {==}
def Laws.corrected_abandoned_exit_executes_stopped(): {==}
def Laws.guard_refuses_unacknowledged_delivery(): {==}
def Laws.failed_pipe_crash_does_not_launch_replacement(): {==}

def Laws.composed_journal_admission_monotone(s, event):
  CP.acceptedStep(s, event)
