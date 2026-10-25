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
# Coop13 adds a CPU-side picking cache to VehicleMesh. The upstream 0.3.41
# trolley-pole test fixture must initialize it too, otherwise cargo test fails
# even though the release executable itself compiles.
once(mesh,
     "    fn mesh(def_index: usize) -> VehicleMesh {\n        VehicleMesh {\n            def_index,",
     "    fn mesh(def_index: usize) -> VehicleMesh {\n        VehicleMesh {\n            pick_geometry: Default::default(),\n            def_index,")
# Native Coop tests instantiate vehicles without the newer 0.3.41 LOD cache.
# Two regression fixtures, with their existing layouts kept exactly intact.
once(root/"crates/omsi-app/src/coop_drive.rs",
     "program: Arc::new(program), meshes: vec![], paint_schemes: vec![],",
     "program: Arc::new(program), meshes: vec![], lod_meshes: vec![], paint_schemes: vec![],")
once(root/"crates/omsi-app/src/coop_view.rs",
     "            meshes: Vec::new(),\n",
     "            meshes: Vec::new(),\n            lod_meshes: Vec::new(),\n")
# 0.3.41 adds far-distance rendering instances.
render=root/"crates/omsi-app/src/spawn.rs"
once(render,
     "        window_wipers: None, instances: vec![], own_materials: vec![], set: None,",
     "        window_wipers: None, instances: vec![], far: vec![], own_materials: vec![], set: None,")
print("Coop14: v0.3.41 aerodynamics, LOD mesh picking, test helpers and far render integrated.")

# The existing full-state handover test uses a pre-0.3.41 vehicle fixture.
once(root/"crates/omsi-lan/src/vehicle_transfer.rs",
     "            meshes: vec![],\n            paint_schemes: vec![],",
     "            meshes: vec![],\n            lod_meshes: vec![],\n            paint_schemes: vec![],")
