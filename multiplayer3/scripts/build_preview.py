"""Build the isolated 3.x preview and minimal updates without compiling Rust."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import zipfile

import plugin_host_resources
import setup_resources

PRODUCT = "BBS+OpenOmsi Multiplayer"
IDENTITY = "bbs-openomsi-multiplayer"

def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()

def metadata(source: Path, version: str, arch: str, description: str, original: str):
    base = plugin_host_resources.version_strings
    def strings(v):
        values = base(v)
        values.update(ProductName=PRODUCT, FileDescription=description, OriginalFilename=original,
                      InternalName=original.removesuffix(".exe"))
        return values
    plugin_host_resources.version_strings = strings
    resources = {16: {1: plugin_host_resources.version_resource(version)}}
    if original == "Setup.exe":
        resources.update(setup_resources.resources_from_assets(source / "resources"))
    path = source / f"product_windows_{arch}.syso"
    setup_resources.write_coff(path, resources, arch)
    plugin_host_resources.version_strings = base
    return path, resources, strings(version)

def compile_exe(go: str, source: Path, arch: str, output: Path, flags: str, resources, fields):
    env = dict(os.environ, GOOS="windows", GOARCH=arch, CGO_ENABLED="0")
    subprocess.run([go, "build", "-trimpath", "-buildvcs=false", "-ldflags", flags,
                    "-o", str(output), "."], cwd=source, env=env, check=True)
    setup_resources.verify_executable(output, resources)
    if os.name == "nt":
        import ctypes
        from ctypes import wintypes
        api = ctypes.WinDLL("version", use_last_error=True)
        api.GetFileVersionInfoSizeW.argtypes = [wintypes.LPCWSTR, ctypes.POINTER(wintypes.DWORD)]
        api.GetFileVersionInfoSizeW.restype = wintypes.DWORD
        handle = wintypes.DWORD()
        size = api.GetFileVersionInfoSizeW(str(output), ctypes.byref(handle))
        if not size: raise ctypes.WinError(ctypes.get_last_error())
        buffer = ctypes.create_string_buffer(size)
        api.GetFileVersionInfoW.argtypes = [wintypes.LPCWSTR, wintypes.DWORD, wintypes.DWORD, ctypes.c_void_p]
        if not api.GetFileVersionInfoW(str(output), 0, size, buffer): raise ctypes.WinError(ctypes.get_last_error())
        api.VerQueryValueW.argtypes = [ctypes.c_void_p, wintypes.LPCWSTR, ctypes.POINTER(ctypes.c_void_p), ctypes.POINTER(wintypes.UINT)]
        for key, value in fields.items():
            pointer, length = ctypes.c_void_p(), wintypes.UINT()
            if not api.VerQueryValueW(buffer, "\\StringFileInfo\\040904B0\\" + key, ctypes.byref(pointer), ctypes.byref(length)) or not pointer.value:
                raise ValueError("Windows cannot read executable metadata: " + key)
            if ctypes.wstring_at(pointer, length.value).rstrip("\0") != value:
                raise ValueError("Incorrect Windows executable metadata: " + key)

def archive(path: Path, files: dict[str, Path]):
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        for name, source in sorted(files.items()):
            z.write(source, name)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--go", default="go")
    ap.add_argument("--server", type=Path, help="Verified server folder; omit for minimal update builds")
    ap.add_argument("--output", type=Path, required=True)
    ap.add_argument("--update-origin", default="https://openomsi-bbs-updates-v3.maycongamesmaximo.workers.dev", help="Separate deployed 3.x HTTPS feed")
    ap.add_argument("--sequence", default="2026101402")
    ap.add_argument("--source-commit", default=os.environ.get("GITHUB_SHA", ""))
    args = ap.parse_args()
    root = Path(__file__).resolve().parents[1]
    source = root / "source"
    version = re.search(r'const version = "([^"]+)"', (source / "main.go").read_text()).group(1)
    if not args.sequence.isdecimal() or (args.update_origin and not re.fullmatch(r"https://[a-z0-9.-]+", args.update_origin)):
        raise SystemExit("Invalid update origin or sequence")
    args.output = args.output.resolve(); args.output.mkdir(parents=True, exist_ok=True)
    receipt = None
    if args.server:
        args.server = args.server.resolve()
        receipt = json.loads((args.server / "bbs-server.json").read_text())
        for name, sha in receipt["files"].items():
            if Path(name).name != name or digest(args.server / name) != sha:
                raise SystemExit(f"Native server verification failed: {name}")
    with tempfile.TemporaryDirectory(prefix="bbs-v3-build-") as tmp:
        staging = Path(tmp)
        helper = staging / "omsi-plugin-host32.exe"
        obj, resources, fields = metadata(source / "pluginhost", version, "386", "BCS 32-bit plugin host", "omsi-plugin-host32.exe")
        try:
            compile_exe(args.go, source / "pluginhost", "386", helper, "-H=windowsgui", resources, fields)
        finally:
            obj.unlink(missing_ok=True)
        launcher = staging / "Setup.exe"
        obj, resources, fields = metadata(source, version, "amd64", PRODUCT + " setup and BCS launcher", "Setup.exe")
        flags = (f"-H=windowsgui -X main.bundledPluginHostSHA256={digest(helper)} "
                 f"-X main.bridgeUpdateSequence={args.sequence} -X main.bridgeUpdateOrigin={args.update_origin}")
        try:
            compile_exe(args.go, source, "amd64", launcher, flags, resources, fields)
        finally:
            obj.unlink(missing_ok=True)
        runtime = {"Setup.exe": launcher,
                   "OpenOMSI-BBS/PeDePeAdapter/openomsi.exe": launcher,
                   "OpenOMSI-BBS/compat/omsi-plugin-host32.exe": helper}
        build = {"product": IDENTITY, "version": version, "sequence": int(args.sequence), "channel": "preview", "source_commit": args.source_commit, "toolchain": subprocess.check_output([args.go,"version"],text=True).strip(),
                 "client_default": "latest-official", "update_origin": args.update_origin,
                 "native_server": receipt, "files": {n: digest(p) for n, p in runtime.items()},
                 "public_release_ready": False,
                 "bcs": "5.0.2.0", "multiplayer_protocol": 7, "minimum_openomsi": "0.2.48", "next_trip": "native-bcs-command", "obfuscation": False, "executable_packer": False, "authenticode": "unsigned", "virus_total": "not_scanned",
                 "pending": ["real_bcs_two_pc_full_trip", "virus_total_final_binary_report"]}
        manifest = staging / "package.json"; manifest.write_text(json.dumps(build, indent=2))
        payload_files = dict(runtime, **{"package.json": manifest})
        payload_path = source / "updater" / "update_payload.zip"
        generated = source / "updater" / "payload_generated.go"
        archive(payload_path, payload_files)
        generated.write_text('package main\nimport _ "embed"\n//go:embed update_payload.zip\nvar embeddedPayload []byte\nfunc init(){payload=embeddedPayload}\n')
        updater = args.output / f"Atualizar-Multiplayer-{version}.exe"
        obj, resources, fields = metadata(source / "updater", version, "amd64", PRODUCT + " updater", "Atualizar-Multiplayer.exe")
        try:
            compile_exe(args.go, source / "updater", "amd64", updater, "-H=windowsgui", resources, fields)
        finally:
            obj.unlink(missing_ok=True); payload_path.unlink(missing_ok=True); generated.unlink(missing_ok=True)
        runtime["OpenOMSI-BBS/build.json"] = manifest
        archive(args.output / f"BBS-OpenOmsi-Multiplayer-{version}-arquivos-atualizacao.zip", runtime)
        full = dict(runtime)
        if args.server:
            for file in args.server.rglob("*"):
                if file.is_file(): full["OpenOMSI-BBS/server/" + file.relative_to(args.server).as_posix()] = file
        for folder in (source, root / "scripts", root / "relay"):
            for file in folder.rglob("*"):
                if file.is_file() and file.suffix not in {".syso", ".pyc", ".zip", ".exe", ".dll"} and "__pycache__" not in file.parts:
                    full["OpenOMSI-BBS/" + file.relative_to(root).as_posix()] = file
        for name in ["LICENSE", "LEIA-ME.txt", "README-en.txt", "LESEN-de.txt", "DESENVOLVIMENTO.md", "TUTORIAL.html"]:
            full["OpenOMSI-BBS/" + name] = root / name
        if args.server:
            checks = staging / "SHA256SUMS.txt"
            checks.write_text("".join(digest(p)+"  "+n+"\n" for n,p in sorted(full.items())),encoding="utf-8")
            full["OpenOMSI-BBS/SHA256SUMS.txt"] = checks
            archive(args.output / f"BBS-OpenOmsi-Multiplayer-{version}-teste.zip", full)
        for p in args.output.glob("*.zip"):
            (p.with_name(p.name+".sha256")).write_text(digest(p)+"  "+p.name+"\n",encoding="utf-8")
        updater.with_name(updater.name+".sha256").write_text(digest(updater)+"  "+updater.name+"\n",encoding="utf-8")
        build["updater_sha256"] = digest(updater)
        build["updater_size"] = updater.stat().st_size
        (args.output / "BUILD-INFO.json").write_text(json.dumps(build, indent=2))
        print(json.dumps({"version": version, "files": [{"name": p.name, "size": p.stat().st_size, "sha256": digest(p)} for p in sorted(args.output.iterdir()) if p.is_file()]}, indent=2))

if __name__ == "__main__":
    main()
