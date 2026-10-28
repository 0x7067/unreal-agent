# Root uncached race-test permission-fixture fix — 2026-10-28

## Scope and diagnosis

Follow-up to the [private dogfood run](private-dogfood-20261028.md), whose root tests genuinely failed. That historical outcome is not rewritten as passing.

Only three test fixtures changed (12 inserted lines):

- `harness/ingress/storage_test.go`: explicitly chmod the negative-test directory to 0755 before asking ingress to reject it.
- `harness/operation/remote_job_artifact_test.go`: explicitly chmod the `private-directory` failure fixture to 0755 before artifact handling.
- `harness/taskgraph/fingerprint_test.go`: explicitly establish starting file mode 0644 before taking the baseline fingerprint, so the later chmod to 0600 changes a mode.

Creation modes are filtered by the process umask. With 077, the first two fixtures previously created private 0700 directories, and the third started at 0600. The security/fingerprint behavior was correct; the test premises were not. The fix establishes those premises without weakening assertions or production permission checks. It does not change process-global umask inside tests, add skips, or change the taskgraph reducer. The intentionally non-private fixtures remain inside private temporary parents.

`harness/tool/tool.go`, including Context, Registry and translator interfaces, compared byte-for-byte equal to local upstream `a5f3fd13032737142916523ae4344c392292f9d5`; no fetch or shared-interface/invariant changes.

## Validation

**PASS.** Focused race tests passed under both 022 and 077. `make check`, `make build`, and `make test` all exited **0**, with the full uncached two-module race suite under **077**.

| Module | Exact packages | Passing | No test files | Exit |
|---|---:|---:|---:|---:|
| Root | 52 | 50 | 2 | 0 |
| TUI | 5 | 4 | 1 | 0 |
| Exact union | **57** | **54** | **3** | **0** |

No cached package rows or race diagnostics. Source before/after/final-current manifests are identical: **653 tracked/untracked relevant files**, SHA-256 **`efbcd425bc28d5ee0f0522615c859fc85f7757814f419d16313bd3874d3c930e`**. Their name inventory is unchanged. Exactly the three listed test files differ from the failing baseline (`017f78bb07f654fbce153eba3132903995ee997e993d63e5db68996f237de212`).

The unit uses private HOME/config/state/TMPDIR, an explicit environment with existing Go compiler/dependency caches, Go 1.27.3, `GOPROXY=off`, `GOTOOLCHAIN=local`, and `GOFLAGS=-p=2 -count=1`. Go applies count only to commands that recognize it; this makes both `make test` module commands uncached without imposing an unknown command-line flag on build/vet or their child builds. No race instrumentation bypass, test-cache clearing, foreground timeout suite, or friendlier full-suite umask is used.

Fresh per-module package lists and tracked/untracked source manifests before/after/current were captured and verified. The prior failing root log and the pre-fix source manifest are retained to establish that only these three test files differ from the failing baseline.

## Execution accounting

The first private validation wrapper used zsh's read-only `status` parameter as a scratch variable. Its focused 022 tests passed, then the wrapper exited 1 before the second focused mask or any make phase. That unit was joined/drained/stopped and is rejected as full-validation evidence. Only private wrapper variable names changed before the corrected validation launch. Its journal and original cgroup audit are retained separately.

## Boundaries

No production Go changes, protected graph edits, user frontend/session access, protected tool-output reads, commits, fetches or production rollout. The earlier dogfood archive/recipe remains historical and intentionally pinned to its original source hash; do not bypass its refusal on the now-fixed source tree.

## Rebuild, reinstall and actual private UI proof

Both binaries were rebuilt with `make build`, staged beside their installed destinations, fsynced and atomically replaced individually in `~/.local/bin`, then each compared byte-for-byte to its `bin/` counterpart. Installed modes are 0755. Production code did not change, so the rebuilt binary hashes remained:

- Runner: `b7d7be82195527995d745a572836ec92c009177946007e93a7bd7b6b152787c8`
- TUI: `8d39575dfdec0b2deabe27d0f13c0f3bc68417f3533296b489f08b66c91d36b8`

The actual **installed absolute paths** then ran the owned M4/lifecycle fixture in private tmux with sanitized inherited environment. It proved four frontend actual waits/return-code-0/exact-TTY/no-force receipts; private worker pidfd exit and old host wait1; fresh host incarnation reattached the same remote job; one remote effect, `firstsecond` output, independent executor actual wait0; live Echo/Reminder; then baseline model/Bash controls and offline reconciliation. All **13 cleanup checks** were true, with 8 fake-provider requests (4 auxiliary, 2 remote, 2 baseline). The job in this UI proof was the short lifecycle fixture, **not another full race-suite run**. Binary/driver/base hashes matched before/after/current.

This is an actual new private UI/runtime replacement proof using the reinstalled pair, not a claim that the user's frontend was restarted. No user frontend/session was inspected or controlled.

## Units, receipts and cleanup

- Full validation: `unreal-agent-umask-fix-validation-20261028T153755Z.service`, RuntimeMaxSec 30min.
- Installed-pair UI: `unreal-agent-umask-fix-ui-20261028T155012Z.service`, RuntimeMaxSec 3min.
- Source/check/build/test/install artifacts: `/tmp/unreal-agent-umask-fix.TUGDn4`.
- Private test HOME/TMPDIR root: `/tmp/uf.aKpovx`.
- Private UI root: `/tmp/uu.i9NoYS`.
- UI canonical remote operation: `15c2cc54-118b-4bae-84c1-3f80da603df2`.

Both accepted units had **loaded meaningful success/ExecMainStatus0/MainPID0** receipts and their original exact cgroup directories absent **before** successful exact-name stop; original cgroups remained absent after. Original ControlGroups:

```text
/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-umask-fix-validation-20261028T153755Z.service
/user.slice/user-1000.slice/user@1000.service/app.slice/unreal-agent-umask-fix-ui-20261028T155012Z.service
```

The first UI wrapper also had a private installed-path substitution NameError before any UI was launched (`unreal-agent-umask-fix-ui-20261028T154802Z.service`). It was rejected/drained/stopped; the corrected transform was preflighted in memory without launching processes before the actual successful UI run. The earlier zsh wrapper rejection was `unreal-agent-umask-fix-validation-20261028T153546Z.service`. Both rejected starts have nonzero receipts/journals and exact cgroup absence recorded; neither is called passing validation. Neither private-wrapper correction changed Go inputs or invalidated the accepted suite evidence.

HEAD remains `9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; `harness/taskgraph/graph.go` remains blob `2315afc35788160fc81bf84657f2600bd61c9e86`. The preexisting dirty/untracked tree was preserved; no commits/fetch or production rollout. Dotfiles/Pi-hook configuration was outside this latest request and was not modified.

## Archived evidence

[Evidence archive](root-race-umask-fix-20261028-evidence.tar.gz) and [SHA-256](root-race-umask-fix-20261028-evidence.tar.gz.sha256) contain focused/full logs, exact package inventories and source manifests, the patch/pre-fix test files, independent validation/install/UI/wait/TTY receipts, original unit cgroups/journals, and rejected-start evidence. No binaries, private TLS keys, user credentials, private home/cache contents, or protected tool-output files are included.

Archive SHA-256: `bb1d1f5152b415cd7b2d2f9bbc189e9cfc3dd8a1526878d85dc31b7a9df22170`; 159 regular members, 1121377 uncompressed bytes / 256559 compressed bytes. Member bytes were independently compared to the owned artifacts; no absolute/parent traversal/symlink entries or private-key PEM were accepted.
