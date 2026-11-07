# Check every indexed law and its implementation. Do not run this as a program.
# Canonical fail-closed invocation: make prove
import ./LAWS.bend as Laws
import ./docs/formal/review/PROOF.bend as ReviewProofs
import ./docs/formal/handoff/bend/PROOF.bend as HandoffProofs
import ./docs/formal/taskgraph/bend/PROOF.bend as TaskgraphProofs
import ./docs/formal/coordinator-dispatch/bend/PROOF.bend as CoordinatorProofs
import ./docs/formal/operations/bend/PROOF.bend as OperationProofs

import ./docs/formal/implementation/enums/PROOF.bend as SourcePredicatesProofs

import ./docs/formal/implementation/api-enums/PROOF.bend as APIEnumsProofs
