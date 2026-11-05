#!/usr/bin/env python3
"""Reconcile exact v0.3.49 additions after the 0.3.47-based Bridge source merge.

Run ONLY after merging official 0784e3cd into the Bridge tree. Fails closed
when an expected local conflict shape changes. This is a source candidate,
not authorization to deploy the BCS updater without all release gates.
"""
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]

def replace_exact(path: str, old: str, new: str, expected: int = 1) -> None:
    target=ROOT/path
    source=target.read_text(encoding="utf-8")
    found=source.count(old)
    if found==0 and source.count(new)==expected:
        print(f'already integrated {path}: {expected} expected reconciliation(s)', flush=True)
        return
    if found!=expected:
        raise RuntimeError(f"{path}: expected {expected} occurrences, got {found} of {old!r}")
    target.write_text(source.replace(old,new),encoding="utf-8")
    print(f"merged {path}: {expected} precise reconciliation(s)", flush=True)

# Official P17 wire identity must never be overwritten by the Bridge P16 hunk.
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "pub const PROTOCOL: u32 = 16;",
    "pub const PROTOCOL: u32 = 17;",
)

# The v0.3.49 Traffic/Random module supersedes horn's local pseudo-random helper.
replace_exact(
    "crates/omsi-sim/src/ai_traffic/illegal_parking.rs",
    "use super::horn::draw;",
    "use super::random::draw;",
)

# Restore the 0.3.49 per-light AI traffic decision state while preserving
# the Bridge's bus-lane passing and incident vehicle fields.
replace_exact(
    "crates/omsi-sim/src/ai_traffic/model.rs",
    "    pub passing: Option<Passing>,\n",
    "    pub passing: Option<Passing>,\n"
    "    /// 0.3.49: current traffic light decision, including amber/red.\n"
    "    pub current_light: Option<(usize, usize)>,\n"
    "    pub amber_decision: f32,\n",
)
replace_exact(
    "crates/omsi-sim/src/ai_traffic/model.rs",
    "            passing: None,\n",
    "            passing: None,\n"
    "            current_light: None,\n"
    "            amber_decision: 0.0,\n",
)

# Restore official short-trip behavior without replacing Bridge's accessible
# passenger path, transfers, boarding and wheelchair code.
replace_exact(
    "crates/omsi-sim/src/people/pax_task.rs",
    "use super::pax_stops::entry_on_platform;\n",
    "use super::pax_stops::entry_on_platform;\n\n"
    "const SHORT_RIDE_KM: f32 = 3.0;\n"
    "pub(super) fn short_ride(dest: Option<&str>, ride_km: f32) -> bool {\n"
    "    dest.is_none() && ride_km > 0.0 && ride_km < SHORT_RIDE_KM\n"
    "}\n",
)

# P17 adds the SoundAcks category to an existing BCS-specific per-traffic
# rate-limit counter. Its last index is 12, so both storage and diagnostics
# must expand without clobbering the 12 earlier categories.
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "    drop_counts: [u32; 12],",
    "    drop_counts: [u32; 13],",
)
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "            drop_counts: [0; 12],",
    "            drop_counts: [0; 13],",
)
replace_exact(
    "crates/omsi-net/src/lib.rs",
    "                p.drop_counts[11]\n",
    "                p.drop_counts[11],\n"
    "                p.drop_counts[12]\n",
)

# New upstream population tests must initialize the Bridge's network seat field.
replace_exact(
    "crates/omsi-sim/src/people/population_limit_tests.rs",
    "        remote: false,\n    };",
    "        remote: false,\n        remote_seat: None,\n    };",
    expected=2,
)

print("P17 reconciliation complete. Compile/test before considering release.",flush=True)


# ---- v0.3.49 UI / settings / traffic reconciliation (omsi-app build) ----
# -X ours can retain Bridge declarations while bringing in upstream call sites.
# Patch only the specific missing half of each conflict, keeping both features.

def ensure_decl(path: str, trigger: str, anchor: str, declaration: str) -> None:
    source=(ROOT/path).read_text(encoding="utf-8")
    if trigger not in source:
        raise RuntimeError(f"{path}: expected v0.3.49 trigger {trigger!r} missing")
    if declaration in source:
        return
    replace_exact(path, anchor, anchor + "\n" + declaration)

