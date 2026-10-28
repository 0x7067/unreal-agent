# M0.3 recovery-boundary verification — 2026-10-27

## Scope and result

**Completed:** version/phase [recovery matrix](../plans/durable-recovery-matrix.md)
and six new boundary scenarios in
[`recovery_boundary_test.go`](../../cmd/internal/workerruntime/recovery_boundary_test.go).
No production recovery behavior, interface or guard changed.

Shell versions 3 and 4 each cover committed intent only, actual process dispatch
before the gated external effect, and one completed external effect plus a
successful manager result before that result/checkpoint is saved. This is a
**controlled lossy-persistence driver**, not a physical crash test. Real abrupt
process-death evidence remains in [M0.2](durable-lifecycle-m02-20261027.md).

Observed counts are **0 / 0 / 1**, with the same restored ready intent. The real
manager drains before reopening storage. Resume refuses before provider/credential
setup or history transfer, leaves session JSONL bytes unchanged, does not repeat
the effect, and releases the session lease. The result is a refusal baseline,
**not permission to retry ready operations**.

Important documented gaps include provider attempts without durable dispatch
identity, source mutation during image/skill recovery, interrupted manual-only
compaction, subagent/TaskGraph nested checkpoint enqueue without persistence
acknowledgement, and cross-crash control-result delivery. Untested cells remain
explicitly blocked. Report, terminal update, cancellation request, and drain are
not conflated.

## Commands and accepted validation

- `go test -race ./cmd/internal/workerruntime -run '^TestResumeRefusesShellIntentAcrossUncommittedExecution$' -v -count=3 -timeout=60s`
  **passed**, six scenarios per run, **18 subtest passes**, 1.088s final focused run.
- `make test` **exit 0**, both modules: **41 passing packages / 3 no-test packages**.
  Package names were checked against `go list ./...` in each module; all 44 were
  accounted for and no FAIL/panic/make-error marker appeared. Workerruntime ran in
  1.771s; unchanged packages reused valid Go race-test cache, including the
  expensive unchanged TaskGraph checker already validated in M0.1/M0.2.
- `make check` **exit 0**: vet both modules and repository gofmt check.
- `make build` **exit 0**: runner and TUI rebuilt with `-trimpath`.
- Both binaries **atomically reinstalled individually** in `~/.local/bin`
  using same-directory temporary files and rename; `cmp` and SHA-256 matched.
  Installed runner `-h` and installed TUI `--version` both exited 0.
  No UI source changed and no running frontend was restarted.

The fixture initially exposed a missing artifact-parent-directory setup in the
new driver; that setup was corrected before the final-source validations above.
No production fix was needed.

Source HEAD: `d24507b7789652736a3f9e3d89e37abcc22d046c`.
Tracked source diff and a complete tracked/untracked relevant-source manifest
(`cmd`, `harness`, `internal`, module files, Makefile, formal specs) were unchanged
across the full run. Manifest SHA-256:
`3405d9db1fad18f2afac660821cb097ed0e6185c76e09d6a9fb8c2a128828d43`.
Final-source focused repetition used the same new-test hash:

```text
7c1ba5da8afcd620d02e712a1e5ba620e033dd51aae2c25619a31be03a90ae07  cmd/internal/workerruntime/recovery_boundary_test.go
76b7c60598086370a85577036c0f26d8dcf53fc2d769a06570b805e59a271f7e  cmd/internal/workerhost/submission_lifecycle_test.go
8b24cf0eae6222832fb894b47b08a7ef8c015b9dc6a7c4c670e64c5ec43699ae  cmd/internal/workerhost/lifecycle_boundaries_linux_test.go

fc158a84db86e5ee902ab08bd959ae2b1dd1bcab07e14ee13f2b313d510fc327  bin/unreal-agent-runner (= installed)
27906ed29bdd600063068e3769fa28dc1435c1072526e17af00782cef60c2dfd  bin/unreal-agent-tui (= installed)
```

Compared against locally available `upstream/main`
`a5f3fd13032737142916523ae4344c392292f9d5` (not freshly fetched). Only tests/docs
added by this slice: no new shared-interface/invariant divergence. Existing
nested enqueue-versus-persist ordering is surfaced, not silently changed.

