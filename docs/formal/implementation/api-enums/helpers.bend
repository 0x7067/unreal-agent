import Base
import ../enums/semantics.bend as M

# These definitions are total, structurally recursive in the declaration list.
# chain is a semantic normal form of sequential switch branches, NOT the policy.
def chain(values: List<&2, List<&2, Nat>>, +x: List<&2, Nat>) -> Bool:
  match values:
    case Nil{}: False{}
    case Con{value, rest}: M.choose(M.byteEq(x, value), True{}, chain(rest, x))

# Policy uses disjunction, independently of source conditional semantics.
def members(values: List<&2, List<&2, Nat>>, +x: List<&2, Nat>) -> Bool:
  match values:
    case Nil{}: False{}
    case Con{value, rest}: M.byteEq(x, value) || members(rest, x)
