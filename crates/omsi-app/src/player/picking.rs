//! Rays into a vehicle: the `[mouseevent]` meshes, the HTML pages and the trailers' switches.

use super::*;

/// The `[mouseevent]` mesh a ray picks, and whether the ray through the cursor itself found
/// it (not the rings around it). `reach`: how far along the ray through the cursor and along
/// the rings a switch may be and still be reached ([`Reach`]).
pub(crate) fn pick_in(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach) -> Option<(usize, bool)> {
    pick_in_filtered(vehicle,origin,dir,spread,reach,&|_|true)
}
pub(crate) fn pick_in_filtered(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,allowed:&impl Fn(&str)->bool)->Option<(usize,bool)> {
    let o = (origin - vehicle.position).as_vec3();
    let right = Vec3::new(-dir.y, dir.x, 0.0).normalize_or_zero();
    let up = dir.cross(right).normalize_or_zero();
    let mut rings: Vec<Vec<Vec3>> = vec![vec![dir]];
    if spread > 0.0 {
        for ring in 1..=2 {
            let r = spread * ring as f32;
            let n = 8 * ring;
            rings.push(
                (0..n)
                    .map(|k| {
                        let a = k as f32 / n as f32 * std::f32::consts::TAU;
                        (dir + right * (a.cos() * r) + up * (a.sin() * r)).normalize()
                    })
                    .collect(),
            );
        }
    }
    // The same broad-phase applies to every ring. A large cockpit can contain
    // hundreds of meshes; calculating their posed transforms three times made
    // hovering over its controls needlessly expensive.
    let candidates: Vec<(usize, glam::Mat4, Vec<u32>, &omsi_geometry::MeshData)> = vehicle.ty.meshes.iter().enumerate().filter_map(|(i, vm)| {
        if !vehicle.ty.model.meshes[vm.def_index].mouse_event.as_deref().is_some_and(allowed) || !vehicle.mesh_props[i].visible {
            return None;
        }
        let xf = vehicle.mesh_local_transform(i);
        if !ray_may_hit(&vehicle.ty, i, &xf, o, dir, spread * 2.2) {
            return None;
        }
        let data = vehicle.ty.pick_mesh_data(i)?;
        let tris = omsi_geometry::cone_triangles(o, dir, spread * 2.0 + 1e-4, data, &xf);
        (!tris.is_empty()).then_some((i, xf, tris, data))
    }).collect();
    for (ring, dirs) in rings.iter().enumerate() {
        let limit = reach.limit(ring);
        // Collect every hit in this ring, then among surfaces within a depth band of the
        // nearest prefer the mesh whose centre the ray aims at. Aachen ibox D10/D11/D12 are
        // tilted, overlap by ~6 mm, and sit ~16 mm apart along Z - a 2 mm depth-only pick
        // let backspace (D10) steal digit 0 (D11) whenever both boxes were hit.
        let mut hits: Vec<(f32, f32, usize)> = Vec::new();
        for (i, xf, tris, data) in &candidates {
            let (i, xf) = (*i, *xf);
            let lateral = vehicle
                .ty
                .mesh_boxes
                .get(i)
                .map(|&(lo, hi)| {
                    let c = (lo + hi) * 0.5;
                    let p = xf.transform_point3(c) - o;
                    let along = p.dot(dir);
                    (p - dir * along).length_squared()
                })
                .unwrap_or(0.0);
            for d in dirs {
                if let Some(t) = omsi_geometry::ray_triangles(o, *d, data, &xf, tris).filter(|&t| t <= limit) {
                    hits.push((t, lateral, i));
                }
            }
        }
        if let Some(i) = best_mouseevent_hit(&hits) {
            return Some((i, ring == 0));
        }
    }
    None
}

/// How far a switch may stand behind the nearest part that hides it, along the ray through
/// the cursor, and still take the click: a button flush with its panel, or a hair behind its
/// face where the two were modelled as one surface, stays reachable.
pub(crate) const FLUSH_SLACK: f32 = 0.01;

/// The same for the rings of rays around the cursor, measured against the part the ray
/// through the cursor meets: a ring ray lands on a switch beside it at another depth than the
/// middle ray on a tilted panel, so it is given more room; a switch modelled well behind a
/// panel (an engine hatch behind the dashboard) is still out of reach.
pub(crate) const RING_SLACK: f32 = 0.05;

/// How far along a ray into a vehicle a switch can be reached: up to the nearest part that
/// hides what is behind it (infinite: nothing does).
#[derive(Clone, Copy, Debug, PartialEq)]
pub(crate) struct Reach(pub f32);

