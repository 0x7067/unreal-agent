"""M3.1 two compatible builds and one deliberately incompatible real frontend.

Usage: python3 -I durable_upgrade.py REPO PRIVATE_SHORT_RUN FRONTEND_A FRONTEND_B BAD_FRONTEND
Uses the unchanged owned lifecycle driver, with explicit executable selection and
an additional incompatible-frontend cut. It never builds, installs or touches a
user frontend. Linux only (owned-process pidfds and /proc child/descriptor proof).
Run in a bounded named user systemd unit. All three binaries must be explicit,
regular executable files with distinct hashes. BAD_FRONTEND must be a separately
built real TUI with an incompatible attach-protocol constant (record overlay).
"""
import fcntl
import hashlib
import json
import os
import pathlib
import select
import stat
import sys


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def observe_worker(host_pid, state, sid):
    threads = list(pathlib.Path(f'/proc/{host_pid}/task').iterdir())
    assert len(threads) <= 256
    children = set()
    for thread in threads:
        try:
            children.update((thread / 'children').read_text().split())
        except FileNotFoundError:
            pass
    assert len(children) == 1, 'expected the one actual owned resident worker'
    pid = int(next(iter(children)))
    lease = state / (sid + '.session.lock')
    inode = (lease.stat().st_dev, lease.stat().st_ino)
    if 'worker_pid' not in REPORT:
        REPORT.update(worker_pid=pid, lease_inode=inode)
        RUNTIME['worker_pidfd'] = os.pidfd_open(pid)
    assert pid == REPORT['worker_pid'] and inode == tuple(REPORT['lease_inode'])
    assert not select.select([RUNTIME['worker_pidfd']], [], [], 0)[0], 'worker exited during frontend upgrade'
    descriptors = list(pathlib.Path(f'/proc/{pid}/fd').iterdir())
    assert len(descriptors) <= 256
    held = False
    for descriptor in descriptors:
        try:
            info = descriptor.stat()
            held |= (info.st_dev, info.st_ino) == inode
        except FileNotFoundError:
            pass
    assert held, 'same owned worker no longer holds the exact session lease descriptor'
    with lease.open('rb') as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            pass
        else:
            fcntl.flock(lock, fcntl.LOCK_UN)
            raise AssertionError('session lease was not exclusively held')
    REPORT['worker_observations'] = REPORT.get('worker_observations', 0) + 1


BAD_CUT = '''
        observe_worker(host.pid, state, sid)
        bad_directory = run / 'frontend-incompatible'
        bad_directory.mkdir(mode=0o700)
        bad_session = 'ul-bad-' + sid[-12:]
        bad_command = [UPGRADE_BINARIES[2]] + command[1:]
        bad_wrapper = PANE_WRAPPER + "\\nimport time\\nend = time.monotonic() + 8\\nwhile not (root / 'release-wrapper').exists() and time.monotonic() < end: time.sleep(.01)\\n"
        wrapped = [sys.executable, '-I', '-c', bad_wrapper, str(bad_directory), json.dumps(bad_command)]
        tmux('new-session', '-d', '-s', bad_session, '-x', '160', '-y', '50',
             '-c', str(workspace), 'exec ' + shlex.join(wrapped))
        bad_pid = int(tmux('display-message', '-p', '-t', bad_session, '#{pane_pid}').stdout)
        bad_pidfd = os.pidfd_open(bad_pid)
        try:
            wait(lambda: (bad_directory / 'joined.json').exists(), 'incompatible real frontend exit', seconds=8)
            receipt = json.loads((bad_directory / 'joined.json').read_text())
            assert receipt['child_waited'] and receipt['returncode'] != 0 and receipt['tty_exact'] and not receipt['forced']
            (bad_directory / 'release-wrapper').touch()
            wait(lambda: bool(select.select([bad_pidfd], [], [], 0)[0]), 'incompatible wrapper pidfd exit', seconds=3)
            REPORT['incompatible_frontend_joined'] = True
            REPORT['incompatible_receipt'] = receipt
            REPORT['incompatible_stderr'] = (bad_directory / 'stderr.txt').read_text()[:4096]
            assert host.poll() is None and len(calls) == 1
            assert_connection_identity(a, identity)
            observe_worker(host.pid, state, sid)
            assert draft_a in screen(a), 'incompatible frontend disturbed current draft'
            snap(a, '01b-incompatible-rejected-original-still-active')
        finally:
            (bad_directory / 'release-wrapper').touch()
            if not (bad_directory / 'joined.json').exists():
                tmux('send-keys', '-t', bad_session, 'C-c', check=False)
                wait(lambda: (bad_directory / 'joined.json').exists(), 'incompatible frontend cleanup join', seconds=5, cleanup=True)
            os.close(bad_pidfd)
            tmux('kill-session', '-t', bad_session, check=False)
'''


