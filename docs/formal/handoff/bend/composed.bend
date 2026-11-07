import Base
import ./generation.bend as G
import ./arithmetic.bend as A
import ./lists.bend as L

# An interval FIFO [head, seen) encodes the ascending journal positions held
# by this inbox. Seen is the dedup prefix, seeded from the actual resumed store.
# Store is independent, reverse append history, never synthesized from head.
# Host writes and ordered worker receipts are one atomic action in this model.
type State is Data:
  System{accepted: Nat, acked: Nat, sent: Nat, head: Nat, seen: Nat, store: List<&2, Nat>, broken: Bool, host: G.State}

type Outcome is Data:
  Delivered{}
  FailedLost{}
  FailedDelivered{}

type Event is Data:
  Submit{}
  Write{outcome: Outcome}
  Persist{}
  PersistFail{}
  Ack{}
  Control{event: G.Event}

def initial() -> State:
  System{0n, 0n, 0n, 0n, 0n, [], False{}, G.initial()}

def store(s: State) -> List<&2, Nat>:
  System{n, a, w, h, t, xs, b, host} = s
  xs

def host(s: State) -> G.State:
  System{n, a, w, h, t, xs, b, host} = s
  host

def replaceHost(s: State, host: G.State) -> State:
  System{n, a, w, h, t, xs, b, old} = s
  System{n, a, w, h, t, xs, b, host}

def active(host: G.State) -> Bool:
  match host:
    case G.Host{live, Some{pid}, r, False{}, failed, status}: True{}
    case other: False{}

def unfenced(r: G.Reload) -> Bool:
  match r:
    case G.Idle{}: True{}
    case G.Staging{}: True{}
    case G.Staged{}: True{}
    case G.Fenced{}: False{}
    case G.Awaiting{}: False{}

def delivery(+host: G.State) -> Bool:
  G.Host{live, cur, r, retired, failed, status} = host
  Bool.and(active(host), unfenced(r))

# Mark-before-write, including ambiguous errors. A broken pipe cannot send a
# later frame. FailedDelivered permits receipt despite the error/cancellation;
# FailedLost does not. Either can still be followed by crash and durable replay
# accounting, but crash latches G.failed and prevents automatic replacement.
def delivered(+s: State, +id: Nat, dup: Bool, broken: Bool) -> State:
  match s dup:
    case System{n, a, w, h, t, xs, b, host} True{}:
      System{n, a, 1n+id, h, t, xs, broken, host}
    case System{n, a, w, h, t, xs, b, host} False{}:
      System{n, a, 1n+id, h, 1n+id, xs, broken, host}

def attempt(+s: State, +id: Nat, outcome: Outcome) -> State:
  match s outcome:
    case System{n, a, w, h, t, xs, b, host} Delivered{}:
      delivered(s, id, Nat.is_lt(id, t), False{})
    case System{n, a, w, h, t, xs, b, host} FailedDelivered{}:
      delivered(s, id, Nat.is_lt(id, t), True{})
    case System{n, a, w, h, t, xs, b, host} FailedLost{}:
      System{n, a, 1n+id, h, t, xs, True{}, host}

def writeIf(+s: State, id: Nat, outcome: Outcome, enabled: Bool) -> State:
  match enabled:
    case True{}: attempt(s, id, outcome)
    case False{}: s

def write(+s: State, outcome: Outcome) -> State:
  System{n, a, w, h, t, xs, b, host} = s
  +id = Nat.max(w, a)
  writeIf(s, id, outcome, Bool.and(Bool.not(b), Bool.and(delivery(host), Nat.is_lt(id, n))))

# A successful store append consumes precisely the inbox head. Failure leaves
# both store and FIFO unchanged; it cannot create an acknowledgement. This is
# an atomic append abstraction, not a claim about ambiguous filesystem writes.
def persistIf(+s: State, enabled: Bool) -> State:
  match s enabled:
    case System{n, a, w, h, t, xs, b, host} True{}:
      System{n, a, w, 1n+h, t, h <> xs, b, host}
    case System{n, a, w, h, t, xs, b, host} False{}: s

def persist(+s: State) -> State:
  System{n, a, w, h, t, xs, b, host} = s
  persistIf(s, Bool.and(active(host), Nat.is_lt(h, t)))

