#!/usr/bin/env python3
"""Package the exact approved 0.3.50/P17 bytes over the full released 2.2.3.1.

The latest Hotfix 2 activation source is retained. Old P14/P15/P16 hosts and
servers remain byte-identical; P17 companions are built and tested separately.
"""
from pathlib import Path
import argparse, hashlib, io, json, os, shutil, subprocess, tempfile, zipfile
import build, build_bridge_update, setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED
ROOT=Path(__file__).resolve().parents[1]
SRC=build.SOURCE
VERSION='2.2.3.2'; SEQUENCE=2026110601
ORIGIN='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
NATIVE='0.3.50-coop.17-2.2.3.2-candidate'
COMMIT='9b1d236c84b8af2f4c5a13d7c110b0db5e0c1c68'
NATIVE_RUN=42202071912
COOP='OpenOmsi Coop 0.3.41'
BASE_SHA='c0cf59361d96dbfe97e414d134b339db62b429ea805cd34387505e53eb664275'
BASE_COOP='34c80650a32b26348b9da3e64f127429b20d184fad673ed9e47d38545f4fccea'
sha=lambda b:hashlib.sha256(b).hexdigest()
def js(p): return json.loads(p.read_text(encoding='utf-8-sig'))
def run(args,**kw):
 print('RUN',args[0],args[1:3],flush=True);subprocess.run(args,check=True,**kw)
