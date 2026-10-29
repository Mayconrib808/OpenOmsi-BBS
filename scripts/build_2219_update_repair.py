"""Repair the localized release's update configuration without changing native assets."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import zipfile

import build
import build_bridge_update
import setup_resources
from build_runtime_2203 import embedded_payload

ROOT = Path(__file__).resolve().parents[1]
VERSION = "2.2.1.9-hotfix.3"
SEQUENCE = 2026102901
ORIGIN = "https://openomsi-bbs-updates.maycongamesmaximo.workers.dev"
BASE_INSTALLER = "98ec07db801f028deb0874718aac3251fb6f44893dd6c96a38d4e792289ee338"
BASE_COOP = "bef8e67010ea9c53fe2b80684ae0b8727b7f18e3b9f0e14c31ea5c75ab406574"
REBUILD = ("Setup.exe", "PeDePeAdapter/openomsi.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe")
sha = lambda data: hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=False)
    assert build.VERSION == VERSION
    assert subprocess.check_output(["go", "version"], text=True).split()[2] == "go1.23.2"
    original = (args.baseline / "Atualizar-Bridge.exe").read_bytes()
    prior = json.loads((args.baseline / "build-report.json").read_text(encoding="utf-8-sig"))
    spec = json.loads((args.baseline / "coop-spec.json").read_text(encoding="utf-8-sig"))
    assert sha(original) == BASE_INSTALLER == prior["updater_sha256"]
    assert spec["sha256"] == BASE_COOP == prior["coop_archive_sha256"]
    parts = []
    for item in spec["parts"]:
        data = (args.baseline / "coop-assets" / (item["sha256"] + ".bin")).read_bytes()
        assert len(data) == item["size"] and sha(data) == item["sha256"]
        parts.append(data)
    archive = b"".join(parts)
    assert len(archive) == spec["size"] and sha(archive) == BASE_COOP
    with zipfile.ZipFile(io.BytesIO(archive)) as z:
        assert set(z.namelist()) == {spec["prefix"] + name for name in spec["files"]}
        for name, digest in spec["files"].items():
            assert sha(z.read(spec["prefix"] + name)) == digest, name
    # The localized branch carried an obsolete Coop 12 manifest. Always compile
    # the exact retained Hotfix 2 archive, including its matched P15 server.
    manifest = (args.baseline / "coop_bundle_manifest.go").read_bytes()
    assert BASE_COOP.encode() in manifest
    (build.SOURCE / "coop_bundle_manifest.go").write_bytes(manifest)
    (out / "coop_bundle_manifest.go").write_bytes(manifest)
    shutil.copy2(args.baseline / "coop-spec.json", out / "coop-spec.json")
    payload = embedded_payload(original)
    assert {name: sha(data) for name, data in payload.items()} == prior["components_sha256"]
    retained = {name: sha(data) for name, data in payload.items() if name not in REBUILD}
    package = out / "runtime"
    for name, data in payload.items():
        target = package / name
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(data)
    # Retained servers are staged only so the actual complete host UI can be
    # tested on Windows; their production hashes and bytes stay unchanged.
    with zipfile.ZipFile(io.BytesIO(archive)) as z:
        for name in spec["files"]:
            if name.startswith(("app/server-dinamico-p14/", "app/server-dinamico-p15/")):
                target = package / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(z.read(spec["prefix"] + name))
    env = dict(os.environ, GOOS="windows", GOARCH="386", GO386="sse2", CGO_ENABLED="0", GOTOOLCHAIN="local", GO111MODULE="off", GOEXPERIMENT="")
    with tempfile.TemporaryDirectory() as temp:
        for name in REBUILD:
            flags = "-s -w -H=windowsgui -X=main.bundledPluginHostSHA256=" + sha(payload["app/compat/omsi-plugin-host32.exe"])
            if name == "Setup.exe":
                work = Path(temp) / "setup"
                work.mkdir()
                for filename in build.PROGRAMS[name]:
                    shutil.copy2(build.SOURCE / filename, work / filename)
                (work / "go.mod").write_text("module openomsi-bbs/setup\n\ngo 1.23.2\n")
                resources = setup_resources.prepare(work, build.SOURCE / "resources")
                flags += f" -X=main.bridgeUpdateOrigin={ORIGIN} -X=main.bridgeUpdateSequence={SEQUENCE}"
                sources, local_env = ["."], dict(env, GO111MODULE="on")
            else:
                work, sources, local_env = build.SOURCE, build.PROGRAMS[name], env
            target = package / name
            subprocess.run(["go", "build", "-trimpath", "-buildvcs=false", "-ldflags=" + flags, "-o", str(target), *sources], cwd=work, env=local_env, check=True)
            if name == "Setup.exe":
                setup_resources.verify_executable(target, resources)
            payload[name] = target.read_bytes()
    assert all(sha(payload[name]) == digest for name, digest in retained.items())
    embedded = build.SOURCE / "bridge_update_payload.zip"
    assert not embedded.exists()
    try:
        with zipfile.ZipFile(embedded, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
            for name, data in payload.items():
                z.writestr(name, data)
        subprocess.run(["go", "build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -H=windowsgui", "-o", str(out / "Atualizar-Bridge.exe"), *build_bridge_update.UPDATER_SOURCES], cwd=build.SOURCE, env=env, check=True)
    finally:
        embedded.unlink(missing_ok=True)
    installer = (out / "Atualizar-Bridge.exe").read_bytes()
    assert embedded_payload(installer) == payload
    assert len(installer) <= 25 << 20
    report = {
        "version": VERSION, "sequence": SEQUENCE, "update_origin": ORIGIN,
        "updater_sha256": sha(installer), "installer_size": len(installer),
        "components_sha256": {name: sha(data) for name, data in payload.items()},
        "retained_runtime_sha256": retained, "base_installer_sha256": BASE_INSTALLER,
        "coop_archive_sha256": BASE_COOP, "coop_archive_unchanged": True,
        "native_game_and_servers_unchanged": True, "complete_hosts_rebuilt": False,
        "localized_bcs_driver": True, "setup_update_unsaved_root": True,
        "setup_live_update_ui_tested": False, "two_pc_gameplay_tested": False,
    }
    if os.name == "nt":
        subprocess.run(["python", str(ROOT / "scripts/test_complete_host_ui.py"), str(package)], check=True)
        subprocess.run(["python", str(ROOT / "scripts/test_setup_update_service_windows.py"), str(package)], check=True)
        subprocess.run([str(package / "Setup.exe"), "--gui-smoke"], cwd=package, timeout=60, check=True)
        report["setup_live_update_ui_tested"] = True
        report["complete_host_ui_tested"] = True
    (out / "build-report.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
