import Base
import ./generation.bend as G
import ./composed.bend as C

# A separate, richer lifecycle slice. Metadata is INDEPENDENT of phase; the
# invariant is established by transitions, not by defining metadata as phase.
# Candidate means an UNUSED staged artifact (launch transfers ownership).
type Meta is Data:
  Tracking{inflight: Nat, candidate: Bool, compatible: Bool}

type State is Data:
  Attempt{host: G.State, meta: Meta, launchAudit: Bool}

def initial() -> State:
  Attempt{G.initial(), Tracking{0n, False{}, False{}}, True{}}

def phase(host: G.State) -> G.Reload:
  G.Host{live, cur, r, retired, failed, status} = host
  r

def expected(r: G.Reload) -> Meta:
  match r:
    case G.Idle{}: Tracking{0n, False{}, False{}}
    case G.Staging{}: Tracking{1n, False{}, False{}}
    case G.Staged{}: Tracking{1n, True{}, True{}}
    case G.Fenced{}: Tracking{1n, True{}, True{}}
    case G.Awaiting{}: Tracking{1n, True{}, True{}}

def count(m: Meta) -> Nat:
  Tracking{k, cand, probe} = m
  k

def candidate(m: Meta) -> Bool:
  Tracking{k, cand, probe} = m
  cand

def compatible(m: Meta) -> Bool:
  Tracking{k, cand, probe} = m
  probe

def emptyLive(host: G.State) -> Bool:
  match host:
    case G.Host{Nil{}, cur, r, retired, failed, status}: True{}
    case G.Host{x <> xs, cur, r, retired, failed, status}: False{}

def begin(+s: State) -> State:
  match s:
    case Attempt{G.Host{live, cur, G.Idle{}, retired, False{}, status}, Tracking{0n, cand, probe}, audit}:
      Attempt{G.Host{live, cur, G.Staging{}, retired, False{}, status}, Tracking{1n, False{}, False{}}, audit}
    case other: s

def probe(+s: State, ok: Bool) -> State:
  match s ok:
    case Attempt{G.Host{live, cur, G.Staging{}, retired, failed, status}, Tracking{k, cand, passed}, audit} True{}:
      Attempt{G.Host{live, cur, G.Staged{}, retired, failed, status}, Tracking{k, True{}, True{}}, audit}
    case Attempt{G.Host{live, cur, G.Staging{}, retired, failed, status}, meta, audit} False{}:
      Attempt{G.Host{live, cur, G.Idle{}, retired, failed, status}, Tracking{0n, False{}, False{}}, audit}
    case s b: s

def fence(+s: State) -> State:
  match s:
    case Attempt{G.Host{live, cur, G.Staged{}, retired, failed, status}, meta, audit}:
      Attempt{G.Host{live, cur, G.Fenced{}, retired, failed, status}, meta, audit}
    case other: s

def quiesce(s: State) -> State:
  Attempt{host, meta, audit} = s
  Attempt{G.quiesce(host), meta, audit}

def abort(s: State) -> State:
  Attempt{host, meta, audit} = s
  Attempt{G.abort(host), Tracking{0n, False{}, False{}}, audit}

def exit(s: State) -> State:
  Attempt{host, meta, audit} = s
  Attempt{G.exit(host), meta, audit}

# Record an audit of the ACTUAL old live collection on each launch. Merely
# proving a canLaunch permission would not prove the launch history safe.
def launchIf(+s: State, +id: Nat, enabled: Bool) -> State:
  match s enabled:
    case Attempt{host, meta, audit} True{}:
      Attempt{G.launch(host, id), Tracking{0n, False{}, False{}}, Bool.and(audit, emptyLive(host))}
    case s False{}: s

def launch(+s: State, +id: Nat) -> State:
  Attempt{host, meta, audit} = s
  launchIf(s, id, C.canLaunch(host))

