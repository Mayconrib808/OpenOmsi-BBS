"""Build only the isolated P16 companions; complete legacy hosts and game stay exact."""
import argparse, hashlib, json, os, subprocess
from pathlib import Path
import build, package_p16_host_repair

p=argparse.ArgumentParser();p.add_argument('candidate',type=Path);p.add_argument('output',type=Path);p.add_argument('--server-output',type=Path,required=True)
a=p.parse_args();out=a.output.resolve()
package_p16_host_repair.stage(a.candidate,out,a.server_output)
env=dict(os.environ,GOOS='windows',GOARCH='386',GO386='sse2',CGO_ENABLED='0',GOTOOLCHAIN='local',GO111MODULE='off',GOEXPERIMENT='')
manifest=json.loads((out/'P16-HOST-REPAIR.json').read_text())
for name in ('HostAgent.exe','CompanyHost.exe'):
    rel='app/hosts-p16/'+name
    subprocess.run(['go','build','-trimpath','-buildvcs=false','-ldflags=-s -w','-o',str(out/rel),*build.PROGRAMS[name]],cwd=build.SOURCE,env=env,check=True)
    manifest['files'][rel]=hashlib.sha256((out/rel).read_bytes()).hexdigest()
manifest.update(repair='2.2.2.4-hotfix.3-private-preferences',source_commit=os.environ['GITHUB_SHA'],
                source_url='https://github.com/Mayconrib808/OpenOmsi-BBS/tree/'+os.environ['GITHUB_SHA'],host_preferences_limit=32<<20)
(out/'P16-HOST-REPAIR.json').write_text(json.dumps(manifest,indent=2)+'\n')
(out/'CHANGELOG-HOTFIX-3.md').write_bytes((build.ROOT/'docs/releases/2.2.2.4-hotfix.3.md').read_bytes())
