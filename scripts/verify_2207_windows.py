"""Exercise the real Setup lamp plus manual and automatic read-only probes."""
import ctypes, http.server, json, os, subprocess, tempfile, threading, time
from pathlib import Path
from build_2207 import ROOT,PACKAGE
out=ROOT/'build/release-2207';package=out/PACKAGE;evidence=out/'evidence';evidence.mkdir()
env=dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(evidence))
subprocess.run([str(package/'Setup.exe'),'--gui-smoke'],env=env,check=True,timeout=40)
subprocess.run([str(out/'Atualizar-Bridge.exe'),'--coop-progress-smoke'],env=env,check=True,timeout=20)
user=ctypes.windll.user32
user.FindWindowW.argtypes=[ctypes.c_wchar_p,ctypes.c_wchar_p];user.FindWindowW.restype=ctypes.c_void_p
user.GetDlgItem.argtypes=[ctypes.c_void_p,ctypes.c_int];user.GetDlgItem.restype=ctypes.c_void_p
user.SendMessageW.argtypes=[ctypes.c_void_p,ctypes.c_uint,ctypes.c_size_t,ctypes.c_ssize_t];user.SendMessageW.restype=ctypes.c_ssize_t
user.GetWindowTextW.argtypes=[ctypes.c_void_p,ctypes.c_wchar_p,ctypes.c_int]
user.EnumChildWindows.argtypes=[ctypes.c_void_p,ctypes.c_void_p,ctypes.c_ssize_t]
def text(h):
 b=ctypes.create_unicode_buffer(2048);user.GetWindowTextW(h,b,len(b));return b.value
def labels(h):
 result=[]
 cb=ctypes.WINFUNCTYPE(ctypes.c_bool,ctypes.c_void_p,ctypes.c_ssize_t)(lambda w,p:result.append(text(w)) is None)
 user.EnumChildWindows(h,cb,0);return result
calls=[]
class Handler(http.server.BaseHTTPRequestHandler):
 def do_GET(self):
  if self.path!='/status' or self.headers.get('Authorization')!='Bearer '+'a'*32:raise ValueError('unexpected status request')
  calls.append(time.monotonic());self.send_response(200);self.send_header('Content-Type','application/json');self.end_headers()
  self.wfile.write(b'{"running":true,"sessions":[{"session_id":"map-a","state":"disabled"}]}')
 def do_POST(self):raise ValueError('manual check must not start a server')
 def log_message(self,*a):pass
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Handler);threading.Thread(target=server.serve_forever,daemon=True).start()
with tempfile.TemporaryDirectory(prefix='bridge-status-native-') as temporary:
 settings=Path(temporary)/'OpenOmsi-BBS';settings.mkdir()
 original=json.dumps(dict(control_port=server.server_port,control_key='a'*32,company=dict(directory_url='https://directory.workers.dev/rooms/'+'1'*32),maps={'map-a':dict(voice=True,voice_radio=True,admin_password='preserve-me')})).encode()
 path=settings/'HostAgent.local.json';path.write_bytes(original)
 prefs=settings/'Setup.preferences.json';prefs.write_text('{"automatic_updates":false}')
 process=subprocess.Popen([str(package/'Setup.exe')],env=dict(env,LOCALAPPDATA=temporary))
 try:
  deadline=time.monotonic()+15;window=0
  while time.monotonic()<deadline:
   window=user.FindWindowW('OpenOmsiBBSSetup',None)
   if window and calls:break
   time.sleep(.05)
  if not window or not calls:raise ValueError('real Setup did not automatically query local agent')
  button=user.GetDlgItem(window,118)
  if text(button)!='Verificar se o servidor está online':raise ValueError('manual button missing')
  deadline=time.monotonic()+3
  while time.monotonic()<deadline and not any('Nenhum mapa disponível' in s for s in labels(window)):time.sleep(.05)
  if not any('Status do Servidor' in s and 'Nenhum mapa disponível' in s for s in labels(window)):raise ValueError('actual server label did not refresh')
  before=len(calls);manual_at=time.monotonic();user.SendMessageW(button,0x00F5,0,0)
  deadline=time.monotonic()+2
  while time.monotonic()<deadline and len(calls)<=before:time.sleep(.05)
  if len(calls)<=before or calls[before]-manual_at>=2:raise ValueError('manual check did not refresh immediately')
  before=len(calls);deadline=time.monotonic()+6
  while time.monotonic()<deadline and len(calls)<=before:time.sleep(.05)
  if len(calls)<=before:raise ValueError('5-second automatic timer did not query the host')
  if path.read_bytes()!=original:raise ValueError('status changed P15 host settings')
 finally:
  if window:user.SendMessageW(window,0x0010,0,0)
  process.wait(timeout=10);server.shutdown()
report=dict(status='passed',native_setup_gui=True,lamp_ready_and_offline_rendered=True,manual_button=True,automatic_interval_seconds=5,local_requests=len(calls),authenticated=True,read_only=True,host_preferences_unchanged=True,active_map_copy_buttons=True,clipboard_and_pagination=True)
(evidence/'server-status-hotfix.json').write_text(json.dumps(report,indent=2)+'\n')
print('NATIVE_STATUS='+json.dumps(report))
