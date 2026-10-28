"""Owned real-tmux M1.3/M1.4 fault/reconnect/view proof; no user processes touched.
Usage: python3 -I durable_reconnect.py REPO EXISTING_PRIVATE_SHORT_RUN_DIRECTORY
"""
import hashlib,re,json,os,pathlib,select,shlex,signal,socket,subprocess,sys,threading,time
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
repo,root=map(lambda x:pathlib.Path(x).resolve(),sys.argv[1:])
assert root.is_dir() and root.stat().st_mode&0o777==0o700
# Isolate the tmux server as well as its sessions; never share the user's server.
os.environ.pop('TMUX',None)
os.environ['TMUX_TMPDIR']=str(root)
runner,tui=repo/'bin/unreal-agent-runner',repo/'bin/unreal-agent-tui'
script=repo/'.claude/skills/verify-unreal-agent-tui/scripts/tui.sh'
def wait(fn,label,seconds=15):
 end=time.monotonic()+seconds
 while time.monotonic()<end:
  value=fn()
  if value:return value
  time.sleep(.04)
 raise AssertionError('deadline: '+label)
def line(p):
 assert select.select([p.stdout],[],[],15)[0],'host frame deadline'
 data=p.stdout.readline();assert data,'host exited before ready';return json.loads(data)
def stop(p):
 if p is None:return
 if p.poll() is None:p.send_signal(signal.SIGINT)
 try:p.wait(timeout=25)
 except subprocess.TimeoutExpired:p.kill();p.wait(timeout=5);raise AssertionError('owned process needed forced cleanup')
