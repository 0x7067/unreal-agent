import Base
import ./semantics.bend as M
import ./SOURCE.bend as S
import ./POLICY.bend as P
import ./LAWS.bend as Laws

def Laws.choose_boolean(b):
  match b:
    case True{}: {==}
    case False{}: {==}

def Laws.or_associative(a, b, c):
  match a:
    case True{}: {==}
    case False{}: {==}

def Laws.merge_identical_branches(a, b, p, r):
  match a:
    case True{}: {==}
    case False{}: {==}

def Laws.local_finished_matches_policy(x):
  Laws.choose_boolean(P.terminal3(x))

def Laws.primitive_completed_matches_policy(x):
  Laws.choose_boolean(P.primitive_allowed(x))

def Laws.remote_transition_matches_policy(x, y):
  Laws.merge_identical_branches(M.byteEq(x, P.ready()), M.byteEq(x, P.awaiting()), P.live_update(y), M.choose(M.byteEq(x, P.canceling()), P.cancel_update(y), False{}))

def Laws.coordinator_terminal_matches_policy(x):
  Laws.choose_boolean(P.terminal3(x))

def Laws.taskgraph_terminal_matches_policy(x):
  {==}

def Laws.shell_output_matches_policy(x):
  Laws.choose_boolean(P.shell_allowed(x))

def Laws.activity_terminal_matches_policy(x):
  Laws.or_associative(M.byteEq(x, P.completed()), M.byteEq(x, P.failed()), M.byteEq(x, P.canceled()))

def Laws.stored_terminal_matches_policy(x):
  Laws.choose_boolean(P.terminal3(x))

def Laws.worker_terminal_matches_policy(x):
  {==}

def Laws.workload_terminal_matches_policy(x):
  Laws.or_associative(M.byteEq(x, P.completed()), M.byteEq(x, P.failed()), M.byteEq(x, P.canceled()))

def Laws.reasoning_valid_matches_policy(x):
  Laws.choose_boolean(P.reasoning_allowed(x))

def Laws.eval_truth(b, x, y):
  {==}

def Laws.eval_first_equality(v, x, y):
  {==}

def Laws.eval_second_equality(v, x, y):
  {==}

def Laws.eval_negation(e, x, y):
  {==}

def Laws.eval_conjunction(a, b, x, y):
  {==}

def Laws.eval_disjunction(a, b, x, y):
  {==}

def Laws.eval_branch(g, a, b, x, y):
  {==}

def Laws.byte_equality_empty():
  {==}

def Laws.byte_equality_cons(a, b, xs, ys):
  {==}

def Laws.natural_equality_reflexive(n):
  match n:
    case 0n: {==}
    case 1n+p: Laws.natural_equality_reflexive(p)

def Laws.byte_equality_reflexive(xs):
  match xs:
    case Nil{}: {==}
    case Con{x, rest}:
      %Equal.sym(Bool, Nat.is_eq(x, x), True{}, Laws.natural_equality_reflexive(x)) : {(_ && M.byteEq(rest, rest)) == True{} : Bool}
      Laws.byte_equality_reflexive(rest)

def Laws.completed_remote_never_revives(updated):
  {==}

def Laws.failed_remote_never_revives(updated):
  {==}

def Laws.canceled_remote_never_revives(updated):
  {==}

def Laws.unsupported_remote_never_revives(updated):
  {==}

def Laws.canceling_cannot_report_completed():
  {==}

def Laws.ready_can_become_awaiting():
  {==}

def Laws.taskgraph_accepts_unsupported_terminal():
  {==}

def Laws.coordinator_rejects_unsupported_terminal():
  {==}
