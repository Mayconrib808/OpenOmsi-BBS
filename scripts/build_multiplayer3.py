#!/usr/bin/env python3
"""Build the independent native-BCS 3.0 preview; never bundle the old facade."""
import argparse, hashlib, json, os
from pathlib import Path
import shutil, subprocess, zipfile
import build_native_server
ROOT=Path(__file__).resolve().parents[1]
VERSION='3.0.0-dev.1'
def main():
 p=argparse.ArgumentParser(description=__doc__)
 p.add_argument('--go',default='go');p.add_argument('--native-server',type=Path,required=True)
 p.add_argument('--output',type=Path,default=ROOT/'dist'/('OpenOmsi-BBS-'+VERSION))
 a=p.parse_args();build_native_server.verify_package(a.native_server)
 if a.output.exists() or a.output.with_suffix('.zip').exists():p.error('Choose a new output directory')
 env=os.environ.copy();env.update(CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='on');env.pop('GOOS',None);env.pop('GOARCH',None)
 subprocess.run([a.go,'test','./...'],cwd=ROOT/'multiplayer3',env=env,check=True)
 subprocess.run([a.go,'vet','./...'],cwd=ROOT/'multiplayer3',env=env,check=True)
 subprocess.run(['node','--test','test/multiplayer3.test.js','test/room.test.js'],cwd=ROOT/'relay',check=True)
 a.output.mkdir(parents=True);env.update(GOOS='windows',GOARCH='amd64')
 subprocess.run([a.go,'build','-trimpath','-ldflags=-H=windowsgui','-o',str(a.output/'openomsi.exe'),'.'],cwd=ROOT/'multiplayer3',env=env,check=True)
 shutil.copy2(a.output/'openomsi.exe',a.output/'Configurar.exe')
 shutil.copytree(a.native_server,a.output/'server')
 shutil.copy2(ROOT/'docs'/'MULTIPLAYER3-LEIA-ME.txt',a.output/'LEIA-ME.txt')
 shutil.copy2(ROOT/'docs'/'MULTIPLAYER3.md',a.output/'DESENVOLVIMENTO.md')
 shutil.copy2(ROOT/'LICENSE',a.output/'LICENSE')
 shutil.copytree(ROOT/'relay',a.output/'relay',ignore=shutil.ignore_patterns('node_modules','.wrangler'))
 shutil.copytree(ROOT/'multiplayer3',a.output/'source',ignore=shutil.ignore_patterns('*.exe'))
 info={'version':VERSION,'native_bcs':True,'game':'Use installed official openOMSI; preserve PeDePe Lua and original launch arguments','validation':'Go tests/vet, Worker tests and Windows x64 compilation; two real PCs + native BCS not validated','directory_deployment':'Requires deploying the included Worker update before creating/joining 3.0 companies'}
 (a.output/'BUILD-INFO.json').write_text(json.dumps(info,indent=2),encoding='utf-8')
 entries=sorted(x for x in a.output.rglob('*') if x.is_file())
 (a.output/'SHA256.txt').write_text(''.join(hashlib.sha256(x.read_bytes()).hexdigest()+'  '+x.relative_to(a.output).as_posix()+'\n' for x in entries),encoding='utf-8')
 target=Path(str(a.output)+'.zip')
 with zipfile.ZipFile(target,'x',compression=zipfile.ZIP_DEFLATED,compresslevel=6) as z:
  for x in sorted(a.output.rglob('*')):
   if x.is_file():z.write(x,x.relative_to(a.output).as_posix())
 print(str(target));print('SHA256='+hashlib.sha256(target.read_bytes()).hexdigest())
if __name__=='__main__':main()
