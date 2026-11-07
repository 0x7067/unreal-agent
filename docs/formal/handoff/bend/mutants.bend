import Base
import ./composed.bend as C
import ./generation.bend as G

# Executable counterfactuals, not supplied bad-state predicates. NoGuard is
# useful as a stronger seed stress test: replay can lag successful persistence.
# NoSeedNoGuard removes BOTH mechanisms, as in the retained TLA mutant.
type Variant is Data:
  NoGuard{}
  NoSeedNoGuard{}

def unseededIf(+s: C.State, +id: Nat, enabled: Bool) -> C.State:
  match s enabled:
    case C.System{n, a, w, h, t, xs, b, host} True{}:
      C.System{n, a, 0n, 0n, 0n, xs, False{}, G.launch(host, id)}
    case s False{}: s

def unseeded(+s: C.State, +id: Nat) -> C.State:
  unseededIf(s, id, C.canLaunch(C.host(s)))

def control(+s: C.State, v: Variant, +e: G.Event) -> C.State:
  match v e:
    case NoGuard{} G.Quiesce{}: C.replaceHost(s, G.quiesce(C.host(s)))
    case NoSeedNoGuard{} G.Quiesce{}: C.replaceHost(s, G.quiesce(C.host(s)))
    case NoSeedNoGuard{} G.Launch{id}: unseeded(s, id)
    case v e: C.control(s, e)

def step(+s: C.State, +v: Variant, +e: C.Event) -> C.State:
  match e:
    case C.Control{ev}: control(s, v, ev)
    case C.Submit{}: C.submit(s)
    case C.Write{o}: C.write(s, o)
    case C.Persist{}: C.persist(s)
    case C.PersistFail{}: s
    case C.Ack{}: C.ack(s)

def run(es: List<&2, C.Event>, +v: Variant, s: C.State) -> C.State:
  match es:
    case Nil{}: s
    case e <> rest: run(rest, v, step(s, v, e))

# Shared concrete schedules. Values are executed by reduction in paired proofs.
def replacement() -> List<&2, C.Event>:
  [C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}, C.Control{G.Exit{}}, C.Control{G.Launch{1n}}]

def replayTrace() -> List<&2, C.Event>:
  [C.Submit{}, C.Write{C.Delivered{}}, C.Persist{}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}, C.Control{G.Exit{}}, C.Control{G.Launch{1n}}, C.Write{C.Delivered{}}, C.Persist{}]

def fifoMutantTrace() -> List<&2, C.Event>:
  [C.Submit{}, C.Submit{}, C.Write{C.Delivered{}}, C.Persist{}, C.Write{C.Delivered{}}, C.Persist{}, C.Control{G.Begin{}}, C.Control{G.ProbeOK{}}, C.Control{G.Fence{}}, C.Control{G.Quiesce{}}, C.Control{G.Exit{}}, C.Control{G.Launch{1n}}, C.Write{C.Delivered{}}, C.Persist{}]