def ackIf(+s: State, enabled: Bool) -> State:
  match s enabled:
    case System{n, a, w, h, t, xs, b, host} True{}:
      System{n, 1n+a, w, h, t, xs, b, host}
    case System{n, a, w, h, t, xs, b, host} False{}: s

def ack(+s: State) -> State:
  System{n, a, w, h, t, xs, b, host} = s
  ackIf(s, Nat.is_lt(a, h))

# Only pending entries at or after acked are candidates for Write. Acks are an
# ordered prefix here, while the TLA/Go journal allows arbitrary persisted acks.
# The persistence guard refuses before control delivery if a sent entry remains
# unacknowledged. Busy refusal discards the attempt through G.Abort.
def quiesceIf(+s: State, enabled: Bool) -> State:
  match s enabled:
    case System{n, a, w, h, t, xs, b, host} True{}:
      replaceHost(s, G.quiesce(host))
    case System{n, a, w, h, t, xs, b, host} False{}:
      replaceHost(s, G.abort(host))

def quiesce(+s: State) -> State:
  System{n, a, w, h, t, xs, b, host} = s
  quiesceIf(s, Bool.and(Bool.not(b), Nat.is_le(w, a)))

# The live/current/retirement checks are the same as the separately proved
# generation slice. Every replacement reconstructs head AND seen from store.
def canLaunch(host: G.State) -> Bool:
  match host:
    case G.Host{Nil{}, None{}, G.Awaiting{}, True{}, False{}, status}: True{}
    case G.Host{Nil{}, None{}, G.Fenced{}, True{}, False{}, status}: True{}
    case other: False{}

def launchIf(+s: State, +id: Nat, enabled: Bool) -> State:
  match s enabled:
    case System{n, a, w, h, t, xs, b, host} True{}:
      System{n, a, 0n, A.length(xs), A.length(xs), xs, False{}, G.launch(host, id)}
    case s False{}: s

def launch(+s: State, +id: Nat) -> State:
  launchIf(s, id, canLaunch(host(s)))

def control(+s: State, +e: G.Event) -> State:
  match e:
    case G.Launch{id}: launch(s, id)
    case G.Quiesce{}: quiesce(s)
    case other: replaceHost(s, G.step(host(s), e))

def submit(s: State) -> State:
  System{n, a, w, h, t, xs, b, host} = s
  System{1n+n, a, w, h, t, xs, b, host}

def step(+s: State, e: Event) -> State:
  match e:
    case Submit{}: submit(s)
    case Write{o}: write(s, o)
    case Persist{}: persist(s)
    case PersistFail{}: s
    case Ack{}: ack(s)
    case Control{e}: control(s, e)

def run(es: List<&2, Event>, s: State) -> State:
  match es:
    case Nil{}: s
    case e <> rest: run(rest, step(s, e))

# Independently stored queue head/store, journal bounds, and per-generation
# cursor bounds. Broken cursors may point beyond seen because a write was lost.
def Cursor(broken: Bool, w: Nat, t: Nat) -> Data:
  match broken:
    case False{}: A.Le(w, t)
    case True{}: Unit

def Inv(+s: State) -> Data:
  System{n, a, w, h, t, xs, b, host} = s
  Sigma<&2, &2, A.Le(a, h), _ => Sigma<&2, &2, A.Le(h, t), _ => Sigma<&2, &2, A.Le(t, n), _ => Sigma<&2, &2, Cursor(b, w, t), _ => {xs == A.down(h) : List<&2, Nat>}>>>>

# Pending-or-persisted for every identity; restricting to id < accepted gives
# I2. Acknowledging is the ONLY way an accepted ID leaves durable pending.
def accounted(+id: Nat, s: State) -> Bool:
  System{n, a, w, h, t, xs, b, host} = s
  Bool.or(Bool.not(Nat.is_lt(id, a)), L.contains(id, xs))

def seen(s: State) -> Nat:
  System{n, a, w, h, t, xs, b, host} = s
  t

def accepted(s: State) -> Nat:
  System{n, a, w, h, t, xs, b, host} = s
  n

def sent(s: State) -> Nat:
  System{n, a, w, h, t, xs, b, host} = s
  w

def acknowledged(s: State) -> Nat:
  System{n, a, w, h, t, xs, b, host} = s
  a

def head(s: State) -> Nat:
  System{n, a, w, h, t, xs, b, host} = s
  h

def broken(s: State) -> Bool:
  System{n, a, w, h, t, xs, b, host} = s
  b
