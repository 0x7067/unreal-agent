#!/usr/bin/env python3
"""M3.5 private two-host A/B: owned waits, fenced pending input, contention/recovery.
REPO PRIVATE_SHORT_RUN GATED_A HEALTHY_B STARTUP_FAIL_HOST_B
"""
import importlib.util
from pathlib import Path
import sys

SETUP = '''    original_popen=subprocess.Popen
    def owned_host_popen(command,*args,**kwargs):
        if isinstance(command,list) and len(command)>1 and command[0]==str(runner) and command[1]=='host':
            command=list(command)
            command[0]=str(ARTIFACTS[0])
            kwargs['env']=dict(kwargs['env'])|{'OWNED_M35_GATE_ROOT':str(run)}
        return original_popen(command,*args,**kwargs)
    subprocess.Popen=owned_host_popen
'''
CUT = '''
        import io
        prelude_threads=[]
        prelude_calls=[]
        original_post=Model.do_POST
        def post(self):
            body=self.rfile.read(int(self.headers['Content-Length']))
            if not prelude_calls and b'M35_ACCEPTED_INPUT' in body:
                prelude_threads.append(threading.current_thread())
                prelude_calls.append(json.loads(body))
                assert len(prelude_calls)==1,'duplicate recovered input request'
                event={'type':'response.completed','response':{'id':'m35-recovered','status':'completed','output':[],
                    'usage':{'input_tokens':1,'output_tokens':1}}}
                self.send_response(200)
                self.send_header('Content-Type','text/event-stream')
                self.end_headers()
                self.wfile.write(('data: '+json.dumps(event)+'\\n\\n').encode())
            else:
                self.rfile=io.BytesIO(body)
                original_post(self)
        Model.do_POST=post
        old_pid,old_lease=capture_worker(host,state,sid)
        HANDLES['old']=os.pidfd_open(old_pid)
        before_identity=identity
        ingress_path=state/('host-'+sid+'.session.lock')
        def ingress_owner(pid):
            inode=ingress_path.stat().st_ino
            fds=list(Path('/proc',str(pid),'fd').iterdir())
            assert len(fds)<=256
            held=[]
            for fd in fds:
                try:
                    if fd.stat().st_ino==inode and 'lock:' in Path('/proc',str(pid),'fdinfo',fd.name).read_text():held.append(fd.name)
                except FileNotFoundError:pass
            assert len(held)==1
            with ingress_path.open('rb') as probe:
                try:fcntl.flock(probe,fcntl.LOCK_EX|fcntl.LOCK_NB)
                except BlockingIOError:pass
                else:raise AssertionError('ingress lease not exclusive')
            return inode
        ingress_inode=ingress_owner(host.pid)
        def detach(front):
            keys(front,'C-c')
            join_front(front)
            assert host.poll() is None
            expected=0 if front['directory'].name=='frontend-a' else (1 if front['directory'].name=='frontend-recovered' else 2)
            assert len(calls)==expected
        detach(a)
        retire_command=[str(ARTIFACTS[0]),'host-retire','--runtime-directory',str(run/'back'),
            '--session-directory',str(state),'--session',sid,'--expected-incarnation','intentionally-wrong',
            '--expected-generation',str(identity[1])]
        wrong=subprocess.run(retire_command,env=env,capture_output=True,timeout=8)
        assert wrong.returncode!=0 and b'owner/generation mismatch' in wrong.stderr
        assert not select.select([HANDLES['old']],[],[],0)[0]
        control=socket.socket(socket.AF_UNIX)
        control.settimeout(8)
        control.connect(str(back_endpoint))
        control_stream=control.makefile('rb')
        control_events=[]
        def receive():
            line=control_stream.readline((1<<20)+1)
            assert line.endswith(b'\\n') and len(line)<=1<<20
            event=json.loads(line)
            control_events.append(event)
            assert event['Protocol']==1 and event['SessionID']==sid
            return event
        def send(request):
            request.update(Protocol=1,SessionID=sid)
            control.sendall((json.dumps(request)+'\\n').encode())
        send({'Type':'attach','ID':'attach'})
        hello=receive()
        assert hello['Type']=='hello' and hello['Incarnation']==identity[0] and 'host_retire_v1' in hello['Capabilities']
        while receive()['Type']!='snapshot_end':pass
        token=hello['Connection']
        send({'Type':'runtime','ID':'m35-retire','Connection':token,'Runtime':{'Version':1,'Action':'retire-host','ExpectedGeneration':identity[1]}})
        wait((run/'worker-retired').exists,'real worker Run joined, process exit held')
        assert int((run/'worker-retired').read_text())==old_pid
        assert not select.select([HANDLES['old']],[],[],0)[0]
        accepted_id='m35-pending-stable'
        send({'Type':'submit','ID':'m35-input','Connection':token,'Input':{'ID':accepted_id,'Kind':'external','Payload':'M35_ACCEPTED_INPUT'}})
        while True:
            event=receive()
            if event.get('ID')=='m35-input':
                assert event['Type']=='accepted' and event['InputID']==accepted_id,event
                break
        assert not calls and not prelude_calls
        (run/'allow-worker-exit').write_text('explicit exit gate release')
        while True:
            event=receive()
            if event.get('ID')=='m35-retire' and event.get('Runtime',{}).get('State')=='drained':
                drained=event['Runtime'];break
        assert select.select([HANDLES['old']],[],[],0)[0] and host.poll() is None
        assert not calls and not prelude_calls
        assert ingress_owner(host.pid)==ingress_inode
        with (state/(sid+'.session.lock')).open('rb') as probe:
            fcntl.flock(probe,fcntl.LOCK_EX|fcntl.LOCK_NB)
            fcntl.flock(probe,fcntl.LOCK_UN)
        control_stream.close();control.close()
        journal_before={str(p.relative_to(state)):hashlib.sha256(p.read_bytes()).hexdigest()
            for p in state.rglob('*') if p.is_file() and not os.access(p,os.X_OK)}
        failed_host_pid=host.pid
        old_host_fd=os.pidfd_open(host.pid)
        host.terminate()
        wait((run/'host-stop-requested').exists,'old host stop requested but exit deliberately delayed')
        assert not select.select([old_host_fd],[],[],0)[0]
        contender_command=list(host_command)+['-base-url',f'http://127.0.0.1:{server.server_port}']
        contender_command[0]=str(ARTIFACTS[1])
        contender=subprocess.run(contender_command,env=env,capture_output=True,timeout=8)
        assert contender.returncode!=0 and b'own session frontend' in contender.stderr,contender.stderr
        assert not select.select([old_host_fd],[],[],0)[0] and host.poll() is None
        journal_after={str(p.relative_to(state)):hashlib.sha256(p.read_bytes()).hexdigest()
            for p in state.rglob('*') if p.is_file() and not os.access(p,os.X_OK)}
        assert journal_before==journal_after,'contending B mutated old owner state'
        (run/'allow-host-exit').write_text('explicit host exit gate release')
        failed_returncode=host.wait(timeout=8)
        assert select.select([old_host_fd],[],[],0)[0] and failed_returncode!=0
        os.close(old_host_fd)
        assert not back_endpoint.exists()
        bad_command=list(contender_command);bad_command[0]=str(ARTIFACTS[2])
        bad=subprocess.run(bad_command,env=env,capture_output=True,timeout=8)
        assert bad.returncode!=0 and b'M35 injected host B startup failure' in bad.stderr
        assert not back_endpoint.exists() and not calls and not prelude_calls
        atomic_candidate(ARTIFACTS[1],candidate_slot)
        host=subprocess.Popen(contender_command,env=env,stdout=(run/'m35-host-b-stdout.txt').open('w'),stderr=(run/'m35-host-b-stderr.txt').open('w'))
        wait(back_endpoint.exists,'explicit healthy B host startup')
        wait(lambda:len(prelude_calls)==1,'accepted fenced input recovered on host B')
        wait(lambda:len([i for i in canonical_items() if i['Kind']=='input' and i['Data'].get('ID')==accepted_id])==1,'one canonical original input identity')
        assert not calls,'recovered prelude retried or leaked into baseline'
        a=launch('frontend-recovered')
        identity=assert_connection_identity(a)
        assert identity[0]!=before_identity[0] and identity[1]==1 and identity[2]!=failed_host_pid
        new_pid,new_lease=capture_worker(host,state,sid)
        assert new_pid!=old_pid and new_lease==old_lease
        assert ingress_owner(host.pid)==ingress_inode
        HANDLES['new']=os.pidfd_open(new_pid)
        REPORT.update(before={'worker_pid':old_pid,'lease':old_lease,'identity':before_identity},
            after={'worker_pid':new_pid,'lease':new_lease,'identity':identity},drained=drained,
            old_host_pid=failed_host_pid,old_host_actual_wait=failed_returncode,old_worker_wait_observed=True,
            contention_returncode=contender.returncode,contention_stderr=contender.stderr.decode(),
            bad_host_returncode=bad.returncode,accepted_input_id=accepted_id,recovered_input_requests=len(prelude_calls),
            no_automatic_takeover=True,ingress_lease_inode=ingress_inode,control_events=control_events)
        snap(a,'m35-new-host-new-incarnation-recovered-input')
'''

def main():
    assert len(sys.argv)==6
    path=Path(sys.argv[1]).resolve()/'.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_upgrade.py'
    spec=importlib.util.spec_from_file_location('owned_host_upgrade',path)
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
    module.SETUP+=SETUP
    module.CUT=CUT
    module.ACTIVE=''  # This proof replaces hosts; M3.2/3.3 separately prove runtime active fences.
    module.FINISH=module.FINISH.replace("write('runtime-upgrade-result.json', REPORT)",
        "assert len(prelude_calls)==1 and all(not t.is_alive() for t in prelude_threads)\n        REPORT['total_provider_calls']=len(calls)+len(prelude_calls)\n        write('host-upgrade-result.json', REPORT)")
    module.__dict__['__name__']='__main__'
    module.main()

if __name__=='__main__':main()