def main():
 p=argparse.ArgumentParser();p.add_argument('--approved',type=Path,required=True);p.add_argument('--native',type=Path,required=True);p.add_argument('--source',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 approved=a.approved.resolve();native_dir=a.native.resolve();out=a.output.resolve();out.mkdir(parents=True,exist_ok=False)
 assert build.VERSION==VERSION
 assert subprocess.check_output(['go','version'],text=True).split()[2]=='go1.23.2'
 old=js(approved/'build-report.json');spec=js(approved/'coop-spec.json');base=(approved/'Atualizar-Bridge.exe').read_bytes()
 assert old['version']=='2.2.3.1' and old['sequence']==2026110501
 assert sha(base)==old['updater_sha256']==BASE_SHA and spec['sha256']==old['coop_archive_sha256']==BASE_COOP
 payload=embedded_payload(base);assert set(payload)==set(EXPECTED)
 for n,b in payload.items():assert sha(b)==old['components_sha256'][n],n
 portable=approved/'OpenOMSI-BCS-Bridge-2.2.3.1-FULL.zip'
 assert sha(portable.read_bytes())==old['portable_sha256']
 runtime=out/'runtime'
 with zipfile.ZipFile(portable) as z:
  for item in z.infolist():
   if item.is_dir():continue
   dest=(runtime/item.filename).resolve();assert dest.is_relative_to(runtime)
   dest.parent.mkdir(parents=True,exist_ok=True)
   with z.open(item) as inp,dest.open('wb') as dst:shutil.copyfileobj(inp,dst,1<<20)
 for n,h in {**spec['files'],**old['components_sha256']}.items():assert sha((runtime/n).read_bytes())==h,n
 previous={n:sha((runtime/n).read_bytes()) for n in spec['files']}
 meta=js(native_dir/'BUILD-INFO.json');assert meta['version']==NATIVE and meta['source_commit']==COMMIT and meta['protocol']==17
 assert meta['validation']=='compiled-only-not-live-tested'
 assert 'exit code: 0' in (native_dir/'native-self-test.txt').read_text(encoding='utf-8-sig')
 binaries={'openomsi.exe':meta['native_sha256'],'openomsi-launcher-app.exe':meta['desktop_sha256'],'openomsi-launcher.exe':meta['cli_sha256']}
 assert binaries['openomsi.exe']=='0d73ff860bb473363f7c4feb83bd68426e7eaf1cedd97d7afbc50a1eaf3a607e'
 for name,digest in binaries.items():
  content=(native_dir/name).read_bytes();assert content[:2]==b'MZ' and sha(content)==digest
  (runtime/COOP/'programa'/name).write_bytes(content)
  if name=='openomsi.exe':(runtime/'app/server-dinamico-p17/openomsi.exe').write_bytes(content)
 steam=(native_dir/'steam_api64.dll').read_bytes()
 for folder in (runtime/COOP/'programa',runtime/'app/server-dinamico-p17'):
  assert sha((folder/'steam_api64.dll').read_bytes())==sha(steam),'Unexpected Steam ABI change'
 source=a.source.resolve();assert source.is_file()
 source_dest=runtime/COOP/'source/openomsi-2.2.3.2-source.tar.gz';shutil.copy2(source,source_dest)
 source_info=dict(bridge_version=VERSION,version=NATIVE,commit=COMMIT,source_commit=COMMIT,run_id=NATIVE_RUN,upstream_commit='11ff78bab1998963162be97c49e909a2996710e1',native_sha256=meta['native_sha256'],source_sha256=sha(source.read_bytes()),protocol=17,full_upstream_update=True,windows_gameplay_tested=False,paired_gameplay_tested=False)
 (runtime/COOP/'source/SOURCE-2.2.3.2.json').write_text(json.dumps(source_info,indent=2)+'\n')
 (runtime/COOP/'CHANGELOG-2.2.3.2.md').write_bytes((ROOT/'docs/releases/2.2.3.2.md').read_bytes())
 marker_path=runtime/'app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json';marker=js(marker_path)
 marker.update(bridge_line=VERSION,source_commit=COMMIT,native_version=NATIVE,native_sha256=meta['native_sha256'],protocol=17,version=NATIVE,openomsi_version='0.3.50',upstream_commit=source_info['upstream_commit']);marker_path.write_text(json.dumps(marker,indent=2)+'\n')
 info_path=runtime/COOP/'source/BUILD-INFO.json';info=js(info_path)
 info.update(version=NATIVE,bridge_version=VERSION,game_sha256=meta['native_sha256'],native_candidate_run=NATIVE_RUN,native_candidate_commit=COMMIT,native_build_commit=COMMIT,dynamic_p17_sha256=meta['native_sha256'],dynamic_p17_version=NATIVE,launcher_sha256=meta['desktop_sha256'],launcher_build_commit=COMMIT,upstream_commit=source_info['upstream_commit']);info_path.write_text(json.dumps(info,indent=2)+'\n')
 host_dir=runtime/'app/hosts-p17'
 host_env=dict(os.environ,GOOS='windows',GOARCH='amd64',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 for name in ('HostAgent.exe','CompanyHost.exe'):
  flags='-s -w'+(' -H=windowsgui' if name=='HostAgent.exe' else '')
  run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(host_dir/name),*build.PROGRAMS[name]],cwd=SRC,env=host_env)
 run(['python',str(ROOT/'scripts/test_complete_host_ui.py'),str(host_dir),'--protocol','17','--large-inventory','--server',str(runtime/'app/server-dinamico-p17/openomsi.exe'),'--report',str(out/'native-p17-controls.json')])
 assert js(out/'native-p17-controls.json')['status']=='passed'
 host_manifest_path=runtime/'docs/HOST-PESSOAL-2.2.0.3.json';host_manifest=js(host_manifest_path);host_manifest['version']=VERSION
 for entry in host_manifest['files']:
  entry['sha256']=sha((runtime/entry['path']).read_bytes())
 host_manifest_path.write_text(json.dumps(host_manifest,indent=2)+'\n')
 checksums=runtime/COOP/'SHA256.json'
 checksums.write_text(json.dumps({f.relative_to(runtime/COOP).as_posix():sha(f.read_bytes()) for f in sorted((runtime/COOP).rglob('*')) if f.is_file() and f!=checksums},indent=2)+'\n')
 changed={COOP+'/programa/'+n for n in binaries}|{'app/server-dinamico-p17/openomsi.exe','app/server-dinamico-p17/BBS-DYNAMIC-SERVER.json','app/hosts-p17/HostAgent.exe','app/hosts-p17/CompanyHost.exe',COOP+'/source/BUILD-INFO.json',COOP+'/SHA256.json','docs/HOST-PESSOAL-2.2.0.3.json'}
 for n,h in previous.items():
  if n not in changed:assert sha((runtime/n).read_bytes())==h,('Unrelated asset changed',n)
 files={n: (runtime/n).read_bytes() for n in previous}
 for n in (COOP+'/source/openomsi-2.2.3.2-source.tar.gz',COOP+'/source/SOURCE-2.2.3.2.json',COOP+'/CHANGELOG-2.2.3.2.md'):files[n]=(runtime/n).read_bytes()
 prefix='Bridge-2.2.3.2/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
  for n,b in sorted(files.items()):z.writestr(prefix+n,b)
 packed=bundle.getvalue();assert len(packed)<=512<<20 and sum(map(len,files.values()))<=1536<<20
 partdir=out/'coop-assets';partdir.mkdir();parts=[]
 for pos in range(0,len(packed),16<<20):
  b=packed[pos:pos+(16<<20)];h=sha(b);(partdir/(h+'.bin')).write_bytes(b);parts.append(dict(sha256=h,size=len(b)))
 new_spec=dict(files={n:sha(b) for n,b in files.items()},parts=parts,prefix=prefix,sha256=sha(packed),size=len(packed))
 (out/'coop-spec.json').write_text(json.dumps(new_spec,indent=2)+'\n')
 manifest='package main\n\nconst bundledCoopOrigin = '+json.dumps(ORIGIN)+'\nvar bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:'+json.dumps(new_spec['sha256'])+',Size:'+str(new_spec['size'])+',Prefix:'+json.dumps(prefix)+',\nParts:[]coopDistributionPart{\n'
 manifest+=''.join('{SHA256:'+json.dumps(x['sha256'])+',Size:'+str(x['size'])+'},\n' for x in parts)+'},\nFiles:map[string]string{\n'
 manifest+=''.join(json.dumps(n)+':'+json.dumps(h)+',\n' for n,h in sorted(new_spec['files'].items()))+'},}\n'
 (SRC/'coop_bundle_manifest.go').write_text(manifest)
 del files,packed,bundle
 env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GO111MODULE='on',GOTOOLCHAIN='local',GOEXPERIMENT='')
 pinned=['PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe','app/compat/omsi-plugin-host32.exe']
 with tempfile.TemporaryDirectory(prefix='bbs-2232-') as t:
  work=Path(t)
  for n in build.PROGRAMS['Setup.exe']:shutil.copy2(SRC/n,work/n)
  (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
  resources=setup_resources.prepare(work,SRC/'resources')
  flags='-s -w -H=windowsgui -X=main.bundledPluginHostSHA256='+sha(payload['app/compat/omsi-plugin-host32.exe'])+' -X=main.activationPinnedRuntimeSHA256='+':'.join(sha(payload[x]) for x in pinned)+' -X=main.bridgeUpdateOrigin='+ORIGIN+' -X=main.bridgeUpdateSequence='+str(SEQUENCE)
  run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(runtime/'Setup.exe'),'.'],cwd=work,env=env)
  setup_resources.verify_executable(runtime/'Setup.exe',resources)
 run([str(runtime/'Setup.exe'),'--verify-activation'],cwd=runtime,timeout=90)
 # Reproduce a stale Setup checksum left by the prior installation.
 (runtime/'docs/SHA256.txt').write_text(old['components_sha256']['Setup.exe']+'  Setup.exe\n'+''.join(old['components_sha256'][n]+'  '+n+'\n' for n in pinned))
 run([str(runtime/'Setup.exe'),'--verify-activation'],cwd=runtime,timeout=90)
 (runtime/'docs/SHA256.txt').unlink()
 game=runtime/COOP/'programa/openomsi.exe'
 with game.open('r+b') as f:f.seek(64);original=f.read(1);f.seek(64);f.write(bytes([original[0]^1]))
 tamper=subprocess.run([str(runtime/'Setup.exe'),'--verify-activation'],cwd=runtime,timeout=90)
 assert tamper.returncode!=0,'Tampered game accepted'
 with game.open('r+b') as f:f.seek(64);f.write(original)
 for game_dir in (runtime/COOP/'programa',runtime/'app/server-dinamico-p17'):
  run([str(game_dir/'openomsi.exe'),'--self-test'],cwd=game_dir,timeout=90)
 run([str(runtime/'Setup.exe'),'--gui-smoke'],cwd=runtime,env=dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(out/'setup-previews')),timeout=120)
 payload['Setup.exe']=(runtime/'Setup.exe').read_bytes()
 embedded=SRC/'bridge_update_payload.zip';assert not embedded.exists()
 try:
  with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
   for n,b in sorted(payload.items()):z.writestr(n,b)
  flags='-s -w -H=windowsgui -X=main.bridgeUpdateOrigin='+ORIGIN+' -X=main.bridgeUpdateSequence='+str(SEQUENCE)
  run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=SRC,env=dict(env,GOARCH='amd64',GO111MODULE='off'))
 finally:embedded.unlink(missing_ok=True)
 installer=(out/'Atualizar-Bridge.exe').read_bytes();assert embedded_payload(installer)==payload and len(installer)<25<<20
 run(['python',str(ROOT/'scripts/test_2222_test2_install.py'),str(out),'--baseline',str(approved)],timeout=420)
 report=dict(old)
 report.update(version=VERSION,sequence=SEQUENCE,updater_sha256=sha(installer),installer_size=len(installer),setup_sha256=sha(payload['Setup.exe']),components_sha256={n:sha(b) for n,b in payload.items()},coop_archive_sha256=new_spec['sha256'],native_version=NATIVE,native_candidate_run=NATIVE_RUN,native_candidate_commit=COMMIT,native_build_commit=COMMIT,native_source_sha256=source_info['source_sha256'],upstream_commit=source_info['upstream_commit'],passenger_upstream_tag='v0.3.50',passenger_upstream_commit=source_info['upstream_commit'],dynamic_p17_sha256=meta['native_sha256'],dynamic_p17_version=NATIVE,native_client_sha256={'openomsi.exe':meta['native_sha256']},desktop_launcher_sha256=meta['desktop_sha256'],desktop_launcher_cli_sha256=meta['cli_sha256'],desktop_launcher_commit=COMMIT,desktop_launcher_run=NATIVE_RUN,p17_host_ui_tested=True,p17_hosts_rebuilt_from_source=True,p17_host_hashes={n:sha((host_dir/n).read_bytes()) for n in ('HostAgent.exe','CompanyHost.exe')},windows_setup_activation_tested=True,stale_legacy_sha256_activation_tested=True,installer_first_click_tested=True,wheelchair_universal_alight_hud=True,wheelchair_signal_reset=True,other_six_components_unchanged=True,signed=False,windows_gameplay_tested=False,paired_gameplay_tested=False)
 for obsolete in ('portable_sha256','portable_size','portable_file'):report.pop(obsolete,None)
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('CUMULATIVE 2.2.3.2 APPROVED',report['updater_sha256'],report['installer_size'],len(parts),'parts',flush=True)
if __name__=='__main__':main()
