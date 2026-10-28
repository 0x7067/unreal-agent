"""Owned real-tmux lifecycle proof (not a retained-background-agent proof).

Usage: python3 -I durable_lifecycle.py REPO EXISTING_PRIVATE_SHORT_RUN_DIRECTORY
Run under a bounded, named user systemd unit; inspect its cgroup after stop.
This script never builds/installs binaries, opens another session, or uses the
user's tmux server. Success requires canonical outcomes AND actual owner joins.
The first remote HTTP handler deliberately stays blocked until fixture cleanup:
client transport cancellation is NOT evidence about remote provider billing.
"""
import hashlib
import json
import os
import pathlib
import select
import shlex
import signal
import socket
import subprocess
import sys
import threading
import time
import traceback
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

# This wrapper owns and waits for the frontend child; tmux owns the wrapper.
# pidfds below observe the exact wrapper identity obtained from its owned pane.
PANE_WRAPPER = r'''
import json, os, pathlib, signal, subprocess, sys
root = pathlib.Path(sys.argv[1])
command = json.loads(sys.argv[2])
before = subprocess.check_output(['stty', '-g'], text=True)
(root / 'tty-before.txt').write_text(before)
p = None
forced = False
def terminate(signum, frame):
    global forced
    forced = True
    if p is not None and p.poll() is None:
        p.send_signal(signal.SIGINT)
signal.signal(signal.SIGTERM, terminate)
try:
    with (root / 'stderr.txt').open('w') as err:
        p = subprocess.Popen(command, stderr=err)
        (root / 'child.json').write_text(json.dumps({'pid': p.pid, 'wrapper': os.getpid()}))
        code = p.wait()
finally:
    after = subprocess.check_output(['stty', '-g'], text=True)
    (root / 'tty-after.txt').write_text(after)
    (root / 'joined.json').write_text(json.dumps({
        'returncode': p.returncode if p is not None else None,
        'child_waited': p is not None and p.returncode is not None,
        'tty_exact': before == after, 'forced': forced}))
'''


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    repo, root = (pathlib.Path(x).resolve() for x in sys.argv[1:])
    assert root.is_dir() and root.stat().st_mode & 0o7777 == 0o700
    assert root.stat().st_uid == os.getuid()
    os.umask(0o077)
    os.environ.pop('TMUX', None)
    os.environ['TMUX_TMPDIR'] = str(root)
    # Exclusive subdirectory rejects accidental reuse of an earlier proof.
    run = root / 'lifecycle'
    run.mkdir(mode=0o700)
    for name in ('state', 'workspace', 'clients', 'front', 'back', 'evidence', 'config', 'home'):
        (run / name).mkdir(mode=0o700)
    evidence = run / 'evidence'
    state, workspace = run / 'state', run / 'workspace'
    sid = 'lifecycle-' + hashlib.sha256(str(root).encode()).hexdigest()[:12]
    endpoint_name = hashlib.sha256((str(state) + '\0' + sid).encode()).hexdigest() + '.sock'
    front_endpoint, back_endpoint = run / 'front' / endpoint_name, run / 'back' / endpoint_name
    assert max(len(str(front_endpoint)), len(str(back_endpoint))) < 104, 'use a shorter private run directory'
    tmux_socket = root / 'lifecycle-tmux.sock'
    runner, tui = repo / 'bin/unreal-agent-runner', repo / 'bin/unreal-agent-tui'
    command = [str(tui), 'attach', '-runtime-directory', str(run / 'front'),
               '-session-directory', str(state), '-session', sid,
               '-client-state-directory', str(run / 'clients')]
    host_command = [str(runner), 'host', '-runtime-directory', str(run / 'back'),
                    '-session-directory', str(state), '-session', sid, '-workspace', str(workspace),
                    '-provider', 'openai', '-model', 'fake']
    env = os.environ | {'HOME': str(run / 'home'), 'XDG_STATE_HOME': str(state),
                        'XDG_CONFIG_HOME': str(run / 'config'), 'XDG_RUNTIME_DIR': str(run / 'back'),
                        'OPENAI_API_KEY': 'fake', 'UNREAL_HARNESS_LLM_API_KEY': 'fake'}
    started = time.monotonic()
    proof_deadline = started + 48  # leave bounded cleanup time within the enclosing unit
    cleanup_gate = threading.Event()
    proxy_stop = threading.Event()
    model_started = threading.Event()
    calls, handler_threads, proxy_threads, sockets, frames, frontends, errors = [], [], [], [], [], [], []
    lock = threading.Lock()
    host = tmux_server = provider = acceptor = server = listener = None
    success = False
    cleanup_errors = []
    outcomes = []

    def write(name, value):
        (evidence / name).write_text(json.dumps(value, indent=2) + '\n')

    def wait(fn, label, seconds=7, cleanup=False):
        end = time.monotonic() + seconds
        if not cleanup:
            end = min(end, proof_deadline)
        while time.monotonic() < end:
            value = fn()
            if value:
                return value
            time.sleep(.035)
        raise AssertionError('deadline: ' + label)

    def tmux(*args, check=True):
        return subprocess.run(['tmux', '-S', str(tmux_socket), *args], env=env,
                              capture_output=True, text=True, check=check, timeout=2)

    def screen(front):
        return tmux('capture-pane', '-p', '-t', front['session'], check=False).stdout

    def snap(front, name):
        text = screen(front)
        (evidence / (name + '.txt')).write_text(text)
        return text

    def keys(front, *args):
        tmux('send-keys', '-t', front['session'], *args)

    def text(front, value, submit=False):
        tmux('send-keys', '-t', front['session'], '-l', value)
        if submit:
            keys(front, 'Enter')

    def canonical_items():
        path = state / (sid + '.session.jsonl')
        if not path.exists():
            return []
        data = path.read_bytes()
        # Only newline-committed records; never parse a concurrent partial tail.
        records = [json.loads(line) for line in data[:data.rfind(b'\n') + 1].splitlines()]
        return [r['data']['Item'] for r in records if r['type'] == 'item']

    def controls():
        files = list((run / 'clients').glob('*.json'))
        if not files:
            return []
        assert len(files) == 1
        saved = json.loads(files[0].read_text())
        assert saved['Version'] == 2
        return saved.get('Controls', [])

    def outcome(control, mode, phase):
        items = canonical_items()
        intents = [i for i in items if i['Kind'] == 'input' and i['Data'].get('ID') == control['ID']]
        assert len(intents) <= 1, 'duplicate canonical control input'
        if not intents:
            return None
        assert intents[0]['Data'] == control['Input'], 'original saved control changed on wire'
        for item in items:
            if item['Kind'] != 'input':
                continue
            payload = item['Data'].get('Payload', {})
            if not isinstance(payload, dict) or payload.get('Mode') != mode:
                continue
            result = payload['Parameters']
            if result.get('InputID') != control['ID']:
                continue
            if mode == 'foreground_abort_finished':
                assert result['TargetTurnID'] == control['TargetTurnID']
                complete = result['Status'] == phase
            else:
                assert result['Scope'] == control['Scope']
                complete = result['Phase'] == phase
            if complete:
                assert not result.get('Error'), result
                return {'control': control, 'canonical_item': item}
        return None

    def abort(front, number):
        old = len(controls())
        text(front, '/abort', True)
        control = wait(lambda: controls()[-1] if len(controls()) == old + 1 else None,
                       'saved abort identity')
        assert control['TargetTurnID'] and control['Scope']
        result = wait(lambda: outcome(control, 'foreground_abort_finished', 'completed'),
                      'canonical foreground abort completed')
        outcomes.append(result)
        wait(lambda: control['ID'] in screen(front) and 'completed' in screen(front),
             'frontend canonical abort outcome')
        snap(front, number + '-abort-completed')
        return control

    def launch(label):
        directory = run / label
        directory.mkdir(mode=0o700)
        front = {'session': 'ul-' + sid[-12:] + '-' + label, 'directory': directory,
                 'pidfd': None, 'joined': False}
        frontends.append(front)
        wrapped = [sys.executable, '-I', '-c', PANE_WRAPPER, str(directory), json.dumps(command)]
        tmux('new-session', '-d', '-s', front['session'], '-x', '160', '-y', '50',
             '-c', str(workspace), 'exec ' + shlex.join(wrapped))
        pane_pid = int(tmux('display-message', '-p', '-t', front['session'], '#{pane_pid}').stdout)
        front['pane_pid'] = pane_pid
        front['pidfd'] = os.pidfd_open(pane_pid)
        wait(lambda: 'Unreal Agent · attached ·' in screen(front), 'frontend ' + label + ' attached')
        hello = wait(lambda: next((f for f in reversed(frames)
                                  if f['event'].get('Type') == 'snapshot'), None), 'snapshot frame')
        front['snapshot_connection'] = hello['connection']
        return front

    def join_front(front, cleanup=False):
        joined = front['directory'] / 'joined.json'
        wait(joined.exists, 'frontend child wait receipt', seconds=5, cleanup=cleanup)
        receipt = json.loads(joined.read_text())
        assert receipt['child_waited'] and receipt['returncode'] == 0 and receipt['tty_exact']
        assert not receipt['forced'], 'forced frontend cleanup is not successful detach proof'
        wait(lambda: bool(select.select([front['pidfd']], [], [], 0)[0]),
             'exact pane wrapper process exit', seconds=3, cleanup=cleanup)
        front['joined'] = True
        write(front['directory'].name + '-tty-join.json', receipt | {
            'pane_pid': front['pane_pid'], 'wrapper_exit_pidfd_observed': True})

    def detach(front):
        keys(front, 'C-c')
        join_front(front)
        assert host.poll() is None, 'detach terminated host'
        assert len(calls) == (1 if front['directory'].name == 'frontend-a' else 2)

    def assert_connection_identity(front, expected=None):
        connection = front['snapshot_connection']
        events = [f['event'] for f in frames if f['connection'] == connection]
        hello = next(e for e in events if e['Type'] == 'hello')
        snapshot = next(e for e in events if e['Type'] == 'snapshot')
        identity = (hello['Incarnation'], snapshot['Status']['Generation'], hello['HostPID'])
        assert identity[0] and identity[1] > 0 and identity[2] == host.pid
        if expected is not None:
            assert identity == expected, 'host incarnation/generation changed on detach'
        write(front['directory'].name + '-identity.json', {'hello': hello, 'snapshot': snapshot})
        return identity

    class Model(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            with lock:
                handler_threads.append(threading.current_thread())
            body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            with lock:
                calls.append(body)
                number = len(calls)
            if number == 1:
                model_started.set()
                cleanup_gate.wait()  # intentionally only fixture cleanup releases the remote owner
                output = []
            elif number == 2:
                shell = ('printf x >> ' + shlex.quote(str(workspace / 'sentinel')) +
                         '; printf started > ' + shlex.quote(str(workspace / 'bash-started')) +
                         '; while [ ! -e ' + shlex.quote(str(workspace / 'bash-gate')) +
                         ' ]; do sleep 0.05; done; printf LATE > ' +
                         shlex.quote(str(workspace / 'late-tool')))
                output = [{'type': 'function_call', 'id': 'bash2', 'call_id': 'lifecycle-bash',
                           'name': 'Bash', 'arguments': json.dumps({'command': shell})}]
            else:
                errors.append('unexpected provider call ' + str(number))
                output = [{'type': 'message', 'id': 'unexpected', 'role': 'assistant',
                           'status': 'completed', 'content': [{'type': 'output_text', 'text': 'UNEXPECTED_LATE_MODEL'}]}]
            event = {'type': 'response.completed', 'response': {
                'id': str(number), 'status': 'completed', 'output': output,
                'usage': {'input_tokens': 1, 'output_tokens': 1}}}
            try:
                self.send_response(200)
                self.send_header('Content-Type', 'text/event-stream')
                self.end_headers()
                self.wfile.write(('data: ' + json.dumps(event) + '\n\n').encode())
            except (BrokenPipeError, ConnectionResetError):
                pass

    # A transparent owned relay records *actual frontend* hello/status identities
    # without taking a competing attach lease or issuing any control itself.
    def relay(client, index):
        backend = socket.socket(socket.AF_UNIX)
        with lock:
            sockets.extend([client, backend])
        try:
            backend.settimeout(2)
            backend.connect(str(back_endpoint))
            backend.setblocking(False)
            client.setblocking(False)
            buffers = {client: bytearray(), backend: bytearray()}
            wire = bytearray()
            while not proxy_stop.is_set():
                readable, writable, _ = select.select([client, backend],
                    [s for s in (client, backend) if buffers[s]], [], .05)
                for source in readable:
                    try:
                        data = source.recv(65536)
                    except BlockingIOError:
                        continue
                    if not data:
                        return
                    target = backend if source is client else client
                    buffers[target].extend(data)
                    assert len(buffers[target]) <= 8 << 20, 'relay queue exceeded bound'
                    if source is backend:
                        wire.extend(data)
                        assert len(wire) <= 1 << 20, 'relay frame exceeded bound'
                        while b'\n' in wire:
                            line, _, remaining = wire.partition(b'\n')
                            wire[:] = remaining
                            frames.append({'connection': index, 'event': json.loads(line)})
                for target in writable:
                    try:
                        sent = target.send(buffers[target])
                        del buffers[target][:sent]
                    except BlockingIOError:
                        pass
        except (BrokenPipeError, ConnectionResetError):
            pass
        except OSError as exc:
            if not proxy_stop.is_set():
                errors.append('relay: ' + repr(exc))
        except BaseException as exc:
            errors.append('relay: ' + repr(exc))
        finally:
            client.close()
            backend.close()

    def accept():
        while not proxy_stop.is_set():
            try:
                client, _ = listener.accept()
            except socket.timeout:
                continue
            except OSError:
                return
            thread = threading.Thread(target=relay, args=(client, len(proxy_threads)), name='lifecycle-relay')
            proxy_threads.append(thread)
            thread.start()

    def attempt(label, fn):
        try:
            fn()
        except BaseException:
            cleanup_errors.append(label + ': ' + traceback.format_exc())

    try:
        server = ThreadingHTTPServer(('127.0.0.1', 0), Model)
        server.daemon_threads = False
        provider = threading.Thread(target=server.serve_forever, kwargs={'poll_interval': .05}, name='lifecycle-http')
        provider.start()
        listener = socket.socket(socket.AF_UNIX)
        listener.bind(str(front_endpoint))
        front_endpoint.chmod(0o600)  # The real SDK requires a private 0600 socket.
        listener.listen(4)
        listener.settimeout(.1)
        acceptor = threading.Thread(target=accept, name='lifecycle-accept')
        acceptor.start()
        tmux_server = subprocess.Popen(['tmux', '-D', '-S', str(tmux_socket), '-f', '/dev/null'],
            env=env, stdout=(run / 'tmux-stdout.txt').open('w'), stderr=(run / 'tmux-stderr.txt').open('w'))
        wait(lambda: tmux_socket.exists(), 'owned foreground tmux server')
        host = subprocess.Popen(host_command + ['-base-url', f'http://127.0.0.1:{server.server_port}'],
            env=env, stdout=(run / 'host-stdout.txt').open('w'), stderr=(run / 'host-stderr.txt').open('w'))
        wait(back_endpoint.exists, 'host endpoint ready')
        a = launch('frontend-a')
        identity = assert_connection_identity(a)
        text(a, 'LIFECYCLE_MODEL_BLOCKED', True)
        wait(model_started.is_set, 'blocked remote model request')
        draft_a = 'UNSENT_MODEL_DRAFT'
        text(a, draft_a)
        wait(lambda: draft_a in screen(a), 'draft A')
        snap(a, '01-model-blocked-draft')
        detach(a)
        assert not cleanup_gate.is_set() and handler_threads[0].is_alive() and len(calls) == 1
        saved = json.loads(next((run / 'clients').glob('*.json')).read_text())
        assert saved['Draft'] == draft_a
        write('model-detach.json', {'host_alive': host.poll() is None, 'remote_handler_alive': handler_threads[0].is_alive(),
                                    'provider_calls': len(calls), 'saved': saved})
        b = launch('frontend-b')
        assert_connection_identity(b, identity)
        wait(lambda: draft_a in screen(b), 'replacement B restored draft')
        snap(b, '02-model-replacement-draft')
        assert len(calls) == 1
        keys(b, 'C-a', 'C-k')
        abort(b, '03-model')
        assert len(calls) == 1 and handler_threads[0].is_alive() and not cleanup_gate.is_set()
        text(b, 'LIFECYCLE_BASH_BLOCKED', True)
        wait(lambda: (workspace / 'bash-started').exists(), 'Bash started and blocked')
        assert (workspace / 'sentinel').read_text() == 'x' and len(calls) == 2
        draft_b = 'UNSENT_BASH_DRAFT'
        text(b, draft_b)
        wait(lambda: draft_b in screen(b), 'draft B')
        snap(b, '04-bash-blocked-draft')
        detach(b)
        assert not (workspace / 'bash-gate').exists() and not (workspace / 'late-tool').exists()
        c = launch('frontend-c')
        assert_connection_identity(c, identity)
        wait(lambda: draft_b in screen(c), 'replacement C restored Bash-phase draft')
        snap(c, '05-bash-replacement-draft')
        keys(c, 'C-a', 'C-k')
        abort(c, '06-bash')
        assert len(calls) == 2 and (workspace / 'sentinel').read_text() == 'x'
        assert not (workspace / 'late-tool').exists() and not (workspace / 'bash-gate').exists()
        old = len(controls())
        text(c, '/shutdown', True)
        shutdown = wait(lambda: controls()[-1] if len(controls()) == old + 1 else None, 'saved shutdown UUID')
        assert shutdown['TargetTurnID'] == '' and shutdown['Scope'] == outcomes[0]['control']['Scope']
        result = wait(lambda: outcome(shutdown, 'session_shutdown_finished', 'completed'), 'canonical shutdown completed')
        outcomes.append(result)
        assert host.wait(timeout=5) == 0, 'actual host process must exit zero'
        wait(lambda: not back_endpoint.exists(), 'actual host endpoint gone')
        snap(c, '07-shutdown-final-state')
        text(c, '/retry-control ' + shutdown['ID'], True)
        wait(lambda: shutdown['ID'] in screen(c) and 'completed' in screen(c), 'disconnected read-only reconciliation')
        snap(c, '08-disconnected-canonical-reconciliation')
        keys(c, 'C-c')
        join_front(c)
        # Remove the relay endpoint as well, so startup truly encounters ENOENT,
        # not a fixture listener accepting and then failing backend connect.
        proxy_stop.set()
        listener.close()
        acceptor.join(2)
        for thread in proxy_threads:
            thread.join(2)
        front_endpoint.unlink(missing_ok=True)
        canonical = state / (sid + '.session.jsonl')
        before = canonical.read_bytes()
        offline = subprocess.run(command, env=env, stdin=subprocess.DEVNULL,
                                 capture_output=True, text=True, timeout=5, cwd=workspace)
        (evidence / '09-offline-reconciliation.txt').write_text(offline.stdout)
        (evidence / '09-offline-stderr.txt').write_text(offline.stderr)
        assert offline.returncode == 0, offline.stderr
        assert 'read-only canonical lifecycle reconciliation' in offline.stdout
        assert 'Lifecycle ' + shutdown['ID'] + ' · completed' in offline.stdout
        assert canonical.read_bytes() == before, 'offline reconciliation mutated canonical state'
        assert not back_endpoint.exists() and len(calls) == 2 and not errors
        assert (workspace / 'sentinel').read_text() == 'x' and not (workspace / 'late-tool').exists()
        write('outcomes.json', outcomes)
        write('result.json', {'passed': True, 'host_returncode': host.returncode,
            'identity': identity, 'provider_calls': len(calls), 'sentinel': 'x',
            'no_late_tool': True, 'offline_returncode': offline.returncode,
            'canonical_unchanged_offline': True, 'elapsed_seconds': time.monotonic() - started,
            'remote_billing_not_proven': True, 'retained_background_agent_proof': 'separate Go integration helper'})
        success = True
    except BaseException:
        (evidence / 'failure.txt').write_text(traceback.format_exc())
        for front in frontends:
            attempt('failure screenshot', lambda front=front: snap(front, front['directory'].name + '-FAILURE'))
    finally:
        # Fixture release is not a runtime-abort proof; only the completed path
        # above asserts sentinel stability while the Bash gate is still closed.
        cleanup_gate.set()
        if not success:
            (workspace / 'bash-gate').touch()
        for front in frontends:
            if front['joined']:
                continue
            attempt('frontend Ctrl+C', lambda front=front: keys(front, 'C-c'))
            attempt('frontend join', lambda front=front: join_front(front, cleanup=True))
            if not front['joined'] and front['pidfd'] is not None:
                attempt('owned wrapper TERM', lambda front=front: signal.pidfd_send_signal(front['pidfd'], signal.SIGTERM))
                attempt('wrapper exit', lambda front=front: wait(
                    lambda: bool(select.select([front['pidfd']], [], [], 0)[0]), 'wrapper cleanup exit', 3, cleanup=True))
        if host is not None and host.poll() is None:
            attempt('host interrupt', lambda: host.send_signal(signal.SIGINT))
            attempt('host wait', lambda: host.wait(timeout=6))
        proxy_stop.set()
        if listener is not None:
            listener.close()
        if acceptor is not None:
            acceptor.join(2)
        for connection in sockets:
            attempt('relay socket close', connection.close)
        for thread in proxy_threads:
            thread.join(2)
        if server is not None:
            server.shutdown()
            # With daemon_threads=False, server_close joins all owned request
            # handlers. Release gate precedes this; no HTTP thread is abandoned.
            server.server_close()
        if provider is not None:
            provider.join(2)
        for thread in handler_threads:
            thread.join(2)
        if tmux_server is not None and tmux_server.poll() is None:
            attempt('tmux exit-empty', lambda: tmux('set-option', '-g', 'exit-empty', 'on'))
            attempt('tmux server wait', lambda: tmux_server.wait(timeout=3))
        for front in frontends:
            if front['pidfd'] is not None:
                os.close(front['pidfd'])
        write('provider-calls.json', calls)
        write('frontend-frames.json', frames)
        write('outcomes.json', outcomes)
        write('cleanup-errors.json', cleanup_errors)
        cleanup = {
            'proof_completed': success,
            'host_joined_zero': host is not None and host.returncode == 0,
            'host_endpoint_gone': not back_endpoint.exists(),
            'frontends_waited_zero_and_tty_exact': bool(frontends) and all(f['joined'] for f in frontends),
            'relay_acceptor_joined': acceptor is not None and not acceptor.is_alive(),
            'relay_watchers_joined': all(not t.is_alive() for t in proxy_threads),
            'http_server_joined': provider is not None and not provider.is_alive(),
            'http_handlers_joined': all(not t.is_alive() for t in handler_threads),
            'tmux_server_waited_zero': tmux_server is not None and tmux_server.returncode == 0,
            'provider_calls_exactly_two': len(calls) == 2,
            'no_late_tool': not (workspace / 'late-tool').exists(),
            'sentinel_exactly_once': (workspace / 'sentinel').exists() and (workspace / 'sentinel').read_text() == 'x',
            'no_fixture_errors': not errors and not cleanup_errors,
        }
        write('cleanup.json', cleanup)
        write('fixture-errors.json', errors)
    if not all(cleanup.values()):
        raise SystemExit('FAIL: owned lifecycle proof/cleanup incomplete; inspect ' + str(evidence))
    print('PASS: model/Bash frontend replacement, canonical abort/shutdown, host wait0, offline reconciliation, exact TTY restoration and owned joins; ' + str(evidence))


if __name__ == '__main__':
    main()
