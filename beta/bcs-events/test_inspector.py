#!/usr/bin/env python3
"""Inspector capture fixtures; no vendor application or UI is executed."""
import hashlib,json,shutil,subprocess,tempfile
from pathlib import Path
import build_probe

BASE={
'o/e.java':'package o; public interface e {}',
'o/j.java':'package o; public class j implements e {}',
'l/t.java':'package l; public class t {}',
'l/i.java':'package l; public class i {}',
'p/E.java':'package p; public class E {}',
'webservicesbbs/FahrkartenkontrolleurDto.java':'package webservicesbbs; public class FahrkartenkontrolleurDto {}',
'o/f.java':'''package o; import java.util.concurrent.atomic.AtomicBoolean;
public class f { private final AtomicBoolean i=new AtomicBoolean(), a=new AtomicBoolean(), r=new AtomicBoolean(); private int n;
 public void checked(){n++;} public void pause(boolean value){a.set(value);} public void stop(){i.set(true);r.set(true);}
 public void assertUnchanged(int expected){if(n!=expected||!i.get()||!r.get()||a.get())throw new AssertionError("Observer mutated inspector");}
}''',
'FixtureMain.java':'''import java.nio.file.*; public class FixtureMain {public static void main(String[] args)throws Exception{
 system.q q=new system.q();q.start();q.join();if(q.failure!=null)throw new RuntimeException(q.failure);
 System.out.println("PASS: inspector fixture state unchanged");}}'''
}

def main():
 with tempfile.TemporaryDirectory(prefix='inspector-capture-')as temp:
  work=Path(temp)
  for state in ['M','O']:
   source=work/state/'src';classes=work/state/'classes';classes.mkdir(parents=True)
   sources=dict(BASE)
   sources[f'system/{state}.java']=f'''package system; public class {state} {{private static int aE; private static String aF=""; private static short S;
 public static void choose(int id,String name){{aE=id;aF=name;S=3;}}}}'''
   sources['system/q.java']='''package system; public class q extends Thread {
 private int j;private byte l;private long m;private o.e h;private o.f g;public Throwable failure;
 private void hold()throws Exception{Thread.sleep(700);}
 public void run(){try{
 STATE.choose(0,"");Thread.sleep(2700);
 STATE.choose(17,"Nome editável feminino");hold();
 g=new o.f();hold();g.checked();hold();g.pause(true);hold();g.pause(false);g.checked();hold();g.stop();hold();g.assertUnchanged(2);g=null;hold();
 STATE.choose(18,"Nome editável masculino");g=new o.f();hold();g.checked();hold();g.stop();hold();g.assertUnchanged(1);g=null;STATE.choose(0,"");hold();
 }catch(Throwable e){failure=e;}}}'''.replace('STATE',state)
   for name,text in sources.items():
    p=source/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
   subprocess.run([shutil.which('java'),'--module','jdk.compiler/com.sun.tools.javac.Main','--release','8','-encoding','UTF-8','-d',str(classes),*[str(source/n)for n in sources]],check=True)
   hashes={str(p.relative_to(classes)):hashlib.sha256(p.read_bytes()).hexdigest()for p in classes.rglob('*.class')}
   for mismatch in ([False,True]if state=='O'else[False]):
    profile=dict(hashes)
    if mismatch:profile['o/f.class']='0'*64
    agent=build_probe.make_agent(work/f'{state}-{mismatch}.jar',profile)
    out=work/f'capture {state} {mismatch}'
    result=subprocess.run([shutil.which('java'),'-javaagent:'+str(agent)+'='+str(out),'-cp',str(classes),'FixtureMain'],capture_output=True,text=True,timeout=25)
    assert result.returncode==0,result.stdout+result.stderr
    rows=[json.loads(line)for p in out.glob('*.ndjson')for line in p.read_text().splitlines()]
    compatible=[r for r in rows if r.get('status')=='compatible']
    assert compatible and not any(r.get('status')=='capture_error'for r in rows)
    if mismatch:
     assert all(r.get('inspector_status')=='unsupported_bcs_build'for r in compatible)
     assert not any('inspector_id'in r for r in compatible)
     continue
    assert all(r.get('inspector_status')=='compatible'for r in compatible)
    assert any(r.get('inspector_selected')is False for r in compatible)
    assert {r.get('inspector_id')for r in compatible}>={0,17,18}
    assert all(r.get('inspector_gender')=='unknown'for r in compatible)
    phases={r.get('inspection_phase')for r in compatible}
    assert phases>={'idle','requested','checking','paused','stopped','closed'},phases
    assert {r.get('inspection_instance')for r in compatible}>={0,1,2}
    assert any(r.get('inspection_passengers_checked')==2 for r in compatible)
    assert all(r.get('inspection_outcome')=='unknown'for r in compatible)
  print('PASS: both reviewed state layouts, selected/absent inspector, requested/checking/paused/stopped/closed, repeated inspections, editable names never guess gender, read-only state, inspector fingerprint rejection independent of emergency capture.')

if __name__=='__main__':main()
