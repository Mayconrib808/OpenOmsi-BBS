#!/usr/bin/env python3
"""Reconcile selected v0.3.50 Rust API conflicts after merging official upstream.

The merge prefers the Bridge for conflicting hunks.  This script applies only
known missing 0.3.50 compatibility changes, preserving the Bridge's wheelchair,
coop, P17 and mouse-event filtering.  Fail closed on unexpected source shapes.
This is not a release or updater publisher.
"""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]

def get(path):
    return (ROOT / path).read_text(encoding="utf-8")

def put(path, text):
    (ROOT / path).write_text(text, encoding="utf-8")

def once(text, before, after, path):
    if before == after:
        return text
    count = text.count(before)
    if count != 1:
        raise RuntimeError(f"{path}: expected one occurrence of {before[:100]!r}, got {count}")
    return text.replace(before, after, 1)

# Upstream added these fields to Settings and its parser, but the Bridge's
# desktop default initializer wins the conflict. Do not reset BCS gameplay
# settings (hailing, wheelchair chance, accessibility) to upstream defaults.
path = "crates/omsi-app/src/settings.rs"
s = get(path)
struct = s.split("pub struct Settings {", 1)[1].split("\n}", 1)[0]
defaults = (
    ("upscale_quality", '"quality".into()'),
    ("remote_cockpit", "false"),
    ("remote_cockpit_port", "8642"),
)
for name, value in defaults:
    if f"pub {name}:" not in struct:
        raise RuntimeError(f"{path}: upstream field {name} missing")
    head = s.index("    fn desktop() -> Self {")
    tail = s.index("\n    }", head)
    body = s[head:tail]
    if f"{name}:" not in body:
        body = once(body, "info_bar: false,", f"{name}: {value}, info_bar: false,", path)
    s = s[:head] + body + s[tail:]
put(path, s)

# Preserve Bridge's named mouse event filtering and 0.3.50's restriction
# to controls visible in the actual interior/exterior rendered viewpoint.
path = "crates/omsi-app/src/player/picking.rs"
s = get(path)
top = (
    "pub(crate) fn pick_in(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach) -> Option<(usize, bool)> {\n"
    "    pick_in_filtered(vehicle,origin,dir,spread,reach,&|_|true)\n"
    "}\n"
)
if "pub(crate) fn pick_in_view(" not in s:
    s = once(s, top, top + """
/// OMSI 0.3.50: only controls visible in this interior/exterior view are pickable.
pub(crate) fn pick_in_view(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach, outside: bool) -> Option<(usize, bool)> {
    pick_in_filtered_view(vehicle, origin, dir, spread, reach, outside, &|_| true)
}
""", path)
signature = "pub(crate) fn pick_in_filtered(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,allowed:&impl Fn(&str)->bool)->Option<(usize,bool)> {"
replacement = signature + """
    pick_in_filtered_view(vehicle, origin, dir, spread, reach, false, allowed)
}
fn pick_in_filtered_view(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,outside:bool,allowed:&impl Fn(&str)->bool)->Option<(usize,bool)> {"""
if "fn pick_in_filtered_view(" not in s:
    s = once(s, signature, replacement, path)
interior_check = "        if !vehicle.ty.model.meshes[vm.def_index].mouse_event.as_deref().is_some_and(allowed) || !vehicle.mesh_props[i].visible {"
interior_new = """        let def = &vehicle.ty.model.meshes[vm.def_index];
        if !def.mouse_event.as_deref().is_some_and(allowed) || !vehicle.mesh_props[i].visible || !bridge_drawn_in_view(def.viewpoint, outside) {"""
if interior_new not in s:
    s = once(s, interior_check, interior_new, path)

top = (
    "pub(crate) fn pick_trailer_in(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach) -> Option<((usize, usize), bool)> {\n"
    "    pick_trailer_in_filtered(vehicle,origin,dir,spread,reach,&|_|true)\n"
    "}\n"
)
if "pub(crate) fn pick_trailer_in_view(" not in s:
    s = once(s, top, top + """
/// The coupled/trailer sections observe the same interior/exterior restriction.
pub(crate) fn pick_trailer_in_view(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach, outside: bool) -> Option<((usize, usize), bool)> {
    pick_trailer_in_filtered_view(vehicle, origin, dir, spread, reach, outside, &|_| true)
}
""", path)
signature = "pub(crate) fn pick_trailer_in_filtered(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,allowed:&impl Fn(&str)->bool)->Option<((usize,usize),bool)> {"
replacement = signature + """
    pick_trailer_in_filtered_view(vehicle, origin, dir, spread, reach, false, allowed)
}
fn pick_trailer_in_filtered_view(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,outside:bool,allowed:&impl Fn(&str)->bool)->Option<((usize,usize),bool)> {"""
if "fn pick_trailer_in_filtered_view(" not in s:
    s = once(s, signature, replacement, path)
trailer_check = "            if !trailer.ty.model.meshes[vm.def_index].mouse_event.as_deref().is_some_and(allowed) || !trailer.mesh_props[i].visible {"
trailer_new = """            let def = &trailer.ty.model.meshes[vm.def_index];
            if !def.mouse_event.as_deref().is_some_and(allowed) || !trailer.mesh_props[i].visible || !bridge_drawn_in_view(def.viewpoint, outside) {"""
if trailer_new not in s:
    s = once(s, trailer_check, trailer_new, path)
if "fn bridge_drawn_in_view(" not in s:
    s += """
/// Upstream 0.3.50 [viewpoint]: 0 is shared, bit 1 outside, bit 2 inside.
fn bridge_drawn_in_view(viewpoint: i32, outside: bool) -> bool {
    viewpoint == 0 || viewpoint & if outside { 1 } else { 2 } != 0
}
"""
put(path, s)

# 0.3.50 passed drawn_outside to both pickers: keep that argument, and route
# through the view-aware wrappers without breaking Bridge's 5-arg test API.
path = "crates/omsi-app/src/player/mouse.rs"
s = get(path)
for old, new in (
    ("pick_in(&self.vehicle, origin, dir, spread, self.reach(origin, dir), self.drawn_outside)?",
     "pick_in_view(&self.vehicle, origin, dir, spread, self.reach(origin, dir), self.drawn_outside)?"),
    ("pick_trailer_in(&self.vehicle, origin, dir, spread, self.reach(origin, dir), self.drawn_outside)?",
     "pick_trailer_in_view(&self.vehicle, origin, dir, spread, self.reach(origin, dir), self.drawn_outside)?"),
):
    if new not in s:
        s = once(s, old, new, path)
put(path, s)

# The upstream plugin cable renderer keeps borrowing scene until later in
# the block. Move the coop view update *after* that borrow ends, but before
# the simulated pose is restored and the hover tooltip is calculated.
path = "crates/omsi-app/src/app_events/frame/drawn.rs"
s = get(path)
if "self.coop_drawn_view();" in s:
    s = once(s, "            self.coop_drawn_view();\n", "", path)
    s = once(s, "        self.player_back_to_sim_pose(sim);",
             "        self.coop_drawn_view();\n        self.player_back_to_sim_pose(sim);", path)
elif "        self.coop_drawn_view();\n        self.player_back_to_sim_pose(sim);" not in s:
    raise RuntimeError(f"{path}: unexpected coop view drawing structure")
put(path, s)

print("Reconciled 0.3.50 Settings, viewpoint-aware mouse picking and coop draw borrow.", flush=True)
