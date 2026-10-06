#!/usr/bin/env python3
"""Check/build the three Go components; never bundle an unverified native host."""
from __future__ import annotations

import argparse
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "source"
REFERENCE_GO = "go1.23.2"
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


def run(go: str, args: list[str], env: dict[str, str]) -> None:
    print("+ go " + " ".join(args), flush=True)
    subprocess.run([go, *args], cwd=SOURCE, env=env, check=True)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check-only", action="store_true",
                        help="Run tests, vet, Windows compilation and PE checks in a temporary folder.")
    parser.add_argument("--output", type=Path, default=ROOT / "dist/source-build",
                        help="New destination for development components; must not already exist.")
    args = parser.parse_args()
    go = shutil.which("go")
    if go is None:
        parser.error("Install Go 1.23.2 and put its bin directory on PATH.")
    native = os.environ.copy()
    native.update(GO111MODULE="off", CGO_ENABLED="0", GOTOOLCHAIN="local")
    native.pop("GOOS", None)
    native.pop("GOARCH", None)
    version = subprocess.check_output([go, "version"], env=native, text=True).strip()
    if version.split()[2] != REFERENCE_GO:
        parser.error(f"This facade layout uses {REFERENCE_GO}; found {version}.")
    if not args.check_only and args.output.exists():
        parser.error("The output directory already exists; choose a new --output path.")
    platform_file = "process_windows.go" if os.name == "nt" else "process_teststub_test.go"
    test_sources = [*TEST_COMMON, platform_file, *TEST_FILES]
    run(go, ["test", "-v", *test_sources], native)
    run(go, ["vet", *test_sources], native)
    windows = native.copy()
    windows.update(GOOS="windows", GOARCH="386")
    with tempfile.TemporaryDirectory(prefix="openomsi-bbs-build-") as temporary:
        stage = Path(temporary) / "components"
        for rel, sources in PROGRAMS.items():
            target = stage / rel
            target.parent.mkdir(parents=True, exist_ok=True)
            flags = "-s -w" + (" -H=windowsgui" if rel != "Setup.exe" else "")
            run(go, ["build", "-trimpath", "-ldflags=" + flags, "-o", str(target), *sources], windows)
        symbols = Path(temporary) / "facade-symbols.exe"
        run(go, ["build", "-trimpath", "-ldflags=-H=windowsgui", "-o", str(symbols),
                 *PROGRAMS["app/compat/Omsi.exe"]], windows)
        run(go, ["run", "tools/release_tools.go", "layout",
                 str(stage / "app/compat/Omsi.exe"), str(symbols)], native)
        if not args.check_only:
            shutil.copytree(SOURCE, stage / "source")
            shutil.copytree(ROOT / "docs", stage / "docs")
            shutil.copytree(ROOT / "scripts", stage / "scripts",
                            ignore=shutil.ignore_patterns("__pycache__", "*.pyc"))
            for p in ROOT.glob("*.md"):
                shutil.copy2(p, stage / p.name)
            shutil.copy2(ROOT / "LICENSE", stage / "LICENSE")
            shutil.copy2(ROOT / "app/bridge.ini.example", stage / "app/bridge.ini.example")
            (stage / "SOURCE_BUILD.txt").write_text(
                "DEVELOPMENT COMPONENTS ONLY - NOT A COMPLETE RUNTIME RELEASE.\n"
                "The custom native 32-bit plugin host is intentionally excluded.\n"
                "See docs/HELPER_PROVENANCE.md before attempting activation.\n"
                "No proprietary OMSI 2, BCS/BBS or DLC components are supplied.\n",
                encoding="utf-8",
            )
            args.output.parent.mkdir(parents=True, exist_ok=True)
            shutil.copytree(stage, args.output)
            print(f"Development components: {args.output}")
            print("No native helper or ready-to-play release ZIP was generated.")
    print("Portable tests, vet, three Windows builds and facade layout checks passed.")
    print("This result does not verify live Windows/BCS integration.")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode)
