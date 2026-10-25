#!/usr/bin/env bash
# Bounded TLA+/TLC check of the TUI worker reload/handoff model.
# Supply a local tla2tools.jar; this script never downloads or vendors binaries.
set -euo pipefail
: "${TLA2TOOLS_JAR:?Set TLA2TOOLS_JAR to the TLA+ tools jar}"
jar=$(realpath "$TLA2TOOLS_JAR")
cd "$(dirname "$0")"
work=$(mktemp -d "${TMPDIR:-/tmp}/unreal-handoff-tlc.XXXXXX")
trap 'rm -rf "$work"' EXIT

# Every run is bounded: -workers 1 -seed 1 for determinism, and a hard five
# minute wall clock per configuration (see README "Bounds").
run_tlc() { # name module
  timeout 300 java -Xmx1g -XX:+UseParallelGC -cp "$jar" tlc2.TLC -workers 1 \
    -seed 1 -metadir "$work/states-$1" -noGenerateSpecTE -config "$1.cfg" \
    "$2.tla" > "$work/$1.log" 2>&1
}

summary() { grep -o '[0-9]* distinct states found' "$work/$1.log" | tail -1; }
last_state() { sed -n '/The behavior up to this point is:/,$p' "$work/$1.log" | tail -22; }

fail=0
expect_clean() { # name module
  local rc=0
  run_tlc "$1" "$2" || rc=$?
  if [[ $rc -ne 0 ]] || ! grep -q 'Model checking completed. No error has been found.' "$work/$1.log"; then
    echo "FAIL $1: expected an exhaustive clean run, exit=$rc" >&2
    cat "$work/$1.log" >&2
    fail=1
    return
  fi
  echo "ok  $1: clean, $(summary "$1")"
}

expect_violation() { # name module invariant
  local rc=0
  run_tlc "$1" "$2" || rc=$?
  if [[ $rc -ne 12 ]] || ! grep -q "Invariant $3 is violated" "$work/$1.log"; then
    echo "FAIL $1: expected TLC exit 12 with 'Invariant $3 is violated', exit=$rc" >&2
    cat "$work/$1.log" >&2
    fail=1
    return
  fi
  echo "ok  $1: $3 violated, $(summary "$1")"
  last_state "$1"
}

# --- Historical guarded baseline: I1-I8 hold; status repair not enabled ---
expect_clean handoff-fixed Handoff

# --- I9/I10 reproduce the original status defect, not current Go behavior ---
expect_violation handoff-defect-status Handoff I9
expect_violation handoff-defect-input  Handoff I10
# Corrected-status candidate reflects the shipped repair, without Go refinement.
# It must restore I9/I10 without weakening I1-I8.
expect_clean handoff-corrected-status Handoff

# --- Counterfactuals: assert outcomes, including redundant defenses --------
expect_violation handoff-legacy-early-launch     Handoff I5
expect_clean handoff-legacy-no-seed Handoff
expect_violation handoff-legacy-no-seed-no-guard Handoff I3
expect_violation handoff-legacy-no-discard       Handoff I8
expect_violation handoff-legacy-no-probe         Handoff I6
expect_violation handoff-legacy-no-cas           Handoff I7

# Under this abstraction, the persistence guard is redundant with the dedup seed:
# removing it (seed intact) keeps I1-I8, so this run must stay clean.
expect_clean handoff-legacy-no-busy-guard Handoff

# --- Inbox admission fence (harness/inbox/local.go) --------------------------
expect_clean inbox-fixed InboxFence
expect_violation inbox-legacy-thaw-abandons     InboxFence NoLoss
expect_violation inbox-legacy-thaw-resets-dedup InboxFence NoDuplicateDelivery

# Anti-vacuity: successful replacement delivery and nonempty Freeze are reachable.
expect_violation handoff-reachable Handoff NoReplacementDelivery
expect_violation inbox-reachable InboxFence NoTransfer

if [[ $fail -ne 0 ]]; then
  echo "handoff model check FAILED" >&2
  exit 1
fi
echo "All handoff/inbox configurations produced their expected outcome."
