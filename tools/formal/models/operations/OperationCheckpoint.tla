---------------------------- MODULE OperationCheckpoint ----------------------------
(***************************************************************************)
(* Abstract model of the operation manager actor (harness/operation/        *)
(* local_manager.go), the coordinator that persists its updates            *)
(* (harness/coordinator/loop.go handleOperationUpdate -> SaveOperation),    *)
(* and crash/resume (Run -> dispatchOperationsToManager).                   *)
(*                                                                         *)
(* Two operations:                                                         *)
(*   "shell"  - a local shell operation from its create_err phase on.      *)
(*              Shell.resume (shell.go) fails a restored phase=process     *)
(*              checkpoint instead of re-running the command, and re-runs  *)
(*              any earlier phase; file creation is idempotent             *)
(*              (useExistingPath), so create_err always succeeds again.    *)
(*   "remote" - a remote job whose cancellation can fail                   *)
(*              (remoteUnknown: terminal, but retained in the map) and     *)
(*              whose handler update stream can close.                     *)
(*                                                                         *)
(* Fix toggles (FALSE = code as of 9700570):                               *)
(*   ExclusiveErrCreate - the err-file create, the last step before the    *)
(*                    process starts, fails if the file exists; a resumed  *)
(*                    shell then fails as "outcome unknown".               *)
(*   CloseSkipsTerm - the handler-closed branch skips operations that are  *)
(*                    already terminal (as every other remote branch does).*)
(***************************************************************************)
EXTENDS Naturals, Sequences

CONSTANTS ExclusiveErrCreate, CloseSkipsTerm, MaxCrashes

Ops == {"shell", "remote"}
NoUpdate == [op |-> "none", term |-> FALSE, phase |-> "none"]

VARIABLES
  up,          \* runtime (manager + coordinator) alive
  crashes,
  \* manager-local, volatile
  phase,       \* shell phase in the manager: "none" (not admitted), "create_err", "process", "done"
  active,      \* shell primitive in flight: "none", "create", "process"
  errFile,     \* the shell's err file exists (filesystem: survives crashes)
  rmap,        \* remote op retained in the manager map
  rterm,       \* remote op terminal in the manager
  runknown,    \* remote outcome unknown (retained after terminal)
  hopen,       \* remote handler update stream open
  queue,       \* pendingUpdates + Updates channel, FIFO, volatile
  \* durable (session store)
  stored,      \* op -> [term, phase]
  \* ghost history, survives crashes
  spawns,      \* number of times the shell command process was started
  termWrites   \* op -> number of terminal checkpoints written

vars == <<up, crashes, phase, active, errFile, rmap, rterm, runknown, hopen,
          queue, stored, spawns, termWrites>>

Upd(o, t, p) == [op |-> o, term |-> t, phase |-> p]

Init ==
  /\ up = TRUE
  /\ crashes = 0
  \* The shell op is mid-flight: create_err is durable and its create runs.
  /\ phase = "create_err"
  /\ active = "create"
  /\ errFile = FALSE
  /\ rmap = TRUE
  /\ rterm = FALSE
  /\ runknown = FALSE
  /\ hopen = TRUE
  /\ queue = <<>>
  /\ stored = [o \in Ops |-> IF o = "shell"
                             THEN [term |-> FALSE, phase |-> "create_err"]
                             ELSE [term |-> FALSE, phase |-> "none"]]
  /\ spawns = 0
  /\ termWrites = [o \in Ops |-> 0]

------------------------------------------------------------------------------
(* Shell operation, acceptLocalStep: appendLocalUpdate then startLocalPrimitive *)

CreateErrDone ==
  /\ up /\ phase = "create_err" /\ active = "create"
  /\ IF ExclusiveErrCreate /\ errFile
       \* Exclusive create fails: the shell fails as outcome unknown.
       THEN /\ phase' = "done" /\ active' = "none"
            /\ queue' = Append(queue, Upd("shell", TRUE, "done"))
            /\ UNCHANGED <<spawns, errFile>>
       \* useExistingPath (or a fresh create) succeeds, then process starts.
       ELSE /\ phase' = "process" /\ active' = "process" /\ errFile' = TRUE
            /\ queue' = Append(queue, Upd("shell", FALSE, "process"))
            /\ spawns' = spawns + 1
  /\ UNCHANGED <<up, crashes, rmap, rterm, runknown, hopen, stored, termWrites>>

ProcessExit ==
  /\ up /\ active = "process"
  /\ active' = "none"
  /\ phase' = "done"
  /\ queue' = Append(queue, Upd("shell", TRUE, "done"))
  /\ UNCHANGED <<up, crashes, errFile, rmap, rterm, runknown, hopen, stored, spawns, termWrites>>

