#!/usr/bin/env python3
"""Publish a checked CI package whose executables match a live-tested release."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import zipfile

ROOT = Path(__file__).resolve().parents[1]
VERSION = re.search(r'const bridgeVersion = "([^"]+)"', (ROOT / "source/version.go").read_text()).group(1)
PACKAGE = f"OpenOMSI_BCS_Bridge_v{VERSION}_by_Mayconrib808.zip"
RELEASE_DIR = ROOT / "docs/releases"


def hashes(text: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for line in text.splitlines():
        if not line:
            continue
        digest, name = line.split("  ", 1)
        if not re.fullmatch(r"[0-9a-f]{64}", digest) or name in result:
            raise ValueError("Invalid or duplicate checksum entry")
        result[name] = digest
    return result


def verify_package(path: Path) -> str:
    checksum = hashes(path.with_name(path.name + ".sha256").read_text(encoding="utf-8"))
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    if checksum != {path.name: digest}:
        raise ValueError("ZIP checksum mismatch")
    approved = hashes((RELEASE_DIR / f"v{VERSION}-binaries.sha256").read_text(encoding="utf-8"))
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()
        if len(names) != len(set(names)):
            raise ValueError("Duplicate ZIP paths")
        for name in names:
            part = PurePosixPath(name)
            if part.is_absolute() or ".." in part.parts or "\\" in name:
                raise ValueError("Unsafe ZIP path")
        manifest = hashes(archive.read("docs/SHA256.txt").decode("utf-8"))
        if set(manifest) != set(names) - {"docs/SHA256.txt"}:
            raise ValueError("Package manifest does not cover the complete archive")
        for name, expected in manifest.items():
            if hashlib.sha256(archive.read(name)).hexdigest() != expected:
                raise ValueError(f"Package checksum mismatch: {name}")
        executables = {name for name in names if name.lower().endswith(".exe")}
        if executables != set(approved):
            raise ValueError("Unexpected executable inventory")
        for name, expected in approved.items():
            if manifest[name] != expected:
                raise ValueError(f"Executable differs from the live-tested package: {name}")
        if archive.read("source/version.go") != (ROOT / "source/version.go").read_bytes():
            raise ValueError("Package version source mismatch")
    print(f"Verified full ZIP, internal manifest and all four approved executables: {digest}", flush=True)
    return digest


def gh(*args: str) -> str:
    return subprocess.check_output(["gh", *args], text=True)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--verify-only", type=Path)
    parser.add_argument("--run-id")
    parser.add_argument("--commit")
    parser.add_argument("--artifact-dir", type=Path)
    args = parser.parse_args()
    if args.verify_only:
        verify_package(args.verify_only)
        return
    if not args.run_id or not args.run_id.isdecimal() or not args.commit or not re.fullmatch(r"[0-9a-f]{40}", args.commit) or args.artifact_dir is None:
        parser.error("Publishing requires a CI run ID, full commit SHA and artifact directory")
    repo = os.environ["GH_REPO"]
    if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", repo):
        raise ValueError("Invalid repository name")
    tag = f"v{VERSION}"
    notes = RELEASE_DIR / f"{tag}.md"
    if not notes.is_file():
        raise ValueError("No release notes for this version")
    # Only default-branch push runs from this repository are accepted; the
    # workflow enforces the same boundary before granting its write token.
    run = json.loads(gh("api", f"repos/{repo}/actions/runs/{args.run_id}"))
    if run["head_sha"] != args.commit or run["conclusion"] != "success" or run["event"] != "push" or run["head_branch"] != "main" or run["head_repository"]["full_name"] != repo or run["path"] != ".github/workflows/source-checks.yml":
        raise ValueError("Release must come from successful trusted Source checks")
    existing = json.loads(gh("api", f"repos/{repo}/releases?per_page=100"))
    for release in existing:
        if release["tag_name"] == tag:
            print(f"Release already exists; existing assets preserved: {release['html_url']}")
            return
    args.artifact_dir.mkdir(parents=True, exist_ok=False)
    gh("run", "download", args.run_id, "--repo", repo, "--name", f"OpenOMSI-BCS-Bridge-v{VERSION}", "--dir", str(args.artifact_dir))
    archive = args.artifact_dir / PACKAGE
    digest = verify_package(archive)
    checksum = archive.with_name(archive.name + ".sha256")
    # Upload as a draft, check both assets, then make the prerelease available.
    gh("release", "create", tag, str(archive), str(checksum), "--repo", repo,
       "--target", args.commit, "--title", f"openOMSI BBS Bridge {tag} (experimental)",
       "--notes-file", str(notes), "--prerelease", "--draft")
    releases = json.loads(gh("api", f"repos/{repo}/releases?per_page=100"))
    release = next(item for item in releases if item["tag_name"] == tag)
    assets = {item["name"]: item for item in release["assets"]}
    expected = {archive.name: archive, checksum.name: checksum}
    if set(assets) != set(expected) or not release["draft"] or not release["prerelease"] or release["target_commitish"] != args.commit:
        raise ValueError("Draft release metadata or asset inventory mismatch")
    for name, path in expected.items():
        if assets[name]["size"] != path.stat().st_size:
            raise ValueError(f"Uploaded asset size mismatch: {name}")
        uploaded_digest = assets[name].get("digest")
        if uploaded_digest and uploaded_digest != "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest():
            raise ValueError(f"Uploaded asset checksum mismatch: {name}")
    gh("release", "edit", tag, "--repo", repo, "--draft=false")
    print(f"Published {release['html_url']}\nZIP SHA-256: {digest}")


if __name__ == "__main__":
    main()