# The upstream Radio panel and its event handlers need the module declaration;
# retain the Bridge's wheelchair/coop panel declarations and input handlers.
ensure_decl(
    "crates/omsi-app/src/game_panels/mod.rs",
    'radio_panel::click(self, el)',
    "mod move_here;",
    "pub(crate) mod radio_panel;",
)
panels="crates/omsi-app/src/game_panels/mod.rs"
panel_text=(ROOT/panels).read_text(encoding="utf-8")
if "|| clean;" in panel_text and "let clean = self.clean_screen_shown();" not in panel_text:
    replace_exact(
        panels,
        "        let (Some(r), Some(scene)) = (self.renderer.as_ref(), self.scene.as_mut()) else { return };",
        "        let clean = self.clean_screen_shown();\n"
        "        let (Some(r), Some(scene)) = (self.renderer.as_ref(), self.scene.as_mut()) else { return };",
    )

# The merge contains both the Bridge coop/wheelchair tiles and the new Radio
# tile: compute the actual array size rather than discarding a feature.
quick="crates/omsi-app/src/game_panels/quick_menu.rs"
quick_source=(ROOT/quick).read_text(encoding="utf-8")
import re
tiles=re.search(
    r'(pub\(crate\) const TILES: \[\(&str, &str, &str\); )(\d+)(\] = \[\n)(.*?)(\n\];)',
    quick_source, re.S,
)
if tiles is None:
    raise RuntimeError("quick menu TILES layout changed")
tile_body=tiles.group(4)
# Upstream adds Radio, but "-X ours" keeps Bridge's wheelchair/coop
# layout in the conflicting hunk. Install the missing tile explicitly.
if not all(f'("{name}",' in tile_body for name in ("coop", "wheelchair")):
    raise RuntimeError("0.3.49 reconciliation lost a Bridge accessibility/coop tile")
if '("radio",' not in tile_body:
    if quick_source.count('"radio" => super::radio_panel::open(app),')!=1:
        raise RuntimeError("0.3.49 Radio panel handler missing; refusing empty tile")
    anchor='    ("coop", "Cooperative", "directions_bus"),'
    if tile_body.count(anchor)!=1:
        raise RuntimeError("quick menu coop tile anchor not unique")
    tile_body=tile_body.replace(anchor,anchor+'\n    ("radio", "Radio", "radio"),')
if not all(f'("{name}",' in tile_body for name in ("coop", "wheelchair", "radio")):
    raise RuntimeError("0.3.49 reconciliation must retain coop, wheelchair and radio tiles")
tile_count=len(re.findall(r'^\s*\("',tile_body,re.M))
# Replace the entire entry block, preserving the neighboring menu sections.
updated=(quick_source[:tiles.start(2)] + str(tile_count) +
         quick_source[tiles.end(2):tiles.start(4)] + tile_body +
         quick_source[tiles.end(4):])
if updated!=quick_source:
    (ROOT/quick).write_text(updated,encoding="utf-8")
    print(f"quick menu reconciled with {tile_count} tiles; Radio, coop and wheelchair retained",flush=True)

# New find-street menu entry referenced by the upstream menu handlers.
lists="crates/omsi-app/src/game_lists.rs"
list_source=(ROOT/lists).read_text(encoding="utf-8")
enum_begin=list_source.index("pub(crate) enum ListKind {")
enum_end=list_source.index("\n}",enum_begin)
if "ListKind::Streets" not in list_source:
    raise RuntimeError("0.3.49 find-street handlers missing")
if "    Streets," not in list_source[enum_begin:enum_end]:
    replace_exact(lists,"    Liveries,\n","    Liveries,\n    Streets,\n")

# Upstream traffic table now sends host-owned depot/number/registration metadata.
# Keep the Bridge's existing car_display helper available too.
lan="crates/omsi-app/src/lan_world.rs"
lan_source=(ROOT/lan).read_bytes()
old=b"pub(crate) use display::car_display;"
new=b"pub(crate) use display::{car_display, car_meta};"
if old in lan_source:
    if lan_source.count(old)!=1:
        raise RuntimeError("lan_world display export repeated")
    (ROOT/lan).write_bytes(lan_source.replace(old,new))
