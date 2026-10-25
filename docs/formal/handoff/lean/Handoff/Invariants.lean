/-
  Handoff/Invariants.lean

  Safety invariants of the worker reload / handoff protocol, proved for EVERY
  state reachable in the unbounded transition system of Handoff/Model.lean.

  Kernel-proved in this file:
    I1  at most one live generation owns delivery        (I1, I1_atMostOne)
    I2  acknowledge implies persist; journal membership is monotone (I2, I2_monotone)

  NOT kernel-proved here: I3, I4, I5, I6, I7, I8.  See README.md section
  "Verdicts" and "Residual obligations": each of those is stated precisely, the
  Go mechanism carrying it is cited, and the missing Lean lemma is named.
  The development contains no unfinished-proof placeholder and no invented
  axiom: an unproved statement is reported as such in README.md instead.
-/
import Handoff.Model

namespace Handoff

open St

/-- Workhorse for the preservation proofs: hoist the slot condition, split it, then
    discharge with the (deliberately pruned) invariant hypotheses in context. -/
macro "hb" : tactic => `(tactic|
  (try (all_goals (try split))) <;> (try (all_goals (try split))) <;>
  (try (all_goals (try split))) <;>
  simp_all (config := { failIfUnchanged := false })
    [Live, Ackd, Pend, ackJournal, updW_pid, updW_stage, updW_phase, updW_sent, updW_queue,
     updW_seen, updW_frozen, updW_broken] <;>
  (try (all_goals (first | assumption | decide))))

/-- The inductive invariant bundle proved here. -/
structure Inv (s : St) : Prop where
  /-- I1: a generation is live exactly when it is the current one, so delivery
      ownership is single (h.current is one pointer, host.go:70, 330-333). -/
  own : ∀ p, Live s p ↔ s.current = some p
  /-- I2: acknowledgement implies persistence -- an input is acknowledged only
      when the host observed a persisted item for it (host.go:423-428), and only
      a persisted input can be acknowledged (journal.go:160-181). -/
  ackSub : ∀ i, Ackd s i → i ∈ s.store

/-- Reachability from the post-Start state. -/
inductive Reach : St → Prop where
  | init : Reach s0
  | step {s t : St} : Reach s → Step s t → Reach t

theorem Inv_init : Inv s0 := by
  refine { own := ?own, ackSub := ?ackSub }
  case own => decide
  case ackSub => decide

theorem launch_own {s : St} {oldp p' : Pid} (v : W)
    (hown : ∀ p, Live s p ↔ s.current = some p)
    (hcur : s.current = some oldp ∨ s.current = none)
    (hold : (s.w oldp).phase = WPhase.exited)
    (hv : v.phase = WPhase.launching) :
    ∀ p, Live { s with w := updW s.w p' v, current := some p' } p ↔
      ({ s with w := updW s.w p' v, current := some p' } : St).current = some p := by
  intro p
  by_cases hpp : p = p'
  · subst hpp
    simp [Live, hv]
  · simp only [Live, updW_phase, ite_eq_right hpp]
    constructor
    · intro hlive
      have hsp : s.current = some p := (hown p).mp hlive
      rcases hcur with h | h
      · rw [hsp] at h
        have hpe : p = oldp := Option.some.inj h
        rw [hpe] at hlive
        exact absurd hold hlive
      · rw [hsp] at h
        exact absurd h (by simp)
    · intro hc
      exact absurd (Option.some.inj hc).symm hpp

theorem own_step {s t : St} (hown : ∀ p, Live s p ↔ s.current = some p)
    (hs : Step s t) : ∀ p, Live t p ↔ t.current = some p := by
  intro p
  cases hs with
  | submit i hc hfresh => exact hown p
  | dlvLock p' hfree hcur hready hc hf => exact hown p
  | sendNew p' i hd hcur hready hc hf hnext hbr hnew =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hcur, hown]
  | sendDup p' i hd hcur hready hc hf hnext hbr hdup =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hcur, hown]
  | sendLost p' i hd hcur hready hc hf hnext hbr =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hcur, hown]
  | dlvUnlock hd => exact hown p
  | persist p' i rest hready hfrozen hq =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hown]
  | ack i hp hs' => exact hown p
  | quiesceAdmit rid p' hact hrp hcur hready =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hcur, hown]
  | quiesceBusy rid p' a hact hrp hst ha => exact hown p
  | quiesceFail rid p' a hact hrp hst ha =>
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hown]
  | reloadAdmit rid hfree hnone hc hf => exact hown p
  | reloadStage rid a hact hrp => exact hown p
  | reloadStageFail rid hact hrp => exact hown p
  | reloadFence rid hfree hact hrp hc hf => exact hown p
  | reloadGuardBusy rid p' i a hact hrp hdel hcur hpend hsent hst ha => exact hown p
  | reloadQuiesce rid p' hact hrp hdel hcur hguard => exact hown p
  | reloadAbortAwait rid p' a hact hrp hst ha => exact hown p
  | reloadNoProcStop rid a hnone hact hrp hdel hstopped hst ha => exact hown p
  | launchAwait rid oldp p' a hact hrp hold hnone hst hfree hf =>
      exact launch_own (oldp := oldp) (p' := p') (W.fresh p' a s.store) hown
        (Or.inr hnone) hold rfl p
  | launchNoProc rid p' a hnone hact hrp hdel hstopped hst hfree hf =>
      exact launch_own (p' := p') (W.fresh p' a s.store) hown (Or.inr hnone) hfree rfl p
  | hello p' hp =>
      have hc : s.current = some p' :=
        (hown p').mp (by show (s.w p').phase ≠ WPhase.exited; rw [hp]; simp)
      by_cases hpp : p = p' <;> simp [Live, updW_phase, hpp, hc, hown]
  | wExit p' a hlive hstage =>
      by_cases hpp : p = p'
      · subst hpp
        have hcur : s.current = some p := (hown p).mp hlive
        simp [Live, hcur]
      · have hne : (updW s.w p' { s.w p' with phase := WPhase.exited }) p = s.w p :=
          updW_ne hpp _
        by_cases hcur : s.current = some p'
        · have hn : ¬ Live s p := by
            intro hl
            have h := (hown p).mp hl
            rw [hcur] at h
            exact hpp (Option.some.inj h).symm
          simp [Live, hne, hcur, hn]
        · simp [Live, hne, hcur, hown]
  | hostClose hnone hdel => exact hown p
set_option maxHeartbeats 4000000 in
/-- Preservation: one protocol step keeps the bundle. -/

theorem ackSub_step {s t : St} (hack : ∀ i, Ackd s i → i ∈ s.store) (hs : Step s t) :
    ∀ i, Ackd t i → i ∈ t.store := by
  cases hs <;> hb

theorem Inv_step {s t : St} (h : Inv s) (hs : Step s t) : Inv t := by
  obtain ⟨hown, hack⟩ := h
  exact { own := own_step hown hs, ackSub := ackSub_step hack hs }

theorem Reach_inv {s : St} (h : Reach s) : Inv s := by
  induction h with
  | init => exact Inv_init
  | step _ hstep ih => exact Inv_step ih hstep

/-! ### Published verdicts -/

/-- I1: at most one live generation owns delivery. -/
theorem I1 {s : St} (h : Reach s) : ∀ p, Live s p → s.current = some p :=
  fun p hp => ((Reach_inv h).own p).mp hp

/-- I1 (corollary): at most one generation is live at a time. -/
theorem I1_atMostOne {s : St} (h : Reach s) : ∀ p q, Live s p → Live s q → p = q := by
  intro p q hp hq
  have a := ((Reach_inv h).own p).mp hp
  have b := ((Reach_inv h).own q).mp hq
  rw [a] at b
  exact Option.some.inj b

/-- I2: an acknowledged input is always a persisted input. -/
theorem I2 {s : St} (h : Reach s) : ∀ i, Ackd s i → i ∈ s.store :=
  (Reach_inv h).ackSub

/-- I2 (monotonicity): model journal membership is preserved on every step,
    including aborts. No filesystem durability property is proved here. -/
theorem I2_monotone {s t : St} (hs : Step s t) : ∀ i, i ∈ ids s → i ∈ ids t := by
  cases hs <;> first
    | (intro j hj; exact hj)
    | (intro j hj
       simp only [ids, List.mem_append]
       exact Or.inl hj)

end Handoff
