#!/usr/bin/env python3
"""M3.4 owned TUI: compatible probe, failed candidate, stopped host, explicit restart.
Arguments REPO PRIVATE_SHORT_RUN RUNNER_A HEALTHY_B STARTUP_FAIL_B.
"""
import importlib.util
from pathlib import Path
import sys

CUT = '''
        old_pid,old_lease=capture_worker(host,state,sid)
        HANDLES['old']=os.pidfd_open(old_pid)
        before_identity=identity
        def runtime_command(command,wanted):
            start=len(frames)
            text(a,command,True)
            def result():
                for record in reversed(frames[start:]):
                    event=record.get('event',{})
                    if event.get('Type')=='runtime' and event.get('Runtime',{}).get('State')==wanted:
                        return event['Runtime']
            value=wait(result,command+' '+wanted)
            keys(a,'C-a','C-k')
            return value
        atomic_candidate(ARTIFACTS[2],candidate_slot)
        checked=runtime_command('/builds','ready')
        assert DIGESTS[0][:12] in checked['RunningBuild'] and DIGESTS[2][:12] in checked['CandidateBuild']
        snap(a,'m34-compatible-probe-before-startup-failure')
        stopped=runtime_command('/reload','stopped')
        assert stopped['State']!='completed' and stopped['Error'],stopped
        wait(lambda:'M34 injected candidate startup failure' in (run/'host-stderr.txt').read_text(),'actual injected candidate startup fault')
        assert select.select([HANDLES['old']],[],[],0)[0]
        assert host.poll() is None and not calls
        wait(lambda:'stopped' in screen(a).lower(),'stopped failure visible')
        snap(a,'m34-stopped-no-automatic-fallback')
        with (state/(sid+'.session.lock')).open('rb') as stream:
            fcntl.flock(stream,fcntl.LOCK_EX|fcntl.LOCK_NB)
            fcntl.flock(stream,fcntl.LOCK_UN)
        failed_host_pid=host.pid
        failed_host_pidfd=os.pidfd_open(host.pid)
        def detach(front):
            keys(front,'C-c')
            join_front(front)
            assert host.poll() is None,'detach terminated host'
            expected=0 if front['directory'].name=='frontend-a' else (1 if front['directory'].name=='frontend-recovered' else 2)
            assert len(calls)==expected
        detach(a)
        host.terminate()  # only this exact owned subprocess; explicit operator recovery
        failed_returncode=host.wait(timeout=8)
        assert failed_returncode!=0 and select.select([failed_host_pidfd],[],[],0)[0]
        os.close(failed_host_pidfd)
        assert not back_endpoint.exists()
        atomic_candidate(ARTIFACTS[1],candidate_slot)
        host=subprocess.Popen(host_command+['-base-url',f'http://127.0.0.1:{server.server_port}'],env=env,
            stdout=(run/'m34-recovery-host-stdout.txt').open('w'),stderr=(run/'m34-recovery-host-stderr.txt').open('w'))
        wait(back_endpoint.exists,'explicit healthy resident restart')
        a=launch('frontend-recovered')
        identity=assert_connection_identity(a)
        assert identity[0]!=before_identity[0] and identity[2]!=failed_host_pid and identity[1]==1
        new_pid,new_lease=capture_worker(host,state,sid)
        assert new_pid!=old_pid and new_lease==old_lease
        HANDLES['new']=os.pidfd_open(new_pid)
        REPORT.update(before={'worker_pid':old_pid,'lease':old_lease,'identity':before_identity},
            after={'worker_pid':new_pid,'lease':new_lease,'identity':identity},checked=checked,stopped=stopped,
            old_exit_observed=True,no_automatic_fallback=True,failed_host_pid=failed_host_pid,
            failed_host_actual_wait_returncode=failed_returncode,explicit_recovery_host_pid=host.pid)
        snap(a,'m34-explicit-healthy-recovery-new-incarnation')
'''


def main():
    assert len(sys.argv)==6,'REPO PRIVATE_SHORT_RUN RUNNER_A HEALTHY_B STARTUP_FAIL_B'
    source=Path(sys.argv[1]).resolve()/'.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_upgrade.py'
    spec=importlib.util.spec_from_file_location('owned_runtime_failure',source)
    module=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.CUT=CUT
    module.FINISH=module.FINISH.replace("write('runtime-upgrade-result.json', REPORT)","write('runtime-failure-result.json', REPORT)")
    module.__dict__['__name__']='__main__'
    module.main()


if __name__=='__main__':
    main()
