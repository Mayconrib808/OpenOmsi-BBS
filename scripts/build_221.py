"""Assemble 2.2.1 from the authenticated Hotfix3 payload, preserving complete hosts/servers."""
import argparse,hashlib,io,json,os,shutil,subprocess,tempfile,zipfile
from pathlib import Path
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload,EXPECTED

ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1'
SEQUENCE=2026102515
COOP='OpenOmsi Coop 0.3.41'
BASE_UPDATER='7c67bc9acdaab5089afe657b72f6b7a96e5408b96131478e3537814e1baae9ae'
BASE_COOP='d9456597517c4785d664e045310bd3cbc17b1278b945cbffa43f4d49d3045ab9'
sha=lambda b:hashlib.sha256(b).hexdigest()

def main():
 p=argparse.ArgumentParser();p.add_argument('--prior',type=Path,required=True);p.add_argument('--native',type=Path,required=True);p.add_argument('--launcher',type=Path,required=True);a=p.parse_args()
 assert build.VERSION==VERSION
 out=ROOT/'build/release-221';out.mkdir(parents=True,exist_ok=False)
 spec=json.loads((a.prior/'coop-spec.json').read_text(encoding='utf-8-sig'))
 previous=json.loads((a.prior/'build-report.json').read_text(encoding='utf-8-sig'))
 updater=(a.prior/'Atualizar-Bridge.exe').read_bytes()
 assert sha(updater)==BASE_UPDATER==previous['updater_sha256']
 assert previous['version']=='2.2.0.8.3' and spec['sha256']==BASE_COOP
 assert previous['complete_p15_ui_tested']
 payload=embedded_payload(updater)
 assert set(payload)==set(EXPECTED)
 for name,b in payload.items():assert sha(b)==previous['components_sha256'][name],name
 runtime_before={n:sha(b) for n,b in payload.items()}
 parts=[]
 for part in spec['parts']:
  b=(a.prior/'coop-assets'/(part['sha256']+'.bin')).read_bytes()
  assert len(b)==part['size'] and sha(b)==part['sha256'];parts.append(b)
 packed=b''.join(parts);assert sha(packed)==BASE_COOP
 with zipfile.ZipFile(io.BytesIO(packed)) as z:
  files={n:z.read(spec['prefix']+n) for n in spec['files']}
 for name,b in files.items():assert sha(b)==spec['files'][name],name
 before={n:sha(b) for n,b in files.items()}
 inputs=json.loads((ROOT/'build-support/bridge221/native.json').read_text())
 assert json.loads((a.native/'SOURCE.json').read_text())['commit']==inputs['commit']
 patch=ROOT/'build-support/passengers221/fixes.patch'
 assert sha(patch.read_bytes())==inputs['patch_sha256']
 version=(a.native/'VERSAO.txt').read_text(encoding='utf-8')
 assert '0.3.41-coop.14-2.2.1' in version
 replacements={
  'openomsi.exe':a.native/'openomsi.exe',
  'openomsi-launcher.exe':a.native/'openomsi-launcher.exe',
  'openomsi-launcher-app.exe':a.launcher/'openomsi-launcher-app.exe',
 }
 for name,path in replacements.items():
  b=path.read_bytes();assert b[:2]==b'MZ' and len(b)>100000
  files[COOP+'/programa/'+name]=b
 files[COOP+'/validation/passengers-221-version.txt']=version.encode()
 files[COOP+'/source/passengers-2.2.1.patch']=patch.read_bytes()
 sources=io.BytesIO()
 with zipfile.ZipFile(sources,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
  for folder,label in [('passenger-weather-beta1','visuals-beta1.patch'),('passenger-weather-beta2','visuals-beta2.patch'),('passengers221','passengers-221.patch')]:
   file=ROOT/'build-support'/folder/('fixes.patch' if folder=='passengers221' else 'visuals.patch');z.writestr(label,file.read_bytes())
  z.writestr('launcher-Cargo.lock',(a.launcher/'launcher-Cargo.lock').read_bytes())
  z.writestr('native-input.json',json.dumps(inputs,indent=2))
  z.writestr('README.txt','OpenOMSI v0.3.41 + cumulative Coop14 Hotfix1 + the three patches in order. Existing Coop14 source archive is retained alongside this archive.\n')
 files[COOP+'/source/passengers-2.2.1-source.zip']=sources.getvalue()
 notes=(ROOT/'docs/releases/2.2.1.md').read_bytes()
 files[COOP+'/PASSAGEIROS-2.2.1.txt']=notes
 info_name=COOP+'/source/BUILD-INFO.json';info=json.loads(files[info_name])
 info.update(version='0.3.41-coop.14-2.2.1',bridge_version=VERSION,game_sha256=sha(files[COOP+'/programa/openomsi.exe']),native_candidate_run=inputs['run_id'],native_candidate_commit=inputs['commit'],passenger_effects_defaults=True)
 files[info_name]=(json.dumps(info,indent=2)+'\n').encode()
 # Complete P14/P15 servers, custom sounds and all unrelated bundled files must be byte-identical.
 permitted={COOP+'/programa/'+name for name in replacements}|{info_name}
 for name,h in before.items():
  if name not in permitted:assert sha(files[name])==h,('Unrelated bundled file changed',name)
 servers={n:before[n] for n in before if n.startswith(('app/server-dinamico-p14/','app/server-dinamico-p15/'))}
 assert servers and all(sha(files[n])==h for n,h in servers.items())
 package=out/'runtime';package.mkdir()
 for name,b in payload.items():
  target=package/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
 # Real windows test against the actual complete host executables, not rebuilt old Go hosts.
 subprocess.run(['python',str(ROOT/'scripts/test_complete_host_ui.py'),str(package)],check=True)
 prefix='Bridge-2.2.1/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
  for name,b in sorted(files.items()):
   item=zipfile.ZipInfo(prefix+name,(2026,10,25,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;z.writestr(item,b,compresslevel=9)
 data=bundle.getvalue();assert len(data)<=128<<20 and sum(map(len,files.values()))<=384<<20
 assets=out/'coop-assets';assets.mkdir();chunks=[]
 for pos in range(0,len(data),16<<20):
  b=data[pos:pos+(16<<20)];h=sha(b);(assets/(h+'.bin')).write_bytes(b);chunks.append(dict(sha256=h,size=len(b)))
 new_spec=dict(files={n:sha(b)for n,b in files.items()},parts=chunks,prefix=prefix,sha256=sha(data),size=len(data))
 (out/'coop-spec.json').write_text(json.dumps(new_spec,indent=2)+'\n')
 go='package main\n\nconst bundledCoopOrigin = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"\nvar bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:'+json.dumps(new_spec['sha256'])+',Size:'+str(new_spec['size'])+',Prefix:'+json.dumps(prefix)+',\nParts:[]coopDistributionPart{\n'
 go+=''.join('{SHA256:'+json.dumps(x['sha256'])+',Size:'+str(x['size'])+'},\n'for x in chunks)+'},\nFiles:map[string]string{\n'
 go+=''.join(json.dumps(n)+':'+json.dumps(h)+',\n'for n,h in sorted(new_spec['files'].items()))+'},}\n'
 (build.SOURCE/'coop_bundle_manifest.go').write_text(go)
 (out/'coop_bundle_manifest.go').write_text(go)
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
 report.update(version=VERSION,sequence=SEQUENCE,updater_sha256=sha(final),installer_size=len(final),coop_archive_sha256=new_spec['sha256'],
  components_sha256={n:sha(b)for n,b in payload.items()},retained_runtime_sha256={n:h for n,h in runtime_before.items()if n not in changed_runtime},
  retained_servers_sha256=servers,retained_unrelated_files=len(before)-len(permitted),previous_coop_sha256=BASE_COOP,previous_installer_sha256=BASE_UPDATER,
  native_candidate_run=inputs['run_id'],native_candidate_commit=inputs['commit'],native_version='0.3.41-coop.14-2.2.1',native_client_sha256={n:sha(files[COOP+'/programa/'+n])for n in replacements},
  native_rain_film=True,early_bus_hailing=True,umbrella_down_when_closed=True,default_maps=9,passenger_migration_tested=True,complete_hosts_unchanged=True,servers_unchanged=True,windows_gameplay_tested=False)
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('2.2.1 candidate verified:',report['updater_sha256'],new_spec['sha256'])

if __name__=='__main__':main()
