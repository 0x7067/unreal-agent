# Authoritative project law index. Laws describe the cited Bend models, not Go
# refinement. See docs/formal/README.md and coverage.json for exact limitations.
import ./docs/formal/review/LAWS.bend as ReviewLaws
import ./docs/formal/handoff/bend/LAWS.bend as HandoffLaws
import ./docs/formal/taskgraph/bend/LAWS.bend as TaskgraphLaws
import ./docs/formal/coordinator-dispatch/bend/LAWS.bend as CoordinatorLaws
import ./docs/formal/operations/bend/LAWS.bend as OperationLaws

import ./docs/formal/implementation/enums/LAWS.bend as SourcePredicatesLaws

import ./docs/formal/implementation/api-enums/LAWS.bend as APIEnumsLaws
