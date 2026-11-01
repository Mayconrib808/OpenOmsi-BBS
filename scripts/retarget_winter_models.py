#!/usr/bin/env python3
"""Prepare real MakeHuman OBJ clothing for stock OpenOMSI human rig fitting.

Converts axes and units and writes a per-vertex semantic bone binding derived from
the real artist mesh. Never substitutes cubes/cylinders or procedurally models clothes.
This is an intermediate staging step, not evidence of correct in-game rigging.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import math
from pathlib import Path

# Source MakeHuman OBJ rest-pose: horizontal X, vertical Y, depth Z in dm-like units.
# OMSI human rest frame: right X, forward Y, up Z in metres. Actual rig fitting is
# performed against each HumanType's [links] before enabling the garment visually.
MAKEHUMAN_TO_OMSI_M = 0.18
SLOTS = ("MAIN", "HEAD", "UPPER_L", "FORE_L", "HAND_L",
         "UPPER_R", "FORE_R", "HAND_R")
MANNEQUIN = {
    "MAIN": (0.0, 5.15, 0.0),
    "HEAD": (0.0, 8.10, 0.0),
    "UPPER_L": (-2.0, 6.2, 0.0),
    "FORE_L": (-3.55, 5.85, 0.0),
    "HAND_L": (-4.75, 5.55, 0.0),
    "UPPER_R": (2.0, 6.2, 0.0),
    "FORE_R": (3.55, 5.85, 0.0),
    "HAND_R": (4.75, 5.55, 0.0),
}
def point(x, y, z):
    return (x*MAKEHUMAN_TO_OMSI_M, -z*MAKEHUMAN_TO_OMSI_M, y*MAKEHUMAN_TO_OMSI_M)

def garment_kind(name: str) -> str:
    n=name.lower()
    if "glove" in n:return "gloves"
    if "hat" in n or "beanie" in n:return "hat"
    return "torso"

def weight_for(v:tuple[float,float,float], kind:str):
    # Original MakeHuman geometry is preserved. Bindings are initial semantic
    # estimates: render must be checked on each target stock rig before release.
    x,y,z=v
    if kind=="hat":return [("HEAD",1.)]
    if kind=="gloves":
        return [(("HAND_L" if x<0 else "HAND_R"),1.)]
    side="L" if x<0 else "R"
    ax=abs(x)
    if ax<1.30:
        return [("MAIN",1.)]
    if ax<2.0:
        w=(ax-1.30)/0.70
        return [("MAIN",1.-w),(f"UPPER_{side}",w)]
    if ax<3.1:
        w=(ax-2.0)/1.1
        return [(f"UPPER_{side}",1.-w),(f"FORE_{side}",w)]
    if ax<4.5:
        w=(ax-3.1)/1.4
        return [(f"FORE_{side}",1.-w),(f"HAND_{side}",w)]
    return [(f"HAND_{side}",1.)]

def convert_obj(source:Path,dest:Path,base_id:str):
    kind=garment_kind(base_id)
    output=[]
    weights=[]
    limits=[[math.inf,-math.inf] for _ in range(3)]
    for raw in source.read_text(encoding="utf-8",errors="replace").splitlines():
        if raw.startswith("v "):
            v=[float(t) for t in raw.split()[1:4]]
            if not all(math.isfinite(t) for t in v):raise ValueError("nonfinite source vertex")
            q=point(*v)
            for i,val in enumerate(q):
                limits[i][0]=min(limits[i][0],val)
                limits[i][1]=max(limits[i][1],val)
            output.append("v "+ " ".join(f"{x:.8f}" for x in q))
            weights.append(weight_for(tuple(v),kind))
        elif raw.startswith("vn "):
            x,y,z=map(float,raw.split()[1:4])
            q=(x,-z,y)
            output.append("vn "+" ".join(f"{i:.7f}" for i in q))
        else:
            output.append(raw)
    if not weights:raise ValueError("empty clothing OBJ")
    if any((hi-lo)>3.0 for lo,hi in limits):raise ValueError("implausible scale in source clothes")
    dest.mkdir(parents=True,exist_ok=True)
    (dest/"garment.obj").write_text("\n".join(output)+"\n",encoding="utf-8")
    binding={
        "schema":"openomsi-winter-fit-v1",
        "source_obj":str(source),
        "out_obj":"garment.obj",
        "source_type":"artist-made-MakeHuman-clothing",
        "garment":base_id,
        "garment_kind":kind,
        "units":"metres",
        "frame":"x_right_y_forward_z_up",
        "default_reference_height_m":1.75,
        "bind_slots":list(SLOTS),
        "reference_anchors_m":{k:list(point(*v)) for k,v in MANNEQUIN.items()},
        "per_vertex_weights":weights,
        "transformed_bounds_m":limits,
        "fitted_to_stock_omsi_human":False,
        "gpu_loaded":False,
        "released":False
    }
    assert len(weights)==sum(raw.startswith("v ") for raw in output)
    assert all(abs(sum(w for _,w in weights_v)-1.)<1e-5 for weights_v in weights)
    (dest/"skin_binding.json").write_text(json.dumps(binding,ensure_ascii=False,separators=(",",":"))+"\n",encoding="utf-8")
    print(f"{base_id}: transformed {len(weights)} artist vertices, source={source.name}, dimensions="+str(limits))
    return binding

def main():
    ap=argparse.ArgumentParser()
    ap.add_argument("--source",type=Path,required=True,help="Unpacked licensed model staging directory")
    ap.add_argument("--output",type=Path,required=True)
    a=ap.parse_args()
    manifests=list(a.source.rglob("manifest.json"))
    if len(manifests)!=1:raise ValueError(f"Expected one license manifest, got {len(manifests)}")
    manifest=json.loads(manifests[0].read_text(encoding="utf8"))
    target=[]
    for name,item in manifest["authors_and_licenses"].items():
        meshes=[p for p in item["original_files"] if p.lower().endswith(".obj")]
        if not meshes:raise ValueError(f"No real artist OBJ: {name}")
        selected=(a.source/"raw"/meshes[0])
        if not selected.is_file():raise FileNotFoundError(selected)
        dest=a.output/name
        binding=convert_obj(selected,dest,name)
        # Keep authentic artist UV maps and surface textures with the adapted geometry.
        # Do not replace fabric with synthetic solid-colour materials.
        source_materials=[]
        for material in selected.parent.iterdir():
            if material.is_file() and material.suffix.lower() in {".png",".jpg",".jpeg",".tga",".bmp",".mhmat",".mtl"}:
                blob=material.read_bytes()
                dest_material=dest/material.name
                dest_material.write_bytes(blob)
                source_materials.append({"file":material.name,"sha256":hashlib.sha256(blob).hexdigest(),
                    "bytes":len(blob)})
        if not any(m["file"].lower().endswith((".png",".jpg",".jpeg",".tga")) for m in source_materials):
            raise ValueError(f"No actual fabric texture for {name}")
        binding["copied_real_textures_and_materials"]=source_materials
        (dest/"skin_binding.json").write_text(json.dumps(binding,ensure_ascii=False,separators=(",",":"))+"\\n",encoding="utf-8")
        (dest/"ATTRIBUTION.txt").write_text(
            f"{name} | Artist: {item['creator']} | License: {item['license']}\n"
            f"Source: {item['source']}\n"
            "This mesh was axis/scale-converted only, not yet fitted to an OMSI stock NPC.\n",
            encoding="utf-8")
        target.append({"name":name,"artist":item["creator"],"license":item["license"],
            "mesh":str(dest/"garment.obj"),"binding":str(dest/"skin_binding.json"),
            "vertex_count":len(binding["per_vertex_weights"]),
            "visual_integration_status":"NEEDS_STOCK_RIG_FIT_TEST"})
    a.output.mkdir(parents=True,exist_ok=True)
    (a.output/"conversion_report.json").write_text(
        json.dumps({"entries":target,"release_approved":False},ensure_ascii=False,indent=2)+"\n",
        encoding="utf-8")

if __name__=="__main__":main()
