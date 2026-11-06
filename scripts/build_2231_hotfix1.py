#!/usr/bin/env python3
"""Build the 2.2.3.1 Hotfix 1 signed-feed candidate from the exact approved P17 bytes.

Only Setup.exe is recompiled. The seven-component update installer is rebuilt
with this Setup, preserving the other six exact P17-era binaries, all native
Coop chunks, servers, HostAgents and player preferences.
"""
from pathlib import Path
from hashlib import sha256
import argparse,io,os,json,zipfile,shutil,subprocess,tempfile
import build,build_bridge_update,setup_resources
from build_runtime_2203 import embedded_payload, EXPECTED

ROOT=Path(__file__).resolve().parents[1]
SRC=build.SOURCE
VERSION="2.2.3.1-hotfix.1"
SEQUENCE=2026110502
ORIGIN="https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"
BASE_SHA="c0cf59361d96dbfe97e414d134b339db62b429ea805cd34387505e53eb664275"
COOP_SHA="34c80650a32b26348b9da3e64f127429b20d184fad673ed9e47d38545f4fccea"
UPSTREAM="0.3.49-coop.17-2.2.3.1"
sha=lambda b:sha256(b).hexdigest()
def readjson(p): return json.loads(p.read_text(encoding="utf-8-sig"))
def check(value,msg): 
    if not value: raise ValueError(msg)
def run(args,**kwargs):
    print("RUN",args[0],args[1:3],flush=True)
    subprocess.run(args,check=True,**kwargs)
