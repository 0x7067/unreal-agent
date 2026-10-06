--------------------------- MODULE Checkpoint ---------------------------
EXTENDS Naturals, FiniteSets

\* Finite abstraction of Graph.Validate + validateAttempts at runtime.add's
\* restore boundary. Declarations are canonical and valid, versions/generations
\* are valid, accepted evidence is nonblank, and required shell children and
\* fingerprints are well-formed. Only the cross-node checks vary here.
CONSTANT Fixed
Tasks == {"A", "B"}
Statuses == {"pending", "running", "canceling", "completed",
             "accepted", "failed", "canceled"}
Active(s) == s \in {"running", "canceling"}
Reserved(s) == Active(s) \/ s = "completed"
NeedsChild(s) == Reserved(s) \/ s = "accepted"

VARIABLES status, stale, dependent, conflicting, cap, childID, restored
vars == <<status, stale, dependent, conflicting, cap, childID, restored>>

\* A can write "shared" while B only reads it (legal one-way interference),
\* or both have empty claims. dependent means B depends on A (acyclic).
\* childID abstracts CURRENT attempts only; obsolete attempts do not match
\* runtime.child's generation guard. Pending/failed/canceled attempts are
\* omitted here. Go regressions additionally cover optional current attempts.
TypeOK == /\ status \in [Tasks -> Statuses]
          /\ stale \in [Tasks -> BOOLEAN]
          /\ dependent \in BOOLEAN
          /\ conflicting \in BOOLEAN
          /\ cap \in {1, 2}
          /\ childID \in [Tasks -> {1, 2}]
          /\ restored \in BOOLEAN

PrerequisitesOK(t) == t # "B" \/ ~dependent \/
                       (status["A"] = "accepted" /\ ~stale["A"])
LegacyValid == /\ \A t \in Tasks : stale[t] => Active(status[t])
               /\ \A t \in Tasks : status[t] = "accepted" => PrerequisitesOK(t)

ClaimExclusion == ~(conflicting /\ Reserved(status["A"]) /\
                                    Reserved(status["B"]))
ConcurrencyBound == Cardinality({t \in Tasks : Active(status[t])}) <= cap
DependencyGating == \A t \in Tasks :
    ((Active(status[t]) /\ ~stale[t]) \/
     status[t] \in {"completed", "accepted"}) => PrerequisitesOK(t)
UniqueChildren == \A a, b \in Tasks :
    (a # b /\ NeedsChild(status[a]) /\ NeedsChild(status[b])) =>
       childID[a] # childID[b]
CrossNodeSafety == ClaimExclusion /\ ConcurrencyBound /\
                   DependencyGating /\ UniqueChildren
Valid == LegacyValid /\ (IF Fixed THEN CrossNodeSafety ELSE TRUE)

\* Enumerate supplied checkpoints, not just states produced by legal reducer
\* events: Start already enforces these checks, which hid the restore gap.
Init == /\ TypeOK /\ ~restored
Restore == /\ ~restored /\ Valid /\ restored' = TRUE
           /\ UNCHANGED <<status, stale, dependent, conflicting, cap, childID>>
Next == Restore \/ UNCHANGED vars
Spec == Init /\ [][Next]_vars

SafeClaims == restored => ClaimExclusion
SafeConcurrency == restored => ConcurrencyBound
SafeDependencies == restored => DependencyGating
SafeChildIdentity == restored => UniqueChildren
SafeRestore == restored => CrossNodeSafety
=============================================================================