impl Reach {
    pub(crate) const ANY: Reach = Reach(f32::INFINITY);

    /// The farthest hit ring `ring` (0: the ray through the cursor) may take.
    fn limit(self, ring: usize) -> f32 {
        self.0 + if ring == 0 { FLUSH_SLACK } else { RING_SLACK }
    }
}

/// How far a switch of body `ty` can be reached along a ray from `o` (the body's frame, `dir`
/// of unit length). From the cab nothing hides a switch, as in Omsi.exe: buses made for it
/// put seats, ticket machines and body panels in front of switches that are clicked through
/// them (the O560's stop request behind the driver's seat, the Lion's City G's roof hatches
/// behind its body). From outside (`outside`) any part of the body hides what is behind it.
pub(crate) fn reach_into(ty: &omsi_sim::VehicleType, props: &[omsi_sim::vehicle::MeshProps], xf_of: impl Fn(usize) -> glam::Mat4, o: Vec3, dir: Vec3, outside: bool) -> Reach {
    if !outside {
        return Reach::ANY;
    }
    Reach(hiding_part(ty, props, xf_of, o, dir).map_or(f32::INFINITY, |(_, t)| t))
}

/// The nearest part of a body, seen from outside, that hides what is behind it along a ray
/// (see [`reach_into`]): its mesh and how far along the ray it is. The switches themselves
/// (`[mouseevent]` meshes) are left out - among them the nearest wins anyway - and so are
/// shadow meshes, meshes the outside view does not draw (`[viewpoint]`) and slots a script
/// has faded out; walls and windows alike hide.
pub(crate) fn hiding_part(ty: &omsi_sim::VehicleType, props: &[omsi_sim::vehicle::MeshProps], xf_of: impl Fn(usize) -> glam::Mat4, o: Vec3, dir: Vec3) -> Option<(usize, f32)> {
    let mut nearest: Option<(usize, f32)> = None;
    for (i, vm) in ty.meshes.iter().enumerate() {
        let def = &ty.model.meshes[vm.def_index];
        let Some(p) = props.get(i).filter(|p| p.visible) else { continue };
        let vp = def.viewpoint;
        let drawn = vp == 0 || vp & 1 != 0;
        if def.mouse_event.is_some() || def.is_shadow || !drawn {
            continue;
        }
        let xf = xf_of(i);
        if !ray_may_hit(ty, i, &xf, o, dir, 0.0) {
            continue;
        }
        let hides = |slot: usize| p.slot_alpha.get(slot).is_none_or(|&a| a > 0.01);
        let tris: Vec<u32> = vm.data.ranges.iter().filter(|r| hides(r.2 as usize)).flat_map(|r| (r.0..r.0 + r.1).step_by(3)).collect();
        // (a face the eye stands in hides nothing)
        if let Some(t) = omsi_geometry::ray_triangles(o, dir, &vm.data, &xf, &tris).filter(|&t| t > 0.02) {
            if nearest.is_none_or(|(_, n)| t < n) {
                nearest = Some((i, t));
            }
        }
    }
    nearest
}

/// Depth band (m) inside which overlapping cockpit `[mouseevent]` meshes are disambiguated
/// by aim (lateral distance to the mesh centre) rather than by which face is a hair closer.
/// Sized for the Aachen ibox digit row (D10/D11/D12 centres ~16 mm apart on a tilted pad).
pub(crate) const MOUSEEVENT_PICK_DEPTH_BAND: f32 = 0.03;

/// Among ray hits `(t, lateral², id)`, pick the aimed mesh: nearest surface first, then the
/// smallest lateral distance among hits within [`MOUSEEVENT_PICK_DEPTH_BAND`] of that nearest.
pub(crate) fn best_mouseevent_hit(hits: &[(f32, f32, usize)]) -> Option<usize> {
    let min_t = hits.iter().map(|h| h.0).fold(f32::INFINITY, f32::min);
    if !min_t.is_finite() {
        return None;
    }
    hits.iter()
        .filter(|h| h.0 <= min_t + MOUSEEVENT_PICK_DEPTH_BAND)
        .min_by(|a, b| {
            a.1.partial_cmp(&b.1)
                .unwrap_or(std::cmp::Ordering::Equal)
                .then_with(|| a.0.partial_cmp(&b.0).unwrap_or(std::cmp::Ordering::Equal))
        })
        .map(|h| h.2)
}

#[cfg(test)]
mod mouseevent_pick_tests {
    use super::{best_mouseevent_hit, MOUSEEVENT_PICK_DEPTH_BAND};

