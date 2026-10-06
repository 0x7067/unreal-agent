#!/usr/bin/env bash
# Regenerate patches/ from the fork tree.
#
# Invariant enforced by script/verify-upstream.sh:
#   apply(script/upstream-pin, patches/*.patch) == FORK_REF (ignoring patches/)
#
# The series is ordered and split into buckets so that a rebase conflict is
# isolated to the bucket that owns the file. Add changed paths to a bucket here
# when you add new work; the script refuses to run if any changed path is not
# assigned to a bucket.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

FORK_REF=${FORK_REF:-main}

die() { printf 'refresh-patches: %s\n' "$*" >&2; exit 1; }

pin=$(tr -d '[:space:]' < script/upstream-pin)
[ -n "$pin" ] || die "script/upstream-pin is empty"
git cat-file -e "${pin}^{commit}" 2>/dev/null ||
	die "pinned commit $pin is not in this repository; run: git fetch upstream --tags"
git rev-parse --verify "${FORK_REF}^{commit}" >/dev/null 2>&1 ||
	die "fork ref $FORK_REF does not exist"

if ! git diff --quiet || ! git diff --cached --quiet; then
	die "working tree has staged or unstaged changes; commit them first"
fi

# Ordered buckets. Paths must be disjoint. patches/ is deliberately absent:
# the series cannot contain itself.
bucket_names=(
	0001-formal-models
	0002-review-product
	0003-tui
	0004-build-integration
	0005-agent-skills
	0006-fork-maintenance
)
bucket_paths=(
	"tools/formal LAWS.bend PROOF.bend"
	"cmd/internal/review cmd/internal/reviewcli review cmd/internal/providers/selection.go cmd/internal/providers/selection_test.go tools/review-canary .claude/skills/verify-unreal-review"
	"cmd/unreal-agent-tui"
	"Makefile .goreleaser.yaml .github .golangci.yml .gitattributes .dockerignore Dockerfile .gitignore cmd/unreal-agent-runner"
	".claude/skills/verify-unreal-agent-tui"
	"script"
)

message_for() {
	case "$1" in
	0001-formal-models)
		printf '%s\n\n%s\n' \
			"formal: add Bend models, source-linked contracts, and proof tooling" \
			"Adds the Bend/TLA/Lean model tree together with the Python drivers that gate the restricted Go predicates and generated API enums. Additive: no upstream file is touched."
		;;
	0002-review-product)
		printf '%s\n\n%s\n' \
			"review: add the automated code review pipeline" \
			"Adds the review command (diff acquisition, planning, focused/planned strategies, findings, GitHub rendering) plus its offline and live canaries. Depends only on upstream public packages plus the fork's own provider selection."
		;;
	0003-tui)
		printf '%s\n\n%s\n' \
			"tui: add /review integration and presentation improvements" \
			"Declared soft-fork: modifies ten upstream TUI files (command picker, review command, tool details, scroll coalescing, terminal-text sanitizers). Conflicts here are expected on upstream sync; git rerere is recommended."
		;;
	0004-build-integration)
		printf '%s\n\n%s\n' \
			"build: integrate the review command and fork release" \
			"Seven lines in the runner entry point dispatch the review subcommand, plus the fork's Makefile targets, release configuration, CI jobs, Dockerfile and lint config."
		;;
	0005-agent-skills)
		printf '%s\n\n%s\n' \
			"agent skills: add TUI verification scripts" \
			"Adds the agent-facing scripts used to drive and inspect the TUI during verification."
		;;
	0006-fork-maintenance)
		printf '%s\n\n%s\n' \
			"maintenance: add upstream patch series tooling" \
			"Pins the upstream base and adds the scripts that refresh, verify and rebase the fork delta. Applying the series reproduces the fork tree; CI enforces that invariant."
		;;
	*) die "no commit message for bucket $1" ;;
	esac
}

# Refuse to silently drop changed paths that belong to no bucket.
all_paths=$(git diff --name-only "${pin}..${FORK_REF}" | LC_ALL=C sort -u | grep -v '^patches/' || true)
bucketed=$(
	for paths in "${bucket_paths[@]}"; do
		# shellcheck disable=SC2086 # intentional path splitting
		git diff --name-only "${pin}..${FORK_REF}" -- $paths
	done | LC_ALL=C sort -u
)
leftover=$(LC_ALL=C comm -23 <(printf '%s\n' "$all_paths") <(printf '%s\n' "$bucketed") | sed '/^$/d')
if [ -n "$leftover" ]; then
	printf 'refresh-patches: these changed paths are not assigned to any bucket:\n%s\n' "$leftover" >&2
	die "add them to bucket_paths in script/refresh-patches.sh and document them in patches/README.md"
fi

tmp=$(mktemp -d)
cleanup() {
	git worktree remove --force "$tmp/tree" >/dev/null 2>&1 || true
	rm -rf "$tmp"
}
trap cleanup EXIT
git worktree prune
git worktree add --detach "$tmp/tree" "$pin" >/dev/null

# Fixed to the pinned base so repeated refreshes are byte-stable.
date=$(git show -s --format=%cI "$pin")
mkdir -p "$root/patches"
rm -f "$root/patches"/*.patch

(
	cd "$tmp/tree"
	for i in "${!bucket_names[@]}"; do
		name=${bucket_names[$i]}
		paths=${bucket_paths[$i]}

		# Files the fork deleted relative to upstream.
		while IFS= read -r f; do
			[ -n "$f" ] && git rm -q -f --ignore-unmatch -- "$f"
		done < <(git -C "$root" diff --name-only --diff-filter=D "${pin}..${FORK_REF}" -- $paths)

		if [ -z "$(git -C "$root" ls-tree -r --name-only "$FORK_REF" -- $paths)" ]; then
			die "bucket $name matches no files at $FORK_REF"
		fi
		# shellcheck disable=SC2086 # intentional path splitting
		git checkout "$FORK_REF" -- $paths
		git add -A
		if git diff --cached --quiet; then
			printf 'refresh-patches: bucket %s is empty, skipping\n' "$name" >&2
			continue
		fi
		message_for "$name" >"$tmp/message"
		GIT_AUTHOR_DATE=$date GIT_COMMITTER_DATE=$date \
			git commit -q --no-verify -F "$tmp/message"
		git format-patch -1 HEAD --stdout --no-signature >"$root/patches/$name.patch"
	done
)

printf 'refresh-patches: wrote %s patch file(s) from %s\n' "$(ls -1 "$root/patches"/*.patch 2>/dev/null | wc -l | tr -d ' ')" "$FORK_REF"
