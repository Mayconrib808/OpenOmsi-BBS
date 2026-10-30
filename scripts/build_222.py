"""Build the 2.2.2 cumulative update over exact Hotfix 3, retaining complete hosts."""
import argparse,hashlib,io,json,os,shutil,subprocess,tempfile,zipfile
from pathlib import Path
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload,EXPECTED

ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.2';SEQUENCE=2026103002
COOP='OpenOmsi Coop 0.3.41'  # Existing installation path; actual runtime is 0.3.46.
ORIGIN='https://openomsi-bbs-updates.maycongamesmaximo.workers.dev'
BASE_UPDATER='cd055e9a00c2b0459128c088f93b05b7cfd8668bdd42616e582673cf338f4f6b'
BASE_COOP='bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574'
NATIVE_VERSION='0.3.46-coop.15-2.2.2'
UPSTREAM='17be22987d7c01c813a1473e7696fc71fb3b1d89'
REBUILD=('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe','app/compat/Omsi.exe')
sha=lambda data:hashlib.sha256(data).hexdigest()
readjson=lambda path:json.loads(path.read_text(encoding='utf-8-sig'))

def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--parts',type=Path,required=True);p.add_argument('--native',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
 out=a.output.resolve();out.mkdir(parents=True,exist_ok=False)
 assert build.VERSION==VERSION
 assert subprocess.check_output(['go','version'],text=True).split()[2]=='go1.23.2'
 old=readjson(a.baseline/'build-report.json');spec=readjson(a.baseline/'coop-spec.json')
 updater=(a.baseline/'Atualizar-Bridge.exe').read_bytes()
 assert old['version']=='2.2.1.9-hotfix.3' and old['updater_sha256']==sha(updater)==BASE_UPDATER
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
 evidence=readjson(a.native/'SOURCE.json');pin=readjson(ROOT/'build-support/release222/native-build.json')
 assert evidence['build_commit']==pin['commit'] and evidence['run_id']==pin['run_id']
 assert evidence['upstream_commit']==UPSTREAM and evidence['version']==NATIVE_VERSION
 native=(a.native/'openomsi.exe').read_bytes();source=(a.native/'openomsi-source.tar.gz').read_bytes()
 assert sha(native)==evidence['native_sha256'] and sha(source)==evidence['source_sha256'] and native[:2]==b'MZ'
 version=(a.native/'version.txt').read_text();assert NATIVE_VERSION in version
 game=COOP+'/programa/openomsi.exe';server='app/server-dinamico-p15/openomsi.exe';marker_name='app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json'
 files[game]=native;files[server]=native
 marker=json.loads(files[marker_name])
 marker.update(bridge_line=VERSION,source_commit=evidence['commit'],native_version=NATIVE_VERSION,native_sha256=sha(native),protocol=15,version=NATIVE_VERSION,openomsi_version='0.3.46',upstream_commit=UPSTREAM)
 files[marker_name]=(json.dumps(marker,indent=2)+'\n').encode()
 notes=(ROOT/'docs/releases/2.2.2.md').read_bytes()
 info_name=COOP+'/source/BUILD-INFO.json';info=json.loads(files[info_name])
 info.update(version=NATIVE_VERSION,bridge_version=VERSION,game_sha256=sha(native),native_candidate_run=evidence['run_id'],native_candidate_commit=evidence['commit'],native_build_commit=evidence['build_commit'],dynamic_p15_sha256=sha(native),dynamic_p15_version=NATIVE_VERSION,upstream_commit=UPSTREAM)
 files[info_name]=(json.dumps(info,indent=2)+'\n').encode()
 files[COOP+'/source/openomsi-2.2.2-source.tar.gz']=source
 files[COOP+'/source/SOURCE-2.2.2.json']=(json.dumps(evidence,indent=2)+'\n').encode()
 files[COOP+'/CHANGELOG-2.2.2.md']=notes
 files[COOP+'/validation/version-2.2.2.txt']=version.encode()
 # Fallback content is installed beside each actual runtime. No commercial map,
 # human, bus, BCS class or user configuration is included.
 assets=a.native/'bcs-assets';assert (assets/'LICENSE-emergency-fallbacks.txt').is_file()
 for path in sorted(assets.rglob('*')):
  if not path.is_file():continue
  rel=path.relative_to(assets).as_posix();data=path.read_bytes()
  for prefix in (COOP+'/programa/bcs-assets/','app/server-dinamico-p15/bcs-assets/'):files[prefix+rel]=data
 # Existing DLLs and custom audio stay exact. Reject changed ABI payloads.
 for path in a.native.glob('*.dll'):
  rel=COOP+'/programa/'+path.name
  assert rel in files and sha(path.read_bytes())==sha(files[rel]),'Native DLL differs from retained package: '+path.name
 host_manifest_name='docs/HOST-PESSOAL-2.2.0.3.json';host_manifest=json.loads(files[host_manifest_name]);host_manifest['version']=VERSION
 for entry in host_manifest['files']:
  if entry['path']in(server,marker_name):entry['sha256']=sha(files[entry['path']])
 files[host_manifest_name]=(json.dumps(host_manifest,indent=2)+'\n').encode()
 checksum=COOP+'/SHA256.json'
 files[checksum]=(json.dumps({name[len(COOP)+1:]:sha(data)for name,data in files.items()if name.startswith(COOP+'/')and name!=checksum},indent=2)+'\n').encode()
 changed={game,server,marker_name,info_name,host_manifest_name,checksum}
 for name,digest in before.items():
  if name not in changed:assert sha(files[name])==digest,('Unrelated inherited asset changed',name)
 p14={name:digest for name,digest in before.items()if name.startswith('app/server-dinamico-p14/')};assert p14
 assert all(sha(files[name])==digest for name,digest in p14.items())
 assert files[game]==files[server]
 prefix='Bridge-2.2.2/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=6)as z:
  for name,data in sorted(files.items()):
   item=zipfile.ZipInfo(prefix+name,(2026,10,30,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;z.writestr(item,data,compresslevel=6)
 packed=bundle.getvalue();assert max(map(len,files.values()))<=128<<20 and len(packed)<=256<<20 and sum(map(len,files.values()))<=512<<20
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
 helpfile=out/'native-help.txt';helpfile.write_text(cli.stdout+cli.stderr)
 subprocess.run(['python',str(ROOT/'scripts/build_2203.py'),'--tests-only'],env=dict(os.environ,OPENOMSI_FUEL_CLI_HELP=str(helpfile)),check=True)
 previews=out/'setup-previews'
 subprocess.run([str((package/'Setup.exe').resolve()),'--gui-smoke'],cwd=package,env=dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(previews)),timeout=60,check=True)
 subprocess.run(['python',str(ROOT/'scripts/test_setup_update_service_windows.py'),str(package)],check=True)
 embedded=build.SOURCE/'bridge_update_payload.zip';assert not embedded.exists()
 try:
  with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9)as z:
   for name,data in payload.items():z.writestr(name,data)
  subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
 finally:embedded.unlink(missing_ok=True)
 final=(out/'Atualizar-Bridge.exe').read_bytes();assert embedded_payload(final)==payload and len(final)<=25<<20
 report=dict(version=VERSION,sequence=SEQUENCE,update_origin=ORIGIN,updater_sha256=sha(final),installer_size=len(final),
  coop_archive_sha256=new_spec['sha256'],components_sha256={n:sha(b)for n,b in payload.items()},retained_runtime_sha256={n:h for n,h in runtime_before.items()if n not in REBUILD},
  previous_installer_sha256=BASE_UPDATER,previous_coop_sha256=BASE_COOP,retained_host_hashes={n:sha(payload[n])for n in('HostAgent.exe','CompanyHost.exe')},
  complete_hosts_unchanged=True,complete_hosts_rebuilt=False,p14_server_unchanged=True,dynamic_p15_sha256=sha(native),dynamic_p15_version=NATIVE_VERSION,
  native_version=NATIVE_VERSION,native_candidate_run=evidence['run_id'],native_candidate_commit=evidence['commit'],native_build_commit=evidence['build_commit'],upstream_commit=UPSTREAM,
  native_client_sha256={'openomsi.exe':sha(native)},native_source_sha256=sha(source),coop_protocol=15,custom_sounds_unchanged=True,desktop_launcher_unchanged=True,
  bcs_capture_normal_launch_tested=True,bcs_single_and_company_event_fixture_tested=True,setup_changelog_languages=['pt','en','de'],setup_changelog_ui_tested=True,
  setup_live_update_ui_tested=True,bridge_regressions_tested=True,windows_gameplay_tested=False,paired_gameplay_tested=False)
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('2.2.2 cumulative update built and verified:',report['updater_sha256'])

if __name__=='__main__':main()
