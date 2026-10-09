# Repository Guidelines

## Project Structure & Module Organization

- `harness/` contains the reusable Go agent library: coordination, context building, providers, tools, operations, and session storage.
- `cmd/unreal-agent-runner/` provides the JSONL CLI; `cmd/unreal-agent-tui/` is a separate Go module for the terminal UI. Shared CLI code lives in `cmd/internal/`.
- `internal/` holds implementation helpers and generated API types; `third_party/` holds upstream API specifications.
- Go tests sit beside source as `*_test.go`; fixtures use `testdata/`. Python benchmark integration and tests live in `benchmarks/harbor/`. TUI theme assets live in `cmd/unreal-agent-tui/themes/`.

## Build, Test, and Development Commands

Use Go 1.27+; Harbor development also requires Python 3.12+ and `uv`.

- `make build`: build runner and TUI binaries into `bin/`.
- `make test`: run Go tests with the race detector in both modules.
- `make check`: run `go vet` in both modules and check Go formatting.
- `go run ./cmd/unreal-agent-runner -p 'Summarize this project.'`: run the CLI with configured provider credentials.
- `go -C cmd/unreal-agent-tui run .`: launch the TUI from source.
- `make -C benchmarks/harbor test check`: run Python unittest tests and Ruff lint/format checks.

## Coding Style & Naming Conventions

Format Go with `gofmt` (tabs); use lowercase package names and Go's exported-name conventions. Follow existing package boundaries. Format Python with Ruff. Regenerate API types through their `go:generate` directives rather than editing generated output directly.

## Testing Guidelines

Use Go's `testing` package, `Test...` names, and `Fuzz...` targets. Add behavioral regression tests beside changed code. Run `make test check build` before delivery; root-only `go test ./...` omits the TUI module. No numeric coverage threshold is configured. Ollama live tests require `OLLAMA_TEST_MODEL` and a running server.

## Commit & Pull Request Guidelines

History favors concise imperative subjects, occasionally scoped, such as `fix(tui): pretty-print saved preferences`. Keep commits focused. `CONTRIBUTING.md` currently directs contributors to GitHub issues and states maintainers cannot review or merge PRs. For requested patches, explain the behavior change and validation; include screenshots for UI changes.

## Architecture & Security

Tool translators and context builders must perform no I/O; asynchronous operations own execution. Preserve versioned, serializable sessions and operations, with explicit errors for unsupported session versions. Keep provider credentials, `.env` files, and private session logs out of commits.
