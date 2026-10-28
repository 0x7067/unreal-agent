# Private durable-job dogfood — 2026-10-28

> Follow-up: [the permission-fixture fix](root-race-umask-fix-20261028.md) passed fresh uncached race tests under 077. This document preserves the original failing run/pinned recipe; its historical result is not rewritten.

## Result: recovery PASS, test correctness FAIL

This was a real, uncached race-test job through the **installed `RunTrustedJob` adapter and independent executor**, not a simulated test result. A loopback fake model selected the single trusted job and emitted an outcome after the actual job finished. It did not implement, gate, or replace the tests.

| Command | Packages | Passing | Failed | No test files | Exit |
|---|---:|---:|---:|---:|---:|
| `go test -race -count=1 ./...` | 52 | 47 | 3 | 2 | **1** |
| `go -C cmd/unreal-agent-tui test -race -count=1 ./...` | 5 | 4 | 0 | 1 | **0** |
| Exact union | **57** | **51** | **3** | **3** | Job **1** |

Both commands ran, even though the first failed. Fresh package lists before/after, the preflight lists, and the per-package result rows agree exactly, with no missing/duplicate packages or cached result rows. No `WARNING: DATA RACE` diagnostic appeared; this does **not** make the failed root suite pass.

### Actual test failures and diagnosis

The trusted wrapper deliberately used `umask 077`, with private 0700 parent directories. Three existing permission fixtures assume creation modes survive the process umask:

- `harness/ingress`: `TestPathConfinementAndPrivatePermissions`, `storage_test.go:158`. `os.Mkdir(..., 0755)` actually created 0700, so the supposedly unsafe directory was private and correctly accepted.
- `harness/operation`: `TestRemoteArtifactsPathFailures/private-directory`, `remote_job_artifact_test.go:225`. Its `os.Mkdir(..., 0755)` likewise created a private directory, so the anticipated unsafe-directory failure did not occur.
- `harness/taskgraph`: `TestFingerprintContentAndMembership`, `fingerprint_test.go:62`. The helper's `os.WriteFile(..., 0644)` created 0600; the subsequent `Chmod(..., 0600)` changed no mode, so the test's expected fingerprint change did not occur.

Source inspection and an independent filesystem probe confirmed requested 0755 becomes 0700 under 077, and explicit chmod establishes 0755. These are **test-environment/fixture portability findings**, not evidence that production permission enforcement or fingerprinting is broken. No Go fix was made, and no tests were rerun with a friendlier umask. A future fixture fix should explicitly establish/assert its intended mode; it requires the normal Go-change validation/rebuild/reinstall workflow. Alternatively, a separately authorized daily-test job can use a standard test-process umask while keeping its enclosing directories private. Neither alternative was executed here.

## Recovery actually exercised

1. Created fresh private HOME/config/state, a private foreground tmux server, and a buildable copy of the dirty working tree, including relevant untracked inputs.
2. A committed `RunTrustedJob({"job":"repo-race-tests"})` operation launched the real workload in a separate bounded named executor unit. No model-supplied command or local fallback was used.
3. Observed the actual `go test -race -count=1 ./...` process and a nonzero durable output checkpoint.
4. Detached frontend A with an actual child wait, return code 0, pidfd exit observation, exact TTY restoration, and no forced cleanup.
5. Identified **only** the owned host's private worker by its open session-lock inode and refused lock acquisition. Sent SIGKILL through that worker's observed pidfd; observed worker exit and actually waited for the old host to exit **1**.
6. Verified the same Go test process and executor were still alive. Explicitly started a fresh host and frontend B. Observed a new incarnation and host PID, but the **same session, canonical operation, external job, attempt, fingerprint, and output handle**.
7. Output resumed from offset **19** and reached **5789** bytes. The sole reservation finished `failed` with real exit **1**. The model continuation and actual UI showed `DOGFOOD_TESTS_FAILED root=1 tui=0`.
8. Frontend B also waited 0 with exact TTY restoration and no force. The fresh host actually waited 0 after terminal reconciliation and graceful interrupt. The executor was gracefully stopped and its owned work drained. All **13 recovery/cleanup checks** passed; they are not test-correctness checks.

