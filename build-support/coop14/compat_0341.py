"""Compatibility of cumulative Coop simulation with the *genuine* v0.3.41 changes.

Upstream added road aerodynamics and separate AI LOD meshes. The Coop's network
state serializers and on-demand remote-control picker must coexist with them.
This is a strictly scoped port, not a version-label rewrite.
"""
from pathlib import Path

def once(path: Path, before: str, after: str):
    source=path.read_text(encoding="utf-8")
    if source.count(before)!=1:
        raise RuntimeError(f"Port conflict changed unexpectedly: {path} ({source.count(before)} matches)")
    path.write_text(source.replace(before,after),encoding="utf-8",newline="\n")

root=Path("game")
aero=root/"crates/omsi-sim/src/rigid/realism.rs"
once(aero,
     "#[derive(Debug, Clone, Copy, PartialEq)]\npub struct Aero {",
     "#[derive(Debug, Clone, Copy, PartialEq, serde::Serialize, serde::Deserialize)]\npub struct Aero {")
mesh=root/"crates/omsi-sim/src/vehicle/types.rs"
once(mesh,
     "                lod_meshes.push(VehicleMesh {\n                    def_index: first + i,",
     "                lod_meshes.push(VehicleMesh {\n                    pick_geometry: Default::default(),\n                    def_index: first + i,")
print("Coop14: v0.3.41 aerodynamics serialization and LOD mesh picking integrated.")
