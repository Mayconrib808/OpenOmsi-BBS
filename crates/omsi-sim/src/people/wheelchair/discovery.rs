//! Accessibility evidence from the loaded model, live script bindings and geometry.
//! Missing or ambiguous evidence keeps the add-on pending instead of inventing access.
use super::*;
use crate::VehicleType;
use glam::Vec2;
use std::{
    hash::{Hash, Hasher},
    sync::{Mutex, OnceLock},
};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NativeDeck {
    pub mesh: String,
    pub corners: [Vec3; 4],
    #[serde(default)]
    pub thickness: f32,
    pub trigger: Option<String>,
    pub control_var: Option<String>,
    #[serde(skip)]
    pub index: Option<usize>,
}
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FoldSeat {
    pub trigger: String,
    pub variable: String,
    pub folded_value: f32,
}
#[derive(Debug, Clone, Serialize)]
pub struct Detection {
    pub family: String,
    pub native_meshes: Vec<String>,
    pub static_equipment: Vec<String>,
    pub reasons: Vec<String>,
    pub profile: Option<Profile>,
    /// Distinct verified bays, including alternatives served by the same door.
    pub profiles: Vec<Profile>,
}
static CACHE: OnceLock<Mutex<HashMap<u64, Detection>>> = OnceLock::new();
fn equipment_name(name: &str) -> bool {
    let n = name.to_lowercase();
    ["ramp", "rampe", "hublift", "elevador", "pandus"]
        .iter()
        .any(|k| n.contains(k))
        && ![
            "painel",
            "panel",
            "taster",
            "display",
            "multiplex",
            "adesivo",
            "warning",
            "lameiro",
            "mudflap",
            "botao",
            "button",
            "ads_",
        ]
        .iter()
        .any(|k| n.contains(k))
}
fn reserved_name(name: &str) -> bool {
    let n = name.to_lowercase();
    [
        "cadeirante",
        "wheelchair",
        "rollstuhl",
        "retratil",
        "klappsitz",
    ]
    .iter()
    .any(|k| n.contains(k))
        && !["painel", "panel", "adesivo", "luz", "light", "display", "ads_"]
            .iter()
            .any(|k| n.contains(k))
}
fn live_animation(ty: &VehicleType, index: usize) -> bool {
    let mut at = Some(index);
    let mut seen = HashSet::new();
    while let Some(i) = at {
        if !seen.insert(i) {
            break;
        }
        let d = &ty.model.meshes[ty.meshes[i].def_index];
        if d.animations.iter().any(|a| {
            ty.program
                .var(&a.variable)
                .is_some_and(|v| ty.program.stores(v))
        }) {
            return true;
        }
        at = d.anim_parent.as_ref().and_then(|id| {
            ty.meshes
                .iter()
                .position(|m| ty.model.meshes[m.def_index].mesh_ident.as_ref() == Some(id))
        });
    }
    false
}
fn fingerprint(v: &VehicleInstance, c: &Cabin) -> u64 {
    let ty = &v.ty;
    let mut h = std::collections::hash_map::DefaultHasher::new();
    // Installation directories and drive letters do not describe vehicle geometry.
    for n in &ty.program.var_names { n.to_ascii_lowercase().hash(&mut h); }
    for (i, m) in ty.meshes.iter().enumerate() {
        let d = &ty.model.meshes[m.def_index];
        d.file.hash(&mut h);
        d.mouse_event.hash(&mut h);
        d.anim_parent.hash(&mut h);
        d.mesh_ident.hash(&mut h);
        v.mesh_props.get(i).is_some_and(|p| p.visible).hash(&mut h);
        for p in [ty.mesh_boxes[i].0, ty.mesh_boxes[i].1] {
            for x in p.to_array() {
                x.to_bits().hash(&mut h);
            }
        }
        for a in &d.animations {
            a.variable.hash(&mut h);
            a.factor.to_bits().hash(&mut h);
            a.offset.to_bits().hash(&mut h);
            format!("{:?}", a.origins).hash(&mut h);
            ty.program
                .var(&a.variable)
                .is_some_and(|id| ty.program.stores(id))
                .hash(&mut h);
        }
    }
    let mut triggers: Vec<_> = ty.program.triggers.keys().collect();
    triggers.sort();
    triggers.hash(&mut h);
    for p in &c.graph.points {
        for x in p.to_array() {
            x.to_bits().hash(&mut h);
        }
    }
    for d in c.entries.iter().chain(&c.exits) {
        for x in d.inside.to_array() {
            x.to_bits().hash(&mut h);
        }
    }
    for s in &c.seats {
        s.switch_var.hash(&mut h);
        s.seated.hash(&mut h);
        for x in s.floor.to_array() {
            x.to_bits().hash(&mut h);
        }
    }
    h.finish()
}
pub fn detect(v: &VehicleInstance, c: &Cabin) -> Detection {
    let key = fingerprint(v, c);
    let cache = CACHE.get_or_init(Default::default);
    if let Ok(cached) = cache.lock() {
        if let Some(d) = cached.get(&key) {
            return d.clone();
        }
    }
    let mut d = discover(v, c, key);
    if d.profiles.is_empty() {
        if let Some(profile)=calibrated_profile(v,c) {
            d.reasons.push("Exact previously gameplay-tested geometry: calibrated access route".into());
            d.profiles.push(profile.clone());d.profile=Some(profile);
        }
    }
    if let Ok(mut cached) = cache.lock() {
        if cached.len() >= 128 {
            cached.clear();
        }
        cached.insert(key, d.clone());
    }
    d
}
fn calibrated_profile(v:&VehicleInstance,c:&Cabin)->Option<Profile> {
    // Match geometry, never installation directories or a driver's vehicle name.
    // This is the Torino 2007 route already exercised in the player's real game.
    let expected=[
        ("Interior/Outros_Rampa.o3d",Vec3::new(0.40220,-4.98784,0.39804),Vec3::new(0.68910,-3.59747,2.07723)),
        ("Interior/Balaustres_ferros_cadeirante.o3d",Vec3::new(-1.19519,-5.05059,0.99232),Vec3::new(-0.37069,-3.73161,3.01726)),
    ];
    if !expected.iter().all(|(name,lo,hi)|v.ty.meshes.iter().enumerate().any(|(i,m)|
        v.ty.model.meshes[m.def_index].file.replace('\\',"/").eq_ignore_ascii_case(name)
            &&(v.ty.mesh_boxes[i].0-*lo).abs().max_element()<0.002
            &&(v.ty.mesh_boxes[i].1-*hi).abs().max_element()<0.002)){return None;}
    if !c.exits.iter().any(|d|(d.inside-Vec3::new(1.25523,-4.13698,0.44279)).length()<0.002){return None;}
    let mut p:Profile=serde_json::from_str(include_str!("../../../../../accessibility-profiles/torino2007.json")).ok()?;
    p.replacement_meshes=vec!["Interior\\Outros_Rampa.o3d".into(),"Interior\\Outros_Rampa1.o3d".into(),"Interior\\Outros_Rampa2.o3d".into()];
    (p.validate().is_ok()&&v.var(&p.door_var).is_some()).then_some(p)
}
fn deck(ty: &VehicleType, index: usize) -> Option<NativeDeck> {
    let m = &ty.meshes[index];
    let d = &ty.model.meshes[m.def_index];
    let (lo, hi) = ty.mesh_boxes[index];
    let size = hi - lo;
    if size.y < 0.78 || !(0.55..=2.2).contains(&size.x) || size.z > 0.20 {
        return None;
    }
    let faces: Vec<_> = m
        .data
        .indices
        .chunks_exact(3)
        .filter_map(|t| {
            let p = t.map_positions(&m.data.positions);
            let cross = (p[1] - p[0]).cross(p[2] - p[0]);
            (cross.normalize_or_zero().z.abs() > 0.98).then_some((cross.length(), p))
        })
        .collect();
    let largest = faces.iter().map(|f| f.0).max_by(f32::total_cmp)?;
    let centre = (lo + hi).truncate() * 0.5;
    let planes: Vec<_> = faces
        .iter()
        .filter(|f| f.0 >= largest * 0.5)
        .filter_map(|f| height_on_plane(f.1, centre).map(|z| (z, f.1)))
        .collect();
    let top = planes.iter().max_by(|a, b| a.0.total_cmp(&b.0))?;
    let low = planes.iter().map(|f| f.0).min_by(f32::total_cmp)?;
    let thickness = (top.0 - low).max(0.0);
    if thickness > 0.10 {
        return None;
    }
    let mut corners = [
        Vec3::new(lo.x, lo.y, 0.0),
        Vec3::new(hi.x, lo.y, 0.0),
        Vec3::new(hi.x, hi.y, 0.0),
        Vec3::new(lo.x, hi.y, 0.0),
    ];
    for corner in &mut corners {
        corner.z = height_on_plane(top.1, corner.truncate())?;
    }
    let trigger = d
        .mouse_event
        .as_ref()
        .filter(|t| ty.program.trigger(t).is_some())
        .cloned()
        .or_else(|| {
            ty.program
                .trigger("ramplift_wunsch")
                .map(|_| "ramplift_wunsch".into())
        });
    let control_var = d
        .animations
        .iter()
        .find_map(|a| {
            let n = a.variable.replace("_mov", "_mode");
            (n != a.variable && ty.program.var(&n).is_some()).then_some(n)
        })
        .or_else(|| (trigger.as_deref() == Some("ramplift_wunsch")).then(|| "ramp_soll".into()));
    Some(NativeDeck {
        mesh: d.file.clone(),
        corners,
        thickness,
        trigger,
        control_var,
        index: Some(index),
    })
}
trait TriPositions {
    fn map_positions(&self, p: &[Vec3]) -> [Vec3; 3];
}
impl TriPositions for [u32] {
    fn map_positions(&self, p: &[Vec3]) -> [Vec3; 3] {
        [
            p[self[0] as usize],
            p[self[1] as usize],
            p[self[2] as usize],
        ]
    }
}

