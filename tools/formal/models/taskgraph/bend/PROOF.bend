import Base
import ./model.bend as M
import ./LAWS.bend as Laws

def left(a: Bool, b: Bool, ok: {Bool.and(a, b) == True{} : Bool}) -> {a == True{} : Bool}:
  match a:
    case True{}: {==}
    case False{}: ok

def right(a: Bool, b: Bool, ok: {Bool.and(a, b) == True{} : Bool}) -> {b == True{} : Bool}:
  match a b:
    case True{} b: ok
    case False{} True{}: {==}
    case False{} False{}: ok

def admissionSafe(a: Bool, b: Bool) -> {Bool.or(Bool.not(Bool.and(a, b)), b) == True{} : Bool}:
  match a b:
    case False{} _: {==}
    case True{} False{}: {==}
    case True{} True{}: {==}

def Laws.restored_cross_node_safety(c, ok):
  right(M.legacy(c), M.cross(c), ok)

def Laws.restored_claim_exclusion(c, ok):
  left(M.claims(c), Bool.and(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c))), Laws.restored_cross_node_safety(c, ok))

def Laws.restored_concurrency_bound(c, ok):
  left(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c)), right(M.claims(c), Bool.and(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c))), Laws.restored_cross_node_safety(c, ok)))

def Laws.restored_dependency_gating(c, ok):
  left(M.dependencies(c), M.identity(c), right(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c)), right(M.claims(c), Bool.and(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c))), Laws.restored_cross_node_safety(c, ok))))

def Laws.restored_unique_children(c, ok):
  right(M.dependencies(c), M.identity(c), right(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c)), right(M.claims(c), Bool.and(M.concurrency(c), Bool.and(M.dependencies(c), M.identity(c))), Laws.restored_cross_node_safety(c, ok))))

def Laws.every_restore_attempt_safe(cs):
  match cs:
    case Nil{}: {==}
    case +c <> rest:
      %Equal.sym(Bool, M.acceptedSafe(c), True{}, admissionSafe(M.legacy(c), M.cross(c))) : {Bool.and(_, M.historySafe(rest)) == True{} : Bool}
      Laws.every_restore_attempt_safe(rest)

def Laws.terminal_children_retired(a, b):
  match a b:
    case False{} False{}: {==}
    case False{} True{}: {==}
    case True{} False{}: {==}
    case True{} True{}: {==}

def Laws.legacy_duplicate_children_rejected(): {==}

def Laws.legacy_routing_loses_second_child(): {==}

def Laws.legacy_claims_counterexample(): {==}
def Laws.legacy_concurrency_counterexample(): {==}
def Laws.legacy_dependencies_counterexample(): {==}
def Laws.legacy_identity_counterexample(): {==}
