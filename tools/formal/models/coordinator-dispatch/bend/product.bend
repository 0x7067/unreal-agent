import Base

# Independent two-operation record port of CoordinatorDispatch.tla. Tick chooses
# the unique ordinary action enabled by pc/cursor; error/update/crash choices
# are separate events. Disabled events stutter, as in [Next]_vars.
type Status is Data:
  Absent{}
  Ready{}
  Awaiting{}
  Terminal{}

type Snapshots is Data:
  Snapshots{one: Status, two: Status}

type Flags is Data:
  Flags{one: Bool, two: Bool}

type Cursor is Data:
  One{}
  Two{}
  End{}

type PC is Data:
  Translate{}
  Install{}
  AppendStatus{}
  StatusReturn{}
  Guard{}
  Add{}
  Idle{}
  AppendUpdate{}
  UpdateReturn{}
  Stopped{}
  Replay{}
  Overlay{}

type Core is Data:
  Core{pc: PC, cursor: Cursor, committed: Bool, latest: Snapshots, local: Snapshots,
       localStatus: Bool, added: Flags, badCommit: Bool, badCheckpoint: Bool}

type State is Data:
  State{core: Core, crashes: Nat}

type Config is Data:
  Config{skipCommit: Bool, skipTerminal: Bool, skipOverlay: Bool}

type Boundary is Data:
  Tick{}
  StatusInvisible{}
  StatusVisible{}
  UpdateOneAwaiting{}
  UpdateOneTerminal{}
  UpdateTwoAwaiting{}
  UpdateTwoTerminal{}
  UpdateInvisible{}
  UpdateVisible{}
  Crash{}
  Stutter{}

def fixed() -> Config: Config{False{}, False{}, False{}}
def empty() -> Snapshots: Snapshots{Absent{}, Absent{}}
def ready() -> Snapshots: Snapshots{Ready{}, Ready{}}
def noneAdded() -> Flags: Flags{False{}, False{}}
def initial() -> Core:
  Core{Translate{}, One{}, False{}, empty(), empty(), False{}, noneAdded(), False{}, False{}}

def get(p: Snapshots, i: Cursor) -> Status:
  match p i:
    case Snapshots{a, _} One{}: a
    case Snapshots{_, b} Two{}: b
    case _ End{}: Absent{}

def put(p: Snapshots, i: Cursor, v: Status) -> Snapshots:
  match p i:
    case Snapshots{_, b} One{}: Snapshots{v, b}
    case Snapshots{a, _} Two{}: Snapshots{a, v}
    case Snapshots{a, b} End{}: Snapshots{a, b}

def marked(p: Flags, i: Cursor) -> Bool:
  match p i:
    case Flags{a, _} One{}: a
    case Flags{_, b} Two{}: b
    case _ End{}: False{}

def mark(p: Flags, i: Cursor) -> Flags:
  match p i:
    case Flags{_, b} One{}: Flags{True{}, b}
    case Flags{a, _} Two{}: Flags{a, True{}}
    case Flags{a, b} End{}: Flags{a, b}

def succ(i: Cursor) -> Cursor:
  match i:
    case One{}: Two{}
    case Two{}: End{}
    case End{}: End{}

def same(a: Status, b: Status) -> Bool:
  match a b:
    case Absent{} Absent{}: True{}
    case Ready{} Ready{}: True{}
    case Awaiting{} Awaiting{}: True{}
    case Terminal{} Terminal{}: True{}
    case _ _: False{}

def terminal(a: Status) -> Bool:
  match a:
    case Terminal{}: True{}
    case _: False{}

def progresses(a: Status, b: Status) -> Bool:
  match a b:
    case Ready{} Awaiting{}: True{}
    case Ready{} Terminal{}: True{}
    case Awaiting{} Terminal{}: True{}
    case _ _: False{}

def choosePair(b: Bool, yes: Snapshots, no: Snapshots) -> Snapshots:
  match b:
    case True{}: yes
    case False{}: no

def guardPC(skip: Bool, v: Status) -> PC:
  match skip v:
    case False{} Terminal{}: Guard{}
    case _ _: Add{}

def guardCursor(skip: Bool, v: Status, i: Cursor) -> Cursor:
  match skip v:
    case False{} Terminal{}: succ(i)
    case _ _: i

def schedule(b: Bool) -> PC:
  match b:
    case True{}: Guard{}
    case False{}: Translate{}

