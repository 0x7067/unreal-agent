--------------------------- MODULE Handoff ---------------------------
\* Host-side TUI worker reload/handoff protocol, abstracted from
\* cmd/internal/workerhost (host.go, stage.go, events.go) plus the worker-side
\* admissions the host's safety argument depends on (workerruntime/run.go,
\* harness/coordinator/quiescence.go, harness/inbox/local.go).
\*
\* Every action names the Go code it abstracts; README.md carries the table and
\* the exact TLC bounds. "fixed" retains the historical guarded baseline,
\* including its abandoned-retirement status defect; it is not the status fix.
\* "statusOnAbandonedRetire" is the corrected-status candidate model for the
\* shipped I9/I10 repair: publish stopped when an abandoned retirement completes.
\* Neither variant is a verified refinement of the Go implementation. The
\* remaining Variant values are deliberately weakened counterfactuals that drop
\* selected guards; some remain safe within this abstraction. They
\* document the boundary of the argument and are not claims about the code.
EXTENDS Naturals, FiniteSets, Sequences

CONSTANT Variant
Variants == {"fixed", "statusOnAbandonedRetire", "earlyLaunch", "noSeed",
             "noSeedNoGuard", "noDiscard", "noProbe", "noSingleFlight",
             "noBusyGuard"}
ASSUME Variant \in Variants

\* Guard enablement per variant: noBusyGuard removes the persistence guard of
\* host.go:585-590, noSeedNoGuard removes it together with the replacement's
\* inbox dedup seed, earlyLaunch launches before the old process exited,
\* noDiscard leaks an unused staged candidate, noProbe retires without an
\* identity probe, noSingleFlight drops the reload CAS.
GuardEnabled == ~(Variant \in {"noBusyGuard", "noSeedNoGuard"})
SeedInbox    == ~(Variant \in {"noSeed", "noSeedNoGuard"})

\* Two inputs, admitted in ascending order (journal admission order), and at
\* most two generations (gen 1, and gen 2 after one completed handoff).
Inputs == 1..2
Gens == 1..2
Phases == {"idle", "staged", "fenced", "sent", "quiesced", "exited"}
PublishedStatuses == {"ready", "stopped"}

VARIABLES
  nextInput, pending, persisted,                    \* durable ingress journal
  started, oldExited, live, owner, stopped, failed, \* generations and latches
  abandoned, published,                             \* published frontend status
  inflight, fenceHeld, rp, probe, cand,             \* reload single flight
  sent, queue, seen, frozen                         \* pipe and worker inbox

AdmissionVars  == <<nextInput, pending, persisted>>
GenerationVars == <<started, oldExited, live, owner, stopped, failed>>
StatusVars     == <<abandoned, published>>
ReloadVars     == <<inflight, fenceHeld, rp, probe, cand>>
WorkerVars     == <<sent, queue, seen, frozen>>
vars == <<AdmissionVars, GenerationVars, StatusVars, ReloadVars, WorkerVars>>

Persisted == {persisted[k] : k \in 1..Len(persisted)}

TypeOK ==
  /\ nextInput \in 1..3
  /\ pending \in SUBSET Inputs
  /\ persisted \in Seq(Inputs)
  /\ started \in 1..2
  /\ oldExited \in BOOLEAN
  /\ live \in SUBSET Gens
  /\ owner \in 0..2
  /\ stopped \in BOOLEAN
  /\ failed \in BOOLEAN
  /\ abandoned \in BOOLEAN
  /\ published \in PublishedStatuses
  /\ inflight \in 0..2
  /\ fenceHeld \in BOOLEAN
  /\ rp \in Phases
  /\ probe \in {"ok", "bad"}
  /\ cand \in {"none", "staged"}
  /\ sent \in [Gens -> SUBSET Inputs]
  /\ queue \in Seq(Inputs)
  /\ seen \in SUBSET Inputs
  /\ frozen \in BOOLEAN

