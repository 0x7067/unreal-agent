# Project agent memory

unreal-agent: this file is the always-loaded memory for agents working in this repo.
It is kept short on purpose - every line here is paid on every session.

## Learnings

- User preference: after code changes, always rebuild and reinstall before finishing. In this environment, use `make build` and atomically replace both binaries in `~/.local/bin`; UI changes require a frontend restart.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
