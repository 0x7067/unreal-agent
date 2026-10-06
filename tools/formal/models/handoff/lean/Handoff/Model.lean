/-
  Handoff/Model.lean

  An inductive state-transition model of the TUI worker reload / handoff protocol
  of the Go agent harness (cmd/internal/workerhost, cmd/internal/workerruntime,
  harness/inbox, harness/ingress).  Every rule cites the Go code it abstracts.

  Identity bounds: 3 input ids, 3 reusable process slots, 3 artifact ids and
  3 reusable reload slots. Lists and transition traces have no length bound;
  this is NOT a finite-state explorer or a refinement proof of the Go code.
  Handoff/Invariants.lean proves properties of all reachable model states.
-/
import Std

namespace Handoff

/-- Input identity: the durable-ingress dedup key (harness/inbox/inbox.go:14). -/
abbrev Id  := Fin 3
/-- Staged-artifact identity: the immutable per-generation executable copy
    (cmd/internal/workerhost/stage.go:55-93). -/
abbrev AId := Fin 3
/-- Worker process slot: one slot per generation. -/
abbrev Pid := Fin 3
/-- Reload instance identity (single-flight h.reload CAS, host.go:536). -/
abbrev Rid := Fin 3

/-- Worker-generation lifecycle.  launching = started, hello/ready not yet
    observed (host.go:323-365); ready = serving; retired = the coordinator
    retired after an admitted quiesce (run.go:353-371, host.go:465-467);
    exited = the process has been reaped and its callbacks drained
    (host.go:488-530). -/
inductive WPhase where
  | launching | ready | retired | exited
  deriving DecidableEq, BEq, Repr

/-- One worker generation.  sent is process.sent (host.go:104, marked before
    the write at host.go:266); queue is the worker-side inbox FIFO holding
    admitted-but-not-yet-persisted inputs (harness/inbox/local.go:143-148);
    seen is the inbox dedup set seeded from persisted input ids
    (run.go:250 inbox.New(ownerCtx, restored.InputIDs),
    sessionstore/localfile/state.go:293-314); frozen is the Freeze admission
    fence (local.go:112-141); broken latches a transport failure: any frame
    write error cancels the worker (host.go:266-271), abstracted here by forbidding further
    delivery on that pipe (not a proved property of every write error). -/
structure W where
  pid    : Pid
  stage  : AId
  phase  : WPhase
  sent   : List Id
  queue  : List Id
  seen   : List Id
  frozen : Bool
  broken : Bool
  deriving DecidableEq, BEq, Repr

/-- Phase of one reload instance.  starting = admitted by the CAS before the
    candidate is staged (host.go:536-555); staged = probe succeeded
    (stage.go:132-170); fenced = the delivery token is held and the
    pending-journal guard runs (host.go:566-590); quiesced p = the quiesce
    frame was written to p and waits for busy/quiesced (host.go:596-613);
    awaiting p = quiesce admitted, waiting for the old process to exit
    (host.go:617-621). -/
inductive RPhase where
  | starting
  | staged
  | fenced
  | quiesced (pid : Pid)
  | awaiting (pid : Pid)
  deriving DecidableEq, BEq, Repr

/-- One reload instance.  stage is the probed staged artifact it holds; it is
    none until the probe succeeded, and it is the only artifact a replacement may
    be launched from (host.go:555, 631). -/
structure R where
  phase : RPhase
  stage : Option AId
  deriving DecidableEq, BEq, Repr

/-- Holder of the delivery fence h.delivery (host.go:82, 744-752): a one-token
    mutual exclusion between the dispatch loop and a handoff. -/
inductive Hold where
  | free
  | byDispatch
  | byReload (rid : Rid)
  deriving DecidableEq, BEq, Repr

/-- Instances for the finite function components (available for comparing and
    printing states; not used by the invariant proofs). -/

