#!/usr/bin/env bash
# Verify that the committed patch series applies to the pinned upstream base and
# that the fork still builds. Two independent checks:
#
#   1. every patch applies cleanly to a pristine checkout of script/upstream-pin;
#   2. the applied tree is byte-identical to the fork ref (modes and blobs),
#      except for patches/ itself, which cannot be part of the series.
#
# Environment:
#   VERIFY_REF       fork ref to compare against          (default: HEAD)
#   VERIFY_IDENTITY  1 to check byte-identity             (default: 1)
#   VERIFY_VET       1 to run go vet                      (default: 1)
#   VERIFY_TESTS     1 to run the full test suites        (default: 1)
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

VERIFY_REF=${VERIFY_REF:-HEAD}
VERIFY_IDENTITY=${VERIFY_IDENTITY:-1}
VERIFY_VET=${VERIFY_VET:-1}
VERIFY_TESTS=${VERIFY_TESTS:-1}

die() { printf 'verify-upstream: %s\n' "$*" >&2; exit 1; }

pin=$(tr -d '[:space:]' < script/upstream-pin)
[ -n "$pin" ] || die "script/upstream-pin is empty"
git cat-file -e "${pin}^{commit}" 2>/dev/null ||
	die "pinned commit $pin is not in this repository; run: git fetch upstream --tags"
verify_ref=$(git rev-parse --verify "${VERIFY_REF}^{commit}") ||
	die "fork ref $VERIFY_REF does not exist"

shopt -s nullglob
patches=("$root"/patches/*.patch)
shopt -u nullglob
[ "${#patches[@]}" -gt 0 ] || die "no patches in patches/; run script/refresh-patches.sh"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

printf 'verify-upstream: base %s, %s patch(es)\n' "$pin" "${#patches[@]}"
git archive "$pin" | tar -x -C "$tmp"
cd "$tmp"
git init -q .
git config user.name "unreal-agent verify"
git config user.email "verify@invalid"
git add -A
git commit -q --no-verify -m "pinned upstream $pin"

for patch in "${patches[@]}"; do
	git apply --check "$patch" ||
		die "$(basename "$patch") does not apply to $pin; rebase the fork (script/sync-upstream.sh)"
	git apply "$patch"
done
git add -A
git commit -q --no-verify -m "fork patches"
printf 'verify-upstream: patches applied\n'

if [ "$VERIFY_IDENTITY" = 1 ] &&
	git -C "$root" merge-base --is-ancestor "$pin" "$verify_ref" 2>/dev/null; then
	git -C "$root" ls-tree -r --format='%(objectmode) %(objectname) %(path)' "$verify_ref" |
		LC_ALL=C sort -k3 | grep -v '^[0-9]\+ [0-9a-f]\+ patches/' >"$tmp/fork.tree" || true
	git ls-tree -r --format='%(objectmode) %(objectname) %(path)' HEAD |
		LC_ALL=C sort -k3 >"$tmp/applied.tree"
	if ! diff -u "$tmp/fork.tree" "$tmp/applied.tree" >"$tmp/tree.diff"; then
		head -50 "$tmp/tree.diff" >&2
		die "patches do not reproduce $VERIFY_REF ($verify_ref); run script/refresh-patches.sh"
	fi
	printf 'verify-upstream: applied tree matches %s\n' "$VERIFY_REF"
fi

go build ./... || die "root module does not build after patching"
go -C cmd/unreal-agent-tui build ./... || die "TUI module does not build after patching"
printf 'verify-upstream: both modules build\n'

if [ "$VERIFY_VET" = 1 ]; then
	go vet ./... || die "go vet failed in the patched tree"
	go -C cmd/unreal-agent-tui vet ./... || die "go vet failed in the patched TUI module"
	printf 'verify-upstream: go vet clean\n'
fi

if [ "$VERIFY_TESTS" = 1 ]; then
	go test -count=1 ./... || die "root tests failed in the patched tree"
	go -C cmd/unreal-agent-tui test -count=1 ./... || die "TUI tests failed in the patched tree"
	printf 'verify-upstream: tests pass\n'
fi

printf 'verify-upstream: ok\n'
