"""Build the Windows test harnesses once with unified features, then execute each gate."""
import json,os,subprocess,tomllib
from pathlib import Path
packages={'omsi-sim':None,'omsi-net':None,'omsi-lan':None,'omsi-launcher-core':None,'omsi-input':None,'omsi-audio':['greeting'],'omsi-scene':['rain_probe'],'omsi-app':['coop','roof_edges']}
args=['cargo','test','--locked','--release','--lib','--no-run','--message-format=json']
for package in packages:args+=['-p',package]
p=subprocess.Popen(args,stdout=subprocess.PIPE,text=True,encoding='utf-8',errors='replace')
executables={}
for raw in p.stdout:
 try:row=json.loads(raw)
 except json.JSONDecodeError:print(raw,end='');continue
 if row.get('reason')=='compiler-message':
  msg=row['message']
  if msg.get('level')=='error':print(msg.get('rendered',msg['message']))
 if row.get('reason')=='compiler-artifact' and row.get('executable') and row.get('profile',{}).get('test'):
  # The game's library is openomsi_game and launcher-core's is
  # omsi_launcher_lib: target names are not Cargo package identities.
  package=tomllib.loads(Path(row['manifest_path']).read_text(encoding='utf-8'))['package']['name']
  executables[package]=row['executable']
assert p.wait()==0,'Native test compilation failed'
# Cargo normally adds the target library directories when starting its test
# binaries. Preserve that environment when running the unified harnesses
# directly, including the Steam redistributable placed by omsi-app/build.rs.
runenv=dict(os.environ)
library_dirs=sorted({str(Path(exe).parent) for exe in executables.values()} |
                   {str(Path(exe).parent.parent) for exe in executables.values()})
if os.name=='nt':
 library_dirs.append(str(Path('assets/steam_redist').resolve()))
 runenv['PATH']=os.pathsep.join([*library_dirs,runenv.get('PATH','')])
else:
 runenv['LD_LIBRARY_PATH']=os.pathsep.join([*library_dirs,runenv.get('LD_LIBRARY_PATH','')])
for package,filters in packages.items():
 assert package in executables,('No harness',package,executables.keys())
 for selection in filters or ['']:
  print('REGRESSION GATE:',package,selection or 'ALL',flush=True)
  listed=subprocess.check_output([executables[package],*([selection] if selection else []),'--list'],text=True,env=runenv)
  assert any(line.endswith(': test') for line in listed.splitlines()),('Empty regression gate',package,selection)
  subprocess.run([executables[package],*([selection] if selection else [])],check=True,timeout=1200,env=runenv)
print('All retained native and new passenger/audio regression gates passed.')
