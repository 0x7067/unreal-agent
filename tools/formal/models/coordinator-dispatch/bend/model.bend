import Base

# One-operation projection; the TLA model has two independent operations.
type Status is Data:
  Ready{}
  Awaiting{}
  Terminal{}

type Live is Data:
  ReadyLive{}
  AwaitingLive{}

def status(s: Live) -> Status:
  match s:
    case ReadyLive{}: Ready{}
    case AwaitingLive{}: Awaiting{}

def same(a: Status, b: Status) -> Bool:
  match a b:
    case Ready{} Ready{}: True{}
    case Awaiting{} Awaiting{}: True{}
    case Terminal{} Terminal{}: True{}
    case _ _: False{}

def nonterminal(s: Status) -> Bool:
  match s:
    case Terminal{}: False{}
    case _: True{}

# Typestate strengthening: dispatchable states have a single shared latest/local
# checkpoint. Replay's initial Ready snapshot is NOT a dispatchable state.
type State is Data:
  Translate{}
  Install{}
  Apply{}
  Append{}
  StatusReturn{latest: Status}
  Guard{latest: Status}
  Adding{latest: Live}
  Idle{latest: Status}
  UpdatePending{latest: Status, local: Status}
  UpdateReturn{latest: Status}
  Stopped{durable: Maybe<&2, Status>}
  Replay{durable: Maybe<&2, Status>}
  Overlay{latest: Status}

type Event is Data:
  Advance{}
  Add{}
  ToAwaiting{}
  ToTerminal{}
  ErrorInvisible{}
  ErrorVisible{}
  Crash{}

# The absence of a durable checkpoint is explicit, not a boolean assertion.
def durable(s: State) -> Maybe<&2, Status>:
  match s:
    case Translate{}: None{}
    case Install{}: None{}
    case Apply{}: None{}
    case Append{}: None{}
    case StatusReturn{latest}: Some{latest}
    case Guard{latest}: Some{latest}
    case Adding{latest}: Some{status(latest)}
    case Idle{latest}: Some{latest}
    case UpdatePending{latest, _}: Some{latest}
    case UpdateReturn{latest}: Some{latest}
    case Stopped{d}: d
    case Replay{d}: d
    case Overlay{latest}: Some{latest}

def guard(s: Status) -> State:
  match s:
    case Ready{}: Adding{ReadyLive{}}
    case Awaiting{}: Adding{AwaitingLive{}}
    case Terminal{}: Idle{Terminal{}}

def replay(d: Maybe<&2, Status>) -> State:
  match d:
    case None{}: Translate{}
    case Some{s}: Overlay{s}

def advance(+s: State) -> State:
  match s:
    case Translate{}: Install{}
    case Install{}: Apply{}
    case Apply{}: Append{}
    case Append{}: StatusReturn{Ready{}}
    case StatusReturn{latest}: Guard{latest}
    case Guard{latest}: guard(latest)
    case UpdatePending{_, local}: UpdateReturn{local}
    case UpdateReturn{latest}: Idle{latest}
    case Replay{d}: replay(d)
    case Overlay{latest}: Guard{latest}
    case _: s

def add(+s: State) -> State:
  match s:
    case Adding{latest}: Idle{status(latest)}
    case _: s

def awaiting(+s: State) -> State:
  match s:
    case Idle{Ready{}}: UpdatePending{Ready{}, Awaiting{}}
    case _: s

def terminal(+s: State) -> State:
  match s:
    case Idle{Ready{}}: UpdatePending{Ready{}, Terminal{}}
    case Idle{Awaiting{}}: UpdatePending{Awaiting{}, Terminal{}}
    case _: s

def invisible(+s: State) -> State:
  match s:
    case Append{}: Stopped{None{}}
    case UpdatePending{latest, _}: Stopped{Some{latest}}
    case _: s

def visible(+s: State) -> State:
  match s:
    case Append{}: Stopped{Some{Ready{}}}
    case UpdatePending{_, local}: Stopped{Some{local}}
    case _: s

def step(e: Event, s: State) -> State:
  match e:
    case Advance{}: advance(s)
    case Add{}: add(s)
    case ToAwaiting{}: awaiting(s)
    case ToTerminal{}: terminal(s)
    case ErrorInvisible{}: invisible(s)
    case ErrorVisible{}: visible(s)
    case Crash{}: Replay{durable(s)}

type Audit is Data:
  NoAdd{}
  Added{committed: Bool, local: Status, latest: Status}

def addAudit(s: State) -> Audit:
  match s:
    case Adding{+latest}: Added{True{}, status(latest), status(latest)}
    case _: NoAdd{}

def audit(e: Event, s: State) -> Audit:
  match e:
    case Add{}: addAudit(s)
    case _: NoAdd{}

def commitSafe(a: Audit) -> Bool:
  match a:
    case NoAdd{}: True{}
    case Added{committed, _, _}: committed

def checkpointSafe(a: Audit) -> Bool:
  match a:
    case NoAdd{}: True{}
    case Added{_, +local, latest}: Bool.and(nonterminal(local), same(local, latest))

def traceCommit(+events: List<&2, Event>, +s: State) -> Bool:
  match events:
    case Nil{}: True{}
    case +e <> rest: Bool.and(commitSafe(audit(e, s)), traceCommit(rest, step(e, s)))

def traceCheckpoint(+events: List<&2, Event>, +s: State) -> Bool:
  match events:
    case Nil{}: True{}
    case +e <> rest: Bool.and(checkpointSafe(audit(e, s)), traceCheckpoint(rest, step(e, s)))

def run(events: List<&2, Event>, s: State) -> State:
  match events:
    case Nil{}: s
    case e <> rest: run(rest, step(e, s))
