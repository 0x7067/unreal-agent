import Base
import ./lists.bend as L

# One actor, arbitrary Nat input identities, unbounded FIFOs and traces.
# seen is in reverse first-admission order, not a bounded ID set.
type Phase is Data:
  Open{}
  Frozen{}

type Box is Data:
  Inbox{phase: Phase, seen: List<&2, Nat>, queue: List<&2, Nat>, handed: List<&2, Nat>, delivered: List<&2, Nat>}

type Event is Data:
  Admit{id: Nat}
  Emit{}
  Freeze{}
  ProcessHanded{}
  Thaw{}

def initial() -> Box:
  Inbox{Open{}, [], [], [], []}

# Disabled commands stutter. Admit abstracts successful actor receipt;
# blocked caller submissions and duplicate receipts need no extra queue.
def admitOpen(+s: Box, +id: Nat, dup: Bool) -> Box:
  match s dup:
    case Inbox{p, seen, q, h, d} True{}:
      s
    case Inbox{p, seen, q, h, d} False{}:
      Inbox{p, id <> seen, L.append(q, [id]), h, d}

def admit(+s: Box, +id: Nat) -> Box:
  match s:
    case Inbox{Frozen{}, seen, q, h, d}:
      s
    case Inbox{Open{}, seen, q, h, d}:
      admitOpen(s, id, L.contains(id, seen))

def emit(+s: Box) -> Box:
  match s:
    case Inbox{Open{}, seen, x <> q, h, d}:
      Inbox{Open{}, seen, q, h, L.append(d, [x])}
    case other:
      s

def freeze(+s: Box) -> Box:
  match s:
    case Inbox{Open{}, seen, q, h, d}:
      Inbox{Frozen{}, seen, [], L.append(h, q), d}
    case other:
      s

def process(+s: Box) -> Box:
  match s:
    case Inbox{Frozen{}, seen, q, x <> h, d}:
      Inbox{Frozen{}, seen, q, h, L.append(d, [x])}
    case other:
      s

def thaw(+s: Box) -> Box:
  match s:
    case Inbox{Frozen{}, seen, q, Nil{}, d}:
      Inbox{Open{}, seen, q, [], d}
    case other:
      s

def step(s: Box, e: Event) -> Box:
  match e:
    case Admit{id}:
      admit(s, id)
    case Emit{}:
      emit(s)
    case Freeze{}:
      freeze(s)
    case ProcessHanded{}:
      process(s)
    case Thaw{}:
      thaw(s)

def run(es: List<&2, Event>, s: Box) -> Box:
  match es:
    case Nil{}:
      s
    case e <> rest:
      run(rest, step(s, e))

def order(+s: Box) -> List<&2, Nat>:
  Inbox{p, seen, q, h, d} = s
  L.reverse(seen)

def accounted(+s: Box) -> List<&2, Nat>:
  Inbox{p, seen, q, h, d} = s
  L.append(d, L.append(h, q))

def phaseSafe(s: Box) -> Data:
  match s:
    case Inbox{Open{}, seen, q, h, d}:
      {h == [] : List<&2, Nat>}
    case Inbox{Frozen{}, seen, q, h, d}:
      {q == [] : List<&2, Nat>}

# Conservation is an equality of entire ordered sequences, not just membership:
# every admitted ID is delivered, held by Freeze's caller, or still queued.
def Safe(+s: Box) -> Data:
  Inbox{p, seen, q, h, d} = s
  Sigma<&2, &2, {order(s) == accounted(s) : List<&2, Nat>}, _ => Sigma<&2, &2, {L.unique(seen) == True{} : Bool}, _ => phaseSafe(s)>>
