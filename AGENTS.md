# Project agent memory

unreal-agent: this file is the always-loaded memory for agents working in this repo.
It is kept short on purpose - every line here is paid on every session.

## Commands

Two Go modules: the root (library, runner, `cmd/internal`) and `cmd/unreal-agent-tui` (own `go.mod`; use `go -C cmd/unreal-agent-tui ...`). `make test check build` is what CI runs.

- `make build` -> `bin/unreal-agent-runner`, `bin/unreal-agent-tui`, and fork-only `bin/unreal-review`
- `make test` (`-race`, both modules, including `cmd/internal/review`, `cmd/unreal-review`, and `review/findings`); `make check` (vet + gofmt on `cmd harness internal review`)
- Review (fork-only, not upstream): `make review-canary` (loopback stub, no paid calls), `make review-prove` (Bend, `docs/formal/review/prove.sh`), `make review-lint` (golangci-lint on the review packages). `make review-canary-live` calls OpenRouter and is not a CI gate.
- Single test: `go test ./harness/coordinator -run TestName -count=1`; in the TUI module: `go -C cmd/unreal-agent-tui test -run TestName .`
- Fuzz targets CI runs are listed in `.github/workflows/ci.yml` (e.g. `go test ./harness/coordinator -run '^$' -fuzz '^FuzzCoordinatorFaults$' -fuzztime=30s`)
- Wire types in `internal/openaiapi` and `internal/anthropicapi` are generated from `third_party/*-openapi` via `go generate` (`uv run --script generate.py`); don't hand-edit them.
- TLA+ checkpoint specs: `docs/formal/taskgraph/check.sh` (needs `TLA2TOOLS_JAR`).
- Harbor benchmark: `make -C benchmarks/harbor test check build smoke BUNDLE=../../bin/harbor/ci`.

Project skills to use when relevant: `taskgraph-reducer-safety` (edits to `harness/taskgraph/graph.go`), `inbox-admission-safety` (edits to `harness/inbox`), `verify-unreal-agent-tui` (prove TUI changes in tmux against a fake model).

## Architecture

The README glossary and components table define the core terms (Input, Inbox, Session, Operation, tool translator). How they connect:

- Inputs enter through the `harness/inbox` actor (dedup by input ID, FIFO, Freeze/Thaw for handoff). `harness/ingress` journals them durably.
- `harness/coordinator` owns the single event loop: it persists accepted inputs to `harness/sessionstore` (append-only, versioned, forkable), builds model input with `harness/contextbuilder` (in-memory, no I/O; also compaction), calls an `harness/llm` adapter (`responsesapi`, `messagesapi`; provider clients under `llm/clients`), and runs tool translators from `harness/tool`.
- Translators run synchronously on the loop and must not do I/O. They only produce serializable `harness/operation` values. The session store records them atomically with the tool-call status, and only then does the coordinator dispatch them to the operation manager actor (shell, image, skill use, remote jobs; built on `harness/primitives`).
- `harness/taskgraph` = a pure DAG reducer (`graph.go`) driven by a single-goroutine `Runtime` and exposed to the model as the TaskGraph tool. Checkpoints are re-validated with `Graph.Validate` on resume.
- Compatibility: session-store items and operations are serializable and versioned. Resuming an unsupported version must fail explicitly. Preserve backward reads of old sessions.
- Binaries: `cmd/unreal-agent-runner` -> `cmd/internal/agentrunner` (one-shot run, JSONL events on stdout). The TUI is a frontend that supervises a resident worker (the same binary run with `worker`): `workerhost` (frontend side, `/reload` via graceful `StopWhenIdle`) talks to `workerruntime` over the private `workerproto` pipe protocol. Subagents: `cmd/internal/subagent`. Providers/auth: `cmd/internal/providers`. `cmd/unreal-review` is fork-only (not upstream): it reviews a git diff into findings-v1 (`docs/review/findings-v1.md`) using the same provider registry, default OpenRouter.

## Learnings

- User preference: stay faithful to upstream (`upstream` remote, `unreallabsai/unreal-agent`). Keep the design rules in its README glossary and components table (e.g. translators and the context builder do no I/O; operations are dispatched only after they are committed). Compare against `upstream/main` before changing a shared interface or invariant, and raise any divergence with the user rather than making it silently.

- Fork release config intentionally diverges: tags are `vX.Y.Z-fork.N` (workflow trigger `v*-fork.*` only; first tag `v0.4.1-fork.1`). Goreleaser publishes GitHub releases to `0x7067/unreal-agent`. TUI ldflags use `.Version`, which keeps the `-fork.N` suffix. Docker Hub and the upstream Homebrew tap stay disabled. See `docs/RELEASING.md`.

- User preference: after code changes, always rebuild and reinstall before finishing. In this environment, use `make build` and atomically replace both binaries in `~/.local/bin`; UI changes require a frontend restart.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
