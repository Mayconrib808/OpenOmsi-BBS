#!/usr/bin/env python3
"""Run the bridge's actual metadata checks against pinned official Windows builds.

This checks CLI/native-plugin-host prerequisites without loading OMSI content.
It does not replace a real BBS trip or a multiplayer test with two players.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import shutil
import stat
import struct
import subprocess
import tempfile
import time
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[1]
SOURCE = ROOT / "source"
RELEASES = (
    ("0.2.0", "windows-x64", "d4ab6aca3efadbc0fe593eeee5633325d8335eb0101ff430a7b9f82be3fee9a6"),
    ("0.2.0", "server-windows-x64", "d9b68d9c8b79d53bf68aaa523734c81fb6f401244a61b61e979c8634dc6e4a11"),
    ("0.2.11", "windows-x64", "e37558f744080dd39ed6552eadbb7bdc9e5198b2714d7104949e69844dec9002"),
    ("0.2.11", "server-windows-x64", "93ffa3cdf0c335b7d7fd29864f9d257528bd68bc992fbd096cad6184eb5bc18c"),
)
MAX_DOWNLOAD = 64 << 20
MAX_EXPANDED = 512 << 20


class HTTPSRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        if not newurl.startswith("https://"):
            raise ValueError("Refusing an unencrypted release download redirect")
        return super().redirect_request(req, fp, code, msg, headers, newurl)


def download_release(version: str, platform: str, digest: str, target: Path) -> None:
    name = f"openOMSI-{version}-{platform}.zip"
    url = f"https://github.com/openOMSI-Project/openOMSI/releases/download/v{version}/{name}"
    request = urllib.request.Request(url, headers={"User-Agent": "OpenOmsi-BBS-Compatibility-Smoke/2.0.1"})
    opener = urllib.request.build_opener(HTTPSRedirect())
    deadline = time.monotonic() + 120
    count = 0
    sha256 = hashlib.sha256()
    with opener.open(request, timeout=20) as response, target.open("wb") as output:
        if response.status != 200:
            raise ValueError(f"Release download returned HTTP {response.status}")
        while chunk := response.read(1 << 20):
            count += len(chunk)
            if count > MAX_DOWNLOAD or time.monotonic() > deadline:
                raise ValueError("Release download exceeded its size or time limit")
            output.write(chunk)
            sha256.update(chunk)
    if sha256.hexdigest() != digest:
        raise ValueError(f"Official release digest differs for {name}: {sha256.hexdigest()}")


def extract_release(archive: Path, target: Path) -> Path:
    target.mkdir()
    seen = set()
    total = 0
    executables = []
    with zipfile.ZipFile(archive) as package:
        if len(package.infolist()) > 4096:
            raise ValueError("Release archive contains too many entries")
        for entry in package.infolist():
            name = entry.filename.replace("\\", "/")
            relative = PurePosixPath(name)
            if relative.is_absolute() or any(p in {"..", ""} or ":" in p or "\x00" in p for p in relative.parts):
                raise ValueError(f"Unsafe release archive path: {entry.filename}")
            key = str(relative).casefold()
            if key in seen or stat.S_ISLNK(entry.external_attr >> 16):
                raise ValueError(f"Duplicate or linked release archive path: {entry.filename}")
            seen.add(key)
            total += entry.file_size
            if total > MAX_EXPANDED:
                raise ValueError("Release archive exceeds the expanded-size limit")
            destination = target.joinpath(*relative.parts)
            if entry.is_dir():
                destination.mkdir(parents=True, exist_ok=True)
                continue
            destination.parent.mkdir(parents=True, exist_ok=True)
            with package.open(entry) as source, destination.open("wb") as output:
                shutil.copyfileobj(source, output)
            if destination.name.casefold() == "openomsi.exe":
                executables.append(destination)
    if len(executables) != 1:
        raise ValueError(f"Expected exactly one openomsi.exe, found {len(executables)}")
    executable = executables[0]
    with executable.open("rb") as stream:
        header = stream.read(64)
        if len(header) != 64 or header[:2] != b"MZ":
            raise ValueError("Official openomsi.exe has no valid DOS header")
        offset = struct.unpack_from("<I", header, 60)[0]
        if offset > 1 << 20:
            raise ValueError("Invalid PE header offset")
        stream.seek(offset)
        pe = stream.read(6)
        if len(pe) != 6 or pe[:4] != b"PE\0\0" or struct.unpack_from("<H", pe, 4)[0] != 0x8664:
            raise ValueError("Official openomsi.exe is not Windows x64")
    return executable


def build_probe(go: str, target: Path) -> Path:
    sources = target / "probe"
    sources.mkdir()
    for name in ("openomsi_compatibility.go", "openomsi_probe_windows.go"):
        shutil.copy2(SOURCE / name, sources / name)
    (sources / "main.go").write_text(
        '''package main
import ("encoding/json"; "fmt"; "os")
func main() {
    if len(os.Args) != 3 { panic("executable path and mode are required") }
    dedicated := os.Args[2] == "server"
    result, err := checkOpenOMSICompatibility(os.Args[1], dedicated, !dedicated)
    if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
    if err := json.NewEncoder(os.Stdout).Encode(result); err != nil { panic(err) }
}
''', encoding="utf-8"
    )
    executable = sources / "compatibility-probe.exe"
    env = os.environ.copy()
    env.update(GO111MODULE="off", GOOS="windows", GOARCH="amd64", CGO_ENABLED="0")
    subprocess.run([go, "build", "-o", str(executable), "main.go", "openomsi_compatibility.go", "openomsi_probe_windows.go"],
                   cwd=sources, env=env, check=True, timeout=90)
    return executable


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default=shutil.which("go"), help="Go executable for the actual bridge probe")
    parser.add_argument("--report", type=Path, help="Optional JSON report path")
    args = parser.parse_args()
    if os.name != "nt":
        parser.error("This smoke check runs the official Windows executables and requires Windows")
    if not args.go:
        parser.error("Go was not found")
    report = []
    with tempfile.TemporaryDirectory(prefix="openomsi-cli-smoke-") as temporary:
        directory = Path(temporary)
        probe = build_probe(args.go, directory)
        for version, platform, digest in RELEASES:
            print(f"Checking pinned official openOMSI {version} / {platform}", flush=True)
            archive = directory / f"{version}-{platform}.zip"
            download_release(version, platform, digest, archive)
            executable = extract_release(archive, directory / f"{version}-{platform}")
            mode = "server" if platform.startswith("server-") else "player"
            result = subprocess.run([str(probe), str(executable), mode], check=True, capture_output=True,
                                    text=True, encoding="utf-8", timeout=25)
            metadata = json.loads(result.stdout)
            report.append({"official_release": version, "platform": platform, "archive_sha256": digest,
                           "reported_version": metadata["Version"], "cli_and_host_prerequisites": "passed"})
            print(json.dumps(report[-1]), flush=True)
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print("Official CLI prerequisite checks passed; real BBS trips and two-player gameplay remain separate tests.")


if __name__ == "__main__":
    main()