instance : BEq (Pid → W) := ⟨fun f g => (f 0 == g 0) && (f 1 == g 1) && (f 2 == g 2)⟩
instance : Repr (Pid → W) := ⟨fun f _ => repr ((f 0, f 1, f 2))⟩
instance : BEq (Rid → Bool) := ⟨fun f g => (f 0 == g 0) && (f 1 == g 1) && (f 2 == g 2)⟩
instance : Repr (Rid → Bool) := ⟨fun f _ => repr ((f 0, f 1, f 2))⟩
instance : BEq (Rid → R) := ⟨fun f g => (f 0 == g 0) && (f 1 == g 1) && (f 2 == g 2)⟩
instance : Repr (Rid → R) := ⟨fun f _ => repr ((f 0, f 1, f 2))⟩

/-- The complete host state. -/
structure St where
  /-- durable ingress journal in admission order (harness/ingress/journal.go:119-157).
      The acknowledged flag of each entry lives in the separate set acked; Pending()
      is the journal minus that set (journal.go:148-157). -/
  journal  : List Id
  /-- the acknowledged subset of the journal (journal.go:160-181) -/
  acked    : List Id
  /-- session persistence: input records in append order
      (coordinator loop.go:1031 -> localfile/store.go:122-141) -/
  store    : List Id
  /-- all generations ever launched, indexed by slot -/
  w        : Pid → W
  /-- h.current (host.go:70): may be none between generations -/
  current  : Option Pid
  /-- which reload-instance slots exist (h.reload CAS, host.go:536) -/
  active   : Rid → Bool
  /-- reload instance data -/
  rs       : Rid → R
  /-- the delivery fence -/
  delivery : Hold
  /-- h.stages: artifacts recorded for cleanup (stage.go:167-169) -/
  stages   : List AId
  /-- h.stopped: a proven retirement (host.go:76, 507-511) -/
  stopped  : Bool
  /-- h.failure non-nil (host.go:77) -/
  failed   : Bool
  /-- h.closing (host.go:74) -/
  closing  : Bool
  deriving BEq, Repr

namespace St

/-- Update one worker slot. -/
def updW (f : Pid → W) (p : Pid) (v : W) : Pid → W := fun q => if q = p then v else f q
/-- Update one reload slot. -/
def updR (f : Rid → R) (i : Rid) (v : R) : Rid → R := fun j => if j = i then v else f j
/-- Update one reload-active flag. -/
def updA (f : Rid → Bool) (i : Rid) (b : Bool) : Rid → Bool := fun j => if j = i then b else f j

variable {f : Pid → W} {g : Rid → R} {a : Rid → Bool}

@[simp] theorem updW_same (p : Pid) (v : W) : updW f p v p = v := by simp [updW]
theorem updW_ne {p q : Pid} (h : q ≠ p) (v : W) : updW f p v q = f q := by simp [updW, h]
@[simp] theorem updR_same (i : Rid) (v : R) : updR g i v i = v := by simp [updR]
theorem updR_ne {i j : Rid} (h : j ≠ i) (v : R) : updR g i v j = g j := by simp [updR, h]
@[simp] theorem updA_same (i : Rid) (b : Bool) : updA a i b i = b := by simp [updA]
theorem updA_ne {i j : Rid} (h : j ≠ i) (b : Bool) : updA a i b j = a j := by simp [updA, h]

/-- Field projections of a slot update: hoisting the condition to the top is what
    lets proofs split the (decidable) slot comparison. -/
@[simp] theorem updW_pid (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).pid = if q = p then v.pid else (f q).pid := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_stage (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).stage = if q = p then v.stage else (f q).stage := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_phase (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).phase = if q = p then v.phase else (f q).phase := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_sent (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).sent = if q = p then v.sent else (f q).sent := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_queue (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).queue = if q = p then v.queue else (f q).queue := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_seen (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).seen = if q = p then v.seen else (f q).seen := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_frozen (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).frozen = if q = p then v.frozen else (f q).frozen := by
  by_cases h : q = p <;> simp [updW, h]
