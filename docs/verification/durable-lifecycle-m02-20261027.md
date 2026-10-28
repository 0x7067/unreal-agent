# M0.2 verification — 2026-10-27

Result: **passed**. [M0.2](../../goal.md#m02--characterize-disconnect-worker-death-and-deliberate-shutdown)
characterizes frontend loss, unexpected worker death, and explicit shutdown.
No production lifecycle behavior, recovery guard, protocol, or storage format
changed. The [current-versus-target matrix](../plans/durable-lifecycle-boundaries.md)
records the observed hazards rather than treating them as solved.

## New focused evidence

Command (final implementation):

```sh
go test -race ./cmd/internal/workerhost \
  -run '^(TestRealFrontendProcessLossExposesInterruptedShell|TestRealWorkerUnexpectedDeathRetainsHostOwnershipAndRecoveryBoundary|TestRealWorkerCloseDeadlineRetainsHostLeaseAfterWorkerExit)$' \
  -v -count=3 -timeout=85s
```

Passed all three repetitions (38.218s), with no pidfd skips. Five scenarios:

1. Thin frontend SIGKILL during a blocked model request: frontend joined, worker
   exited, provider transport disconnected, leases released; one input/turn and
   no response remain committed.
2. Thin frontend SIGKILL during a FIFO-blocked shell: worker exited but shell
   survived; unresolved state refused actual restart without repeating its effect.
3. Unexpected worker SIGKILL during a model request: no automatic replacement,
   host lease retained until Close; explicit restart issued a second request/turn
   for the same one logical input.
4. Unexpected worker SIGKILL during a shell: separately owned shell survived,
   worker/host leases had distinct lifetimes, startup refused before unsafe replay,
   and the sentinel stayed `x` until fixture cleanup killed/observed the shell.
5. Expired Close caller deadline while callback gated: worker exit did not release
   host/ingress ownership; release plus another Close did.

Tests pin observed Linux process identities with pidfds, not PID-only liveness
checks. The fixture handshake registers worker ownership before admitting work.
Review identified two failure-path cleanup gaps; both were fixed before the final
run, ordering cooperative cleanup before worker-kill fallback if shell discovery
fails. Surviving-shell fixture cleanup is not credited as runtime cleanup.

## Repository validation and installation

- Full `/usr/bin/make test`: **exit 0**, both modules with `-race`; 41 passing
  packages and 3 no-test packages. Compared output with both `go list ./...`
  inventories: all 44 expected packages were accounted for.
- Changed workerhost package: passed in 122.459s, including existing cooperative
  Close, forced Close, runtime resume, and new lifecycle coverage. Unchanged
  TaskGraph checker validation was reused by Go's test cache from the accepted
  M0.1 full run; no assertions or cases were excluded.
- `make check`: passed both modules' vet and the formatting gate.
- `make build`: passed both binary builds; both binaries were atomically installed
  in `~/.local/bin`, file/directory synced, and SHA-256 verified. Runner `-h` and
  TUI `--version` succeeded. No UI implementation changed, so no new terminal UI
  or detach behavior is claimed.
- Installed runner SHA-256: `fc158a84db86e5ee902ab08bd959ae2b1dd1bcab07e14ee13f2b313d510fc327`.
- Installed TUI SHA-256: `27906ed29bdd600063068e3769fa28dc1435c1072526e17af00782cef60c2dfd`.

Source HEAD `d24507b7789652736a3f9e3d89e37abcc22d046c`, tracked source diff, and these
new test hashes were unchanged across the full run:

```text
76b7c60598086370a85577036c0f26d8dcf53fc2d769a06570b805e59a271f7e  cmd/internal/workerhost/submission_lifecycle_test.go
8b24cf0eae6222832fb894b47b08a7ef8c015b9dc6a7c4c670e64c5ec43699ae  cmd/internal/workerhost/lifecycle_boundaries_linux_test.go
```

## Supervision and drain

- Unit: `unreal-agent-m02-validation-20261027184548.service`, intentionally task-scoped.
- Working directory: repository; trusted invoking `PATH`.
- `RuntimeMaxSec=20min`, `TimeoutStopSec=10s`, `KillMode=control-group`.
- `RemainAfterExit=yes` retained the actual service result long enough to inspect.
  The wrapper also atomically recorded make's exit code as `0`.
- Command started 18:45:48 UTC and exited 18:48:07 UTC, approximately 2min 19s.
- Raw temporary artifacts: `/tmp/unreal-agent-m02-validation.2aLmkz`.
- Full output SHA-256: `831bd9bfb56dbf5a5bdb5b6f7fc95c2cbb662980864736a43087a016aa78f9b3`.

Observed completed service state, **before** releasing the retained unit:

```text
ActiveState=active
SubState=exited
RuntimeMaxUSec=20min
RemainAfterExit=yes
MainPID=0
Result=success
ExecMainStartTimestamp=Tue 2026-10-27 18:45:48 UTC
ExecMainExitTimestamp=Tue 2026-10-27 18:48:07 UTC
ExecMainCode=1
ExecMainStatus=0
ControlGroup=
KillMode=control-group
```

Stopped that exact unit after verifying its result. Terminal state:

```text
LoadState=not-found
ActiveState=inactive
SubState=dead
MainPID=0
```

The exact validation cgroup `/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-m02-validation-20261027184548.service` was absent after cleanup. The scoped
read-only review agent also drained (process wait, EOF, reader, and owner exit
confirmed). No retained validation service/helper remains.

## Full `make test` output

```text
go test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner	30.449s
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/providers	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerhost	122.459s
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
ok  	github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/settings	(cached)
ok  	github.com/unreallabsai/unreal-agent/harness/taskgraph	(cached)
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
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui	14.214s
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning	(cached)
?   	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/runcontrol	[no test files]
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext	(cached)
```