def step(s: State, e: G.Event) -> State:
  match e:
    case G.Begin{}: begin(s)
    case G.ProbeOK{}: probe(s, True{})
    case G.ProbeBad{}: probe(s, False{})
    case G.Fence{}: fence(s)
    case G.Quiesce{}: quiesce(s)
    case G.Abort{}: abort(s)
    case G.Exit{}: exit(s)
    case G.Launch{id}: launch(s, id)

def run(es: List<&2, G.Event>, s: State) -> State:
  match es:
    case Nil{}: s
    case e <> rest: run(rest, step(s, e))

def Inv(+s: State) -> Data:
  Attempt{host, meta, audit} = s
  Sigma<&2, &2, {meta == expected(phase(host)) : Meta}, _ => {audit == True{} : Bool}>

def singleFlight(s: State) -> Bool:
  Attempt{host, meta, audit} = s
  Nat.is_le(count(meta), 1n)

def artifactSafe(s: State) -> Bool:
  match s:
    case Attempt{G.Host{live, cur, G.Idle{}, retired, failed, status}, meta, audit}:
      Bool.not(candidate(meta))
    case other: True{}

def probeSafe(+s: State) -> Bool:
  match s:
    case Attempt{G.Host{live, cur, G.Awaiting{}, retired, failed, status}, meta, audit}:
      Bool.and(candidate(meta), compatible(meta))
    case other: True{}

def launchSafe(s: State) -> Bool:
  Attempt{host, meta, audit} = s
  audit

# Four executable mutants, removing a concrete transition guard or cleanup.
type Variant is Data:
  NoCAS{}
  NoDiscard{}
  NoProbe{}
  EarlyLaunch{}
  OldStatus{}

def noCAS(+s: State) -> State:
  match s:
    case Attempt{host, Tracking{k, cand, passed}, audit}:
      Attempt{G.begin(host), Tracking{1n+k, cand, passed}, audit}

def noDiscard(s: State) -> State:
  Attempt{host, meta, audit} = s
  Attempt{G.abort(host), meta, audit}

def noProbe(+s: State) -> State:
  match s:
    case Attempt{G.Host{live, cur, G.Staging{}, retired, failed, status}, Tracking{k, cand, passed}, audit}:
      Attempt{G.Host{live, cur, G.Staged{}, retired, failed, status}, Tracking{k, True{}, False{}}, audit}
    case other: s

def earlyLaunch(+s: State, +id: Nat) -> State:
  Attempt{G.Host{live, cur, r, retired, failed, status}, meta, audit} = s
  Attempt{G.Host{id <> live, Some{id}, G.Idle{}, False{}, False{}, G.Ready{}}, Tracking{0n, False{}, False{}}, Bool.and(audit, emptyLive(G.Host{live, cur, r, retired, failed, status}))}

# Historical abandoned-retirement defect: ownership is cleared without a
# stopped publication. Correct exit above always publishes stopped.
def legacyExit(+s: State) -> State:
  match s:
    case Attempt{G.Host{live, Some{pid}, G.Idle{}, True{}, failed, status}, meta, audit}:
      Attempt{G.Host{[], None{}, G.Idle{}, True{}, failed, status}, meta, audit}
    case other: exit(s)

def statusSafe(s: State) -> Bool:
  match s:
    case Attempt{G.Host{live, None{}, r, retired, failed, G.Ready{}}, meta, audit}: False{}
    case other: True{}

def mutantStep(s: State, v: Variant, e: G.Event) -> State:
  match v e:
    case NoCAS{} G.Begin{}: noCAS(s)
    case NoDiscard{} G.Abort{}: noDiscard(s)
    case NoProbe{} G.ProbeOK{}: noProbe(s)
    case EarlyLaunch{} G.Launch{id}: earlyLaunch(s, id)
    case OldStatus{} G.Exit{}: legacyExit(s)
    case v e: step(s, e)

def mutantRun(es: List<&2, G.Event>, +v: Variant, s: State) -> State:
  match es:
    case Nil{}: s
    case e <> rest: mutantRun(rest, v, mutantStep(s, v, e))
