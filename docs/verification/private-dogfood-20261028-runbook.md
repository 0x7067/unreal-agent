# Private dogfood operator runbook

> Follow-up: [the permission-fixture fix](root-race-umask-fix-20261028.md) passed fresh uncached race tests under 077. This document preserves the original failing run/pinned recipe; its historical result is not rewritten.

See [the observed result and limitations](private-dogfood-20261028.md). **Recovery passed; root tests failed under umask 077; TUI tests passed.** This replays that environment, not a green-suite claim.

## Preconditions and safety

- Use zsh as the same unprivileged user. Requires the validated installed runner/TUI pair, user systemd, tmux, OpenSSL, Go 1.27.3 with race support, and populated dependency/compiler caches.
- The archived recipe refuses changed validated source hashes, a changed lifecycle fixture, snapshot symlinks, or installed binaries unequal to repo `bin/`. If Go inputs differ, first follow the required validation/build/reinstall workflow; do not bypass the refusal.
- It creates fresh private session/config/HOME/state/tmux and source-copy directories. Never substitute production session IDs, directories, endpoints or units.
- The loopback fake model only selects `RunTrustedJob`. Both actual uncached race commands run, even if the first fails. Dependencies are offline (`GOPROXY=off`); no fetch is needed.
- `umask 077` intentionally reproduces the three permission-fixture failures. No silent retry with different umask or flags is included.

## 1. Extract and launch

From `/data/development/projects/0x7067/unreal-agent`:

```zsh
sha256sum -c docs/verification/private-dogfood-20261028-evidence.tar.gz.sha256
E=$(mktemp -d /tmp/unreal-agent-dogfood-recipe.XXXXXX)
tar -xzf docs/verification/private-dogfood-20261028-evidence.tar.gz -C "$E"
zsh "$E/dogfood-evidence/recipe/start.zsh" "$PWD"
```

The final line prints **QUEUED**, an `artifacts=/tmp/unreal-agent-dogfood.…` directory and unit name. Submission is not evidence of running tests, completed recovery or passing tests. Save the printed artifact path as `D` below.

The recipe copies relevant tracked/untracked current inputs (not HEAD), verifies source/copy manifests, and preflights both package lists with `GOFLAGS=-p=2 -buildvcs=false`. It starts a separately bounded executor unit with strict private configs, TLS name/CA verification and a controller allowlist. Its scoped token travels through environment/memory/header, not model arguments, plans or checkpoints. The real job has an explicit environment without that token.

It launches actual private host/frontend A in private tmux. After observing the real Go command and committed output, it detaches/waits A, identifies the private worker using the owned host's children and session-lock inode, kills only that worker through pidfd, waits for the old host, and explicitly starts fresh host/frontend B. It verifies stable operation/job/attempt/output identity and launch count 1, waits for the real result, then joins the private UI/host/executor/relay/HTTP/tmux resources.

No global worker scan, `pkill`, `killall`, automatic resubmission or local fallback is used. Do not manually retry because a UI disconnects. The executor stays alive throughout recovery; executor restart is a different proof case.

## 2. Observe and join — do not start tests locally

Replace only the `D` value with the fresh printed path:

```zsh
D=/tmp/unreal-agent-dogfood.REPLACE_ME
R=$(cat "$D/root")
U=$(cat "$D/ui-unit")
systemctl --user show "$U" -p LoadState -p ActiveState -p SubState -p MainPID
# Observation only: tests already run in their executor unit.
while [[ $(systemctl --user show "$U" -p SubState --value) == running ]]; do
  sleep 2
done
cat "$D/driver-exit-code"
cat "$D/job-result.json"
cat "$R/lifecycle/evidence/dogfood-result.json"
cat "$R/lifecycle/evidence/cleanup.json"
```

Missing receipts or nonzero driver exit mean incomplete/failed fixture proof, not PASS. Inspect `preflight.stderr`, `driver.log` and `lifecycle/evidence/failure.txt` if present. The first recovery receipt says `running`; require terminal `PASS` plus completed cleanup/waits.

The exact commands are in `job.zsh`; full logs are `root-tests.log` and `tui-tests.log`. **Do not follow paths advertised by truncated tool-result previews**; those output files are unnecessary for this audit.

## 3. Separate recovery from test correctness

