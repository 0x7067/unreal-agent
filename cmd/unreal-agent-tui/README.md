# Unreal Agent

A minimal terminal client for Harness. Run it from your workspace:

```sh
go -C unreal-agent install ./cmd/unreal-agent
unreal-agent -provider openai-codex -model gpt-5.6-luna
unreal-agent -provider ollama -model qwen3.8:27b
unreal-agent -provider openai -model MODEL -theme catppuccin-dark
unreal-agent -help
```

Provider, model, and effort are set at startup. See `-help` for flags and
[theme examples](themes/).

Sessions are stored in `$XDG_STATE_HOME/unreal-agent/sessions`, falling back to
`~/.local/state/unreal-agent/sessions`. Each launch starts a new session.

Workspace skills are loaded from `.harness/skills/*/SKILL.md`.

The former TUI, setup, saved configuration, resume, and client MCP integration
have been removed. Configured MCP servers are not loaded; harness MCP support is
unchanged. Existing session and configuration files are left untouched.

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

## Development

```sh
go -C unreal-agent test ./...
go -C unreal-agent build -o /tmp/unreal-agent ./cmd/unreal-agent
```
