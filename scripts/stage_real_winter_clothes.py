#!/usr/bin/env python3
"""Stage artist-made MakeHuman cold-weather meshes, NEVER generate replacement geometry.

This program only downloads/inspects authorized source packs and makes a LICENSE-aware
selection artifact. Fitting to OMSI human rigs and shipping the assets are separate,
explicit approval/test gates. Run on CI with outbound HTTPS.
"""
from __future__ import annotations

import argparse
import hashlib
import io
import json
from pathlib import Path, PurePosixPath
import re
import urllib.request
import zipfile

PACKS = {
    "shirts01": {
        "file": "shirts01_cc0.zip",
        "assets": {"toigo_fisherman_sweater": ("MargaretToigo", "CC0-1.0", "sweater")},
    },
    "shirts02": {
        "file": "shirts02_cc-by.zip",
        "assets": {
            "elvs_hooded_sweat_jacket1": ("Elvaerwyn", "CC-BY-4.0", "hooded jacket"),
            "mindfront_knitted_sweater_01": ("Mindfront", "CC-BY-4.0", "knitted sweater"),
        },
    },
    "gloves01": {
        "file": "gloves01_cc0.zip",
        "assets": {
            "toigo_gloves_short": ("MargaretToigo", "CC0-1.0", "short gloves"),
            "toigo_gloves_medium": ("MargaretToigo", "CC0-1.0", "medium gloves"),
        },
    },
    "hats03": {
        "file": "hats03_cc-by.zip",
        "assets": {
            "mindfront_knitted_hat_01": ("Mindfront", "CC-BY-4.0", "knitted cap"),
            "elvs_slouchy_beanie1": ("Elvaerwyn", "CC-BY-4.0", "beanie"),
        },
    },
}
BASES = (
    "https://files2.makehumancommunity.org/asset_packs",
    "https://files.makehumancommunity.org/asset_packs",
)
PAGE_BASE = "https://static.makehumancommunity.org/assets/assetpacks"
MESH_EXT = {".obj", ".mhclo", ".mtl", ".mhmat"}
TEXTURE_EXT = {".png", ".jpg", ".jpeg", ".tga", ".bmp", ".webp"}
TOKEN_RE = re.compile(r'[^\s"\']+\.(?:png|jpg|jpeg|tga|bmp|webp|mtl)', re.I)


def download(pack: str) -> bytes:
    filename = PACKS[pack]["file"]
    errors = []
    for base in BASES:
        uri = f"{base}/{pack}/{filename}"
        try:
            req = urllib.request.Request(uri, headers={"User-Agent": "OpenOMSI-BCS-Bridge-AssetStager/1.0"})
            with urllib.request.urlopen(req, timeout=160) as res:
                if res.status != 200:
                    raise RuntimeError(f"HTTP {res.status}")
                data = res.read(300_000_001)
            if len(data) > 300_000_000 or not zipfile.is_zipfile(io.BytesIO(data)):
                raise ValueError("Source is not a valid ZIP within 300 MB")
            print(f"Downloaded {pack}: {len(data):,} bytes from {uri}")
            return data
        except Exception as exc:
            errors.append(f"{uri}: {exc}")
    raise RuntimeError("All verified project mirrors failed:\n" + "\n".join(errors))


def valid_name(path: str) -> bool:
    parts = PurePosixPath(path.replace("\\", "/")).parts
    return not (not parts or ".." in parts or parts[0] in {".", "/"}) and not any(p.startswith(":") for p in parts)


def asset_matches(entry: str, name: str) -> bool:
    parts = PurePosixPath(entry.lower()).parts
    needle = name.lower()
    return needle in parts or any(
        p.startswith(needle + ".") or p.startswith(needle + "_")
        for p in parts[-1:]
    )


def object_stats(data: bytes) -> dict:
    counts = {"vertices": 0, "texcoords": 0, "faces": 0, "triangles_est": 0}
    for line in data.splitlines():
        if line.startswith(b"v "):
            counts["vertices"] += 1
        elif line.startswith(b"vt "):
            counts["texcoords"] += 1
        elif line.startswith(b"f "):
            counts["faces"] += 1
            counts["triangles_est"] += max(0, len(line.split()) - 3)
    return counts


