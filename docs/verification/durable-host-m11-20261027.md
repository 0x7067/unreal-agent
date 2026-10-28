# M1.1 independent headless host verification — 2026-10-27

## Result

**Completed M1.1:** explicitly launched foreground runner host plus authenticated
local JSONL attach/submit/watch. One session, one active client; client failure
and disconnection do not own the worker's lifetime. The existing TUI path is
unchanged. [Implemented protocol, commands and limits](../plans/durable-headless-host.md).

- [Transport](../../cmd/internal/attachproto/), [server/publisher](../../cmd/internal/resident/),
  [CLI](../../cmd/internal/residentcli/), [runner entry point](../../cmd/unreal-agent-runner/main.go).
- [Real-process scenario](../../cmd/internal/residentcli/run_test.go),
  [client integrity/lost-ack tests](../../cmd/internal/residentcli/view_test.go),
  [publication/admission-fence tests](../../cmd/internal/resident/publisher_test.go),
  [admission observer test](../../cmd/internal/workerhost/admission_observer_test.go).
- [Complete final validation and focused logs](durable-host-m11-20261027.log),
  [current source/dependency/formal-input manifest](durable-host-m11-20261027.sha256).

## Observed scenario

Production dispatcher, host, worker runtime, coordinator, ingress/store and
provider adapter execute in separate real **host / worker / client** processes;
only the loopback SSE provider is fake.

1. Host publishes PID/incarnation and owns host/session leases. A client gets a
   consistent snapshot, submits one stable external input ID and receives a
   correlated durable-ingress acknowledgment. The provider is blocked mid-turn.
2. Another authenticated client gets `busy`; another host fails its lease before
   replacing/unlinking the endpoint. The original socket inode is unchanged.
3. Interrupt only the first client process. Host/session leases remain held;
   the provider request is not canceled. An incomplete client handshake is
   rejected and joined through peer EOF without retiring the host.
4. A separate client process, with deliberately poisoned provider environment,
   reconnects to the same host PID/incarnation and worker generation **1**.
   Its snapshot has exactly settings/input/turn (**H=3**).
5. Release the provider. A contiguous live model-response item contains the exact
   expected answer. Client reconstruction equals the complete canonical prefix
   through that response. There is exactly **one provider call**, no interruption
   and no repeated input/answer. Explicit later host shutdown adds only its
   expected StopHard shutdown input, releases the session lease and removes the
   socket. Persistent endpoint-lock files intentionally remain.

All three final focused race repeats passed, including explicit expected-answer
content checks. This proves the selected model-only reconnect scenario, not
recovery of arbitrary shell/remote/taskgraph effects.

## Additional checks

- Strict/deterministic JSON framing, exact uint64 values, 1 MiB newline-inclusive
  limit, duplicate/unknown fields, partial/trailing JSON and short writes.
- Same-UID socket credentials, runtime permissions/ancestor/symlink validation,
  lock exclusion, conservative stale recovery, ordinary-file/live-socket
  preservation, replacement-safe cleanup and read-only Dial behavior.
- Unsafe runtime directory rejected **before worker startup/state creation**.
- Pending payload isolation, admission-before-persistence, recovered admission,
  retries/conflicts and late admission callback reconciliation. Mirror overflow
  marks it unavailable; queue overflow invalidates/drains only the client and
  publication continues. Capacity still allows known-ID retries.
- Wrong version/session/token/cursor/type, control input, missing identity and
  oversized payload cannot reach host mutation. Already stopped/stale clients
  cannot authorize a new submission.
- Client validates connection/incarnation, fixed snapshot R/H and ordering,
  contiguous live revisions/heads/items and correlated replies. Lost reply after
  writing a submission is explicitly **uncertain**, retaining the stable input ID.
- Help/probe and attach-side provider override rejection do not read provider
  configuration; installed runner one-shot, host, attach, embedded child-runner
  help and private worker probe smoke checks passed.

