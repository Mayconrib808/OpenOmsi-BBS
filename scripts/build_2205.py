"""Cumulative 2.2.0.4 + Coop 11 + P14/P15 dedicated streaming update."""
import argparse, hashlib, io, json, os, shutil, subprocess, tempfile, zipfile
from pathlib import Path
import build, build_bridge_update, setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED
from build_2203 import tests, archive
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.5'
SEQUENCE=2026102405
PACKAGE='OpenOmsi_BBS_2.2.0.5_Setup_e_Coop11'
OUT=ROOT/'build/release-2205'
COOP='OpenOmsi Coop 0.3.38'
sha=lambda b:hashlib.sha256(b).hexdigest()

def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--game',type=Path,required=True);p.add_argument('--server',type=Path,required=True);p.add_argument('--server15',type=Path,required=True);p.add_argument('--old-spec',type=Path,required=True);p.add_argument('--sources',type=Path,required=True);a=p.parse_args()
 assert build.VERSION==VERSION
 baseline=a.baseline.read_bytes()
 assert sha(baseline)=='6ac271d5aaa6a6998a12f51d7a95a857329f8774ede89466ffbf32d2ef798d22', '2.2.0.4 full baseline differs'
 OUT.mkdir(parents=True,exist_ok=False)
 package=OUT/PACKAGE
 with zipfile.ZipFile(io.BytesIO(baseline)) as z:z.extractall(package)
 old=json.loads(a.old_spec.read_text())
 files={name:(package/name).read_bytes() for name in old['files']}
 for n,b in files.items():assert sha(b)==old['files'][n], n
 # CI authorization, native tests and executable identity are checked by the release workflow.
 for folder in (a.game,a.server,a.server15):
  assert (folder/'openomsi.exe').read_bytes()[:2]==b'MZ', 'validated Windows executable missing'
  assert (folder/'self-test.log').is_file() and (folder/'version.log').is_file(), 'native Windows smoke evidence missing'
 files[COOP+'/programa/openomsi.exe']=(a.game/'openomsi.exe').read_bytes()
 files['app/server-dinamico-p14/openomsi.exe']=(a.server/'openomsi.exe').read_bytes()
 for name,data in list(files.items()):
  if name.startswith('app/server-dinamico-p14/'):
   files[name.replace('server-dinamico-p14','server-dinamico-p15')]=data
 files['app/server-dinamico-p15/openomsi.exe']=(a.server15/'openomsi.exe').read_bytes()
 for old_file in ('openomsi-coop9-source.zip','openomsi-coop10-source.zip'):
  files.pop(COOP+'/source/'+old_file,None)
 files[COOP+'/source/openomsi-coop11-source.zip']=(a.sources/'openomsi-coop11-source.zip').read_bytes()
 files['docs/source/OpenOMSI_P14_P15_Coop11_patches.zip']=(a.sources/'openomsi-coop11-source.zip').read_bytes()
 for folder,label in ((a.game,'coop11'),(a.server,'dynamic-stream4-p14'),(a.server15,'dynamic-stream-p15')):
  for path in folder.iterdir():
   if path.is_file() and path.suffix in ('.log','.json'):
    files[COOP+'/validation/'+label+'-'+path.name]=path.read_bytes()
 info=dict(version='0.3.38-coop.11',protocol=14,game_sha256=sha(files[COOP+'/programa/openomsi.exe']),dynamic_version='0.3.39-bbs-stream.4-dev.1',dynamic_sha256=sha(files['app/server-dinamico-p14/openomsi.exe']),dynamic_p15_version='0.3.40-bbs-stream.1-dev.1',dynamic_p15_sha256=sha(files['app/server-dinamico-p15/openomsi.exe']),two_player_gameplay_tested=False,world_interest='bus and walker positions on both hosts')
 files[COOP+'/source/BUILD-INFO.json']=(json.dumps(info,indent=2)+'\n').encode()
 files[COOP+'/LEIA-ME.txt']+=b'\nCoop 11: transferencia nativa integral do estado do veiculo; BCS permanece no proprietario.\nMotorista visitante com fisica/scripts/camera/F1; atualizacao dinamica de regioes e IA ao redor de onibus e pessoas.\nServidores dinamicos P14 e P15 separados; HostAgent/CompanyHost originais mantidos.\nTestes de dois jogadores reais e viagem completa BCS ainda pendentes.\n'
 marker=json.loads(files['app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json'])
 marker.update(bridge_line=VERSION,protocol=14,native_version=info['dynamic_version'],native_sha256=info['dynamic_sha256'],world_interest='buses_and_walkers',build_commit=os.environ.get('GITHUB_SHA'),build_run=os.environ.get('GITHUB_RUN_ID'))
 files['app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json']=(json.dumps(marker,indent=2)+'\n').encode()
 marker15=dict(marker,protocol=15,native_version=info['dynamic_p15_version'],native_sha256=info['dynamic_p15_sha256'])
 files['app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json']=(json.dumps(marker15,indent=2)+'\n').encode()
 # Keep the personal-host activation route consistent with the new server bytes.
 host_name='docs/HOST-PESSOAL-2.2.0.3.json'
 host_manifest=json.loads(files[host_name])
 for item in host_manifest['files']:
  if item['path'] in files:item['sha256']=sha(files[item['path']])
 if not any(item.get('path')=='app/server-dinamico-p15/openomsi.exe' for item in host_manifest['files']):
  host_manifest['files'].append({'path':'app/server-dinamico-p15/openomsi.exe','sha256':info['dynamic_p15_sha256']})
 files[host_name]=(json.dumps(host_manifest,indent=2)+'\n').encode()
 server_old=old['files']['app/server-dinamico-p14/openomsi.exe']
 script=files['scripts/atualizar_host_pessoal.ps1'].decode('utf-8-sig')
 script=script.replace(server_old,info['dynamic_sha256'])
 ancient='8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
 script=script.replace("@('"+ancient+"','"+info['dynamic_sha256']+"')", "@('"+ancient+"','"+server_old+"','"+info['dynamic_sha256']+"')")
 assert info['dynamic_sha256'] in script, 'Host installer did not receive new P14 hash'
 files['scripts/atualizar_host_pessoal.ps1']=b'\xef\xbb\xbf'+script.encode()
 files[COOP+'/SHA256.json']=(json.dumps({n[len(COOP)+1:]:sha(b) for n,b in files.items() if n.startswith(COOP+'/') and n!=COOP+'/SHA256.json'},indent=2)+'\n').encode()
 prefix='Bridge-2.2.0.5/'
 bundle=io.BytesIO()
 with zipfile.ZipFile(bundle,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
  for n,b in sorted(files.items()):
   item=zipfile.ZipInfo(prefix+n,(2026,10,24,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;z.writestr(item,b,compresslevel=9)
 data=bundle.getvalue();assert len(data)<=128<<20 and sum(map(len,files.values()))<=384<<20
 assets=OUT/'coop-assets';assets.mkdir();parts=[]
 for at in range(0,len(data),16<<20):
  b=data[at:at+(16<<20)];digest=sha(b);parts.append(dict(sha256=digest,size=len(b)));(assets/(digest+'.bin')).write_bytes(b)
 spec=dict(files={n:sha(b) for n,b in files.items()},parts=parts,prefix=prefix,sha256=sha(data),size=len(data))
 (OUT/'coop-spec.json').write_text(json.dumps(spec,indent=2)+'\n')
 (ROOT/'docs/releases/coop-0.3.38-coop.11.json').write_text(json.dumps(spec,indent=2)+'\n')
 text='package main\n\nconst bundledCoopOrigin = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"\nvar bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:'+json.dumps(spec['sha256'])+',Size:'+str(spec['size'])+',Prefix:'+json.dumps(prefix)+',\nParts:[]coopDistributionPart{\n'
 text+=''.join('{SHA256:'+json.dumps(p['sha256'])+',Size:'+str(p['size'])+'},\n' for p in parts)+'},\nFiles:map[string]string{\n'
 text+=''.join(json.dumps(n)+':'+json.dumps(h)+',\n' for n,h in sorted(spec['files'].items()))+'},}\n'
 (build.SOURCE/'coop_bundle_manifest.go').write_text(text)
 shutil.copy2(build.SOURCE/'coop_bundle_manifest.go',OUT/'coop_bundle_manifest.go')
 tests('go')
 payload={name:(package/name).read_bytes() for name in EXPECTED}
 before={n:sha(b) for n,b in payload.items()}
 env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 assert subprocess.check_output(['go','version'],text=True).split()[2]=='go1.23.2'
 with tempfile.TemporaryDirectory() as tmp:
  for name in ('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe'):
   flags='-s -w -H=windowsgui -X=main.bundledPluginHostSHA256='+sha(payload['app/compat/omsi-plugin-host32.exe'])
   if name=='Setup.exe':
    work=Path(tmp)/'setup';work.mkdir()
    for f in build.PROGRAMS[name]:shutil.copy2(build.SOURCE/f,work/f)
    (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
    resource=setup_resources.prepare(work,build.SOURCE/'resources');args=['.'];build_env=dict(env,GO111MODULE='on')
    flags+=' -X=main.bridgeUpdateOrigin=https://openomsi-bbs-updates.maycongamesmaximo.workers.dev -X=main.bridgeUpdateSequence='+str(SEQUENCE)
   else:work=build.SOURCE;args=build.PROGRAMS[name];build_env=env
   target=package/name
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(target),*args],cwd=work,env=build_env,check=True)
   if name=='Setup.exe':setup_resources.verify_executable(target,resource)
   payload[name]=target.read_bytes()
  embedded=build.SOURCE/'bridge_update_payload.zip'
  assert not embedded.exists()
  try:
   with zipfile.ZipFile(embedded,'w',zipfile.ZIP_DEFLATED,compresslevel=9) as z:
    for name,b in payload.items():z.writestr(name,b)
   subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(OUT/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
  finally:embedded.unlink(missing_ok=True)
 assert embedded_payload((OUT/'Atualizar-Bridge.exe').read_bytes())==payload
 assert (OUT/'Atualizar-Bridge.exe').stat().st_size<=25<<20
 for n in ('HostAgent.exe','CompanyHost.exe','app/compat/Omsi.exe','app/compat/omsi-plugin-host32.exe'):assert sha(payload[n])==before[n]
 for name,b in files.items():
  path=package/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(b)
 for name in ('source','scripts','docs','build-support'):
  shutil.copytree(ROOT/name,package/name,dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.pyc','*.exe','*.dll','*.syso','*.zip','assets','node_modules'))
 (package/'LEIA-ME.txt').write_text('OpenOmsi BBS 2.2.0.5 + Coop 11\n\n'+(package/'LEIA-ME.txt').read_text(encoding='utf-8')+'\n2.2.0.5: transferencia real de motorista com fisica/scripts, camera F1, IA e passageiros ao redor de todos os jogadores, servidor dinamico P14/P15. Sem remover correcoes anteriores.\nTestes automaticos e nativos Windows; sessao grafica com dois jogadores e avaliacao BCS ainda nao validadas.\n',encoding='utf-8')
 report=dict(version=VERSION,sequence=SEQUENCE,updater_sha256=sha((OUT/'Atualizar-Bridge.exe').read_bytes()),installer_size=(OUT/'Atualizar-Bridge.exe').stat().st_size,coop_archive_sha256=spec['sha256'],components_sha256={n:sha(b) for n,b in payload.items()},retained_components_sha256={n:before[n] for n in before if n not in ('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe')},base_package_sha256=sha(baseline),world_interest_both_hosts=True,dynamic_p15_sha256=info['dynamic_p15_sha256'],native_candidate_run=int(os.environ.get('NATIVE_CANDIDATE_RUN','0')),windows_gameplay_tested=False)
 (OUT/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')

def finalize():
 package=OUT/PACKAGE
 shutil.copytree(OUT/'evidence',package/'validation',dirs_exist_ok=True)
 names=sorted(p.relative_to(package).as_posix() for p in package.rglob('*') if p.is_file() and p.relative_to(package).as_posix() not in ('docs/SHA256.txt','docs/PACKAGE_FILES.txt'))
 names.append('docs/PACKAGE_FILES.txt');(package/'docs/PACKAGE_FILES.txt').write_text('\n'.join(sorted(names))+'\n')
 (package/'docs/SHA256.txt').write_text(''.join(sha((package/n).read_bytes())+'  '+n+'\n' for n in sorted(names)))
 target=OUT/(PACKAGE+'.zip');archive(package,target)
 report=json.loads((OUT/'build-report.json').read_text());report.update(full_package_sha256=sha(target.read_bytes()),native_setup_smoke=True,native_server_lamp=True,native_manual_and_timer=True)
 (OUT/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')

if __name__=='__main__':
 import sys
 if '--finalize' in sys.argv:finalize()
 else:main()