    #[test]
    fn aachen_ibox_d11_wins_when_d10_is_closer_but_off_aim() {
        // Realistic AC O530 digit row: D10 (backspace) ~12 mm closer along the ray than
        // D11 (digit 0), but the cursor aims at D11's centre (lateral ≈ 0).
        let d10 = 0usize;
        let d11 = 1usize;
        let d12 = 2usize;
        let hits = [
            (0.488, 0.012_f32.powi(2), d10),
            (0.500, 0.001_f32.powi(2), d11),
            (0.512, 0.015_f32.powi(2), d12),
        ];
        assert!(
            (hits[1].0 - hits[0].0) > 0.002,
            "fixture must exceed the old 2 mm tie window"
        );
        assert!((hits[1].0 - hits[0].0) < MOUSEEVENT_PICK_DEPTH_BAND);
        assert_eq!(best_mouseevent_hit(&hits), Some(d11));
    }

    #[test]
    fn clearly_nearer_surface_still_wins_outside_the_band() {
        let hits = [(0.40, 0.02_f32.powi(2), 0usize), (0.50, 0.0, 1usize)];
        assert_eq!(best_mouseevent_hit(&hits), Some(0));
    }

    #[test]
    fn empty_hits_yield_none() {
        assert_eq!(best_mouseevent_hit(&[]), None);
    }
}

/// The page of an `[htmltexture]` a ray lands on: its script texture index and the place on it
/// (`u`/`v` from 0 to 1, `v` down from the top), for the nearest such surface. The nearest
/// triangle of a mesh decides: a bezel of the same mesh in front of the screen takes the click
/// away from it. `also`: script textures clicked like pages (those `.opl` plugins draw).
pub(crate) fn pick_html_in(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, also: &[usize]) -> Option<(usize, f32, f32)> {
    let pages: Vec<usize> = vehicle.html_textures.iter().map(|t| t.script_index).chain(also.iter().copied()).collect();
    if pages.is_empty() {
        return None;
    }
    let o = (origin - vehicle.position).as_vec3();
    let mut best: Option<(f32, usize, f32, f32)> = None;
    for (i, vm) in vehicle.ty.meshes.iter().enumerate() {
        if !vehicle.mesh_props[i].visible {
            continue;
        }
        let def = &vehicle.ty.model.meshes[vm.def_index];
        let shows_page = |n: Option<i32>| n.is_some_and(|n| pages.contains(&(n.max(0) as usize)));
        if !def.materials.iter().any(|m| shows_page(m.use_script_texture)) {
            continue;
        }
        let xf = vehicle.mesh_local_transform(i);
        if !ray_may_hit(&vehicle.ty, i, &xf, o, dir, 0.0) {
            continue;
        }
        let Some(data) = vehicle.ty.pick_mesh_data(i) else { continue };
        let Some(hit) = omsi_geometry::ray_mesh_hit(o, dir, data, &xf) else {
            continue;
        };
        // the page the hit material slot shows (a slot that shows none is in the way)
        let slot = data.slot_of(hit.index) as usize;
        let page = def
            .materials
            .iter()
            .filter(|m| omsi_sim::vehicle::override_slot(&vm.materials, m) == Some(slot))
            .find_map(|m| m.use_script_texture)
            .map(|n| n.max(0) as usize)
            .filter(|n| pages.contains(n));
        let Some(page) = page else { continue };
        if best.is_none_or(|b| hit.t < b.0) {
            best = Some((hit.t, page, hit.uv.x.clamp(0.0, 1.0), hit.uv.y.clamp(0.0, 1.0)));
        }
    }
    best.map(|(_, page, u, v)| (page, u, v))
}

