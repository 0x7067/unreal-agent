import Base
import ./composed.bend as C
import ./arithmetic.bend as A
import ./generation.bend as G
import ./lists.bend as L

# Preservation is proved for arbitrary invariant-satisfying states, not just
# concrete executions. This module is reachable from the public paired PROOF.
def replaceHost(+s: C.State, host: G.State, +inv: C.Inv(s)) -> C.Inv(C.replaceHost(s, host)):
  match s:
    case C.System{n, a, w, h, t, xs, b, old}: inv

def cursor(b: Bool, w: Nat, t: Nat, le: A.Le(w, t)) -> C.Cursor(b, w, t):
  match b:
    case False{}: le
    case True{}: Unit{}

def delivered(+s: C.State, +id: Nat, dup: Bool, broken: Bool, +known: {Nat.is_lt(id, C.seen(s)) == dup : Bool}, bound: A.Le(id, C.seen(s)), limit: A.Le(1n+id, C.accepted(s)), +inv: C.Inv(s)) -> C.Inv(C.delivered(s, id, dup, broken)):
  match s dup:
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      (ah, (ht, (tn, (cursor(broken, 1n+id, t, A.ltTrue(id, t, known)), eq))))
    case C.System{n, a, w, h, t, xs, b, host} False{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      hi = A.trans(h, t, 1n+id, ht, A.weaken(t, id, A.notLt(id, t, known)))
      (ah, (hi, (limit, (cursor(broken, 1n+id, 1n+id, A.refl(1n+id)), eq))))

def attempt(+s: C.State, +id: Nat, o: C.Outcome, bound: A.Le(id, C.seen(s)), limit: A.Le(1n+id, C.accepted(s)), +inv: C.Inv(s)) -> C.Inv(C.attempt(s, id, o)):
  match s o:
    case C.System{n, a, w, h, t, xs, b, host} C.Delivered{}:
      delivered(s, id, Nat.is_lt(id, t), False{}, {==}, bound, limit, inv)
    case C.System{n, a, w, h, t, xs, b, host} C.FailedDelivered{}:
      delivered(s, id, Nat.is_lt(id, t), True{}, {==}, bound, limit, inv)
    case C.System{n, a, w, h, t, xs, b, host} C.FailedLost{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      (ah, (ht, (tn, (Unit{}, eq))))

def healthyCursor(b: Bool, w: Nat, t: Nat, e: {b == False{} : Bool}, cur: C.Cursor(b, w, t)) -> A.Le(w, t):
  %e : C.Cursor(_, w, t)
  cur

def writeEnabled(+s: C.State, +o: C.Outcome, enabled: Bool, +known: {Bool.and(C.delivery(C.host(s)), Nat.is_lt(Nat.max(C.sent(s), C.acknowledged(s)), C.accepted(s))) == enabled : Bool}, +inv: C.Inv(s), healthy: {C.broken(s) == False{} : Bool}) -> C.Inv(C.writeIf(s, Nat.max(C.sent(s), C.acknowledged(s)), o, enabled)):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} False{}: inv
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      wt = healthyCursor(b, w, t, healthy, cur)
      bound = A.maxBound(w, a, t, wt, A.trans(a, h, t, ah, ht))
      limit = A.ltTrue(Nat.max(w, a), n, A.andRight(C.delivery(host), Nat.is_lt(Nat.max(w, a), n), known))
      attempt(s, Nat.max(w, a), o, bound, limit, (ah, (ht, (tn, (cur, eq)))))

def write(+s: C.State, +o: C.Outcome, +inv: C.Inv(s)) -> C.Inv(C.write(s, o)):
  match s:
    case C.System{n, a, w, h, t, xs, True{}, host}: inv
    case C.System{n, a, w, h, t, xs, False{}, host}:
      writeEnabled(s, o, Bool.and(C.delivery(host), Nat.is_lt(Nat.max(w, a), n)), {==}, inv, {==})

def persistEnabled(+s: C.State, enabled: Bool, +known: {Bool.and(C.active(C.host(s)), Nat.is_lt(C.head(s), C.seen(s))) == enabled : Bool}, +inv: C.Inv(s)) -> C.Inv(C.persistIf(s, enabled)):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} False{}: inv
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      hi = A.ltTrue(h, t, A.andRight(C.active(host), Nat.is_lt(h, t), known))
      (A.weaken(a, h, ah), (hi, (tn, (cur,
        %eq : {h <> xs == h <> _ : List<&2, Nat>}
        {==}))))

def persist(+s: C.State, +inv: C.Inv(s)) -> C.Inv(C.persist(s)):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      persistEnabled(s, Bool.and(C.active(host), Nat.is_lt(h, t)), {==}, inv)