## Supervision / cleanup

User-authorized supervised budget retained from prior slices. Task-scoped user
unit `unreal-agent-m03-validation-20261027190730.service` ran unmodified
`make test` with a 20-minute runtime maximum, 10-second stop bound and
control-group cleanup. Its wrapper atomically recorded make exit code **0**.
Observed completion before releasing the unit:

```text
ActiveState=active
SubState=exited
RuntimeMaxUSec=20min
RemainAfterExit=yes
MainPID=0
Result=success
ExecMainStartTimestamp=Tue 2026-10-27 19:07:30 UTC
ExecMainExitTimestamp=Tue 2026-10-27 19:07:32 UTC
ExecMainCode=1
ExecMainStatus=0
ControlGroup=
KillMode=control-group
```

Stopped that exact unit after checking result, full output and source freshness.
Terminal state:

```text
LoadState=not-found
ActiveState=inactive
SubState=dead
MainPID=0
```

Exact cgroup
`/sys/fs/cgroup/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-m03-validation-20261027190730.service`
was absent. Raw artifacts: `/tmp/unreal-agent-m03-validation.PpqIFU`.
Full make-test log SHA-256:
`3ef5bb33bf3e1a3e62f9012b7a4cc1394956a4d2871720aa92cf9ebfde5cfa5f`.
Read-only model/remote matrix helpers and final boundary reviewer all **drained**:
process wait, EOF, output reader and owner exit were observed. Final review
handle `a8b8ba71-5408-408f-8ab0-3c47455d44eb`, generation 1, explicitly returned
`DrainConfirmed=true`. No retained helper or validation service remains.

Review found no blocking semantic issue. Its failure-path hardening suggestion
was applied: a manager drain timeout now aborts the scenario rather than allowing
fresh-store recovery to proceed after unconfirmed drain. Final focused/full
validation, check/build and installation above were renewed after that change.

## Full `make test` output

```text
go test -race ./...
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/providers	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/subagent	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerhost	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerproto	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime	1.771s
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
ok  	github.com/unreallabsai/unreal-agent/harness/primitives	(cached)
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
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/filesearch	(cached)
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/reasoning	(cached)
?   	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/runcontrol	[no test files]
ok  	github.com/unreallabsai/unreal-agent/cmd/unreal-agent-tui/internal/terminaltext	(cached)
```

## Final-source focused race repetitions

```text
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only
    recovery_boundary_test.go:158: v3 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect
    recovery_boundary_test.go:158: v3 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit
    recovery_boundary_test.go:158: v3 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only
    recovery_boundary_test.go:158: v4 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect
    recovery_boundary_test.go:158: v4 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit
    recovery_boundary_test.go:158: v4 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
--- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution (0.03s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect (0.01s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect (0.01s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit (0.01s)
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only
    recovery_boundary_test.go:158: v3 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect
    recovery_boundary_test.go:158: v3 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit
    recovery_boundary_test.go:158: v3 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only
    recovery_boundary_test.go:158: v4 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect
    recovery_boundary_test.go:158: v4 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit
    recovery_boundary_test.go:158: v4 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
--- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution (0.02s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect (0.01s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit (0.00s)
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only
    recovery_boundary_test.go:158: v3 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect
    recovery_boundary_test.go:158: v3 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit
    recovery_boundary_test.go:158: v3 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only
    recovery_boundary_test.go:158: v4 intent_only: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect
    recovery_boundary_test.go:158: v4 dispatched_before_effect: same durable ready intent, effect=""; resume refused without execution or mutation
=== RUN   TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit
    recovery_boundary_test.go:158: v4 effect_before_result_commit: same durable ready intent, effect="x"; resume refused without execution or mutation
--- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution (0.02s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/dispatched_before_effect (0.01s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v3/effect_before_result_commit (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/intent_only (0.00s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/dispatched_before_effect (0.01s)
    --- PASS: TestResumeRefusesShellIntentAcrossUncommittedExecution/v4/effect_before_result_commit (0.00s)
PASS
ok  	github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime	1.088s
```
