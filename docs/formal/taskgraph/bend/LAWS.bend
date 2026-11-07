import Base
import ./model.bend as M

law restored_cross_node_safety:
  for +c: M.Checkpoint
  for ok: {M.restore(c) == True{} : Bool}
  {M.cross(c) == True{} : Bool}

law restored_claim_exclusion:
  for +c: M.Checkpoint
  for ok: {M.restore(c) == True{} : Bool}
  {M.claims(c) == True{} : Bool}

law restored_concurrency_bound:
  for +c: M.Checkpoint
  for ok: {M.restore(c) == True{} : Bool}
  {M.concurrency(c) == True{} : Bool}

law restored_dependency_gating:
  for +c: M.Checkpoint
  for ok: {M.restore(c) == True{} : Bool}
  {M.dependencies(c) == True{} : Bool}

law restored_unique_children:
  for +c: M.Checkpoint
  for ok: {M.restore(c) == True{} : Bool}
  {M.identity(c) == True{} : Bool}

law every_restore_attempt_safe:
  for +cs: List<&2, M.Checkpoint>
  {M.historySafe(cs) == True{} : Bool}

law terminal_children_retired:
  for a: Bool
  for b: Bool
  {M.routing(a, b) == True{} : Bool}

law legacy_duplicate_children_rejected:
  {M.valid(M.Checkpoint{M.Node{M.Running{}, False{}, False{}}, M.Node{M.Running{}, False{}, False{}}, False{}, False{}, True{}}) == False{} : Bool}

law legacy_routing_loses_second_child:
  {M.deliverSecond(False{}, False{}, M.deliver(False{}, False{}, False{}, False{}, False{})) == False{} : Bool}

# Each checkpoint is LegacyValid but fails precisely the named cross-node check.
law legacy_claims_counterexample:
  {(M.legacy(M.Checkpoint{M.Node{M.Completed{}, False{}, False{}}, M.Node{M.Completed{}, False{}, True{}}, False{}, True{}, True{}}), M.claims(M.Checkpoint{M.Node{M.Completed{}, False{}, False{}}, M.Node{M.Completed{}, False{}, True{}}, False{}, True{}, True{}})) == (True{}, False{}) : Bool & Bool}

law legacy_concurrency_counterexample:
  {(M.legacy(M.Checkpoint{M.Node{M.Running{}, False{}, False{}}, M.Node{M.Running{}, False{}, True{}}, False{}, False{}, False{}}), M.concurrency(M.Checkpoint{M.Node{M.Running{}, False{}, False{}}, M.Node{M.Running{}, False{}, True{}}, False{}, False{}, False{}})) == (True{}, False{}) : Bool & Bool}

law legacy_dependencies_counterexample:
  {(M.legacy(M.Checkpoint{M.Node{M.Pending{}, False{}, False{}}, M.Node{M.Running{}, False{}, True{}}, True{}, False{}, True{}}), M.dependencies(M.Checkpoint{M.Node{M.Pending{}, False{}, False{}}, M.Node{M.Running{}, False{}, True{}}, True{}, False{}, True{}})) == (True{}, False{}) : Bool & Bool}

law legacy_identity_counterexample:
  {(M.legacy(M.Checkpoint{M.Node{M.Running{}, False{}, False{}}, M.Node{M.Running{}, False{}, False{}}, False{}, False{}, True{}}), M.identity(M.Checkpoint{M.Node{M.Running{}, False{}, False{}}, M.Node{M.Running{}, False{}, False{}}, False{}, False{}, True{}})) == (True{}, False{}) : Bool & Bool}
