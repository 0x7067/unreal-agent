import Base

# OperationCheckpoint.tla product. Disabled actions stutter. Crash budget is
# UNBOUNDED; crashes is an independent Nat history field in State.
type Phase is Data:
  NoPhase{}
  CreateErr{}
  Process{}
  PhaseDone{}

type Active is Data:
  NoActive{}
  Creating{}
  Running{}

type Op is Data:
  Shell{}
  Remote{}

type Checkpoint is Data:
  Checkpoint{term: Bool, phase: Phase}

type Update is Data:
  Update{op: Op, checkpoint: Checkpoint}

type Core is Data:
  Core{up: Bool, phase: Phase, active: Active, errFile: Bool,
       rmap: Bool, rterm: Bool, runknown: Bool, hopen: Bool,
       queue: List<&2, Update>, shell: Checkpoint, remote: Checkpoint,
       spawns: Nat, shellWrites: Nat, remoteWrites: Nat}

type State is Data:
  State{core: Core, crashes: Nat}

type Config is Data:
  Config{exclusiveErrCreate: Bool, closeSkipsTerm: Bool}

type Boundary is Data:
  CreateErrDone{}
  ProcessExit{}
  CancelFails{}
  RemoteFinishes{}
  HandlerCloses{}
  Persist{}
  Crash{}
  Resume{}
  Stutter{}

def fixed() -> Config: Config{True{}, True{}}

def initial() -> Core:
  Core{True{}, CreateErr{}, Creating{}, False{}, True{}, False{}, False{}, True{},
       [], Checkpoint{False{}, CreateErr{}}, Checkpoint{False{}, NoPhase{}}, 0n, 0n, 0n}

def append(xs: List<&2, Update>, u: Update) -> List<&2, Update>:
  match xs:
    case Nil{}: [u]
    case x <> rest: x <> append(rest, u)

def createResult(blocked: Bool, s: Core) -> Core:
  match blocked s:
    case True{} Core{up, _, _, err, rm, rt, ru, ho, q, sh, re, n, sw, rw}:
      Core{up, PhaseDone{}, NoActive{}, err, rm, rt, ru, ho,
           append(q, Update{Shell{}, Checkpoint{True{}, PhaseDone{}}}), sh, re, n, sw, rw}
    case False{} Core{up, _, _, _, rm, rt, ru, ho, q, sh, re, n, sw, rw}:
      Core{up, Process{}, Running{}, True{}, rm, rt, ru, ho,
           append(q, Update{Shell{}, Checkpoint{False{}, Process{}}}), sh, re, 1n+n, sw, rw}

def create(c: Config, +s: Core) -> Core:
  match c s:
    case Config{ex, _} Core{True{}, CreateErr{}, Creating{}, err, _, _, _, _, _, _, _, _, _, _}:
      createResult(Bool.and(ex, err), s)
    case _ _: s

def process(+s: Core) -> Core:
  match s:
    case Core{True{}, _, Running{}, err, rm, rt, ru, ho, q, sh, re, n, sw, rw}:
      Core{True{}, PhaseDone{}, NoActive{}, err, rm, rt, ru, ho,
           append(q, Update{Shell{}, Checkpoint{True{}, PhaseDone{}}}), sh, re, n, sw, rw}
    case _: s

def cancel(+s: Core) -> Core:
  match s:
    case Core{True{}, ph, ac, err, True{}, False{}, _, ho, q, sh, re, n, sw, rw}:
      Core{True{}, ph, ac, err, True{}, True{}, True{}, ho,
           append(q, Update{Remote{}, Checkpoint{True{}, NoPhase{}}}), sh, re, n, sw, rw}
    case _: s

def finishQueue(terminal: Bool, q: List<&2, Update>) -> List<&2, Update>:
  match terminal:
    case True{}: q
    case False{}: append(q, Update{Remote{}, Checkpoint{True{}, NoPhase{}}})

def finish(+s: Core) -> Core:
  match s:
    case Core{True{}, ph, ac, err, True{}, rt, _, True{}, q, sh, re, n, sw, rw}:
      Core{True{}, ph, ac, err, False{}, True{}, False{}, True{}, finishQueue(rt, q), sh, re, n, sw, rw}
    case _: s

def closeResult(emit: Bool, s: Core) -> Core:
  match emit s:
    case True{} Core{up, ph, ac, err, rm, _, _, _, q, sh, re, n, sw, rw}:
      Core{up, ph, ac, err, rm, True{}, True{}, False{},
           append(q, Update{Remote{}, Checkpoint{True{}, NoPhase{}}}), sh, re, n, sw, rw}
    case False{} Core{up, ph, ac, err, rm, rt, ru, _, q, sh, re, n, sw, rw}:
      Core{up, ph, ac, err, rm, rt, ru, False{}, q, sh, re, n, sw, rw}

def close(c: Config, +s: Core) -> Core:
  match c s:
    case Config{_, skip} Core{True{}, _, _, _, rm, rt, _, True{}, _, _, _, _, _, _}:
      closeResult(Bool.and(rm, Bool.not(Bool.and(skip, rt))), s)
    case _ _: s

def bump(terminal: Bool, n: Nat) -> Nat:
  match terminal:
    case True{}: 1n+n
    case False{}: n

