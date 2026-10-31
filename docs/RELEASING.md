# Fork releases

This repository is the `0x7067/unreal-agent` fork. Release configuration
intentionally diverges from `unreallabsai/unreal-agent`. The Go module path
stays `github.com/unreallabsai/unreal-agent`.

## Tags

Fork releases use `vX.Y.Z-fork.N` (`N` is a positive integer, no leading
zeros). The first one will be `v0.4.1-fork.1`.

`.github/workflows/release.yml` runs only on tags matching `v*-fork.*`, then
rejects anything that is not `vX.Y.Z-fork.N`. A plain `vX.Y.Z` tag does not
start the workflow, so it cannot collide with an upstream release of the same
name.

The tag must be an ancestor of `origin/main`. Do not push a tag until that
commit is on `main`.

`v0.4.0` is already an annotated tag on `bbfde0d` and has no GitHub release.
`v0.4.0-fork.1` would sort below that tag in semver, so the fork series starts
at `v0.4.1-fork.1`. Goreleaser's changelog uses the nearest ancestor tag
(`git describe`). A `v0.4.1-fork.1` on later `main` therefore starts its range
at `v0.4.0`. The release name is the full tag.

## GitHub release

`.goreleaser.yaml` publishes to `0x7067/unreal-agent`.

The TUI build injects `main.version` from `{{ .Version }}`, which is the tag
with one leading `v` removed (`v0.4.1-fork.1` becomes `0.4.1-fork.1`).
`{{ .RawVersion }}` is only `X.Y.Z` and would drop the suffix. The runner
binary has no version ldflags. `unreal-review` is built the same way as the
runner (linux and darwin, amd64 and arm64, `-mod=readonly`, `-trimpath`,
`-s -w`) and is packed into the same archives and checksums. The Homebrew
cask lists it in `binaries` and documents the `unreal-agent-runner review`
alias, the openai provider default, and the missing-key message in
`caveats`. `skip_upload` and the tap repository block stay as they are.
User-facing notes: [release notes](release-notes.md).

Snapshot builds replace `.Version` via
`{{ incpatch .Version }}-snapshot`. `incpatch` on a `-fork.N` prerelease
yields the plain `X.Y.Z` (so `v0.4.1-fork.1` snapshots as `0.4.1-snapshot`);
that path is local only and is not a published release.

The release workflow's job runs only when `github.repository` is
`0x7067/unreal-agent`.

## Not published

Docker images stay in the config and are disabled (`dockers_v2.disable`).
They still name `unrea1labs/unreal-agent`. The workflow's Docker login,
QEMU, and buildx steps are `if: false` because this fork has no
`DOCKERHUB_USERNAME` or `DOCKERHUB_TOKEN`.

Homebrew casks have no `disable` field. `skip_upload: true` still writes
`dist/homebrew/Casks/unreal-agent.rb` (used by `make install-local`) and does
not push `unreallabsai/homebrew-tap`, which would need
`HOMEBREW_TAP_DEPLOY_KEY`. The cask repository block still names that tap;
do not clear `skip_upload` without pointing it at a fork-owned tap.

`make install-local` rewrites cask download URLs from
`https://github.com/0x7067/unreal-agent/releases/download/...` to the local
`dist/` archives.
