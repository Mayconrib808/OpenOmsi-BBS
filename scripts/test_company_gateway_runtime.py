#!/usr/bin/env python3
"""Run the host adapter against an unchanged, SHA-256-verified official server."""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import urllib.request
import zipfile

import build

VERSION = "0.2.23"
ASSETS = {
    "nt": ("windows", "b8296340956670031de576056776ef6f02b62ddaa11c63b5d6c4f40988b49bd5"),
    "posix": ("linux", "57822a77d899494d47b95c6c5a6e37f77a81cdc81a3dc1df60cb0de32c5df527"),
}


def download_server(destination: Path) -> Path:
    platform, expected = ASSETS[os.name]
    asset = f"openOMSI-{VERSION}-server-{platform}-x64.zip"
    destination.mkdir(parents=True, exist_ok=True)
    archive = destination / asset
    url = f"https://github.com/openOMSI-org/openOMSI/releases/download/v{VERSION}/{asset}"
    if not archive.is_file() or hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        request = urllib.request.Request(url, headers={"User-Agent": "OpenOmsi-BBS-runtime-test"})
        with urllib.request.urlopen(request, timeout=90) as response, archive.open("wb") as output:
            shutil.copyfileobj(response, output)
    if hashlib.sha256(archive.read_bytes()).hexdigest() != expected:
        raise ValueError("Official server ZIP failed release SHA-256 verification")
    extracted = destination / "extracted"
    extracted.mkdir(exist_ok=True)
    executable_name = "openomsi.exe" if os.name == "nt" else "openomsi"
    with zipfile.ZipFile(archive) as package:
        for entry in package.infolist():
            target = (extracted / entry.filename).resolve()
            if not target.is_relative_to(extracted.resolve()):
                raise ValueError("Unsafe path in official archive")
        package.extractall(extracted)
    matches = list(extracted.rglob(executable_name))
    if len(matches) != 1:
        raise ValueError("Official server executable could not be located uniquely")
    matches[0].chmod(0o755)
    return matches[0].resolve()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", type=Path, help="Existing official executable for an offline runtime test.")
    args = parser.parse_args()
    executable = args.server.resolve() if args.server else download_server(build.ROOT / "build" / "official-gateway-test")
    go = shutil.which("go")
    if not go:
        parser.error("Go 1.23.2 must be on PATH")
    env = os.environ.copy()
    env.update(GO111MODULE="off", CGO_ENABLED="0", GOTOOLCHAIN="local", OPENOMSI_GATEWAY_TEST_SERVER=str(executable))
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    platform = ["process_windows.go", "company_host_process_windows.go", "openomsi_probe_windows.go"] if os.name == "nt" else ["process_teststub_test.go", "company_host_process_other.go", "openomsi_probe_other.go"]
    sources = [*build.TEST_COMMON, *platform, *build.TEST_FILES]
    subprocess.run([go, "test", "-count=1", "-run=^TestCompanyGatewayOfficialRuntime$", "-v", *sources], cwd=build.SOURCE, env=env, check=True)


if __name__ == "__main__":
    main()
