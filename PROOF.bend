# Check every indexed law and its implementation. Do not run this as a program.
# Canonical fail-closed invocation: make prove
import ./LAWS.bend as Laws
import ./tools/formal/models/review/PROOF.bend as ReviewProofs
# HISTORICAL pre-upstream-simplification protocol models, NOT runtime contracts.
import ./tools/formal/models/handoff/bend/PROOF.bend as HandoffProofs
import ./tools/formal/models/taskgraph/bend/PROOF.bend as TaskgraphProofs
import ./tools/formal/models/coordinator-dispatch/bend/PROOF.bend as CoordinatorProofs
import ./tools/formal/models/operations/bend/PROOF.bend as OperationProofs

# ACTIVE restricted contracts for six selected upstream predicates.
import ./tools/formal/models/implementation/enums/PROOF.bend as SourcePredicatesProofs

import ./tools/formal/models/implementation/api-enums/PROOF.bend as APIEnumsProofs

# HISTORICAL original eleven-target component; preserved and compiler-checked.
import ./tools/formal/models/history/enums/PROOF.bend as HistoricalSourcePredicatesProofs
