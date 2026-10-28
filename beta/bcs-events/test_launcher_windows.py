#!/usr/bin/env python3
"""Windows process-chain test using a synthetic launcher and JVM; never runs vendor code."""
import hashlib,json,os,shutil,subprocess,tempfile,time
from pathlib import Path
import build_probe
from test_probe import SOURCES
ROOT=Path(__file__).resolve().parent

def main():
 if os.name!='nt':
  print('SKIP: Windows process-chain test requires Windows');return
 with tempfile.TemporaryDirectory(prefix='bcs-launcher-test-')as folder:
  work=Path(folder);root=work/'BCS com espaços';package=root/'Beta-Eventos-BCS';package.mkdir(parents=True)
  sources=work/'sources';classes=work/'classes';classes.mkdir()
  for name,text in SOURCES.items():
   p=sources/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
  java=Path(shutil.which('java')).resolve()
  subprocess.run([str(java),'--module','jdk.compiler/com.sun.tools.javac.Main','--release','8','-d',str(classes),*[str(sources/n)for n in SOURCES]],check=True)
  subprocess.run(['cmd','/d','/c','mklink','/J',str(root/'jre'),str(java.parents[1])],check=True,capture_output=True)
  for name in ['Iniciar-Beta.ps1','Verificar-Captura.ps1']:
   (package/name).write_text((ROOT/'package'/name).read_text(encoding='utf-8'),encoding='utf-8-sig')
  hashes={n:hashlib.sha256((classes/n).read_bytes()).hexdigest()for n in build_probe.CLASSES}
  build_probe.make_agent(package/'bcs-event-probe.jar',hashes)
  csc=Path(os.environ['WINDIR'])/'Microsoft.NET/Framework64/v4.0.30319/csc.exe'
  assert csc.exists(),'Windows C# fixture compiler missing'

  def launcher(out,drop):
   command='-cp "'+str(classes)+'" FixtureMain "'+str(out)+'"'
   code='''using System.Diagnostics; class Launcher { static void Main() {
    var info=new ProcessStartInfo(JAVA,ARGS);info.UseShellExecute=false;info.CreateNoWindow=true;
    DROP
    Process.Start(info);
   }}'''.replace('JAVA',json.dumps(str(java))).replace('ARGS',json.dumps(command)).replace('DROP','info.EnvironmentVariables.Remove("JAVA_TOOL_OPTIONS");'if drop else '')
   src=work/'Launcher.cs';src.write_text(code)
   subprocess.run([str(csc),'/nologo','/target:winexe','/out:'+str(root/'launcher.exe'),str(src)],check=True)

  def run(out,*extra):
   return subprocess.run(['powershell.exe','-NoProfile','-ExecutionPolicy','Bypass','-File',str(package/'Iniciar-Beta.ps1'),'-CaptureDirectory',str(out),'-NoPause','-StartupTimeoutSeconds','8',*extra],capture_output=True,text=True,timeout=45)

  out=work/'capturas reais com espaços';launcher(out,False)
  pre=run(work/'preflight only','-PreflightOnly')
  assert pre.returncode==0,pre.stdout+pre.stderr
  assert 'PRE-TESTE OK'in pre.stdout and 'sem abrir o BCS'in pre.stdout,pre.stdout
  result=run(out)
  assert result.returncode==0,result.stdout+result.stderr
  assert 'CAPTURA ATIVA'in result.stdout,result.stdout
  # Read status while the synthetic BCS is alive, before any long event wait.
  status=subprocess.run(['powershell.exe','-NoProfile','-ExecutionPolicy','Bypass','-File',str(package/'Verificar-Captura.ps1'),'-CaptureDirectory',str(out)],capture_output=True,text=True,timeout=15)
  assert status.returncode==0 and 'CAPTURA ATIVA'in status.stdout,status.stdout+status.stderr
  limit=time.monotonic()+20
  while time.monotonic()<limit:
   rows=[json.loads(line)for f in out.glob('*.ndjson')for line in f.read_text().splitlines()]
   if len([r for r in rows if r.get('transition')=='event_no_longer_visible'])>=3:break
   time.sleep(.2)
  assert [r['event_id']for r in rows if r.get('transition')=='event_started']==[4,32,32]
  time.sleep(.8)
  badout=work/'capturas sem heranca';launcher(badout,True)
  failed=run(badout)
  assert failed.returncode!=0 and 'nenhum processo Java confirmou'in failed.stdout,failed.stdout+failed.stderr
  assert list(badout.glob('diagnostico-inicio-*.txt'))
  assert not list(badout.glob('bcs-events-*.json'))
  print('PASS: Windows installed-JRE preflight, launcher-to-JVM inheritance, live status before events, recorded events, and visible failure when inheritance is lost. Interactive UAC was not exercised.')

if __name__=='__main__':main()
