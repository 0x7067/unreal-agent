#!/bin/sh
# Review-only compatibility gate. The authoritative whole-project gate is make prove.
# No unsafe/foreign exceptions are permitted, even if Bend exits zero.
set -u
export BEND_NO_TELEMETRY=1
cd "$(dirname "$0")" || exit 1

bend_bin="${BEND:-bend}"
if ! command -v "$bend_bin" >/dev/null 2>&1 && [ -x "$HOME/.bend/bin/bend" ]; then
  bend_bin="$HOME/.bend/bin/bend"
fi

out="$("$bend_bin" PROOF.bend --check-only 2>&1)"
status=$?
printf '%s\n' "$out"
[ "$status" -eq 0 ] || exit 1
[ "$out" = "All terms check." ] || {
  echo "prove: bend did not report an untainted checked proof" >&2
  exit 1
}

allowed="$(sed -e 's/#.*//' -e '/^[[:space:]]*$/d' spec/unsafe-allow.txt)"
[ -z "$allowed" ] || {
  echo "prove: unsafe/foreign exceptions are prohibited" >&2
  exit 1
}
echo "prove: ok (review models only; use make prove for all indexed laws)"