elif new not in lan_source:
    raise RuntimeError("lan_world display exports changed")

# The 0.3.49 app mirrors AI vehicles with the new amber-light decision state;
# this state was added to the simulation model above.
mirror="crates/omsi-app/src/traffic/mirror.rs"
mirror_text=(ROOT/mirror).read_text(encoding="utf-8")
if "current_light: None," not in mirror_text:
    replace_exact(
        mirror,
        "            passing: None,\n",
        "            passing: None,\n            current_light: None,\n            amber_decision: 0.0,\n",
    )
elif "amber_decision: 0.0," not in mirror_text:
    raise RuntimeError("AI traffic mirror has partial amber-decision state")

# A surviving Bridge anti-starvation call site needs the matching TrafficSim
# member and constructor initialization. Do not delete population recovery.
traffic="crates/omsi-sim/src/ai_traffic.rs"
traffic_source=(ROOT/traffic).read_text(encoding="utf-8")
if "pub starved_since: Option<f32>," not in traffic_source:
    replace_exact(
        traffic,
        "    pub spawn_radius: f64,\n",
        "    pub spawn_radius: f64,\n"
        "    /// When local AI has remained below its requested population.\n"
        "    pub starved_since: Option<f32>,\n",
    )
setup="crates/omsi-sim/src/ai_traffic/setup.rs"
setup_source=(ROOT/setup).read_text(encoding="utf-8")
if "            starved_since: None," not in setup_source:
    replace_exact(setup, "            spawn_radius: 400.0,\n",
                  "            spawn_radius: 400.0,\n            starved_since: None,\n")

# Merge the settings ABI and its default initializer without resetting the
# Bridge's enabled hailing, wheelchair, transfer and passenger settings.
settings="crates/omsi-app/src/settings.rs"
settings_text=(ROOT/settings).read_text(encoding="utf-8")
struct_start=settings_text.index("pub struct Settings {")
struct_end=settings_text.index("\n}",struct_start)
prefix=settings_text[:struct_start]
struct=settings_text[struct_start:struct_end]
suffix=settings_text[struct_end:]
required=[
    ("pax_short_rides_stand","bool","    pub passenger_full_bus_reaction: bool,","false"),
    ("zoom_hold_amount","f32","    pub alt_view: bool,","0.5"),
    ("zoom_hold_latch","bool","    pub alt_view: bool,","false"),
    ("flashlight_cursor","bool","    pub alt_view: bool,","false"),
    ("nav_ai_cars","bool","    pub nav_ai: bool,","true"),
    ("notes_driving","bool","    pub notes: bool,","true"),
    ("notes_script","bool","    pub notes: bool,","true"),
    ("notes_trip","bool","    pub notes: bool,","true"),
    ("notes_tickets","bool","    pub notes: bool,","true"),
]
for name,kind,anchor,default in required:
    if re.search(r"\bpub\s+"+re.escape(name)+r"\s*:",struct):
        continue
    if struct.count(anchor)!=1:
        raise RuntimeError(f"settings: cannot safely add {name} at {anchor!r}")
    struct=struct.replace(anchor,anchor+f"\n    pub {name}: {kind},")
settings_text=prefix+struct+suffix
desktop=settings_text.index("    fn desktop() -> Self {")
line_start=settings_text.index("        Self {",desktop)
line_end=settings_text.index("\n",line_start)
initializer=settings_text[line_start:line_end]
if initializer.count("info_bar: false,")!=1:
    raise RuntimeError("settings desktop initializer changed")
for name,kind,anchor,default in required:
    if not re.search(r"\b"+re.escape(name)+r"\s*:",initializer):
        initializer=initializer.replace("info_bar: false,",f"{name}: {default}, info_bar: false,")
settings_text=settings_text[:line_start]+initializer+settings_text[line_end:]
(ROOT/settings).write_text(settings_text,encoding="utf-8")
print("0.3.49 UI, AI and settings reconciliation complete (Bridge features retained)",flush=True)