------------------------------------------------------------------------------
(* Remote job *)

\* requestCancellation error: remoteUnknown, failLocalOperation emits Failed,
\* finishOwned keeps the op because remoteUnknown.
CancelFails ==
  /\ up /\ rmap /\ ~rterm
  /\ rterm' = TRUE /\ runknown' = TRUE
  /\ queue' = Append(queue, Upd("remote", TRUE, "none"))
  /\ UNCHANGED <<up, crashes, phase, active, errFile, rmap, hopen, stored, spawns, termWrites>>

\* A handler terminal update. For a retained unknown op it only releases it.
RemoteFinishes ==
  /\ up /\ rmap /\ hopen
  /\ IF rterm
       THEN UNCHANGED queue
       ELSE queue' = Append(queue, Upd("remote", TRUE, "none"))
  /\ rmap' = FALSE /\ rterm' = TRUE /\ runknown' = FALSE
  /\ UNCHANGED <<up, crashes, phase, active, errFile, hopen, stored, spawns, termWrites>>

\* update.closed: every op of the handler is failed via failLocalOperation.
HandlerCloses ==
  /\ up /\ hopen
  /\ hopen' = FALSE
  /\ IF rmap /\ ~(CloseSkipsTerm /\ rterm)
       THEN /\ queue' = Append(queue, Upd("remote", TRUE, "none"))
            /\ rterm' = TRUE /\ runknown' = TRUE
       ELSE UNCHANGED <<queue, rterm, runknown>>
  /\ UNCHANGED <<up, crashes, phase, active, errFile, rmap, stored, spawns, termWrites>>

------------------------------------------------------------------------------
(* Coordinator: handleOperationUpdate stores every update unconditionally. *)

Persist ==
  /\ up /\ queue /= <<>>
  /\ LET u == Head(queue) IN
       /\ stored' = [stored EXCEPT ![u.op] = [term |-> u.term, phase |-> u.phase]]
       /\ termWrites' = IF u.term THEN [termWrites EXCEPT ![u.op] = @ + 1] ELSE termWrites
  /\ queue' = Tail(queue)
  /\ UNCHANGED <<up, crashes, phase, active, errFile, rmap, rterm, runknown, hopen, spawns>>

------------------------------------------------------------------------------
(* Crash and resume.  A crash loses all volatile state; resume re-adds every  *)
(* stored nonterminal operation (dispatchOperationsToManager).               *)

Crash ==
  /\ up /\ crashes < MaxCrashes
  /\ up' = FALSE /\ crashes' = crashes + 1
  /\ queue' = <<>>
  /\ phase' = "none" /\ active' = "none"
  /\ rmap' = FALSE /\ rterm' = FALSE /\ runknown' = FALSE /\ hopen' = FALSE
  /\ UNCHANGED <<errFile, stored, spawns, termWrites>>

Resume ==
  /\ ~up
  /\ up' = TRUE /\ hopen' = TRUE
  /\ IF stored["shell"].term
       THEN UNCHANGED <<phase, active, queue>>
       ELSE IF stored["shell"].phase = "process"
         \* Shell.resume: a restored process phase fails, never re-runs.
         THEN /\ phase' = "done" /\ active' = "none"
              /\ queue' = <<Upd("shell", TRUE, "done")>>
         \* Any earlier phase is re-dispatched (create_err stands for the lagging phases).
         ELSE /\ phase' = stored["shell"].phase /\ active' = "create"
              /\ queue' = <<>>
  /\ rmap' = ~stored["remote"].term
  /\ UNCHANGED <<crashes, errFile, rterm, runknown, stored, spawns, termWrites>>

Next ==
  \/ CreateErrDone \/ ProcessExit
  \/ CancelFails \/ RemoteFinishes \/ HandlerCloses
  \/ Persist \/ Crash \/ Resume

Spec == Init /\ [][Next]_vars

------------------------------------------------------------------------------
TypeOK ==
  /\ up \in BOOLEAN /\ crashes \in 0..MaxCrashes
  /\ phase \in {"none", "create_err", "process", "done"}
  /\ active \in {"none", "create", "process"}
  /\ spawns \in Nat
  /\ termWrites \in [Ops -> Nat]

\* The shell command must run at most once (Shell.resume refuses to re-run a
\* recorded process phase precisely to guarantee this).
AtMostOnceSpawn == spawns <= 1

\* An operation reaches exactly one durable terminal checkpoint: the result the
\* model was shown must not be silently rewritten.
SingleTerminalCheckpoint == \A o \in Ops : termWrites[o] <= 1
==============================================================================