**One trusted job launch in this recovery exercise**, no recovery resubmission, no local fallback. There was also one earlier rejected setup-only wrapper launch in a different private session: see the explicit accounting below.

### IDs and ownership

| Object | Value |
|---|---|
| Private session | `lifecycle-cb1e661df1fb` |
| Canonical operation | `ae1d38b7-8682-4acd-bda1-c468c4998b1f` |
| External job | `job-e7b47335f886bd7094b070de5e7a1a12bea2792a80c578b99b00a25b897ce185` |
| External attempt | `attempt-e7b47335f886bd7094b070de5e7a1a12bea2792a80c578b99b00a25b897ce185` |
| Controller | `session-2ec6ad06f75e671ed9cb528e22dd6ecd` |
| Environment | `dogfood-tests` |
| Old worker / old host | `1661352` / `1661337` |
| Fresh host / independent executor | `1661556` / `1661269` |
| Surviving real Go command | `1661502` |
| Host/frontend unit | `unreal-agent-dogfood-ui-20261028T150408Z.service` |
| Executor unit | `unreal-agent-dogfood-executor-cb1e661df1fb.service` |

Original ControlGroups (not reconstructed from current empty unit properties):

```text
/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-dogfood-ui-20261028T150408Z.service
/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-dogfood-executor-cb1e661df1fb.service
```

The executor had RuntimeMaxSec **22min**; host/frontend driver **25min**, with bounded stop budgets and control-group kill ownership. The job finished at **2026-10-28 15:13:23 UTC**, before those bounds. Host/frontend driver exit was **0**, distinct from the test/job exit **1**. Named-unit journal, actual child wait receipts, CLI final receipt, original cgroup absence, and final exact-name stop audits are preserved. Transient successful units subsequently unloaded: a later exact-name stop returned **5, not loaded**. This is recorded, not called a successful new stop, and unloaded default `ExecMainStatus=0` fields are **not** used as historical execution proof. MainPID is now 0 and all four original cgroups (including rejected setup) are absent before/after final audit attempts. No dogfood unit retains running work.

## Snapshot, binaries, and unchanged accepted inputs

- **655 files / 5,703,645 bytes**, copied from tracked plus relevant untracked files under `cmd`, `harness`, `internal`, root module files/Makefile, `docs/formal`, `AGENTS.md`, and `README.md`. Regular files only, with source modes preserved; no HEAD-only checkout and no repository mutation by the test job.
- Source before/after/current and snapshot manifests are byte-identical, SHA-256 **`7775d90f9da2225607628cbddca9f28a55e1fefff3f1ccf5bdb31abd50fef510`**.
- The original accepted **653-file** validation scope, excluding the two extra instruction/readme inputs, remains exactly **`017f78bb07f654fbce153eba3132903995ee997e993d63e5db68996f237de212`**. Prior `make check`/build evidence and the shared-interface comparison against local upstream `a5f3fd13032737142916523ae4344c392292f9d5` remain applicable to these unchanged inputs; they do not override this new failed test run. See [the accepted M4 report](durable-workloads-m4-20261028.md).
- Installed runner SHA-256 **`b7d7be82195527995d745a572836ec92c009177946007e93a7bd7b6b152787c8`**.
- Installed TUI SHA-256 **`8d39575dfdec0b2deabe27d0f13c0f3bc68417f3533296b489f08b66c91d36b8`**.
- Each installed binary compared equal to its repo `bin/` counterpart before/after; no rebuild/install or Go edits were needed or performed.
- HEAD remains **`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`**; `harness/taskgraph/graph.go` remains blob **`2315afc35788160fc81bf84657f2600bd61c9e86`**.

