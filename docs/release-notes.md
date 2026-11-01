# Release notes

Fork release notes. Tags and publish rules are in [RELEASING.md](RELEASING.md).
The GitHub release page for the next fork tag starts with this note
(`.goreleaser.yaml` `release.header`):

`unreal-agent-runner review` is the review command. The separate `unreal-review` binary is gone.

Review uses the runner's provider selection. An empty `UNREAL_HARNESS_LLM_PROVIDER` defaults to `openai`.
If you only have `OPENROUTER_API_KEY`, set `UNREAL_HARNESS_LLM_PROVIDER=openrouter`.

## Review is a runner command

The review code now runs inside `unreal-agent-runner`. The release archive and
the Homebrew cask no longer contain `unreal-review`. Replace `unreal-review <args>`
with `unreal-agent-runner review <args>`; the arguments, findings.jsonl and
exit codes are unchanged. Error lines now start with
`unreal-agent-runner review:` instead of `unreal-review:`. The GitHub check run
and comment markers keep the `unreal-review` name, so `run --pr` continues
from reviews posted by earlier releases.

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
