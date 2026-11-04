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
        && !["painel", "panel", "adesivo", "luz", "light", "display"]
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
    ty.model_dir.hash(&mut h);
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
    let d = discover(v, c, key);
    if let Ok(mut cached) = cache.lock() {
        if cached.len() >= 128 {
            cached.clear();
        }
        cached.insert(key, d.clone());
    }
    d
}
fn deck(ty: &VehicleType, index: usize) -> Option<NativeDeck> {
    let m = &ty.meshes[index];
    let d = &ty.model.meshes[m.def_index];
    let (lo, hi) = ty.mesh_boxes[index];
    let size = hi - lo;
    if size.y < 0.78 || !(0.55..=2.2).contains(&size.x) || size.z > 0.20 {
        return None;
    }
    if !m.data.indices.chunks_exact(3).any(|t| {
        let p = t.map_positions(&m.data.positions);
        (p[1] - p[0]).cross(p[2] - p[0]).normalize_or_zero().z.abs() > 0.98
    }) {
        return None;
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
        corners: [
            Vec3::new(lo.x, lo.y, hi.z),
            Vec3::new(hi.x, lo.y, hi.z),
            Vec3::new(hi.x, hi.y, hi.z),
            Vec3::new(lo.x, hi.y, hi.z),
        ],
        thickness: size.z,
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
                "visible",
            ]
            .iter()
            .any(|k| n.contains(k))
            && ty.program.var(&n).is_some_and(|id| ty.program.stores(id))
    });
    if natives.is_empty() && reserved.is_empty() && out.static_equipment.is_empty() {
        out.reasons
            .push("No accessible equipment or reserved-space evidence in the active model".into());
        return out;
    }
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
        let (width, vars) = door_binding(ty, d.inside.y, floor);
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
        if native.is_none() && (!hint || unbound_native || !out.native_meshes.is_empty()) {
            continue;
        }
        let mut threshold = Vec3::new(d.side * half, d.inside.y, floor);
        if let Some(n) = &native {
            threshold.x = if d.side > 0.0 {
                n.corners[2].x
            } else {
                n.corners[0].x
            };
            threshold.y = (n.corners[0].y + n.corners[2].y) * 0.5;
            threshold.z = n.corners[0].z.max(d.inside.z);
            if (threshold.z - floor).abs() > 0.18 {
                continue;
            }
        }
        let bays = find_bays(c, &geometry, threshold, &reserved, &static_boxes);
        let count = bays.len();
        let selected = bays.into_iter().find_map(|(bay, heading)| {
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
            routes
                .into_iter()
                .find(|r| geometry.route_clear(threshold, r, bay, heading))
                .map(|route| (bay, heading, route))
        });
        let Some((bay, heading, route)) = selected else {
            out.reasons.push(format!(
                "Door y={:.2}: {count} candidate bays, no verified turning route",
                threshold.y
            ));
            continue;
        };
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
            native,
        };
        if let Err(e) = p.validate() {
            out.reasons.push(e);
            continue;
        }
        let score =
            (bay - threshold).truncate().length() + if p.native.is_some() { 0.0 } else { 4.0 };
        choices.push((score, p));
    }
    choices.sort_by(|a, b| a.0.total_cmp(&b.0));
    out.profile = choices.into_iter().next().map(|(_, p)| p);
    if out.profile.is_none() && out.reasons.is_empty() {
        out.reasons.push("Native equipment could not be bound to a verified door and route; no duplicate equipment added".into());
    }
    out
}
fn door_binding(ty: &VehicleType, y: f32, floor: f32) -> (f32, Vec<String>) {
    let mut low = f32::INFINITY;
    let mut high = f32::NEG_INFINITY;
    let mut vars = Vec::new();
    for (i, m) in ty.meshes.iter().enumerate() {
        let (a, b) = ty.mesh_boxes[i];
        if ((a.y + b.y) * 0.5 - y).abs() > 0.85 || a.z > floor + 0.8 || b.z < floor + 0.8 {
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
        let mut cells: HashMap<_, Vec<usize>> = HashMap::new();
        let mut large = Vec::new();
        for (i, t) in triangles.iter().enumerate() {
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
        self.floor
            .iter()
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
                (u >= -0.001 && w >= -0.001 && u + w <= 1.001 && (z - p.z).abs() < 0.15)
                    .then_some(z)
            })
            .max_by(f32::total_cmp)
    }
    fn chair_clear(&self, p: Vec3, heading: f32) -> bool {
        let (s, c) = heading.to_radians().sin_cos();
        let forward = Vec3::new(s, c, 0.0);
        let right = Vec3::new(c, -s, 0.0);
        // Wheels, footrests, legs, arms, torso, head and back frame.
        [
            (0.0, 0.33, Vec3::new(0.38, 0.32, 0.30)),
            (0.49, 0.145, Vec3::new(0.29, 0.20, 0.105)),
            (0.36, 0.45, Vec3::new(0.22, 0.21, 0.28)),
            (0.025, 0.80, Vec3::new(0.38, 0.25, 0.20)),
            (-0.04, 0.88, Vec3::new(0.24, 0.22, 0.30)),
            (-0.025, 1.22, Vec3::new(0.15, 0.17, 0.18)),
            (-0.26, 0.77, Vec3::new(0.29, 0.12, 0.34)),
        ]
        .into_iter()
        .all(|(y, z, half)| {
            let centre = p + forward * y + Vec3::Z * z;
            let ext = right.abs() * half.x + forward.abs() * half.y + Vec3::Z * half.z;
            !self.hit(centre - ext, centre + ext, |t| {
                triangle_box(
                    t.p.map(|v| {
                        let v = v - centre;
                        Vec3::new(v.dot(right), v.dot(forward), v.z)
                    }),
                    Vec3::ZERO,
                    half,
                )
            })
        })
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
        for ix in 0..7 {
            for iy in -32_i32..=32 {
                if side == threshold.x.signum() && iy.abs() < 21 {
                    continue;
                }
                let mut p = Vec3::new(
                    side * (0.25 + ix as f32 * 0.09),
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
    let normal = (corners[1] - corners[0])
        .cross(corners[3] - corners[0])
        .normalize_or_zero();
    let world = v.position + v.body_rotation().transform_vector3(outer).as_dvec3();
    let contact = v
        .ground
        .as_ref()
        .and_then(|g| g(world.x, world.y))
        .is_some_and(|z| (world.z - z).abs() < 0.10);
    let ground_ready = extension > 0.45 && contact && normal.z.abs() > 0.94;
    let floor_ready = ground_ready
        && (inner.z - p.threshold.z).abs() < 0.075
        && (inner.x - p.threshold.x).abs() < 0.15;
    let stowed = extension < 0.12
        && v.animators.get(i).is_some_and(|m| {
            m.anims.iter().all(|(a, _, s)| {
                s.value.abs()
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
            vec![Triangle {
                p,
                low: p[0].min(p[1]).min(p[2]),
                high: p[0].max(p[1]).max(p[2]),
            }],
        );
        let start = Vec3::new(1.25, -1.02, 0.0);
        let bay = Vec3::new(-0.60, -0.88, 0.0);
        assert!(g.route_clear(start, &[start.lerp(bay, 0.5)], bay, 180.0));
        assert!(!g.route_clear(
            start,
            &[Vec3::new(0.0, -1.02, 0.0), Vec3::new(0.0, -0.88, 0.0)],
            bay,
            180.0
        ));
    }
}
