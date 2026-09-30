# Unreal Agent Lite

A minimal terminal client for the existing harness, alongside `unreal-agent`.

```sh
go -C unreal-agent build -o /tmp/unreal-agent-lite ./cmd/unreal-agent-lite
cd /path/to/workspace
/tmp/unreal-agent-lite -provider openai-codex -model gpt-5.6-luna
/tmp/unreal-agent-lite -provider ollama -model qwen3.8:27b
/tmp/unreal-agent-lite -help
```