```zsh
python3 - "$D" <<'PY'
from pathlib import Path
import json,re,sys
p=Path(sys.argv[1]);r=Path((p/'root').read_text().strip());e=r/'lifecycle/evidence'
result=json.loads((e/'dogfood-result.json').read_text())
cleanup=json.loads((e/'cleanup.json').read_text())
receipt=json.loads((p/'job-result.json').read_text())
assert (p/'driver-exit-code').read_text().strip()=='0'
assert result['recovery']=='PASS' and all(cleanup.values())
assert len((p/'job-launches').read_text().splitlines())==1
seen=[]
for module in ['root','tui']:
    before=(p/f'packages.{module}.before').read_text().splitlines()
    assert before==(p/f'packages.{module}.after').read_text().splitlines()
    assert before==(p/f'preflight.{module}').read_text().splitlines()
    rows=[]
    for line in (p/f'{module}-tests.log').read_text().splitlines():
        m=re.match(r'^(ok|FAIL|\?)\s+(github\.com/unreallabsai/unreal-agent(?:/\S*)?)\s+(.*)$',line)
        if m:
            status,name,tail=m.groups();assert '(cached)' not in tail
            rows.append(name)
    assert len(rows)==len(set(rows)) and sorted(rows)==sorted(before)
    seen.extend(rows)
    print(module,'packages=',len(rows),'actual exit=',receipt[module+'_exit'])
assert len(seen)==57 and len(set(seen))==57
assert receipt['job_exit']==result['job_exit']
print('RECOVERY: PASS')
print('TEST CORRECTNESS:', 'PASS' if receipt['root_exit']==receipt['tui_exit']==0 else 'FAIL')
PY
```

Historical expected result: recovery PASS, root exit **1**, TUI exit **0**, job exit **1**. Driver exit 0 is not job/test exit 0. The fake model's wording is not test evidence; use actual receipts/logs.

## 4. Verify original cgroups are gone; stop only owned units

The fixture normally stops the executor itself. Successful transient units may already have unloaded. Persisted receipts and named-unit journals matter: default status fields for `LoadState=not-found` are not historical exit evidence.

```zsh
python3 - "$D" <<'PY'
from pathlib import Path
import json,subprocess,sys
p=Path(sys.argv[1]);x=json.loads((p/'executor-unit.json').read_text())
pairs=[((p/'ui-unit').read_text().strip(),(p/'ui-cgroup').read_text().strip()),(x['unit'],x['cgroup'])]
for unit,cg in pairs:
    fields=subprocess.check_output(['systemctl','--user','show',unit,'-p','LoadState','-p','MainPID','-p','Result','-p','ExecMainStatus'],text=True)
    print(unit,fields,sep='\n')
    state=dict(line.split('=',1) for line in fields.splitlines())
    assert state['MainPID']=='0', 'still running: do not report completion'
    assert not Path('/sys/fs/cgroup'+cg).exists(), 'original cgroup still present'
    stopped=subprocess.run(['systemctl','--user','stop',unit],capture_output=True,text=True)
    assert stopped.returncode==0 or (stopped.returncode==5 and state['LoadState']=='not-found'),stopped.stderr
    assert not Path('/sys/fs/cgroup'+cg).exists()
    print('exact final stop:',stopped.returncode,'(5 allowed only for already-unloaded unit)')
    subprocess.run(['journalctl','--user','-u',unit,'-n','80','--no-pager'],check=True)
PY
```

If the fixture fails while running, preserve receipts and classify it failed/incomplete. Stop only exact recorded private units to terminate owned work, then verify original cgroups are gone. Forced cancellation is not a completed test result. Never kill production frontend/host/worker/executor or the user's tmux server.

## Historical evidence and next action

The archive contains `run/validation.json`, `run/unit-audits.json`, complete command logs/package manifests, `run/recovery/` UI/wait/TTY receipts, `run/session/` canonical records/reservation, and `rejected-setup/` for the earlier VCS-stamping preparation failure. Exact executed driver source is separate from supporting `recipe/` files. Recipe packaging/syntax checks did not launch another test job.

No fix is included. A useful separate follow-up is to make the three permission fixtures explicitly establish/assert intended modes, then perform required Go-change validation and installed-private-UI verification. That is not silently folded into this dogfood run.
