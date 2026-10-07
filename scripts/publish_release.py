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
PACKAGE = f"OpenOmsi.+.BBS.{VERSION}.zip"
PACKAGE_LABEL = f"OpenOmsi + BBS {VERSION}.zip"
LEGACY_PACKAGE = f"OpenOMSI_BCS_Bridge_v{VERSION}_by_Mayconrib808.zip"
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


def finish_asset_rename(repo: str, release: dict, directory: Path) -> None:
    """Refresh renamed v1.1.3 metadata without replacing its tested ZIP."""
    assets = {item["name"]: item for item in release["assets"]}
    old_checksum = assets.get(LEGACY_PACKAGE + ".sha256")
    if old_checksum is None:
        return
    published = RELEASE_DIR / f"v{VERSION}-package.sha256"
    approved = hashes(published.read_text(encoding="utf-8"))
    archive = assets.get(PACKAGE) or assets.get(LEGACY_PACKAGE)
    if archive is None or archive["state"] != "uploaded" or set(approved) != {PACKAGE}:
        raise ValueError("Cannot identify the published ZIP for renaming")
    digest = approved[PACKAGE]
    old_digests = {"sha256:" + hashlib.sha256(f"{digest}  {LEGACY_PACKAGE}{newline}".encode()).hexdigest() for newline in ("\n", "\r\n")}
    if archive.get("digest") != "sha256:" + digest or old_checksum.get("digest") not in old_digests:
        raise ValueError("Published ZIP or legacy checksum differs from the recorded release")
    directory.mkdir(parents=True, exist_ok=True)
    checksum = directory / (PACKAGE + ".sha256")
    checksum.write_text(f"{digest}  {PACKAGE}\n", encoding="utf-8")
    checksum_digest = "sha256:" + hashlib.sha256(checksum.read_bytes()).hexdigest()
    current = assets.get(checksum.name)
    if current is None:
        gh("release", "upload", release["tag_name"], str(checksum) + "#" + PACKAGE_LABEL + ".sha256", "--repo", repo)
    elif current.get("digest") != checksum_digest:
        raise ValueError("A different checksum already exists under the new name")
    updated = json.loads(gh("api", f"repos/{repo}/releases/{release['id']}"))
    uploaded = next(item for item in updated["assets"] if item["name"] == checksum.name)
    if uploaded.get("digest") != checksum_digest or uploaded["size"] != checksum.stat().st_size or uploaded["state"] != "uploaded":
        raise ValueError("Renamed checksum upload mismatch")
    gh("api", "--method", "PATCH", f"repos/{repo}/releases/assets/{archive['id']}",
       "-f", "name=" + PACKAGE, "-f", "label=" + PACKAGE_LABEL)
    # Only remove the obsolete, fully reproducible checksum after its checked
    # replacement is available. The ZIP asset and its bytes remain intact.
    gh("api", "--method", "DELETE", f"repos/{repo}/releases/assets/{old_checksum['id']}")
    body = release["body"].replace(LEGACY_PACKAGE, PACKAGE_LABEL)
    if body != release["body"]:
        gh("api", "--method", "PATCH", f"repos/{repo}/releases/{release['id']}", "-f", "body=" + body)
    print(f"Renamed release metadata; original ZIP SHA-256 preserved: {digest}", flush=True)


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
    if "-" in VERSION:
        print(f"Development build {VERSION}: retained as a CI artifact; no release or tag published.")
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
            finish_asset_rename(repo, release, args.artifact_dir)
            print(f"Release already exists; existing assets preserved: {release['html_url']}")
            return
    args.artifact_dir.mkdir(parents=True, exist_ok=False)
    gh("run", "download", args.run_id, "--repo", repo, "--name", f"OpenOMSI-BCS-Bridge-v{VERSION}", "--dir", str(args.artifact_dir))
    archive = args.artifact_dir / PACKAGE
    digest = verify_package(archive)
    checksum = archive.with_name(archive.name + ".sha256")
    # Upload as a draft, check both assets, then make the prerelease available.
    gh("release", "create", tag, str(archive) + "#" + PACKAGE_LABEL,
       str(checksum) + "#" + PACKAGE_LABEL + ".sha256", "--repo", repo,
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
