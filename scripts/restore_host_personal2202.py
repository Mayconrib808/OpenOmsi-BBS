"""Restore the COMPLETE approved P15 hosts; change identification data only.

The archived Go source predates the complete P15 binary. Do not rebuild from it.
Every byte of each executable code section must stay equal to the original.
"""
import argparse,hashlib,json,os,shutil,struct,subprocess,sys,zipfile
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'build/host-personal2202'
PACKAGE='Bridge_2.2.0.2_Host_Particular_Maycon'
NATIVE_SHA='8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
PREVIOUS={
 'HostAgent.exe':['a483e435a23b9a74276fb045945ce58357ab5ea99f8759ddc2dddae41a41dba4','9611698e6318da299c32841c08a770959693dbaa495c33caa6671ff3649cadd3','b02fd974513b2e5f94987f459af72fe736fb80e132a0af68d51f0df7d42e368a'],
 'CompanyHost.exe':['3d57a4733cbfb8bec677c29d888de339a92ae43c161f9e48ccc9b7a97c7d6cb3','ebfd3836d70ea342e65494309496d85f22232af540faacbc62dbcf6df66beb5c','69c28833dab1ecec927acfbf61c26b75c7c4227d25b019baf0c3b68c29ed393f']}
FEATURES=[b'voice_radio',b'voice_range',b'require_hardware_id',b'public_server',b'owner_bans',b'admin_password',b'company_host_modern.go']
def sha(b):return hashlib.sha256(b).hexdigest()
def dump(p,o):p.write_text(json.dumps(o,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')
def sections(b):
 pe=struct.unpack_from('<I',b,0x3c)[0];count=struct.unpack_from('<H',b,pe+6)[0];size=struct.unpack_from('<H',b,pe+20)[0];result=[]
 for i in range(count):
  h=pe+24+size+40*i;n=b[h:h+8].split(b'\0')[0].decode();length,offset=struct.unpack_from('<II',b,h+16);result.append((n,offset,offset+length))
 return result
def retag(b,name):
 original=b;old=b'2.1.9.8';new=b'2.2.0.2';positions=[];at=0;parts=sections(b)
 while (at:=b.find(old,at))>=0:
  section=next(n for n,start,end in parts if start<=at and at+len(old)<=end)
  if section!='.rdata':raise ValueError('Version literal unexpectedly touches executable code')
  positions.append(at);at+=len(old)
 if len(positions)!=(3 if name=='HostAgent.exe' else 1):raise ValueError('Unexpected original identification layout')
 b=b.replace(old,new)
 allowed={p+i for p in positions for i in range(len(old))}
 if any(a!=c and i not in allowed for i,(a,c) in enumerate(zip(original,b))):raise ValueError('Unexpected binary change')
 code={}
 for n,start,end in parts:
  if n=='.text':
   if original[start:end]!=b[start:end]:raise ValueError('Executable code changed')
   code[n]=sha(b[start:end])
 return b,{'original_sha256':sha(original),'updated_sha256':sha(b),'version_literal_offsets':positions,'changed_sections':['.rdata'],'executable_code_sections_unchanged':code,'original_version':'2.1.9.8','identification_version':'2.2.0.2','source_recompiled':False}
def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--regressed',type=Path);p.add_argument('--finalize',action='store_true');a=p.parse_args();package=OUT/PACKAGE
 if a.finalize:
  for major in (5,7):
   r=json.loads((package/f'validation/installer-powershell-{major}.json').read_text(encoding='utf-8-sig'))
   if r['status']!='passed' or len(r['passed'])<16:raise ValueError('Installer scenarios missing')
  ui=json.loads((package/'validation/native-p15-controls.json').read_text())
  if ui['status']!='passed' or len(ui['persisted_fields'])!=6 or not ui['map_switch_preserves_values']:raise ValueError('Actual P15 controls and persistence did not pass')
  scan=json.loads((package/'validation/defender.json').read_text(encoding='utf-8-sig'))
  if scan['status']!='passed':raise ValueError('Defender did not pass')
  sums=''.join(sha(f.read_bytes())+'  '+f.relative_to(package).as_posix()+'\n' for f in sorted(package.rglob('*')) if f.is_file())
  (package/'SHA256SUMS.txt').write_text(sums,encoding='utf-8',newline='\n')
  target=OUT/(PACKAGE+'.zip')
  with zipfile.ZipFile(target,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
   for f in sorted(package.rglob('*')):
    if f.is_file():z.write(f,f.relative_to(package).as_posix())
  digest=sha(target.read_bytes());(OUT/(PACKAGE+'.zip.sha256')).write_text(digest+'  '+target.name+'\n')
  print('HOST2202_FINAL='+json.dumps({'sha256':digest,'size':target.stat().st_size,'name':target.name,'edition':'host-pessoal.2'}));return
 OUT.mkdir(parents=True,exist_ok=True);package.mkdir();baseline=OUT/'baseline';baseline.mkdir();provenance={}
 if not a.regressed:raise ValueError('Actual previous regression package is required for upgrade validation')
 regressed=OUT/'regressed';regressed.mkdir()
 with zipfile.ZipFile(a.regressed) as z:
  for name in PREVIOUS:
   data=z.read(name)
   if sha(data)!=PREVIOUS[name][2]:raise ValueError('Unexpected previous regression binary')
   (regressed/name).write_bytes(data)
 with zipfile.ZipFile(a.baseline) as z:
  if z.testzip():raise ValueError('Corrupt baseline')
  for name in PREVIOUS:
   original=z.read(name)
   if sha(original)!=PREVIOUS[name][0]:raise ValueError('The complete P15 original differs')
   for marker in FEATURES:
    if marker not in original:raise ValueError('Complete P15 code is missing '+marker.decode())
   (baseline/name).write_bytes(original)
   updated,record=retag(original,name);(package/name).write_bytes(updated);provenance[name]=record
  for name in z.namelist():
   if name.startswith('app/server/') and not name.endswith('/'):
    data=z.read(name);dest=package/'app/server-dinamico-p14'/Path(name).name;dest.parent.mkdir(parents=True,exist_ok=True);dest.write_bytes(data)
    old=baseline/name;old.parent.mkdir(parents=True,exist_ok=True);old.write_bytes(data)
 if sha((package/'app/server-dinamico-p14/openomsi.exe').read_bytes())!=NATIVE_SHA:raise ValueError('Dynamic server changed')
 validation=package/'validation';validation.mkdir();env=os.environ.copy();env['BRIDGE_SETUP_PREVIEWS']=str(OUT/'native-previews')
 with (validation/'hostagent-native-smoke.log').open('w',encoding='utf-8') as log:subprocess.run([str(package/'HostAgent.exe'),'--gui-smoke'],env=env,stdout=log,stderr=subprocess.STDOUT,timeout=45,check=True)
 for arg in ['--version','--self-test']:
  with (validation/('dynamic-native-'+arg[2:]+'.log')).open('w',encoding='utf-8') as log:subprocess.run([str(package/'app/server-dinamico-p14/openomsi.exe'),arg],stdout=log,stderr=subprocess.STDOUT,timeout=40,check=True)
 for n in ['ATUALIZAR-MEU-HOST.cmd','LEIA-ME.txt']:
  shutil.copy2(ROOT/'build-support/host-personal2202'/n,package/n)
 scripts=package/'scripts';scripts.mkdir()
 for n in ['atualizar_host_pessoal.ps1','test_host_pessoal.ps1','restore_host_personal2202.py','test_p15_host_controls.py']:shutil.copy2(ROOT/'scripts'/n,scripts/n)
 docs=package/'docs';docs.mkdir();shutil.copy2(ROOT/'build-support/host-personal2202/native-stream-build.py',docs/'native-stream-build.py')
 for n in ['LICENSE','NOTICE_FOR_PEDEPE.md','THIRD_PARTY_NOTICES.md']:
  if (ROOT/n).is_file():shutil.copy2(ROOT/n,package/n)
 metadata=json.loads((package/'app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json').read_text(encoding='utf-8-sig'));metadata.update(bridge_line='2.2.0.2',edition='host-pessoal.2',native_executable_unchanged=True);dump(package/'app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json',metadata)
 files=[{'path':f.relative_to(package).as_posix(),'sha256':sha(f.read_bytes())} for f in sorted(package.rglob('*')) if f.is_file() and (f.name in PREVIOUS or 'app/server-dinamico-p14' in f.as_posix())]
 if len(files)!=10:raise ValueError('Unexpected host payload')
 dump(docs/'HOST-PESSOAL-2.2.0.2.json',{'schema':1,'version':'2.2.0.2','edition':'host-pessoal.2','files':files,'previous_hosts':PREVIOUS,'host_protocols':[6,7,14,15],'included_dynamic_protocol':14,'preserves_player_files':True,'public_update_channel_changed':False,'original_p15_build':provenance,'build_commit':os.environ.get('GITHUB_SHA','local')})
 dump(docs/'BINARY-PROVENANCE.json',{'components':provenance,'method':'Only same-length identification literals in PE read-only data were changed. Executable code sections are byte-identical to the approved complete P15 originals. The archived Go source predates these binaries and is deliberately not supplied as corresponding source.','baseline_run':39698116557,'real_two_player_bcs_trip_tested':False})
 print('Complete P15 hosts restored; all executable code bytes preserved; version data and native controls validated.')
if __name__=='__main__':main()
