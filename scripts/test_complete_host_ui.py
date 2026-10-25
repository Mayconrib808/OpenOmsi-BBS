"""Exercise all six actual P15 controls in the restored shipped executable."""
import argparse,importlib.util,json,os,subprocess,tempfile
from pathlib import Path
root=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('p15ui',root/'build-support/bridge2203/test_p15_host_controls.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
def main_window(self):
 return next((w for w in self.windows() if 'Host Agent 2.2.0.8' in self.text(w)),None)
m.NativeUI.main_window=main_window
p=argparse.ArgumentParser();p.add_argument('package',type=Path);a=p.parse_args();package=a.package.resolve()
with tempfile.TemporaryDirectory() as tmp:
 folder=Path(tmp);env=os.environ.copy()
 for key in ('LOCALAPPDATA','APPDATA'):
  env[key]=str(folder/key);Path(env[key]).mkdir()
 config=Path(env['LOCALAPPDATA'])/'OpenOmsi-BBS/HostAgent.local.json';config.parent.mkdir()
 before=m.fixture(folder,package/'app/server-dinamico-p15/openomsi.exe');config.write_text(json.dumps(before))
 values=dict(voice=True,voice_radio=True,voice_range=135,public_server=True,require_hardware_id=True,admin_password='ci-private-admin')
 for reopen in (False,True):
  proc=subprocess.Popen([str(package/'HostAgent.exe'),'--config',str(config)],cwd=package,env=env)
  ui=m.NativeUI(proc.pid);window=None
  try:
   window=m.wait_for(ui.main_window);controls=m.wait_for(lambda:ui.controls(window))
   if reopen:
    ui.assert_values(controls,values)
    values=dict(values,voice=False,voice_radio=False,public_server=False,require_hardware_id=False)
   else:
    ui.set_values(controls,values);ui.select_map(controls,1);ui.select_map(controls,0);ui.assert_values(controls,values)
   ui.set_values(controls,values);ui.save(controls)
   m.wait_for(lambda:all(json.loads(config.read_text())['maps']['ci-a'].get(k,False)==v for k,v in values.items()))
   saved=json.loads(config.read_text())
   for key in ('host_key','room_id','control_key','server','company'):assert saved[key]==before[key],key
  finally:
   if window:ui.u.PostMessageW(window,0x10,0,0)
   try:proc.wait(timeout=5)
   except subprocess.TimeoutExpired:proc.terminate();proc.wait(timeout=5)
 (package/'native-p15-controls.json').write_text(json.dumps(dict(status='passed',fields=m.FIELDS,save_reopen=True,map_switch=True,explicit_false_values=True,unrelated_config_preserved=True),indent=2)+'\n')
 print('ACTUAL P15 CONTROLS PASSED: six controls, map switching, Save, reopen, explicit false values, existing keys/server/company preserved')
