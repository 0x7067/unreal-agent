#!/usr/bin/env bash
# Bounded TLA+/TLC check of the operation checkpoint/dispatch model.
# Supply a local tla2tools.jar; this script never downloads or vendors binaries.
set -euo pipefail
: "${TLA2TOOLS_JAR:?Set TLA2TOOLS_JAR to the TLA+ tools jar}"
jar=$(realpath "$TLA2TOOLS_JAR")
cd "$(dirname "$0")"
work=$(mktemp -d "${TMPDIR:-/tmp}/unreal-operations-tlc.XXXXXX")
trap 'rm -rf "$work"' EXIT
run_tlc() {
  java -Xmx512m -XX:+UseParallelGC -cp "$jar" tlc2.TLC -workers 1 -seed 1 \
    -metadir "$work/states-$1" -noGenerateSpecTE -config "$1.cfg" \
    OperationCheckpoint.tla > "$work/$1.log" 2>&1
}
for pair in 'legacy-spawn AtMostOnceSpawn' 'legacy-close SingleTerminalCheckpoint'; do
  read -r name invariant <<< "$pair"
  rc=0
  run_tlc "$name" || rc=$?
  # TLC exit 12 is an invariant violation, not a parser/tool failure.
  if [[ "$rc" != 12 ]] || ! grep -q "Invariant $invariant is violated" "$work/$name.log"; then
    cat "$work/$name.log"
    echo "Expected $name counterexample for $invariant; exit=$rc" >&2
    exit 1
  fi
  echo "Confirmed $name counterexample: $invariant"
done
if ! run_tlc fixed || ! grep -q 'Model checking completed. No error has been found.' "$work/fixed.log"; then
  cat "$work/fixed.log"
  exit 1
fi
grep -o '[0-9]* distinct states found' "$work/fixed.log"
echo 'Both fixes hold in the model.'
