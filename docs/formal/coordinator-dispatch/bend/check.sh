#!/usr/bin/env bash
# Checked artifact freshness, complete public proof, and genuine proof mutants.
# This local regression helper does not replace the root authoritative runner.
set -euo pipefail
here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
bend=${BEND:-"$HOME/.bend/bin/bend"}
python3 "$here/generate_certificate.py" --check
"$bend" "$here/PROOF.bend" --check-only
work=$(mktemp -d "${TMPDIR:-/tmp}/coordinator-dispatch-bend.XXXXXX")
trap 'rm -rf -- "$work"' EXIT
cp "$here/"*.bend "$work/"
for mutant in commit terminal overlay; do
  cp "$here/product.bend" "$work/product.bend"
  # Python replacement is portable across Linux/macOS (no GNU sed -i syntax).
  python3 - "$work/product.bend" "$mutant" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
configs = {
    'commit': 'True{}, False{}, False{}',
    'terminal': 'False{}, True{}, False{}',
    'overlay': 'False{}, False{}, True{}',
}
old = 'def fixed() -> Config: Config{False{}, False{}, False{}}'
new = 'def fixed() -> Config: Config{' + configs[sys.argv[2]] + '}'
text = path.read_text()
assert text.count(old) == 1, 'default config anchor changed'
path.write_text(text.replace(old, new))
PY
  if "$bend" "$work/PROOF.bend" --check-only > "$work/$mutant.log" 2>&1; then
    cat "$work/$mutant.log"
    echo "Unexpected proof acceptance of $mutant mutant" >&2
    exit 1
  fi
  # A syntax/import/tool error is NOT a successful negative proof test.
  if ! grep -q 'Location: closure.edgeTick' "$work/$mutant.log"; then
    cat "$work/$mutant.log"
    echo "Unexpected failure while checking $mutant mutant" >&2
    exit 1
  fi
  echo "Confirmed $mutant mutant rejected by independent-field transition closure"
done
