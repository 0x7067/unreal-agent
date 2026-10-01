# Unreal Agent

A minimal terminal client for Harness. Run it from your workspace:

```sh
go -C unreal-agent install ./cmd/unreal-agent
unreal-agent -provider openai-codex -model gpt-5.6-luna
unreal-agent -provider ollama -model qwen3.8:27b
unreal-agent -provider openai -model MODEL -theme catppuccin-dark
unreal-agent -help
```

[Theme examples](themes/).

Workspace skills are loaded from `.harness/skills/*/SKILL.md`.

## ChatGPT subscription

Sign in with Codex using file credentials:

```sh
codex -c 'cli_auth_credentials_store="file"' login
unreal-agent -provider openai-codex -model gpt-5.6-luna
```

Credentials come from `$CODEX_HOME/auth.json` or `~/.codex/auth.json`;
override with `OPENAI_CODEX_AUTH_FILE`. Alternatively use
`OPENAI_CODEX_ACCESS_TOKEN` and `OPENAI_CODEX_ACCOUNT_ID`.
Renew credentials with Codex, then restart the agent.