\* gen 1 is live and owns the pipe, nothing is journalled yet, and start() has
\* already published "Worker ready" (host.go:366).  start() itself
\* (host.go:175-198) is outside the reload protocol.
Init ==
  /\ nextInput = 1
  /\ pending = {}
  /\ persisted = <<>>
  /\ started = 1
  /\ oldExited = FALSE
  /\ live = {1}
  /\ owner = 1
  /\ stopped = FALSE
  /\ failed = FALSE
  /\ abandoned = FALSE
  /\ published = "ready"
  /\ inflight = 0
  /\ fenceHeld = FALSE
  /\ rp = "idle"
  /\ probe = "ok"
  /\ cand = "none"
  /\ sent = [g \in Gens |-> {}]
  /\ queue = <<>>
  /\ seen = {}
  /\ frozen = FALSE

\* host.Submit (host.go:217-243): durable ingress admission only.  It touches no
\* worker state, so it is admitted during a handoff as well -- and also when
\* no worker is live at all. The status repair preserves durable admission
\* and later explicit reload recovery; I10 checks publication, not delivery.
\* ingress.Journal.Submit keeps first-admission order and dedups by ID
\* (harness/ingress/journal.go:119-144); the model admits each ID at most once,
\* in ascending order.
Submit ==
  /\ nextInput <= 2
  /\ pending' = pending \cup {nextInput}
  /\ nextInput' = nextInput + 1
  /\ UNCHANGED <<persisted, GenerationVars, StatusVars, ReloadVars, WorkerVars>>

