#!/usr/bin/env python3
"""Test and assemble a complete source-backed Windows x86 bridge package."""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "source"
REFERENCE_GO = "go1.23.2"
VERSION = re.search(r'const bridgeVersion = "([^"]+)"', (SOURCE / "version.go").read_text()).group(1)
PACKAGE_NAME = f"OpenOMSI_BCS_Bridge_v{VERSION}_by_Mayconrib808"
TEST_COMMON = (
    "main.go timetable.go diagnostics.go launch_checks.go facade_memory.go "
    "driver.go session.go config.go plugin_host.go registry.go setup_files.go "
    "paths.go version.go"
).split()
TEST_FILES = (
    "driver_test.go session_test.go setup_test.go diagnostics_test.go "
    "timetable_test.go timetable_endpoints_test.go regression_test.go "
    "launch_checks_test.go"
).split()
PROGRAMS = {
    "Setup.exe": (
        "setup_main.go setup_files.go diagnostics.go setup_windows.go "
        "config.go plugin_host.go registry.go paths.go version.go"
    ).split(),
    "app/OpenOMSI_BCS_Bridge.exe": (
        "main.go timetable.go diagnostics.go launch_checks.go driver.go session.go "
        "config.go plugin_host.go paths.go version.go process_windows.go"
    ).split(),
    "app/compat/Omsi.exe": (
        "compat.go facade_memory.go driver.go version.go"
    ).split(),
}
HOST = "app/compat/omsi-plugin-host32.exe"
HOST_SOURCES = ["pluginhost/" + name for name in (
    "main.go", "wire.go", "environment.go", "dll_windows.go"
)]


def run(go: str, args: list[str], env: dict[str, str]) -> None:
    print("+ go " + " ".join(args), flush=True)
    subprocess.run([go, *args], cwd=SOURCE, env=env, check=True)


def assemble(stage: Path, host_hash: str) -> None:
    ignore = shutil.ignore_patterns("__pycache__", "*.pyc", "*.exe", "*.dll", "*.obj", "*.lib", "*.exp")
    for name in ("source", "docs", "scripts"):
        shutil.copytree(ROOT / name, stage / name, ignore=ignore)
    for p in ROOT.glob("*.md"):
        shutil.copy2(p, stage / p.name)
    for name in ("LICENSE", "LEIA-ME.txt", "TUTORIAL.html"):
        shutil.copy2(ROOT / name, stage / name)
    shutil.copy2(ROOT / "app/bridge.ini.example", stage / "app/bridge.ini.example")
    (stage / "docs/BUILD-INFO.txt").write_text(
        f"OpenOMSI BCS Bridge v{VERSION} - by Mayconrib808\n"
        f"Toolchain: {REFERENCE_GO}; GOOS=windows; GOARCH=386; GO386=sse2; CGO_ENABLED=0\n"
        "All four executables are built from the source included in this package.\n"
        f"Plugin host SHA-256: {host_hash}\n"
        "The historical host with unresolved provenance is not bundled.\n"
        "Expected integration: openOMSI 0.2.0 Windows x64 + BCS/BBS 5.0.0.1.\n"
        "Automated checks do not certify a real vendor-plugin session or trip evaluation.\n"
        "See docs/VALIDATION.md and docs/TEST_ON_WINDOWS.md.\n",
        encoding="utf-8",
    )
    inventory = stage / "docs/PACKAGE_FILES.txt"
    paths = sorted(p.relative_to(stage).as_posix() for p in stage.rglob("*") if p.is_file())
    paths.append("docs/PACKAGE_FILES.txt")
    inventory.write_text("\n".join(sorted(paths)) + "\n", encoding="utf-8")
    manifest = "".join(
        hashlib.sha256((stage / rel).read_bytes()).hexdigest() + "  " + rel + "\n"
        for rel in sorted(paths)
    )
    (stage / "docs/SHA256.txt").write_text(manifest, encoding="utf-8")


