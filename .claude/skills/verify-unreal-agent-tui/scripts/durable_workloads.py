#!/usr/bin/env python3
"""Owned M4 remote-job frontend/runtime replacement, followed by baseline lifecycle.
Usage: durable_workloads.py REPO PRIVATE_SHORT_0700_ROOT. No builds or installs.
Only private tmux, host, executor and configured /bin/sh job are controlled.
"""
import hashlib
from pathlib import Path
import sys

SETUP = r'''
    remote_process = None
    remote_worker_fd = None
    remote_report = {}
    remote_threads = []
'''
PREPARE = r'''
        import io, fcntl
        remote_work = run / 'remote-work'
        remote_work.mkdir(mode=0o700)
        remote_token = 'owned-m45-secret-not-in-canonical-or-snapshot'
        cert_file, key_file = run/'remote-cert.pem', run/'remote-key.pem'
        subprocess.run(['openssl','req','-x509','-newkey','ed25519','-nodes','-days','1',
            '-subj','/CN=executor.test','-addext','subjectAltName=DNS:executor.test',
            '-keyout',str(key_file),'-out',str(cert_file)],check=True,
            stdout=subprocess.DEVNULL,stderr=(run/'cert-stderr.txt').open('w'))
        cert_file.chmod(0o600);key_file.chmod(0o600)
        remote_effect, remote_gate = remote_work/'effect', remote_work/'release'
        remote_command = 'test -z "$UNREAL_AGENT_EXECUTOR_TOKEN" || exit 99; printf x >> '+shlex.quote(str(remote_effect))+'; printf first; while [ ! -e '+shlex.quote(str(remote_gate))+' ]; do /bin/sleep 0.01; done; printf second'
        executor_config = run/'remote-executor.json'
        executor_config.write_text(json.dumps({'Version':1,'Directory':str(run/'executor'),
            'EnvironmentID':'owned-remote','ControllerIDs':['session-'+hashlib.sha256(sid.encode()).hexdigest()[:32]],
            'TLSAddress':'127.0.0.1:0','CertFile':str(cert_file),'KeyFile':str(key_file),
            'Jobs':{'tests':{'Path':'/bin/sh','Args':['-c',remote_command],
            'Dir':str(remote_work),'Env':['PATH=/usr/bin:/bin']}}}))
        executor_config.chmod(0o600)
        env['UNREAL_AGENT_EXECUTOR_TOKEN']=remote_token
        remote_process = subprocess.Popen([str(runner),'job-executor','--config',str(executor_config)],env=env,
            stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=(run/'executor-stderr.txt').open('w'))
        assert select.select([remote_process.stdout],[],[],5)[0], 'executor readiness timeout'
        remote_ready=json.loads(remote_process.stdout.readline())
        assert remote_ready['PID']==remote_process.pid and remote_ready['Version']==1 and remote_ready['EnvironmentID']=='owned-remote'
        job_config=run/'remote-jobs.json'
        job_config.write_text(json.dumps({'Version':1,'EnvironmentID':'owned-remote','TLSAddress':remote_ready['TLSAddress'],
            'TLSServerName':'executor.test','CAFile':str(cert_file),'Jobs':['tests']}));job_config.chmod(0o600)
        env['UNREAL_AGENT_JOB_CONFIG']=str(job_config)
        extension_file=run/'extensions.json'
        extension_file.write_text(json.dumps({'Version':1,'Known':[{'Name':'Echo','Generation':1,'SchemaVersion':1,'ImplementationVersion':1,'Implementation':'A'}],'Selected':1}));extension_file.chmod(0o600)
        env['UNREAL_AGENT_EXTENSION_CONFIG']=str(extension_file)
        env['UNREAL_AGENT_DURABLE_TIMERS']='1'

        # Selection metadata is not model-visible: fixed schema, recorded plans pin meaning.
        M4={'phase':'armed','calls':0}
        original_post=Model.do_POST
        def remote_post(request):
            raw=request.rfile.read(int(request.headers['Content-Length']))
            if (M4['phase']=='armed' and b'M45_REMOTE' in raw) or M4['phase']=='running':
                remote_threads.append(threading.current_thread());M4['calls']+=1
                if M4['phase']=='armed':
                    M4['phase']='running'
                    output=[{'type':'function_call','id':'remote-call','call_id':'remote-call','name':'RunTrustedJob','arguments':'{"job":"tests"}'}]
                else:
                    assert b'firstsecond' in raw,'provider resumed before terminal remote output'
                    M4['phase']='done'
                    output=[{'type':'message','id':'remote-finished','role':'assistant','content':[{'type':'output_text','text':'M45_REMOTE_FINISHED firstsecond'}]}]
                event={'type':'response.completed','response':{'id':'m45-'+str(M4['calls']),'status':'completed','output':output,'usage':{'input_tokens':1,'output_tokens':1}}}
                request.send_response(200);request.send_header('Content-Type','text/event-stream');request.end_headers()
                request.wfile.write(('data: '+json.dumps(event)+'\n\n').encode())
            else:
                request.rfile=io.BytesIO(raw);original_post(request)
        Model.do_POST=remote_post
        AUX={'phase':'echo-armed','calls':0}
        def auxiliary_post(request):
            import datetime
            raw=request.rfile.read(int(request.headers['Content-Length']))
            phase=AUX['phase']
            echo=(phase=='echo-armed' and b'M41_ECHO_UI' in raw) or phase=='echo-running'
            timer=(phase=='timer-armed' and b'M44_TIMER_UI' in raw) or phase=='timer-running'
            if echo or timer:
                remote_threads.append(threading.current_thread());AUX['calls']+=1
                if phase=='echo-armed':
                    AUX['phase']='echo-running'
                    output=[{'type':'function_call','id':'echo-ui','call_id':'echo-ui','name':'TrustedEcho','arguments':'{"input":"UI_ECHO_INPUT"}'}]
                elif phase=='echo-running':
                    assert b'A:UI_ECHO_INPUT' in raw
                    AUX['phase']='echo-done'
                    output=[{'type':'message','id':'echo-ui-done','role':'assistant','content':[{'type':'output_text','text':'UI_ECHO_A_FINISHED A:UI_ECHO_INPUT'}]}]
                elif phase=='timer-armed':
                    AUX['phase']='timer-running'
                    due=(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(seconds=1)).isoformat().replace('+00:00','Z')
                    output=[{'type':'function_call','id':'timer-ui','call_id':'timer-ui','name':'DurableReminder','arguments':json.dumps({'due_at':due,'message':'UI_TIMER_NOTIFICATION','background':True})}]
                else:
                    assert b'UI_TIMER_NOTIFICATION' in raw
                    AUX['phase']='done'
                    output=[{'type':'message','id':'timer-ui-done','role':'assistant','content':[{'type':'output_text','text':'UI_TIMER_FINISHED UI_TIMER_NOTIFICATION'}]}]
                event={'type':'response.completed','response':{'id':'aux-'+str(AUX['calls']),'status':'completed','output':output,'usage':{'input_tokens':1,'output_tokens':1}}}
                request.send_response(200);request.send_header('Content-Type','text/event-stream');request.end_headers()
                request.wfile.write(('data: '+json.dumps(event)+'\n\n').encode())
            else:
                request.rfile=io.BytesIO(raw);remote_post(request)
        Model.do_POST=auxiliary_post

        def remote_operation(plan_type='durable_executor_job'):
            file=state/(sid+'.session.jsonl')
            if not file.exists():return None
            data=file.read_bytes();found=[]
            def walk(value):
                if isinstance(value,dict):
                    if value.get('Type')=='remote_job' and value.get('State',{}).get('Plan',{}).get('Type')==plan_type:found.append(value)
                    for item in value.values():walk(item)
                elif isinstance(value,list):
                    for item in value:walk(item)
            for line in data[:data.rfind(b'\n')+1].splitlines():walk(json.loads(line))
            return found[-1] if found else None
        def exact_worker(owner):
            children=set()
            tasks=list(pathlib.Path('/proc/'+str(owner.pid)+'/task').iterdir());assert 0<len(tasks)<=256
            for task in tasks:
                try:children.update((task/'children').read_text().split())
                except FileNotFoundError:pass
            assert 0<len(children)<=16
            lease=state/(sid+'.session.lock');st=lease.stat();identity=(st.st_dev,st.st_ino);holders=[]
            for child in children:
                try:fds=list(pathlib.Path('/proc/'+child+'/fd').iterdir())
                except FileNotFoundError:continue
                assert len(fds)<=512
                held=False
                for fd in fds:
                    try:st=fd.stat();held |= (st.st_dev,st.st_ino)==identity
                    except FileNotFoundError:pass
                if held:holders.append(int(child))
            assert len(holders)==1,holders
            with lease.open('r+') as probe:
                try:fcntl.flock(probe,fcntl.LOCK_EX|fcntl.LOCK_NB)
                except BlockingIOError:pass
                else:raise AssertionError('observed lease is not held')
            return holders[0]
'''
SCENARIO = r'''

        text(a,'M41_ECHO_UI',True)
        wait(lambda:'UI_ECHO_A_FINISHED' in screen(a),'compiled Echo tool through actual coordinator')
        echo=remote_operation('trusted_echo')
        assert echo['Status']=='completed' and echo['State']['TerminalResult']=='A:UI_ECHO_INPUT'
        assert echo['State']['Plan']['Data']['Generation']==1 and echo['State']['Plan']['Data']['OwnerTurnID']
        snap(a,'m4-00-compiled-echo-tool')
        AUX['phase']='timer-armed'
        text(a,'M44_TIMER_UI',True)
        wait(lambda:'UI_TIMER_FINISHED' in screen(a),'durable reminder tool canonical notification')
        timer=remote_operation('durable_timer')
        assert timer['Status']=='completed'
        notification=json.loads(timer['State']['TerminalResult'])
        assert notification['message']=='UI_TIMER_NOTIFICATION' and notification['stableNotificationID']
        assert AUX['calls']==4
        snap(a,'m4-00-durable-reminder-tool')
        text(a,'M45_REMOTE',True)
        wait(lambda:remote_effect.exists() and remote_effect.read_text()=='x','remote effect started')
        original_operation=wait(remote_operation,'remote committed operation')['ID']
        snap(a,'m45-01-remote-running')
        old_identity=identity
        old_pid=exact_worker(host)
        remote_worker_fd=os.pidfd_open(old_pid)
        a['expected_provider_calls']=0
        detach(a)
        assert remote_process.poll() is None and not remote_gate.exists() and remote_effect.read_text()=='x'
        # Exact observed private worker, never a guessed PID or a user worker.
        signal.pidfd_send_signal(remote_worker_fd,signal.SIGKILL)
        wait(lambda:bool(select.select([remote_worker_fd],[],[],0)[0]),'old owned runtime pidfd exit')
        old_host_exit=host.wait(timeout=8)
        assert old_host_exit!=0 and remote_process.poll() is None
        assert not back_endpoint.exists() and remote_effect.read_text()=='x' and not remote_gate.exists()
        old_host_pid=host.pid
        host=subprocess.Popen(host_command+['-base-url',f'http://127.0.0.1:{server.server_port}'],env=env,
            stdout=(run/'replacement-host-stdout.txt').open('w'),stderr=(run/'replacement-host-stderr.txt').open('w'))
        wait(back_endpoint.exists,'replacement host ready')
        a=launch('frontend-remote-b')
        a['expected_provider_calls']=1
        identity=assert_connection_identity(a)
        assert identity[0]!=old_identity[0] and identity[2]!=old_host_pid
        assert remote_operation()['ID']==original_operation and remote_process.poll() is None
        assert remote_effect.read_text()=='x' and not remote_gate.exists()
        snap(a,'m45-02-new-runtime-same-remote-job')
        remote_gate.touch(mode=0o600)
        wait(lambda:remote_operation().get('Status')=='completed','same remote operation completed')
        wait(lambda:'M45_REMOTE_FINISHED' in screen(a),'remote result in replacement frontend')
        assert M4['calls']==2 and remote_effect.read_text()=='x'
        records=list((run/'executor').glob('*.json'));assert len(records)==1
        reservation=json.loads(records[0].read_text())
        assert reservation['Plan']['EnvironmentID']=='owned-remote' and reservation['Plan']['ControllerID']=='session-'+hashlib.sha256(sid.encode()).hexdigest()[:32]
        canonical=(state/(sid+'.session.jsonl')).read_text()
        projected=json.dumps(frames)
        assert remote_token not in canonical and remote_token not in projected
        assert remote_ready['TLSAddress'] not in canonical and str(cert_file) not in canonical
        snap(a,'m45-03-remote-output-reconciled')
        remote_report={'same_operation':original_operation,'old_worker_pid':old_pid,'old_worker_pidfd_exited':True,
            'old_host_actual_wait':old_host_exit,'old_host_pid':old_host_pid,'new_host_pid':host.pid,
            'new_incarnation':identity[0],'old_incarnation':old_identity[0],
            'remote_job_identity':reservation['Plan'],'remote_process_survived':True,'execution_effects':1,
            'remote_provider_calls':M4['calls'],'terminal_output':'firstsecond','credentials_absent':True,
            'auxiliary_provider_calls':AUX['calls'],'echo_output':echo['State']['TerminalResult'],
            'timer_notification':notification,'timer_operation':timer['ID']}
        write('m4-workloads-result.json',remote_report)
'''
CLEANUP = r'''
        if remote_worker_fd is not None:os.close(remote_worker_fd)
        if remote_process is not None:
            if remote_process.poll() is None:attempt('remote executor interrupt',lambda:remote_process.send_signal(signal.SIGINT))
            attempt('remote executor actual wait',lambda:remote_process.wait(timeout=6))
            if remote_process.stdout is not None:remote_process.stdout.close()
            if remote_process.returncode!=0:cleanup_errors.append('remote executor not joined zero')
        for thread in remote_threads:
            thread.join(2)
            if thread.is_alive():cleanup_errors.append('remote provider handler not joined')
        if remote_report:
            remote_report['executor_actual_wait_zero']=remote_process.returncode==0
            remote_report['provider_threads_joined']=all(not thread.is_alive() for thread in remote_threads)
            write('m4-workloads-result.json',remote_report)
'''