fn discover(v: &VehicleInstance, c: &Cabin, key: u64) -> Detection {
    let ty = &v.ty;
    let mut out = Detection {
        family: format!("{key:016x}"),
        native_meshes: Vec::new(),
        static_equipment: Vec::new(),
        reasons: Vec::new(),
        profile: None,
        profiles: Vec::new(),
    };
    let mut natives = Vec::new();
    let mut reserved = Vec::new();
    let mut static_boxes = Vec::new();
    for (i, m) in ty.meshes.iter().enumerate() {
        if !v.mesh_props.get(i).is_some_and(|p| p.visible) {
            continue;
        }
        let d = &ty.model.meshes[m.def_index];
        let (lo, hi) = ty.mesh_boxes[i];
        if equipment_name(&d.file) {
            if live_animation(ty, i) {
                out.native_meshes.push(d.file.clone());
                if !d.file.to_lowercase().contains("lift") {
                    if let Some(n) = deck(ty, i) {
                        natives.push(n);
                    }
                }
            } else {
                out.static_equipment.push(d.file.clone());
                static_boxes.push((lo, hi));
            }
        }
        if reserved_name(&d.file) {
            reserved.push((lo + hi) * 0.5);
        }
    }
    let unbound_native = ty.program.var_names.iter().any(|n| {
        let n = n.to_lowercase();
        ["ramp", "pandus", "lift_pos", "elevador_pos"]
            .iter()
            .any(|k| n.contains(k))
            && ![
                "light", "lamp", "warning", "cockpit", "wunsch", "button", "luz", "display",
                "visible", "config_", "partida", "assist", "hill", "accel", "slope",
            ]
            .iter()
            .any(|k| n.contains(k))
            && ty.program.var(&n).is_some_and(|id| ty.program.stores(id))
    });
    let geometry = Geometry::new(v);
    let half = ty
        .def
        .bounding_box
        .map_or(1.25, |b| b[0] * 0.5)
        .clamp(0.9, 1.5);
    let mut doors: Vec<_> = c.exits.iter().enumerate().collect();
    for d in &c.entries {
        if !doors
            .iter()
            .any(|(_, e)| (e.inside - d.inside).length() < 0.15)
        {
            doors.push((usize::MAX, d));
        }
    }
    let mut choices = Vec::new();
    for (exit, d) in doors {
        if c.parts
            .iter()
            .skip(1)
            .any(|p| d.inside.y < p.joint_y + 0.65)
        {
            continue;
        }
        let mut floors: Vec<_> = c
            .graph
            .points
            .iter()
            .filter(|p| p.x.abs() < 0.65 && (p.y - d.inside.y).abs() < 1.3)
            .map(|p| p.z)
            .collect();
        if floors.is_empty() {
            continue;
        }
        floors.sort_by(f32::total_cmp);
        let floor = floors[floors.len() / 2];
        let (width, vars, opening) = door_binding(ty, d.inside.y, floor, d.side);
        if width < 0.78 || vars.is_empty() {
            out.reasons.push(format!(
                "Door y={:.2}: no verified wide door animation",
                d.inside.y
            ));
            continue;
        }
        let native = natives
            .iter()
            .filter(|n| {
                let centre = (n.corners[0] + n.corners[2]) * 0.5;
                (centre.y - d.inside.y).abs() < 0.8 && centre.x.signum() == d.side
            })
            .min_by(|a, b| {
                ((a.corners[0].y + a.corners[2].y) * 0.5 - d.inside.y)
                    .abs()
                    .total_cmp(&((b.corners[0].y + b.corners[2].y) * 0.5 - d.inside.y).abs())
            })
            .cloned();
        let hint = reserved.iter().any(|p| (p.y - d.inside.y).abs() < 1.7)
            || static_boxes
                .iter()
                .any(|(a, b)| ((a.y + b.y) * 0.5 - d.inside.y).abs() < 1.7);
        if native.is_none() && (unbound_native || !out.native_meshes.is_empty()) {
            continue;
        }
        let mut threshold = Vec3::new(d.side * opening.x.clamp(0.9,half), opening.y, floor);
        if let Some(n) = &native {
            threshold.x = if d.side > 0.0 {
                n.corners[2].x
            } else {
                n.corners[0].x
            };
            threshold.y = (n.corners[0].y + n.corners[2].y) * 0.5;
            threshold.z = n.corners[0].z.max(d.inside.z);
            if (0.0..=0.03).contains(&(floor-threshold.z)){threshold.z=floor;}
            if (threshold.z - floor).abs() > 0.18 {
                continue;
            }
        }
        let centre=threshold;
        let door_choices=choices.len();
        let limit=((width.min(native.as_ref().map_or(width,|n|(n.corners[2].y-n.corners[0].y).abs()))-0.78)*0.5).max(0.0);
        let mut offsets=vec![0.0];
        if native.is_none()&&(d.inside.y-centre.y).abs()<=limit{offsets.push(d.inside.y-centre.y);}
        for offset in [-0.04_f32,0.04,-0.08,0.08,-0.12,0.12,-0.18,0.18] {
            if offset.abs()<=limit {offsets.push(offset);}
        }
        for offset in offsets {
        let mut threshold=centre;threshold.y+=offset;
        if native.is_none() {
            threshold.z=geometry.height(Vec3::new(0.0,threshold.y,threshold.z)).unwrap_or(threshold.z);
        }
        let bays = find_bays(c, &geometry, threshold, &reserved, &static_boxes);
        let count = bays.len();
        let mut selected_bays: Vec<(Vec3, f32, Vec<Vec3>,Vec<Maneuver>)> = Vec::new();
        for &(bay, heading) in &bays {
            if selected_bays.iter().any(|(other, _, _,_)| (other.truncate()-bay.truncate()).length()<1.35) { continue; }
            let centre = Vec3::new(0.0, threshold.y, threshold.z);
            // A direct diagonal avoids unnecessary pivot turns beside the rails.
            let mut routes = vec![
                vec![threshold.lerp(bay, 0.5)],
                vec![centre, Vec3::new(0.0, bay.y, bay.z)],
            ];
            for ahead in [0.25, 0.4, 0.6, 0.8] {
                let approach = Vec3::new(
                    0.0,
                    bay.y + if heading == 180.0 { ahead } else { -ahead },
                    bay.z,
                );
                routes.push(vec![centre, approach]);
                routes.push(vec![approach]);
            }
            for route in &mut routes {
                for p in route {
                    if let Some(z) = geometry.height(*p) {
                        p.z = z;
                    }
                }
            }
            if let Some(route) = routes
                .into_iter()
                .find(|r| geometry.route_clear(threshold, r, bay, heading))
            { selected_bays.push((bay, heading, route,Vec::new())); }
        }
        if selected_bays.is_empty() {
            if let Some((bay,heading,maneuvers))=geometry.plan_route(threshold,&bays) {
                let route=maneuvers.iter().map(|m|m.at).collect();
                selected_bays.push((bay,heading,route,maneuvers));
            }
        }
        if selected_bays.is_empty() {
            let entrance_heading=if threshold.x>0.0{270.0}else{90.0};
            let reason=if count==0 {"no clear wheelchair bay found"}
                else if !geometry.chair_clear(threshold,entrance_heading) {
                    "door threshold blocked for wheelchair footprint"
                } else {"no collision-verified turning route"};
            out.reasons.push(format!(
                "Door y={:.2}: {count} candidate bays, {reason}",
                threshold.y
            ));
            continue;
        }
        for (bay, heading, route,maneuvers) in selected_bays {
        let platform_length = native
            .as_ref()
            .map_or(1.4, |n| (n.corners[2].x - n.corners[0].x).abs());
        let clear_width = width.min(
            native
                .as_ref()
                .map_or(1.4, |n| (n.corners[2].y - n.corners[0].y).abs() - 0.02),
        );
        let replacement_meshes = if native.is_none() {
            out.static_equipment
                .iter()
                .filter(|name| {
                    ty.meshes
                        .iter()
                        .position(|m| ty.model.meshes[m.def_index].file == **name)
                        .is_some_and(|i| {
                            ((ty.mesh_boxes[i].0.y + ty.mesh_boxes[i].1.y) * 0.5 - threshold.y)
                                .abs()
                                < 1.0
                        })
                })
                .cloned()
                .collect()
        } else {
            Vec::new()
        };
        let deploy_trigger = native.as_ref().and_then(|n| n.trigger.clone());
        let p = Profile {
            schema: 1,
            name: format!(
                "Automatic accessibility · {}",
                ty.def
                    .path
                    .file_stem()
                    .unwrap_or_default()
                    .to_string_lossy()
            ),
            equipment: if native.is_some() {
                Equipment::NativeRamp
            } else {
                Equipment::AddedLift
            },
            threshold,
            bay,
            route,
            maneuvers,
            bay_heading: heading as f64,
            door_var: vars[0].clone(),
            door_vars: vars[1..].to_vec(),
            door_open_value: 0.95,
            exit_index: exit.min(c.exits.len().saturating_sub(1)),
            clear_width,
            platform_length,
            native_ground_var: None,
            native_floor_var: None,
            native_height_var: None,
            native_stowed_var: None,
            deploy_trigger: deploy_trigger.clone(),
            alight_trigger: deploy_trigger.clone(),
            stow_trigger: deploy_trigger.filter(|t| t != "ramplift_wunsch"),
            raise_trigger: None,
            lower_trigger: None,
            fold: fold_seat(ty, c, bay),
            replacement_meshes,
            native: native.clone(),
        };
        if let Err(e) = p.validate() {
            out.reasons.push(e);
            continue;
        }
        let score =
            (bay - threshold).truncate().length() + if p.native.is_some() { 0.0 } else { 4.0 };
        choices.push((score, p));
        }
        if !hint && native.is_none() && !choices.is_empty() {
            out.reasons.push("Verified wide door and clear floor area; driver-operated auxiliary lift required".into());
        }
        if choices.len()>door_choices{break;}
    }
    }
    choices.sort_by(|a, b| a.0.total_cmp(&b.0));
    for (_, p) in choices {
        if !out.profiles.iter().any(|other| (other.bay-p.bay).truncate().length()<1.35) {
            out.profiles.push(p);
        }
    }
    out.profile = out.profiles.first().cloned();
    if out.profile.is_none() && out.reasons.is_empty() {
        out.reasons.push("Native equipment could not be bound to a verified door and route; no duplicate equipment added".into());
    }
    out
}
fn door_binding(ty: &VehicleType, y: f32, floor: f32, side: f32) -> (f32, Vec<String>,Vec2) {
    let mut low = f32::INFINITY;
    let mut high = f32::NEG_INFINITY;
    let mut vars = Vec::new();
    let mut outer=0.0_f32;
    for (i, m) in ty.meshes.iter().enumerate() {
        let (a, b) = ty.mesh_boxes[i];
        if ((a.y + b.y) * 0.5 - y).abs() > 0.85 || (a.x+b.x).signum()!=side.signum() || a.z > floor + 0.8 || b.z < floor + 0.8 {
            continue;
        }
        for anim in &ty.model.meshes[m.def_index].animations {
            let n = anim.variable.to_lowercase();
            let door = n
                .strip_prefix("door_")
                .is_some_and(|s| !s.is_empty() && s.chars().all(|c| c.is_ascii_digit()))
                || n.strip_prefix("door")
                    .and_then(|s| s.strip_suffix("_positiony"))
                    .is_some_and(|s| !s.is_empty() && s.chars().all(|c| c.is_ascii_digit()));
            if door && ty.program.var(&n).is_some_and(|id| ty.program.stores(id)) {
                if !vars.contains(&n) {
                    vars.push(n);
                }
                low = low.min(a.y);
                high = high.max(b.y);
                outer=outer.max(a.x*side).max(b.x*side);
            }
        }
    }
    vars.sort();
    (
        if vars.is_empty() {
            0.0
        } else {
            (high - low - 0.04).min(1.8)
        },
        vars,
        Vec2::new(outer,if low.is_finite()&&high.is_finite(){(low+high)*0.5}else{y}),
    )
}
fn fold_seat(ty: &VehicleType, c: &Cabin, bay: Vec3) -> Option<FoldSeat> {
    for (i, m) in ty.meshes.iter().enumerate() {
        let d = &ty.model.meshes[m.def_index];
        let (lo, hi) = ty.mesh_boxes[i];
        if !reserved_name(&d.file) || (((lo + hi) * 0.5 - bay).truncate()).length() > 1.2 {
            continue;
        }
        let Some(t) = d
            .mouse_event
            .as_ref()
            .filter(|t| ty.program.trigger(t).is_some())
        else {
            continue;
        };
        if let Some(a) = d.animations.iter().find(|a| {
            ty.program
                .var(&a.variable)
                .is_some_and(|v| ty.program.stores(v))
        }) {
            let switched = c.seats.iter().any(|s| {
                (s.floor - bay).truncate().length() < 0.9
                    && s.switch_var.as_deref() == Some(a.variable.as_str())
            });
            return Some(FoldSeat {
                trigger: t.clone(),
                variable: a.variable.clone(),
                folded_value: if switched || hi.z - lo.z > (hi.x - lo.x).min(hi.y - lo.y) {
                    0.0
                } else {
                    1.0
                },
            });
        }
    }
    None
}

