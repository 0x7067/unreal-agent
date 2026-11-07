import Base

# Bytes are arbitrary Nat lists. Go byte strings embed in the 0..255 subset;
# the checked laws deliberately quantify over the larger domain as well.
type Slot is Data:
  First{}
  Second{}

type Expr is Data:
  Truth{value: Bool}
  Same{slot: Slot, value: List<&2, Nat>}
  Neg{body: Expr}
  Both{left: Expr, right: Expr}
  Either{left: Expr, right: Expr}
  Branch{guard: Expr, yes: Expr, no: Expr}

def byteEq(xs: List<&2, Nat>, ys: List<&2, Nat>) -> Bool:
  match xs ys:
    case Nil{} Nil{}: True{}
    case Nil{} Con{_, _}: False{}
    case Con{_, _} Nil{}: False{}
    case Con{x, xt} Con{y, yt}: Nat.is_eq(x, y) && byteEq(xt, yt)

def choose(b: Bool, yes: Bool, no: Bool) -> Bool:
  match b:
    case True{}: yes
    case False{}: no

def eval(expr: Expr, +x: List<&2, Nat>, +y: List<&2, Nat>) -> Bool:
  match expr:
    case Truth{b}: b
    case Same{slot, value}:
      match slot:
        case First{}: byteEq(x, value)
        case Second{}: byteEq(y, value)
    case Neg{body}: Bool.not(eval(body, x, y))
    case Both{left, right}: eval(left, x, y) && eval(right, x, y)
    case Either{left, right}: eval(left, x, y) || eval(right, x, y)
    case Branch{guard, yes, no}: choose(eval(guard, x, y), eval(yes, x, y), eval(no, x, y))
