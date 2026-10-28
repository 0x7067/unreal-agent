# Operation checkpoint audit with TLA+/TLC

`OperationCheckpoint.tla` models the operation manager actor
(`harness/operation/local_manager.go`), the coordinator that persists every
update it receives (`handleOperationUpdate` -> `SaveOperation`), and
crash/resume (`dispatchOperationsToManager` re-adds stored nonterminal
operations). It is abstract: it does not prove refinement of the Go code.
Each counterexample was reproduced against the real code by a test in
`harness/operation/formal_findings_test.go`; those tests now guard the fixes.

```sh
TLA2TOOLS_JAR=/path/to/tla2tools.jar docs/formal/operations/check.sh
go test ./harness/operation -run 'TestShellCrashBeforeProcessCheckpointRunsCommandOnce|TestRemoteJobHandlerStopAfterCancellationFailureEmitsOneTerminalUpdate' -count=1 -v
```

| Config | Toggles | Expected |
| --- | --- | --- |
| `legacy-spawn.cfg` | code before the fix | `AtMostOnceSpawn` violated |
| `legacy-close.cfg` | code before the fix | `SingleTerminalCheckpoint` violated |
| `fixed.cfg` | `ExclusiveErrCreate`, `CloseSkipsTerm` | clean, 324 states, crashes <= 2 |

## Findings

### 1. A crash before the process checkpoint is durable re-runs the shell command

Reachable in `upstream/main`. In this fork the shipped resume paths
(`agentrunner` and `workerruntime`) first run `cmd/internal/recoverygate`,
which refuses any restored shell outside its read phases ("prior execution
uncertain; ownership reconciliation required"). So the binaries refuse to
resume instead of re-running. The fix below is defense in depth for direct
users of `harness/operation` and `harness/coordinator`.

`acceptLocalStep` queues the `phase=process` checkpoint (`appendLocalUpdate`)
and calls `startLocalPrimitive` in the same actor step. The process runs before
the coordinator has even received that checkpoint. `Shell.resume` refuses to
re-run a recorded `process` phase ("execution outcome is unknown"), but a crash
in this window leaves `create_err` as the last durable phase. Resume
re-dispatches `create_err`; `useExistingPath` makes creation idempotent; the
shell moves to `process` again and **runs the command a second time**.

TLC trace (`legacy-spawn`): `CreateErrDone` (spawn 1, checkpoint queued) ->
`Crash` -> `Resume` (stored phase `create_err`) -> `CreateErrDone` (spawn 2).

Test: `TestShellCrashBeforeProcessCheckpointRunsCommandOnce` reads updates only
up to `create_err`, waits for the command's side effect, cancels the manager,
resumes the last persisted operation in a new manager, and observes
`"run\nrun\n"`.

Fix (modeled by `ExclusiveErrCreate`): the err file is the last thing created
before the process starts, and the shell now creates it with
`IOCreateRequest.Exclusive`. Because the stored phase may lag several
checkpoints, a resumed shell that finds the err file already present cannot
know whether the command ran, so it fails with "shell execution outcome is
unknown ..." instead of re-running it. This matches the existing fail-closed
policy for a recorded `process` phase without adding a coordinator->manager
durability acknowledgement.

### 2. Handler stop after a failed cancellation emits a second terminal update

Introduced in this fork (upstream deletes failed operations immediately).

A failed `CancelRemoteJob` marks the operation `remoteUnknown`, emits a terminal
`Failed` update, and `finishOwned` keeps it in the map. When the handler's
update stream later closes, the `update.closed` branch calls
`failLocalOperation` on every operation of that handler without checking
`current.terminal`, which every other remote branch does. The coordinator stores
the second `Failed` (`"remote job handler 0 stopped"`) over the first (`"remote
job outcome is unknown because cancellation failed"`), so the durable
checkpoint no longer matches the result the model was given. The session store
accepts any status overwrite (`sessionHead.saveOperation`), so nothing catches
it.

Test: `TestRemoteJobHandlerStopAfterCancellationFailureEmitsOneTerminalUpdate`.

Fix (modeled by `CloseSkipsTerm`): the closed branch keeps `remoteUnknown =
true` for retained terminal operations but no longer fails them again.
