"""Fix BCS native launch; keep the entire 2.2.1.3 game distribution byte-identical."""
import argparse,hashlib,io,json,os,shutil,subprocess,tempfile,zipfile,sys
from pathlib import Path
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload,EXPECTED
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.4'
SEQUENCE=2026102601
COOP='OpenOmsi Coop 0.3.41'
sha=lambda b:hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--prior',type=Path,required=True);a=p.parse_args()
 assert build.VERSION==VERSION
 out=ROOT/'build/release-2214';out.mkdir(parents=True,exist_ok=False)
 previous=json.loads((a.prior/'build-report.json').read_text(encoding='utf-8-sig'))
 spec=json.loads((a.prior/'coop-spec.json').read_text(encoding='utf-8-sig'))
 updater=(a.prior/'Atualizar-Bridge.exe').read_bytes()
 assert previous['version']=='2.2.1.3' and sha(updater)==previous['updater_sha256']=='810b5e530b60f0d719a1c367701dc8c0ecd3d2748562a85a541fa779d1156617'
 assert spec['sha256']==previous['coop_archive_sha256']=='8f01eaedb278549b3de33773e43acf77cf6f3d440989e4b78ea9f5eedb24895d'
 payload=embedded_payload(updater);assert set(payload)==set(EXPECTED)
 for n,b in payload.items():assert sha(b)==previous['components_sha256'][n],n
 runtime_before={n:sha(b) for n,b in payload.items()}
 parts=[]
 for part in spec['parts']:
  b=(a.prior/'coop-assets'/(part['sha256']+'.bin')).read_bytes()
  assert len(b)==part['size'] and sha(b)==part['sha256'];parts.append(b)
 packed=b''.join(parts);assert sha(packed)==spec['sha256']
 with zipfile.ZipFile(io.BytesIO(packed)) as z:
  files={n:z.read(spec['prefix']+n) for n in spec['files']}
 for n,b in files.items():assert sha(b)==spec['files'][n],n
 # Replay the adapter's real Go-generated startup options against the exact
 # Windows game from the released archive; no game/host/server rebuild.
 check=out/'native-cli';check.mkdir()
 exe=check/'openomsi.exe';exe.write_bytes(files[COOP+'/programa/openomsi.exe'])
 for n,b in files.items():
  if n.startswith(COOP+'/programa/') and n.endswith('.dll') and '/' not in n[len(COOP+'/programa/'):]: (check/Path(n).name).write_bytes(b)
 native=subprocess.run([str(exe.resolve()),'--help'],capture_output=True,text=True,timeout=30)
 assert native.returncode==0,(native.stdout,native.stderr)
 helpfile=check/'help.txt';helpfile.write_text(native.stdout+native.stderr,encoding='utf-8')
 assert '--setvar' in helpfile.read_text() and '--keep-fuel' not in helpfile.read_text()
 broken=subprocess.run([str(exe.resolve()),'--keep-fuel','--help'],capture_output=True,text=True,timeout=30)
 assert broken.returncode!=0 and "unexpected argument '--keep-fuel'" in broken.stdout+broken.stderr
 cases=check/'cases.json'
 testenv=dict(os.environ,OPENOMSI_FUEL_CLI_HELP=str(helpfile.resolve()),OPENOMSI_FUEL_CLI_CASES=str(cases.resolve()))
 subprocess.run([sys.executable,str(ROOT/'scripts/build_2203.py'),'--tests-only'],env=testenv,check=True)
 commands=json.loads(cases.read_text());assert len(commands)==3
 for args in commands:
  launch=subprocess.run([str(exe.resolve()),*args,'--help'],capture_output=True,text=True,timeout=30)
  assert launch.returncode==0,(args,launch.stdout,launch.stderr)
 (out/'native-cli-evidence.json').write_text(json.dumps(dict(game_sha256=sha(exe.read_bytes()),bug_reproduced=True,cases=commands,all_commands_accepted=True),indent=2)+'\n')
 for n in ('coop-spec.json','coop_bundle_manifest.go'):shutil.copy2(a.prior/n,out/n)
 shutil.copytree(a.prior/'coop-assets',out/'coop-assets')
 shutil.copy2(a.prior/'coop_bundle_manifest.go',build.SOURCE/'coop_bundle_manifest.go')
 package=out/'runtime';package.mkdir()
 for n,b in payload.items():
  target=package/n;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
 env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 assert subprocess.check_output(['go','version'],text=True).split()[2]=='go1.23.2'
 with tempfile.TemporaryDirectory() as tmp:
  for name in ('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'):
   flags='-s -w -H=windowsgui -X=main.bundledPluginHostSHA256='+sha(payload['app/compat/omsi-plugin-host32.exe'])
   if name=='Setup.exe':
    work=Path(tmp)/'setup';work.mkdir()
    for f in build.PROGRAMS[name]:shutil.copy2(build.SOURCE/f,work/f)
    (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n');resource=setup_resources.prepare(work,build.SOURCE/'resources');args=['.'];cmd_env=dict(env,GO111MODULE='on')
    flags+=' -X=main.bridgeUpdateOrigin=https://openomsi-bbs-updates.maycongamesmaximo.workers.dev -X=main.bridgeUpdateSequence='+str(SEQUENCE)
   else:work=build.SOURCE;args=build.PROGRAMS[name];cmd_env=env
   target=package/name
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(target),*args],cwd=work,env=cmd_env,check=True)
   if name=='Setup.exe':setup_resources.verify_executable(target,resource)
   payload[name]=target.read_bytes()
  embedded=build.SOURCE/'bridge_update_payload.zip';assert not embedded.exists()
  try:
   with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
    for name,b in payload.items():z.writestr(name,b)
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
  finally:embedded.unlink(missing_ok=True)
 final=(out/'Atualizar-Bridge.exe').read_bytes();assert embedded_payload(final)==payload and len(final)<=25<<20
 changed_runtime={'Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'}
 for name,h in runtime_before.items():
  if name not in changed_runtime:assert sha(payload[name])==h,('Runtime regression',name)
 assert all(sha(payload[n])==runtime_before[n] for n in ('HostAgent.exe','CompanyHost.exe'))
 report=dict(previous)
 report.update(version=VERSION,sequence=SEQUENCE,updater_sha256=sha(final),installer_size=len(final),components_sha256={n:sha(b)for n,b in payload.items()},retained_runtime_sha256={n:h for n,h in runtime_before.items()if n not in changed_runtime},previous_installer_sha256=previous['updater_sha256'],previous_coop_sha256=spec['sha256'],coop_archive_unchanged=True,gameplay_files_unchanged=True,fuel_cli_tested=True,fuel_cli_cases=3)
 assert (out/'coop-spec.json').read_bytes()==(a.prior/'coop-spec.json').read_bytes()
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('2.2.1.4 verified: original launch failure reproduced, 3 corrected fuel commands accepted; complete native distribution unchanged')
if __name__=='__main__':main()