# This is an actual transition, not an audit assembled with committed=True.
# In particular Add copies the independent committed/local/latest fields.
def tick(c: Config, +s: Core) -> Core:
  match c s:
    case Config{+sc, +st, so} Core{pc, +i, +co, +la, +lo, ls, a, bc, bk}:
      match pc i:
        case Translate{} _:
          Core{Install{}, One{}, co, la, lo, ls, a, bc, bk}
        case Install{} End{}:
          Core{AppendStatus{}, End{}, co, la, lo, True{}, a, bc, bk}
        case Install{} _:
          Core{Install{}, succ(i), co, la, put(lo, i, Ready{}), ls, a, bc, bk}
        case AppendStatus{} _:
          Core{StatusReturn{}, i, Bool.or(co, Bool.not(sc)), choosePair(sc, la, ready()), lo, ls, a, bc, bk}
        case StatusReturn{} _:
          Core{Guard{}, One{}, co, la, lo, ls, a, bc, bk}
        case Guard{} End{}:
          Core{Idle{}, End{}, co, la, lo, ls, a, bc, bk}
        case Guard{} _:
          Core{guardPC(st, get(lo, i)), guardCursor(st, get(lo, i), i), co, la, lo, ls, a, bc, bk}
        case Add{} End{}: s
        case Add{} _:
          Core{Guard{}, succ(i), co, la, lo, ls, mark(a, i), Bool.or(bc, Bool.not(co)),
               Bool.or(bk, Bool.or(terminal(get(lo, i)), Bool.not(same(get(la, i), get(lo, i)))))}
        case AppendUpdate{} _:
          Core{UpdateReturn{}, i, co, put(la, i, get(lo, i)), lo, ls, a, bc, bk}
        case UpdateReturn{} _:
          Core{Idle{}, i, co, la, lo, ls, a, bc, bk}
        case Replay{} End{}:
          Core{Overlay{}, One{}, co, la, lo, co, a, bc, bk}
        case Replay{} _:
          Core{Replay{}, succ(i), co, la, choosePair(co, put(lo, i, Ready{}), lo), ls, a, bc, bk}
        case Overlay{} End{}:
          Core{schedule(ls), One{}, co, la, lo, ls, a, bc, bk}
        case Overlay{} _:
          Core{Overlay{}, succ(i), co, la, choosePair(Bool.and(co, Bool.not(so)), put(lo, i, get(la, i)), lo), ls, a, bc, bk}
        case _ _: s

def statusError(+visible: Bool, +s: Core) -> Core:
  match s:
    case Core{AppendStatus{}, i, co, la, lo, ls, a, bc, bk}:
      Core{Stopped{}, i, Bool.or(co, visible), choosePair(visible, ready(), la), lo, ls, a, bc, bk}
    case _: s

def updateResult(enabled: Bool, +i: Cursor, v: Status, +s: Core) -> Core:
  match enabled s:
    case True{} Core{_, _, co, la, lo, ls, a, bc, bk}:
      Core{AppendUpdate{}, i, co, la, put(lo, i, v), ls, a, bc, bk}
    case False{} _: s

def update(+i: Cursor, +v: Status, +s: Core) -> Core:
  match s:
    case Core{Idle{}, _, +co, _, +lo, _, +a, _, _}:
      updateResult(Bool.and(co, Bool.and(marked(a, i), progresses(get(lo, i), v))), i, v, s)
    case _: s

def updateError(visible: Bool, +s: Core) -> Core:
  match s:
    case Core{AppendUpdate{}, +i, co, +la, +lo, ls, a, bc, bk}:
      Core{Stopped{}, i, co, choosePair(visible, put(la, i, get(lo, i)), la), lo, ls, a, bc, bk}
    case _: s

def crash(s: Core) -> Core:
  match s:
    case Core{_, _, co, la, _, _, _, bc, bk}:
      Core{Replay{}, One{}, co, la, empty(), False{}, noneAdded(), bc, bk}

def next(c: Config, e: Boundary, s: Core) -> Core:
  match e:
    case Tick{}: tick(c, s)
    case StatusInvisible{}: statusError(False{}, s)
    case StatusVisible{}: statusError(True{}, s)
    case UpdateOneAwaiting{}: update(One{}, Awaiting{}, s)
    case UpdateOneTerminal{}: update(One{}, Terminal{}, s)
    case UpdateTwoAwaiting{}: update(Two{}, Awaiting{}, s)
    case UpdateTwoTerminal{}: update(Two{}, Terminal{}, s)
    case UpdateInvisible{}: updateError(False{}, s)
    case UpdateVisible{}: updateError(True{}, s)
    case Crash{}: crash(s)
    case Stutter{}: s

def safe(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, bc, bk}: Bool.and(Bool.not(bc), Bool.not(bk))

def crashCount(e: Boundary, n: Nat) -> Nat:
  match e:
    case Crash{}: 1n+n
    case _: n

def step(c: Config, +e: Boundary, s: State) -> State:
  match s:
    case State{x, n}: State{next(c, e, x), crashCount(e, n)}

def core(s: State) -> Core:
  match s:
    case State{x, _}: x

def run(events: List<&2, Boundary>, +c: Config, s: State) -> State:
  match events:
    case Nil{}: s
    case e <> rest: run(rest, c, step(c, e, s))

def allSafe(events: List<&2, Boundary>, +s: Core) -> Bool:
  match events:
    case Nil{}: safe(s)
    case e <> rest: Bool.and(safe(s), allSafe(rest, next(fixed(), e, s)))

