#!/usr/bin/env python3
"""Collect verified artist-made winter outfits into a non-released OpenOMSI asset overlay.

Do not upload to official openOMSI, or sign an updater release before actual
gameplay tests. Every mesh preserves its original UVs and credited artist.
"""
from __future__ import annotations
from pathlib import Path
import argparse
import hashlib
import json
import shutil
import sys
import zlib

PACKS = ["shirts01", "shirts02", "gloves01", "hats03"]
MAX_TEXTURE = 2048

def clean_bad_png_ancillary(path: Path) -> list[str]:
    """Repair broken ancillary metadata only; never alter critical image/IDAT bytes."""
    data=path.read_bytes()
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise ValueError(f"Not a PNG: {path}")
    off=8
    out=bytearray(data[:8])
    removed=[]
    complete=False
    while off < len(data):
        if off+12 > len(data): raise ValueError(f"Truncated PNG header: {path}")
        size=int.from_bytes(data[off:off+4],"big")
        if size>64_000_000 or off+12+size>len(data):raise ValueError(f"Invalid PNG chunk: {path}")
        typ=data[off+4:off+8]
        raw=data[off:off+12+size]
        expect=int.from_bytes(raw[-4:],"big")
        actual=zlib.crc32(raw[4:-4]) & 0xFFFFFFFF
        if expect != actual:
            if typ[0] & 0x20:
                removed.append(typ.decode("ascii","replace"))
                off+=12+size
                continue
            raise ValueError(f"Critical PNG chunk {typ!r} corrupted: {path}")
        out.extend(raw)
        off+=12+size
        if typ==b"IEND":
            complete=True
            break
    if not complete:raise ValueError(f"PNG IEND missing: {path}")
    if removed:
        path.write_bytes(out)
    return removed

def file_sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()

def assemble(staged: Path, out: Path) -> dict:
    from PIL import Image
    dest = out / "bcs-assets" / "WinterClothes"
    dest.mkdir(parents=True, exist_ok=True)
    selection = []
    creators = []
    for pack in PACKS:
        source = staged / pack
        report_file = source / "conversion_report.json"
        if not report_file.is_file():
            raise RuntimeError(f"Missing completed artist retargeting: {pack}")
        report = json.loads(report_file.read_text(encoding="utf-8"))
        if report.get("release_approved", True):
            raise RuntimeError(f"Unexpected release-approved source flag in {pack}")
        for piece in report["entries"]:
            key = piece["name"]
            if not key or "/" in key or "\\" in key or ".." in key:
                raise ValueError("Unsafe garment name")
            part_src = source / key
            part = dest / key
            if part.exists(): raise RuntimeError(f"Duplicate garment {key}")
            shutil.copytree(part_src, part)
            binding_file = part / "skin_binding.json"
            binding = json.loads(binding_file.read_text(encoding="utf-8"))
            if binding["garment"] != key or binding["fitted_to_stock_omsi_human"] or binding["released"]:
                raise RuntimeError(f"Invalid/bogus fit status {key}")
            if not (part / "garment.obj").is_file():
                raise RuntimeError(f"Missing artist 3D geometry {key}")
            if len(binding["per_vertex_weights"]) != piece["vertex_count"]:
                raise RuntimeError(f"Invalid bone weights {key}")
            # Resize textures without touching original UVs; conservative filter avoids
            # reauthoring/low-poly model art and reduces VRAM for crowded bus stops.
            for image in part.glob("*.png"):
                removed=clean_bad_png_ancillary(image)
                if removed:print(f"Restored original pixels by removing invalid PNG metadata {removed}: {image}")
                with Image.open(image) as im:
                    im.load()
                    if max(im.size) > MAX_TEXTURE:
                        im.thumbnail((MAX_TEXTURE, MAX_TEXTURE), Image.Resampling.LANCZOS)
                        im.save(image, optimize=True)
                    size = im.size
                if not all(0 < n <= MAX_TEXTURE for n in size):
                    raise RuntimeError(f"Invalid texture dimensions in {image}")
            original = piece["artist"]
            license_id = piece["license"]
            creators.append(f"{key}: {original}, {license_id}; adapted axes/weights, original artist mesh and UVs")
            selection.append({
                "name": key,
                "artist": original,
                "license": license_id,
                "triangles": sum(1 for s in (part/"garment.obj").read_text().splitlines() if s.startswith("f ")) * 2,
                "textures": sorted(x.name for x in part.glob("*.png")),
                "mesh_sha256": file_sha(part/"garment.obj"),
                "binding_sha256": file_sha(binding_file),
                "status": "candidate_only_requires_in_game_fit_validation",
            })
    if len(selection) != 7:
        raise RuntimeError(f"Expected seven artist garments, got {len(selection)}")
    (dest / "AUTHORS_AND_LICENSES.txt").write_text(
        "MakeHuman Community artist-made winter clothing overlay for OpenOMSI Bridge.\n"
        "Each item retains its original licensed authorship and geometry.\n"
        "Source: https://www.makehumancommunity.org/assets/assetpacks/\n"
        "CC0: https://creativecommons.org/publicdomain/zero/1.0/\n"
        "CC BY 4.0: https://creativecommons.org/licenses/by/4.0/\n\n"
        + "\n".join(creators) + "\n", encoding="utf-8"
    )
    manifest = {
        "schema": 1,
        "version": "STAGING_ONLY",
        "target": "OpenOMSI BCS Bridge Windows app and modified server",
        "release_approved": False,
        "gameplay_visual_tested": False,
        "rigged_on_stock_passengers": False,
        "garments": selection,
        "instructions": "Retargeted references and approximate weights still need actual animation, seating and FPS verification before signed Setup update",
    }
    (out / "winter-candidate-manifest.json").write_text(json.dumps(manifest, indent=2,ensure_ascii=False)+"\n",encoding="utf-8")
    print(json.dumps({"garments":len(selection),"total_asset_mb":round(sum(x.stat().st_size for x in dest.rglob("*") if x.is_file())/1e6,2)},indent=2))
    return manifest

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument("--source", type=Path,required=True)
    ap.add_argument("--output", type=Path,required=True)
    args=ap.parse_args()
    assemble(args.source,args.output)

if __name__=="__main__":main()
