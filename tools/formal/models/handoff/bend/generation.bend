import Base

# Independent live-process collection and current pointer, not one field
# derived from the other. Unbounded generation IDs, at most one process per
# reachable state. A process remains live during admitted retirement.
type Reload is Data:
  Idle{}
  Staging{}
  Staged{}
  Fenced{}
  Awaiting{}

type Status is Data:
  Ready{}
  Stopped{}

type State is Data:
  Host{live: List<&2, Nat>, current: Maybe<&2, Nat>, reload: Reload, retired: Bool, failed: Bool, status: Status}

type Event is Data:
  Begin{}
  ProbeOK{}
  ProbeBad{}
  Fence{}
  Quiesce{}
  Abort{}
  Exit{}
  Launch{id: Nat}

def initial() -> State:
  Host{[0n], Some{0n}, Idle{}, False{}, False{}, Ready{}}

def ownerList(owner: Maybe<&2, Nat>) -> List<&2, Nat>:
  match owner:
    case None{}:
      []
    case Some{pid}:
      [pid]

def Own(s: State) -> Data:
  Host{live, current, r, retired, failed, status} = s
  {live == ownerList(current) : List<&2, Nat>}

def statusSafeCur(cur: Maybe<&2, Nat>, status: Status) -> Data:
  match cur:
    case None{}:
      {status == Stopped{} : Status}
    case Some{pid}:
      Unit

def StatusSafe(s: State) -> Data:
  Host{live, cur, r, retired, failed, status} = s
  statusSafeCur(cur, status)

# A reload remains single-flight through every phase. Stage acceptance stands
# for a successful compatible identity probe, not a proof of the probe IO.
def begin(+s: State) -> State:
  match s:
    case Host{live, cur, Idle{}, retired, False{}, status}:
      Host{live, cur, Staging{}, retired, False{}, status}
    case other:
      s

def probe(+s: State, ok: Bool) -> State:
  match s ok:
    case Host{live, cur, Staging{}, retired, failed, status} True{}:
      Host{live, cur, Staged{}, retired, failed, status}
    case Host{live, cur, Staging{}, retired, failed, status} False{}:
      Host{live, cur, Idle{}, retired, failed, status}
    case other b:
      s

def fence(+s: State) -> State:
  match s:
    case Host{live, cur, Staged{}, retired, failed, status}:
      Host{live, cur, Fenced{}, retired, failed, status}
    case other:
      s

def quiesce(+s: State) -> State:
  match s:
    case Host{live, Some{pid}, Fenced{}, retired, failed, status}:
      Host{live, Some{pid}, Awaiting{}, True{}, failed, status}
    case other:
      s

def abort(s: State) -> State:
  Host{live, cur, r, retired, failed, status} = s
  Host{live, cur, Idle{}, retired, failed, status}

# Reap the current generation, including after a caller abandoned its reload.
# Corrected publication is explicit: stopped is published even after Abort.
# Crashes latch failure, so they cannot enable a later Launch.
def exit(+s: State) -> State:
  Host{live, cur, r, retired, failed, status} = s
  Host{[], None{}, r, retired, Bool.or(failed, Bool.not(retired)), Stopped{}}

# A replacement cannot start until the independently tracked live collection
# is empty. No worker + proven retirement also permits explicit recovery after
# an abandoned handoff. As in the Lean model, hello/ready is abstract here.
def launch(+s: State, +id: Nat) -> State:
  match s:
    case Host{Nil{}, None{}, Awaiting{}, True{}, False{}, status}:
      Host{[id], Some{id}, Idle{}, False{}, False{}, Ready{}}
    case Host{Nil{}, None{}, Fenced{}, True{}, False{}, status}:
      Host{[id], Some{id}, Idle{}, False{}, False{}, Ready{}}
    case other:
      s

def step(s: State, e: Event) -> State:
  match e:
    case Begin{}:
      begin(s)
    case ProbeOK{}:
      probe(s, True{})
    case ProbeBad{}:
      probe(s, False{})
    case Fence{}:
      fence(s)
    case Quiesce{}:
      quiesce(s)
    case Abort{}:
      abort(s)
    case Exit{}:
      exit(s)
    case Launch{id}:
      launch(s, id)

def run(es: List<&2, Event>, s: State) -> State:
  match es:
    case Nil{}:
      s
    case e <> rest:
      run(rest, step(s, e))

# These projections make one case analysis suffice for every lifecycle event.
def current(s: State) -> Maybe<&2, Nat>:
  Host{live, cur, r, retired, failed, status} = s
  cur

def live(s: State) -> List<&2, Nat>:
  Host{live, cur, r, retired, failed, status} = s
  live

def Safe(+s: State) -> Data:
  Sigma<&2, &2, Own(s), _ => StatusSafe(s)>
