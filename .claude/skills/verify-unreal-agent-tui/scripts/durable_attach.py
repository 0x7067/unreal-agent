"""Real tmux M1.2 proof; owns only its created host/client processes and sessions.
Usage: python3 -I durable_attach.py REPO EXISTING_PRIVATE_SHORT_RUN_DIRECTORY
Retains evidence/state; cleanup joins owned processes and never signals others.
"""
import json, os, pathlib, select, shlex, signal, subprocess, sys, threading, time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

repo, root = (pathlib.Path(x).resolve() for x in sys.argv[1:])
runner, tui = repo/'bin/unreal-agent-runner', repo/'bin/unreal-agent-tui'
script = repo/'.claude/skills/verify-unreal-agent-tui/scripts/tui.sh'
assert root.is_dir() and root.stat().st_mode & 0o777 == 0o700

def wait_for(predicate, label, seconds=15):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        result=predicate()
        if result: return result
        time.sleep(.05)
    raise AssertionError('deadline: '+label)

def line(process):
    assert select.select([process.stdout],[],[],15)[0], 'process output deadline'
    data=process.stdout.readline()
    assert data, 'process exited before frame'
    return json.loads(data)

def stop(process):
    if process is None: return
    if process.poll() is None: process.send_signal(signal.SIGINT)
    try: process.wait(timeout=25)
    except subprocess.TimeoutExpired:
        process.kill(); process.wait(timeout=5)
        raise AssertionError('owned process failed to join gracefully')

