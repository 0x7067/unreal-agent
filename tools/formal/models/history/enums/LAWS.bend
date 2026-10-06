import Base
import ./semantics.bend as M
import ./SOURCE.bend as S
import ./POLICY.bend as P

# Public source-to-independent-policy obligations. ALL byte sequences, not an
# enumeration of known Go constants or finite sampled test inputs.
law local_finished_matches_policy:
  for +x: List<&2, Nat>
  {S.local_finished(x) == P.terminal3(x) : Bool}

law primitive_completed_matches_policy:
  for +x: List<&2, Nat>
  {S.primitive_completed(x) == P.primitive_allowed(x) : Bool}

law remote_transition_matches_policy:
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {S.remote_transition(x, y) == P.remote_allowed(x, y) : Bool}

law coordinator_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.coordinator_terminal(x) == P.terminal3(x) : Bool}

law taskgraph_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.taskgraph_terminal(x) == P.terminal4(x) : Bool}

law shell_output_matches_policy:
  for +x: List<&2, Nat>
  {S.shell_output(x) == P.shell_allowed(x) : Bool}

law activity_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.activity_terminal(x) == P.terminal3(x) : Bool}

law stored_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.stored_terminal(x) == P.terminal3(x) : Bool}

law worker_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.worker_terminal(x) == P.terminal4(x) : Bool}

law workload_terminal_matches_policy:
  for +x: List<&2, Nat>
  {S.workload_terminal(x) == P.terminal3(x) : Bool}

law reasoning_valid_matches_policy:
  for +x: List<&2, Nat>
  {S.reasoning_valid(x) == P.reasoning_allowed(x) : Bool}

# Generic Boolean normalization, over arbitrary Boolean results of equality
# comparisons. No finite string sampling or assumptions about known statuses.
law choose_boolean:
  for +b: Bool
  {M.choose(b, True{}, False{}) == b : Bool}

law or_associative:
  for +a: Bool
  for +b: Bool
  for +c: Bool
  {(a || b) || c == a || (b || c) : Bool}

law merge_identical_branches:
  for +a: Bool
  for +b: Bool
  for +p: Bool
  for +r: Bool
  {M.choose(a, p, M.choose(b, p, r)) == M.choose(a || b, p, r) : Bool}

# Public interpreter semantic-preservation equations for every accepted AST
# constructor. Parser/translation correctness remains explicitly trusted.
law eval_truth:
  for +b: Bool
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Truth{b}, x, y) == b : Bool}

law eval_first_equality:
  for +v: List<&2, Nat>
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Same{M.First{}, v}, x, y) == M.byteEq(x, v) : Bool}

law eval_second_equality:
  for +v: List<&2, Nat>
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Same{M.Second{}, v}, x, y) == M.byteEq(y, v) : Bool}

law eval_negation:
  for +e: M.Expr
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Neg{e}, x, y) == Bool.not(M.eval(e, x, y)) : Bool}

law eval_conjunction:
  for +a: M.Expr
  for +b: M.Expr
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Both{a, b}, x, y) == (M.eval(a, x, y) && M.eval(b, x, y)) : Bool}

law eval_disjunction:
  for +a: M.Expr
  for +b: M.Expr
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Either{a, b}, x, y) == (M.eval(a, x, y) || M.eval(b, x, y)) : Bool}

law eval_branch:
  for +g: M.Expr
  for +a: M.Expr
  for +b: M.Expr
  for +x: List<&2, Nat>
  for +y: List<&2, Nat>
  {M.eval(M.Branch{g, a, b}, x, y) == M.choose(M.eval(g, x, y), M.eval(a, x, y), M.eval(b, x, y)) : Bool}

law byte_equality_empty:
  {M.byteEq([], []) == True{} : Bool}

law byte_equality_cons:
  for +a: Nat
  for +b: Nat
  for +xs: List<&2, Nat>
  for +ys: List<&2, Nat>
  {M.byteEq(a <> xs, b <> ys) == (Nat.is_eq(a, b) && M.byteEq(xs, ys)) : Bool}

# Structural unbounded byte-equality preservation (not sampled witnesses).
law natural_equality_reflexive:
  for +n: Nat
  {Nat.is_eq(n, n) == True{} : Bool}

law byte_equality_reflexive:
  for +xs: List<&2, Nat>
  {M.byteEq(xs, xs) == True{} : Bool}

# Direct source safety consequences, independently visible at the public API.
# Final/unsupported current states cannot be revived by ANY updated byte string.
law completed_remote_never_revives:
  for +updated: List<&2, Nat>
  {S.remote_transition(P.completed(), updated) == False{} : Bool}

law failed_remote_never_revives:
  for +updated: List<&2, Nat>
  {S.remote_transition(P.failed(), updated) == False{} : Bool}

law canceled_remote_never_revives:
  for +updated: List<&2, Nat>
  {S.remote_transition(P.canceled(), updated) == False{} : Bool}

law unsupported_remote_never_revives:
  for +updated: List<&2, Nat>
  {S.remote_transition([], updated) == False{} : Bool}

law canceling_cannot_report_completed:
  {S.remote_transition(P.canceling(), P.completed()) == False{} : Bool}

law ready_can_become_awaiting:
  {S.remote_transition(P.ready(), P.awaiting()) == True{} : Bool}

# Preserve the intentionally DIFFERENT current empty/unsupported policies.
law taskgraph_accepts_unsupported_terminal:
  {S.taskgraph_terminal([]) == True{} : Bool}

law coordinator_rejects_unsupported_terminal:
  {S.coordinator_terminal([]) == False{} : Bool}
