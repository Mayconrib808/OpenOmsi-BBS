#!/usr/bin/env python3
"""Build the seven update binaries without recompiling any native game/server."""
import argparse, hashlib, os
from pathlib import Path
import shutil, subprocess, tempfile
import build, setup_resources, plugin_host_resources

p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--output',type=Path,required=True)
p.add_argument('--update-origin',required=True)
p.add_argument('--update-sequence',required=True)
a=p.parse_args()
if not a.update_origin.startswith('https://') or any(c in a.update_origin for c in ' \r\n\"'):p.error('HTTPS origin required')
if not a.update_sequence.isdigit():p.error('numeric sequence required')
out=a.output.resolve();out.mkdir(parents=True,exist_ok=False)
go=shutil.which('go');env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local')
def run(args,cwd):subprocess.run([go,*args],cwd=cwd,env=env,check=True)
with tempfile.TemporaryDirectory(prefix='bridge-components-') as tmp:
 work=Path(tmp)
 host_source=work/'host';host_source.mkdir()
 for name in build.HOST_SOURCES:shutil.copy2(build.SOURCE/name,host_source/Path(name).name)
 (host_source/'go.mod').write_text('module openomsi-bbs/pluginhost\n\ngo 1.23.2\n')
 resources=plugin_host_resources.prepare(host_source,build.SOURCE/'resources',build.VERSION)
 host=out/build.HOST;host.parent.mkdir(parents=True)
 run(['build','-trimpath','-buildvcs=false','-ldflags=-H=windowsgui','-o',str(host),'.'],host_source)
 setup_resources.verify_executable(host,resources)
 digest=hashlib.sha256(host.read_bytes()).hexdigest()
 for name,sources in build.PROGRAMS.items():
  target=out/name;target.parent.mkdir(parents=True,exist_ok=True)
  flags='-s -w'+(' -H=windowsgui' if name!='CompanyHost.exe' else '')
  if 'plugin_host.go' in sources:flags+=' -X=main.bundledPluginHostSHA256='+digest
  if name=='Setup.exe':
   flags+=' -X=main.bridgeUpdateOrigin='+a.update_origin+' -X=main.bridgeUpdateSequence='+a.update_sequence
   source=work/'setup';source.mkdir()
   for file in sources:shutil.copy2(build.SOURCE/file,source/file)
   (source/'go.mod').write_text('module openomsi-bbs/setup\n\ngo 1.23.2\n')
   resources=setup_resources.prepare(source,build.SOURCE/'resources')
   run(['build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(target),'.'],source)
   setup_resources.verify_executable(target,resources)
  else:run(['build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(target),*sources],build.SOURCE)
 if os.name=='nt':subprocess.run([str(out/'Setup.exe'),'--gui-smoke'],cwd=out,check=True,timeout=30)
 print('Update binaries built; original game, server and configurations are excluded.')