def run_case(mode):
    run=root/('u12-'+root.name[-6:]+'-'+mode)
    for name in ['workspace','state','config','evidence']: (run/name).mkdir(parents=True,exist_ok=True)
    (run/'workspace/.env').write_text('UNREAL_HARNESS_LLM_PROVIDER=untrusted\n')
    (run/'config/unreal-agent').mkdir(exist_ok=True)
    (run/'config/unreal-agent/preferences.json').write_text('not valid preferences JSON')
    (run/'config/unreal-agent/settings.json').write_text('not valid model settings JSON')
    session='tui-'+run.name
    host=seed=None; captures=[]; release=threading.Event(); began=threading.Event()
    marker, release_file=run/'workspace/shell-start', run/'workspace/shell-release'
    answer='M12 FINAL ANSWER '+mode
    calls=[]
    class Handler(BaseHTTPRequestHandler):
        def log_message(self,*args): pass
        def do_POST(self):
            body=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            calls.append(body); n=len(calls)
            if mode=='tool' and n==1:
                command=f"printf 'M12 PARTIAL OUTPUT\\n'; touch {shlex.quote(str(marker))}; while ! test -f {shlex.quote(str(release_file))}; do sleep 0.02; done; printf 'M12 FINAL OUTPUT\\n'"
                output=[{'type':'function_call','id':'tool','call_id':'m12-shell','name':'Bash','arguments':json.dumps({'command':command})}]
            else:
                if n==1:
                    began.set()
                    if not release.wait(35): return
                output=[{'type':'message','id':'answer','role':'assistant','status':'completed','content':[{'type':'output_text','text':answer}]}]
            response={'type':'response.completed','response':{'id':str(n),'status':'completed','output':output,'usage':{'input_tokens':1,'output_tokens':1}}}
            self.send_response(200);self.send_header('Content-Type','text/event-stream');self.end_headers()
            try: self.wfile.write(('data: '+json.dumps(response)+'\n\n').encode())
            except BrokenPipeError: pass
    server=ThreadingHTTPServer(('127.0.0.1',0),Handler);server.daemon_threads=True
    thread=threading.Thread(target=server.serve_forever);thread.start()
    host_env=os.environ|{'OPENAI_API_KEY':'fake','UNREAL_HARNESS_LLM_API_KEY':'fake'}
    ui_env=os.environ|{'XDG_CONFIG_HOME':str(run/'config'),'UNREAL_HARNESS_LLM_PROVIDER':'untrusted-provider','UNREAL_HARNESS_LLM_BASE_URL':'http://127.0.0.1:1','OPENAI_API_KEY':'not-a-host-credential'}
    namespace=['-runtime-directory',str(root),'-session-directory',str(run/'state'),'-session','m12-'+mode]
    ui_pids=[]
    def alive(): return subprocess.run(['tmux','has-session','-t',session],capture_output=True).returncode==0
    def screen():
        p=subprocess.run(['tmux','capture-pane','-p','-t',session],capture_output=True,text=True)
        return p.stdout if p.returncode==0 else ''
    def launch():
        command='exec '+shlex.join(['env']+[k+'='+v for k,v in ui_env.items() if os.environ.get(k)!=v]+[str(tui),'attach']+namespace)+' 2>>'+shlex.quote(str(run/'stderr.txt'))
        for attempt in range(5):
            subprocess.run(['tmux','new-session','-d','-s',session,'-x','160','-y','50','-c',str(run/'workspace'),command],check=True)
            ready=False
            for _ in range(100):
                text=screen()
                if 'attached' in text and 'Ctrl+C detaches' in text: ready=True;break
                if not alive():break
                time.sleep(.05)
            if ready:
                pid=int(subprocess.check_output(['tmux','display-message','-p','-t',session,'#{pane_pid}']))
                ui_pids.append(pid);return
            assert not alive(), 'TUI started but did not render attach view'
        raise AssertionError('TUI attach failed: '+(run/'stderr.txt').read_text())
    def snap(name):
        subprocess.run([str(script),'snap',str(run),name],stdout=subprocess.DEVNULL,check=True)
        path=run/'evidence'/f'{name}.txt';captures.append(path);return path.read_text()
    def keys(*args): subprocess.run([str(script),'keys',str(run),*args],check=True)
    def detach():
        keys('C-c');wait_for(lambda:not alive(),'TUI graceful detach')
        assert host.poll() is None,'client stopped host'
    try:
        host=subprocess.Popen([str(runner),'host']+namespace+['-workspace',str(run/'workspace'),'-provider','openai','-base-url',f'http://127.0.0.1:{server.server_port}','-model','fake'],env=host_env,stdout=subprocess.PIPE,bufsize=0,stderr=(run/'host-stderr.txt').open('w'))
        ready=line(host);assert ready['Type']=='host_ready'
        seed=subprocess.Popen([str(runner),'attach']+namespace+['-submit','-input-id','m12-stable-'+mode,'-message','M12 headless input'],env=host_env,stdout=subprocess.PIPE,bufsize=0,stderr=(run/'seed-stderr.txt').open('w'))
        while line(seed)['Type']!='accepted': pass
        if mode=='model': assert began.wait(10),'provider not reached'
        else: wait_for(marker.exists,'shell producing output')
        stop(seed);seed=None
        launch();before=snap('01-attached-running')
        assert 'M12 headless input' in before and 'attached' in before
        assert '/reload at idle boundary' not in before and 'not checked' not in before
        if mode=='tool':
            assert 'Bash' in before
            keys('Tab','Enter');wait_for(lambda:'Awaiting result' in screen(),'pending tool details')
            snap('01b-pending-tool-details');keys('Escape')
            wait_for(lambda:'Conversation' in screen(),'return from details')
        else: assert 'Working' in before
        detach()
        (run/'evidence/detached.json').write_text(json.dumps({'host_pid':host.pid,'host_alive':host.poll() is None,'provider_calls':len(calls)}))
        if mode=='model':
            release.set()
            wait_for(lambda:answer in (run/'state'/('m12-'+mode+'.session.jsonl')).read_text(),'answer while no TUI is attached')
            launch();wait_for(lambda:answer in screen(),'fresh snapshot or ordered completion')
            after=snap('02-fresh-answer');assert after.count(answer)==1
            detach()
        else:
            # A second actual frontend joins while the shell is STILL blocked,
            # then receives committed tool output and the final answer live.
            launch();snap('02-reattached-tool-running')
            release_file.touch();wait_for(lambda:answer in screen(),'live final answer')
            snap('03-live-answer');keys('Tab','Enter')
            wait_for(lambda:'M12 FINAL OUTPUT' in screen(),'committed shell details')
            details=snap('04-tool-output');assert 'M12 PARTIAL OUTPUT' in details
            detach();launch();wait_for(lambda:answer in screen(),'fresh final answer')
            final=snap('05-fresh-answer');assert final.count(answer)==1
            detach()
        # Independent final snapshot verifies host/worker identity and cardinality.
        observer=subprocess.Popen([str(runner),'attach']+namespace,env=host_env,stdout=subprocess.PIPE,bufsize=0,stderr=(run/'observer-stderr.txt').open('w'))
        try:
            frames=[]
            while True:
                e=line(observer);frames.append(e)
                if e['Type']=='snapshot_end':break
            hello=frames[0];header=next(e for e in frames if e['Type']=='snapshot')
            items=[e['Item'] for e in frames if e['Type']=='snapshot_item']
            assert hello['HostPID']==ready['HostPID']==host.pid and hello['Incarnation']==ready['Incarnation']
            assert header['Status']['Generation']==1 and not header['Status']['Stopped']
            assert json.dumps(items).count(answer)==1
            expected=1 if mode=='model' else 2
            assert len(calls)==expected,(len(calls),expected)
            assert len(set(ui_pids))==len(ui_pids),'frontend was not replaced'
            report={'mode':mode,'host_pid':host.pid,'incarnation':hello['Incarnation'],'generation':1,'provider_calls':len(calls),'answer_count':1,'head':frames[-1]['Head'],'frontend_pids':ui_pids,'captures':[str(p) for p in captures]}
            (run/'evidence/result.json').write_text(json.dumps(report,indent=2)+'\n')
            (run/'evidence/final-snapshot.json').write_text(json.dumps(frames,indent=2)+'\n')
            print(json.dumps(report),flush=True)
        finally: stop(observer)
    finally:
        release.set();release_file.touch()
        if alive():
            subprocess.run(['tmux','send-keys','-t',session,'C-c'],check=False)
            try:wait_for(lambda:not alive(),'cleanup UI',5)
            except AssertionError:subprocess.run(['tmux','kill-session','-t',session],check=False);raise
        stop(seed);stop(host)
        if host is not None: assert host.returncode==0,('host exit',host.returncode)
        server.shutdown();server.server_close();thread.join(timeout=5);assert not thread.is_alive()
        (run/'evidence/cleanup.json').write_text(json.dumps({'host_joined':host is None or host.poll() is not None,'provider_thread_joined':not thread.is_alive(),'tmux_session_absent':not alive()}))

for mode in ['model','tool']:run_case(mode)