/// The same forgiving pick as `pick`, but for the coupled sections of an articulated
/// bus.  The old picker only searched the lead vehicle, so GN92's rear door opener and
/// every other button in the second section could never receive a click.
pub(crate) fn pick_trailer_in(vehicle: &omsi_sim::VehicleInstance, origin: DVec3, dir: Vec3, spread: f32, reach: Reach) -> Option<((usize, usize), bool)> {
    pick_trailer_in_filtered(vehicle,origin,dir,spread,reach,&|_|true)
}
pub(crate) fn pick_trailer_in_filtered(vehicle:&omsi_sim::VehicleInstance,origin:DVec3,dir:Vec3,spread:f32,reach:Reach,allowed:&impl Fn(&str)->bool)->Option<((usize,usize),bool)> {
    let right = Vec3::new(-dir.y, dir.x, 0.0).normalize_or_zero();
    let up = dir.cross(right).normalize_or_zero();
    let mut rings: Vec<Vec<Vec3>> = vec![vec![dir]];
    if spread > 0.0 {
        for ring in 1..=2 {
            let r = spread * ring as f32;
            let n = 8 * ring;
            rings.push(
                (0..n)
                    .map(|k| {
                        let a = k as f32 / n as f32 * std::f32::consts::TAU;
                        (dir + right * (a.cos() * r) + up * (a.sin() * r)).normalize()
                    })
                    .collect(),
            );
        }
    }
    let candidates: Vec<(usize, usize, glam::Mat4, Vec<u32>, &omsi_geometry::MeshData)> = vehicle.trailers.iter().enumerate().flat_map(|(ti, trailer)| {
        let o = (origin - trailer.position).as_vec3();
        trailer.ty.meshes.iter().enumerate().filter_map(move |(i, vm)| {
            if !trailer.ty.model.meshes[vm.def_index].mouse_event.as_deref().is_some_and(allowed) || !trailer.mesh_props[i].visible {
                return None;
            }
            let xf = trailer.mesh_local_transform(i);
            if !ray_may_hit(&trailer.ty, i, &xf, o, dir, spread * 2.2) {
                return None;
            }
            let data = trailer.ty.pick_mesh_data(i)?;
            let tris = omsi_geometry::cone_triangles(o, dir, spread * 2.0 + 1e-4, data, &xf);
            (!tris.is_empty()).then_some((ti, i, xf, tris, data))
        })
    }).collect();
    for (ring, dirs) in rings.iter().enumerate() {
        let limit = reach.limit(ring);
        let mut hits: Vec<(f32, f32, usize)> = Vec::new();
        let mut id_of: Vec<(usize, usize)> = Vec::new();
        for (ti, i, xf, tris, data) in &candidates {
            let (ti, i, xf) = (*ti, *i, *xf);
            let trailer = &vehicle.trailers[ti];
            let o = (origin - trailer.position).as_vec3();
            let lateral = trailer
                .ty
                .mesh_boxes
                .get(i)
                .map(|&(lo, hi)| {
                    let c = (lo + hi) * 0.5;
                    let p = xf.transform_point3(c) - o;
                    let along = p.dot(dir);
                    (p - dir * along).length_squared()
                })
                .unwrap_or(0.0);
            for d in dirs {
                if let Some(t) = omsi_geometry::ray_triangles(o, *d, data, &xf, tris).filter(|&t| t <= limit) {
                    let id = id_of.len();
                    id_of.push((ti, i));
                    hits.push((t, lateral, id));
                }
            }
        }
        if let Some(id) = best_mouseevent_hit(&hits) {
            return Some((id_of[id], ring == 0));
        }
    }
    None
}

#[cfg(test)]
mod reach_tests {
    use super::{pick_in, reach_into, Reach};
    use glam::{DVec3, Mat4, Vec3};

    /// A square of 1 m across `y` = `at`, facing the origin, its one slot showing `texture`.
    fn wall(at: f32, texture: &str, mouse: bool, def_index: usize) -> (omsi_sim::vehicle::VehicleMesh, omsi_model::MeshDef) {
        let data = omsi_geometry::MeshData {
            positions: vec![Vec3::new(-0.5, at, -0.5), Vec3::new(0.5, at, -0.5), Vec3::new(0.5, at, 0.5), Vec3::new(-0.5, at, 0.5)],
            indices: vec![0, 1, 2, 0, 2, 3],
            ranges: vec![(0, 6, 0)],
            ..Default::default()
        };
        let material = omsi_o3d::Material { diffuse: [1.0; 4], specular: [0.0; 3], emissive: [0.0; 3], specular_power: 0.0, texture: texture.into() };
        let glass = omsi_model::MaterialDef { texture: texture.into(), alpha: 2, alpha_set: true, ..Default::default() };
        let mesh = omsi_sim::vehicle::VehicleMesh {
            def_index,
            data,
            pick_geometry: Default::default(),
            file: Default::default(),
            materials: vec![material],
            overrides: if texture == "glass.dds" { vec![glass.clone()] } else { Vec::new() },
            pivot: Mat4::IDENTITY,
            viewpoint: 0,
            skin: Vec::new(),
            keep_winding: false,
        };
        let def = omsi_model::MeshDef {
            mouse_event: mouse.then(|| "switch".to_string()),
            materials: if texture == "glass.dds" { vec![glass] } else { Vec::new() },
            ..Default::default()
        };
        (mesh, def)
    }

