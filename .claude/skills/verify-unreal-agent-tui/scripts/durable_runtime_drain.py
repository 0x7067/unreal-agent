#!/usr/bin/env python3
"""Owned M3.3 proof: A->B natural model drain; cancel queued model/shell upgrades.
Uses exact transforms of the accepted private M3.2/lifecycle drivers, not user UI.
"""
import importlib.util
from pathlib import Path
import sys

PRELUDE = '''
        import io
        SAFE = {'armed':True,'entered':threading.Event(),'release':threading.Event(),'finished':threading.Event(),'calls':0,'threads':[]}
        original_post = Model.do_POST
        def drain_post(request):
            raw = request.rfile.read(int(request.headers['Content-Length']))
            if SAFE['armed'] and b'M33_SAFE_DRAIN' in raw:
                SAFE['armed'] = False
                SAFE['calls'] += 1
                SAFE['threads'].append(threading.current_thread())
                SAFE['entered'].set()
                assert SAFE['release'].wait(12), 'owned safe-provider release not received'
                event = {'type':'response.completed','response':{'id':'m33-safe','status':'completed','output':[{'type':'message','id':'m33-safe-output','role':'assistant','status':'completed','content':[{'type':'output_text','text':'M33 safe model finished naturally'}]}],'usage':{'input_tokens':1,'output_tokens':1}}}
                request.send_response(200)
                request.send_header('Content-Type','text/event-stream')
                request.end_headers()
                request.wfile.write(('data: '+json.dumps(event)+'\\n\\n').encode())
                SAFE['finished'].set()
            else:
                request.rfile = io.BytesIO(raw)
                original_post(request)
        Model.do_POST = drain_post
        def observe_runtime(wanted, start=0):
            def result():
                for record in reversed(frames[start:]):
                    event = record.get('event',{})
                    if event.get('Type')=='runtime' and event.get('Runtime',{}).get('State')==wanted:
                        return event
            return wait(result, 'runtime '+wanted)
        def cancel_drain(front,label):
            start = len(frames)
            text(front,'/upgrade',True)
            queued = observe_runtime('queued',start)
            wait(lambda:'queued' in screen(front), label+' queued visible')
            keys(front,'C-a','C-k')
            text(front,'/cancel-upgrade',True)
            admitted = observe_runtime('cancel_requested',start)
            settled = observe_runtime('canceled',start)
            assert queued['ID']==settled['ID'] and admitted['ID']!=settled['ID']
            assert settled['Runtime']['Generation']==identity[1]
            assert capture_worker(host,state,sid)==(new_pid,new_lease)
            assert not select.select([HANDLES['new']],[],[],0)[0]
            assert not cleanup_gate.is_set()
            REPORT[label+'_upgrade_canceled_without_work_cancel']={'queued':queued,'admitted':admitted,'settled':settled}
            snap(front,'m33-'+label+'-upgrade-canceled')
            keys(front,'C-a','C-k')
        original_text = text
        def drain_text(front,value,submit=False):
            if value=='UNSENT_BASH_DRAFT':
                cancel_drain(front,'unsafe-shell')
                assert (workspace/'sentinel').read_text()=='x'
                assert not (workspace/'bash-gate').exists() and not (workspace/'late-tool').exists()
            original_text(front,value,submit)
        text = drain_text
'''
NATURAL = '''
        text(a,'M33_SAFE_DRAIN',True)
        wait(SAFE['entered'].is_set,'safe model request active')
        start = len(frames)
        queued = runtime_command('/upgrade','queued')
        wait(lambda:'queued' in screen(a),'natural drain queued visible')
        assert SAFE['calls']==1 and not SAFE['finished'].is_set()
        assert not select.select([HANDLES['old']],[],[],0)[0]
        assert capture_worker(host,state,sid)==(old_pid,old_lease)
        snap(a,'m33-safe-request-upgrade-queued')
        SAFE['release'].set()
        completed = observe_runtime('completed',start)['Runtime']
        wait(SAFE['finished'].is_set,'safe model completed naturally')
        keys(a,'C-a','C-k')
        REPORT['natural_drain_approved']={'queued':queued,'completed':completed,'provider_calls':SAFE['calls'],'work_not_canceled':True}
'''
ACTIVE = '''
        cancel_drain(a,'active-model')
        assert len(calls)==1 and handler_threads[0].is_alive() and not cleanup_gate.is_set()
'''



def capture_lease_holder(host, state, sid):
    # Queued reload legitimately has a short-lived side-effect-free probe child.
    # Observe all bounded owned children; select by exact held lease, not PID alone.
    import fcntl
    threads=list(Path(f'/proc/{host.pid}/task').iterdir())
    assert 0<len(threads)<=256
    children=set()
    for thread in threads:
        try: children.update((thread/'children').read_text().split())
        except FileNotFoundError: pass
    assert 0<len(children)<=16
    lease=state/(sid+'.session.lock')
    st=lease.stat();identity=(st.st_dev,st.st_ino);holders=[]
    for child in children:
        try: descriptors=list(Path(f'/proc/{child}/fd').iterdir())
        except FileNotFoundError: continue
        assert len(descriptors)<=256
        held=False
        for descriptor in descriptors:
            try:
                st=descriptor.stat();held |= (st.st_dev,st.st_ino)==identity
            except FileNotFoundError: pass
        if held: holders.append(int(child))
    assert len(holders)==1, f'expected one exact lease holder: {holders}'
    with lease.open('rb') as stream:
        try: fcntl.flock(stream,fcntl.LOCK_EX|fcntl.LOCK_NB)
        except BlockingIOError: pass
        else:
            fcntl.flock(stream,fcntl.LOCK_UN)
            raise AssertionError('session lease not exclusively held')
    return holders[0],identity


def main():
    assert len(sys.argv)==6, 'REPO PRIVATE_SHORT_RUN RUNNER_A RUNNER_B BAD_RUNNER'
    repo=Path(sys.argv[1]).resolve()
    source=repo/'.claude/skills/verify-unreal-agent-tui/scripts/durable_runtime_upgrade.py'
    spec=importlib.util.spec_from_file_location('owned_runtime_upgrade',source)
    module=importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    old="        completed = runtime_command('/reload', 'completed')"
    assert module.CUT.count(old)==1
    module.CUT=PRELUDE+module.CUT.replace(old,NATURAL,1)
    module.SETUP += "    if os.environ.get(\"OWNED_M33_TUI\"): command[0]=os.environ[\"OWNED_M33_TUI\"]\n"
    module.capture_worker=capture_lease_holder
    module.ACTIVE=ACTIVE
    module.FINISH='''        for thread in SAFE['threads']:
            thread.join(timeout=2)
            assert not thread.is_alive(), 'owned safe-provider thread did not join'
        assert SAFE['calls']==1 and SAFE['finished'].is_set()
        REPORT['total_actual_provider_requests']=len(calls)+SAFE['calls']
        assert REPORT['total_actual_provider_requests']==3
'''+module.FINISH.replace("write('runtime-upgrade-result.json', REPORT)","write('runtime-drain-result.json', REPORT)")
    module.__dict__["__name__"] = "__main__"  # execute trusted fixture entrypoint too
    module.main()


if __name__=='__main__':
    main()
