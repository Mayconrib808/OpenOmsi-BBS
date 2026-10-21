#!/usr/bin/env python3
"""Assemble Bridge 2.1.9.8 with a stream-capable dedicated OpenOMSI 0.3.39.

NEVER substitute older Bridge executables: only binaries from the immutable
2.1.9.8 installer are included. The native 0.3.39 runtime is a TEST candidate.
"""
import hashlib
import io
import json
from pathlib import Path
import shutil
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ORIGINAL = ROOT / "build/verified-2198"
DYNAMIC = ROOT / "build/dynamic-server"
OUTPUT = ROOT / "build/Bridge-2.1.9.8-Streaming-Protocolo14-TESTE.zip"
SERVER_URL = "https://github.com/openOMSI-org/openOMSI/releases/download/v0.3.39/openOMSI-0.3.39-server-windows-x64.zip"
SERVER_SHA = "f5a9c8db88d1fec5fa4dadd3c488e95039704e88c92c38b5b14790ac6262b2b3"
DEPENDENCIES = ("steam_api64.dll", "vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll")

def sha(b: bytes) -> str:
    return hashlib.sha256(b).hexdigest()

def main():
    components=json.loads((ROOT / "release-candidates/2.1.9.8/candidate.json").read_text())["components"]
    files={}
    for name,digest in components.items():
        b=(ORIGINAL/name).read_bytes()
        if sha(b)!=digest: raise ValueError("Modified 2.1.9.8 component: "+name)
        files[name]=b
    custom=(DYNAMIC/"openomsi.exe").read_bytes()
    if custom[:2]!=b"MZ": raise ValueError("Native Windows server is not PE")
    files["app/server/openomsi.exe"]=custom
    manifest=json.loads((DYNAMIC/"BBS-DYNAMIC-SERVER.json").read_text())
    if manifest["native_version"]!="0.3.39-bbs-stream.1" or manifest["protocol"]!=14:
        raise ValueError("Wrong native server")
    files["app/server/BBS-DYNAMIC-SERVER.json"]=(DYNAMIC/"BBS-DYNAMIC-SERVER.json").read_bytes()
    files["app/server/OpenOMSI-LICENSE.txt"]=(DYNAMIC/"OpenOMSI-LICENSE.txt").read_bytes()
    # Reuse the OFFICIAL signed-release support libraries; no Windows DLL is rebuilt.
    with urllib.request.urlopen(SERVER_URL,timeout=180) as res:
        upstream=res.read()
    if sha(upstream)!=SERVER_SHA: raise ValueError("Upstream native runtime digest mismatch")
    with zipfile.ZipFile(io.BytesIO(upstream)) as z:
        for name in DEPENDENCIES:
            matches=[x for x in z.namelist() if Path(x).name.lower()==name.lower()]
            if len(matches)!=1: raise ValueError("Official runtime missing "+name)
            files["app/server/"+name]=z.read(matches[0])
    readme = (
        "OPENOMSI BCS BRIDGE 2.1.9.8 - EXPERIMENTAL STREAMING HOST\n"
        "============================================================\n\n"
        "This is NOT the abandoned 3.0.x Bridge. All seven 2.1.9.8 components\n"
        "are byte-for-byte identical to the verified official 2.1.9.8 installer.\n"
        "The changed part is only the bundled OpenOMSI 0.3.39 protocol-14 host.\n"
        "Unpack into a NEW folder first; stop the HostAgent and servers before\n"
        "installing. Back up your existing folder and HostAgent.local.json.\n"
        "Do not overwrite a custom selected 0.3.40/protocol-15 host: this test\n"
        "server supports protocol 14, not protocol 15. Custom server selection\n"
        "remains supported and is not replaced by this package.\n"
        "The new dedicated host streams tiles around all peers. It is a test\n"
        "candidate: benchmark loading and drive with two players far apart,\n"
        "including AI, passengers, timetables and clock synchronization.\n"
        "It has NOT been validated with a full real BCS multiplayer trip.\n"
    )
    files["LEIA-PRIMEIRO-STREAMING.txt"]=readme.encode("utf-8")
    files["app/server/.openomsi-content"]=b""
    hashes="".join(f"{sha(b)}  {name}\n" for name,b in sorted(files.items()))
    files["SHA256SUMS.txt"]=hashes.encode("ascii")
    OUTPUT.parent.mkdir(parents=True,exist_ok=True)
    with zipfile.ZipFile(OUTPUT,"w",zipfile.ZIP_DEFLATED,compresslevel=9) as z:
        for path,body in sorted(files.items()):
            z.writestr(path,body)
    with zipfile.ZipFile(OUTPUT,"r") as check:
        if check.testzip() is not None: raise ValueError("ZIP integrity failure")
    (OUTPUT.with_suffix(OUTPUT.suffix+".sha256")).write_text(
        sha(OUTPUT.read_bytes())+"  "+OUTPUT.name+"\n",encoding="ascii")
    print("ZIP",OUTPUT,"bytes",OUTPUT.stat().st_size,"sha256",sha(OUTPUT.read_bytes()),flush=True)

if __name__=="__main__":
    main()
