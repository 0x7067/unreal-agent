import Base
import ./lists.bend as L

# Durable-ingress/session-store safety projection. Reversed lists retain all
# accepted/persisted identities, with arbitrary Nat IDs and no trace bound.
# Persist abstracts a completed session append. It does NOT prove the FIFO
# coordinator or that every Persist originated in an admitted input.
type State is Data:
  Ledger{journal: List<&2, Nat>, store: List<&2, Nat>, acked: List<&2, Nat>}

type Event is Data:
  Submit{id: Nat}
  Persist{id: Nat}
  Ack{id: Nat}
  Other{}

def initial() -> State:
  Ledger{[], [], []}

def submit(+s: State, id: Nat) -> State:
  Ledger{journal, store, acked} = s
  Ledger{id <> journal, store, acked}

def persist(+s: State, id: Nat) -> State:
  Ledger{journal, store, acked} = s
  Ledger{journal, id <> store, acked}

def ackIf(+s: State, id: Nat, allowed: Bool) -> State:
  match s allowed:
    case Ledger{journal, store, acked} True{}:
      Ledger{journal, store, id <> acked}
    case Ledger{journal, store, acked} False{}:
      s

def ack(+s: State, +id: Nat) -> State:
  Ledger{journal, store, acked} = s
  ackIf(s, id, L.contains(id, store))

def step(s: State, e: Event) -> State:
  match e:
    case Submit{id}:
      submit(s, id)
    case Persist{id}:
      persist(s, id)
    case Ack{id}:
      ack(s, id)
    case Other{}:
      s

def run(es: List<&2, Event>, s: State) -> State:
  match es:
    case Nil{}:
      s
    case e <> rest:
      run(rest, step(s, e))

def accepted(s: State) -> List<&2, Nat>:
  Ledger{journal, store, acked} = s
  journal

def persisted(s: State) -> List<&2, Nat>:
  Ledger{journal, store, acked} = s
  store

def acknowledged(s: State) -> List<&2, Nat>:
  Ledger{journal, store, acked} = s
  acked

# Pointwise implication, universally quantified by the public all-traces law.
def Safe(+id: Nat, s: State) -> Type:
  Ledger{journal, store, acked} = s
  {L.contains(id, acked) == True{} : Bool} -> {L.contains(id, store) == True{} : Bool}

# I2 no-loss alternative: pending means journaled and not acknowledged.
# This says every accepted ID is pending or persisted, not eventual delivery.
def accounted(+id: Nat, +s: State) -> Bool:
  Bool.or(Bool.not(L.contains(id, acknowledged(s))), L.contains(id, persisted(s)))