    /// A bus of a panel (`panel`: its texture) at 1 m in front of the eye and a switch at
    /// `switch` m.
    fn bus(panel: &str, switch: f32) -> omsi_sim::VehicleInstance {
        let (pm, pd) = wall(1.0, panel, false, 0);
        let (sm, sd) = wall(switch, "switch.dds", true, 1);
        let mut model = omsi_model::Model::default();
        model.meshes = vec![pd, sd];
        let ty = omsi_sim::VehicleType {
            def: Default::default(),
            model,
            model_dir: Default::default(),
            program: Default::default(),
            meshes: vec![pm, sm],
            lod_meshes: Vec::new(),
            paint_schemes: Vec::new(),
            texchanges: Vec::new(),
            wheel_meshes: Vec::new(),
            suspension_axles: Vec::new(),
            missing_packs: Vec::new(),
            mesh_bounds: Vec::new(),
            mesh_boxes: Vec::new(),
        };
        omsi_sim::VehicleInstance::new(std::sync::Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
    }

    fn picks(v: &omsi_sim::VehicleInstance, outside: bool) -> Option<usize> {
        let reach = reach_into(&v.ty, &v.mesh_props, |i| v.mesh_local_transform(i), Vec3::ZERO, Vec3::Y, outside);
        pick_in(v, v.position, Vec3::Y, 0.0, reach).map(|(i, _)| i)
    }

    /// From outside a switch behind any part of the bus cannot be clicked through it, glass
    /// too; a switch flush with its panel, or in front of it, stays reachable. From the cab
    /// nothing hides a switch, as in Omsi.exe.
    #[test]
    fn a_switch_behind_another_part_is_out_of_reach_from_outside() {
        assert_eq!(picks(&bus("dash.dds", 1.2), true), None);
        assert_eq!(picks(&bus("glass.dds", 1.2), true), None);
        assert_eq!(picks(&bus("dash.dds", 1.2), false), Some(1));
        assert_eq!(picks(&bus("glass.dds", 1.2), false), Some(1));
        assert_eq!(picks(&bus("dash.dds", 1.004), false), Some(1));
        assert_eq!(picks(&bus("dash.dds", 0.995), true), Some(1));
        assert_eq!(pick_in(&bus("dash.dds", 1.2), DVec3::ZERO, Vec3::Y, 0.0, Reach::ANY).map(|h| h.0), Some(1));
    }

    #[test]
    fn forbidden_controls_do_not_steal_the_bell_pick() {
        let mut v=bus("dash.dds",1.2);
        let ty=std::sync::Arc::make_mut(&mut v.ty);
        ty.model.meshes[0].mouse_event=Some("door_driver_switch".into());
        ty.model.meshes[1].mouse_event=Some("int_solicita_parada".into());
        assert_eq!(pick_in(&v,v.position,Vec3::Y,0.0,Reach::ANY).map(|h|h.0),Some(0));
        assert_eq!(super::pick_in_filtered(&v,v.position,Vec3::Y,0.0,Reach::ANY,&|e|e=="int_solicita_parada").map(|h|h.0),Some(1));
    }

    /// Use the real AI loader, which throws away CPU vertices after measuring/uploading
    /// a model. Earlier fixtures kept those vertices and could not reproduce the report.
    struct AiFixture(std::path::PathBuf);
    impl AiFixture {
        fn new() -> Self {
            static NEXT: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(0);
            let id = NEXT.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
            let root = std::env::temp_dir().join(format!("omsi-coop-pick-{}-{id}", std::process::id()));
            std::fs::create_dir_all(root.join("model")).unwrap();
            std::fs::create_dir_all(root.join("script")).unwrap();
            // OBJ is y-up/right-handed; (x, y, z) becomes OMSI world (x, -z, y).
            let quad = "mtllib screen.mtl\nusemtl screen\nv -0.5 -0.5 -1\nv 0.5 -0.5 -1\nv 0.5 0.5 -1\nv -0.5 0.5 -1\nvt 0 0\nvt 1 0\nvt 1 1\nvt 0 1\nf 1/1 2/2 3/3 4/4\n";
            std::fs::write(root.join("model/bell.obj"), quad).unwrap();
            std::fs::write(root.join("model/body.obj"), quad.replace(" -1", " -0.5")).unwrap();
            std::fs::write(root.join("model/screen.mtl"), "newmtl screen\nmap_Kd screen.dds\n").unwrap();
            std::fs::write(root.join("model/model.cfg"), "[mesh]\nbell.obj\n[mouseevent]\nint_solicita_parada\n[mesh]\nbody.obj\n").unwrap();
            std::fs::write(root.join("script/vars.txt"), "test_bell\n").unwrap();
            std::fs::write(root.join("script/bell.osc"), "{trigger:int_solicita_parada}\n1 (S.L.test_bell)\n(T.L.coop_test_bell)\n{end}\n").unwrap();
            std::fs::write(root.join("fixture.bus"), "[friendlyname]\nTest\nAI interaction\nTest\n[model]\nmodel/model.cfg\n[varnamelist]\n1\nscript/vars.txt\n[script]\n1\nscript/bell.osc\n").unwrap();
            Self(root)
        }
        fn vehicle(&self, ai: bool) -> omsi_sim::VehicleInstance {
            let ty = if ai { omsi_sim::VehicleType::load_ai(&self.0, &self.0.join("fixture.bus")) }
                     else { omsi_sim::VehicleType::load(&self.0, &self.0.join("fixture.bus")) }.unwrap();
            assert_eq!(ty.meshes.len(), 2);
            omsi_sim::VehicleInstance::new(std::sync::Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
        }
    }
    impl Drop for AiFixture {
        fn drop(&mut self) { let _ = std::fs::remove_dir_all(&self.0); }
    }

    #[test]
    fn actual_ai_loader_controls_are_pickable_and_only_controls_are_cached() {
        let f = AiFixture::new();
        let mut v = f.vehicle(true);
        assert!(v.ty.mesh_dropped(0) && v.ty.mesh_dropped(1));
        assert!(v.ty.meshes[0].data.positions.is_empty());
        assert!(v.ty.meshes.iter().all(|m| m.pick_geometry.get().is_none()));
        let before = v.ty.mesh_bytes();
        assert_eq!(super::pick_in_filtered(&v, v.position, Vec3::Y, 0.0, Reach::ANY,
            &crate::coop_view::passenger_event).map(|h| h.0), Some(0));
        assert!(v.ty.meshes[0].pick_geometry.get().unwrap().is_some());
        assert!(v.ty.meshes[1].pick_geometry.get().is_none(), "noninteractive fleet geometry stays dropped");
        assert!(v.ty.mesh_bytes() > before, "memory accounting includes the on-demand geometry");
        // Hovering and pressing use the same cache, rather than reading a file each frame.
        std::fs::remove_file(f.0.join("model/bell.obj")).unwrap();
        assert_eq!(pick_in(&v, v.position, Vec3::Y, 0.0, Reach::ANY).map(|h| h.0), Some(0));
        assert!(v.ty.meshes[0].data.positions.is_empty(), "GPU upload behavior remains independent");
        let event = v.ty.model.meshes[v.ty.meshes[0].def_index].mouse_event.clone().unwrap();
        assert!(v.trigger(&event), "the picked control reaches the loaded AI script");
        v.update_scripts_only(0.0);
        assert_eq!(v.var("test_bell"), Some(1.0));
        assert!(v.host.fired_triggers.iter().any(|event| event == "coop_test_bell"),
                "the real script emits its sound trigger for the audio pipeline");
    }

    #[test]
    fn restored_ai_controls_follow_the_drawn_bus_pose() {
        let f = AiFixture::new();
        let mut v = f.vehicle(true);
        v.position = DVec3::new(5635.0, 1303.0, 0.0);
        v.heading = 135.0;
        let shown = crate::coop_view::BodyPose::of(&v);
        let eye = shown.position;
        let ray = shown.rotation.transform_vector3(Vec3::Y);
        v.position += DVec3::new(0.3, -0.4, 0.05);
        v.heading = 137.0;
        let (o, d) = shown.remap_ray(crate::coop_view::BodyPose::of(&v), eye, ray);
        assert_eq!(pick_in(&v, o, d, 0.0, Reach::ANY).map(|h| h.0), Some(0));
    }

    #[test]
    fn ai_rear_section_controls_restore_their_geometry_too() {
        let f = AiFixture::new();
        let mut v = f.vehicle(true);
        let mut rear = omsi_sim::vehicle::TrailerPart::new(v.ty.clone(), &v.ty, &v.ty.program, 0);
        rear.set_pose(DVec3::new(20.0, 30.0, 0.0), 45.0);
        let o = rear.position;
        let d = rear.body_rotation().transform_vector3(Vec3::Y);
        v.trailers.push(rear);
        assert_eq!(super::pick_trailer_in_filtered(&v, o, d, 0.0, Reach::ANY,
            &crate::coop_view::passenger_event).map(|h| h.0), Some((0, 0)));
    }

    #[test]
    fn dropped_html_control_surfaces_use_restored_triangles_and_uvs() {
        let f = AiFixture::new();
        let mut v = f.vehicle(true);
        let ty = std::sync::Arc::make_mut(&mut v.ty);
        ty.model.meshes[0].materials.push(omsi_model::MaterialDef {
            texture: "screen.dds".into(), use_script_texture: Some(7), ..Default::default()
        });
        let hit = super::pick_html_in(&v, v.position, Vec3::Y, &[7]);
        assert!(hit.is_some(), "HTML surfaces need geometry too");
        let (page, u, w) = hit.unwrap();
        assert_eq!(page, 7);
        assert!((u - 0.5).abs() < 1e-4 && (w - 0.5).abs() < 1e-4);
        assert!(v.ty.meshes[1].pick_geometry.get().is_none());
    }

    #[test]
    fn full_remote_controls_still_obey_passenger_and_conductor_filters() {
        let f = AiFixture::new();
        let mut v = f.vehicle(false);
        std::sync::Arc::make_mut(&mut v.ty).model.meshes[0].mouse_event = Some("ticket_key_1".into());
        assert_eq!(super::pick_in_filtered(&v, v.position, Vec3::Y, 0.0, Reach::ANY,
            &crate::coop_view::passenger_event), None);
        assert_eq!(super::pick_in_filtered(&v, v.position, Vec3::Y, 0.0, Reach::ANY,
            &|_| true).map(|h| h.0), Some(0));
        assert!(v.ty.meshes.iter().all(|m| m.pick_geometry.get().is_none()));
    }

    #[test]
    fn picked_conductor_flap_crosses_wire_and_executes_only_on_owner() {
        let f = AiFixture::new();
        std::fs::write(f.0.join("model/model.cfg"), "[mesh]\nbell.obj\n[mouseevent]\ncp_portinha\n[mesh]\nbody.obj\n").unwrap();
        std::fs::write(f.0.join("script/vars.txt"), "flap_pos\n").unwrap();
        std::fs::write(f.0.join("script/bell.osc"), "{trigger:cp_portinha_drag}\n(L.L.flap_pos) (L.S.mouse_x) 100 / - 0 max 1 min (S.L.flap_pos)\n{end}\n{trigger:cp_portinha_off}\n{end}\n").unwrap();
        let remote = f.vehicle(false);
        let mut owner = f.vehicle(false);
        let mesh = super::pick_in_filtered(&remote, remote.position, Vec3::Y, 0.0, Reach::ANY,
            &|_| true).unwrap().0;
        let event = &remote.ty.model.meshes[remote.ty.meshes[mesh].def_index].mouse_event.clone().unwrap();
        let action = crate::protocol::Action {
            epoch: 42, seq: 1, target: crate::protocol::Target::Player, desk: 7,
            kind: 0, part: 0, index: mesh as u32, signature: 123,
            phase: crate::protocol::Phase::Down, x: 0.0, y: 0.0,
        };
        let wire = crate::protocol::Message::Action(action).encode();
        let crate::protocol::Message::Action(a) = crate::protocol::Message::parse(&wire).unwrap()
            else { panic!("not an action") };
        let grant = Some((2, 42));
        assert!(!crate::protocol::authorized(None, 2, &a, true, false, false, 8));
        assert!(crate::protocol::authorized(grant, 2, &a, true, false,
            crate::protocol::desk_bound(a.kind, Some(event)), 8));
        let press = crate::mouse_control::Press::begin(&mut owner, event).unwrap();
        let mut drag = press.release(&mut owner, event, Some("flap_pos".into()), true).unwrap();
        for _ in 0..120 { if !drag.step(&mut owner, 1.0/60.0) { break; } }
        assert!(owner.var("flap_pos").unwrap() > 0.97);
        assert_eq!(remote.var("flap_pos"), Some(0.0), "the passenger does not simulate another authoritative flap");
    }

    /// Real buses: every switch that a ray from one of the bus's cameras picks with nothing
    /// hiding it (aimed at the faces of the switch) is still picked from one of them through
    /// `reach_into` as the cab uses it. Buttons on the body's outer skin or named `_external`
    /// (the doors' outside openers, the pram request) and the SD202's ticket while it rests
    /// inside its printer are for other views and not counted. Stock buses by default; more
    /// with `OMSI_PICK_BUSES` (comma-separated `.bus` paths under the root, or absolute).
    #[test]
    #[ignore = "needs the installed content; set OMSI_ROOT"]
    fn installed_switches_stay_reachable_from_inside() {
        let Some(root) = omsi_cfg::flags::OMSI_ROOT.live_os().map(std::path::PathBuf::from) else {
            eprintln!("skipped: set OMSI_ROOT");
            return;
        };
        let mut files: Vec<String> = ["Vehicles/MAN_SD202/MAN_D92.bus", "Vehicles/MAN_NL_NG/MAN_EN92_main.bus", "Vehicles/MAN_NL_NG/MAN_GN92_main.bus"].map(String::from).to_vec();
        files.extend(std::env::var("OMSI_PICK_BUSES").unwrap_or_default().split(',').filter(|f| !f.trim().is_empty()).map(|f| f.trim().to_string()));
        let mut lost = Vec::new();
        for file in &files {
            let ty = omsi_sim::VehicleType::load(&root, &root.join(file)).unwrap_or_else(|e| panic!("{file}: {e}"));
            let mut v = omsi_sim::VehicleInstance::new(std::sync::Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()));
            // (a few frames standing: the scripts set which parts and switches show)
            for _ in 0..3 {
                v.update(1.0 / 30.0);
            }
            // (the driver's cameras and the passengers': a switch in the saloon is for the
            // views there, not for the driver behind the cab wall)
            let eyes: Vec<Vec3> = v.ty.def.cameras_driver.iter().chain(&v.ty.def.cameras_pax).map(|c| Vec3::from(c.pos)).collect();
            assert!(!v.ty.def.cameras_driver.is_empty(), "{file}: no driver camera");
            let half_width = v.ty.def.bounding_box.map(|b| b[0] * 0.5);
            let (mut switches, mut reachable) = (0, 0);
            for (i, vm) in v.ty.meshes.iter().enumerate() {
                let def = &v.ty.model.meshes[vm.def_index];
                if def.mouse_event.is_none() || !v.mesh_props.get(i).is_some_and(|p| p.visible) || vm.data.indices.len() < 3 {
                    continue;
                }
                let xf = v.mesh_local_transform(i);
                let faces = vm.data.indices.len() / 3;
                // (the middle of each face, and a point near each corner of it: what the eye
                // can see of the switch)
                let samples = faces.min(400);
                let aims: Vec<Vec3> = (0..samples)
                    .flat_map(|k| {
                        let f = k * faces / samples;
                        let corner = |c: usize| xf.transform_point3(vm.data.positions[vm.data.indices[f * 3 + c] as usize]);
                        let mid = (corner(0) + corner(1) + corner(2)) / 3.0;
                        [mid, mid.lerp(corner(0), 0.8), mid.lerp(corner(1), 0.8), mid.lerp(corner(2), 0.8)]
                    })
                    .collect();
                let picked = |with_parts: bool| {
                    eyes.iter().any(|&eye| {
                        aims.iter().any(|&aim| {
                            let dir = (aim - eye).normalize_or_zero();
                            let reach = if with_parts { reach_into(&v.ty, &v.mesh_props, |m| v.mesh_local_transform(m), eye, dir, false) } else { Reach::ANY };
                            pick_in(&v, v.position + eye.as_dvec3(), dir, 0.0, reach).is_some_and(|h| h.0 == i)
                        })
                    })
                };
                let name = def.mouse_event.clone().unwrap_or_default();
                // (on the body's outer skin, or wider: a button for outside)
                let side = aims.iter().map(|a| a.x.abs()).sum::<f32>() / aims.len() as f32;
                let outside = half_width.is_some_and(|h| side >= h - 0.03) || name.ends_with("_external");
                if !picked(false) || outside || name == "ticketprinter_getticket" {
                    continue;
                }
                switches += 1;
                if picked(true) {
                    reachable += 1;
                } else {
                    // (what hides it, along the first aim from the first camera)
                    let (eye, aim) = (eyes[0], aims[0]);
                    let dir = (aim - eye).normalize_or_zero();
                    let hider = super::hiding_part(&v.ty, &v.mesh_props, |m| v.mesh_local_transform(m), eye, dir);
                    let what = hider.map(|(m, t)| {
                        let file = v.ty.meshes[m].file.file_name().map(|f| f.to_string_lossy().into_owned()).unwrap_or_default();
                        format!("{file} at {t:.3} m, switch at {:.3} m", (aim - eye).length())
                    });
                    lost.push(format!("{file}: {} behind {}", def.mouse_event.clone().unwrap_or_default(), what.unwrap_or_default()));
                }
            }
            eprintln!("{file}: {reachable} of {switches} switches reachable from inside");
        }
        assert!(lost.is_empty(), "switches lost to the parts in front of them: {lost:#?}");
    }
}
