#!/usr/bin/env python3
"""Build only Setup and its cumulative updater; retain six exact released binaries."""
import argparse, hashlib, json, os, shutil, subprocess, sys, tempfile, zipfile
from pathlib import Path
import build, build_bridge_update, setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED
from build_2203 import tests, archive
ROOT=Path(__file__).resolve().parents[1]
VERSION='2.2.0.3-hotfix.2'
SEQUENCE=2026102303
PACKAGE='OpenOmsi_BBS_2.2.0.3_hotfix2_Setup_e_Coop9'
BASE_UPDATER='53a6fe30e6f074cbfc6d82a41f7af65337022b1691f06270e59ce03bb964f9d1'
BASE_PACKAGE='f5b921e6b15ee38329d3db405cc1eabeecd11d211624d95b2cbb86ef1a39af07'
def sha(b):return hashlib.sha256(b).hexdigest()
def main():
 p=argparse.ArgumentParser();p.add_argument('--baseline-update',type=Path,required=True);p.add_argument('--baseline-package',type=Path,required=True);a=p.parse_args()
 if build.VERSION!=VERSION:raise ValueError('Hotfix version differs')
 if subprocess.check_output(['go','version'],text=True).split()[2]!='go1.23.2':raise ValueError('Use Go 1.23.2')
 tests('go')
 original=a.baseline_update.read_bytes();full=a.baseline_package.read_bytes()
 if sha(original)!=BASE_UPDATER or sha(full)!=BASE_PACKAGE:raise ValueError('Approved 2.2.0.3 bytes differ')
 payload=embedded_payload(original)
 out=ROOT/'build/release-2203-hotfix2';out.mkdir(parents=True,exist_ok=False)
 runtime=out/'runtime'
 for name,data in payload.items():
  path=runtime/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(data)
 env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
 with tempfile.TemporaryDirectory(prefix='bridge-status-build-') as temporary:
  work=Path(temporary)
  for name in build.PROGRAMS['Setup.exe']:shutil.copy2(build.SOURCE/name,work/name)
  (work/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
  resource=setup_resources.prepare(work,build.SOURCE/'resources')
  flags=f'-s -w -H=windowsgui -X=main.bundledPluginHostSHA256={sha(payload["app/compat/omsi-plugin-host32.exe"])} -X=main.bridgeUpdateOrigin=https://openomsi-bbs-updates.maycongamesmaximo.workers.dev -X=main.bridgeUpdateSequence={SEQUENCE}'
  for mode in ['vet','build']:
   cmd=['go',mode]
   cmd+=['-unsafeptr=false','.'] if mode=='vet' else ['-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(runtime/'Setup.exe'),'.']
   subprocess.run(cmd,cwd=work,env=dict(env,GO111MODULE='on'),check=True)
  setup_resources.verify_executable(runtime/'Setup.exe',resource)
 payload['Setup.exe']=(runtime/'Setup.exe').read_bytes()
 embedded=build.SOURCE/'bridge_update_payload.zip'
 if embedded.exists():raise ValueError('Do not overwrite an existing payload')
 try:
  with zipfile.ZipFile(embedded,'x',compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
   for name,data in payload.items():
    item=zipfile.ZipInfo(name,(1980,1,1,0,0,0));item.compress_type=zipfile.ZIP_DEFLATED;item.create_system=3;item.external_attr=0o100644<<16;z.writestr(item,data,compresslevel=9)
  subprocess.run(['go','vet','-unsafeptr=false',*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
  subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w -H=windowsgui','-o',str(out/'Atualizar-Bridge.exe'),*build_bridge_update.UPDATER_SOURCES],cwd=build.SOURCE,env=env,check=True)
 finally:embedded.unlink(missing_ok=True)
 if embedded_payload((out/'Atualizar-Bridge.exe').read_bytes())!=payload:raise ValueError('Embedded runtime differs')
 if (out/'Atualizar-Bridge.exe').stat().st_size>25<<20:raise ValueError('Updater exceeds old Setup limit')
 package=out/PACKAGE
 with zipfile.ZipFile(a.baseline_package) as z:z.extractall(package)
 for name,data in payload.items():
  path=package/name
  if name!='Setup.exe' and path.read_bytes()!=data:raise ValueError('Retained release binary differs: '+name)
  path.write_bytes(data)
 ignore=shutil.ignore_patterns('__pycache__','*.pyc','*.exe','*.dll','*.syso','*.zip','assets','node_modules')
 for name in ('source','scripts','docs','build-support'):
  shutil.copytree(ROOT/name,package/name,dirs_exist_ok=True,ignore=ignore)
 readme=package/'LEIA-ME.txt';text=readme.read_text(encoding='utf-8');text=text.replace('OpenOmsi BBS 2.2.0.3 +','OpenOmsi BBS 2.2.0.3-hotfix.2 +',1)
 text+='''\nHOTFIX 2.2.0.3-hotfix.2 — Status do Servidor\nO Setup consulta o HostAgent local e confirma a presença no diretório a cada 5 segundos.\nVerde: agente pronto para abrir um mapa sob demanda, iniciando um mapa ou servidor online.\nAmarelo: offline, sem mapas disponíveis, sem configuração ou consulta indisponível.\nO botão Verificar se o servidor está online consulta novamente sem iniciar um mapa.\nO HostAgent e suas seis opções P15, a Coop 9 e o servidor dinâmico são os mesmos\nda 2.2.0.3. Nenhuma configuração do host é modificada.\n'''
 readme.write_text(text,encoding='utf-8')
 report=dict(version=VERSION,sequence=SEQUENCE,rebuilt=['Setup.exe'],retained_components_sha256={n:sha(b) for n,b in payload.items() if n!='Setup.exe'},components_sha256={n:sha(b) for n,b in payload.items()},updater_sha256=sha((out/'Atualizar-Bridge.exe').read_bytes()),installer_size=(out/'Atualizar-Bridge.exe').stat().st_size,base_updater_sha256=BASE_UPDATER,base_package_sha256=BASE_PACKAGE,coop_archive_sha256=json.loads((ROOT/'docs/releases/coop-0.3.38-coop.9.json').read_text())['sha256'],poll_interval_seconds=5,read_only_status=True)
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n')
 (package/'docs/BUILD-2.2.0.3-hotfix.2.json').write_text(json.dumps(report,indent=2)+'\n')
 print('Hotfix built with all six previous release binaries preserved.')
def finalize():
 out=ROOT/'build/release-2203-hotfix2';package=out/PACKAGE
 shutil.copytree(out/'evidence',package/'validation',dirs_exist_ok=True)
 names=sorted(p.relative_to(package).as_posix() for p in package.rglob('*') if p.is_file() and p.relative_to(package).as_posix() not in {'docs/PACKAGE_FILES.txt','docs/SHA256.txt'})
 names.append('docs/PACKAGE_FILES.txt');(package/'docs/PACKAGE_FILES.txt').write_text('\n'.join(sorted(names))+'\n',encoding='utf-8')
 (package/'docs/SHA256.txt').write_text(''.join(sha((package/n).read_bytes())+'  '+n+'\n' for n in sorted(names)),encoding='utf-8')
 target=out/(PACKAGE+'.zip');archive(package,target)
 report=json.loads((out/'build-report.json').read_text());report['full_package_sha256']=sha(target.read_bytes())
 report.update(native_setup_smoke=True,native_server_lamp=True,native_manual_and_timer=True)
 (out/'build-report.json').write_text(json.dumps(report,indent=2)+'\n');print('HOTFIX_REPORT='+json.dumps(report))
if __name__=='__main__':
 if '--finalize' in sys.argv:finalize()
 else:main()
