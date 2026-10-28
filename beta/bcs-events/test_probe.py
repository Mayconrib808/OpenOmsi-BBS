#!/usr/bin/env python3
"""Exercise the real passive agent in isolated JVMs, never executing vendor BCS code."""
import hashlib,json,os,shutil,subprocess,tempfile,zipfile
from pathlib import Path
import build_probe
ROOT=Path(__file__).resolve().parent
SOURCES={
'o/e.java':'package o; public interface e {}',
'o/j.java':'''package o;
import java.util.concurrent.atomic.*;
public class j implements e {
 private final int l; private final AtomicInteger n=new AtomicInteger(60);
 private final AtomicBoolean p=new AtomicBoolean();
 public j(int id){l=id;}
 public void complete(){p.set(true);}
 public int id(){return l;} public boolean done(){return p.get();}
}''',
'l/t.java':'package l; public class t {}',
'system/q.java':'''package system;
import java.nio.file.*; import java.nio.charset.*;
public class q extends Thread {
 private int j=0; private byte l=0; private long m=0; private o.e h=null;
 private final Path output; public Throwable failure;
 public q(Path out){output=out;}
 private void hold(int event, int service, int popup, boolean done, long duration) throws Exception {
  Thread.sleep(duration);
  if(j!=event || l!=(byte)service || (h==null?0:((o.j)h).id())!=popup || (h!=null && ((o.j)h).done()!=done))
   throw new AssertionError("The passive observer changed the fixture's state");
 }
 private void awaitObserver() throws Exception {
  for(int attempt=0;attempt<80;attempt++) {
   if(Files.isDirectory(output))try(java.util.stream.Stream<Path> files=Files.list(output)) {
    for(Path p:(Iterable<Path>)files::iterator)if(p.toString().endsWith(".json")) {
     String s=new String(Files.readAllBytes(p),StandardCharsets.UTF_8);
     if(s.contains("compatible")||s.contains("unsupported_bcs_build"))return;
    }
   }
   Thread.sleep(100);
  }
  throw new AssertionError("No observer status file");
 }
 public void run(){try{
  awaitObserver();
  j=4;h=new o.j(4);hold(4,0,4,false,700);
  l=2;m=System.currentTimeMillis()+1200;hold(4,2,4,false,700);
  l=3;m=System.currentTimeMillis()+600;hold(4,3,4,false,1200);
  ((o.j)h).complete();j=0;hold(0,3,4,true,700);
  h=null;l=0;m=0;hold(0,0,0,false,700);
  for(int repeat=0;repeat<2;repeat++) {
   j=32;h=new o.j(32);hold(32,0,32,false,700);
   l=5;m=System.currentTimeMillis()+400;hold(32,5,32,false,1000);
   ((o.j)h).complete();j=0;hold(0,5,32,true,700);
   h=null;l=0;m=0;hold(0,0,0,false,700);
  }
 }catch(Throwable error){failure=error;}}
}''',
'FixtureMain.java':'''import java.nio.file.*;
public class FixtureMain {
 public static void main(String[] args)throws Exception{
  system.q thread=new system.q(Paths.get(args[0]));thread.start();thread.join();
  if(thread.failure!=null)throw new RuntimeException(thread.failure);
  System.out.println("PASS: fixture actions and results unchanged");
 }
}'''
}

def run_fixture(classes,agent,out):
 env=dict(os.environ,JAVA_TOOL_OPTIONS='-javaagent:"'+str(agent)+'"="'+str(out)+'"')
 result=subprocess.run([shutil.which('java'),'-cp',str(classes),'FixtureMain',str(out)],env=env,capture_output=True,text=True,timeout=45)
 if result.returncode:raise AssertionError(result.stdout+'\n'+result.stderr)
 logs=list(out.glob('*.ndjson'));assert len(logs)==1
 return [json.loads(line)for line in logs[0].read_text().splitlines()]

def main():
 with tempfile.TemporaryDirectory(prefix='bcs-events-test-')as folder:
  work=Path(folder);source=work/'sources';classes=work/'classes';classes.mkdir()
  for name,text in SOURCES.items():
   p=source/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text)
  subprocess.run([shutil.which('java'),'--module','jdk.compiler/com.sun.tools.javac.Main','--release','8','-d',str(classes),*[str(source/name)for name in SOURCES]],check=True)
  hashes={n:hashlib.sha256((classes/n).read_bytes()).hexdigest()for n in build_probe.CLASSES}
  bad=dict(hashes);bad['system/q.class']='0'*64
  agent=build_probe.make_agent(work/'agent with spaces.jar',[bad,hashes])
  rows=run_fixture(classes,agent,work/'capturas com espaços')
  assert any(r['status']=='compatible'for r in rows)
  assert not any(r['status']=='capture_error'for r in rows)
  started=[r for r in rows if r.get('transition')=='event_started']
  assert [r['event_id']for r in started]==[4,32,32],started
  assert [r['event_instance']for r in started]==[1,2,3]
  assert any(r.get('event_id')==4 and r.get('requested_service')=='police'for r in rows)
  assert any(r.get('event_id')==4 and r.get('requested_service')=='medical'for r in rows)
  assert any(r.get('event_id')==32 and r.get('requested_service')=='fire_department'for r in rows)
  assert any(r.get('service_delay_elapsed')and r.get('phase')=='help_requested'for r in rows)
  assert len([r for r in rows if r.get('phase')=='completion_observed'and r.get('transition')!='heartbeat'])>=3
  assert len([r for r in rows if r.get('transition')=='event_no_longer_visible'])==3
  with zipfile.ZipFile(agent)as z:
   assert not any(n.startswith(('system/','o/','l/'))for n in z.namelist())
   for n in z.namelist():
    if n.endswith('.class'):assert int.from_bytes(z.read(n)[6:8],'big')==52
  # Every class must match the same profile: reject a mixture of two builds.
  other_bad=dict(hashes);other_bad['o/j.class']='0'*64
  bad_agent=build_probe.make_agent(work/'unsupported agent.jar',[bad,other_bad])
  unsupported=run_fixture(classes,bad_agent,work/'unsupported capture')
  assert any(r['status']=='unsupported_bcs_build'for r in unsupported)
  assert not any(r.get('event_id',0)>0 for r in unsupported)
  print('PASS: fainting/oil events, wrong/right service calls, deadline vs completion, consecutive same-type events, JVM paths with spaces, read-only state, multiple profiles, and mixed/unsupported-build rejection.')

if __name__=='__main__':main()
