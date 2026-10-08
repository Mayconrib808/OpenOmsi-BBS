#!/usr/bin/env python3
"""Publish the authorized development ZIP after this push passes both CI jobs."""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import struct
import subprocess
import zipfile

from publish_release import hashes

ROOT = Path(__file__).resolve().parents[1]
REPO = "Mayconrib808/OpenOmsi-BBS"
VERSION = "2.0.3-dev.1"
BRANCH = "codex/2.0.3-dev.1"
PACKAGE = f"OpenOmsi.+.BBS.{VERSION}.zip"
BINARIES = {"Setup.exe", "HostAgent.exe", "CompanyHost.exe", "app/OpenOMSI_BCS_Bridge.exe", "app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"}

def gh(*args):
    return subprocess.check_output(["gh", *args], text=True)

def verify_package(path):
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    if hashes(path.with_name(path.name + ".sha256").read_text()) != {path.name: digest}:
        raise ValueError("Development ZIP checksum mismatch")
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)):
            raise ValueError("Duplicate package entries")
        for name in names:
            part = PurePosixPath(name)
            if part.is_absolute() or ".." in part.parts or "\\" in name:
                raise ValueError("Unsafe package path")
        manifest = hashes(archive.read("docs/SHA256.txt").decode())
        if set(manifest) != set(names) - {"docs/SHA256.txt"}:
            raise ValueError("Incomplete package manifest")
        for name, expected in manifest.items():
            if hashlib.sha256(archive.read(name)).hexdigest() != expected:
                raise ValueError(f"Package entry checksum mismatch: {name}")
        if {name for name in names if name.lower().endswith(".exe")} != BINARIES:
            raise ValueError("Development package must contain exactly six executables")
        # Require Windows x86 and the GUI subsystem for the facade/helper.
        for name in BINARIES:
            binary = archive.read(name)
            pe = struct.unpack_from("<I", binary, 0x3c)[0]
            if binary[:2] != b"MZ" or binary[pe:pe+4] != b"PE\0\0" or struct.unpack_from("<H", binary, pe+4)[0] != 0x14c:
                raise ValueError(f"Invalid x86 Windows image: {name}")
            if name in {"app/compat/Omsi.exe", "app/compat/omsi-plugin-host32.exe"} and struct.unpack_from("<H", binary, pe+24+68)[0] != 2:
                raise ValueError(f"Unexpected console subsystem: {name}")
        # The artifact must contain the source checked out at the tested commit.
        for name in names:
            if name.startswith(("source/", "scripts/", "relay/")):
                local = ROOT / name
                if not local.is_file() or archive.read(name) != local.read_bytes():
                    raise ValueError(f"Artifact source differs from this commit: {name}")
        if archive.read("source/version.go") != (ROOT / "source/version.go").read_bytes():
            raise ValueError("Development version source mismatch")
    print(f"Verified development ZIP, source, six binaries and complete manifest: {digest}", flush=True)
    return digest

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--commit", required=True)
    parser.add_argument("--artifact-dir", type=Path, required=True)
    args = parser.parse_args()
    actual = re.search(r'const bridgeVersion = "([^"]+)"', (ROOT / "source/version.go").read_text()).group(1)
    if actual != VERSION or os.environ.get("GITHUB_REPOSITORY") != REPO or os.environ.get("GITHUB_EVENT_NAME") != "push" or os.environ.get("GITHUB_REF") != "refs/heads/" + BRANCH:
        raise ValueError("This publication is restricted to the authorized development branch/version")
    if not re.fullmatch(r"[0-9a-f]{40}", args.commit) or gh("api", f"repos/{REPO}/commits/{args.commit}", "--jq", ".sha").strip() != args.commit:
        raise ValueError("Invalid publication commit")
    run = json.loads(gh("api", f"repos/{REPO}/actions/runs/{args.run_id}"))
    if run["head_sha"] != args.commit or run["head_branch"] != BRANCH or run["event"] != "push" or run["head_repository"]["full_name"] != REPO:
        raise ValueError("Workflow identity mismatch")
    current = json.loads(gh("api", f"repos/{REPO}/git/ref/heads/{BRANCH}"))
    if current["object"]["sha"] != args.commit:
        raise ValueError("A newer development push superseded this publication")
    if subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip() != args.commit:
        raise ValueError("Checkout does not match the tested push")
    jobs = json.loads(gh("api", f"repos/{REPO}/actions/runs/{args.run_id}/jobs?per_page=100"))["jobs"]
    for name in ("check (ubuntu-latest)", "check (windows-latest)"):
        matching = [j for j in jobs if j["name"] == name]
        if len(matching) != 1 or matching[0]["conclusion"] != "success":
            raise ValueError(f"Required CI job did not pass: {name}")
    args.artifact_dir.mkdir(parents=True, exist_ok=True)
    gh("run", "download", args.run_id, "--repo", REPO, "--name", "OpenOMSI-BCS-Bridge-v" + VERSION, "--dir", str(args.artifact_dir))
    package = args.artifact_dir / PACKAGE
    digest = verify_package(package)
    releases = json.loads(gh("api", f"repos/{REPO}/releases?per_page=100"))
    tag = "v" + VERSION
    if any(r["tag_name"] == tag for r in releases):
        raise ValueError("Development release already exists; it will not be overwritten")
    notes = args.artifact_dir / "release-notes.md"
    notes.write_text((ROOT / f"docs/releases/{tag}.md").read_text() +
        f"\nValidação automática: [Linux e Windows]({run['html_url']}), commit `{args.commit}`.\n\nZIP SHA-256: `{digest}`.\n", encoding="utf-8")
    gh("release", "create", tag, str(package), str(package.with_name(package.name + ".sha256")),
       "--repo", REPO, "--target", args.commit, "--prerelease", "--latest=false",
       "--title", "OpenOmsi + BBS 2.0.3 dev1", "--notes-file", str(notes))
    release = json.loads(gh("api", f"repos/{REPO}/releases/tags/{tag}"))
    assets = {a["name"]:a for a in release["assets"]}
    for path in (package, package.with_name(package.name + ".sha256")):
        asset = assets[path.name]
        if asset["state"] != "uploaded" or asset["size"] != path.stat().st_size or asset.get("digest") != "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest():
            raise ValueError("Published development asset differs from the checked artifact")
    if release["draft"] or not release["prerelease"]:
        raise ValueError("Development publication must be a public prerelease")
    print(release["html_url"], flush=True)

if __name__ == "__main__":
    main()