def main():
    global REPORT, RUNTIME
    if len(sys.argv) != 6:
        raise SystemExit(__doc__)
    repo, run = [pathlib.Path(p).resolve() for p in sys.argv[1:3]]
    assert run.is_dir() and run.stat().st_uid == os.getuid() and stat.S_IMODE(run.stat().st_mode) == 0o700
    binaries = []
    for value in sys.argv[3:]:
        p = pathlib.Path(value)
        assert not p.is_symlink() and p.is_file() and os.access(p, os.X_OK)
        binaries.append(p.resolve())
    hashes = [digest(p) for p in binaries]
    assert len(set(hashes)) == 3, 'proof requires three distinct real builds'
    REPORT = {'binary_paths': list(map(str, binaries)), 'binary_sha256': hashes, 'passed': False}
    RUNTIME = {}
    base = repo / '.claude/skills/verify-unreal-agent-tui/scripts/durable_lifecycle.py'
    source = base.read_text()
    REPORT['base_driver_sha256'] = digest(base)
    old = 'json.dumps(command)]'
    assert source.count(old) == 1
    source = source.replace(old, "json.dumps([UPGRADE_BINARIES[0 if label == 'frontend-a' else 1]] + command[1:])]")
    anchor = "        snap(a, '01-model-blocked-draft')\n"
    assert source.count(anchor) == 1
    source = source.replace(anchor, anchor + BAD_CUT)
    for label in ('b', 'c'):
        anchor = f'        assert_connection_identity({label}, identity)\n'
        assert source.count(anchor) == 1
        source = source.replace(anchor, anchor + '        observe_worker(host.pid, state, sid)\n')
    (run / 'upgrade-driver.expanded.py').write_text(source)
    REPORT['expanded_driver_sha256'] = hashlib.sha256(source.encode()).hexdigest()
    sys.argv = [str(base), str(repo), str(run)]
    try:
        exec(compile(source, str(base), 'exec'), {'__name__': '__main__', '__file__': str(base),
             'UPGRADE_BINARIES': list(map(str, binaries)), 'REPORT': REPORT,
             'observe_worker': observe_worker})
        assert REPORT.get('incompatible_frontend_joined') and REPORT['worker_observations'] >= 4
        assert hashes == [digest(p) for p in binaries], 'proof binaries changed during execution'
        REPORT['passed'] = True
    finally:
        if 'worker_pidfd' in RUNTIME:
            REPORT['worker_exit_pidfd_observed_after_host_join'] = bool(select.select([RUNTIME['worker_pidfd']], [], [], 0)[0])
            os.close(RUNTIME['worker_pidfd'])
        (run / 'upgrade-result.json').write_text(json.dumps(REPORT, indent=2) + '\n')
    assert REPORT['worker_exit_pidfd_observed_after_host_join']
    print('PASS: distinct compatible frontend builds, incompatible actual TUI refusal, stable exact worker/lease, draft/view and once-only effects')


if __name__ == '__main__':
    main()