def main():
    if len(sys.argv)!=3:raise SystemExit(__doc__)
    repo=Path(sys.argv[1]).resolve()
    base=repo/'.claude/skills/verify-unreal-agent-tui/scripts/durable_lifecycle.py'
    source=base.read_text()
    def replace(old,new):
        nonlocal source
        assert source.count(old)==1,old
        source=source.replace(old,new,1)
    replace('    proof_deadline = started + 48','    proof_deadline = started + 85')
    replace("        assert len(calls) == (1 if front['directory'].name == 'frontend-a' else 2)","        assert len(calls) == front.get('expected_provider_calls',1 if front['directory'].name == 'frontend-a' else 2)")
    replace("    def launch(label):\n", "    def launch(label):\n        minimum_frame_index=len(frames)\n")
    replace("        wait(lambda: 'Unreal Agent · attached ·' in screen(front), 'frontend ' + label + ' attached')", "        wait(lambda: 'Ctrl+C detach' in screen(front) and any(f['event'].get('Type')=='snapshot' for f in frames[minimum_frame_index:]), 'frontend ' + label + ' attached with fresh snapshot')")
    replace('    success = False','    success = False\n'+SETUP)
    replace("        server = ThreadingHTTPServer(('127.0.0.1', 0), Model)",PREPARE+"\n        server = ThreadingHTTPServer(('127.0.0.1', 0), Model)")
    replace("        text(a, 'LIFECYCLE_MODEL_BLOCKED', True)",SCENARIO+"\n        text(a, 'LIFECYCLE_MODEL_BLOCKED', True)")
    replace("        write('provider-calls.json', calls)",CLEANUP+"\n        write('provider-calls.json', calls)")
    namespace={'__name__':'owned_m4_lifecycle','__file__':str(base)}
    exec(compile(source,str(base),'exec'),namespace)
    namespace['main']()

if __name__=='__main__':main()