# Readable companion to the finite relational invariant. These predicates refer
# to independent fields, and explicitly describe partial replay/overlay.
def sameSnapshots(a: Snapshots, b: Snapshots) -> Bool:
  match a b:
    case Snapshots{x, y} Snapshots{u, v}: Bool.and(same(x, u), same(y, v))

def noAdded(a: Flags) -> Bool:
  match a:
    case Flags{x, y}: Bool.and(Bool.not(x), Bool.not(y))

def durableShape(co: Bool, p: Snapshots) -> Bool:
  match co p:
    case False{} Snapshots{Absent{}, Absent{}}: True{}
    case True{} Snapshots{Absent{}, _}: False{}
    case True{} Snapshots{_, Absent{}}: False{}
    case True{} _: True{}
    case _ _: False{}

def replayPrefix(co: Bool, i: Cursor) -> Snapshots:
  match co i:
    case False{} _: empty()
    case True{} One{}: empty()
    case True{} Two{}: Snapshots{Ready{}, Absent{}}
    case True{} End{}: ready()

def overlayPrefix(co: Bool, i: Cursor, la: Snapshots) -> Snapshots:
  match co i la:
    case False{} _ _: empty()
    case True{} One{} _: ready()
    case True{} Two{} Snapshots{x, _}: Snapshots{x, Ready{}}
    case True{} End{} p: p

def validCursor(i: Cursor) -> Bool:
  match i:
    case End{}: False{}
    case _: True{}

def dispatchRelation(+i: Cursor, co: Bool, +la: Snapshots, +lo: Snapshots, ls: Bool) -> Bool:
  Bool.and(validCursor(i), Bool.and(co, Bool.and(ls,
    Bool.and(sameSnapshots(la, lo), Bool.not(terminal(get(lo, i)))))))

def sameBool(a: Bool, b: Bool) -> Bool:
  match a b:
    case True{} True{}: True{}
    case False{} False{}: True{}
    case _ _: False{}

def phaseRelation(s: Core) -> Bool:
  match s:
    case Core{pc, +i, +co, +la, +lo, +ls, a, _, _}:
      match pc:
        case Translate{}:
          Bool.and(Bool.not(co), Bool.and(Bool.not(ls), Bool.and(noAdded(a), sameSnapshots(lo, empty()))))
        case Install{}:
          Bool.and(Bool.not(co), Bool.and(Bool.not(ls), Bool.and(noAdded(a), sameSnapshots(lo, replayPrefix(True{}, i)))))
        case AppendStatus{}:
          Bool.and(Bool.not(co), Bool.and(ls, Bool.and(noAdded(a), sameSnapshots(lo, ready()))))
        case StatusReturn{}:
          Bool.and(co, Bool.and(ls, Bool.and(sameSnapshots(la, ready()), sameSnapshots(lo, ready()))))
        case Guard{}: Bool.and(co, Bool.and(ls, sameSnapshots(lo, la)))
        case Add{}: dispatchRelation(i, co, la, lo, ls)
        case Idle{}: Bool.and(co, Bool.and(ls, sameSnapshots(lo, la)))
        case AppendUpdate{}:
          Bool.and(co, Bool.and(ls, Bool.and(validCursor(i), Bool.and(marked(a, i), sameSnapshots(put(la, i, get(lo, i)), lo)))))
        case UpdateReturn{}: Bool.and(co, Bool.and(ls, sameSnapshots(lo, la)))
        case Stopped{}: True{}
        case Replay{}:
          Bool.and(Bool.not(ls), Bool.and(noAdded(a), sameSnapshots(lo, replayPrefix(co, i))))
        case Overlay{}:
          Bool.and(sameBool(ls, co), Bool.and(noAdded(a), sameSnapshots(lo, overlayPrefix(co, i, la))))

def consistent(+s: Core) -> Bool:
  match s:
    case Core{_, _, co, la, _, _, _, _, _}: Bool.and(durableShape(co, la), phaseRelation(s))

# Exact MaxCrashes guard is available as well as the unbounded generalization.
# Only Crash is gated; the budget never bounds event-list induction.
def budgetEvent(allowed: Bool, +e: Boundary) -> Boundary:
  match allowed e:
    case False{} Crash{}: Stutter{}
    case _ _: e

def boundedStep(+max: Nat, c: Config, e: Boundary, s: State) -> State:
  match s:
    case State{x, +n}: step(c, budgetEvent(Nat.is_lt(n, max), e), State{x, n})

def boundedRun(events: List<&2, Boundary>, +max: Nat, +c: Config, s: State) -> State:
  match events:
    case Nil{}: s
    case e <> rest: boundedRun(rest, max, c, boundedStep(max, c, e, s))

# Direct field projections for named safety and reachability statements.
def badCommit(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, bc, _}: bc

def badCheckpoint(s: Core) -> Bool:
  match s:
    case Core{_, _, _, _, _, _, _, _, bk}: bk
