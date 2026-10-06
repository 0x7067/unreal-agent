------------------------- MODULE CoordinatorDispatch -------------------------
EXTENDS Naturals, TLC

CONSTANTS MaxCrashes, SkipStatusCommit, SkipTerminalGuard, SkipLatestOverlay
Ops == 1..2
Statuses == {"Absent", "Ready", "Awaiting", "Terminal"}
Empty == [o \in Ops |-> "Absent"]
FalseMap == [o \in Ops |-> FALSE]

VARIABLES pc, cursor, crashes, committed, latest, local, localStatus,
          added, badUncommittedAdd, badCheckpointAdd
vars == <<pc, cursor, crashes, committed, latest, local, localStatus,
          added, badUncommittedAdd, badCheckpointAdd>>

Init ==
    /\ pc = "translate"
    /\ cursor = 1
    /\ crashes = 0
    /\ committed = FALSE
    /\ latest = Empty
    /\ local = Empty
    /\ localStatus = FALSE
    /\ added = FalseMap
    /\ badUncommittedAdd = FALSE
    /\ badCheckpointAdd = FALSE

\* Pure Translate/Submit builds two Ready values, NOT coordinator local state.
Translate ==
    /\ pc = "translate"
    /\ pc' = "install"
    /\ cursor' = 1
    /\ UNCHANGED <<crashes, committed, latest, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* Each addOperationToLocalState is separate, before local status application.
Install ==
    /\ pc = "install" /\ cursor \in Ops
    /\ local' = [local EXCEPT ![cursor] = "Ready"]
    /\ cursor' = cursor + 1
    /\ UNCHANGED <<pc, crashes, committed, latest, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

ApplyStatus ==
    /\ pc = "install" /\ cursor = 3
    /\ localStatus' = TRUE
    /\ pc' = "appendStatus"
    /\ UNCHANGED <<cursor, crashes, committed, latest, local, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* The one durable record contains BOTH the status and ALL initial operations.
\* Return/dispatch is not part of this atomic journal abstraction.
AppendStatus ==
    /\ pc = "appendStatus"
    /\ committed' = IF SkipStatusCommit THEN committed ELSE TRUE
    /\ latest' = IF SkipStatusCommit THEN latest
                  ELSE [o \in Ops |-> "Ready"]
    /\ pc' = "statusReturn"
    /\ UNCHANGED <<cursor, crashes, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

StatusReturn ==
    /\ pc = "statusReturn"
    /\ pc' = "guard"
    /\ cursor' = 1
    /\ UNCHANGED <<crashes, committed, latest, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* Failures stop this invocation. A full record may nevertheless be recoverable
\* (e.g. close reports an error). No Add is possible before a new recovery.
StatusError(visible) ==
    /\ pc = "appendStatus"
    /\ committed' = (committed \/ visible)
    /\ latest' = IF visible THEN [o \in Ops |-> "Ready"] ELSE latest
    /\ pc' = "stopped"
    /\ UNCHANGED <<cursor, crashes, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

Guard ==
    /\ pc = "guard" /\ cursor \in Ops
    /\ IF local[cursor] = "Terminal" /\ ~SkipTerminalGuard
       THEN /\ cursor' = cursor + 1 /\ pc' = "guard"
       ELSE /\ cursor' = cursor /\ pc' = "add"
    /\ UNCHANGED <<crashes, committed, latest, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* Ghost flags record bad Add events, not a restriction on the transition.
Add ==
    /\ pc = "add" /\ cursor \in Ops
    /\ badUncommittedAdd' = (badUncommittedAdd \/ ~committed)
    /\ badCheckpointAdd' = (badCheckpointAdd
          \/ local[cursor] = "Terminal"
          \/ latest[cursor] # local[cursor])
    /\ added' = [added EXCEPT ![cursor] = TRUE]
    /\ pc' = "guard"
    /\ cursor' = cursor + 1
    /\ UNCHANGED <<crashes, committed, latest, local, localStatus>>

DispatchDone ==
    /\ pc = "guard" /\ cursor = 3
    /\ pc' = "idle"
    /\ UNCHANGED <<cursor, crashes, committed, latest, local, localStatus,
                    added, badUncommittedAdd, badCheckpointAdd>>