def stage(pack: str, target: Path) -> dict:
    data = download(pack)
    spec = PACKS[pack]
    with zipfile.ZipFile(io.BytesIO(data)) as src:
        candidates = {info.filename: info for info in src.infolist()
                      if not info.is_dir() and valid_name(info.filename)
                      and info.file_size <= 64_000_000}
        selected: dict[str, set[str]] = {}
        for name in spec["assets"]:
            hits = {f for f in candidates if asset_matches(f, name)}
            if not hits or not any(Path(f).suffix.lower() == ".obj" for f in hits):
                raise ValueError(f"Missing real authored OBJ for {name}. Matches: {sorted(hits)[:10]}")
            selected[name] = hits

        # MakeHuman materials may refer to external texture names without the model prefix.
        # Keep only the dependency filenames actually referenced by selected authored files.
        needed = set().union(*selected.values())
        for f in sorted(list(needed)):
            if Path(f).suffix.lower() not in MESH_EXT:
                continue
            body = src.read(f).decode("utf-8", "replace")
            for token in TOKEN_RE.findall(body):
                filename = PurePosixPath(token.replace("\\", "/")).name.lower()
                matches = [key for key in candidates if PurePosixPath(key).name.lower() == filename]
                # When identical texture basenames occur, prefer the mesh's directory.
                current_dir = PurePosixPath(f).parent
                matches.sort(key=lambda key: (PurePosixPath(key).parent != current_dir, len(key)))
                if matches:
                    needed.add(matches[0])
        target.mkdir(parents=True, exist_ok=True)
        detail = {}
        for name, (author, license_id, item_type) in spec["assets"].items():
            original = selected[name]
            obj_paths = [p for p in original if Path(p).suffix.lower() == ".obj"]
            stats = [object_stats(src.read(p)) for p in obj_paths]
            if not any(x["vertices"] > 0 and x["faces"] > 0 for x in stats):
                raise ValueError(f"{name}: no triangle mesh found")
            detail[name] = {
                "creator": author,
                "license": license_id,
                "description": item_type,
                "geometry": stats,
                "original_files": sorted(original),
                "source": f"{PAGE_BASE}/{pack}.html",
                "status": "SOURCE_ONLY_NOT_FITTED_TO_OMSI",
            }
        staged_files = []
        for path in sorted(needed):
            payload = src.read(path)
            rel = Path(*PurePosixPath(path.replace("\\", "/")).parts)
            out = target / "raw" / rel
            out.parent.mkdir(parents=True, exist_ok=True)
            out.write_bytes(payload)
            staged_files.append({
                "path": str(Path("raw") / rel).replace("\\", "/"),
                "sha256": hashlib.sha256(payload).hexdigest(),
                "bytes": len(payload),
            })
    manifest = {
        "pack": pack,
        "pack_source_sha256": hashlib.sha256(data).hexdigest(),
        "source_page": f"{PAGE_BASE}/{pack}.html",
        "authors_and_licenses": detail,
        "staged_files": staged_files,
        "distributor_notice": "Art is third-party licensed. Credit CC-BY authors in bundled notices; do not imply OpenOMSI or OMSI compatibility until retargeted and visually tested.",
        "release_approved": False,
    }
    (target / "manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    notice = ["Winter clothing: original artist-made models curated from MakeHuman Community.",
              f"Source: {PAGE_BASE}/{pack}.html", ""]
    for name, detail in sorted(manifest["authors_and_licenses"].items()):
        notice.append(f"{name}: {detail['creator']}, {detail['license']}, original work, derived adaptations (if any) must be credited.")
    notice.append("\nLicense links: https://creativecommons.org/licenses/by/4.0/ and https://creativecommons.org/publicdomain/zero/1.0/")
    (target / "CREDITS_AND_LICENSES.txt").write_text("\n".join(notice) + "\n", encoding="utf-8")
    print(json.dumps({name: data["geometry"] for name, data in detail.items()}, indent=2))
    return manifest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--pack", choices=sorted(PACKS), required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    stage(args.pack, args.out)


if __name__ == "__main__":
    main()
