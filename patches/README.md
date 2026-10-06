# Fork patch series

This directory is the fork's delta over upstream, expressed as an ordered series of
patches that can be applied to a **pristine** upstream checkout. It exists so that
syncing with upstream is a rebase of a small, known surface instead of a
reconciliation of a diverged tree.

## The invariant

For the pinned base in [`script/upstream-pin`](../script/upstream-pin) and the fork
branch (`main`):

```
apply(upstream-pin, patches/*.patch)  ==  main   (ignoring patches/ itself)
```

`script/verify-upstream.sh` enforces this: it exports the pinned commit, applies every
patch in filename order, and then compares the file modes and blob hashes against the
fork branch. It also builds both Go modules and, by default, runs `go vet` and the test
suites. CI runs the same script as the `upstream-intact` job, so a fork commit that
forgets to refresh the series — or an upstream change that stops the series applying —
fails loudly instead of drifting.

`patches/` is the one path excluded from the comparison: a series cannot contain itself.

## The series

| Patch | Contents | Why it is separate |
| --- | --- | --- |
| `0001-formal-models.patch` | `tools/formal/**`, root `LAWS.bend`, `PROOF.bend` | Additive, no Go imports; largest and most independent part of the delta |
| `0002-review-product.patch` | `cmd/internal/review/**`, `cmd/internal/reviewcli/**`, `review/**`, provider selection, canaries, docs | Additive; imports only upstream public packages |
| `0003-tui.patch` | `cmd/unreal-agent-tui/**` | The declared soft-fork: modifies ten upstream TUI files |
| `0004-build-integration.patch` | runner `main.go` dispatch (7 lines), Makefile, goreleaser, CI, Dockerfile, lint config | Small integration surface |
| `0005-agent-skills.patch` | `.claude/skills/verify-unreal-agent-tui/**` | Verification scripts for the TUI |
| `0006-fork-maintenance.patch` | `script/**` | This tooling |

Order matters only where code depends on code: review and formal apply before the TUI
and the build integration that import them.

## Workflow

Day to day, work on `main` as usual. When the fork changes, regenerate and commit the
series:

```sh
script/refresh-patches.sh
script/verify-upstream.sh
git add patches && git commit -m "Refresh patch series"
```

`refresh-patches.sh` derives the series from the current fork tree, one commit per
bucket, and writes `patches/NNNN-*.patch` with `git format-patch` metadata. It refuses to
run with a dirty tree and **refuses to drop work**: any changed path that no bucket
claims is an error, with the list of unassigned paths. Assign new paths in
`bucket_paths` inside the script and describe the bucket here.

## Syncing with upstream

```sh
script/sync-upstream.sh v0.5.0     # fetch, rebase main, update the pin, refresh, verify
```

If the rebase stops on conflicts:

```sh
# resolve, git add ...
script/sync-upstream.sh --continue
```

The pre-sync tip is tagged `sync/pre-*`; review what upstream changed in your patches
with:

```sh
git range-diff <old-base>..<old-tip> <new-base>..main
```

Conflicts are expected in `0003-tui.patch` only. `git rerere` is worth enabling
(`git config rerere.enabled true`) so repeated TUI hunks resolve themselves; always
review the replay and run the tests afterwards, because rerere replays resolutions
blindly.

Pin upstream **release tags**, not `main`: upstream is active (on the order of a hundred
commits a month), and each sync is a rebase of the fork's six buckets.

## Applying the series without the fork history

```sh
git archive <pinned-commit> | tar -x -C /tmp/upstream
cd /tmp/upstream && git init && git apply /path/to/patches/*.patch
```

The result is the fork tree minus `patches/`, which is enough to build and test the
agent against a vanilla upstream checkout.

## Why not a Go plugin

Go has no source-level plugin mechanism. `-buildmode=plugin` loads shared objects at
runtime with an identical toolchain and identical dependency versions, cannot add a CLI
subcommand or TUI behaviour, and does not help with a fork that must share upstream
source. A patch series is the smallest mechanism that keeps upstream untouched while
remaining reviewable and testable; the alternative — a separate Go module importing the
upstream `harness/*` packages — is possible for the review product but is blocked today
by upstream's `internal/` provider packages and would not cover the TUI or the runner
dispatch.
