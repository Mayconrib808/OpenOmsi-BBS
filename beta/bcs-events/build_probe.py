#!/usr/bin/env python3
"""Build the passive Java agent; only hashes are read from the user's vendor JAR."""
import argparse,hashlib,json,shutil,subprocess,tempfile,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parent
CLASSES=['system/q.class','o/j.class','l/t.class']
MANIFEST='Manifest-Version: 1.0\nPremain-Class: org.openomsi.bridge.events.EventProbe\nAgent-Class: org.openomsi.bridge.events.EventProbe\nCan-Redefine-Classes: false\nCan-Retransform-Classes: false\n\n'

def hashes_from_jar(vendor):
 with zipfile.ZipFile(vendor)as z:
  return {name:hashlib.sha256(z.read(name)).hexdigest()for name in CLASSES}

def make_agent(destination,hashes):
 java=shutil.which('java')
 if not java:raise RuntimeError('Java JDK 17 or newer is required to build the observer')
 with tempfile.TemporaryDirectory(prefix='bcs-event-build-')as temp:
  classes=Path(temp)/'classes';classes.mkdir()
  subprocess.run([java,'--module','jdk.compiler/com.sun.tools.javac.Main','--release','8','-encoding','UTF-8','-d',str(classes),str(ROOT/'src/org/openomsi/bridge/events/EventProbe.java')],check=True)
  destination.parent.mkdir(parents=True,exist_ok=True)
  with zipfile.ZipFile(destination,'w',zipfile.ZIP_DEFLATED)as z:
   z.writestr('META-INF/MANIFEST.MF',MANIFEST)
   z.writestr('bcs-event-fingerprints.properties',''.join(name+'='+hashes[name]+'\n'for name in CLASSES))
   for file in sorted(classes.rglob('*.class')):z.write(file,file.relative_to(classes))
 return destination

def main():
 p=argparse.ArgumentParser();p.add_argument('--vendor-jar',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 hashes=hashes_from_jar(a.vendor_jar)
 package=a.output/'Beta-Eventos-BCS';package.mkdir(parents=True,exist_ok=True)
 agent=make_agent(package/'bcs-event-probe.jar',hashes)
 for name in ['Iniciar-Beta.bat','Abrir-Capturas.bat','LEIA-ME.txt']:
  data=(ROOT/'package'/name).read_text(encoding='utf-8').replace('\r\n','\n').replace('\n','\r\n')
  (package/name).write_bytes(data.encode('utf-8'))
 shutil.copy2(ROOT.parents[1]/'LICENSE',package/'LICENSE.txt')
 provenance={'version':'0.1.0','observer_sha256':hashlib.sha256(agent.read_bytes()).hexdigest(),'vendor_class_sha256':hashes,'vendor_code_included':False,'mutates_vendor_jar':False,'visual_events_implemented':False}
 (package/'BUILD.json').write_text(json.dumps(provenance,indent=2)+'\n')
 artifact=a.output/'Beta_Eventos_BCS_0.1.0.zip'
 with zipfile.ZipFile(artifact,'w',zipfile.ZIP_DEFLATED)as z:
  for file in sorted(package.rglob('*')):
   if file.is_file():z.write(file,file.relative_to(a.output))
 print(json.dumps({'zip':str(artifact.resolve()),'sha256':hashlib.sha256(artifact.read_bytes()).hexdigest(),'size':artifact.stat().st_size}))

if __name__=='__main__':main()