def case(cut):
 run=root/('u13-'+root.name[-6:]+'-'+cut);run.mkdir();front=root/('f'+cut[0]);back=root/('b'+cut[0]);front.mkdir(mode=0o700);back.mkdir(mode=0o700)
 for name in ('state','workspace','evidence','clients'):(run/name).mkdir(mode=0o700)
 session='tui-'+run.name;sid='m13-'+cut
 (run/'session').write_text(session);state=run/'state';draft='M14_UNSENT_DRAFT'
 message='M13 exact stable payload '+cut
 answer='\n\n'.join('ARCHIVE %03d preserved canonical paragraph %s'%(i,'durable session '*8) for i in range(40))
 began,release,cut_done,proxy_stop=threading.Event(),threading.Event(),threading.Event(),threading.Event()
 calls=[];submits=[];connections=[];gaps=[];errors=[];ui_pids=[];host=None;observer=None;owned=[];lock=threading.Lock()
 class Model(BaseHTTPRequestHandler):
  def log_message(self,*args):pass
  def do_POST(self):
   body=json.loads(self.rfile.read(int(self.headers['Content-Length'])));calls.append(body);n=len(calls)
   if n==1:began.set();assert release.wait(45),'provider release deadline';output=[{'type':'function_call','id':'tool','call_id':'local-tool','name':'Bash','arguments':json.dumps({'command':'printf M14_TOOL_DETAIL'})}]
   else:output=[{'type':'message','id':'answer','role':'assistant','status':'completed','content':[{'type':'output_text','text':answer}]}]
   e={'type':'response.completed','response':{'id':str(n),'status':'completed','output':output,'usage':{'input_tokens':1,'output_tokens':1}}}
   self.send_response(200);self.send_header('Content-Type','text/event-stream');self.end_headers()
   try:self.wfile.write(('data: '+json.dumps(e)+'\n\n').encode())
   except BrokenPipeError:pass
 server=ThreadingHTTPServer(('127.0.0.1',0),Model);server.daemon_threads=True
 provider=threading.Thread(target=server.serve_forever);provider.start()
 key=hashlib.sha256((str(state)+'\0'+sid).encode()).hexdigest()+'.sock'
 listener=socket.socket(socket.AF_UNIX);listener.bind(str(front/key));os.chmod(front/key,0o600);listener.listen(4);listener.settimeout(.2)
 def proxy(conn,index):
  backend=socket.socket(socket.AF_UNIX);backend.connect(str(back/key));conn.settimeout(25);backend.settimeout(25)
  with lock:owned.extend([conn,backend])
  armed=threading.Event();closed=threading.Event()
  def close():
   if closed.is_set():return
   closed.set()
   for c in (conn,backend):
    try:c.shutdown(socket.SHUT_RDWR)
    except OSError:pass
    c.close()
  def pump(src,dst,client):
   try:
    f=src.makefile('rb',buffering=0)
    while not closed.is_set():
     data=f.readline((1<<20)+1)
     if not data:break
     assert len(data)<=1<<20 and data.endswith(b'\n'),'unbounded/partial fault frame'
     event=json.loads(data)
     if client and event['Type']=='submit':
      submits.append(event['Input']);armed.set()
      if index==0 and cut=='before':cut_done.set();break
     if not client and index==0 and cut=='accepted' and armed.is_set():
      if event['Type']=='accepted':cut_done.set();break
      continue # Suppress canonical updates too, leaving the UI genuinely uncertain.
     if not client and index==1 and cut=='accepted' and event['Type']=='snapshot':
      captured=event['Head'];release.set()
      wait(lambda:'ARCHIVE 039' in (state/(sid+'.session.jsonl')).read_text(),'commits during withheld reconnect snapshot')
      gaps.append({'captured_head':captured,'answer_committed_before_snapshot_delivery':True,'base':event.get('Base')})
     dst.sendall(data)
   except (BrokenPipeError,ConnectionResetError,OSError):pass
   except Exception as exc:errors.append(repr(exc))
   finally:close()
  p=threading.Thread(target=pump,args=(conn,backend,True));p.start();pump(backend,conn,False);p.join(5);assert not p.is_alive(),'proxy pump not joined'
 def accept():
  while not proxy_stop.is_set():
   try:conn,_=listener.accept()
   except socket.timeout:continue
   except OSError:break
   p=threading.Thread(target=proxy,args=(conn,len(connections)));connections.append(p);p.start()
 acceptor=threading.Thread(target=accept);acceptor.start()
 namespace=['-runtime-directory',str(back),'-session-directory',str(state),'-session',sid]
 env=os.environ|{'OPENAI_API_KEY':'fake','UNREAL_HARNESS_LLM_API_KEY':'fake'}
 def alive():return subprocess.run(['tmux','has-session','-t',session],capture_output=True).returncode==0
 def screen():return subprocess.run(['tmux','capture-pane','-p','-t',session],capture_output=True,text=True).stdout
 def keys(*args):subprocess.run([str(script),'keys',str(run),*args],check=True)
 def snap(name):subprocess.run([str(script),'snap',str(run),name],check=True,stdout=subprocess.DEVNULL);return (run/'evidence'/(name+'.txt')).read_text()
 def launch():
  command='exec '+shlex.join([str(tui),'attach','-runtime-directory',str(front),'-session-directory',str(state),'-session',sid,'-client-state-directory',str(run/'clients')])+' 2>>'+shlex.quote(str(run/'stderr.txt'))
  subprocess.run(['tmux','new-session','-d','-s',session,'-x','160','-y','50','-c',str(run/'workspace'),command],check=True)
  wait(lambda:'Ctrl+C detaches' in screen(),'frontend attach')
  ui_pids.append(int(subprocess.check_output(['tmux','display-message','-p','-t',session,'#{pane_pid}'])))
 def detach():keys('C-c');wait(lambda:not alive(),'owned frontend detach');assert host.poll() is None

 try:
  host=subprocess.Popen([str(runner),'host']+namespace+['-workspace',str(run/'workspace'),'-provider','openai','-base-url',f'http://127.0.0.1:{server.server_port}','-model','fake'],env=env,stdout=subprocess.PIPE,bufsize=0,stderr=(run/'host-stderr.txt').open('w'))
  ready=line(host);(run/'evidence/host-created.json').write_text(json.dumps(ready));assert ready['Type']=='host_ready'
  launch();keys(message,'Enter');assert cut_done.wait(10),'fault not reached'
  wait(lambda:'Disconnected' in screen() and 'Delivery uncertain' in screen(),'truthful uncertain disconnected view');first=snap('01-lost-ack');assert first.count(message)==1
  if cut=='before':assert not calls,'pre-admission drop reached provider'
  else:assert began.wait(10),'accepted input did not reach provider'
  keys('F5');wait(lambda:'Reconnected at revision' in screen(),'cursor reconnect');second=snap('02-cursor-reconnected');assert second.count(message)==1
  if cut=='before':
   assert not calls and len(submits)==1,'reconnect resubmitted automatically'
   keys('/retry','Enter');assert began.wait(10),'explicit retry did not execute';wait(lambda:'Persisted in session' in screen(),'canonical echo reconciliation');assert snap('03-one-persisted-echo').count(message)==1
   assert len(submits)==2 and submits[0]==submits[1],'retry identity or normalized payload changed'
   release.set()
  else:
   keys('/retry','Enter');wait(lambda:'No retained unconfirmed input' in screen(),'already-persisted retry avoided');assert len(submits)==1
  wait(lambda:'ARCHIVE 039' in screen(),'single final answer');snap('04-final-answer');assert len(calls)==2,'duplicate model request'
  keys('C-a','C-k',draft);wait(lambda:draft in screen(),'draft entered');keys('PPage');time.sleep(.15);manual=snap('04b-manual-scroll-with-draft');manual_markers=re.findall(r'ARCHIVE [0-9]{3}',manual);assert manual_markers,'manual scroll not visible'
  # Persist and restore a selected expanded tool plus the underlying manual scroll.
  keys('Tab','Enter');wait(lambda:'M14_TOOL_DETAIL' in screen(),'expanded tool result');snap('05-expanded-before-detach')
  detach();localfile=next((run/'clients').glob('*.json'));saved=json.loads(localfile.read_text());assert saved['Draft']==draft and saved['DetailsCallID'] and saved['ConversationOffset']>0
  (run/'evidence/saved-view.json').write_text(json.dumps(saved,indent=2))
  launch();wait(lambda:'M14_TOOL_DETAIL' in screen(),'expanded detail restored in replacement');snap('06-expanded-after-replacement')
  keys('Escape');time.sleep(.35);wait(lambda:draft in screen() and 'Conversation' in screen(),'draft restored after separate Escape');restored=snap('07-restored-scroll-and-draft');assert re.findall(r'ARCHIVE [0-9]{3}',restored)==manual_markers,'manual scroll changed during frontend replacement';assert len(calls)==2
  detach()
  # Independent canonical snapshot; local draft/preferences must not be ingress/history.
  observer=subprocess.Popen([str(runner),'attach']+namespace,env=env,stdout=subprocess.PIPE,bufsize=0,stderr=(run/'observer-stderr.txt').open('w'))
  records=[]
  while True:
   e=line(observer);records.append(e)
   if e['Type']=='snapshot_end':break
  stop(observer);assert observer.returncode in (0,1,-signal.SIGINT,128+signal.SIGINT) # watch canceled by this driver after complete snapshot
  items=[e['Item'] for e in records if e['Type']=='snapshot_item'];external=[i for i in items if i['Kind']=='input' and i['Data'].get('Kind')=='external']
  assert len(external)==1 and external[0]['Data']['ID']==submits[0]['ID']
  assert draft not in json.dumps(records) and len(calls)==2 and not errors
  (run/'evidence/final-snapshot.json').write_text(json.dumps(records,indent=2))
  (run/'evidence/result.json').write_text(json.dumps({'host':ready,'frontend_pids':ui_pids,'submits':submits,'provider_calls':len(calls),'canonical_external_inputs':len(external),'gap':gaps,'errors':errors},indent=2))
  print(cut+': stable identity, one echo/input, two legitimate tool/model calls, restored view; PASS',flush=True)
 except BaseException:
  if alive():snap('FAILURE')
  raise
 finally:
  release.set();stop(observer)
  if alive():keys('C-c');wait(lambda:not alive(),'frontend cleanup')
  stop(host);proxy_stop.set();listener.close();acceptor.join(3)
  with lock:
   for c in owned:
    try:c.shutdown(socket.SHUT_RDWR)
    except OSError:pass
    c.close()
  for p in connections:p.join(5)
  server.shutdown();server.server_close();provider.join(3)
  cleanup={'host_joined':host is None or host.poll() is not None,'frontend_gone':not alive(),'proxy_joined':not acceptor.is_alive() and all(not p.is_alive() for p in connections),'provider_joined':not provider.is_alive()}
  (run/'evidence/cleanup.json').write_text(json.dumps(cleanup));assert all(cleanup.values()),cleanup
for cut in ('before','accepted'):case(cut)
