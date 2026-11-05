"""Exercise all six actual P15 controls in the restored shipped executable."""
import argparse,importlib.util,json,os,subprocess,tempfile
from pathlib import Path
root=Path(__file__).resolve().parents[1]
spec=importlib.util.spec_from_file_location('p15ui',root/'build-support/bridge2203/test_p15_host_controls.py');m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
def main_window(self):
 return next((w for w in self.windows() if 'Host Agent ' in self.text(w)),None)
m.NativeUI.main_window=main_window
p=argparse.ArgumentParser();p.add_argument('package',type=Path);p.add_argument('--protocol',type=int,default=15);p.add_argument('--server',type=Path);p.add_argument('--report',type=Path);p.add_argument('--large-inventory',action='store_true');a=p.parse_args();package=a.package.resolve()
with tempfile.TemporaryDirectory() as tmp:
 folder=Path(tmp);env=os.environ.copy()
 for key in ('LOCALAPPDATA','APPDATA'):
  env[key]=str(folder/key);Path(env[key]).mkdir()
 config=Path(env['LOCALAPPDATA'])/'OpenOmsi-BBS/HostAgent.local.json';config.parent.mkdir()
 before=m.fixture(folder,a.server.resolve() if a.server else package/'app/server-dinamico-p15/openomsi.exe')
 if a.protocol in (16,17):before['company']['protocol']=a.protocol;before['company']['openomsi_version']='0.3.49' if a.protocol==17 else '0.3.47'
 before['future_private']={'keep':42}
 before['maps']['ci-a']['future_map']='keep'
 before['company']['future_company']='keep'
 if a.large_inventory:
  files=[dict(path='Sceneryobjects/Large/'+('inventory'*10)+'_%05d.sco'%i,sha256='a'*64) for i in range(22000)]
  before['company']['packages'].append(dict(id='large-inventory',name='Large installed map inventory',version='1',folders=['Sceneryobjects/Large'],files=files))
 config.write_text(json.dumps(before,indent=2))
 if a.large_inventory:assert 4<<20 < config.stat().st_size < 32<<20
 values=dict(voice=True,voice_radio=True,voice_range=135,public_server=True,require_hardware_id=True,admin_password='ci-private-admin')
 for reopen in (False,True):
  proc=subprocess.Popen([str(package/'HostAgent.exe'),'--config',str(config)],cwd=package,env=env)
  ui=m.NativeUI(proc.pid);window=None
  try:
   window=m.wait_for(ui.main_window);controls=m.wait_for(lambda:ui.controls(window))
   clock_controls = [c for c in ui.children(window) if c['id'] == 326 and c['cls'].lower() == 'button']
   assert len(clock_controls) == 1, 'Admin clock opt-in checkbox missing'
   clock = clock_controls[0]['hwnd']
   assert bool(ui.send(clock, 0xF0)) == reopen, 'Admin clock default or saved state changed'
   clock_enabled = not reopen
   ui.send(clock, 0xF1, int(clock_enabled))
   if reopen:
    ui.assert_values(controls,values)
    values=dict(values,voice=False,voice_radio=False,public_server=False,require_hardware_id=False)
   else:
    ui.set_values(controls,values);ui.select_map(controls,1);ui.select_map(controls,0);ui.assert_values(controls,values)
   assert bool(ui.send(clock, 0xF0)) == clock_enabled, 'Map switching reset admin clock setting'
   ui.set_values(controls,values);ui.save(controls)
   m.wait_for(lambda:all(json.loads(config.read_text())['maps']['ci-a'].get(k,False)==v for k,v in values.items()))
   m.wait_for(lambda: json.loads(config.read_text()).get('allow_admin_time', False) == clock_enabled)
   saved=json.loads(config.read_text())
   assert saved['maps']['ci-a']['future_map']=='keep'
   for key in ('host_key','room_id','control_key','server','company','future_private'):assert saved[key]==before[key],key
  finally:
   if window:ui.u.PostMessageW(window,0x10,0,0)
   try:proc.wait(timeout=5)
   except subprocess.TimeoutExpired:proc.terminate();proc.wait(timeout=5)
 (a.report or package/('native-p'+str(a.protocol)+'-controls.json')).write_text(json.dumps(dict(status='passed',fields=m.FIELDS,save_reopen=True,map_switch=True,explicit_false_values=True,unrelated_config_preserved=True,admin_clock_opt_in_tested=True,large_inventory_tested=a.large_inventory,inventory_files=22000 if a.large_inventory else 0,final_private_size=config.stat().st_size),indent=2)+'\n')
 print('ACTUAL HOST CONTROLS PASSED: six controls, map switching, Save, reopen, explicit false values, existing keys/server/company preserved; admin clock default-off, map switch, save/reopen and disable passed')