\* host.dispatch write step (host.go:245-276): the only writer, in journal
\* order, at most once per input per generation (p.sent carries the "failed
\* writes have ambiguous admission" rule).  dispatch takes and releases the
\* delivery fence around every pass, so one write is atomic with respect to a
\* handoff; fenceHeld therefore means "a reload currently holds the fence".
Write ==
  /\ ~fenceHeld
  /\ owner # 0
  /\ \E i \in Inputs :
       /\ i \in pending
       /\ i \notin sent[owner]
       /\ \A j \in pending : j < i => j \in sent[owner]
       /\ sent' = [sent EXCEPT ![owner] = sent[owner] \cup {i}]
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, ReloadVars, queue,
                 seen, frozen>>

\* Worker pipe decode into the inbox (workerruntime/run.go:350, the inbox actor
\* in harness/inbox/local.go:143-148).  Frames are decoded in write order; a
\* frame whose ID is already in the inbox dedup set is dropped without being
\* queued, which is why "sent \ seen" is exactly the not-yet-consumed set.
\* While the inbox is frozen the reader blocks (local.go:125-142).
Admit ==
  /\ owner # 0
  /\ ~frozen
  /\ \E i \in sent[owner] \ seen :
       /\ \A j \in sent[owner] \ seen : i <= j
       /\ queue' = Append(queue, i)
       /\ seen' = seen \cup {i}
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, ReloadVars, sent,
                 frozen>>

\* The coordinator persists the FIFO head to sessionstore
\* (harness/sessionstore/localfile/store.go:122-138, run.go:222-224 observer).
Persist ==
  /\ owner # 0
  /\ ~frozen
  /\ Len(queue) > 0
  /\ persisted' = Append(persisted, Head(queue))
  /\ queue' = Tail(queue)
  /\ UNCHANGED <<nextInput, pending, GenerationVars, StatusVars, ReloadVars,
                 sent, seen, frozen>>

\* The host acknowledges the journal only after the item frame arrives
\* (host.go:423-427 -> ingress/journal.go:160-181).  Acknowledging lags
\* persistence, which is why an input can be persisted and still pending.
Ack ==
  /\ \E i \in pending \cap Persisted : pending' = pending \ {i}
  /\ UNCHANGED <<nextInput, persisted, GenerationVars, StatusVars, ReloadVars,
                 WorkerVars>>

\* Reload single-flight admission (host.go:536-539): h.reload.CompareAndSwap
\* admits exactly one body.  A concurrent call fails the CAS and returns
\* ErrBusy without touching any state (a stutter, so it is not modelled here).
Begin ==
  /\ rp = "idle"
  /\ (Variant = "noSingleFlight" \/ inflight = 0)
  /\ started = 1
  /\ failed = FALSE
  /\ inflight' = inflight + 1
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, fenceHeld, rp, probe,
                 cand, sent, queue, seen, frozen>>

\* h.stage (stage.go:27-171): copy the candidate to an immutable path and probe
\* Identity{Protocol, SessionVersion}.  The probe outcome is chosen
\* nondeterministically: the running worker is untouched either way.
Stage ==
  /\ rp = "idle"
  /\ inflight >= 1
  /\ started = 1
  /\ failed = FALSE
  /\ cand' = "staged"
  /\ probe' \in {"ok", "bad"}
  /\ rp' = "staged"
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, fenceHeld, inflight,
                 sent, queue, seen, frozen>>

\* Incompatible or failed candidate: stage() itself returns before the worker is
\* touched and removes its own temporary file (stage.go:61-68, 132-151).
Reject ==
  /\ rp = "staged"
  /\ probe = "bad"
  /\ Variant # "noProbe"
  /\ cand' = "none"
  /\ rp' = "idle"
  /\ inflight' = inflight - 1
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, fenceHeld, probe,
                 sent, queue, seen, frozen>>

\* h.lockDelivery (host.go:566): take the delivery fence for the whole handoff.
\* The fence is reached only for a compatible candidate; the noProbe variant
\* keeps the fence without the identity probe.
Fence ==
  /\ rp = "staged"
  /\ (Variant = "noProbe" \/ probe = "ok")
  /\ fenceHeld' = TRUE
  /\ rp' = "fenced"
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, inflight, probe, cand,
                 sent, queue, seen, frozen>>

\* The persistence guard (host.go:585-590): refuse the reload before any quiesce
\* request while a journalled input was already written to the live worker and
\* has not reached session persistence.
GuardFires == owner # 0 /\ (\E i \in pending : i \in sent[owner])

GuardBusy ==
  /\ rp = "fenced"
  /\ GuardFires
  /\ GuardEnabled
  /\ rp' = "idle"
  /\ fenceHeld' = FALSE
  /\ inflight' = inflight - 1
  /\ cand' = IF Variant = "noDiscard" THEN "staged" ELSE "none"
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, probe, sent, queue,
                 seen, frozen>>

\* Selected host.Reload abort paths: cancellation before control delivery
\* (host.go:566-572), busy refusal (614-616), or cancellation after quiesced
\* but before exit (617-621). No replacement is launched and deferred discard
\* removes the unused stage (559-564). Failed shutdown and timeout before
\* acceptance are not represented as separate outcomes.
\* Abandonment is represented only AFTER QuiesceAccept: the worker has drained
\* and frozen its inbox and can still exit after Reload returns. Abort from
\* "sent" represents refusal (busy), not a pending retirement. Timeout before
\* acceptance is omitted rather than allowing post-control frames to be drained
\* ahead of the quiesce request. This restriction still exercises the historical defect.
Abort ==
  /\ rp \in {"staged", "fenced", "sent", "quiesced"}
  /\ abandoned' = (rp = "quiesced" /\ owner # 0)
  /\ rp' = "idle"
  /\ fenceHeld' = FALSE
  /\ inflight' = inflight - 1
  /\ cand' = IF Variant = "noDiscard" THEN "staged" ELSE "none"
  /\ UNCHANGED <<AdmissionVars, GenerationVars, published, probe, sent, queue,
                 seen, frozen>>

\* The quiesce request reaches the live worker (host.go:591-599, run.go:353-357).
\* It is sent only once the persistence guard is satisfied.
Quiesce ==
  /\ rp = "fenced"
  /\ owner # 0
  /\ (~GuardEnabled \/ ~GuardFires)
  /\ rp' = "sent"
  /\ UNCHANGED <<AdmissionVars, GenerationVars, StatusVars, inflight, fenceHeld,
                 probe, cand, sent, queue, seen, frozen>>

\* coordinator.handleQuiesce (quiescence.go:64-138): idle check, Inbox.Freeze
\* (local.go:68-71) which transfers and drains the admitted FIFO, processInputs
\* of that FIFO (quiescence.go:98-103), then retirement.  The drained inputs are
\* persisted before the worker retires, so they stay in the transcript.
QuiesceAccept ==
  /\ rp = "sent"
  /\ sent[owner] \subseteq seen  \* control cannot overtake earlier pipe frames
  /\ frozen' = TRUE
  /\ persisted' = persisted \o queue
  /\ queue' = <<>>
  /\ rp' = "quiesced"
  /\ UNCHANGED <<nextInput, pending, GenerationVars, StatusVars, inflight,
                 fenceHeld, probe, cand, sent, seen>>

\* Old process exit: abstract the ordered final-frame drain, process wait,
\* stopped publication, clearing current and p.done closure as one step.
\* In the repaired Go path the stopped callback (with generation/build identity)
\* precedes clearing current and closing p.done. Callback blocking/interleavings
\* are not represented here. Two entry points:
\*  - rp = "quiesced": the handoff still owns the attempt and publishes
\*    "Old worker exited; starting replacement" before launching;
\*  - rp = "idle" with abandoned: Reload already returned. The historical
\*    baseline publishes nothing on a clean retirement; that silence causes
\*    I9/I10 counterexamples. statusOnAbandonedRetire models the shipped status
\*    repair. Publication does not mean finish: clean retirement keeps Done
\*    open and permits durable Submit and later explicit reload recovery.
OldExit ==
  /\ (rp = "quiesced") \/ (rp = "idle" /\ abandoned)
  /\ owner # 0
  /\ live' = live \ {owner}
  /\ owner' = 0
  /\ oldExited' = TRUE
  /\ stopped' = TRUE
  /\ rp' = IF rp = "quiesced" THEN "exited" ELSE "idle"
  /\ published' = IF rp = "quiesced" \/ Variant = "statusOnAbandonedRetire"
                  THEN "stopped" ELSE published
  /\ UNCHANGED <<AdmissionVars, started, abandoned, inflight, fenceHeld, probe,
                 cand, sent, queue, seen, frozen, failed>>

\* h.launch of the replacement (host.go:627-643, 278-368).  Reachable only once
\* the previous generation has exited: from "exited" after a clean retirement,
\* or from "fenced" with no live worker and a latched proven retirement.  The
\* earlyLaunch variant instead launches while the old generation is still live,
\* which is the ambiguity documented at host.go:532-534.
CanLaunch ==
  IF Variant = "earlyLaunch"
  THEN \/ (rp = "exited" /\ owner = 0 /\ stopped)
       \/ (rp \in {"sent", "quiesced"} /\ owner # 0)
  ELSE \/ (rp = "exited" /\ owner = 0 /\ stopped)
       \/ (rp = "fenced" /\ owner = 0 /\ stopped)

\* The replacement seeds its inbox dedup set from the resumed session
\* (run.go:250 with ResumeState.InputIDs, localfile/state.go:290-296), so an
\* already persisted input that is replayed from the journal is dropped.
Launch ==
  /\ CanLaunch
  /\ started' = 2
  /\ live' = live \cup {2}
  /\ owner' = 2
  /\ stopped' = FALSE
  /\ sent' = [sent EXCEPT ![2] = {}]
  /\ queue' = <<>>
  /\ seen' = IF SeedInbox THEN Persisted ELSE {}
  /\ frozen' = FALSE
  /\ rp' = "idle"
  /\ fenceHeld' = FALSE
  /\ inflight' = inflight - 1
  /\ failed' = FALSE
  /\ cand' = "none"
  /\ published' = "ready"          \* host.go:366
  /\ abandoned' = FALSE
  /\ UNCHANGED <<nextInput, pending, persisted, oldExited, probe>>

\* A worker that dies without retiring latches h.failure (host.go:498-529),
\* publishes the stopped status (host.go:516-519) and never becomes the owner
\* again; every later Reload returns that failure (host.go:549-551, 582-584), so
\* no automatic replacement follows.  Death is modelled only between reloads.
Crash ==
  /\ rp = "idle"
  /\ owner # 0
  /\ live' = live \ {owner}
  /\ owner' = 0
  /\ failed' = TRUE
  /\ oldExited' = TRUE
  /\ stopped' = FALSE
  /\ published' = "stopped"
  /\ UNCHANGED <<AdmissionVars, started, abandoned, ReloadVars, sent, queue, seen,
                 frozen>>

Next ==
  \/ Submit \/ Write \/ Admit \/ Persist \/ Ack
  \/ Begin \/ Stage \/ Reject \/ Fence \/ GuardBusy \/ Abort \/ Quiesce
  \/ QuiesceAccept \/ OldExit \/ Launch \/ Crash
  \/ UNCHANGED vars

Spec == Init /\ [][Next]_vars

(***************************************************************************)
(* I1  at most one live generation owns delivery to the session.           *)
(***************************************************************************)
I1 == Cardinality(live) <= 1 /\ (owner # 0 => live = {owner})

(***************************************************************************)
(* I2  no accepted input is lost: it is either still replayable from the   *)
(*     journal or already applied to session persistence.                  *)
(***************************************************************************)
I2 == \A i \in 1..(nextInput - 1) : i \in pending \/ i \in Persisted

(***************************************************************************)
(* I3  no input is applied twice to session persistence.                   *)
(***************************************************************************)
Count(i) == Cardinality({k \in 1..Len(persisted) : persisted[k] = i})
I3 == \A i \in Inputs : Count(i) <= 1

(***************************************************************************)
(* I4  FIFO order of inputs is preserved across the generation change.     *)
(***************************************************************************)
I4 == \A k \in {j \in 1..Len(persisted) : j < Len(persisted)} :
        persisted[k] < persisted[k+1]

(***************************************************************************)
(* I5  no replacement launch before the old process exited, even when the  *)
(*     quiesce request was already admitted and the caller then timed out. *)
(***************************************************************************)
I5 == started = 2 => oldExited

(***************************************************************************)
(* I6  an unprobed or incompatible candidate never retires the worker: no *)
(*     quiesce request is written unless the candidate passed the identity *)
(*     probe, and a staged candidate always exists at that point.          *)
(***************************************************************************)
I6 == rp \in {"sent", "quiesced", "exited"} => (probe = "ok" /\ cand = "staged")

(***************************************************************************)
(* I7  at most one reload is in flight.                                    *)
(***************************************************************************)
I7 == inflight <= 1

(***************************************************************************)
(* I8  no staged-artifact leak: an unused candidate exists on disk only    *)
(*     while a reload is actually in flight.                               *)
(***************************************************************************)
I8 == cand = "staged" => rp # "idle"

(***************************************************************************)
(* I9  the frontend is never told a worker is ready when none is live.     *)
(*     Violated by the historical baseline; corrected-status checks repair. *)
(***************************************************************************)
I9 == published = "ready" => owner # 0

(***************************************************************************)
(* I10 an input is never accepted into a host that has no worker to        *)
(*     deliver it without the host saying that the worker is stopped.      *)
(*     Violated by the historical baseline; corrected-status checks repair. *)
(***************************************************************************)
I10 == owner = 0 /\ pending # {} => published = "stopped"

\* Reachability probes: their intentional violations rule out a vacuous pass
\* that never launches or delivers through the replacement.
NoReplacementDelivery == ~(started = 2 /\ sent[2] = Inputs /\ persisted = <<1, 2>>)

=============================================================================
