import Base
import ./product.bend as P

# Full product executions, not the legacy projection counters.
def spawnEvents() -> List<&2, P.Boundary>:
  [P.CreateErrDone{}, P.Crash{}, P.Resume{}, P.CreateErrDone{}]

def closeEvents() -> List<&2, P.Boundary>:
  [P.CancelFails{}, P.HandlerCloses{}, P.Persist{}, P.Persist{}]

def legacySpawn() -> P.Config: P.Config{False{}, True{}}
def legacyClose() -> P.Config: P.Config{True{}, False{}}

def replay(events: List<&2, P.Boundary>, +c: P.Config, +s: P.State) -> List<&2, P.State>:
  match events:
    case Nil{}: [s]
    case e <> rest: s <> replay(rest, c, P.step(c, e, s))

def spawnTrace() -> List<&2, P.State>:
  replay(spawnEvents(), legacySpawn(), P.State{P.initial(), 0n})

def closeTrace() -> List<&2, P.State>:
  replay(closeEvents(), legacyClose(), P.State{P.initial(), 0n})

def finalSpawn(c: P.Config) -> P.Core:
  P.core(P.run(spawnEvents(), c, P.State{P.initial(), 0n}))

def finalClose(c: P.Config) -> P.Core:
  P.core(P.run(closeEvents(), c, P.State{P.initial(), 0n}))

# One shared FIFO contains a shell progress checkpoint, remote terminal, and
# shell terminal, in that order. No per-operation queue or persist guard.
def interleaved() -> List<&2, P.Boundary>:
  [P.CreateErrDone{}, P.CancelFails{}, P.ProcessExit{}]

def afterInterleaving() -> P.Core:
  P.core(P.run(interleaved(), P.fixed(), P.State{P.initial(), 0n}))

def storedProcessResume() -> P.Core:
  P.core(P.run([P.CreateErrDone{}, P.Persist{}, P.Crash{}, P.Resume{}, P.Persist{}],
              P.fixed(), P.State{P.initial(), 0n}))

def droppedRemoteTerminal() -> P.Core:
  P.core(P.run([P.CancelFails{}, P.Crash{}, P.Resume{}, P.HandlerCloses{}, P.Persist{}],
              P.fixed(), P.State{P.initial(), 0n}))
