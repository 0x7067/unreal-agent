# Authoritative project law index. Laws describe the cited Bend models, not Go
# refinement. coverage.json records the scope and residual obligations.
import ./tools/formal/models/review/LAWS.bend as ReviewLaws
# HISTORICAL pre-upstream-simplification protocol models, NOT runtime contracts.
import ./tools/formal/models/handoff/bend/LAWS.bend as HandoffLaws
import ./tools/formal/models/taskgraph/bend/LAWS.bend as TaskgraphLaws
import ./tools/formal/models/coordinator-dispatch/bend/LAWS.bend as CoordinatorLaws
import ./tools/formal/models/operations/bend/LAWS.bend as OperationLaws

# ACTIVE restricted contracts for six selected upstream predicates.
import ./tools/formal/models/implementation/enums/LAWS.bend as SourcePredicatesLaws

import ./tools/formal/models/implementation/api-enums/LAWS.bend as APIEnumsLaws

# HISTORICAL original eleven-target component; preserved and compiler-checked.
import ./tools/formal/models/history/enums/LAWS.bend as HistoricalSourcePredicatesLaws