# Every head is saved unconditionally, even a duplicate terminal. The theorem
# must exclude duplicate enqueues; Persist itself has NO deduplication guard.
def persist(+s: Core) -> Core:
  match s:
    case Core{True{}, ph, ac, err, rm, rt, ru, ho, Update{Shell{}, Checkpoint{+t, p}} <> rest, _, re, n, sw, rw}:
      Core{True{}, ph, ac, err, rm, rt, ru, ho, rest, Checkpoint{t, p}, re, n, bump(t, sw), rw}
    case Core{True{}, ph, ac, err, rm, rt, ru, ho, Update{Remote{}, Checkpoint{+t, p}} <> rest, sh, _, n, sw, rw}:
      Core{True{}, ph, ac, err, rm, rt, ru, ho, rest, sh, Checkpoint{t, p}, n, sw, bump(t, rw)}
    case _: s

def crash(+s: Core) -> Core:
  match s:
    case Core{True{}, _, _, err, _, _, _, _, _, sh, re, n, sw, rw}:
      Core{False{}, NoPhase{}, NoActive{}, err, False{}, False{}, False{}, False{}, [], sh, re, n, sw, rw}
    case _: s

def restoreShell(sh: Checkpoint, ph: Phase, ac: Active, q: List<&2, Update>) -> Phase & Active & List<&2, Update>:
  match sh:
    case Checkpoint{True{}, _}: (ph, (ac, q))
    case Checkpoint{False{}, Process{}}:
      (PhaseDone{}, (NoActive{}, [Update{Shell{}, Checkpoint{True{}, PhaseDone{}}}]))
    case Checkpoint{False{}, p}: (p, (Creating{}, []))

def resumeWith(s: Core, restored: Phase & Active & List<&2, Update>) -> Core:
  match s:
    case Core{_, _, _, err, _, rt, ru, _, _, sh, Checkpoint{+t, p}, n, sw, rw}:
      match restored:
        case (ph, (ac, q)):
          Core{True{}, ph, ac, err, Bool.not(t), rt, ru, True{}, q, sh, Checkpoint{t, p}, n, sw, rw}

def resume(+s: Core) -> Core:
  match s:
    case Core{False{}, ph, ac, _, _, _, _, _, q, sh, _, _, _, _}: resumeWith(s, restoreShell(sh, ph, ac, q))
    case _: s

def next(c: Config, e: Boundary, s: Core) -> Core:
  match e:
    case CreateErrDone{}: create(c, s)
    case ProcessExit{}: process(s)
    case CancelFails{}: cancel(s)
    case RemoteFinishes{}: finish(s)
    case HandlerCloses{}: close(c, s)
    case Persist{}: persist(s)
    case Crash{}: crash(s)
    case Resume{}: resume(s)
    case Stutter{}: s

def crashCount(e: Boundary, up: Bool, n: Nat) -> Nat:
  match e up:
    case Crash{} True{}: 1n+n
    case _ _: n

def isUp(s: Core) -> Bool:
  match s:
    case Core{up, _, _, _, _, _, _, _, _, _, _, _, _, _}: up

def step(c: Config, +e: Boundary, s: State) -> State:
  match s:
    case State{+x, crashes}:
      State{next(c, e, x), crashCount(e, isUp(x), crashes)}

def run(events: List<&2, Boundary>, +c: Config, s: State) -> State:
  match events:
    case Nil{}: s
    case e <> rest: run(rest, c, step(c, e, s))

def core(s: State) -> Core:
  match s:
    case State{x, _}: x

def safe(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, _, _, _, _, n, sw, rw}:
      Bool.and(Nat.is_le(n, 1n), Bool.and(Nat.is_le(sw, 1n), Nat.is_le(rw, 1n)))

# Safety of every prefix, not just the last state.
def allSafe(events: List<&2, Boundary>, +s: Core) -> Bool:
  match events:
    case Nil{}: safe(s)
    case e <> rest: Bool.and(safe(s), allSafe(rest, next(fixed(), e, s)))


def spawnCount(s: Core) -> Nat:
  match s:
    case Core{_, _, _, _, _, _, _, _, _, _, _, n, _, _}: n

def terminalWrites(o: Op, s: Core) -> Nat:
  match o s:
    case Shell{} Core{_, _, _, _, _, _, _, _, _, _, _, _, sw, _}: sw
    case Remote{} Core{_, _, _, _, _, _, _, _, _, _, _, _, _, rw}: rw

def sameOp(a: Op, b: Op) -> Bool:
  match a b:
    case Shell{} Shell{}: True{}
    case Remote{} Remote{}: True{}
    case _ _: False{}

def pending(+o: Op, q: List<&2, Update>) -> Nat:
  match q:
    case Nil{}: 0n
    case Update{which, Checkpoint{t, _}} <> rest:
      bump(Bool.and(sameOp(o, which), t), pending(o, rest))

# Counts actual FIFO entries plus writes already committed, not an audited
# terminal flag. A crash can discard entries; Persist transfers one entry to
# history without increasing this sum. Manager actions must not allocate twice.
def queueAccounting(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, _, +q, _, _, _, sw, rw}:
      Bool.and(Nat.is_le(Nat.add(sw, pending(Shell{}, q)), 1n),
               Nat.is_le(Nat.add(rw, pending(Remote{}, q)), 1n))

def boolEq(a: Bool, b: Bool) -> Bool:
  match a b:
    case True{} True{}: True{}
    case False{} False{}: True{}
    case _ _: False{}

def durableHistory(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, _, _, Checkpoint{st, _}, Checkpoint{rt, _}, _, sw, rw}:
      Bool.and(boolEq(st, Nat.is_eq(sw, 1n)), boolEq(rt, Nat.is_eq(rw, 1n)))

def queueLength(q: List<&2, Update>) -> Nat:
  match q:
    case Nil{}: 0n
    case _ <> rest: 1n+queueLength(rest)

def queueBound(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, _, q, _, _, _, _, _}: Nat.is_le(queueLength(q), 3n)