def ackEnabled(+s: C.State, enabled: Bool, +known: {Nat.is_lt(C.acknowledged(s), C.head(s)) == enabled : Bool}, +inv: C.Inv(s)) -> C.Inv(C.ackIf(s, enabled)):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} False{}: inv
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      (A.ltTrue(a, h, known), (ht, (tn, (cur, eq))))

def ack(+s: C.State, +inv: C.Inv(s)) -> C.Inv(C.ack(s)):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      ackEnabled(s, Nat.is_lt(a, h), {==}, inv)

def quiesceEnabled(+s: C.State, enabled: Bool, +inv: C.Inv(s)) -> C.Inv(C.quiesceIf(s, enabled)):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: inv
    case C.System{n, a, w, h, t, xs, b, host} False{}: inv

def quiesce(+s: C.State, +inv: C.Inv(s)) -> C.Inv(C.quiesce(s)):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      quiesceEnabled(s, Bool.and(Bool.not(b), Nat.is_le(w, a)), inv)

def launchEnabled(+s: C.State, +id: Nat, enabled: Bool, +inv: C.Inv(s)) -> C.Inv(C.launchIf(s, id, enabled)):
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} False{}: inv
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      %Equal.sym(Nat, A.length(xs), h, A.lengthStore(h, xs, eq)) : C.Inv(C.System{n, a, 0n, _, A.length(xs), xs, False{}, G.launch(host, id)})
      %Equal.sym(Nat, A.length(xs), h, A.lengthStore(h, xs, eq)) : C.Inv(C.System{n, a, 0n, h, _, xs, False{}, G.launch(host, id)})
      (ah, (A.refl(h), (A.trans(h, t, n, ht, tn), (Unit{}, eq))))

def control(+s: C.State, +e: G.Event, +inv: C.Inv(s)) -> C.Inv(C.control(s, e)):
  match e:
    case G.Launch{id}: launchEnabled(s, id, C.canLaunch(C.host(s)), inv)
    case G.Quiesce{}: quiesce(s, inv)
    case G.Begin{}: replaceHost(s, G.begin(C.host(s)), inv)
    case G.ProbeOK{}: replaceHost(s, G.probe(C.host(s), True{}), inv)
    case G.ProbeBad{}: replaceHost(s, G.probe(C.host(s), False{}), inv)
    case G.Fence{}: replaceHost(s, G.fence(C.host(s)), inv)
    case G.Abort{}: replaceHost(s, G.abort(C.host(s)), inv)
    case G.Exit{}: replaceHost(s, G.exit(C.host(s)), inv)

def submit(+s: C.State, +inv: C.Inv(s)) -> C.Inv(C.submit(s)):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      (ah, (ht, (A.weaken(t, n, tn), (cur, eq))))

def step(+s: C.State, +e: C.Event, +inv: C.Inv(s)) -> C.Inv(C.step(s, e)):
  match e:
    case C.Submit{}: submit(s, inv)
    case C.Write{o}: write(s, o, inv)
    case C.Persist{}: persist(s, inv)
    case C.PersistFail{}: inv
    case C.Ack{}: ack(s, inv)
    case C.Control{e}: control(s, e, inv)

def run(+es: List<&2, C.Event>, +s: C.State, +inv: C.Inv(s)) -> C.Inv(C.run(es, s)):
  match es:
    case Nil{}: inv
    case e <> rest: run(rest, C.step(s, e), step(s, e, inv))

# These extracted corollaries concern the ACTUAL append history, not the queue
# counter or a proposed next-append permission.
def unique(+s: C.State, +inv: C.Inv(s)) -> {L.unique(C.store(s)) == True{} : Bool}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      %Equal.sym(List<&2, Nat>, xs, A.down(h), eq) : {L.unique(_) == True{} : Bool}
      A.uniqueDown(h)

def fifo(+s: C.State, +inv: C.Inv(s)) -> {A.descending(C.store(s)) == True{} : Bool}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      %Equal.sym(List<&2, Nat>, xs, A.down(h), eq) : {A.descending(_) == True{} : Bool}
      A.descendingDown(h)

def accounted(+id: Nat, +s: C.State, before: Bool, +known: {Nat.is_lt(id, C.acknowledged(s)) == before : Bool}, +inv: C.Inv(s)) -> {Bool.or(Bool.not(before), L.contains(id, C.store(s))) == True{} : Bool}:
  match s before:
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      %Equal.sym(List<&2, Nat>, xs, A.down(h), eq) : {L.contains(id, _) == True{} : Bool}
      %Equal.sym(Bool, L.contains(id, A.down(h)), Nat.is_lt(id, h), A.belowContains(id, h)) : {_ == True{} : Bool}
      A.leLt(a, h, id, ah, known)

