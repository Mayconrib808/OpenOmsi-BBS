#!/usr/bin/env python3
"""Build the protocol-6 BBS dedicated server from pinned openOMSI + reviewed patch."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
NATIVE = ROOT / "native"
PIN = json.loads((NATIVE / "upstream.json").read_text())
PE_FILES = ("openomsi.exe", "steam_api64.dll", "vcruntime140.dll", "vcruntime140_1.dll", "msvcp140.dll")

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def prepare(source, archive):
    if not archive.is_file():
        archive.parent.mkdir(parents=True, exist_ok=True)
        with urllib.request.urlopen(PIN["url"], timeout=60) as response:
            archive.write_bytes(response.read())
    if sha(archive) != PIN["sha256"]:
        raise ValueError("Upstream source archive hash mismatch")
    if source.exists():
        raise ValueError("Choose a fresh native source directory")
    source.mkdir(parents=True)
    prefix = PIN["archive_root"] + "/"
    with zipfile.ZipFile(archive) as z:
        for name in z.namelist():
            if not name.startswith(prefix) or name.endswith("/"):
                continue
            relative = Path(name[len(prefix):])
            if relative.is_absolute() or ".." in relative.parts:
                raise ValueError("Unsafe upstream source path")
            path = source / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(z.read(name))
    patch = NATIVE / "free-player-vehicles.patch"
    # This source tree is inside the bridge checkout; apply paths relative to it.
    relative = source.resolve().relative_to(ROOT).as_posix()
    command = ["git", "-c", "core.autocrlf=false", "apply", "--directory=" + relative]
    subprocess.run([*command, "--check", str(patch)], cwd=ROOT, check=True)
    subprocess.run([*command, str(patch)], cwd=ROOT, check=True)
    for name, digest in PIN["patched_files"].items():
        if sha(source / name) != digest:
            raise ValueError("Patched source identity mismatch: " + name)
    print("Pinned openOMSI source and every patched file verified", flush=True)

def verify_package(directory):
    info = json.loads((directory / "bbs-server.json").read_text())
    if info["upstream_sha256"] != PIN["sha256"] or info["patch_sha256"] != sha(NATIVE / "free-player-vehicles.patch") or info["version"] != PIN["version"]:
        raise ValueError("Native server provenance differs from this source")
    if set(info["files"]) != set(PE_FILES):
        raise ValueError("Incomplete native runtime inventory")
    for name, digest in info["files"].items():
        if sha(directory / name) != digest:
            raise ValueError("Native runtime checksum mismatch: " + name)
    return info

def build(source, output):
    if os.name != "nt":
        raise ValueError("Build the dedicated Windows x64 server on Windows")
    for name, digest in PIN["patched_files"].items():
        if sha(source / name) != digest:
            raise ValueError("Native source was modified after preparation: " + name)
    env = os.environ.copy()
    env["OPENOMSI_VERSION"] = PIN["version"]
    tests = [
        ["test", "--locked", "--release", "-p", "omsi-app", "free_vehicles::"],
        ["test", "--locked", "--release", "-p", "omsi-app", "server::"],
        ["test", "--locked", "--release", "-p", "omsi-net"],
        ["build", "--locked", "--release", "-p", "omsi-app"],
    ]
    for args in tests:
        print("+ cargo " + " ".join(args), flush=True)
        subprocess.run(["cargo", *args], cwd=source, env=env, check=True)
    output.mkdir(parents=True, exist_ok=False)
    built = source / "target/release"
    for name in ("openomsi.exe", "steam_api64.dll"):
        shutil.copy2(built / name, output / name)
    # Ship the same app-local Microsoft runtimes as upstream's dedicated package.
    vswhere = Path(os.environ["ProgramFiles(x86)"]) / "Microsoft Visual Studio/Installer/vswhere.exe"
    vs = Path(subprocess.check_output([str(vswhere), "-latest", "-products", "*", "-property", "installationPath"], text=True).strip())
    crts = sorted((vs / "VC/Redist/MSVC").glob("*/x64/Microsoft.VC*.CRT"))
    crt = crts[-1] if crts else Path(os.environ["SystemRoot"]) / "System32"
    for name in PE_FILES[2:]:
        shutil.copy2(crt / name, output / name)
    for flag in ("--version", "--help"):
        result = subprocess.run([str((output / "openomsi.exe").resolve()), flag], capture_output=True, text=True, check=True, timeout=20)
        text = result.stdout + result.stderr
        if flag == "--help" and ("--server" not in text or "--root" not in text):
            raise ValueError("Native dedicated-server CLI capabilities missing")
        if flag == "--version" and PIN["version"] not in text:
            raise ValueError("Derivative server version identity missing")
        print(text.strip(), flush=True)
    shutil.copy2(NATIVE / "OpenOMSI-LICENSE.txt", output / "OpenOMSI-LICENSE.txt")
    (output / ".openomsi-content").write_text("")
    commit = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True).strip()
    info = {"version": PIN["version"], "commit": commit, "upstream_url": PIN["url"],
        "upstream_sha256": PIN["sha256"], "patch_sha256": sha(NATIVE / "free-player-vehicles.patch"),
        "rustc": subprocess.check_output(["rustc", "--version"], text=True).strip(),
        "files": {name: sha(output / name) for name in PE_FILES}}
    (output / "bbs-server.json").write_text(json.dumps(info, indent=2) + "\n", encoding="utf-8")
    verify_package(output)
    print("Native server regression tests, CLI probes and runtime inventory passed", flush=True)

def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source", type=Path, default=ROOT / "build/native-source")
    p.add_argument("--archive", type=Path, default=ROOT / "build/openomsi-source.zip")
    p.add_argument("--output", type=Path, default=ROOT / "build/native-server")
    p.add_argument("--prepare-only", action="store_true")
    p.add_argument("--build-only", action="store_true")
    a = p.parse_args()
    if not a.build_only:
        prepare(a.source, a.archive)
    if not a.prepare_only:
        build(a.source, a.output)

if __name__ == "__main__":
    main()