An independent read-only safety review found weaker client view validation and
lost-ack guidance in the first candidate. Both were fixed and covered by tests;
follow-up inspection found no additional blocker in that reviewed scope. Review
was not substituted for execution evidence.

## Required validation and installation

Baseline HEAD **`9c0abbe4a9aa003851ef0b3448b58af0a049c4a9`** was preserved,
including its model-check/skill synchronization changes. Existing M0 work was
preserved. Shared boundaries were compared with available local `upstream/main`
**`a5f3fd13032737142916523ae4344c392292f9d5`** (not freshly fetched). That
snapshot does not yet contain this branch's workerhost. The only production
shared-interface extension here is optional `Config.OnAdmission`; no durable
journal/session interface, translator/context-builder purity, commit-before-
dispatch ordering, private protocol/session version or unresolved-operation
resume guard changed.

Commands/results on current final inputs:

```text
go test -race ./cmd/internal/attachproto ./cmd/internal/resident \
  ./cmd/internal/residentcli ./cmd/internal/workerhost \
  -run 'TestCodec|TestEndpoint|TestDial|TestSameSocket|TestCheckPeer|TestPublisher|TestRejectedOrStale|TestServerRejects|TestAdmissionObserver|TestRealHeadless|TestAttachAndProbe|TestView|TestLostSubmission' \
  -v -count=3 -timeout=90s                 PASS
make test                               PASS (42 root + 5 TUI packages)
make check                              PASS (vet both modules + gofmt)
make build                              PASS (both binaries)
source manifest before/after            IDENTICAL
installed binary cmp + entrypoint smoke PASS (both binaries)
```

The full initial test run was supervised in named systemd user unit
`unreal-agent-m11-validation-20261027T202052Z.service`; its current-HEAD
TaskGraph package passed in **290.376 s**. Tests were subsequently strengthened
(expected-answer content and physical temporary paths), so a new full current-
source validation was required. An unsupervised rerun exceeded the shell budget
and was canceled, **not** counted as passing. The final supervised run,
`unreal-agent-m11-final-20261027T203103Z.service`, completed test/check/build with
atomic exit-code **0**, `Result=success`, `ExecMainStatus=0`, `MainPID=0`.
Valid Go test-cache results were reused only for unchanged relevant inputs.
The final package-result set equals `go list` in both modules: no omissions,
duplicates or failures. Source manifests matched before/after final validation.

Both units used 20-minute runtime budgets and cgroup-wide cleanup. Both were
explicitly stopped and observed inactive/dead/unloaded, with their **exact
observed cgroups absent**. A bounded read-only post-cancellation process probe
found no remaining repository test binaries or temporary staged test workers;
unrelated work and the installed/staged production worker were not signaled.
All scoped implementation/review helpers were observed drained. No validation
service or intentionally retained helper remains.

Both binaries were installed using same-directory temporary files and atomic
renames into `~/.local/bin`, then byte-compared with the built files:

```text
8443b8848c329d45b8fdb6dac1e5ae8e93417f3f5f5615383964590404c23e6d  unreal-agent-runner
acecea13982bbf249593d2bd8c17848e83d8799f0fb680ad41827743305b5960  unreal-agent-tui
```

No TUI source change, tmux UI verification or active frontend restart was needed.
No commit was requested or created.

## Limits retained

Fresh snapshots only; no suffix replay/chunking, operation/activity bridge,
controls/reload RPC, automatic startup/adoption, TUI attach or arbitrary operation
recovery. Client queue bounds do not establish whole-host bounded memory.
Incomplete shutdown remains an error, not a universal descendant-drain proof.
Kernel same-UID checks were exercised on Linux; the different-UID subprocess test
was explicitly **skipped without root**, not claimed as executed. Darwin has a
credential implementation and physical short-path test fixtures but was not
runtime-tested here. Process reconnect tests are not power-loss durability proof.

**Next bounded slice: M1.2 — attach the TUI to an already-running turn.**
