# M0.1 verification — 2026-10-27

Result: **passed**. The complete, unchanged `make test` command ran both Go
modules with `-race`, including all exhaustive TaskGraph cases. All 44 packages
were accounted for: 41 passed and 3 contained no tests. No test case, state bound,
assertion, or race instrumentation was removed.

## Inputs and execution

- Source HEAD: `d24507b7789652736a3f9e3d89e37abcc22d046c` plus the existing working tree.
- New characterization test: `76b7c60598086370a85577036c0f26d8dcf53fc2d769a06570b805e59a271f7e  cmd/internal/workerhost/submission_lifecycle_test.go`.
- Go: `go1.27.3 linux/amd64`.
- Compared the tracked source diff, HEAD, and new test hash before/after this run;
  all were unchanged. Only documentation was edited during verification.
- Task-scoped supervisor: `unreal-agent-m01-validation-20261027181954.service`,
  launched through `systemd-run --user` with repository working directory and
  trusted invoking `PATH`; executable command `/usr/bin/make test`.
- Budget: `RuntimeMaxSec=20min`, `TimeoutStopSec=10s`, `KillMode=control-group`.
  No active frontend restart or global execution-policy change was needed.
- Journal timestamps: started at 18:19:54 UTC, finished at 18:24:33 UTC.
  Reported wall duration: 4min 38.941s; memory peak: 515.3M.
- Raw run artifacts: `/tmp/unreal-agent-m01-validation.z4uOtd` (temporary);
  the full command output is preserved below for repository-local evidence.
- Output SHA-256: `f72872649c46bf6709520ad8ef011a2f9cbcc36866a04a656ec31dd04f52639d`.

Earlier bounded attempts timed out/canceled; those attempts were not treated as
passing validation. This complete run supersedes that validation blocker.

## Ownership and installation checks

After completion the service was inactive/dead, `MainPID=0`, and automatically
collected (`LoadState=not-found`). Its exact cgroup no longer existed. The
unit-scoped journal reported resource consumption and no process failure. Full
command output was checked against `go list ./...` in both modules: every expected
package had a passing or no-test result, both race-suite commands were present,
and no failure/panic/make-error records occurred. A collected unit's default exit
properties alone were not used as proof of success.

There is no retained validation service/job. The service journal and terminal
state were also captured in the temporary evidence directory.

`make check` and `make build` had already passed on the same source inputs.
Both binaries were atomically reinstalled then; their installed SHA-256 values
were rechecked against `bin/` after this full test run and matched. Those build
and check results were reused rather than rerun after documentation-only edits.

## Full `make test` output

```text
go test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner	21.553s
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/providers	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerhost	105.509s
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerproto	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/xdgpath	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-runner	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/contextbuilder	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/coordinator	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/executionbudget	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/inbox	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/ingress	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/anthropic	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/fireworks	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/ollama	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openai	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openaicodex	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/clients/openrouter	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/internal/retryhint	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/messagesapi	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/llm/responsesapi	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/operation	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/primitives	9.281s
?   	github.com/unreallabsai/unreal-agent/harness/session	[no test files]
ok  	github.com/unreallabsai/unreal-agent/harness/sessionstore	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile	2.146s
ok  	github.com/unreallabsai/unreal-agent/harness/settings	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/taskgraph	263.642s
ok  	github.com/unreallabsai/unreal-agent/harness/tool	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/bash	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/taskgraph	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/tool/viewimage	(cached)
ok  	github.com/unreallabsai/unreal-agent/internal/anthropicapi	(cached)
?   	github.com/unreallabsai/unreal-agent/internal/apijson	[no test files]
ok  	github.com/unreallabsai/unreal-agent/internal/jsonoptions	(cached)
ok  	github.com/unreallabsai/unreal-agent/internal/openaiapi	(cached)
go -C cmd/unreal-agent-tui test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui	13.373s
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch	1.023s
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning	1.007s
?   	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/runcontrol	[no test files]
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext	1.204s
```

## Scope

This accepts [M0.1](../../goal.md#m01--trace-one-submission-through-acceptance-and-replay):
[state ownership and failure boundaries](../plans/durable-submission-lifecycle.md)
plus the real-worker submission/retry/completed-history replay test. It does not
claim interrupted-effect recovery, detachable clients, or power-loss durability.