\* Manager updates are processed only once dispatch finishes (single loop).
\* Monotone status abstraction; terminal outcomes are collapsed to Terminal.
LocalUpdate(o, next) ==
    /\ pc = "idle" /\ added[o] /\ committed
    /\ \/ /\ local[o] = "Ready" /\ next \in {"Awaiting", "Terminal"}
       \/ /\ local[o] = "Awaiting" /\ next = "Terminal"
    /\ local' = [local EXCEPT ![o] = next]
    /\ cursor' = o
    /\ pc' = "appendUpdate"
    /\ UNCHANGED <<crashes, committed, latest, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

AppendUpdate ==
    /\ pc = "appendUpdate"
    /\ latest' = [latest EXCEPT ![cursor] = local[cursor]]
    /\ pc' = "updateReturn"
    /\ UNCHANGED <<cursor, crashes, committed, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

UpdateReturn ==
    /\ pc = "updateReturn"
    /\ pc' = "idle"
    /\ UNCHANGED <<cursor, crashes, committed, latest, local, localStatus,
                    added, badUncommittedAdd, badCheckpointAdd>>

UpdateError(visible) ==
    /\ pc = "appendUpdate"
    /\ latest' = IF visible THEN [latest EXCEPT ![cursor] = local[cursor]]
                  ELSE latest
    /\ pc' = "stopped"
    /\ UNCHANGED <<cursor, crashes, committed, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* Crash at any boundary, including during replay/overlay or after an error.
Crash ==
    /\ crashes < MaxCrashes
    /\ crashes' = crashes + 1
    /\ pc' = "replay"
    /\ cursor' = 1
    /\ local' = Empty
    /\ localStatus' = FALSE
    /\ added' = FalseMap
    /\ UNCHANGED <<committed, latest, badUncommittedAdd, badCheckpointAdd>>

\* loadHistory first applies the initial status's embedded Ready snapshots.
ReplayOperation ==
    /\ pc = "replay" /\ cursor \in Ops
    /\ local' = IF committed THEN [local EXCEPT ![cursor] = "Ready"]
                 ELSE local
    /\ cursor' = cursor + 1
    /\ UNCHANGED <<pc, crashes, committed, latest, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

ReplayStatus ==
    /\ pc = "replay" /\ cursor = 3
    /\ localStatus' = committed
    /\ pc' = "overlay"
    /\ cursor' = 1
    /\ UNCHANGED <<crashes, committed, latest, local, added,
                    badUncommittedAdd, badCheckpointAdd>>

\* Restored.Operations overlays latest SaveOperation checkpoints, INCLUDING
\* terminal entries still pending in history (no final tool result yet).
Overlay ==
    /\ pc = "overlay" /\ cursor \in Ops
    /\ local' = IF committed /\ ~SkipLatestOverlay
                 THEN [local EXCEPT ![cursor] = latest[cursor]] ELSE local
    /\ cursor' = cursor + 1
    /\ UNCHANGED <<pc, crashes, committed, latest, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

ScheduleRestored ==
    /\ pc = "overlay" /\ cursor = 3
    /\ pc' = IF localStatus THEN "guard" ELSE "translate"
    /\ cursor' = 1
    /\ UNCHANGED <<crashes, committed, latest, local, localStatus, added,
                    badUncommittedAdd, badCheckpointAdd>>

Next == Translate \/ Install \/ ApplyStatus \/ AppendStatus \/ StatusReturn
     \/ (\E visible \in BOOLEAN : StatusError(visible))
     \/ Guard \/ Add \/ DispatchDone
     \/ (\E o \in Ops, next \in {"Awaiting", "Terminal"} : LocalUpdate(o, next))
     \/ AppendUpdate \/ UpdateReturn
     \/ (\E visible \in BOOLEAN : UpdateError(visible))
     \/ Crash \/ ReplayOperation \/ ReplayStatus \/ Overlay \/ ScheduleRestored

Spec == Init /\ [][Next]_vars

TypeOK ==
    /\ pc \in {"translate", "install", "appendStatus", "statusReturn",
               "guard", "add", "idle", "appendUpdate", "updateReturn",
               "stopped", "replay", "overlay"}
    /\ cursor \in 1..3 /\ crashes \in 0..MaxCrashes
    /\ committed \in BOOLEAN /\ localStatus \in BOOLEAN
    /\ latest \in [Ops -> Statuses] /\ local \in [Ops -> Statuses]
    /\ added \in [Ops -> BOOLEAN]
    /\ badUncommittedAdd \in BOOLEAN /\ badCheckpointAdd \in BOOLEAN
CommitBeforeAdd == ~badUncommittedAdd
NoTerminalOrStaleAdd == ~badCheckpointAdd
=============================================================================