@[simp] theorem updW_broken (f : Pid → W) (p q : Pid) (v : W) : (updW f p v q).broken = if q = p then v.broken else (f q).broken := by
  by_cases h : q = p <;> simp [updW, h]


end St

open St

/-- A generation slot holds a live process (not yet reaped). -/
abbrev Live (s : St) (p : Pid) : Prop := (s.w p).phase ≠ WPhase.exited

/-- Journal ids in admission order (the FIFO order of Pending(),
    harness/ingress/journal.go:146-157). -/
def ids (s : St) : List Id := s.journal

/-- An input is still pending in durable ingress (unacknowledged). -/
abbrev Pend (s : St) (i : Id) : Prop := i ∈ s.journal ∧ i ∉ s.acked

/-- An input has been acknowledged: the host observed a persisted item for it
    (host.go:423-428). -/
abbrev Ackd (s : St) (i : Id) : Prop := i ∈ s.acked

/-- journal.Pending() order (harness/ingress/journal.go:148-157). -/
def pendingIds (s : St) : List Id := s.journal.filter (fun i => !s.acked.contains i)

/-- The frame the dispatch loop would write next: the first pending input the
    current generation has not been handed yet (host.go:262-267). -/
def nextSend (s : St) : Option Id :=
  match s.current with
  | some p => ((pendingIds s).filter (fun i => !(s.w p).sent.contains i)).head?
  | none   => none

/-- Acknowledgement of one journal entry (host.go:423-428, journal.go:160-181). -/
def ackJournal (s : St) (i : Id) : St :=
  { s with acked := i :: s.acked }

/-! ### Initial state

s0 is the post-Start state (host.go:175-198): one staged+launched generation is
ready, the delivery token is free, nothing is journaled yet. -/

/-- The freshly launched worker record: no frames sent yet, empty FIFO, dedup set
    seeded from the session store (run.go:250), pipe healthy (host.go:322, 356-360). -/
def W.fresh (p : Pid) (a : AId) (seen : List Id) : W :=
  { pid := p, stage := a, phase := WPhase.launching, sent := [], queue := [], seen := seen,
    frozen := false, broken := false }

def W.blank (p : Pid) : W :=
  { pid := p, stage := 0, phase := .exited, sent := [], queue := [], seen := [], frozen := false, broken := false }

def s0 : St :=
  { journal := []
    acked := []
    store := []
    w := fun q => if q = (0 : Pid) then
        { pid := 0, stage := 0, phase := .ready, sent := [], queue := [], seen := [], frozen := false, broken := false }
      else W.blank q
    current := some (0 : Pid)
    active := fun _ => false
    rs := fun _ => { phase := .starting, stage := none }
    delivery := .free
    stages := [(0 : AId)]
    stopped := false
    failed := false
    closing := false }

/-! ### Transition relation

Each constructor is one observable step of the protocol.  Host-side steps are the
reload goroutine, the dispatch goroutine, and Submit/Close; worker-side steps are
the coordinator loop, the inbox actor, and the process lifecycle. -/

