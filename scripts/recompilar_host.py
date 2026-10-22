"""Rebuild the two host executables using included source and Go 1.23.2."""
from pathlib import Path
import json,os,shutil,subprocess
root=Path(__file__).resolve().parents[1]
go=shutil.which('go')
if not go or 'go1.23.2' not in subprocess.check_output([go,'version'],text=True):raise SystemExit('Use Go 1.23.2.')
env=os.environ.copy();env.update(GO111MODULE='off',GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local')
spec=json.loads((root/'docs/HOST-BUILD-FILES.json').read_text())
out=root/'recompiled';out.mkdir(exist_ok=True)
for name,files in spec.items():
 flags='-s -w'+(' -H=windowsgui' if name=='HostAgent.exe' else '')
 subprocess.run([go,'build','-trimpath','-buildvcs=false','-ldflags='+flags,'-o',str(out/name),*files],cwd=root/'source',env=env,check=True)
print('HostAgent.exe e CompanyHost.exe recompilados em recompiled/.')
