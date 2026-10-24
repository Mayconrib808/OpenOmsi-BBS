"""Cumulative 2.2.0.5 + Coop12 protocol 15, native clock readiness and active map links."""
import argparse, hashlib, io, json, os, shutil, subprocess, tempfile, zipfile
from pathlib import Path
import build, build_bridge_update, setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED
from build_2203 import tests, archive
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.6.1'
SEQUENCE=2026102407
PACKAGE='OpenOmsi_BBS_2.2.0.6.1_Setup_e_Coop12'
OUT=ROOT/'build/release-2206'
COOP='OpenOmsi Coop 0.3.40'
# Official 0.3.40 launcher binaries are NOT the Coop12 game executable.
# v0.3.38 launchers send players to a P14 lobby even alongside a P15 game.
OFFICIAL_040_WINDOWS_SHA256='698f25ec97085550a26c59f0b846f30b5ab49380e03cec8a495984e2a543b316'
LAUNCHERS=('openomsi-launcher-app.exe','openomsi-launcher.exe')
sha=lambda b:hashlib.sha256(b).hexdigest()

def replace_p14_lobby(files: dict, archive: Path) -> dict:
 data=archive.read_bytes()
 if sha(data) != OFFICIAL_040_WINDOWS_SHA256:
  raise ValueError('Official openOMSI 0.3.40 Windows x64 launcher archive SHA256 differs')
 replaced={}
 with zipfile.ZipFile(io.BytesIO(data)) as z:
  for name in LAUNCHERS:
   # Exactly one entry, independent of whether upstream adds a root folder.
   matches=[p for p in z.namelist() if not p.endswith('/') and p.replace('\\','/').split('/')[-1].lower()==name]
   if len(matches)!=1:
    raise ValueError(f'Official 0.3.40 {name}: expected one executable, found {matches}')
   binary=z.read(matches[0])
   if not binary.startswith(b'MZ') or len(binary)<65536:
    raise ValueError('Official 0.3.40 launcher is not a complete Windows PE executable: '+name)
   target=COOP+'/programa/'+name
   if target not in files or not files[target].startswith(b'MZ'):
    raise ValueError('Coop baseline missing previous P14 launcher: '+target)
   if sha(binary)==sha(files[target]):
    raise ValueError('Coop P14 launcher was accidentally reused: '+name)
   replaced[target]=dict(before=sha(files[target]),after=sha(binary),source=matches[0])
   files[target]=binary
 return replaced