struct Triangle {
    p: [Vec3; 3],
    low: Vec3,
    high: Vec3,
}
struct Geometry {
    floor: Vec<[Vec3; 3]>,
    floor_cells: HashMap<(i32,i32), Vec<usize>>,
    triangles: Vec<Triangle>,
    cells: HashMap<(i32, i32, i32), Vec<usize>>,
    large: Vec<usize>,
}
impl Geometry {
    fn new(v: &VehicleInstance) -> Self {
        let ty = &v.ty;
        let mut floor = Vec::new();
        let mut triangles = Vec::new();
        for (i, m) in ty.meshes.iter().enumerate() {
            let d = &ty.model.meshes[m.def_index];
            if !v.mesh_props.get(i).is_some_and(|p| p.visible)
                || !d.animations.is_empty()
                || d.anim_parent.is_some()
                || equipment_name(&d.file)
            {
                continue;
            }
            for t in m.data.indices.chunks_exact(3) {
                let p = t.map_positions(&m.data.positions);
                if p.iter().any(|p| !p.is_finite()) {
                    continue;
                }
                let low = p[0].min(p[1]).min(p[2]);
                let high = p[0].max(p[1]).max(p[2]);
                if (p[1] - p[0]).cross(p[2] - p[0]).normalize_or_zero().z.abs() > 0.95 {
                    floor.push(p);
                }
                triangles.push(Triangle { p, low, high });
            }
        }
        Self::index(floor, triangles)
    }
    fn index(floor: Vec<[Vec3; 3]>, triangles: Vec<Triangle>) -> Self {
        let mut floor_cells:HashMap<(i32,i32),Vec<usize>>=HashMap::new();
        for (i,t) in floor.iter().enumerate() {
            let lo=(t[0].min(t[1]).min(t[2]).truncate()*2.0).floor().as_ivec2();
            let hi=(t[0].max(t[1]).max(t[2]).truncate()*2.0).floor().as_ivec2();
            if (hi-lo).abs().max_element()>1000 {continue;}
            for x in lo.x..=hi.x {for y in lo.y..=hi.y {floor_cells.entry((x,y)).or_default().push(i);}}
        }
        let mut cells: HashMap<_, Vec<usize>> = HashMap::new();
        let mut large = Vec::new();
        for (i, t) in triangles.iter().enumerate() {
            if t.low.abs().max_element() > 1000.0 || t.high.abs().max_element() > 1000.0 {
                large.push(i);
                continue;
            }
            let a = (t.low * 2.0).floor().as_ivec3();
            let b = (t.high * 2.0).floor().as_ivec3();
            let n = (b - a + glam::IVec3::ONE).as_i64vec3();
            if n.x * n.y * n.z > 65536 {
                large.push(i);
                continue;
            }
            for x in a.x..=b.x {
                for y in a.y..=b.y {
                    for z in a.z..=b.z {
                        cells.entry((x, y, z)).or_default().push(i);
                    }
                }
            }
        }
        Self {
            floor,
            floor_cells,
            triangles,
            cells,
            large,
        }
    }
    fn hit(&self, lo: Vec3, hi: Vec3, test: impl Fn(&Triangle) -> bool) -> bool {
        let hit = |i: usize| {
            let t = &self.triangles[i];
            t.low.cmplt(hi).all() && t.high.cmpgt(lo).all() && test(t)
        };
        if self.large.iter().any(|i| hit(*i)) {
            return true;
        }
        let a = (lo * 2.0).floor().as_ivec3();
        let b = (hi * 2.0).floor().as_ivec3();
        for x in a.x..=b.x {
            for y in a.y..=b.y {
                for z in a.z..=b.z {
                    if self
                        .cells
                        .get(&(x, y, z))
                        .is_some_and(|list| list.iter().any(|i| hit(*i)))
                    {
                        return true;
                    }
                }
            }
        }
        false
    }
    fn height(&self, p: Vec3) -> Option<f32> {
        let key=(p.truncate()*2.0).floor().as_ivec2();
        floor_height_iter(self.floor_cells.get(&(key.x,key.y))?.iter().map(|i|&self.floor[*i]),p)
    }
    fn chair_clear(&self, p: Vec3, heading: f32) -> bool {
        let (s, c) = heading.to_radians().sin_cos();
        let mut forward = Vec3::new(s, c, 0.0);
        let mut right = Vec3::new(c, -s, 0.0);
        let up = contact_normal(p, heading, |at| self.height(at)).unwrap_or(Vec3::Z);
        let tilt = glam::Quat::from_rotation_arc(Vec3::Z, up);
        forward = tilt * forward;
        right = tilt * right;
        // Wheels, footrests, legs, arms, torso, head and back frame.
        [
            // The space between the rear wheels is not a solid block. Preserve
            // each wheel's full width instead of falsely filling that empty space.
            (-0.34, 0.0, 0.31, Vec3::new(0.04, 0.32, 0.30)),
            (0.34, 0.0, 0.31, Vec3::new(0.04, 0.32, 0.30)),
            (0.0, 0.02, 0.44, Vec3::new(0.27, 0.29, 0.09)),
            (-0.255, 0.40, 0.10, Vec3::new(0.04, 0.10, 0.09)),
            (0.255, 0.40, 0.10, Vec3::new(0.04, 0.10, 0.09)),
            (0.0, 0.49, 0.145, Vec3::new(0.29, 0.20, 0.105)),
            (0.0, 0.36, 0.45, Vec3::new(0.22, 0.21, 0.28)),
            (0.0, 0.025, 0.80, Vec3::new(0.38, 0.25, 0.20)),
            (0.0, -0.04, 0.88, Vec3::new(0.24, 0.22, 0.30)),
            (0.0, -0.025, 1.22, Vec3::new(0.15, 0.17, 0.18)),
            (0.0, -0.26, 0.77, Vec3::new(0.29, 0.12, 0.34)),
        ]
        .into_iter()
        .all(|(x,y, z, half)| {
            let centre = p + right*x + forward * y + up * z;
            let ext = right.abs() * half.x + forward.abs() * half.y + up.abs() * half.z;
            !self.hit(centre - ext, centre + ext, |t| {
                // The rolling contact surface is checked as a floor, not a
                // solid obstacle filling the wheel's bounding volume.
                let normal=(t.p[1]-t.p[0]).cross(t.p[2]-t.p[0]).normalize_or_zero();
                if normal.dot(up).abs()>0.95 && height_on_plane(t.p,p.truncate())
                    .is_some_and(|z|z<=p.z+0.06) {return false;}
                let hit=triangle_box(
                    t.p.map(|v| {
                        let v = v - centre;
                        Vec3::new(v.dot(right), v.dot(forward), v.dot(up))
                    }),
                    Vec3::ZERO,
                    half,
                );
                hit
            })
        })
    }
    fn turn_clear(&self,at:Vec3,from:f32,to:f32)->bool {
        let yaw=(to-from+180.0).rem_euclid(360.0)-180.0;
        let steps=(yaw.abs()/5.0).ceil().max(1.0) as usize;
        (0..=steps).all(|i|self.chair_clear(at,from+yaw*i as f32/steps as f32))
    }
    fn leg_clear(&self,a:Vec3,heading:f32,b:Vec3,end:f32)->Option<bool> {
        let delta=b-a;let distance=delta.truncate().length();
        if distance<0.002 {return self.turn_clear(a,heading,end).then_some(false);}
        if delta.z.abs()>0.18 || delta.z.abs()/distance>0.30{return None;}
        let direction=delta.x.atan2(delta.y).to_degrees();
        let reverse=((direction-heading+180.0).rem_euclid(360.0)-180.0).abs()>90.0;
        let wanted=direction+if reverse{180.0}else{0.0};
        if !self.turn_clear(a,heading,wanted){return None;}
        let steps=(distance/0.06).ceil().max(1.0) as usize;
        if !(1..=steps).all(|i|self.chair_clear(a.lerp(b,i as f32/steps as f32),wanted))
            ||!self.turn_clear(b,wanted,end){return None;}
        Some(reverse)
    }
    /// Bounded pose search preserves the wheelchair's full volume through every
    /// pivot and forward/reverse leg. A failed search never authorizes a shortcut.
    fn plan_route(&self,start:Vec3,bays:&[(Vec3,f32)])->Option<(Vec3,f32,Vec<Maneuver>)> {
        // The first grid is fast. The second searches narrow turning bays that
        // fall between its 13 cm samples; both use the full-volume collision test.
        self.plan_route_grid(start,bays,0.13,6000)
            .or_else(||self.plan_route_grid(start,bays,0.09,8000))
    }
    fn plan_route_grid(&self,start:Vec3,bays:&[(Vec3,f32)],step:f32,max_expanded:usize)->Option<(Vec3,f32,Vec<Maneuver>)> {
        use std::{cmp::Reverse,collections::BinaryHeap};
        type Key=(i16,i16,u8);
        if bays.is_empty(){return None;}
        let initial:Key=(0,0,if start.x>0.0{6}else{2});
        if !self.chair_clear(start,initial.2 as f32*45.0){return None;}
        let position=|k:Key| {
            let mut p=start+Vec3::new(k.0 as f32*step,k.1 as f32*step,0.0);
            p.z=self.height(p).unwrap_or(start.z);p
        };
        let heuristic=|p:Vec3|bays.iter().map(|(b,_)|(*b-p).truncate().length()).fold(f32::INFINITY,f32::min);
        let mut queue=BinaryHeap::new();
        let mut visited:HashMap<Key,(f32,Option<Key>,bool)>=HashMap::new();
        visited.insert(initial,(0.0,None,false));
        queue.push(Reverse(((heuristic(start)*1000.0)as u32,0u32,initial)));
        let directions=[(0,1),(1,1),(1,0),(1,-1),(0,-1),(-1,-1),(-1,0),(-1,1)];
        let mut expanded=0;
        while let Some(Reverse((_,queued,key)))=queue.pop() {
            let &(cost,_,_)=visited.get(&key)?;
            if queued!=(cost*1000.0).round()as u32{continue;}
            expanded+=1;if expanded>max_expanded{break;}
            let at=position(key);let heading=key.2 as f32*45.0;
            for &(bay,end) in bays {
                if (at-bay).truncate().length()>0.20{continue;}
                let Some(reverse)=self.leg_clear(at,heading,bay,end)else{continue;};
                let mut path=Vec::new();let mut next=key;
                while next!=initial {
                    let &(_,parent,reverse)=visited.get(&next)?;
                    path.push(Maneuver{at:position(next),heading:next.2 as f64*45.0,reverse});
                    next=parent?;
                }
                path.reverse();
                path.push(Maneuver{at:bay,heading:end as f64,reverse});
                // Coalesce only collinear legs with the same direction, retaining
                // every pivot and any slope break that the controller must follow.
                let mut compact:Vec<Maneuver>=Vec::new();
                for step in path {
                    if compact.len()>=2 {
                        let n=compact.len();let a=&compact[n-2];let b=&compact[n-1];
                        let first=b.at-a.at;let second=step.at-b.at;
                        if a.heading==b.heading&&b.heading==step.heading&&b.reverse==step.reverse
                            &&first.length()>0.001&&second.length()>0.001
                            &&first.normalize().dot(second.normalize())>0.9999 {
                            compact.pop();
                        }
                    }
                    compact.push(step);
                }
                if compact.len()<=64{return Some((bay,end,compact));}
            }
            let (dx,dy)=directions[key.2 as usize];
            for (other,reverse,extra) in [
                ((key.0,key.1,(key.2+1)%8),false,0.11),
                ((key.0,key.1,(key.2+7)%8),false,0.11),
                ((key.0+dx,key.1+dy,key.2),false,step),
                ((key.0-dx,key.1-dy,key.2),true,step+0.03),
            ] {
                let to=position(other);
                if to.x.abs()>1.55||to.x*start.x.signum()>start.x.abs()+0.01
                    ||(to.y-start.y).abs()>3.15||(to.z-start.z).abs()>0.18{continue;}
                // Only the access platform may bridge a missing floor mesh.
                if self.height(to).is_none() && !(to.x*start.x.signum()>start.x.abs()-0.85
                    &&(to.y-start.y).abs()<0.25){continue;}
                let proposed=cost+extra;
                if visited.get(&other).is_some_and(|(c,_,_)|*c<=proposed){continue;}
                let clear=if key.0==other.0&&key.1==other.1 {
                    self.turn_clear(at,heading,other.2 as f32*45.0)
                }else {
                    let distance=(to-at).truncate().length();
                    (to.z-at.z).abs()/distance<0.30
                        &&(1..=3).all(|i|self.chair_clear(at.lerp(to,i as f32/3.0),heading))
                };
                if !clear{continue;}
                visited.insert(other,(proposed,Some(key),reverse));
                let priority=((proposed+heuristic(to))*1000.0).round()as u32;
                queue.push(Reverse((priority,(proposed*1000.0).round()as u32,other)));
            }
        }
        None
    }
    fn route_clear(&self, start: Vec3, route: &[Vec3], bay: Vec3, end_heading: f32) -> bool {
        let mut a = start;
        let mut heading = if start.x > 0.0 { 270.0 } else { 90.0 };
        for &b in route.iter().chain(std::iter::once(&bay)) {
            let delta = b - a;
            let distance = delta.truncate().length();
            if distance < 0.01 {
                continue;
            }
            if delta.z.abs() / distance > 0.30 {
                return false;
            }
            let wanted = delta.x.atan2(delta.y).to_degrees();
            let yaw = (wanted - heading + 180.0).rem_euclid(360.0) - 180.0;
            let steps = (yaw.abs() / 5.0).ceil().max(1.0) as usize;
            if !(1..=steps).all(|i| self.chair_clear(a, heading + yaw * i as f32 / steps as f32)) {
                return false;
            }
            let steps = (distance / 0.08).ceil().max(1.0) as usize;
            for i in 1..=steps {
                let p = a.lerp(b, i as f32 / steps as f32);
                if !self.chair_clear(p, wanted) {
                    return false;
                }
            }
            a = b;
            heading = wanted;
        }
        let yaw = (end_heading - heading + 180.0).rem_euclid(360.0) - 180.0;
        let n = (yaw.abs() / 5.0).ceil().max(1.0) as usize;
        (1..=n).all(|i| self.chair_clear(bay, heading + yaw * i as f32 / n as f32))
    }
}
/// Fit the chair's rear wheels and front casters to a gently sloping floor.
pub fn contact_normal(
    at: Vec3,
    heading: f32,
    height: impl Fn(Vec3) -> Option<f32>,
) -> Option<Vec3> {
    let (s, c) = heading.to_radians().sin_cos();
    let forward = Vec3::new(s, c, 0.0);
    let right = Vec3::new(c, -s, 0.0);
    let rear = [
        height(at - right * 0.30).unwrap_or(at.z),
        height(at + right * 0.30).unwrap_or(at.z),
    ];
    let front = [
        height(at + forward * 0.42 - right * 0.18).unwrap_or(at.z),
        height(at + forward * 0.42 + right * 0.18).unwrap_or(at.z),
    ];
    let sideways = (rear[1] - rear[0]) / 0.60;
    let ahead = ((front[0] + front[1]) - (rear[0] + rear[1])) * 0.5 / 0.42;
    let normal = (Vec3::Z - right * sideways - forward * ahead).normalize_or_zero();
    (normal.is_finite() && normal.z > 0.85).then_some(normal)
}
fn floor_height(triangles: &[[Vec3; 3]], p: Vec3) -> Option<f32> {
    floor_height_iter(triangles.iter(),p)
}
fn floor_height_iter<'a>(triangles:impl Iterator<Item=&'a [Vec3;3]>,p:Vec3)->Option<f32> {
    triangles
        .filter_map(|t| {
            let a = (t[1] - t[0]).truncate();
            let b = (t[2] - t[0]).truncate();
            let v = (p - t[0]).truncate();
            let den = a.perp_dot(b);
            if den.abs() < 1e-7 {
                return None;
            }
            let u = v.perp_dot(b) / den;
            let w = a.perp_dot(v) / den;
            let z = t[0].z + u * (t[1].z - t[0].z) + w * (t[2].z - t[0].z);
            (u >= -0.001 && w >= -0.001 && u + w <= 1.001 && (z - p.z).abs() < 0.15).then_some(z)
        })
        .max_by(f32::total_cmp)
}
/// Only floor triangles are retained; rendering meshes are not held by this sampler.
pub struct FloorSurface {
    triangles: Vec<[Vec3; 3]>,
    last: Mutex<Option<(Vec<u32>, Vec3)>>,
}
impl FloorSurface {
    pub fn load(v: &VehicleInstance) -> Self {
        let mut triangles = Vec::new();
        for (i, m) in v.ty.meshes.iter().enumerate() {
            let d = &v.ty.model.meshes[m.def_index];
            if !v.mesh_props.get(i).is_some_and(|p| p.visible)
                || !d.animations.is_empty()
                || d.anim_parent.is_some()
                || equipment_name(&d.file)
            {
                continue;
            }
            for t in m.data.indices.chunks_exact(3) {
                let p = t.map_positions(&m.data.positions);
                if p.iter().all(|p| p.is_finite())
                    && (p[1] - p[0]).cross(p[2] - p[0]).normalize_or_zero().z.abs() > 0.95
                {
                    triangles.push(p);
                }
            }
        }
        Self {
            triangles,
            last: Mutex::new(None),
        }
    }
    pub fn normal(
        &self,
        at: Vec3,
        heading: f32,
        p: &Profile,
        ground: f32,
        surface: Option<[Vec3; 3]>,
        active: bool,
    ) -> Vec3 {
        let mut key: Vec<_> = at
            .to_array()
            .into_iter()
            .chain([heading, ground])
            .map(f32::to_bits)
            .collect();
        key.push(u32::from(active));
        key.push(u32::from(surface.is_some()));
        if let Some(s) = surface {
            for point in s {
                key.extend(point.to_array().map(f32::to_bits));
            }
        }
        if let Ok(last) = self.last.lock() {
            if let Some((k, n)) = &*last {
                if k == &key {
                    return *n;
                }
            }
        }
        let ramp = active && matches!(p.equipment, Equipment::AddedRamp | Equipment::NativeRamp);
        let normal = contact_normal(at, heading, |point| {
            if (point.x - p.threshold.x) * p.side() > 0.0 {
                Some(if ramp {
                    ramp_height(p, point.truncate(), ground, surface)
                } else {
                    at.z
                })
            } else {
                floor_height(&self.triangles, point)
            }
        })
        .unwrap_or(Vec3::Z);
        if let Ok(mut last) = self.last.lock() {
            *last = Some((key, normal));
        }
        normal
    }
}
pub fn ramp_height(p: &Profile, at: Vec2, ground: f32, surface: Option<[Vec3; 3]>) -> f32 {
    let t = ((at.x - p.threshold.x) * p.side() / p.platform_length).clamp(0.0, 1.0);
    if t >= 1.0 {
        return ground;
    }
    let measured = surface.and_then(|s| height_on_plane(s, at));
    let lip = surface
        .and_then(|s| height_on_plane(s, p.threshold.truncate()))
        .map_or(0.0, |h| p.threshold.z - h);
    measured.unwrap_or(p.threshold.z + (ground - p.threshold.z) * t)
        + lip * (1.0 - t * p.platform_length / 0.20).clamp(0.0, 1.0)
}

