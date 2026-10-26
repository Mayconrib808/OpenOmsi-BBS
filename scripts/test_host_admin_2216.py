"""Rebuild the complete native HostAgent from the restored six-control source.

Runs independently of the 30+ minute openOMSI/Rust game compilation. A native
Win32 test confirms all six original P15 controls, map switching, values,
save and reopen, and preservation of unrelated configurations.
"""
from pathlib import Path
import os,subprocess,sys,json
import build

ROOT=Path(__file__).resolve().parents[1]
out=ROOT/"build"/"host-admin-2216"
out.mkdir(parents=True,exist_ok=True)
package=out/"package";package.mkdir(exist_ok=True)
path=package/"HostAgent.exe"
env=dict(os.environ,GOOS="windows",GOARCH="amd64",CGO_ENABLED="0",GO111MODULE="off",GOTOOLCHAIN="local")
subprocess.run(["go","build","-trimpath","-buildvcs=false","-ldflags=-s -w -H=windowsgui","-o",str(path),*build.PROGRAMS["HostAgent.exe"]],cwd=build.SOURCE,env=env,check=True)
server=package/"app"/"server-dinamico-p15"/"openomsi.exe"
server.parent.mkdir(parents=True,exist_ok=True)
server.write_bytes(b"Native UI fixture only: the full dedicated server is tested in the release builder.")
subprocess.run([sys.executable,str(ROOT/"scripts"/"test_complete_host_ui.py"),str(package)],check=True)
d=json.loads((package/"native-p15-controls.json").read_text(encoding="utf-8"))
assert d["status"]=="passed" and d["save_reopen"] and d["map_switch"] and d["unrelated_config_preserved"]
print("Native P15 HostAgent controls rebuilt and all six actual Win32 settings validated.")
