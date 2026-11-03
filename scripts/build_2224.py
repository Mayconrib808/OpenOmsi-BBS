"""Build full OpenOMSI 0.3.47 over published 2.2.2.4; retain complete legacy hosts."""
import argparse,hashlib,io,json,os,shutil,subprocess,tempfile,zipfile
from pathlib import Path
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload,EXPECTED

ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.2.4-hotfix.3';SEQUENCE=2026110206
COOP='OpenOmsi Coop 0.3.41'  # Existing installation path; actual runtime is 0.3.47.
ORIGIN='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
BASE_UPDATER='53d78b50ae0ac24765f6575dc1bf0faad0a35acdc74b6cb0c41c921ec14c869e'
BASE_COOP='744fd5ecd73578f8461f9945051b93437614f750db89709d950c9019de10383e'
NATIVE_VERSION='0.3.47-coop.16-2.2.2.4'
UPSTREAM='8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6'
REBUILD=('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe')
sha=lambda data:hashlib.sha256(data).hexdigest()
readjson=lambda path:json.loads(path.read_text(encoding='utf-8-sig'))

def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--parts',type=Path,required=True);p.add_argument('--native',type=Path,required=True);p.add_argument('--launcher',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 out=a.output.resolve();out.mkdir(parents=True,exist_ok=False)
 assert build.VERSION==VERSION
 assert subprocess.check_output(['go','version'],text=True).split()[2]=='go1.23.2'
 old=readjson(a.baseline/'build-report.json');spec=readjson(a.baseline/'coop-spec.json')
 updater=(a.baseline/'Atualizar-Bridge.exe').read_bytes()
 assert old['version']=='2.2.2.3' and old['updater_sha256']==sha(updater)==BASE_UPDATER
 assert spec['sha256']==old['coop_archive_sha256']==BASE_COOP
 payload=embedded_payload(updater);assert set(payload)==set(EXPECTED)
 for name,data in payload.items():assert sha(data)==old['components_sha256'][name],name
 runtime_before={name:sha(data)for name,data in payload.items()}
 chunks=[]
 for part in spec['parts']:
  data=(a.parts/'coop-assets'/(part['sha256']+'.bin')).read_bytes()
  assert len(data)==part['size'] and sha(data)==part['sha256'];chunks.append(data)
 packed=b''.join(chunks);assert sha(packed)==BASE_COOP and len(packed)==spec['size']
 with zipfile.ZipFile(io.BytesIO(packed))as z:
  assert set(z.namelist())=={spec['prefix']+name for name in spec['files']}
  files={name:z.read(spec['prefix']+name)for name in spec['files']}
 for name,data in files.items():assert sha(data)==spec['files'][name],name
 before={name:sha(data)for name,data in files.items()}
 evidence=readjson(a.native/'SOURCE.json');pin=readjson(ROOT/'build-support/release2224/native-build.json')
 assert evidence['build_commit']==pin['commit']
 assert evidence['run_id']==pin.get('run_id',int(os.environ['GITHUB_RUN_ID']))
 assert pin['workflow']=='.github/workflows/release-2224-candidate.yml'
 assert evidence['upstream_commit']==UPSTREAM and evidence['version']==NATIVE_VERSION
 assert evidence['winter_clothing_removed'] and evidence['passenger_upstream_tag']=='v0.3.47'
 assert evidence['passenger_upstream_commit']=='8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6'
 assert evidence['paired_gameplay_tested'] is False

 native=(a.native/'openomsi.exe').read_bytes();source=(a.native/'openomsi-source.tar.gz').read_bytes()
 assert sha(native)==evidence['native_sha256'] and sha(source)==evidence['source_sha256'] and native[:2]==b'MZ'
 version=(a.native/'version.txt').read_text();assert NATIVE_VERSION in version
 game=COOP+'/programa/openomsi.exe';server='app/server-dinamico-p16/openomsi.exe';marker_name='app/server-dinamico-p16/BBS-DYNAMIC-SERVER.json'
 files[game]=native;files[server]=native
 # The Tauri lobby is a DIFFERENT executable from the game. The previous
 # 2.2.2.1 release inherited a 2.2.1.7 launcher and silently hid all new
 # settings switches. Refuse to stage another release without its actual UI.
 launcher_name=COOP+'/programa/openomsi-launcher-app.exe'
 launcher_meta=readjson(a.launcher/'BUILD-INFO.json')
 launcher_binary=(a.launcher/'openomsi-launcher-app.exe').read_bytes()
 launcher_pin=readjson(ROOT/'build-support/release2224/native-build.json')
 assert launcher_meta['version']==NATIVE_VERSION, 'Launcher must match the full 0.3.47 game and P16 server'
 assert launcher_meta['source_commit']==launcher_pin['commit'], 'Unexpected launcher source'
 assert launcher_meta['optional_ui_controls']==12 and launcher_meta['passenger_upstream_controls']==3
 assert launcher_meta['path']==launcher_name
 assert sha(launcher_binary)==launcher_meta['launcher_sha256'] and launcher_binary[:2]==b'MZ'
 assert launcher_name in before, 'Missing old launcher: check baseline layout'
 files[launcher_name]=launcher_binary
 marker=json.loads(files['app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json'])
 marker.update(bridge_line=VERSION,source_commit=evidence['commit'],native_version=NATIVE_VERSION,native_sha256=sha(native),protocol=16,version=NATIVE_VERSION,openomsi_version='0.3.47',upstream_commit=UPSTREAM)
 files[marker_name]=(json.dumps(marker,indent=2)+'\n').encode()
 notes=(ROOT/'docs/releases/2.2.2.4-hotfix.3.md').read_bytes()
 info_name=COOP+'/source/BUILD-INFO.json';info=json.loads(files[info_name])
 info.update(version=NATIVE_VERSION,bridge_version=VERSION,game_sha256=sha(native),native_candidate_run=evidence['run_id'],native_candidate_commit=evidence['commit'],native_build_commit=evidence['build_commit'],dynamic_p16_sha256=sha(native),dynamic_p16_version=NATIVE_VERSION,launcher_sha256=sha(launcher_binary),launcher_build_commit=launcher_pin['commit'],upstream_commit=UPSTREAM)
 files[info_name]=(json.dumps(info,indent=2)+'\n').encode()
 files[COOP+'/source/openomsi-2.2.2.4-hotfix.3-source.tar.gz']=source
 files[COOP+'/source/SOURCE-2.2.2.4-hotfix.3.json']=(json.dumps(evidence,indent=2)+'\n').encode()
 files[COOP+'/CHANGELOG-2.2.2.4-hotfix.3.md']=notes
 files[COOP+'/validation/version-2.2.2.4-hotfix.3.txt']=version.encode()
 # Fallback content is installed beside each actual runtime. No commercial map,
 # human, bus, BCS class or user configuration is included.
 assets=a.native/'bcs-assets';assert (assets/'LICENSE-emergency-fallbacks.txt').is_file()
 assert (assets/'Vehicles/OpenOMSI_BCS/Fire/Fire.ovh').is_file(), 'Missing real 3D fire vehicle'
 assert (assets/'Vehicles/OpenOMSI_BCS/Police/Police.ovh').is_file(), 'Missing real 3D police vehicle'
 assert (assets/'Vehicles/OpenOMSI_BCS/Fire/model/Fire.obj').is_file(), 'Missing real 3D fire mesh'
 assert (assets/'Vehicles/OpenOMSI_BCS/Police/model/Police.obj').is_file(), 'Missing real 3D police mesh'
 assert not (assets/'WinterClothes').exists(), 'Winter clothing must not be staged'
 for path in sorted(assets.rglob('*')):
  if not path.is_file():continue
  rel=path.relative_to(assets).as_posix();data=path.read_bytes()
  for prefix in (COOP+'/programa/bcs-assets/','app/server-dinamico-p16/bcs-assets/'):files[prefix+rel]=data
 # Existing DLLs and custom audio stay exact. Reject changed ABI payloads.
 for path in a.native.glob('*.dll'):
  rel=COOP+'/programa/'+path.name
  assert rel in files and sha(path.read_bytes())==sha(files[rel]),'Native DLL differs from retained package: '+path.name
 # Each Windows executable needs its own app-local DLLs. The previous
 # 2.2.2.4 P16 directory omitted these and exited 0xc0000135 on Windows.
 p16_dependencies=('steam_api64.dll','msvcp140.dll','vcruntime140.dll','vcruntime140_1.dll')
 for dll in p16_dependencies:
  retained='app/server-dinamico-p15/'+dll
  assert retained in files, 'Missing trusted P15 runtime: '+dll
  files['app/server-dinamico-p16/'+dll]=files[retained]
 companion=out/'p16-companion-test'/'app/hosts-p16';companion.mkdir(parents=True)
 companion_env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 for name in ('HostAgent.exe','CompanyHost.exe'):
  destination=(companion/name).resolve()
  subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(destination),*build.PROGRAMS[name]],cwd=build.SOURCE,env=companion_env,check=True)
  files['app/hosts-p16/'+name]=destination.read_bytes()
 subprocess.run(['python',str(ROOT/'scripts/test_complete_host_ui.py'),str(companion),'--protocol','16','--large-inventory','--server',str((a.native/'openomsi.exe').resolve()),'--report',str(out/'native-p16-controls.json')],check=True)
 assert readjson(out/'native-p16-controls.json')['status']=='passed'
 assert readjson(out/'native-p16-controls.json')['large_inventory_tested'] is True
 host_manifest_name='docs/HOST-PESSOAL-2.2.0.3.json';host_manifest=json.loads(files[host_manifest_name]);host_manifest['version']=VERSION
 for entry in host_manifest['files']:
  if entry['path']in(server,marker_name,launcher_name):entry['sha256']=sha(files[entry['path']])
 for name in [server,marker_name,'app/hosts-p16/HostAgent.exe','app/hosts-p16/CompanyHost.exe']:
  host_manifest['files'].append(dict(path=name,sha256=sha(files[name])))
 files[host_manifest_name]=(json.dumps(host_manifest,indent=2)+'\n').encode()
 checksum=COOP+'/SHA256.json'
 files[checksum]=(json.dumps({name[len(COOP)+1:]:sha(data)for name,data in files.items()if name.startswith(COOP+'/')and name!=checksum},indent=2)+'\n').encode()
 changed={game,server,launcher_name,marker_name,info_name,host_manifest_name,checksum}
 for name,digest in before.items():
  if name not in changed:assert sha(files[name])==digest,('Unrelated inherited asset changed',name)
 p14={name:digest for name,digest in before.items()if name.startswith('app/server-dinamico-p14/')};assert p14
 assert all(sha(files[name])==digest for name,digest in p14.items())
 p15={name:digest for name,digest in before.items()if name.startswith('app/server-dinamico-p15/')};assert p15
 assert all(sha(files[name])==digest for name,digest in p15.items())
 assert files[game]==files[server]
 prefix='Bridge-2.2.2.4-hotfix.3/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=6)as z:
  for name,data in sorted(files.items()):
   item=zipfile.ZipInfo(prefix+name,(2026,10,31,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;z.writestr(item,data,compresslevel=6)
 packed=bundle.getvalue()
 # This cumulative update includes matching P16 client/server binaries
 # and corresponding source, without the abandoned winter garments. Previous
 # (128/256/512 MiB) package ceilings predated those assets; they were not
 # Cloudflare per-asset limits (actual uploaded ZIP chunks remain <= 16 MiB).
 biggest=max((len(data),name) for name,data in files.items())
 raw_bytes=sum(map(len,files.values()))
 packed_bytes=len(packed)
 print('2.2.2.4 package budget: single=%d MiB, compressed=%d MiB, unpacked=%d MiB' %
       (biggest[0]>>20,packed_bytes>>20,raw_bytes>>20),flush=True)
 print('Largest entry:',biggest[1],flush=True)
 # Keep bounded limits and enforce compression/expanded size independently.
 assert biggest[0]<=256<<20, 'Oversized archive member; reject package'
 assert packed_bytes<=512<<20, 'Compressed update exceeds 512 MiB'
 assert raw_bytes<=1024<<20, 'Expanded update exceeds 1 GiB'
 partdir=out/'coop-assets';partdir.mkdir();parts=[]
 for start in range(0,len(packed),16<<20):
  data=packed[start:start+(16<<20)];digest=sha(data);(partdir/(digest+'.bin')).write_bytes(data);parts.append(dict(sha256=digest,size=len(data)))
 new_spec=dict(files={name:sha(data)for name,data in files.items()},parts=parts,prefix=prefix,sha256=sha(packed),size=len(packed))
 (out/'coop-spec.json').write_text(json.dumps(new_spec,indent=2)+'\n')
 manifest='package main\n\nconst bundledCoopOrigin = '+json.dumps(ORIGIN)+'\nvar bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:'+json.dumps(new_spec['sha256'])+',Size:'+str(new_spec['size'])+',Prefix:'+json.dumps(prefix)+',\nParts:[]coopDistributionPart{\n'
 manifest+=''.join('{SHA256:'+json.dumps(p['sha256'])+',Size:'+str(p['size'])+'},\n'for p in parts)+'},\nFiles:map[string]string{\n'
 manifest+=''.join(json.dumps(name)+':'+json.dumps(digest)+',\n'for name,digest in sorted(new_spec['files'].items()))+'},}\n'
 (build.SOURCE/'coop_bundle_manifest.go').write_text(manifest);(out/'coop_bundle_manifest.go').write_text(manifest)
 package=out/'runtime'
 for name,data in {**payload,**files}.items():
  path=package/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
 env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 with tempfile.TemporaryDirectory(prefix='bridge222-build-')as folder:
  for name in REBUILD:
   flags='-s -w -H=windowsgui -X=main.bundledPluginHostSHA256='+sha(payload['app/compat/omsi-plugin-host32.exe'])
   if name=='Setup.exe':
    work=Path(folder)/'setup';work.mkdir()
    for item in build.PROGRAMS[name]:shutil.copy2(build.SOURCE/item,work/item)
    (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n');resources=setup_resources.prepare(work,build.SOURCE/'resources')
    flags+=' -X=main.bridgeUpdateOrigin='+ORIGIN+' -X=main.bridgeUpdateSequence='+str(SEQUENCE);sources=['.'];local_env=dict(env,GO111MODULE='on')
   else:work=build.SOURCE;sources=build.PROGRAMS[name];local_env=env
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(package/name),*sources],cwd=work,env=local_env,check=True)
   if name=='Setup.exe':setup_resources.verify_executable(package/name,resources)
   payload[name]=(package/name).read_bytes()
 assert all(sha(payload[name])==digest for name,digest in runtime_before.items()if name not in REBUILD)
 assert all(sha(payload[name])==old['components_sha256'][name]for name in('HostAgent.exe','CompanyHost.exe'))
 # One cumulative Bridge suite, with the real new native CLI flags as evidence.
 cli=subprocess.run([str((package/game).resolve()),'--help'],capture_output=True,text=True,timeout=60);assert cli.returncode==0
 # Smoke-test the real installed P16 directory (not the native build folder).
 p16_home=package/'app/server-dinamico-p16'
 for dll in p16_dependencies:
  installed=p16_home/dll
  assert installed.is_file() and sha(installed.read_bytes())==sha(files['app/server-dinamico-p15/'+dll]),dll
 p16_help=subprocess.run([str((p16_home/'openomsi.exe').resolve()),'--help'],cwd=p16_home,capture_output=True,text=True,timeout=60)
 assert p16_help.returncode==0, 'P16 packaged executable failed to start: '+p16_help.stderr[-2000:]
 assert '--server' in (p16_help.stdout+p16_help.stderr), 'P16 server CLI flag missing'
 helpfile=out/'native-help.txt';helpfile.write_text(cli.stdout+cli.stderr)
 subprocess.run(['python',str(ROOT/'scripts/build_2203.py'),'--tests-only'],env=dict(os.environ,OPENOMSI_FUEL_CLI_HELP=str(helpfile),OPENOMSI_GATEWAY_TEST_SERVER=str((a.native/'openomsi.exe').resolve()),OPENOMSI_GATEWAY_TEST_PROTOCOL='16'),check=True)
 previews=out/'setup-previews'
 try:
  subprocess.run([str((package/'Setup.exe').resolve()),'--gui-smoke'],cwd=package,env=dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(previews)),timeout=60,check=True)
 except (subprocess.CalledProcessError,subprocess.TimeoutExpired):
  diagnostic=package/'app'/'setup-v1.1.3.log'
  print('SETUP GUI SMOKE FAILED. Native Setup diagnostics:',flush=True)
  if diagnostic.exists():
   print(diagnostic.read_text(encoding='utf-8',errors='replace')[-12000:],flush=True)
  else:
   print('Missing expected diagnostic file: '+str(diagnostic),flush=True)
   for path in sorted(package.rglob('*setup*.log')):print('Found log:',path,flush=True)
  print('Generated GUI previews:',[p.name for p in previews.glob('*')] if previews.exists() else 'none',flush=True)
  raise
 subprocess.run(['python',str(ROOT/'scripts/test_setup_update_service_windows.py'),str(package)],check=True)
 embedded=build.SOURCE/'bridge_update_payload.zip';assert not embedded.exists()
 try:
  with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9)as z:
   for name,data in payload.items():z.writestr(name,data)
  subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=dict(env,GOARCH='amd64'),check=True)
 finally:embedded.unlink(missing_ok=True)
 final=(out/'Atualizar-Bridge.exe').read_bytes();assert embedded_payload(final)==payload and len(final)<=25<<20
 report=dict(version=VERSION,sequence=SEQUENCE,update_origin=ORIGIN,updater_sha256=sha(final),installer_size=len(final),
  coop_archive_sha256=new_spec['sha256'],components_sha256={n:sha(b)for n,b in payload.items()},retained_runtime_sha256={n:h for n,h in runtime_before.items()if n not in REBUILD},
  previous_installer_sha256=BASE_UPDATER,previous_coop_sha256=BASE_COOP,retained_host_hashes={n:sha(payload[n])for n in('HostAgent.exe','CompanyHost.exe')},
  host_preferences_limit=32<<20,large_host_inventory_ui_tested=True,complete_hosts_unchanged=True,complete_hosts_rebuilt=False,p14_server_unchanged=True,p15_server_unchanged=True,new_p16_host_ui_tested=True,p16_host_hashes={n:sha(files['app/hosts-p16/'+n])for n in('HostAgent.exe','CompanyHost.exe')},dynamic_p16_sha256=sha(native),dynamic_p16_version=NATIVE_VERSION,
  native_version=NATIVE_VERSION,native_candidate_run=evidence['run_id'],native_candidate_commit=evidence['commit'],native_build_commit=evidence['build_commit'],upstream_commit=UPSTREAM,
  native_client_sha256={'openomsi.exe':sha(native)},native_source_sha256=sha(source),coop_protocol=16,custom_sounds_unchanged=True,desktop_launcher_unchanged=False,desktop_launcher_sha256=sha(launcher_binary),desktop_launcher_commit=launcher_pin['commit'],desktop_launcher_run=evidence['run_id'],
  bcs_capture_normal_launch_tested=True,bcs_single_and_company_event_fixture_tested=True,setup_changelog_languages=['pt','en','de'],setup_changelog_ui_tested=True,
  winter_clothing_removed=True,passenger_upstream_tag='v0.3.47',passenger_upstream_commit='8c4a0d3fd4439e49d18c2812bbd4d91cdd0af7f6',brazilian_passenger_recordings=8,passenger_defaults_enabled=True,passenger_coop_handback=True,feature_event_audit_included=True,per_release_changelog_including_hotfixes=True,
  setup_live_update_ui_tested=True,bridge_regressions_tested=True,windows_gameplay_tested=False,paired_gameplay_tested=False,
  updater_architecture="amd64",rollback_storage="disk")
 subprocess.run(['python',str(ROOT/'scripts/test_2222_test2_install.py'),str(out),'--baseline',str(a.baseline)],check=True)
 report['actual_updater_first_attempt_windows_tested']=True
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('2.2.2.4 cumulative update built with matching desktop launcher:',report['updater_sha256'])

if __name__=='__main__':main()
