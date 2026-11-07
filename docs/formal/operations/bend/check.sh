#!/usr/bin/env bash
# Local product checks. The project root proof runner remains authoritative.
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
bend=${BEND:-"$HOME/.bend/bin/bend"}
python3 "$here/generate_certificate.py" --check
"$bend" "$here/PROOF.bend" --check-only
work=$(mktemp -d "${TMPDIR:-/tmp}/operations-bend.XXXXXX")
trap 'rm -rf "$work"' EXIT
cp "$here/"*.bend "$work/"
for mutant in spawn close; do
  cp "$here/product.bend" "$work/product.bend"
  if [[ "$mutant" == spawn ]]; then
    python3 - "$work/product.bend" <<'PYEDIT'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
old = 'def fixed() -> Config: Config{True{}, True{}}'
assert s.count(old) == 1
p.write_text(s.replace(old, 'def fixed() -> Config: Config{False{}, True{}}'))
PYEDIT
    expected=closure.edgeCreateErrDone
  else
    python3 - "$work/product.bend" <<'PYEDIT'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
old = 'def fixed() -> Config: Config{True{}, True{}}'
assert s.count(old) == 1
p.write_text(s.replace(old, 'def fixed() -> Config: Config{True{}, False{}}'))
PYEDIT
    expected=closure.edgeHandlerCloses
  fi
  if "$bend" "$work/PROOF.bend" --check-only > "$work/$mutant.log" 2>&1; then
    cat "$work/$mutant.log"
    echo "Unexpected proof acceptance of $mutant mutant" >&2
    exit 1
  fi
  # Require the expected failed transition equality, not any parser/tool error.
  if ! grep -q "Location: $expected" "$work/$mutant.log"; then
    cat "$work/$mutant.log"
    echo "Unexpected failure while checking $mutant mutant" >&2
    exit 1
  fi
  echo "Confirmed $mutant mutant rejected by full-product closure"
done
