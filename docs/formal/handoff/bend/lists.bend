import Base

def append(xs: List<&2, Nat>, ys: List<&2, Nat>) -> List<&2, Nat>:
  match xs:
    case Nil{}:
      ys
    case h <> t:
      h <> append(t, ys)

def reverse(xs: List<&2, Nat>) -> List<&2, Nat>:
  match xs:
    case Nil{}:
      []
    case h <> t:
      append(reverse(t), [h])

def contains(+id: Nat, xs: List<&2, Nat>) -> Bool:
  match xs:
    case Nil{}:
      False{}
    case h <> t:
      Bool.or(Nat.is_eq(id, h), contains(id, t))

def unique(+xs: List<&2, Nat>) -> Bool:
  match xs:
    case Nil{}:
      True{}
    case h <> t:
      Bool.and(Bool.not(contains(h, t)), unique(t))

def assoc(xs: List<&2, Nat>, ys: List<&2, Nat>, zs: List<&2, Nat>) -> {append(append(xs, ys), zs) == append(xs, append(ys, zs)) : List<&2, Nat>}:
  match xs:
    case Nil{}:
      {==}
    case h <> t:
      %assoc(t, ys, zs) : {h <> append(append(t, ys), zs) == h <> _ : List<&2, Nat>}
      {==}

def rightNil(xs: List<&2, Nat>) -> {append(xs, []) == xs : List<&2, Nat>}:
  match xs:
    case Nil{}:
      {==}
    case h <> t:
      %rightNil(t) : {h <> append(t, []) == h <> _ : List<&2, Nat>}
      {==}