def archive_package(directory: Path) -> Path:
    target = directory.with_name(directory.name + ".zip")
    with zipfile.ZipFile(target, "x", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for p in sorted(directory.rglob("*")):
            if not p.is_file():
                continue
            entry = zipfile.ZipInfo(p.relative_to(directory).as_posix(), date_time=(2026, 10, 6, 0, 0, 0))
            entry.compress_type = zipfile.ZIP_DEFLATED
            entry.external_attr = (0o100644 << 16)
            archive.writestr(entry, p.read_bytes(), compresslevel=9)
    digest = hashlib.sha256(target.read_bytes()).hexdigest()
    target.with_name(target.name + ".sha256").write_text(f"{digest}  {target.name}\n", encoding="utf-8")
    print(f"Complete package: {target}\nZIP SHA-256: {digest}")
    return target


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-only", action="store_true", help="Run complete build/checks in a temporary folder.")
    parser.add_argument("--output", type=Path, default=ROOT / "dist" / PACKAGE_NAME,
                        help="New full-package directory. A ZIP and checksum are written beside it.")
    args = parser.parse_args()
    go = shutil.which("go")
    if go is None:
        parser.error("Install Go 1.23.2 and put its bin directory on PATH.")
    native = os.environ.copy()
    native.update(GO111MODULE="off", CGO_ENABLED="0", GOTOOLCHAIN="local", GOEXPERIMENT="")
    native.pop("GOOS", None)
    native.pop("GOARCH", None)
    version = subprocess.check_output([go, "version"], env=native, text=True).strip()
    if version.split()[2] != REFERENCE_GO:
        parser.error(f"This facade layout uses {REFERENCE_GO}; found {version}.")
    if not args.check_only and any(p.exists() for p in (
        args.output, args.output.with_name(args.output.name + ".zip"),
        args.output.with_name(args.output.name + ".zip.sha256"),
    )):
        parser.error("The output directory/ZIP/checksum already exists; choose a new --output path.")
    platform_file = "process_windows.go" if os.name == "nt" else "process_teststub_test.go"
    test_sources = [*TEST_COMMON, platform_file, *TEST_FILES]
    run(go, ["test", "-v", *test_sources], native)
    run(go, ["vet", *test_sources], native)
    run(go, ["test", "-v", "./pluginhost"], native)
    run(go, ["vet", "./pluginhost"], native)
    windows = native.copy()
    windows.update(GOOS="windows", GOARCH="386", GO386="sse2")
    run(go, ["vet", "./pluginhost"], windows)
    with tempfile.TemporaryDirectory(prefix="openomsi-bbs-build-") as temporary:
        stage = Path(temporary) / "package"
        host = stage / HOST
        host.parent.mkdir(parents=True, exist_ok=True)
        # Explicit files give the main program a stable import path outside
        # GOPATH/modules; -trimpath then removes checkout-machine paths.
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w", "-o", str(host), *HOST_SOURCES], windows)
        host_hash = hashlib.sha256(host.read_bytes()).hexdigest()
        print(f"Source-backed host SHA-256: {host_hash}", flush=True)
        digest_flag = "-X=main.bundledPluginHostSHA256=" + host_hash
        for rel, sources in PROGRAMS.items():
            target = stage / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            flags = "-s -w" + (" -H=windowsgui" if rel != "Setup.exe" else "")
            if "plugin_host.go" in sources:
                flags += " " + digest_flag
            run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=" + flags, "-o", str(target), *sources], windows)
        symbols = Path(temporary) / "facade-symbols.exe"
        run(go, ["build", "-trimpath", "-buildvcs=false", "-ldflags=-H=windowsgui", "-o", str(symbols),
                 *PROGRAMS["app/compat/Omsi.exe"]], windows)
        run(go, ["run", "tools/release_tools.go", "layout", str(stage / "app/compat/Omsi.exe"), str(symbols)], native)
        if os.name == "nt":
            subprocess.run([sys.executable, str(ROOT / "scripts/test_pluginhost_windows.py"), str(host)], check=True, timeout=180)
        else:
            print("Native DLL/message-pump checks run on the Windows CI job, not on this host.")
        assemble(stage, host_hash)
        package_env = native.copy()
        package_env["BRIDGE_BUILT_PACKAGE"] = str(stage)
        # go test compiles the tested package under command-line-arguments and
        # creates a separate generated main; target the tested package's symbol.
        test_digest_flag = "-X=command-line-arguments.bundledPluginHostSHA256=" + host_hash
        run(go, ["test", "-v", "-ldflags=" + test_digest_flag, "-run=^TestBuiltRuntimePackage$", *test_sources], package_env)
        if not args.check_only:
            args.output.parent.mkdir(parents=True, exist_ok=True)
            shutil.copytree(stage, args.output)
            archive_package(args.output)
    print("Bridge/host tests, vet, all four Windows builds, facade layout and full-package integrity passed.")
    print("Live BCS panel, UAC and game-trip evaluation require the separately installed products.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode)