def main():
    p=argparse.ArgumentParser()
    p.add_argument("--approved",type=Path,required=True)
    p.add_argument("--output",type=Path,required=True)
    args=p.parse_args()
    approved=args.approved.resolve()
    out=args.output.resolve()
    out.mkdir(parents=True,exist_ok=False)
    check(build.VERSION==VERSION,"Source version is not the Hotfix 1 release")
    check(subprocess.check_output(["go","version"],text=True).split()[2]=="go1.23.2","Go 1.23.2 required")
    report=readjson(approved/"build-report.json")
    spec=readjson(approved/"coop-spec.json")
    base=(approved/"Atualizar-Bridge.exe").read_bytes()
    check(report["version"]=="2.2.3.1" and report["sequence"]==2026110501,"Unexpected baseline version/sequence")
    check(sha(base)==BASE_SHA==report["updater_sha256"],"Baseline updater changed")
    check(spec["sha256"]==COOP_SHA==report["coop_archive_sha256"],"Approved Coop SHA changed")
    check(report["native_version"]==UPSTREAM and report["coop_protocol"]==17,"Approved native 0.3.49/P17 not found")
    payload=embedded_payload(base)
    check(set(payload)==set(EXPECTED),"Not seven exact original binaries")
    for name,contents in payload.items():
        check(sha(contents)==report["components_sha256"][name],"Changed previous component "+name)

    # Reconstruct the EXACT signed cumulative Coop manifest, without changes.
    manifest="package main\n\nconst bundledCoopOrigin = "+json.dumps(ORIGIN)+"\n"
    manifest+="var bundledCoopDistribution = coopDistribution{FullPaths:true,SHA256:"+json.dumps(spec["sha256"])+",Size:"+str(spec["size"])+",Prefix:"+json.dumps(spec["prefix"])+",\nParts:[]coopDistributionPart{\n"
    manifest+="".join("{SHA256:"+json.dumps(part["sha256"])+",Size:"+str(part["size"])+"},\n" for part in spec["parts"])+"},\nFiles:map[string]string{\n"
    manifest+="".join(json.dumps(name)+":"+json.dumps(digest)+",\n" for name,digest in sorted(spec["files"].items()))+"},}\n"
    check(spec["prefix"]=="Bridge-2.2.3.1/","Unexpected Coop prefix")
    (SRC/"coop_bundle_manifest.go").write_text(manifest,encoding="utf-8")
    
    portable=approved/"OpenOMSI-BCS-Bridge-2.2.3.1-FULL.zip"
    check(portable.is_file() and sha(portable.read_bytes())==report["portable_sha256"],"Wrong published portable ZIP")
    runtime=out/"runtime"
    with zipfile.ZipFile(portable) as z:
        for entry in z.infolist():
            if entry.is_dir():continue
            path=(runtime/entry.filename).resolve()
            check(path.is_relative_to(runtime.resolve()),"Unsafe release archive entry")
            path.parent.mkdir(parents=True,exist_ok=True)
            with z.open(entry) as input, path.open("wb") as output:
                shutil.copyfileobj(input,output,1024*1024)
    for name,digest in spec["files"].items():
        path=runtime/name
        check(path.is_file() and sha(path.read_bytes())==digest,"Approved Coop file changed: "+name)
    for name,expected in report["components_sha256"].items():
        check(sha((runtime/name).read_bytes())==expected,"Portable component mismatch: "+name)
    check(not (runtime/"docs/SHA256.txt").is_file(),"Original SHA manifest unexpectedly present")

    env=dict(os.environ,GOOS="windows",GOARCH="386",GO386="sse2",CGO_ENABLED="0",GO111MODULE="on",GOTOOLCHAIN="local",GOEXPERIMENT="")
    pinned=["PeDePeAdapter/openomsi.exe","app/OpenOMSI_BCS_Bridge.exe","app/compat/Omsi.exe","app/compat/omsi-plugin-host32.exe"]
    with tempfile.TemporaryDirectory(prefix="bbs-2231-hf1-") as t:
        work=Path(t)
        for name in build.PROGRAMS["Setup.exe"]:
            shutil.copy2(SRC/name,work/name)
        (work/"go.mod").write_text("module openomsi-bbs/setup\n\ngo 1.23.2\n",encoding="utf-8")
        resources=setup_resources.prepare(work,SRC/"resources")
        flags="-s -w -H=windowsgui"
        flags+=" -X=main.bundledPluginHostSHA256="+sha(payload["app/compat/omsi-plugin-host32.exe"])
        flags+=" -X=main.activationPinnedRuntimeSHA256="+":".join(sha(payload[x]) for x in pinned)
        flags+=" -X=main.bridgeUpdateOrigin="+ORIGIN
        flags+=" -X=main.bridgeUpdateSequence="+str(SEQUENCE)
        run(["go","build","-trimpath","-buildvcs=false","-ldflags="+flags,"-o",str(runtime/"Setup.exe"),"."],cwd=work,env=env)
        setup_resources.verify_executable(runtime/"Setup.exe",resources)
        run(["go","test","-count=1","."],cwd=work,env=env)

    new_setup=(runtime/"Setup.exe").read_bytes()
    new_setup_sha=sha(new_setup)
    check(new_setup_sha!=report["components_sha256"]["Setup.exe"],"Hotfix Setup did not change")
    check(len(new_setup)>1<<20 and len(new_setup)<16<<20,"Unexpected Setup size")
    run([str((runtime/"Setup.exe").resolve()),"--verify-activation"],cwd=runtime)
    run([str((runtime/"Setup.exe").resolve()),"--gui-smoke"],cwd=runtime,env=dict(os.environ,BRIDGE_SETUP_PREVIEWS=str(out/"setup-previews")),timeout=75)
    # Tampered native P17 gameplay binary MUST be rejected by corrected Setup.
    game=runtime/"OpenOmsi Coop 0.3.41/programa/openomsi.exe"
    with game.open("r+b") as data:
        data.seek(64); old=data.read(1)
        data.seek(64); data.write(bytes([old[0]^1]))
    test=subprocess.run([str((runtime/"Setup.exe").resolve()),"--verify-activation"],cwd=runtime,timeout=75)
    check(test.returncode!=0,"Tampered native game was incorrectly accepted")
    with game.open("r+b") as data:
        data.seek(64);data.write(old)
    check(sha(game.read_bytes())==spec["files"]["OpenOmsi Coop 0.3.41/programa/openomsi.exe"],"Could not restore test-tampered game")
    payload["Setup.exe"]=new_setup

    # Build a new update installer containing the same seven executables with
    # only Setup updated. Existing P17 Coop ZIP/parts remain immutable.
    embedded=SRC/"bridge_update_payload.zip"
    check(not embedded.exists(),"Unexpected stale embedded update payload")
    try:
        with zipfile.ZipFile(embedded,"w",compression=zipfile.ZIP_DEFLATED,compresslevel=9) as z:
            for name,data in sorted(payload.items()): z.writestr(name,data)
        updater_env=dict(env,GOARCH="amd64",GO111MODULE="off")
        build_flags="-s -w -H=windowsgui -X=main.bridgeUpdateOrigin="+ORIGIN+" -X=main.bridgeUpdateSequence="+str(SEQUENCE)
        run(["go","build","-trimpath","-buildvcs=false","-ldflags="+build_flags,"-o",str(out/"Atualizar-Bridge.exe"),*build_bridge_update.UPDATER_SOURCES],cwd=SRC,env=updater_env)
    finally: embedded.unlink(missing_ok=True)
    installer=(out/"Atualizar-Bridge.exe").read_bytes()
    check(set(embedded_payload(installer))==set(payload),"Updater does not contain seven binaries")
    check(all(sha(embedded_payload(installer)[k])==sha(v) for k,v in payload.items()),"Updater components changed")
    check(len(installer)<25<<20,"Installer exceeds 25 MB static asset budget")
    run([str((out/"Atualizar-Bridge.exe").resolve()),"--coop-progress-smoke"],cwd=out,timeout=30)

    # Apply the actual Hotfix installer to a running old 2.2.3.1 Setup in a
    # sandbox, using the already published P17 Coop parts from local cache.
    shutil.copy2(approved/"coop-spec.json",out/"coop-spec.json")
    shutil.copytree(approved/"coop-assets",out/"coop-assets")
    run(["python",str(ROOT/"scripts/test_2222_test2_install.py"),str(out),"--baseline",str(approved)],timeout=300)
    install=readjson(out/"one-click-install-test.json")
    check(install["status"]=="passed" and install["invocations"]==1 and install["p17_actual_updater_migration_tested"],"Updater migration test failed")

    new_report={
      "version":VERSION,"sequence":SEQUENCE,"candidate_base_run":42116052029,
      "candidate_base_artifact":13457334107,
      "candidate_base_updater_sha256":BASE_SHA,
      "candidate_base_coop_archive_sha256":COOP_SHA,
      "updater_sha256":sha(installer),"installer_size":len(installer),
      "setup_sha256":new_setup_sha,"prior_setup_sha256":report["components_sha256"]["Setup.exe"],
      "native_version":UPSTREAM,"coop_protocol":17,
      "parts_unchanged":len(spec["parts"]),"managed_files_unchanged":len(spec["files"]),
      "other_six_components_unchanged":True,
      "windows_setup_activation_tested":True,"windows_ifoe_denied_registry_tested":True,
      "windows_bcs_native_fallback_tested":True,"windows_gameplay_tested":False,
      "languages":["pt","en","de","es","fr","ru"],"installer_first_click_tested":True,
      "signed":False
    }
    (out/"build-report.json").write_text(json.dumps(new_report,indent=2)+"\n",encoding="utf-8")
    # Do not include the 750 MB input and P17 archive parts as an artifact.
    print("HOTFIX 1 APPROVED CANDIDATE",VERSION,SEQUENCE,new_report["updater_sha256"],new_report["installer_size"],flush=True)

if __name__=="__main__":main()
