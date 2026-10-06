#!/usr/bin/env bash
# Supply a local tla2tools.jar; this script never downloads or vendors binaries.
set -euo pipefail
: "${TLA2TOOLS_JAR:?Set TLA2TOOLS_JAR to the TLA+ tools v1.8.0 jar}"
jar=$(realpath "$TLA2TOOLS_JAR")
cd "$(dirname "$0")"
work=$(mktemp -d "${TMPDIR:-/tmp}/unreal-checkpoint-tlc.XXXXXX")
trap 'rm -rf "$work"' EXIT
run_tlc() {
  java -Xmx512m -XX:+UseParallelGC -cp "$jar" tlc2.TLC -workers 1 -seed 1 \
    -metadir "$work/states-$1" -noGenerateSpecTE -config "$1.cfg" \
    "${2:-Checkpoint}.tla" > "$work/$1.log" 2>&1
}
for pair in 'claims SafeClaims' 'concurrency SafeConcurrency' \
            'dependencies SafeDependencies' 'identity SafeChildIdentity'; do
  read -r name invariant <<< "$pair"
  rc=0
  run_tlc "legacy-$name" || rc=$?
  # TLC exit 12 is an invariant violation, not a parser/tool failure.
  if [[ "$rc" != 12 ]] || ! grep -q "Invariant $invariant is violated" "$work/legacy-$name.log"; then
    cat "$work/legacy-$name.log"
    echo "Expected legacy counterexample for $invariant; exit=$rc" >&2
    exit 1
  fi
  echo "Confirmed legacy counterexample: $invariant"
  cat "$work/legacy-$name.log"
done
if ! run_tlc fixed; then
  cat "$work/fixed.log"
  exit 1
fi
cat "$work/fixed.log"
grep -q 'Model checking completed. No error has been found.' "$work/fixed.log"
rc=0
run_tlc routing-legacy ChildRouting || rc=$?
if [[ "$rc" != 12 ]] || ! grep -q 'Invariant AllTerminalChildrenRetired is violated' "$work/routing-legacy.log"; then
  cat "$work/routing-legacy.log"
  echo "Expected duplicate-ID completion counterexample; exit=$rc" >&2
  exit 1
fi
cat "$work/routing-legacy.log"
if ! run_tlc routing-fixed ChildRouting; then
  cat "$work/routing-fixed.log"
  exit 1
fi
cat "$work/routing-fixed.log"
grep -q 'Model checking completed. No error has been found.' "$work/routing-fixed.log"
echo 'All expected legacy failures and fixed restore/routing invariants verified.'
