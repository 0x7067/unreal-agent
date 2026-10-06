import Base

# Projection of CreateErrDone. True means an eligible create_err completion;
# False is any non-spawning boundary (including crash/resume/persist). The file
# bit is durable and EXCLUSIVE creation consumes it before process launch.
# Ignoring phase/up guards admits MORE attempts than OperationCheckpoint.tla.
def shellSpawns(+attempts: List<&2, Bool>, errFile: Bool) -> Nat:
  match attempts:
    case Nil{}: 0n
    case attempt <> rest:
      match attempt errFile:
        case False{} True{}: shellSpawns(rest, True{})
        case True{} True{}: shellSpawns(rest, True{})
        case False{} False{}: shellSpawns(rest, False{})
        case True{} False{}: 1n+shellSpawns(rest, True{})

# Legacy useExistingPath reuses the file and can launch on every retry.
def legacySpawns(attempts: List<&2, Bool>) -> Nat:
  match attempts:
    case Nil{}: 0n
    case attempt <> rest:
      match attempt:
        case False{}: legacySpawns(rest)
        case True{}: 1n+legacySpawns(rest)

type Location is Data:
  Retained{}
  Released{}
  Down{}

type Remote is Data:
  Active{}
  UnknownQueued{}
  ReleasedQueued{}
  Stored{location: Location}
  DownUnstored{}

type Event is Data:
  CancelFails{}
  RemoteFinishes{}
  HandlerCloses{}
  Persist{}
  Crash{}
  Resume{}

# UnknownQueued is a terminal remoteUnknown still in the manager map.
# HandlerCloses must not enqueue another terminal update from that state.
def cancel(+s: Remote) -> Remote:
  match s:
    case Active{}: UnknownQueued{}
    case _: s

def finish(+s: Remote) -> Remote:
  match s:
    case Active{}: ReleasedQueued{}
    case UnknownQueued{}: ReleasedQueued{}
    case Stored{Retained{}}: Stored{Released{}}
    case _: s

def close(+s: Remote) -> Remote:
  match s:
    case Active{}: UnknownQueued{}
    case _: s

def persist(+s: Remote) -> Remote:
  match s:
    case UnknownQueued{}: Stored{Retained{}}
    case ReleasedQueued{}: Stored{Released{}}
    case _: s

def crash(s: Remote) -> Remote:
  match s:
    case Stored{_}: Stored{Down{}}
    case _: DownUnstored{}

def resume(+s: Remote) -> Remote:
  match s:
    case DownUnstored{}: Active{}
    case Stored{Down{}}: Stored{Released{}}
    case _: s

def step(e: Event, s: Remote) -> Remote:
  match e:
    case CancelFails{}: cancel(s)
    case RemoteFinishes{}: finish(s)
    case HandlerCloses{}: close(s)
    case Persist{}: persist(s)
    case Crash{}: crash(s)
    case Resume{}: resume(s)

# Counts actual durable terminal writes, not merely local terminal states.
def write(e: Event, s: Remote) -> Nat:
  match e:
    case Persist{}:
      match s:
        case UnknownQueued{}: 1n
        case ReleasedQueued{}: 1n
        case _: 0n
    case _: 0n

def terminalWrites(+events: List<&2, Event>, +s: Remote) -> Nat:
  match events:
    case Nil{}: 0n
    case +e <> rest: Nat.add(write(e, s), terminalWrites(rest, step(e, s)))

# Local negative-control queue projection. Cancellation failure retains the map
# entry, marks terminal, and enqueues once. Legacy close enqueues it again.
type CloseState is Data:
  CloseState{retained: Bool, terminal: Bool, queued: Nat}

def cancelQueue(s: CloseState) -> CloseState:
  match s:
    case CloseState{True{}, False{}, q}: CloseState{True{}, True{}, 1n+q}
    case CloseState{r, t, q}: CloseState{r, t, q}

def closeQueueGuard(skipTerminal: Bool, terminal: Bool, queued: Nat) -> Nat:
  match skipTerminal terminal:
    case True{} True{}: queued
    case _ _: 1n+queued

def closeQueue(skipTerminal: Bool, s: CloseState) -> CloseState:
  match s:
    case CloseState{True{}, +t, q}: CloseState{True{}, True{}, closeQueueGuard(skipTerminal, t, q)}
    case CloseState{False{}, t, q}: CloseState{False{}, t, q}

def queueCount(s: CloseState) -> Nat:
  match s:
    case CloseState{_, _, q}: q

def drainTerminalQueue(n: Nat) -> Nat:
  match n:
    case 0n: 0n
    case 1n+p: 1n+drainTerminalQueue(p)

def cancelThenCloseWrites(skipTerminal: Bool) -> Nat:
  drainTerminalQueue(queueCount(closeQueue(skipTerminal, cancelQueue(CloseState{True{}, False{}, 0n}))))