The job used Go 1.27.3, `GOFLAGS=-p=2 -buildvcs=false`, private HOME/TMPDIR, explicitly named existing compiler/module caches, `GOTOOLCHAIN=local`, `GOPROXY=off`, and `GOSUMDB=off`. Reusing compilation/dependency caches does not cache test results with `-count=1`. The job environment did not inherit executor or provider credentials. TLS used a private self-signed CA, explicit name verification, an allowlisted controller, and an ephemeral scoped bearer value held only in process environment/memory/header. Its value was absent from canonical state and captured frontend frames. Private key material is excluded from the evidence archive.

## Rejected setup attempt — not hidden as success

Artifacts: `/tmp/unreal-agent-dogfood.jQUG5O`; private root `/tmp/ud.ORBgOX`; session `lifecycle-a57292900138`.

The first wrapper did `go list` without disabling VCS stamping for a snapshot without `.git`. Both lists failed with `error obtaining VCS status`; the guard exited **90 before either test command started**. The fixture then noticed the departed wrapper while inspecting its private children and exited 1. This was neither a completed recovery proof nor a test failure. Its private frontend/host/executor were joined/drained. The correction was **private fixture flags only**, followed by fresh private session/unit identities, not retrying the original logical job.

Rejected units:

- `unreal-agent-dogfood-ui-20261028T145740Z.service`
- `unreal-agent-dogfood-executor-a57292900138.service`

**Aggregate accounting: two trusted wrapper launches across separate sessions = one rejected preflight-only launch + one actual two-module test-job launch.** There was no resubmission during recovery, and no second test-job run to turn the umask failures green.

## Usability findings and limits

1. **The useful part works:** the operator can leave/reconnect and replace the failed private runtime while a real named test job continues, preserving identity, output and a failed outcome rather than inventing success.
2. **Private environment affects tests:** restrictive umask exposed three fixture assumptions. Capture environment, per-module exit codes and full logs; an assistant's final sentence is not correctness evidence.
3. **Source snapshots need explicit build policy:** `-buildvcs=false` is needed here when copying without `.git`; offline caches/toolchain must be explicit. The failed preparation was visible and safely drained.
4. **Operator setup is still cumbersome:** private directories, controller derivation, TLS/config permissions, separate unit ownership and several identity receipts are manual concepts. The replay fixture helps verification, but is not a shipped one-command product UX or a new feature project.
5. **Transient-unit diagnostics need care:** persist original cgroups and exit receipts before collection. `not-found` plus default status fields must not be mistaken for historical success.

This establishes this installed adapter/executor recovery case, not arbitrary tool/plugin recovery, executor-restart recovery, or arbitrary side-effect exactly-once semantics. The executor itself stayed alive. Roadmap acceptance remains the prior **27/27**; this report adds practical dogfood evidence and an unresolved test-portability finding, not a release/rollout.

The protected production session was not attached, controlled, read, or killed. No protected tool-output files were opened. No commits, fetches, production changes, or edits to the preexisting dirty source were made. New repository deliverables are this report, [the copy-paste runbook](private-dogfood-20261028-runbook.md), and [the evidence archive](private-dogfood-20261028-evidence.tar.gz).

## Evidence locations

- Actual run: `/tmp/unreal-agent-dogfood.3F5WQ9`; private root `/tmp/ud.t8zOMh`.
- Snapshot: `/tmp/unreal-agent-dogfood.jQUG5O/snapshot` (identical hashes before/after/current).
- Archive: [evidence](private-dogfood-20261028-evidence.tar.gz), [checksum](private-dogfood-20261028-evidence.tar.gz.sha256). SHA-256 **`605eafa0cd0f6aef8c8f45237028dabe55019c6ec0095a575200b3f4a77a9dbd`**; **118 regular members**, 36,952,154 uncompressed bytes / 519,576 compressed bytes.
- Archive contains full root/TUI logs, package lists/manifests, job/driver/recovery/TTY receipts, canonical session records, captured UI frames/screens, exact-unit journals/audits, rejected-setup evidence, and replay scripts. It omits the source copy, caches, private home, credentials, TLS private key and protected tool-output files. Replay scripts are syntax-checked; the exact executed driver/derived source is retained separately. The packaging recipe itself was not launched as an extra test run.
