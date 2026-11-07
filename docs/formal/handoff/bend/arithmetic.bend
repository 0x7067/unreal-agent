import Base
import ./lists.bend as L

# Total elementary arithmetic, with no bounded machine integers.
def Le(a: Nat, b: Nat) -> Data:
  match a b:
    case 0n _:
      Unit
    case 1n+p 0n:
      Empty
    case 1n+p 1n+q:
      Le(p, q)

def refl(n: Nat) -> Le(n, n):
  match n:
    case 0n: Unit{}
    case 1n+p: refl(p)

def trans(a: Nat, b: Nat, c: Nat, ab: Le(a, b), bc: Le(b, c)) -> Le(a, c):
  match a b c:
    case 0n _ _: Unit{}
    case 1n+p 0n _: match ab:
    case 1n+p 1n+q 0n: match bc:
    case 1n+p 1n+q 1n+r: trans(p, q, r, ab, bc)

def weaken(a: Nat, b: Nat, ab: Le(a, b)) -> Le(a, 1n+b):
  match a b:
    case 0n _: Unit{}
    case 1n+p 0n: match ab:
    case 1n+p 1n+q: weaken(p, q, ab)

def maxBound(a: Nat, b: Nat, c: Nat, ac: Le(a, c), bc: Le(b, c)) -> Le(Nat.max(a, b), c):
  match a b c:
    case 0n _ _: bc
    case 1n+p 0n _: ac
    case 1n+p 1n+q 0n: match ac:
    case 1n+p 1n+q 1n+r: maxBound(p, q, r, ac, bc)

def void(-A: Type, e: Empty) -> A:
  match e:

def falseDisc(b: Bool) -> Data:
  match b:
    case False{}: Unit
    case True{}: Empty

def clash(e: {False{} == True{} : Bool}) -> Empty:
  %e : falseDisc(_)
  Unit{}

def ltTrue(a: Nat, b: Nat, e: {Nat.is_lt(a, b) == True{} : Bool}) -> Le(1n+a, b):
  match a b:
    case 0n 0n: clash(e)
    case 0n 1n+q: Unit{}
    case 1n+p 0n: clash(e)
    case 1n+p 1n+q: ltTrue(p, q, e)

def notLt(a: Nat, b: Nat, e: {Nat.is_lt(a, b) == False{} : Bool}) -> Le(b, a):
  match a b:
    case 0n 0n: Unit{}
    case 0n 1n+q: void(Empty, clash(Equal.sym(Bool, True{}, False{}, e)))
    case 1n+p 0n: Unit{}
    case 1n+p 1n+q: notLt(p, q, e)

def antisym(a: Nat, b: Nat, ab: Le(a, b), ba: Le(b, a)) -> {a == b : Nat}:
  match a b:
    case 0n 0n: {==}
    case 0n 1n+q: match ba:
    case 1n+p 0n: match ab:
    case 1n+p 1n+q:
      %antisym(p, q, ab, ba) : {1n+p == 1n+_ : Nat}
      {==}

def down(+n: Nat) -> List<&2, Nat>:
  match n:
    case 0n: []
    case 1n+p: p <> down(p)

def length(xs: List<&2, Nat>) -> Nat:
  match xs:
    case Nil{}: 0n
    case h <> t: 1n+length(t)

def lengthDown(n: Nat) -> {length(down(n)) == n : Nat}:
  match n:
    case 0n: {==}
    case 1n+p:
      %lengthDown(p) : {1n+length(down(p)) == 1n+_ : Nat}
      {==}

def ltZero(a: Nat) -> {Nat.is_lt(a, 0n) == False{} : Bool}:
  match a:
    case 0n: {==}
    case 1n+p: {==}

def eqLt(a: Nat, b: Nat) -> {Bool.or(Nat.is_eq(a, b), Nat.is_lt(a, b)) == Nat.is_lt(a, 1n+b) : Bool}:
  match a b:
    case 0n 0n: {==}
    case 0n 1n+q: {==}
    case 1n+p 0n: Equal.sym(Bool, Nat.is_lt(p, 0n), False{}, ltZero(p))
    case 1n+p 1n+q: eqLt(p, q)

def belowContains(+id: Nat, +n: Nat) -> {L.contains(id, down(n)) == Nat.is_lt(id, n) : Bool}:
  match n:
    case 0n: Equal.sym(Bool, Nat.is_lt(id, 0n), False{}, ltZero(id))
    case 1n+q:
      %Equal.sym(Bool, L.contains(id, down(q)), Nat.is_lt(id, q), belowContains(id, q)) : {Bool.or(Nat.is_eq(id, q), _) == Nat.is_lt(id, 1n+q) : Bool}
      eqLt(id, q)

def ltRefl(n: Nat) -> {Nat.is_lt(n, n) == False{} : Bool}:
  match n:
    case 0n: {==}
    case 1n+p: ltRefl(p)

def uniqueDown(+n: Nat) -> {L.unique(down(n)) == True{} : Bool}:
  match n:
    case 0n: {==}
    case 1n+p:
      %Equal.sym(Bool, L.contains(p, down(p)), Nat.is_lt(p, p), belowContains(p, p)) : {Bool.and(Bool.not(_), L.unique(down(p))) == True{} : Bool}
      %Equal.sym(Bool, Nat.is_lt(p, p), False{}, ltRefl(p)) : {Bool.and(Bool.not(_), L.unique(down(p))) == True{} : Bool}
      uniqueDown(p)

# Strictly descending reverse-store order is strict FIFO in admission order.
def descending(+xs: List<&2, Nat>) -> Bool:
  match xs:
    case Nil{}: True{}
    case x <> Nil{}: True{}
    case x <> y <> rest: Bool.and(Nat.is_lt(y, x), descending(y <> rest))

def ltSucc(n: Nat) -> {Nat.is_lt(n, 1n+n) == True{} : Bool}:
  match n:
    case 0n: {==}
    case 1n+p: ltSucc(p)

def descendingDown(+n: Nat) -> {descending(down(n)) == True{} : Bool}:
  match n:
    case 0n: {==}
    case 1n+q:
      match q:
        case 0n: {==}
        case 1n+r:
          %Equal.sym(Bool, Nat.is_lt(r, 1n+r), True{}, ltSucc(r)) : {Bool.and(_, descending(down(1n+r))) == True{} : Bool}
          descendingDown(1n+r)

def leLt(a: Nat, b: Nat, c: Nat, ab: Le(a, b), e: {Nat.is_lt(c, a) == True{} : Bool}) -> {Nat.is_lt(c, b) == True{} : Bool}:
  match a b c:
    case 0n _ _:
      bad = Equal.trans(Bool, False{}, Nat.is_lt(c, 0n), True{}, Equal.sym(Bool, Nat.is_lt(c, 0n), False{}, ltZero(c)), e)
      void({Nat.is_lt(c, b) == True{} : Bool}, clash(bad))
    case 1n+p 0n _: match ab:
    case 1n+p 1n+q 0n: {==}
    case 1n+p 1n+q 1n+r: leLt(p, q, r, ab, e)

def andRight(a: Bool, b: Bool, e: {Bool.and(a, b) == True{} : Bool}) -> {b == True{} : Bool}:
  match a:
    case False{}: void({b == True{} : Bool}, clash(e))
    case True{}: e

def lengthStore(+h: Nat, +xs: List<&2, Nat>, e: {xs == down(h) : List<&2, Nat>}) -> {length(xs) == h : Nat}:
  %Equal.sym(List<&2, Nat>, xs, down(h), e) : {length(_) == h : Nat}
  lengthDown(h)