inductive Step : St → St → Prop where

  /-- Host.Submit admits an input to durable ingress (host.go:217-243).  New
      messages are admitted during a reload too (host.go:215-216). -/
  | submit (s : St) (i : Id) (hc : s.closing = false) (hfresh : i ∉ s.journal) :
      Step s { s with journal := s.journal ++ [i] }

  /-- The dispatch loop takes the delivery token and snapshots h.current
      (host.go:245-261). -/
  | dlvLock (s : St) (p : Pid) (hfree : s.delivery = Hold.free) (hcur : s.current = some p)
      (hready : (s.w p).phase = WPhase.ready) (hc : s.closing = false) (hf : s.failed = false) :
      Step s { s with delivery := Hold.byDispatch }

  /-- One input frame: the input is new to this generation's inbox, so it is
      admitted into the FIFO (host.go:262-267 write, local.go:143-148 admit).
      sent is marked before the write, so a failed write still marks it
      (host.go:266). -/
  | sendNew (s : St) (p : Pid) (i : Id) (hd : s.delivery = Hold.byDispatch)
      (hcur : s.current = some p) (hready : (s.w p).phase = WPhase.ready)
      (hc : s.closing = false) (hf : s.failed = false) (hnext : nextSend s = some i)
      (hbr : (s.w p).broken = false) (hnew : i ∉ (s.w p).seen) :
      Step s { s with w := updW s.w p { s.w p with
                sent := i :: (s.w p).sent, seen := i :: (s.w p).seen,
                queue := (s.w p).queue ++ [i] } }

  /-- One input frame the inbox already knows: admitted to the pipe, dropped by
      dedup (local.go:144-147).  No second persistence follows. -/
  | sendDup (s : St) (p : Pid) (i : Id) (hd : s.delivery = Hold.byDispatch)
      (hcur : s.current = some p) (hready : (s.w p).phase = WPhase.ready)
      (hc : s.closing = false) (hf : s.failed = false) (hnext : nextSend s = some i)
      (hbr : (s.w p).broken = false) (hdup : i ∈ (s.w p).seen) :
      Step s { s with w := updW s.w p { s.w p with sent := i :: (s.w p).sent } }

  /-- One input frame whose write failed: sent is marked anyway and the worker is
      cancelled (host.go:266-271). This abstraction assumes total loss and
      permanently latches broken; partial/encoding failures are not represented. -/
  | sendLost (s : St) (p : Pid) (i : Id) (hd : s.delivery = Hold.byDispatch)
      (hcur : s.current = some p) (hready : (s.w p).phase = WPhase.ready)
      (hc : s.closing = false) (hf : s.failed = false) (hnext : nextSend s = some i)
      (hbr : (s.w p).broken = false) :
      Step s { s with w := updW s.w p { s.w p with
                sent := i :: (s.w p).sent, broken := true } }

  /-- The dispatch loop releases the delivery token (host.go:274). -/
  | dlvUnlock (s : St) (hd : s.delivery = Hold.byDispatch) :
      Step s { s with delivery := Hold.free }

  /-- The coordinator persists the head of its inbox FIFO, in order
      (quiescence.go:100 processInputs, loop.go:1031 AppendInput).  A frozen
      inbox admits nothing, and the frozen FIFO is drained only by the quiesce
      handler. -/
  | persist (s : St) (p : Pid) (i : Id) (rest : List Id)
      (hready : (s.w p).phase = WPhase.ready) (hfrozen : (s.w p).frozen = false)
      (hq : (s.w p).queue = i :: rest) :
      Step s { s with store := s.store ++ [i], w := updW s.w p { s.w p with queue := rest } }

  /-- The host observes a persisted item for an input and acknowledges durable
      ingress (host.go:423-428).  Acks lag persistence and may be repeated by the
      startup replay (run.go:206-221); both are over-approximated here as a free
      ack of any persisted input. -/
  | ack (s : St) (i : Id) (hp : Pend s i) (hs : i ∈ s.store) : Step s (ackJournal s i)

  /-- Quiesce admitted: the coordinator freezes admission, drains the already
      admitted FIFO into the session store in FIFO order, and retires
      (quiescence.go:64-138; busy is the other outcome, quiesceBusy). -/
  | quiesceAdmit (s : St) (rid : Rid) (p : Pid) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.quiesced p) (hcur : s.current = some p)
      (hready : (s.w p).phase = WPhase.ready) :
      Step s { s with
        store := s.store ++ (s.w p).queue
        w := updW s.w p { s.w p with queue := [], frozen := true, phase := WPhase.retired }
        rs := updR s.rs rid { s.rs rid with phase := RPhase.awaiting p } }

  /-- Quiesce refused because the loop is busy: the reload aborts, the staged
      artifact is discarded and the token released (host.go:614-616, 560-564, 569).
      Live work is never cancelled (host.go:532). -/
  | quiesceBusy (s : St) (rid : Rid) (p : Pid) (a : AId) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.quiesced p) (hst : (s.rs rid).stage = some a)
      (ha : a ∈ s.stages) :
      Step s { s with
        stages := s.stages.filter (fun x => x ≠ a)
        active := updA s.active rid false
        delivery := Hold.free }

  /-- The quiesce frame write failed: transport uncertain, no replacement started,
      the artifact is discarded (host.go:596-599, 560-564). -/
  | quiesceFail (s : St) (rid : Rid) (p : Pid) (a : AId) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.quiesced p) (hst : (s.rs rid).stage = some a)
      (ha : a ∈ s.stages) :
      Step s { s with
        stages := s.stages.filter (fun x => x ≠ a)
        active := updA s.active rid false
        w := updW s.w p { s.w p with broken := true }
        delivery := Hold.free }

  /-- The reload CAS admits exactly one in-flight reload (host.go:536-539). -/
  | reloadAdmit (s : St) (rid : Rid) (hfree : s.active rid = false)
      (hnone : ∀ j, s.active j = false) (hc : s.closing = false) (hf : s.failed = false) :
      Step s { s with active := updA s.active rid true,
                       rs := updR s.rs rid { phase := RPhase.starting, stage := none } }

  /-- stage() copied the candidate to a private immutable path and the
      worker-probe handshake returned a compatible Identity (stage.go:25-171,
      workerproto/protocol.go:30-35, run.go:33-44). -/
  | reloadStage (s : St) (rid : Rid) (a : AId) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.starting) :
      Step s { s with
        rs := updR s.rs rid { s.rs rid with phase := RPhase.staged, stage := some a }
        stages := a :: s.stages }

  /-- The candidate was rejected (unbuildable, unprobeable, or an incompatible
      Identity): the reload ends and the current worker is untouched
      (host.go:555-558). -/
  | reloadStageFail (s : St) (rid : Rid) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.starting) :
      Step s { s with active := updA s.active rid false }

  /-- The reload takes the delivery token and checks the pending-journal guard
      (host.go:566-590). -/
  | reloadFence (s : St) (rid : Rid) (hfree : s.delivery = Hold.free)
      (hact : s.active rid = true) (hrp : (s.rs rid).phase = RPhase.staged)
      (hc : s.closing = false) (hf : s.failed = false) :
      Step s { s with delivery := Hold.byReload rid,
                       rs := updR s.rs rid { s.rs rid with phase := RPhase.fenced } }

  /-- The guard refuses: a pending input has already been handed to the current
      generation, so its fate is unconfirmed.  Abort, discard, release
      (host.go:586-590, 560-564, 569). -/
  | reloadGuardBusy (s : St) (rid : Rid) (p : Pid) (i : Id) (a : AId) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.fenced) (hdel : s.delivery = Hold.byReload rid)
      (hcur : s.current = some p) (hpend : Pend s i) (hsent : i ∈ (s.w p).sent)
      (hst : (s.rs rid).stage = some a) (ha : a ∈ s.stages) :
      Step s { s with
        stages := s.stages.filter (fun x => x ≠ a)
        active := updA s.active rid false
        delivery := Hold.free }

  /-- The guard passes: the quiesce frame is written to the current generation
      with a fresh correlation id (host.go:591-599). -/
  | reloadQuiesce (s : St) (rid : Rid) (p : Pid) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.fenced) (hdel : s.delivery = Hold.byReload rid)
      (hcur : s.current = some p) (hguard : ∀ i, Pend s i → i ∉ (s.w p).sent) :
      Step s { s with rs := updR s.rs rid { s.rs rid with phase := RPhase.quiesced p } }

  /-- Cancellation/timeout after quiesce admission:
      ambiguous, so no replacement is launched and the artifact is discarded
      (host.go:611-613, 617-621). -/
  | reloadAbortAwait (s : St) (rid : Rid) (p : Pid) (a : AId) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.awaiting p) (hst : (s.rs rid).stage = some a)
      (ha : a ∈ s.stages) :
      Step s { s with
        stages := s.stages.filter (fun x => x ≠ a)
        active := updA s.active rid false
        delivery := Hold.free }

  /-- No worker and no proven retirement: ErrStopped, nothing launched
      (host.go:627-629). -/
  | reloadNoProcStop (s : St) (rid : Rid) (a : AId) (hnone : s.current = none)
      (hact : s.active rid = true) (hrp : (s.rs rid).phase = RPhase.fenced)
      (hdel : s.delivery = Hold.byReload rid) (hstopped : s.stopped = false)
      (hst : (s.rs rid).stage = some a) (ha : a ∈ s.stages) :
      Step s { s with
        stages := s.stages.filter (fun x => x ≠ a)
        active := updA s.active rid false
        delivery := Hold.free }

  /-- The replacement is launched ONLY after the quiesce-admitted old generation
      was reaped, from the probed staged artifact (host.go:617-631).  The new
      inbox is seeded with every persisted input id (run.go:250,
      localstore/state.go:293-314), which is what makes a resend idempotent. -/
  | launchAwait (s : St) (rid : Rid) (oldp : Pid) (p' : Pid) (a : AId)
      (hact : s.active rid = true) (hrp : (s.rs rid).phase = RPhase.awaiting oldp)
      (hold : (s.w oldp).phase = WPhase.exited) (hnone : s.current = none)
      (hst : (s.rs rid).stage = some a)
      (hfree : (s.w p').phase = WPhase.exited) (hf : s.failed = false) :
      Step s { s with
        w := updW s.w p' (W.fresh p' a s.store)
        current := some p'
        active := updA s.active rid false
        stages := a :: s.stages.filter (fun x => x ≠ a)
        stopped := false
        delivery := Hold.free }

  /-- The no-worker path: a proven retirement (h.stopped) and no process start the
      replacement without a quiesce (host.go:627-631). -/
  | launchNoProc (s : St) (rid : Rid) (p' : Pid) (a : AId)
      (hnone : s.current = none) (hact : s.active rid = true)
      (hrp : (s.rs rid).phase = RPhase.fenced) (hdel : s.delivery = Hold.byReload rid)
      (hstopped : s.stopped = true) (hst : (s.rs rid).stage = some a)
      (hfree : (s.w p').phase = WPhase.exited) (hf : s.failed = false) :
      Step s { s with
        w := updW s.w p' (W.fresh p' a s.store)
        current := some p'
        active := updA s.active rid false
        stages := a :: s.stages.filter (fun x => x ≠ a)
        stopped := false
        delivery := Hold.free }

  /-- hello then ready were observed for the new generation (host.go:353-367). -/
  | hello (s : St) (p : Pid) (hp : (s.w p).phase = WPhase.launching) :
      Step s { s with w := updW s.w p { s.w p with phase := WPhase.ready } }

  /-- The process exits (free environment step: a crash, a SIGINT, or the reaping
      after a clean retirement).  This joins command.Wait, discards the
      generation's staged artifact, and records h.stopped/h.failure
      (host.go:488-530). -/
  | wExit (s : St) (p : Pid) (a : AId) (hlive : (s.w p).phase ≠ WPhase.exited)
      (hstage : (s.w p).stage = a) :
      Step s { s with
        w := updW s.w p { s.w p with phase := WPhase.exited }
        stages := s.stages.filter (fun x => x ≠ a)
        current := if s.current = some p then none else s.current
        stopped := if (s.current = some p ∧ (s.w p).phase = WPhase.retired ∧ s.failed = false)
                   then true else s.stopped
        failed := if (s.current = some p ∧ (s.w p).phase ≠ WPhase.retired ∧ s.closing = false)
                  then true else s.failed }

  /-- Abstract Close clears artifact IDs with no reload active. Real Close
      (host.go:648-719) also joins workers/admissions; that is not modelled here. -/
  | hostClose (s : St) (hnone : ∀ j, s.active j = false) (hdel : s.delivery = Hold.free) :
      Step s { s with closing := true, stages := [] }

end Handoff
