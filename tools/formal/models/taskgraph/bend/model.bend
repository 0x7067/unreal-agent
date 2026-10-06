import Base

def eq(a: Bool, b: Bool) -> Bool:
  match a b:
    case False{} False{}: True{}
    case True{} True{}: True{}
    case _ _: False{}

type Status is Data:
  Pending{}
  Running{}
  Canceling{}
  Completed{}
  Accepted{}
  Failed{}
  Canceled{}

type Node is Data:
  Node{status: Status, stale: Bool, child: Bool}

type Checkpoint is Data:
  Checkpoint{a: Node, b: Node, dependent: Bool, conflicting: Bool, capTwo: Bool}

def active(s: Status) -> Bool:
  match s:
    case Running{}: True{}
    case Canceling{}: True{}
    case _: False{}

def reserved(+s: Status) -> Bool:
  match s:
    case Completed{}: True{}
    case _: active(s)

def needsChild(+s: Status) -> Bool:
  match s:
    case Accepted{}: True{}
    case _: reserved(s)

def accepted(s: Status) -> Bool:
  match s:
    case Accepted{}: True{}
    case _: False{}

def needsPrereq(+s: Status, stale: Bool) -> Bool:
  match s:
    case Completed{}: True{}
    case Accepted{}: True{}
    case _: Bool.and(active(s), Bool.not(stale))

def claims(c: Checkpoint) -> Bool:
  match c:
    case Checkpoint{Node{a, _, _}, Node{b, _, _}, _, conflict, _}:
      Bool.not(Bool.and(conflict, Bool.and(reserved(a), reserved(b))))

def concurrency(c: Checkpoint) -> Bool:
  match c:
    case Checkpoint{Node{a, _, _}, Node{b, _, _}, _, _, capTwo}:
      Bool.or(capTwo, Bool.not(Bool.and(active(a), active(b))))

def dependencies(c: Checkpoint) -> Bool:
  match c:
    case Checkpoint{Node{a, staleA, _}, Node{b, staleB, _}, dependent, _, _}:
      Bool.or(Bool.not(Bool.and(dependent, needsPrereq(b, staleB))), Bool.and(accepted(a), Bool.not(staleA)))

def identity(c: Checkpoint) -> Bool:
  match c:
    case Checkpoint{Node{a, _, idA}, Node{b, _, idB}, _, _, _}:
      Bool.not(Bool.and(Bool.and(needsChild(a), needsChild(b)), eq(idA, idB)))

def legacyNode(n: Node) -> Bool:
  match n:
    case Node{s, stale, _}:
      Bool.or(Bool.not(stale), active(s))

def legacyAccepted(a: Node, b: Node, dep: Bool) -> Bool:
  match a b:
    case Node{sa, staleA, _} Node{sb, _, _}:
      Bool.or(Bool.not(Bool.and(dep, accepted(sb))), Bool.and(accepted(sa), Bool.not(staleA)))

def legacy(+c: Checkpoint) -> Bool:
  match c:
    case Checkpoint{+a, +b, dep, conflict, cap}:
      Bool.and(legacyNode(a), Bool.and(legacyNode(b), legacyAccepted(a, b, dep)))

def cross(+c: Checkpoint) -> Bool:
  Bool.and(claims(c), Bool.and(concurrency(c), Bool.and(dependencies(c), identity(c))))

def valid(+c: Checkpoint) -> Bool:
  Bool.and(legacy(c), cross(c))

# Restore only changes the restored marker; invalid candidates leave it false.
def restore(c: Checkpoint) -> Bool:
  valid(c)

# All attempts in a finite restore history satisfy safety on acceptance.
def acceptedSafe(+c: Checkpoint) -> Bool:
  Bool.or(Bool.not(restore(c)), cross(c))

def historySafe(cs: List<&2, Checkpoint>) -> Bool:
  match cs:
    case Nil{}: True{}
    case c <> rest: Bool.and(acceptedSafe(c), historySafe(rest))

# Exact ChildRouting fixed.cfg universe: IDs 1/2 represented by False/True.
# Running=False, Completed=True. The first-ID match wins even if completed.
def route(first: Bool, sa: Bool, sb: Bool) -> Bool & Bool:
  match first:
    case True{}: (True{}, sb)
    case False{}: (sa, True{})

def deliver(+a: Bool, b: Bool, incoming: Bool, sa: Bool, sb: Bool) -> Bool & Bool:
  route(eq(a, incoming), sa, sb)

def retired(states: Bool & Bool) -> Bool:
  match states:
    case (sa, sb): Bool.and(sa, sb)

# Rejected duplicate-ID checkpoints satisfy the done=>all-completed implication
# vacuously; accepted unique IDs actually run both delivery steps.
def deliverSecond(a: Bool, +b: Bool, states: Bool & Bool) -> Bool:
  match states:
    case (sa, sb): retired(deliver(a, b, b, sa, sb))

def routingGuard(duplicate: Bool, +a: Bool, +b: Bool) -> Bool:
  match duplicate:
    case True{}: True{}
    case False{}: deliverSecond(a, b, deliver(a, b, a, False{}, False{}))

def routing(+a: Bool, +b: Bool) -> Bool:
  routingGuard(eq(a, b), a, b)
