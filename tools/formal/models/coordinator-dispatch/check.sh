#!/usr/bin/env bash
# No downloads. Only the installed Java/TLC and ordinary shell utilities run.
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
root=$(cd -- "$here/../../../.." && pwd)
expected=d15655eeaff4b3f7b3d65c46b838bc0f35d131b2ef4b1e473689b62c98d4726f
jar=${TLA2TOOLS_JAR:-$root/.tools/tlaplus/tla2tools.jar}
jar=$(realpath "$jar")
actual=$(sha256sum "$jar" | awk '{print $1}')
if [[ "$actual" != "$expected" ]]; then
  echo "Refusing unpinned TLC jar: $jar (sha256 $actual; expected $expected)" >&2
  exit 1
fi
work=$(mktemp -d "${TMPDIR:-/tmp}/coordinator-dispatch-tlc.XXXXXX")
cleanup() {
  if [[ "${KEEP_TLC_LOGS:-0}" == 1 ]]; then
    echo "TLC logs and state directories: $work"
  else
    rm -rf -- "$work"
  fi
}
trap cleanup EXIT
cd "$here"
echo "TLC jar: $jar"
echo "SHA-256: $actual"
run() {
  java -Xmx256m -XX:+UseParallelGC -cp "$jar" tlc2.TLC \
    -workers 1 -seed 1 -noGenerateSpecTE -metadir "$work/$1-states" \
    -config "$1.cfg" CoordinatorDispatch.tla > "$work/$1.log" 2>&1
}
if ! run fixed; then cat "$work/fixed.log"; exit 1; fi
grep -m1 'TLC2 Version' "$work/fixed.log"
grep -q 'Model checking completed. No error has been found.' "$work/fixed.log"
echo 'fixed: PASS'
grep -E 'states generated|depth of the complete|Finished in' "$work/fixed.log"
for pair in 'skip-status-commit CommitBeforeAdd' \
            'skip-terminal-guard NoTerminalOrStaleAdd' \
            'skip-latest-overlay NoTerminalOrStaleAdd'; do
  read -r config invariant <<< "$pair"
  rc=0
  run "$config" || rc=$?
  if [[ "$rc" != 12 ]] || ! grep -q "Invariant $invariant is violated" "$work/$config.log"; then
    cat "$work/$config.log"
    echo "Expected exit 12 / $invariant from $config; got $rc" >&2
    exit 1
  fi
  echo "$config: expected $invariant violation (exit 12)"
  # Concise reproducible action trace; full variable dumps retained on request.
  grep -E '^State [0-9]+:|states generated|depth of the complete|Finished in' "$work/$config.log"
done
echo 'All bounded safety checks and three expected counterexamples verified.'