def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline',type=Path,required=True);p.add_argument('--game',type=Path,required=True);p.add_argument('--old-spec',type=Path,required=True);p.add_argument('--sources',type=Path,required=True);p.add_argument('--official-launcher',type=Path,required=True);a=p.parse_args()
 assert build.VERSION==VERSION
 baseline=a.baseline.read_bytes()
 assert sha(baseline)=='14cf70b2721d6ce62d6a662f2a88e3929c30d1a795ff1d9c805b4aba50ce5e1c', '2.2.0.5 full baseline differs'
 OUT.mkdir(parents=True,exist_ok=False)
 package=OUT/PACKAGE
 with zipfile.ZipFile(io.BytesIO(baseline)) as z:z.extractall(package)
 old=json.loads(a.old_spec.read_text())
 files={name:(package/name).read_bytes() for name in old['files']}
 for n,b in files.items():assert sha(b)==old['files'][n], n
 # CI authorization, native tests and executable identity are checked by the release workflow.
 source_evidence=json.loads((a.game/'coop12-source.json').read_text())
 assert source_evidence['protocol']==15 and source_evidence['patch_sha256']==sha((ROOT/'build-support/coop12/cumulative.patch').read_bytes()), 'Native overlay differs from this package source'
 clock_evidence=json.loads((a.game/'native-clock-regression.json').read_text())
 assert clock_evidence['passed'] and clock_evidence['protocol']==15, 'Native clock roundtrip did not pass'
 files={n.replace('OpenOmsi Coop 0.3.38/',COOP+'/',1) if n.startswith('OpenOmsi Coop 0.3.38/') else n:b for n,b in files.items()}
 for folder in (a.game,):
  assert (folder/'openomsi.exe').read_bytes()[:2]==b'MZ', 'validated Windows executable missing'
  assert (folder/'self-test.log').is_file() and (folder/'version.log').is_file(), 'native Windows smoke evidence missing'
 files[COOP+'/programa/openomsi.exe']=(a.game/'openomsi.exe').read_bytes()
 files[COOP+'/programa/steam_api64.dll']=(a.game/'steam_api64.dll').read_bytes()
 # Replace BOTH stale 0.3.38/P14 lobby entrypoints with official 0.3.40/P15.
 launcher_evidence=replace_p14_lobby(files,a.official_launcher)
 # The legacy P14 executable stays available for older sessions; new Coop uses P15.
 for name,data in list(files.items()):
  if name.startswith('app/server-dinamico-p14/'):
   files[name.replace('server-dinamico-p14','server-dinamico-p15')]=data
 files['app/server-dinamico-p15/openomsi.exe']=(a.game/'openomsi.exe').read_bytes()
 files['app/server-dinamico-p15/steam_api64.dll']=(a.game/'steam_api64.dll').read_bytes()
 for old_file in ('openomsi-coop9-source.zip','openomsi-coop10-source.zip','openomsi-coop11-source.zip'):
  files.pop(COOP+'/source/'+old_file,None)
 files[COOP+'/source/openomsi-coop12-source.zip']=(a.sources/'openomsi-coop12-source.zip').read_bytes()
 files['docs/source/OpenOMSI_P14_P15_Coop12_patches.zip']=(a.sources/'openomsi-coop12-source.zip').read_bytes()
 for folder,label in ((a.game,'coop12-p15'),):
  for path in folder.iterdir():
   if path.is_file() and path.suffix in ('.log','.json'):
    files[COOP+'/validation/'+label+'-'+path.name]=path.read_bytes()
 files[COOP+'/validation/official-040-launcher.json']=(json.dumps(dict(upstream_release='v0.3.40',upstream_archive_sha256=OFFICIAL_040_WINDOWS_SHA256,binaries=launcher_evidence),indent=2)+'\n').encode()
 info=dict(version='0.3.40-coop.12',protocol=15,game_sha256=sha(files[COOP+'/programa/openomsi.exe']),dynamic_version='0.3.39-bbs-stream.4-dev.1',dynamic_sha256=sha(files['app/server-dinamico-p14/openomsi.exe']),dynamic_p15_version='0.3.40-coop.12',dynamic_p15_sha256=sha(files['app/server-dinamico-p15/openomsi.exe']),two_player_gameplay_tested=False,world_interest='bus and walker positions on both hosts')
 files[COOP+'/source/BUILD-INFO.json']=(json.dumps(info,indent=2)+'\n').encode()
 files[COOP+'/LEIA-ME.txt']=(b'OpenOMSI Coop12 0.3.40 - protocolo 15. Atualize TODOS os participantes e o servidor.\nDuas buzinadas curtas por cumprimento, prontidao nativa apos aquecimento e links por mapa no Setup.\n'+files[COOP+'/LEIA-ME.txt'])
 files[COOP+'/LEIA-ME.txt']+=b'\nCoop 12: transferencia nativa integral do estado do veiculo; BCS permanece no proprietario.\nMotorista visitante com fisica/scripts/camera/F1; atualizacao dinamica de regioes e IA ao redor de onibus e pessoas.\nServidores dinamicos P14 e P15 separados; HostAgent/CompanyHost originais mantidos.\nTestes de dois jogadores reais e viagem completa BCS ainda pendentes.\n'
 marker=json.loads(files['app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json'])
 marker.update(bridge_line=VERSION,protocol=14,native_version=info['dynamic_version'],native_sha256=info['dynamic_sha256'],source='docs/source/OpenOMSI_P14_P15_Coop12_patches.zip',source_commit='9eaf95d4caafabe041edcdfc37931b5259c30c8a',world_interest='buses_and_walkers',build_commit=os.environ.get('GITHUB_SHA'),build_run=os.environ.get('GITHUB_RUN_ID'))
 files['app/server-dinamico-p14/BBS-DYNAMIC-SERVER.json']=(json.dumps(marker,indent=2)+'\n').encode()
 marker15=dict(marker,protocol=15,native_version=info['dynamic_p15_version'],native_sha256=info['dynamic_p15_sha256'],source_commit='e874276e1bfa22896deb1e00adab0ed7d4f3ce2f')
 files['app/server-dinamico-p15/BBS-DYNAMIC-SERVER.json']=(json.dumps(marker15,indent=2)+'\n').encode()
 # Keep the personal-host activation route consistent with the new server bytes.
 host_name='docs/HOST-PESSOAL-2.2.0.3.json'
 host_manifest=json.loads(files[host_name])
 host_manifest['version']=VERSION
 host_manifest['included_dynamic_protocols']=[14,15]
 for item in host_manifest['files']:
  if item['path'] in files:item['sha256']=sha(files[item['path']])
 existing={item['path'] for item in host_manifest['files']}
 for name,data in sorted(files.items()):
  if name.startswith('app/server-dinamico-p15/') and name not in existing:
   host_manifest['files'].append({'path':name,'sha256':sha(data)})
 files[host_name]=(json.dumps(host_manifest,indent=2)+'\n').encode()
 server_old=old['files']['app/server-dinamico-p14/openomsi.exe']
 script=files['scripts/atualizar_host_pessoal.ps1'].decode('utf-8-sig')
 script=script.replace(server_old,info['dynamic_sha256'])
 ancient='8d6875bfd4b4143a8ceae149ee790291affb1f37167bb3db9250009be9217240'
 script=script.replace("@('"+ancient+"','"+info['dynamic_sha256']+"')", "@('"+ancient+"','"+server_old+"','"+info['dynamic_sha256']+"')")
 assert info['dynamic_sha256'] in script, 'Host installer did not receive new P14 hash'
 files['scripts/atualizar_host_pessoal.ps1']=b'\xef\xbb\xbf'+script.encode()
 files[COOP+'/SHA256.json']=(json.dumps({n[len(COOP)+1:]:sha(b) for n,b in files.items() if n.startswith(COOP+'/') and n!=COOP+'/SHA256.json'},indent=2)+'\n').encode()
 prefix='Bridge-2.2.0.6.1/'
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
 (ROOT/'docs/releases/coop-0.3.40-coop.12.json').write_text(json.dumps(spec,indent=2)+'\n')
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
 for name in ('source','scripts','docs','build-support','.github'):
  shutil.copytree(ROOT/name,package/name,dirs_exist_ok=True,ignore=shutil.ignore_patterns('__pycache__','*.pyc','*.exe','*.dll','*.syso','*.zip','assets','node_modules'))
 (package/'LEIA-ME.txt').write_text('OpenOmsi BBS 2.2.0.6.1 + Coop 12\n\n'+(package/'LEIA-ME.txt').read_text(encoding='utf-8')+'\n2.2.0.6.1: Coop12 0.3.40 protocolo 15, prontidao de relogio apos aquecimento nativo, duas buzinadas por cumprimento e mapas ativos com Copiar link no Setup. Mantidas transferencia real, BCS no proprietario, cobrador, personagens e interesse por jogador. Sem remover correcoes anteriores.\nTestes automaticos e nativos Windows; sessao grafica com dois jogadores e avaliacao BCS ainda nao validadas.\n',encoding='utf-8')
 report=dict(official_launcher_040=True,launcher_archive_sha256=OFFICIAL_040_WINDOWS_SHA256,launcher_sha256={name:detail['after'] for name,detail in launcher_evidence.items()},coop_protocol=15,native_clock_readiness_fix=True,double_horn=True,active_map_copy_buttons=True,version=VERSION,sequence=SEQUENCE,updater_sha256=sha((OUT/'Atualizar-Bridge.exe').read_bytes()),installer_size=(OUT/'Atualizar-Bridge.exe').stat().st_size,coop_archive_sha256=spec['sha256'],components_sha256={n:sha(b) for n,b in payload.items()},retained_components_sha256={n:before[n] for n in before if n not in ('Setup.exe','PeDePeAdapter/openomsi.exe','app/OpenOMSI_BCS_Bridge.exe')},base_package_sha256=sha(baseline),world_interest_both_hosts=True,dynamic_p15_sha256=info['dynamic_p15_sha256'],native_candidate_run=int(os.environ.get('NATIVE_CANDIDATE_RUN','0')),windows_gameplay_tested=False)
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
