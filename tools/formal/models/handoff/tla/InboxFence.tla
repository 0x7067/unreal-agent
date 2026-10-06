--------------------------- MODULE InboxFence ---------------------------
\* Admission fence of ONE inbox actor, abstracted from harness/inbox/local.go
\* (Submit/Freeze/Thaw and the single-goroutine run loop), which is what the
\* handoff relies on between the delivery fence and session persistence.
\*
\* "fixed" is the current implementation.  The other Variant values are
\* counterfactuals that remove one documented guarantee of local.go:61-77.
EXTENDS Naturals, FiniteSets, Sequences

CONSTANT Variant
Variants == {"fixed", "thawResetsDedup", "thawAbandons"}
ASSUME Variant \in Variants

Inputs == 1..2
Phases == {"open", "frozen"}

VARIABLES phase, submitted, seen, queue, delivered, handed
vars == <<phase, submitted, seen, queue, delivered, handed>>

RangeOf(s) == {s[k] : k \in 1..Len(s)}

TypeOK ==
  /\ phase \in Phases
  /\ submitted \in SUBSET Inputs
  /\ seen \in SUBSET Inputs
  /\ queue \in Seq(Inputs)
  /\ delivered \in Seq(Inputs)
  /\ handed \in Seq(Inputs)

Init ==
  /\ phase = "open"
  /\ submitted = {}
  /\ seen = {}
  /\ queue = <<>>
  /\ delivered = <<>>
  /\ handed = <<>>

\* Inbox.Submit (local.go:35-55): validate, then hand the input to the actor.
\* The caller is parked until the actor takes it, so at most one outstanding
\* submission per input is modelled.  New input IDs arrive in ascending order
\* (the same convention as Handoff's journal); an already admitted ID may be
\* submitted again, which is the journal-replay case dedup must absorb.
Submit(i) ==
  /\ submitted = {}   \* the serve loop reads and submits one pipe frame at a time (run.go:328-352)
  /\ i \in Inputs
  /\ (i \in seen \/ (\A j \in seen : j < i))
  /\ submitted' = submitted \cup {i}
  /\ UNCHANGED <<phase, seen, queue, delivered, handed>>

\* Actor admission (local.go:143-148): dedup by input ID, then append to the
\* FIFO.  While frozen the actor does not select on the submission channel, so
\* no admission can happen (local.go:125-128).
Admit ==
  /\ phase = "open"
  /\ \E i \in submitted :
       /\ i \notin seen
       /\ seen' = seen \cup {i}
       /\ queue' = Append(queue, i)
       /\ submitted' = submitted \ {i}
  /\ UNCHANGED <<phase, delivered, handed>>

\* Duplicate ID: dropped without being queued (local.go:144-146).
Drop ==
  /\ phase = "open"
  /\ \E i \in submitted :
       /\ i \in seen
       /\ submitted' = submitted \ {i}
  /\ UNCHANGED <<phase, seen, queue, delivered, handed>>

\* Output (local.go:150-152): the coordinator receives the FIFO head.
Emit ==
  /\ phase = "open"
  /\ Len(queue) > 0
  /\ delivered' = Append(delivered, Head(queue))
  /\ queue' = Tail(queue)
  /\ UNCHANGED <<phase, submitted, seen, handed>>

\* Inbox.Freeze (local.go:68-71, actor case at local.go:130-141): stop admission
\* and output and transfer the already admitted FIFO to the caller.  Each call
\* returns its own slice, so a later Freeze appends to what earlier callers hold
\* instead of overwriting it.
Freeze ==
  /\ phase = "open"
  /\ phase' = "frozen"
  /\ handed' = handed \o queue
  /\ queue' = <<>>
  /\ UNCHANGED <<submitted, seen, delivered>>

\* The caller must process the transferred FIFO before thawing (local.go:67,
\* coordinator/quiescence.go:98-103).
ProcessHanded ==
  /\ Len(handed) > 0
  /\ delivered' = Append(delivered, Head(handed))
  /\ handed' = Tail(handed)
  /\ UNCHANGED <<phase, submitted, seen, queue>>

\* Inbox.Thaw (local.go:74-77): resume admission and output WITHOUT resetting
\* deduplication.  thawResetsDedup clears the seen set; thawAbandons drops a
\* transferred FIFO the caller never processed.
\* Thawing is only sound once the caller has processed what it received
\* (local.go:67, "The caller must process the returned inputs before thawing");
\* the thawAbandons counterfactual drops that requirement.
Thaw ==
  /\ phase = "frozen"
  /\ (Variant = "thawAbandons" \/ handed = <<>>)
  /\ phase' = "open"
  /\ seen' = IF Variant = "thawResetsDedup" THEN {} ELSE seen
  /\ handed' = IF Variant = "thawAbandons" THEN <<>> ELSE handed
  /\ UNCHANGED <<submitted, queue, delivered>>

Next ==
  \/ (\E i \in Inputs : Submit(i))
  \/ Admit \/ Drop \/ Emit \/ Freeze \/ ProcessHanded \/ Thaw
  \/ UNCHANGED vars

Spec == Init /\ [][Next]_vars

(***************************************************************************)
(* NoLoss: every input the actor ever admitted is still accounted for --    *)
(* queued, transferred to the freeze caller, or already delivered.         *)
(***************************************************************************)
NoLoss == \A i \in seen :
            i \in RangeOf(delivered) \/ i \in RangeOf(queue) \/ i \in RangeOf(handed)

(***************************************************************************)
(* NoDuplicateDelivery: dedup by input ID survives Freeze/Thaw.            *)
(***************************************************************************)
CountDelivered(i) == Cardinality({k \in 1..Len(delivered) : delivered[k] = i})
NoDuplicateDelivery == \A i \in Inputs : CountDelivered(i) <= 1

(***************************************************************************)
(* FIFO: the delivered sequence follows the admission order.               *)
(***************************************************************************)
FIFO == \A k \in {j \in 1..Len(delivered) : j < Len(delivered)} :
          delivered[k] < delivered[k+1]

\* Intentional violation witnesses a nonempty Freeze transfer.
NoTransfer == handed = <<>>

=============================================================================
