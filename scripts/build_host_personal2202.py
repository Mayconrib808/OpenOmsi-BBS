"""Build only the personal host update; never stage any player/game executable."""
import argparse,hashlib,json,os,shutil,subprocess,sys,zipfile
from pathlib import Path
import build
import build_2202

ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'build/host-personal2202'
PACKAGE='Bridge_2.2.0.2_Host_Particular_Maycon'
NATIVE_SHA='8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
PREVIOUS={
 'HostAgent.exe':['a483e435a23b9a74276fb045945ce58357ab5ea99f8759ddc2dddae41a41dba4','9611698e6318da299c32841c08a770959693dbaa495c33caa6671ff3649cadd3'],
 'CompanyHost.exe':['3d57a4733cbfb8bec677c29d888de339a92ae43c161f9e48ccc9b7a97c7d6cb3','ebfd3836d70ea342e65494309496d85f22232af540faacbc62dbcf6df66beb5c']}
def sha(data):return hashlib.sha256(data).hexdigest()
def dump(path,obj):path.write_text(json.dumps(obj,ensure_ascii=False,indent=2)+'\n',encoding='utf-8',newline='\n')
def command(args,**kw):subprocess.run(args,check=True,**kw)
def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--finalize',action='store_true');a=p.parse_args()
 package=OUT/PACKAGE
 if a.finalize:
  for major in (5,7):
   result=json.loads((package/f'validation/installer-powershell-{major}.json').read_text(encoding='utf-8-sig'))
   if result['status']!='passed' or len(result['passed'])<12:raise ValueError('Missing real installer regressions')
  scan=json.loads((package/'validation/defender.json').read_text(encoding='utf-8-sig'))
  if scan['status']!='passed':raise ValueError('Native Defender check did not pass')
  # The outer ZIP is scanned again by the workflow after compression.
  sums=''.join(sha(f.read_bytes())+'  '+f.relative_to(package).as_posix()+'\n' for f in sorted(package.rglob('*')) if f.is_file())
  (package/'SHA256SUMS.txt').write_text(sums,encoding='utf-8',newline='\n')
  target=OUT/(PACKAGE+'.zip')
  with zipfile.ZipFile(target,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
   for f in sorted(package.rglob('*')):
    if f.is_file():z.write(f,f.relative_to(package).as_posix())
  digest=sha(target.read_bytes());(OUT/(PACKAGE+'.zip.sha256')).write_text(digest+'  '+target.name+'\n')
  print('HOST2202_FINAL='+json.dumps({'sha256':digest,'size':target.stat().st_size,'name':target.name}));return
 go=shutil.which('go')
 if not go:raise ValueError('Go unavailable')
 if 'go1.23.2' not in subprocess.check_output([go,'version'],text=True):raise ValueError('Unexpected Go toolchain')
 OUT.mkdir(parents=True,exist_ok=True);package.mkdir()
 build_2202.tests(go)
 baseline=OUT/'baseline';baseline.mkdir()
 with zipfile.ZipFile(a.baseline) as z:
  if z.testzip():raise ValueError('Corrupt baseline ZIP')
  for name in ['HostAgent.exe','CompanyHost.exe']:
   data=z.read(name)
   if sha(data) not in PREVIOUS[name]:raise ValueError('Unrecognized baseline host')
   (baseline/name).write_bytes(data)
  native=[n for n in z.namelist() if n.startswith('app/server/') and not n.endswith('/')]
  for name in native:
   data=z.read(name);local=baseline/name;local.parent.mkdir(parents=True,exist_ok=True);local.write_bytes(data)
   destination=package/'app/server-dinamico-p14'/Path(name).name;destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(data)
 if sha((package/'app/server-dinamico-p14/openomsi.exe').read_bytes())!=NATIVE_SHA:raise ValueError('The exact dynamic server was not preserved')
 env=os.environ.copy();env.update(GO111MODULE='off',GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local')
 for name in ['HostAgent.exe','CompanyHost.exe']:
  command([go,'build','-trimpath','-buildvcs=false','-ldflags=-s -w'+(' -H=windowsgui' if name=='HostAgent.exe' else ''),'-o',str(package/name),*build.PROGRAMS[name]],cwd=ROOT/'source',env=env)
 if b'Host Agent 2.2.0.2' not in (package/'HostAgent.exe').read_bytes():raise ValueError('Wrong real HostAgent version')
 validation=package/'validation';validation.mkdir()
 with (validation/'hostagent-native-smoke.log').open('w',encoding='utf-8') as log:
  command([str(package/'HostAgent.exe'),'--gui-smoke'],stdout=log,stderr=subprocess.STDOUT,timeout=40)
 for option in ['--version','--self-test']:
  with (validation/('dynamic-native-'+option[2:]+'.log')).open('w',encoding='utf-8') as log:
   command([str(package/'app/server-dinamico-p14/openomsi.exe'),option],stdout=log,stderr=subprocess.STDOUT,timeout=40)
 with (validation/'companyhost-help.log').open('w',encoding='utf-8') as log:
  command([str(package/'CompanyHost.exe'),'--help'],stdout=log,stderr=subprocess.STDOUT,timeout=15)
 for name in ['ATUALIZAR-MEU-HOST.cmd','LEIA-ME.txt']:
  shutil.copy2(ROOT/'build-support/host-personal2202'/name,package/name)
 scripts=package/'scripts';scripts.mkdir()
 for name in ['atualizar_host_pessoal.ps1','test_host_pessoal.ps1','recompilar_host.py']:
  shutil.copy2(ROOT/'scripts'/name,scripts/name)
 docs=package/'docs';docs.mkdir()
 dump(docs/'HOST-BUILD-FILES.json',{n:build.PROGRAMS[n] for n in PREVIOUS})
 shutil.copy2(ROOT/'build-support/host-personal2202/native-stream-build.py',docs/'native-stream-build.py')
 for name in ['LICENSE','NOTICE_FOR_PEDEPE.md','THIRD_PARTY_NOTICES.md']:
  if (ROOT/name).is_file():shutil.copy2(ROOT/name,package/name)
 metadata=json.loads((package/'app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json').read_text(encoding='utf-8-sig'))
 metadata.update(bridge_line='2.2.0.2',edition='host-pessoal.1',native_executable_unchanged=True,original_native_run=39698116557)
 dump(package/'app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json',metadata)
 files=[{'path':f.relative_to(package).as_posix(),'sha256':sha(f.read_bytes())} for f in sorted(package.rglob('*')) if f.is_file() and (f.name in ['HostAgent.exe','CompanyHost.exe'] or 'app/server-dinamico-p14' in f.as_posix())]
 if len(files)!=10:raise ValueError('Unexpected host payload')
 manifest={'schema':1,'version':'2.2.0.2','edition':'host-pessoal.1','files':files,'previous_hosts':PREVIOUS,'host_protocols':[6,7,14,15],'included_dynamic_protocol':14,'preserves_player_files':True,'preserves_custom_server':True,'public_update_channel_changed':False,'source_base':os.environ.get('GITHUB_SHA','local')}
 dump(docs/'HOST-PESSOAL-2.2.0.2.json',manifest)
 shutil.copytree(ROOT/'source',package/'source',ignore=shutil.ignore_patterns('*.exe','*.dll','*.syso','__pycache__'))
 dump(docs/'SOURCE-AND-VALIDATION.json',{'go':'go1.23.2','source_base':os.environ.get('GITHUB_SHA','local'),'host_go_tests_and_vet':'passed','native_gui_smoke':'passed','dynamic_server_sha256':NATIVE_SHA,'real_two_player_bcs_trip_tested':False})
 print('Built source-backed host-only candidate, native server unchanged, native controls passed.')
if __name__=='__main__':main()
