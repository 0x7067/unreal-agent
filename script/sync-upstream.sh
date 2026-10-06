#!/usr/bin/env bash
# Rebase the fork onto a newer upstream and refresh the patch series.
#
#   script/sync-upstream.sh v0.5.0    # fetch, rebase, update the pin, refresh
#   script/sync-upstream.sh --continue  # finish after resolving conflicts
#
# The pre-rebase tip is tagged so the rebase can be reviewed with git range-diff.
# Conflicts are expected only in the bucket that owns the file; git rerere
# replays previous resolutions of the same hunks.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

FORK_REF=${FORK_REF:-main}
UPSTREAM_REMOTE=${UPSTREAM_REMOTE:-upstream}

die() { printf 'sync-upstream: %s\n' "$*" >&2; exit 1; }
usage() {
	cat >&2 <<'EOF'
usage: script/sync-upstream.sh <new-upstream-ref>
       script/sync-upstream.sh --continue
       script/sync-upstream.sh --help

  <new-upstream-ref>  commit/tag to rebase onto (e.g. v0.5.0, upstream/main)
  --continue          finish an interrupted sync after resolving conflicts

environment:
  FORK_REF         fork branch to rebase            (default: main)
  UPSTREAM_REMOTE  git remote for upstream          (default: upstream)
  NO_FETCH=1       skip git fetch
EOF
}

write_pin() { printf '%s\n' "$1" >script/upstream-pin; }

finish() {
	local new=$1
	# The pin is tracked, so commit the bump before regenerating the series:
	# refresh-patches refuses to run over a dirty tree.
	write_pin "$new"
	git add script/upstream-pin
	git commit -q --no-verify -m "upstream-pin: advance to $new"
	script/refresh-patches.sh
	if [ -n "$(git status --porcelain -- patches)" ]; then
		git add patches
		git commit -q --no-verify -m "Refresh patch series"
	fi
	script/verify-upstream.sh
	printf '\nsync-upstream: rebased onto %s and refreshed patches\n' "$new"
	printf 'review the replay with:\n  git range-diff %s..%s %s..%s\n' \
		"$old_base" "$old_tip" "$new" "$FORK_REF"
}

old_base=$(tr -d '[:space:]' < script/upstream-pin)
old_tip=$(git rev-parse "$FORK_REF")
marker=.git/ua-sync-target
marker_base=.git/ua-sync-base

case "${1:-}" in
"" | -h | --help)
	usage
	exit 0
	;;
--continue)
	[ -d .git/rebase-merge ] || [ -d .git/rebase-apply ] ||
		die "no rebase in progress; nothing to continue"
	[ -f "$marker" ] && [ -f "$marker_base" ] ||
		die "no interrupted sync recorded; finish the rebase manually, then run script/refresh-patches.sh"
	new=$(cat "$marker")
	old_base=$(cat "$marker_base")
	old_tip=$(git tag --list 'sync/pre-*' --sort=-creatordate | head -1)
	[ -n "$old_tip" ] || die "no sync/pre-* tag found"
	GIT_EDITOR=true git rebase --continue || die "rebase still has unresolved conflicts"
	rm -f "$marker" "$marker_base"
	finish "$new"
	exit 0
	;;
-*)
	usage
	exit 2
	;;
esac

requested=$1
if ! git diff --quiet || ! git diff --cached --quiet; then
	die "working tree has staged or unstaged changes; commit or stash them first"
fi

if [ "${NO_FETCH:-0}" != 1 ]; then
	git fetch "$UPSTREAM_REMOTE" --tags
fi

new=$(git rev-parse --verify "${requested}^{commit}") || die "cannot resolve $requested"
[ "$new" != "$old_base" ] || die "already pinned to $new"
git merge-base --is-ancestor "$old_base" "$FORK_REF" ||
	die "$FORK_REF does not descend from the pinned base $old_base; reconcile the history manually before syncing"

tag="sync/pre-$(date -u +%Y%m%d-%H%M%S)"
git tag "$tag" "$old_tip"
printf '%s\n' "$new" >"$marker"
printf '%s\n' "$old_base" >"$marker_base"

if ! git rebase --onto "$new" "$old_base" "$FORK_REF"; then
	cat >&2 <<EOF

sync-upstream: rebase stopped on conflicts.
  1. resolve them (git status), then: git add <files>
  2. finish with: script/sync-upstream.sh --continue
  pre-sync tip is tagged $tag for git range-diff.
EOF
	exit 1
fi
rm -f "$marker" "$marker_base"
finish "$new"
