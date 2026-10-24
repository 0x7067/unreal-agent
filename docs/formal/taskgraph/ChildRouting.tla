-------------------------- MODULE ChildRouting --------------------------
EXTENDS Naturals
CONSTANT Fixed
VARIABLES ids, status, phase, delivered
vars == <<ids, status, phase, delivered>>

\* Two independent running tasks, current generation 1, empty claims,
\* concurrency 2. Both persisted shell children are already terminal.
\* readOnlyFingerprint and output Fingerprint succeed, so completion succeeds.
Init == /\ ids \in [1..2 -> {1, 2}]
        /\ status = [t \in 1..2 |-> "running"]
        /\ phase = "disk" /\ delivered = 0
Restore == /\ phase = "disk"
           /\ phase' = IF Fixed /\ ids[1] = ids[2] THEN "rejected" ELSE "resume"
           /\ UNCHANGED <<ids, status, delivered>>

\* runtime.add iterates nodes and calls child(a.ChildOperation) for each
\* terminal child. runtime.child scans nodes in order and RETURNS on its first
\* ID/current-generation match, even if that node is already Completed.
Deliver == /\ phase = "resume" /\ delivered < 2
           /\ LET incoming == ids[delivered + 1]
                  target == IF ids[1] = incoming THEN 1 ELSE 2
              IN status' = IF status[target] = "running"
                           THEN [status EXCEPT ![target] = "completed"]
                           ELSE status
           /\ delivered' = delivered + 1
           /\ phase' = IF delivered' = 2 THEN "done" ELSE "resume"
           /\ UNCHANGED ids
Next == Restore \/ Deliver \/ UNCHANGED vars
Spec == Init /\ [][Next]_vars
AllTerminalChildrenRetired == phase = "done" =>
                             (\A t \in 1..2 : status[t] = "completed")
=============================================================================
