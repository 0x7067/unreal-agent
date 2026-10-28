#!/usr/bin/env python3
"""Owned Linux resident runtime upgrade, then the full lifecycle/frontend fixture.
A/B are compatible same-source runner builds with distinct link buildids. Negative
runner changes only worker-probe protocol to 999. No production paths touched.
"""
import fcntl
import hashlib
import json
import os
from pathlib import Path
import select
import sys
import tempfile

REPORT = {}
HANDLES = {}


def atomic_candidate(source, target):
    fd, name = tempfile.mkstemp(prefix='.candidate-', dir=target.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(source.read_bytes())
            stream.flush()
            os.fsync(stream.fileno())
        os.chmod(name, 0o700)
        os.replace(name, target)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def capture_worker(host, state, sid):
    threads = list(Path(f'/proc/{host.pid}/task').iterdir())
    assert 0 < len(threads) <= 256
    children = set()
    for thread in threads:
        try:
            children.update((thread / 'children').read_text().split())
        except FileNotFoundError:
            pass
    assert len(children) == 1, f'expected one actual owned worker: {children}'
    pid = int(next(iter(children)))
    lease = state / (sid + '.session.lock')
    identity = (lease.stat().st_dev, lease.stat().st_ino)
    descriptors = list(Path(f'/proc/{pid}/fd').iterdir())
    assert len(descriptors) <= 256
    held = False
    for descriptor in descriptors:
        try:
            st = descriptor.stat()
            held |= (st.st_dev, st.st_ino) == identity
        except FileNotFoundError:
            pass
    assert held, 'owned worker lacks exact session lease descriptor'
    with lease.open('rb') as stream:
        try:
            fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            pass
        else:
            fcntl.flock(stream, fcntl.LOCK_UN)
            raise AssertionError('session lease not exclusively held')
    return pid, identity


SETUP = """    candidate_slot = run / 'runtime-candidate'
    atomic_candidate(ARTIFACTS[0], candidate_slot)
"""
CUT = """
        old_pid, old_lease = capture_worker(host, state, sid)
        HANDLES['old'] = os.pidfd_open(old_pid)
        REPORT['before'] = {'worker_pid':old_pid, 'lease':old_lease, 'identity':identity}
        def runtime_command(command, wanted):
            start = len(frames)
            text(a, command, True)
            def result():
                for record in reversed(frames[start:]):
                    event = record.get('event', {})
                    runtime = event.get('Runtime', {})
                    if event.get('Type') == 'runtime' and runtime.get('State') == wanted:
                        return runtime
                return None
            value = wait(result, command + ' runtime ' + wanted)
            keys(a, 'C-a', 'C-k')  # /reload intentionally preserves local draft
            return value
        atomic_candidate(ARTIFACTS[2], candidate_slot)
        rejected = runtime_command('/builds', 'failed')
        assert '999' in rejected['Error'], rejected
        assert rejected['Generation'] == identity[1] and not calls
        assert not select.select([HANDLES['old']], [], [], 0)[0]
        assert capture_worker(host, state, sid) == (old_pid, old_lease)
        snap(a, '00a-incompatible-candidate-rejected')
        atomic_candidate(ARTIFACTS[1], candidate_slot)
        checked = runtime_command('/builds', 'ready')
        assert DIGESTS[0][:12] in checked['RunningBuild'], checked
        assert DIGESTS[1][:12] in checked['CandidateBuild'], checked
        wait(lambda:DIGESTS[0][:12] in screen(a) and DIGESTS[1][:12] in screen(a), 'running/candidate builds visible')
        snap(a, '00b-running-a-candidate-b')
        completed = runtime_command('/reload', 'completed')
        assert completed['Generation'] == identity[1] + 1 and DIGESTS[1][:12] in completed['RunningBuild']
        assert select.select([HANDLES['old']], [], [], 0)[0], 'old process still exists after completed replacement'
        new_pid, new_lease = capture_worker(host, state, sid)
        assert new_pid != old_pid and new_lease == old_lease
        HANDLES['new'] = os.pidfd_open(new_pid)
        assert not select.select([HANDLES['new']], [], [], 0)[0]
        identity = (identity[0], completed['Generation'], identity[2])
        assert not calls and host.poll() is None
        REPORT.update(rejected=rejected, checked=checked, completed=completed,
                      after={'worker_pid':new_pid,'lease':new_lease,'identity':identity},
                      old_exit_observed=True, preflight_unchanged=True)
        snap(a, '00c-runtime-b-ready')
"""
ACTIVE = """
        blocked = runtime_command('/reload', 'blocked')
        assert blocked['Generation'] == identity[1] and len(calls) == 1
        assert not cleanup_gate.is_set() and handler_threads[0].is_alive()
        assert capture_worker(host, state, sid) == (new_pid, new_lease)
        assert not select.select([HANDLES['new']], [], [], 0)[0]
        REPORT['active_model_blocked_without_cancel'] = blocked
        snap(a, '00d-active-model-upgrade-blocked')
"""
FINISH = """        assert select.select([HANDLES['new']], [], [], 0)[0], 'replacement worker not exited after actual host join'
        REPORT.update(passed=True, replacement_worker_exit_after_host_join=True,
                      runner_sha256=DIGESTS, compatible_limit='same source, distinct link buildids',
                      handoff_input_proof='current full-suite workerhost adversarial tests; fixture canonical model/Bash effects unchanged')
        write('runtime-upgrade-result.json', REPORT)
"""


def main():
    global ARTIFACTS, DIGESTS
    assert len(sys.argv) == 6, 'REPO PRIVATE_SHORT_RUN RUNNER_A RUNNER_B BAD_RUNNER'
    repo, run = Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve()
    ARTIFACTS = [Path(p).resolve() for p in sys.argv[3:]]
    DIGESTS = [hashlib.sha256(p.read_bytes()).hexdigest() for p in ARTIFACTS]
    assert len(set(DIGESTS)) == 3
    source_path = repo / '.claude/skills/verify-unreal-agent-tui/scripts/durable_lifecycle.py'
    source = source_path.read_text()
    def replace(old, new):
        nonlocal source
        assert source.count(old) == 1, repr(old)
        source = source.replace(old, new, 1)
    replace("    host_command = [str(runner), 'host',", SETUP + "    host_command = [str(runner), 'host', '-runtime-replacement', '-worker-executable', str(candidate_slot),")
    replace('        identity = assert_connection_identity(a)\n', '        identity = assert_connection_identity(a)\n' + CUT)
    replace("        wait(model_started.is_set, 'blocked remote model request')\n", "        wait(model_started.is_set, 'blocked remote model request')\n" + ACTIVE)
    replace("        write('result.json', {'passed': True,", FINISH + "        write('result.json', {'passed': True,")
    run.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(run, 0o700)
    (run / 'runtime-expanded.py').write_text(source)
    (run / 'runtime-driver-provenance.json').write_text(json.dumps({'base':str(source_path),'base_sha256':hashlib.sha256(source_path.read_bytes()).hexdigest(),'expanded_sha256':hashlib.sha256(source.encode()).hexdigest(),'artifacts':[str(p) for p in ARTIFACTS],'sha256':DIGESTS},indent=2)+'\n')
    sys.argv = [str(source_path), str(repo), str(run)]
    try:
        exec(compile(source, str(run / 'runtime-expanded.py'), 'exec'), globals())
    finally:
        for descriptor in HANDLES.values():
            os.close(descriptor)


if __name__ == '__main__':
    main()
