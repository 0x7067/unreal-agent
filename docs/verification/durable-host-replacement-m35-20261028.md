# M3.5 accepted: controlled idle/drained resident host replacement

[Operator workflow, chosen drain strategy and limits](../plans/durable-host-replacement.md).
No active-worker transfer, automatic process supervisor, generic forced reload,
new-format migration or automatic downgrade is claimed. Upstream comparison is
local `a5f3fd13032737142916523ae4344c392292f9d5`, no fetch. HEAD unchanged
`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`; reducer blob unchanged
`2315afc35788160fc81bf84657f2600bd61c9e86`. Preexisting dirty work preserved;
no commits, inbox production edits or user frontend/production owner access.

## Implementation and real assertions

- `workerhost.Host.RetireForHostReplacement` shares Reload's exact delivery fence,
  persistence/busy checks, quiesce and actual process wait; it skips candidate
  staging/launch. It seals admission under Submit's Add mutex, joins pre-seal
  admissions, publishes stopped, and retains ingress until explicit Close.
- Distinct `host_retire_v1`, strict `retire-host` runtime request and `drained`
  terminal reply. A drained reply cannot contain a running generation/build/error.
  The current-connection/exact-generation fence and separate capability remain.
- `residentcli/retire.go`: public `host-retire` requires explicit directories,
  session, incarnation and generation; connects only, checks exact owner before
  sending, has one actual reader and closes/joins it. It neither signals nor
  launches any host. Ordinary frontend must first detach. Busy/uncertainty is not
  a drained receipt; explicit supervisor stop/**actual wait** is still required.
- `TestRuntimeHostRetirementRetainsDurableInputUntilFreshHost`: real worker Run
  joins managers but holds actual process exit; stable input is acknowledged
  during fence/provider0. Retirement waits, returns nil/drained/gen0, old PID is
  gone; no candidate starts, host remains alive and future submission refuses.
  Old Serve actually stops/joins with ErrStopped; real state/ingress reopens with
  original pending ID/body. Fresh incarnation recovers canonical/provider1;
  original-ID retry deduplicates, pending0 after actual joins.
- `TestBusyAndBadCandidateLeaveWorkerUntouched` now also explicitly asserts busy
  host retirement leaves active work/worker/generation untouched. CLI unfenced
  arguments refuse before dialing. Full suite includes existing late-cleanup and
  abandoned-retirement uncertainty tests; timeout never starts another writer.

## Current validation / install

`/tmp/unreal-agent-m35-validation.MT2rPS`,
`unreal-agent-m35-validation-20261028T092519Z.service`: `make check`, `make build`,
`make test` PASS, race both modules. Exact fresh package union **55 = 52 passing +
3 no-test**, no missing/extra/duplicate rows, FAIL/panic/race. Relevant tracked +
untracked source before/after/current manifests equal, SHA256
`da0a42003ea06abdc78f9d91b82805cdd795d2ef45c327f8cd1ffe40ebaa2b2f`.
Full evidence includes the NEW **unwired** durable-job adapter package; its five
real-service focused tests pass, but it is NOT M4.3 runtime/tool/gate completion.

Both binaries atomically installed, **individual cmp**:

- runner `2777c2542ac2df48ba1b2afc93fd1fc41bbbf3827bc3c30bee06114ac76c4e7a`
- TUI `dcb1b746a8abb7af1078023deff21aa60cde66917b0606b975879e636dbf6d2a`

Focused real resident race: `/tmp/unreal-agent-m35-focused.fhc0De`,
`unreal-agent-m35-focused-20261028T091815Z.service`, M34/M35 PASS. The later busy
and CLI validation assertions pass in the accepted full suite.

## Actual two-host / private tmux proof

`.claude/skills/verify-unreal-agent-tui/scripts/durable_host_upgrade.py` uses
explicit trusted private artifacts in `/tmp/unreal-agent-m35-build.eVfppd`.
A's private Go overlay gates **after real worker Run joins** and after host stop
request/before cleanup; B is current production source with a distinct link build
identity. Negative B host overlay fails startup. Production source is not patched.

Accepted `/tmp/unreal-agent-m35-ui.2CxcFn`, `/tmp/u35.TGu78h`,
`unreal-agent-m35-host-tmux-20261028T093251Z.service`:

1. Real frontend A detaches with process wait and exact TTY restoration. Wrong
   incarnation via actual `host-retire` CLI refuses without worker retirement.
2. Current private protocol control requests idle retirement. Real worker Run
   returns/joins but process exit is held; original stable `m35-pending-stable`
   input is acknowledged during fence/provider0. Release permits actual A worker
   wait and drained reply. Host still holds exact exclusive ingress lock inode;
   session lease is free, not used as takeover authority.
3. Old host stop is requested but actual exit deliberately held. Healthy B starts
   as contender and refuses `own session frontend`; A pidfd remains unexited and
   all nonexecutable canonical/journal bytes unchanged. No timeout authorizes B.
4. Explicit gate release then actual old-host wait/nonzero (latched retirement),
   endpoint gone. Negative B fails with injected startup diagnostic; no fallback.
   Operator explicitly launches healthy B only after old-host actual wait.
5. B recovers exactly one provider request and one canonical original input ID.
   Same ingress lock inode exclusively held, same session lease inode held by its
   new worker, new host incarnation/PID and worker generation1. Fresh frontend
   attaches, followed by full model/Bash frontend replacement, draft/view,
   canonical abort/shutdown and offline reconciliation proof. Three actual
   provider requests (separate recovery prelude1 + baseline2), Bash effect1,
   no late tool. All **13/13** cleanup booleans true; all frontend TTY/process,
   host, socket, relay/server/handler/thread waits. Input hashes unchanged.

Accepted full/focused/build/tmux and unwired adapter focused units all observed
success0/MainPID0/exact recorded cgroup absent, then each exact unit stopped and
same cgroup absent. No intentionally retained unit/helper work.

Rejected proofs were not success: `.IAuTF6` used nonexistent fixture class Handler
(actual Model); `.P3fXNY` shadowed the fixture's frontend command with CLI command;
`.Qr8l4P` intercepted the recovery marker in later conversation history instead
of only its first armed request. All exited1 with scenario assertions false;
owned cleanup and exact cgroup absence/stops observed. Corrected proof uses
separate command name and one armed prelude, final13/13. Early focused `.MiLRgc`
failed a Next signature compile; `.6cOdAi` omitted new test via wrong regex and
was not M35 evidence. Both were exactly stopped/drained. Superseded first build
`.ae3va2` was drained; final artifacts reflect strict drained-status validation.

Archive `durable-host-replacement-m35-20261028-evidence.tar.gz` SHA256:
`09ed1294fedbdf41c4e3c16188d9cef45251a9c2d822162fb89acabca88f68e9`.
**22/27 accepted; continue all five M4 slices. Foundations remain unaccepted.**
