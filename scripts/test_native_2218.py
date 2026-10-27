"""Build the Windows test harnesses once with unified features, then execute each gate."""
import json,subprocess
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
  executables[row['target']['name'].replace('_','-')]=row['executable']
assert p.wait()==0,'Native test compilation failed'
for package,filters in packages.items():
 assert package in executables,('No harness',package,executables.keys())
 for selection in filters or ['']:
  print('REGRESSION GATE:',package,selection or 'ALL',flush=True)
  subprocess.run([executables[package],*([selection] if selection else [])],check=True,timeout=1200)
print('All retained native and new passenger/audio regression gates passed.')