def noLoss(+id: Nat, +s: C.State, +inv: C.Inv(s)) -> {C.accounted(id, s) == True{} : Bool}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      accounted(id, s, Nat.is_lt(id, a), {==}, inv)

# Host projection equalities used to lift the OWNERSHIP and corrected STATUS
# proofs to the SAME composed executions as I2/I3/I4.
def hostDelivered(+s: C.State, +id: Nat, dup: Bool, b: Bool) -> {C.host(C.delivered(s, id, dup, b)) == C.host(s) : G.State}:
  match s dup:
    case C.System{n, a, w, h, t, xs, old, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, old, host} False{}: {==}

def hostAttempt(+s: C.State, +id: Nat, o: C.Outcome) -> {C.host(C.attempt(s, id, o)) == C.host(s) : G.State}:
  match s o:
    case C.System{n, a, w, h, t, xs, b, host} C.Delivered{}: hostDelivered(s, id, Nat.is_lt(id, t), False{})
    case C.System{n, a, w, h, t, xs, b, host} C.FailedDelivered{}: hostDelivered(s, id, Nat.is_lt(id, t), True{})
    case C.System{n, a, w, h, t, xs, b, host} C.FailedLost{}: {==}

def hostWriteIf(+s: C.State, +id: Nat, o: C.Outcome, enabled: Bool) -> {C.host(C.writeIf(s, id, o, enabled)) == C.host(s) : G.State}:
  match enabled:
    case True{}: hostAttempt(s, id, o)
    case False{}: {==}

def hostWrite(+s: C.State, o: C.Outcome) -> {C.host(C.write(s, o)) == C.host(s) : G.State}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      hostWriteIf(s, Nat.max(w, a), o, Bool.and(Bool.not(b), Bool.and(C.delivery(host), Nat.is_lt(Nat.max(w, a), n))))

def hostPersistIf(+s: C.State, enabled: Bool) -> {C.host(C.persistIf(s, enabled)) == C.host(s) : G.State}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def hostPersist(+s: C.State) -> {C.host(C.persist(s)) == C.host(s) : G.State}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      hostPersistIf(s, Bool.and(C.active(host), Nat.is_lt(h, t)))

def hostAckIf(+s: C.State, enabled: Bool) -> {C.host(C.ackIf(s, enabled)) == C.host(s) : G.State}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def hostAck(+s: C.State) -> {C.host(C.ack(s)) == C.host(s) : G.State}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}: hostAckIf(s, Nat.is_lt(a, h))

def hostReplace(+s: C.State, +host: G.State) -> {C.host(C.replaceHost(s, host)) == host : G.State}:
  match s:
    case C.System{n, a, w, h, t, xs, b, old}: {==}

def hostSubmit(+s: C.State) -> {C.host(C.submit(s)) == C.host(s) : G.State}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}: {==}

def prefix(+s: C.State, inv: C.Inv(s)) -> {C.store(s) == A.down(C.head(s)) : List<&2, Nat>}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      eq