fn triangle_box(p: [Vec3; 3], centre: Vec3, half: Vec3) -> bool {
    let p = p.map(|p| p - centre);
    let edges = [p[1] - p[0], p[2] - p[1], p[0] - p[2]];
    let separated = |axis: Vec3| {
        let a = p.map(|p| p.dot(axis));
        let r = half.dot(axis.abs());
        a.into_iter().fold(f32::INFINITY, f32::min) > r + 1e-5
            || a.into_iter().fold(f32::NEG_INFINITY, f32::max) < -r - 1e-5
    };
    if separated(edges[0].cross(edges[1])) {
        return false;
    }
    [Vec3::X, Vec3::Y, Vec3::Z]
        .into_iter()
        .all(|a| !separated(a) && edges.iter().all(|e| !separated(a.cross(*e))))
}
fn find_bays(
    c: &Cabin,
    g: &Geometry,
    threshold: Vec3,
    reserved: &[Vec3],
    static_boxes: &[(Vec3, Vec3)],
) -> Vec<(Vec3, f32)> {
    let mut bays = Vec::new();
    for side in [-threshold.x.signum(), threshold.x.signum()] {
        for ix in 0..9 {
            for iy in -40_i32..=40 {
                // A same-side bay can be adjacent to a wide accessible door.
                // Reject only the doorway footprint, not the surrounding cabin.
                if side == threshold.x.signum() && iy.abs() < 14 {
                    continue;
                }
                let mut p = Vec3::new(
                    side * (0.18 + ix as f32 * 0.09),
                    threshold.y + iy as f32 * 0.065,
                    threshold.z,
                );
                let Some(z) = g.height(p) else {
                    continue;
                };
                p.z = z;
                if static_boxes.iter().any(|(a, b)| {
                    a.x < p.x + 0.38 && b.x > p.x - 0.38 && a.y < p.y + 0.69 && b.y > p.y - 0.69
                }) {
                    continue;
                }
                if c.seats.iter().any(|s| {
                    s.seated
                        && s.switch_var.is_none()
                        && !reserved
                            .iter()
                            .any(|r| (r.truncate() - s.floor.truncate()).length() < 0.7)
                        && (s.floor.x - p.x).abs() < 0.38
                        && (s.floor.y - p.y).abs() < 0.6
                }) {
                    continue;
                }
                if ![-1.0, 1.0].iter().all(|x| {
                    [-1.0, 1.0].iter().all(|y| {
                        g.height(p + Vec3::new(x * 0.30, y * 0.43, 0.0))
                            .is_some_and(|h| (h - p.z).abs() < 0.07)
                    })
                }) {
                    continue;
                }
                let hint = reserved
                    .iter()
                    .map(|r| (r.truncate() - p.truncate()).length())
                    .fold(1.5, f32::min);
                for heading in [180.0, 0.0] {
                    if g.chair_clear(p, heading) {
                        bays.push((
                            (p.y - threshold.y).abs()
                                + hint * 0.5
                                + (p.x.abs() - 0.65).abs()
                                + if heading == 0.0 { 0.2 } else { 0.0 },
                            (p, heading),
                        ));
                    }
                }
            }
        }
    }
    bays.sort_by(|a, b| a.0.total_cmp(&b.0));
    bays.into_iter().map(|p| p.1).collect()
}
pub fn height_on_plane(surface: [Vec3; 3], at: Vec2) -> Option<f32> {
    let n = (surface[1] - surface[0]).cross(surface[2] - surface[0]);
    if !n.is_finite() || n.z.abs() < 1e-6 {
        return None;
    }
    let z = surface[0].z - (n.x * (at.x - surface[0].x) + n.y * (at.y - surface[0].y)) / n.z;
    z.is_finite().then_some(z)
}
pub fn doors_ready(v: &VehicleInstance, p: &Profile) -> bool {
    std::iter::once(&p.door_var)
        .chain(&p.door_vars)
        .all(|name| {
            v.var(name)
                .is_some_and(|x| x.is_finite() && x >= p.door_open_value)
                && v.animators.iter().all(|mesh| {
                    mesh.anims.iter().all(|(a, _, s)| {
                        !a.variable.eq_ignore_ascii_case(name)
                            || a.factor.abs() < 1e-6
                            || (s.value - a.offset) / a.factor >= p.door_open_value
                    })
                })
        })
}
pub fn fold_ready(v: &VehicleInstance, p: &Profile) -> bool {
    let Some(f) = &p.fold else {
        return true;
    };
    v.var(&f.variable)
        .is_some_and(|x| x.is_finite() && (x - f.folded_value).abs() < 0.02)
        && v.animators.iter().all(|mesh| {
            mesh.anims.iter().all(|(a, _, s)| {
                !a.variable.eq_ignore_ascii_case(&f.variable)
                    || (s.value - (f.folded_value * a.factor + a.offset)).abs()
                        < if a.kind == Some(omsi_model::AnimKind::Trans) {
                            0.008
                        } else {
                            1.0
                        }
            })
        })
}
pub fn feedback(v: &VehicleInstance, p: &Profile) -> Feedback {
    let Some(n) = &p.native else {
        return Feedback::default();
    };
    let i = n
        .index
        .filter(|&i| {
            v.ty.meshes.get(i).is_some_and(|m| {
                v.ty.model.meshes[m.def_index]
                    .file
                    .eq_ignore_ascii_case(&n.mesh)
            })
        })
        .or_else(|| {
            v.ty.meshes.iter().position(|m| {
                v.ty.model.meshes[m.def_index]
                    .file
                    .eq_ignore_ascii_case(&n.mesh)
            })
        });
    let Some(i) = i else {
        return Feedback::default();
    };
    let Some(xf) = v.mesh_transforms.get(i) else {
        return Feedback::default();
    };
    if !v.mesh_props.get(i).is_some_and(|p| p.visible) {
        return Feedback::default();
    }
    let mut corners = n.corners.map(|p| xf.transform_point3(p));
    let under = n
        .corners
        .map(|p| xf.transform_point3(p - Vec3::Z * n.thickness));
    if under.iter().map(|p| p.z).sum::<f32>() > corners.iter().map(|p| p.z).sum::<f32>() {
        corners = under;
    }
    if corners.iter().any(|p| !p.is_finite()) {
        return Feedback::default();
    }
    let mut across = corners;
    across.sort_by(|a, b| (a.x * p.side()).total_cmp(&(b.x * p.side())));
    let inner = (across[0] + across[1]) * 0.5;
    let outer = (across[2] + across[3]) * 0.5;
    let extension = (outer.x - p.threshold.x) * p.side();
    let mut normal = (corners[1] - corners[0])
        .cross(corners[3] - corners[0])
        .normalize_or_zero();
    if normal.z < 0.0 {
        normal = -normal;
    }
    let contact_edge = outer - normal * n.thickness;
    let world = v.position + v.body_rotation().transform_vector3(contact_edge).as_dvec3();
    let ground = match &v.contact {
        Some(c) => c.probe(world.x, world.y, world.z + 0.5).below,
        None => v.ground.as_ref().and_then(|g| g(world.x, world.y)),
    };
    let contact = ground.is_some_and(|z| (world.z - z).abs() < 0.035);
    let settled = v.animators.get(i).is_some_and(|m| {
        m.anims.iter().all(|(a, _, s)| {
            v.var(&a.variable).is_some_and(|x| {
                (s.value - (x * a.factor + a.offset)).abs()
                    < if a.kind == Some(omsi_model::AnimKind::Trans) {
                        0.006
                    } else {
                        0.5
                    }
            })
        })
    });
    let extended = n.control_var.as_deref() != Some("ramp_soll")
        || v.var("ramp_pos").is_some_and(|x| x >= 0.995);
    let ground_ready = extension > 0.45
        && contact
        && v.body_rotation().transform_vector3(normal).z.abs() > 0.94
        && settled
        && extended;
    let floor_ready = ground_ready
        && (inner.z - p.threshold.z).abs() < 0.075
        && (inner.x - p.threshold.x).abs() < 0.15;
    let stowed = extension < 0.12
        && v.animators.get(i).is_some_and(|m| {
            m.anims.iter().all(|(a, _, s)| {
                (s.value - a.offset).abs()
                    < if a.kind == Some(omsi_model::AnimKind::Trans) {
                        0.012
                    } else {
                        1.0
                    }
            })
        });
    Feedback {
        door_open: doors_ready(v, p),
        ground_ready,
        floor_ready,
        stowed,
        equipment_height: None,
        surface: ground_ready.then_some([corners[0], corners[1], corners[3]]),
        bay_ready: p.fold.as_ref().map(|_| fold_ready(v, p)),
        floor_normal: None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn indicators_do_not_certify_accessible_equipment() {
        assert!(!equipment_name("Display/VDV_ramp1.o3d"));
        assert!(!equipment_name("Generic/Rampe_Taster.o3d"));
        assert!(equipment_name("Generic/Rampe_Main.o3d"));
        assert!(!reserved_name("Painel/LuzCadeirante.o3d"));
    }
    #[test]
    fn a_diagonal_triangle_does_not_fill_its_bounding_box() {
        let p = [
            Vec3::new(0.0, 0.0, 0.5),
            Vec3::new(2.0, 0.0, 0.5),
            Vec3::new(2.0, 2.0, 0.5),
        ];
        assert!(!triangle_box(p, Vec3::new(0.2, 1.8, 0.5), Vec3::splat(0.1)));
        assert!(triangle_box(p, Vec3::new(1.8, 0.2, 0.5), Vec3::splat(0.1)));
    }
    #[test]
    fn a_diagonal_arrival_avoids_a_rail_beside_a_pivot_turn() {
        let p = [
            Vec3::new(-0.8, -0.4, 0.0),
            Vec3::new(-0.4, -0.4, 1.4),
            Vec3::new(-0.8, -0.4, 1.4),
        ];
        let g = Geometry::index(
            Vec::new(),
            [p, [p[0], Vec3::new(-0.4, -0.4, 0.0), p[1]]]
                .into_iter()
                .map(|p| Triangle {
                    p,
                    low: p[0].min(p[1]).min(p[2]),
                    high: p[0].max(p[1]).max(p[2]),
                })
                .collect(),
        );
        let start = Vec3::new(1.25, -1.02, 0.0);
        let bay = Vec3::new(-0.60, -0.95, 0.0);
        assert!(g.route_clear(start, &[start.lerp(bay, 0.5)], bay, 180.0));
        assert!(!g.route_clear(
            start,
            &[Vec3::new(0.0, -1.02, 0.0), Vec3::new(0.0, -0.95, 0.0)],
            bay,
            180.0
        ));
    }
    #[test]
    fn a_collision_checked_route_can_turn_into_a_bay_on_a_fine_grid() {
        // An actual planar cabin floor, not a geometry-free shortcut.
        let a=Vec3::new(-1.5,-3.0,0.0);
        let b=Vec3::new(1.5,-3.0,0.0);
        let c=Vec3::new(1.5,3.0,0.0);
        let d=Vec3::new(-1.5,3.0,0.0);
        let floor=vec![[a,b,c],[a,c,d]];
        let triangles=floor.iter().map(|&p|Triangle{
            p,low:p[0].min(p[1]).min(p[2]),high:p[0].max(p[1]).max(p[2]),
        }).collect();
        let g=Geometry::index(floor,triangles);
        let start=Vec3::new(1.15,0.0,0.0);
        let bay=Vec3::new(-0.58,0.58,0.0);
        let (_,heading,steps)=g.plan_route(start,&[(bay,180.0)])
            .expect("clear continuous cabin must allow a checked approach");
        assert_eq!(heading,180.0);
        assert!(!steps.is_empty());
        assert!(steps.len()<=64);
        assert!((steps.last().unwrap().at-bay).length()<0.001);
        assert!(steps.iter().all(|m|g.chair_clear(m.at,m.heading as f32)));
    }
    fn native_fixture() -> (VehicleInstance, Profile) {
        let mut v = crate::timetable_run::tests::script_test_vehicle(
            "{frame} (L.L.command) (S.L.angle) {end}",
            "command\nangle\ndoor_a\ndoor_b\n",
            "",
        );
        let animation = omsi_model::Animation {
            origins: vec![
                omsi_model::AnimOrigin::Trans([1.2, 0.0, 0.4]),
                omsi_model::AnimOrigin::RotZ(90.0),
            ],
            kind: Some(omsi_model::AnimKind::Rot),
            variable: "angle".into(),
            max_speed: 90.0,
            ..Default::default()
        };
        let definition = omsi_model::MeshDef {
            file: "ramp".into(),
            animations: vec![animation],
            ..Default::default()
        };
        let ty = std::sync::Arc::make_mut(&mut v.ty);
        ty.model.meshes.push(definition.clone());
        ty.meshes.push(crate::vehicle::VehicleMesh {
            def_index: 0,
            data: Default::default(),
            pick_geometry: Default::default(),
            file: PathBuf::new(),
            materials: Vec::new(),
            overrides: Vec::new(),
            pivot: Mat4::IDENTITY,
            viewpoint: 0,
            skin: Vec::new(),
            keep_winding: false,
        });
        let mut animator =
            crate::anim::MeshAnimator::new(&definition, Mat4::IDENTITY, |n| v.ty.program.var(n));
        v.mesh_transforms = vec![animator.update(0.02, &v.state.vars)];
        v.animators = vec![animator];
        v.mesh_props = vec![crate::vehicle::MeshProps::default()];
        v.set_var("door_a", 1.0);
        v.set_var("door_b", 1.0);
        let mut p = super::super::tests::profile();
        p.equipment = Equipment::NativeRamp;
        p.platform_length = 0.8;
        p.threshold = Vec3::new(1.2, 0.0, 0.4);
        p.bay.z = 0.4;
        for point in &mut p.route {
            point.z = 0.4;
        }
        p.door_var = "door_a".into();
        p.door_vars = vec!["door_b".into()];
        p.native = Some(NativeDeck {
            mesh: "ramp".into(),
            corners: [
                Vec3::new(0.4, -0.5, 0.4),
                Vec3::new(1.2, -0.5, 0.4),
                Vec3::new(1.2, 0.5, 0.4),
                Vec3::new(0.4, 0.5, 0.4),
            ],
            thickness: 0.0,
            trigger: None,
            control_var: Some("command".into()),
            index: Some(0),
        });
        let ground = 0.4 - 0.8 * 15_f64.to_radians().sin();
        v.ground = Some(std::sync::Arc::new(move |_, _| Some(ground)));
        (v, p)
    }
    fn animate(v: &mut VehicleInstance, angle: f32, seconds: f32) {
        v.set_var("angle", angle);
        for _ in 0..(seconds / 0.02).round() as usize {
            v.mesh_transforms[0] = v.animators[0].update(0.02, &v.state.vars);
        }
    }
    #[test]
    fn the_native_deck_must_finish_its_real_animation_and_touch_the_curb() {
        let (mut v, p) = native_fixture();
        p.validate().unwrap();
        assert!(feedback(&v, &p).stowed);
        v.set_var("command", 195.0);
        animate(&mut v, 195.0, 0.02);
        assert!(!feedback(&v, &p).ground_ready);
        animate(&mut v, 195.0, 3.0);
        let f = feedback(&v, &p);
        assert!(f.ground_ready && f.floor_ready, "{f:?}");
        assert!(!f.stowed);
        assert!(
            (height_on_plane(f.surface.unwrap(), Vec2::new(1.2, 0.0)).unwrap() - 0.4).abs() < 0.001
        );
        v.heading = 123.0;
        assert!(feedback(&v, &p).ground_ready);
        v.ground = Some(std::sync::Arc::new(|_, _| Some(-0.3)));
        assert!(!feedback(&v, &p).ground_ready);
    }
    #[test]
    fn a_closed_leaf_or_hidden_deck_does_not_release_the_passenger() {
        let (mut v, p) = native_fixture();
        animate(&mut v, 195.0, 3.0);
        v.set_var("door_b", 0.7);
        assert!(!feedback(&v, &p).door_open);
        v.set_var("door_b", 1.0);
        assert!(feedback(&v, &p).door_open);
        v.mesh_props[0].visible = false;
        assert!(!feedback(&v, &p).ground_ready);
        v.mesh_props[0].visible = true;
        animate(&mut v, 0.0, 3.0);
        assert!(feedback(&v, &p).stowed);
    }
    #[test]
    fn door_feedback_follows_the_leaf_animation_instead_of_its_early_flag() {
        let (mut v, p) = native_fixture();
        v.set_var("door_b", 0.0);
        let def = omsi_model::MeshDef {
            animations: vec![omsi_model::Animation {
                variable: "door_b".into(),
                kind: Some(omsi_model::AnimKind::Trans),
                max_speed: 0.4,
                ..Default::default()
            }],
            ..Default::default()
        };
        let mut leaf =
            crate::anim::MeshAnimator::new(&def, Mat4::IDENTITY, |n| v.ty.program.var(n));
        leaf.update(0.02, &v.state.vars);
        v.animators.push(leaf);
        v.set_var("door_b", 1.0);
        v.animators[1].update(0.02, &v.state.vars);
        assert!(!doors_ready(&v, &p));
        for _ in 0..160 {
            v.animators[1].update(0.02, &v.state.vars);
        }
        assert!(doors_ready(&v, &p));
    }
    #[test]
    fn wheel_contacts_fit_the_same_sloping_floor_while_the_chair_turns() {
        let expected = Vec3::new(-0.10, -0.05, 1.0).normalize();
        for heading in [0.0, 90.0, 180.0, 270.0] {
            let n = contact_normal(Vec3::ZERO, heading, |p| Some(0.10 * p.x + 0.05 * p.y)).unwrap();
            assert!((n - expected).length() < 1e-5);
        }
    }
}
