# Release notes

Fork release notes. Tags and publish rules are in [RELEASING.md](RELEASING.md).
The GitHub release page for the next fork tag starts with this note
(`.goreleaser.yaml` `release.header`):

The release archive includes `unreal-review` next to `unreal-agent-runner` and `unreal-agent-tui`.
`unreal-agent-runner review` runs that binary and returns its exit code.

Review uses the runner's provider selection. An empty `UNREAL_HARNESS_LLM_PROVIDER` defaults to `openai`.
If you only have `OPENROUTER_API_KEY`, set `UNREAL_HARNESS_LLM_PROVIDER=openrouter`.

## unreal-review ships with the fork release

The existing per-platform archive keeps its name,
`unreal-agent_<version>_<os>_<arch>.tar.gz`, where `<version>` is the tag
with one leading `v` removed. That archive now also contains `unreal-review`
beside `unreal-agent-runner` and `unreal-agent-tui`. There is no separate
review archive. Checksums stay in `SHA256SUMS`. A published tag is
downloaded from
`https://github.com/0x7067/unreal-agent/releases/download/<tag>/unreal-agent_<version>_<os>_<arch>.tar.gz`.
The Homebrew cask installs `unreal-review` as well. `skip_upload` is still
set, so the cask is written under `dist/homebrew` and is not pushed to
`unreallabsai/homebrew-tap`.

`unreal-agent-runner review` runs `unreal-review` with the remaining
arguments and returns that process's exit code. The runner looks for
`unreal-review` in the same directory as itself, then on `PATH`. If neither
is present it exits 1 and says `unreal-review not found next to unreal-agent-runner or on PATH`.
`unreal-agent-runner -h` names the alias. `unreal-agent-runner review help`
prints unreal-review's own usage.

## Provider default

Review resolves the model provider with `providers.Select`, the same function
the runner uses. An empty `UNREAL_HARNESS_LLM_PROVIDER` defaults to `openai`.
Users who only have `OPENROUTER_API_KEY` must set
`UNREAL_HARNESS_LLM_PROVIDER=openrouter`.

A missing credential is reported as `UNREAL_HARNESS_LLM_API_KEY or <provider variable> must be set`.
For the default provider that string is `UNREAL_HARNESS_LLM_API_KEY or OPENAI_API_KEY must be set`.
When the selected provider is openrouter it is `UNREAL_HARNESS_LLM_API_KEY or OPENROUTER_API_KEY must be set`.
The previous missing-key text, which told every run to set `OPENROUTER_API_KEY`,
is gone. `ollama` needs no API key. `openai-codex` does not use that
missing-key line; it reports its own credential errors.
