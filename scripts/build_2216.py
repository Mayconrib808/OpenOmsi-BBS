"""Upgrade the authenticated 2.2.1.4 distribution to genuine 0.3.44, retaining complete hosts and P14."""
import argparse,hashlib,io,json,os,shutil,subprocess,tempfile,zipfile
from pathlib import Path
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload,EXPECTED

ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.1.6'
SEQUENCE=2026102603
COOP='OpenOmsi Coop 0.3.41'
BASE_UPDATER='d826b19cd6f099389b861acc45c1790497bb9da53be9f451cf1aa32cfa81dfc7'
BASE_COOP='8f01eaedb278549b3de33773e43acf77cf6f3d440989e4b78ea9f5eedb24895d'
sha=lambda b:hashlib.sha256(b).hexdigest()

def main():
 p=argparse.ArgumentParser();p.add_argument('--prior',type=Path,required=True);p.add_argument('--native',type=Path,required=True);p.add_argument('--launcher',type=Path,required=True);a=p.parse_args()
 assert build.VERSION==VERSION
 out=ROOT/'build/release-2216';out.mkdir(parents=True,exist_ok=False)
 spec=json.loads((a.prior/'coop-spec.json').read_text(encoding='utf-8-sig'))
 previous=json.loads((a.prior/'build-report.json').read_text(encoding='utf-8-sig'))
 updater=(a.prior/'Atualizar-Bridge.exe').read_bytes()
 assert sha(updater)==BASE_UPDATER==previous['updater_sha256']
 assert previous['version']=='2.2.1.4' and spec['sha256']==BASE_COOP
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
 inputs=json.loads((ROOT/'build-support/bridge2216/native.json').read_text())
 assert json.loads((a.native/'SOURCE.json').read_text())['commit']==inputs['commit']
 patch=ROOT/'build-support/coop15-0343/cumulative.patch'
 assert sha(patch.read_bytes())==inputs['patch_sha256']
 version=(a.native/'version.txt').read_text(encoding='utf-8')
 assert '0.3.44-coop.15-2.2.1.6' in version
 replacements={
  'openomsi.exe':a.native/'openomsi.exe',
  'openomsi-launcher.exe':a.native/'openomsi-launcher.exe',
  'openomsi-launcher-app.exe':a.launcher/'openomsi-launcher-app.exe',
 }
 for name,path in replacements.items():
  b=path.read_bytes();assert b[:2]==b'MZ' and len(b)>100000
  files[COOP+'/programa/'+name]=b
 files[COOP+'/validation/passengers-2216-version.txt']=version.encode()
 files[COOP+'/source/passengers-2.2.1.6.patch']=patch.read_bytes()
 sources=io.BytesIO()
 with zipfile.ZipFile(sources,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
  z.writestr('cumulative.patch',patch.read_bytes())
  z.writestr('launcher-Cargo.lock',(a.launcher/'launcher-Cargo.lock').read_bytes())
  z.writestr('native-input.json',json.dumps(inputs,indent=2))
  z.writestr('port-0344-prepare.py',(ROOT/'build-support/coop15-0344/prepare.py').read_bytes())
  z.writestr('port-0344-source.json',(ROOT/'build-support/coop15-0344/source.json').read_bytes())
  z.writestr('README.txt','Apply cumulative.patch to pinned official openOMSI v0.3.43, then use port-0344-prepare.py plus the pinned v0.3.44 source to merge the seven official 0.3.44 changes. All previously shipped Coop/passenger changes plus the street-facing hailing arm. Full corresponding source available from the exact upstream tag and this patch.\n')
 files[COOP+'/source/passengers-2.2.1.6-source.zip']=sources.getvalue()
 server_name='app/server-dinamico-p15/openomsi.exe'
 files[server_name]=files[COOP+'/programa/openomsi.exe']
 marker_name='app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json'
 marker=json.loads(files[marker_name]);marker.update(bridge_line=VERSION,source_commit=inputs['commit'],native_version='0.3.44-coop.15-2.2.1.6',native_sha256=sha(files[server_name]),protocol=15,version='0.3.44-coop.15-2.2.1.6',openomsi_version='0.3.44',upstream_commit='8248fe452ad4e2d420a99a482b84badb10d82ce6')
 files[marker_name]=(json.dumps(marker,indent=2)+'\n').encode()
 server_hash=sha(files[server_name])
 host_manifest_name='docs/HOST-PESSOAL-2.2.0.3.json'
 host_manifest=json.loads(files[host_manifest_name]);host_manifest['version']=VERSION
 for entry in host_manifest['files']:
  if entry['path'] in (server_name,marker_name):entry['sha256']=sha(files[entry['path']])
 files[host_manifest_name]=(json.dumps(host_manifest,indent=2)+'\n').encode()
 assert sha((a.native/'steam_api64.dll').read_bytes())==sha(files[COOP+'/programa/steam_api64.dll'])
 notes=(ROOT/'docs/releases/2.2.1.6.md').read_bytes()
 files[COOP+'/PASSAGEIROS-2.2.1.6.txt']=notes
 info_name=COOP+'/source/BUILD-INFO.json';info=json.loads(files[info_name])
 info.update(version='0.3.44-coop.15-2.2.1.6',bridge_version=VERSION,game_sha256=sha(files[COOP+'/programa/openomsi.exe']),native_candidate_run=inputs['run_id'],native_candidate_commit=inputs['commit'],passenger_effects_defaults=True,dynamic_p15_sha256=server_hash,dynamic_p15_version='0.3.44-coop.15-2.2.1.6')
 files[info_name]=(json.dumps(info,indent=2)+'\n').encode()
 checksum_name=COOP+'/SHA256.json'
 files[checksum_name]=(json.dumps({n[len(COOP)+1:]:sha(b) for n,b in files.items() if n.startswith(COOP+'/') and n!=checksum_name},indent=2)+'\n').encode()
 # Complete hosts, P14 server, custom sounds and all unrelated files stay byte-identical.
 # This generated version marker necessarily changes with the replacement native game.
 # Keep the preservation audit strict for every other inherited file.
 permitted={COOP+'/programa/'+name for name in replacements}|{info_name,checksum_name,server_name,marker_name,host_manifest_name}
 for name,h in before.items():
  if name not in permitted:assert sha(files[name])==h,('Unrelated bundled file changed',name)
 servers={n:sha(files[n]) for n in before if n.startswith(('app/server-dinamico-p14/','app/server-dinamico-p15/'))}
 p14={n:h for n,h in before.items() if n.startswith('app/server-dinamico-p14/')}
 assert p14 and all(sha(files[n])==h for n,h in p14.items())
 assert servers
 # P15 game and its capability marker must be replaced together; every other
 # existing dedicated-server asset (including all P14) is byte-identical.
 assert all(sha(files[n])==h for n,h in servers.items() if n not in (server_name,marker_name))
 assert server_hash!=previous['dynamic_p15_sha256']
 # Validate actual adapter-generated fuel arguments against the new Windows CLI.
 check=out/'native-cli';check.mkdir()
 exe=check/'openomsi.exe';exe.write_bytes(files[COOP+'/programa/openomsi.exe'])
 for n,b in files.items():
  if n.startswith(COOP+'/programa/') and n.endswith('.dll') and '/' not in n[len(COOP+'/programa/'):]: (check/Path(n).name).write_bytes(b)
 native=subprocess.run([str(exe.resolve()),'--help'],capture_output=True,text=True,timeout=60)
 assert native.returncode==0,(native.stdout,native.stderr)
 helpfile=check/'help.txt';helpfile.write_text(native.stdout+native.stderr,encoding='utf-8')
 cases=check/'cases.json'
 testenv=dict(os.environ,OPENOMSI_FUEL_CLI_HELP=str(helpfile.resolve()),OPENOMSI_FUEL_CLI_CASES=str(cases.resolve()))
 subprocess.run(['python',str(ROOT/'scripts/build_2203.py'),'--tests-only'],env=testenv,check=True)
 commands=json.loads(cases.read_text());assert len(commands)==3
 for args in commands:
  launch=subprocess.run([str(exe.resolve()),*args,'--help'],capture_output=True,text=True,timeout=60)
  assert launch.returncode==0,(args,launch.stdout,launch.stderr)
 (out/'native-cli-evidence.json').write_text(json.dumps(dict(game_sha256=sha(exe.read_bytes()),cases=commands,all_commands_accepted=True),indent=2)+'\n')
 package=out/'runtime';package.mkdir()
 for name,b in payload.items():
  target=package/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(b)
 # Save validates the selected server path. Stage the actual authenticated servers
 # alongside the retained hosts for UI testing; they remain in the Coop archive.
 for name in servers:
  target=package/name;target.parent.mkdir(parents=True,exist_ok=True);target.write_bytes(files[name])
 # Save and verify complete native P15 controls AFTER rebuilding the host programs below.
 prefix='Bridge-2.2.1.6/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
  for name,b in sorted(files.items()):
   item=zipfile.ZipInfo(prefix+name,(2026,10,26,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;z.writestr(item,b,compresslevel=9)
 assert max(map(len,files.values()))<=128<<20
 data=bundle.getvalue();assert len(data)<=256<<20 and sum(map(len,files.values()))<=512<<20
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
  for name in ('HostAgent.exe','CompanyHost.exe','Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'):
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
  subprocess.run(['python',str(ROOT/'scripts/test_complete_host_ui.py'),str(package)],check=True)
  ui=json.loads((package/'native-p15-controls.json').read_text())
  assert ui['status']=='passed' and ui['save_reopen'] and ui['map_switch'] and ui['unrelated_config_preserved']
  embedded=build.SOURCE/'bridge_update_payload.zip';assert not embedded.exists()
  try:
   with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
    for name,b in payload.items():z.writestr(name,b)
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
  finally:embedded.unlink(missing_ok=True)
 final=(out/'Atualizar-Bridge.exe').read_bytes();assert embedded_payload(final)==payload and len(final)<=25<<20
 changed_runtime={'HostAgent.exe','CompanyHost.exe','Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'}
 for name,h in runtime_before.items():
  if name not in changed_runtime:assert sha(payload[name])==h,('Runtime regression',name)
 assert all(sha(payload[n])!=runtime_before[n] for n in ('HostAgent.exe','CompanyHost.exe')), 'Admin-clock hosts were not recompiled'
 report=dict(previous)
 report.update(version=VERSION,sequence=SEQUENCE,updater_sha256=sha(final),installer_size=len(final),coop_archive_sha256=new_spec['sha256'],
  components_sha256={n:sha(b)for n,b in payload.items()},retained_runtime_sha256={n:h for n,h in runtime_before.items()if n not in changed_runtime},
  retained_servers_sha256=servers,dynamic_p15_sha256=server_hash,dynamic_p15_version='0.3.44-coop.15-2.2.1.6',retained_unrelated_files=len(before)-len(permitted),previous_coop_sha256=BASE_COOP,previous_installer_sha256=BASE_UPDATER,ticket_chance_tested=True,rigid_umbrella=True,
  native_candidate_run=inputs['run_id'],native_candidate_commit=inputs['commit'],native_version='0.3.44-coop.15-2.2.1.6',native_client_sha256={n:sha(files[COOP+'/programa/'+n])for n in replacements},
  native_rain_film=True,early_bus_hailing=True,umbrella_down_when_closed=True,default_maps=9,passenger_migration_tested=True,complete_hosts_unchanged=False,complete_hosts_rebuilt=True,admin_time_opt_in=True,delegated_passenger_role=True,servers_unchanged=False,p14_server_unchanged=True,p15_server_unchanged=False,neck_body_turn_tested=True,p15_passenger_intent=True,gaze_tested=True,full_hailing_tested=True,fuel_priority_tested=True,windows_gameplay_tested=False,official_041_p15=False,app_server_native_041=False,official_launcher_041=False,official_044_p15=True,app_server_native_044=True,official_launcher_044=True,hail_arm_direction_tested=True,all_native_regressions_tested=True,upstream_commit='8248fe452ad4e2d420a99a482b84badb10d82ce6',coop_archive_unchanged=False,gameplay_files_unchanged=False,fuel_cli_tested=True,fuel_cli_cases=3)
 report['launcher_sha256']={COOP+'/programa/'+n:sha(files[COOP+'/programa/'+n]) for n in ('openomsi-launcher.exe','openomsi-launcher-app.exe')}
 report['launcher_archive_sha256']=None
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 print('2.2.1.6 candidate verified:',report['updater_sha256'],new_spec['sha256'])

if __name__=='__main__':main()