# Justification for the interval FIFO representation: a genuinely new delivered
# frame is EXACTLY the old tail, so extending the interval invents no frames.
def freshTail(+s: C.State, healthy: {C.broken(s) == False{} : Bool}, fresh: {Nat.is_lt(Nat.max(C.sent(s), C.acknowledged(s)), C.seen(s)) == False{} : Bool}, inv: C.Inv(s)) -> {Nat.max(C.sent(s), C.acknowledged(s)) == C.seen(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      (ah, (ht, (tn, (cur, eq)))) = inv
      A.antisym(Nat.max(w, a), t, A.maxBound(w, a, t, healthyCursor(b, w, t, healthy, cur), A.trans(a, h, t, ah, ht)), A.notLt(Nat.max(w, a), t, fresh))

# Journal admission positions never disappear, even on crash/abort/reset.
def acceptedDelivered(+s: C.State, +id: Nat, dup: Bool, b: Bool) -> {C.accepted(C.delivered(s, id, dup, b)) == C.accepted(s) : Nat}:
  match s dup:
    case C.System{n, a, w, h, t, xs, old, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, old, host} False{}: {==}

def acceptedAttempt(+s: C.State, +id: Nat, o: C.Outcome) -> {C.accepted(C.attempt(s, id, o)) == C.accepted(s) : Nat}:
  match s o:
    case C.System{n, a, w, h, t, xs, b, host} C.Delivered{}: acceptedDelivered(s, id, Nat.is_lt(id, t), False{})
    case C.System{n, a, w, h, t, xs, b, host} C.FailedDelivered{}: acceptedDelivered(s, id, Nat.is_lt(id, t), True{})
    case C.System{n, a, w, h, t, xs, b, host} C.FailedLost{}: {==}

def acceptedWriteIf(+s: C.State, +id: Nat, o: C.Outcome, enabled: Bool) -> {C.accepted(C.writeIf(s, id, o, enabled)) == C.accepted(s) : Nat}:
  match enabled:
    case True{}: acceptedAttempt(s, id, o)
    case False{}: {==}

def acceptedWrite(+s: C.State, o: C.Outcome) -> {C.accepted(C.write(s, o)) == C.accepted(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      acceptedWriteIf(s, Nat.max(w, a), o, Bool.and(Bool.not(b), Bool.and(C.delivery(host), Nat.is_lt(Nat.max(w, a), n))))

def acceptedPersistIf(+s: C.State, enabled: Bool) -> {C.accepted(C.persistIf(s, enabled)) == C.accepted(s) : Nat}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def acceptedPersist(+s: C.State) -> {C.accepted(C.persist(s)) == C.accepted(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      acceptedPersistIf(s, Bool.and(C.active(host), Nat.is_lt(h, t)))

def acceptedAckIf(+s: C.State, enabled: Bool) -> {C.accepted(C.ackIf(s, enabled)) == C.accepted(s) : Nat}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def acceptedAck(+s: C.State) -> {C.accepted(C.ack(s)) == C.accepted(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}: acceptedAckIf(s, Nat.is_lt(a, h))

def acceptedReplace(+s: C.State, host: G.State) -> {C.accepted(C.replaceHost(s, host)) == C.accepted(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, old}: {==}

def acceptedLaunchIf(+s: C.State, +id: Nat, enabled: Bool) -> {C.accepted(C.launchIf(s, id, enabled)) == C.accepted(s) : Nat}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def acceptedQuiesceIf(+s: C.State, enabled: Bool) -> {C.accepted(C.quiesceIf(s, enabled)) == C.accepted(s) : Nat}:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}: {==}
    case C.System{n, a, w, h, t, xs, b, host} False{}: {==}

def acceptedQuiesce(+s: C.State) -> {C.accepted(C.quiesce(s)) == C.accepted(s) : Nat}:
  match s:
    case C.System{n, a, w, h, t, xs, b, host}:
      acceptedQuiesceIf(s, Bool.and(Bool.not(b), Nat.is_le(w, a)))

def acceptedControl(+s: C.State, +e: G.Event) -> {C.accepted(C.control(s, e)) == C.accepted(s) : Nat}:
  match e:
    case G.Quiesce{}: acceptedQuiesce(s)
    case G.Launch{id}: acceptedLaunchIf(s, id, C.canLaunch(C.host(s)))
    case G.Begin{}: acceptedReplace(s, G.begin(C.host(s)))
    case G.ProbeOK{}: acceptedReplace(s, G.probe(C.host(s), True{}))
    case G.ProbeBad{}: acceptedReplace(s, G.probe(C.host(s), False{}))
    case G.Fence{}: acceptedReplace(s, G.fence(C.host(s)))
    case G.Abort{}: acceptedReplace(s, G.abort(C.host(s)))
    case G.Exit{}: acceptedReplace(s, G.exit(C.host(s)))

def acceptedSubmit(+s: C.State) -> A.Le(C.accepted(s), C.accepted(C.submit(s))):
  match s:
    case C.System{n, a, w, h, t, xs, b, host}: A.weaken(n, n, A.refl(n))

def acceptedStep(+s: C.State, +e: C.Event) -> A.Le(C.accepted(s), C.accepted(C.step(s, e))):
  match e:
    case C.Submit{}: acceptedSubmit(s)
    case C.Write{o}:
      %Equal.sym(Nat, C.accepted(C.write(s, o)), C.accepted(s), acceptedWrite(s, o)) : A.Le(C.accepted(s), _)
      A.refl(C.accepted(s))
    case C.Persist{}:
      %Equal.sym(Nat, C.accepted(C.persist(s)), C.accepted(s), acceptedPersist(s)) : A.Le(C.accepted(s), _)
      A.refl(C.accepted(s))
    case C.PersistFail{}: A.refl(C.accepted(s))
    case C.Ack{}:
      %Equal.sym(Nat, C.accepted(C.ack(s)), C.accepted(s), acceptedAck(s)) : A.Le(C.accepted(s), _)
      A.refl(C.accepted(s))
    case C.Control{ev}:
      %Equal.sym(Nat, C.accepted(C.control(s, ev)), C.accepted(s), acceptedControl(s, ev)) : A.Le(C.accepted(s), _)
      A.refl(C.accepted(s))
