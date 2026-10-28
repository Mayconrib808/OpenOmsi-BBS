//! Live vehicle state; applying never runs init, physics, fuel pumps or scripts.
use omsi_sim::VehicleInstance;
use serde::{Deserialize, Serialize};
use std::io::Read;
#[derive(Clone, Debug, Serialize, Deserialize)]
pub struct Snapshot {
    pub uid: u64,
    pub bus: String,
    pub paint: Option<String>,
    pub vars: Vec<(String, f32)>,
    pub strings: Vec<(String, String)>,
    pub pos: [f64; 3],
    pub heading: f64,
    pub pitch: f32,
    pub bank: f32,
    pub physics: omsi_sim::physics::VehiclePhysics,
    pub clock: (i32, i32, f64, f32, bool, f64),
    pub vm: ([f32; 8], [f32; 10], [String; 8], u64),
    pub rigid_state: Option<omsi_sim::rigid::RigidBody>,
    pub trailer_state: Vec<omsi_sim::vehicle::TrailerHandover>,
    pub speed: f32,
    pub steer: f32,
    pub odometer: f64,
    pub dirt: f32,
    pub crashes: u32,
    pub rigid: Option<([f64; 3], [f32; 4], [f32; 3], [f32; 3])>,
    pub trailers: Vec<([f64; 3], f64)>,
    pub images: Vec<(u32, u32, bool, Vec<u8>)>,
    pub timetable: (String, f32, Vec<(String, f32, f32)>, Vec<i64>, i32, i32),
}
impl Snapshot {
    pub fn capture(uid: u64, bus: String, v: &VehicleInstance) -> Self {
        Self::capture_stream(uid, bus, v, true)
    }
    /// Steady-state updates omit unchanged texture pixels. Commits/returns always
    /// retain the complete state; omitted images never clear the recipient's screens.
    pub fn capture_stream(uid: u64, bus: String, v: &VehicleInstance, images: bool) -> Self {
        let mut vars: Vec<_> = v
            .var_index()
            .iter()
            .map(|(n, i)| (n.clone(), v.state.vars[*i as usize]))
            .collect();
        vars.sort_by(|a, b| a.0.cmp(&b.0));
        let h = &v.host;
        Self {
            uid,
            bus,
            paint: h
                .paint_scheme
                .flatten()
                .and_then(|i| v.ty.paint_schemes.get(i))
                .map(|s| s.name.clone()),
            vars,
            strings: v
                .ty
                .program
                .str_var_names
                .iter()
                .enumerate()
                .filter_map(|(i, n)| v.state.str_vars.get(i).map(|s| (n.clone(), s.clone())))
                .collect(),
            physics: v.physics.clone(),
            clock: (
                h.clock.year,
                h.clock.day_of_year,
                h.clock.time,
                h.clock.timegap,
                h.clock.paused,
                h.clock.run_time,
            ),
            vm: v.vm.export_handover(),
            rigid_state: v.rigid.clone(),
            trailer_state: v.trailers.iter().map(|t| t.export_handover()).collect(),
            pos: v.position.to_array(),
            heading: v.heading,
            pitch: v.pitch,
            bank: v.bank,
            speed: v.physics.speed,
            steer: v.physics.steer_deg,
            odometer: v.odometer_km(),
            dirt: v.dirt,
            crashes: v.crashes,
            rigid: v.rigid.as_ref().map(|r| {
                (
                    r.position.to_array(),
                    r.orientation.to_array(),
                    r.velocity.to_array(),
                    r.omega.to_array(),
                )
            }),
            trailers: v
                .trailers
                .iter()
                .map(|t| (t.position.to_array(), t.heading))
                .collect(),
            images: if images { h.script_textures.iter()
                .map(|t| (t.width, t.height, t.mipmaps, t.rgba.clone())).collect()
            } else { Vec::new() },
            timetable: (
                h.tt_line.clone(),
                h.tt_delay,
                h.tt_stops.clone(),
                h.tt_stop_ids.clone(),
                h.tt_terminus_index,
                h.tt_busstop_index,
            ),
        }
    }
    pub fn valid(&self) -> bool {
        self.uid != 0
            && !self.bus.is_empty()
            && self.bus.len() < 1024
            && self.vars.len() < 65536
            && self.strings.len() < 65536
            && self.pos.iter().all(|v| v.is_finite())
            && self.heading.is_finite()
            && self.pitch.is_finite()
            && self.bank.is_finite()
            && self.speed.is_finite()
            && self.steer.is_finite()
            && self.odometer.is_finite()
            && self.dirt.is_finite()
            && self
                .vars
                .iter()
                .all(|(n, v)| n.len() < 256 && v.is_finite())
            && self
                .strings
                .iter()
                .all(|(n, v)| n.len() < 256 && v.len() < 131072)
            && self.trailers.len() < 32
            && self
                .trailers
                .iter()
                .all(|(p, h)| p.iter().all(|v| v.is_finite()) && h.is_finite())
            && self.rigid.as_ref().is_none_or(|(p, q, v, o)| {
                p.iter().all(|v| v.is_finite()) && q.iter().chain(v).chain(o).all(|v| v.is_finite())
            })
            && self.images.len() <= 64
            && self.images.iter().map(|i| i.3.len()).sum::<usize>() <= 16 * 1024 * 1024
            && self.images.iter().all(|(w, h, _, p)| {
                *w <= 4096 && *h <= 4096 && p.len() == (*w as usize) * (*h as usize) * 4
            })
    }
    pub fn apply(&self, v: &mut VehicleInstance) -> bool {
        self.apply_inner(v, false)
    }
    pub fn apply_stream(&self, v: &mut VehicleInstance) -> bool {
        self.apply_inner(v, true)
    }
    fn apply_inner(&self, v: &mut VehicleInstance, stream: bool) -> bool {
        if !self.valid()
            || self.trailers.len() != v.trailers.len()
            || (!(stream && self.images.is_empty()) && self.images.len() != v.host.script_textures.len())
        {
            return false;
        }
        if self.physics.wheels.len() != v.physics.wheels.len()
            || self.trailer_state.len() != v.trailers.len()
            || !self
                .trailer_state
                .iter()
                .zip(&v.trailers)
                .all(|(s, t)| t.accepts_handover(s))
        {
            return false;
        }
        if self
            .rigid_state
            .as_ref()
            .is_some_and(|r| r.wheels.len() > 128 || r.coupled.len() > 32)
        {
            return false;
        }
        let names: std::collections::HashSet<_> =
            self.vars.iter().map(|(n, _)| n.as_str()).collect();
        if v.ty
            .program
            .var_names
            .iter()
            .any(|n| !names.contains(n.to_ascii_lowercase().as_str()))
        {
            return false;
        }
        let paint = match &self.paint {
            None => None,
            Some(name) => match v
                .ty
                .paint_schemes
                .iter()
                .position(|s| s.name.eq_ignore_ascii_case(name))
            {
                Some(i) => Some(i),
                None => return false,
            },
        };
        // All validation precedes the first write; incompatible scripts never partly apply.
        for (n, value) in &self.vars {
            v.set_engine_var(n, *value);
        }
        for (n, value) in &self.strings {
            if let Some(i) = v.ty.program.str_var(n) {
                v.state.str_vars[i as usize] = value.clone();
            }
        }
        for t in &mut v.text_textures {
            t.last_text = None;
        }
        v.physics = self.physics.clone();
        v.rigid = self.rigid_state.clone();
        v.vm.import_handover(&self.vm);
        let (year, day, time, gap, paused, run) = self.clock;
        v.host.clock = omsi_sim::SimClock {
            year,
            day_of_year: day,
            time,
            timegap: gap,
            paused,
            run_time: run,
        };
        for (t, s) in v.trailers.iter_mut().zip(&self.trailer_state) {
            t.import_handover(s);
        }

        v.position = glam::DVec3::from_array(self.pos);
        v.heading = self.heading;
        v.pitch = self.pitch;
        v.bank = self.bank;
        v.physics.speed = self.speed;
        v.physics.steer_deg = self.steer;
        v.set_odometer_km(self.odometer);
        v.dirt = self.dirt;
        v.crashes = self.crashes;
        if let (Some((p, q, vel, o)), Some(r)) = (&self.rigid, v.rigid.as_mut()) {
            r.position = glam::DVec3::from_array(*p);
            r.orientation = glam::Quat::from_array(*q);
            r.velocity = glam::Vec3::from_array(*vel);
            r.omega = glam::Vec3::from_array(*o);
        }
        v.host.paint_scheme = Some(paint);
        v.apply_paint_vars(paint);
        for (t, (w, h, m, p)) in v.host.script_textures.iter_mut().zip(&self.images) {
            t.apply_network(*w, *h, *m, p);
        }
        let h = &mut v.host;
        let (line, delay, stops, ids, term, stop) = &self.timetable;
        h.tt_line = line.clone();
        h.tt_delay = *delay;
        h.tt_stops = stops.clone();
        h.tt_stop_ids = ids.clone();
        h.tt_terminus_index = *term;
        h.tt_busstop_index = *stop;
        true
    }
}
#[derive(Clone, Debug, Serialize, Deserialize)]
pub enum Message {
    Work {
        token: u64,
        actor: u32,
        wire: String,
    },
    Request {
        nonce: u64,
    },
    Offer {
        nonce: u64,
        token: u64,
        uid: u64,
        bus: String,
        hof: Option<String>,
        paint: Option<String>,
    },
    Prepared {
        token: u64,
    },
    Commit {
        token: u64,
        state: Snapshot,
    },
    Active {
        token: u64,
    },
    State {
        token: u64,
        desk: u32,
        state: Snapshot,
        sounds: Vec<(String, Option<String>, Option<Vec<f32>>)>,
        coins: Vec<usize>,
        ticket: bool,
        change: bool,
        take_change: bool,
    },
    Context {
        token: u64,
        desk: u32,
        tt: (String, f32, Vec<(String, f32, f32)>, Vec<i64>, i32, i32),
        schedule: f32,
        vars: Vec<(String, f32)>,
        humans: f32,
        paths: Vec<u32>,
        seats: Vec<u32>,
        triggers: Vec<String>,
    },
    Suspended {
        token: u64,
        state: Snapshot,
    },
    Return {
        token: u64,
        state: Snapshot,
    },
    Returned {
        token: u64,
    },
    Heartbeat {
        token: u64,
    },
    Abort {
        token: u64,
    },
    WorkResult {
        token: u64,
        actor: u32,
        epoch: u64,
        seq: u64,
        ok: bool,
    },
    Authority { token: u64, driver: u32, active: bool },
    DeskContext { token: u64, desk: crate::coop::Desk },
    PassengerWork { token: u64, actor: u32, wire: String },
    PassengerResult { token: u64, actor: u32, epoch: u64, seq: u64, ok: bool },
    CashView { token: u64, cash: crate::coop::CashView },
}
// Status metadata only; simulation authority still needs token-bound handover.
// The logical owner is always the transport sender, never a supplied field.
pub fn authority_notice(token:u64,driver:u32,active:bool)->String {
 format!("COOPAUTH1 {token:x} {driver} {}",u8::from(active))
}
pub fn parse_authority_notice(text:&str)->Option<(u64,u32,bool)> {
 let mut p=text.strip_prefix("COOPAUTH1 ")?.split_whitespace();
 let token=u64::from_str_radix(p.next()?,16).ok()?;
 let driver=p.next()?.parse().ok()?;
 let active=match p.next()? {"1"=>true,"0"=>false,_=>return None};
 (p.next().is_none()&&token!=0&&driver!=0).then_some((token,driver,active))
}
pub fn encode(m: &Message) -> Option<Vec<u8>> {
    let raw = rmp_serde::to_vec(m).ok()?;
    if raw.len() > 64 * 1024 * 1024 {
        return None;
    }
    let zip = zstd::stream::encode_all(raw.as_slice(), 1).ok()?;
    (zip.len() <= 512 * 1024).then_some(zip)
}
pub fn decode(data: &[u8]) -> Option<Message> {
    if data.len() > 512 * 1024 {
        return None;
    }
    let mut out = Vec::new();
    zstd::stream::read::Decoder::new(data)
        .ok()?
        .take(64 * 1024 * 1024 + 1)
        .read_to_end(&mut out)
        .ok()?;
    if out.len() > 64 * 1024 * 1024 {
        return None;
    }
    rmp_serde::from_slice(&out).ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    #[test]
    fn execution_results_and_passenger_context_survive_the_wire() {
        let m=Message::PassengerResult { token:71,actor:2,epoch:5,seq:9,ok:false };
        assert!(matches!(decode(&encode(&m).unwrap()),Some(Message::PassengerResult { token:71,actor:2,epoch:5,seq:9,ok:false })));
        let m=Message::DeskContext { token:71,desk:crate::coop::Desk { id:44,stage:3,paid:10.,due:5.,name:"Inteira".into(),..Default::default() } };
        let Some(Message::DeskContext{desk,..})=decode(&encode(&m).unwrap()) else { panic!("lost desk"); };
        assert_eq!(desk.id,44);assert_eq!(desk.stage,3);assert_eq!(desk.due,5.);
    }
    #[test]
    fn authority_notice_has_no_claimed_owner_or_simulation_input() {
        let text=authority_notice(71,3,true);
        assert_eq!(parse_authority_notice(&text),Some((71,3,true)));
        for bad in ["COOPAUTH1 0 3 1","COOPAUTH1 47 0 1","COOPAUTH1 47 3 2",
            "COOPAUTH1 47 3 1 extra","COOPAUTH1 47 3 1 speed 99"] {
            assert!(parse_authority_notice(bad).is_none());
        }
    }
    fn bus() -> VehicleInstance {
        let program = omsi_script::compile(&omsi_script::CompileInput {
            builtin_vars: vec!["fuel".into(), "door".into(), "PAX_Entry0_Req".into()],
            builtin_str_vars: vec!["number".into(), "destination".into()],
            ..Default::default()
        });
        let ty = omsi_sim::VehicleType {
            def: Default::default(),
            model: Default::default(),
            model_dir: Default::default(),
            program: Arc::new(program),
            meshes: vec![],
            lod_meshes: vec![],
            paint_schemes: vec![],
            texchanges: vec![],
            wheel_meshes: vec![],
            suspension_axles: vec![],
            missing_packs: vec![],
            mesh_bounds: vec![],
            mesh_boxes: vec![],
        };
        VehicleInstance::new(Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
    }
    #[test]
    fn streaming_pose_keeps_existing_screen_and_complete_commits_stay_strict() {
        let mut a = bus();
        let mut b = bus();
        a.host.script_textures.push(omsi_sim::scripttex::ScriptTexture::new(1, 1));
        b.host.script_textures.push(omsi_sim::scripttex::ScriptTexture::new(1, 1));
        b.host.script_textures[0].rgba = vec![1, 2, 3, 255];
        a.position.x = 123.0;
        let state = Snapshot::capture_stream(9, "Vehicles/test.bus".into(), &a, false);
        assert!(state.images.is_empty());
        assert!(!state.apply(&mut b), "a partial state cannot activate a new driver");
        assert!(state.apply_stream(&mut b));
        assert_eq!(b.position.x, 123.0);
        assert_eq!(b.host.script_textures[0].rgba, vec![1, 2, 3, 255]);
        a.host.script_textures[0].rgba = vec![7, 8, 9, 255];
        let changed = Snapshot::capture_stream(9, "Vehicles/test.bus".into(), &a, true);
        assert!(changed.apply_stream(&mut b));
        assert_eq!(b.host.script_textures[0].rgba, vec![7, 8, 9, 255]);
    }
    #[test]
    fn physical_cash_survives_wire_with_exact_positions_and_rejects_bad_coordinates() {
        use crate::coop::{CashView, CashPiece};
        let cash = CashView { desk: 44, pieces: vec![CashPiece {
            coin: 2, local: glam::Mat4::from_translation(glam::Vec3::new(0.5, 2.0, 1.2)).to_cols_array(),
            change: false, parent: Some("moving_cashdesk".into()),
        }] };
        assert!(cash.valid());
        let Some(Message::CashView {token: 71, cash: read}) = decode(&encode(&Message::CashView {token: 71,cash: cash.clone()}).unwrap()) else { panic!("lost physical cash"); };
        assert_eq!(read, cash);
        let mut bad = cash;
        bad.pieces[0].local[12] = f32::NAN;
        assert!(!bad.valid());
    }
    #[test]
    fn full_state_preserves_vehicle_scripts_clock_and_physics_without_a_frame() {
        let mut a = bus();
        a.set_var("fuel", 34.5);
        a.set_var("door", 1.0);
        a.set_odometer_km(12345.67);
        a.position = glam::DVec3::new(1234., 5678., 90.);
        a.heading = 123.;
        a.physics.speed = 0.1;
        a.vm.stacks.reg[2] = 19.;
        a.host.clock.run_time = 1234.5;
        let i = a.ty.program.str_var("destination").unwrap() as usize;
        a.state.str_vars[i] = "Linha 123".into();
        let state = Snapshot::capture(9, "Vehicles/test.bus".into(), &a);
        assert!(state.valid());
        let mut b = bus();
        assert!(state.apply(&mut b));
        assert_eq!(b.var("fuel"), Some(34.5));
        assert_eq!(b.var("door"), Some(1.));
        assert_eq!(b.position, a.position);
        assert_eq!(b.heading, a.heading);
        assert_eq!(b.physics.speed, 0.1);
        assert_eq!(b.odometer_km(), 12345.67);
        assert_eq!(b.vm.stacks.reg[2], 19.);
        assert_eq!(b.host.clock.run_time, 1234.5);
        assert_eq!(b.state.str_vars[i], "Linha 123");
        let wire = encode(&Message::Commit { token: 71, state }).unwrap();
        let Some(Message::Commit { token, state }) = decode(&wire) else {
            panic!("not decoded")
        };
        assert_eq!(token, 71);
        assert!(state.apply(&mut b));
        assert_eq!(b.var("fuel"), Some(34.5));
    }
    #[test]
    fn coop15_0343_suspension_state_survives_the_driver_transfer_wire() {
        let mut donor=bus();
        let mut def=omsi_vehicle::Vehicle {mass:10.,moment_of_inertia:[600.,90.,400.],cog_height:1.2,..Default::default()};
        def.axles.push(omsi_vehicle::Axle {long:2.,max_width:2.4,min_width:1.76,wheel_diameter:0.94,spring:240.,max_force:90.,damper:20.,driven:true,inertia_inv:0.002});
        let mut rigid=omsi_sim::rigid::RigidBody::from_definition(&def,&[]);
        assert!(!rigid.wheels.is_empty());
        rigid.wheels[0].strut.kind=omsi_vehicle::vehicle::SuspensionKind::Air;
        rigid.wheels[0].strut.preload=1234.5;
        donor.rigid=Some(rigid);
        let state=Snapshot::capture(17,"Vehicles/test.bus".into(),&donor);
        let wire=encode(&Message::Commit{token:72,state}).unwrap();
        let Some(Message::Commit{state,..})=decode(&wire)else{panic!("missing suspension snapshot")};
        let mut recipient=bus();
        assert!(state.apply(&mut recipient));
        let strut=recipient.rigid.as_ref().unwrap().wheels[0].strut;
        assert_eq!(strut.kind,omsi_vehicle::vehicle::SuspensionKind::Air);
        assert_eq!(strut.preload,1234.5);
    }
    #[test]
    fn coop13_handover_resolves_paint_by_name_instead_of_foreign_cti_index() {
        let mut donor = bus();
        Arc::make_mut(&mut donor.ty).paint_schemes = vec![
            omsi_sim::vehicle::PaintScheme { name:"EMTU".into(), dir:Default::default(), textures:vec![], set_vars:vec![] },
            omsi_sim::vehicle::PaintScheme { name:"Transfort".into(), dir:Default::default(), textures:vec![], set_vars:vec![] },
        ];
        donor.host.paint_scheme = Some(Some(0));
        donor.apply_paint_vars(Some(0));
        let state = Snapshot::capture(9, "Vehicles/test.bus".into(), &donor);
        let mut recipient = bus();
        Arc::make_mut(&mut recipient.ty).paint_schemes = donor.ty.paint_schemes.iter().rev().cloned().collect();
        assert!(state.apply(&mut recipient));
        assert_eq!(recipient.host.paint_scheme, Some(Some(1)));
        assert_eq!(recipient.var("Colorscheme"), Some(1.0));
        assert_eq!(recipient.ty.paint_schemes[1].name, "EMTU");
    }
    #[test]
    fn incompatible_or_nonfinite_snapshots_never_partly_modify_the_bus() {
        let a = bus();
        let mut b = bus();
        b.set_var("fuel", 22.);
        let mut state = Snapshot::capture(9, "Vehicles/test.bus".into(), &a);
        state.vars.iter_mut().find(|(n, _)| n == "fuel").unwrap().1 = f32::NAN;
        assert!(!state.apply(&mut b));
        assert_eq!(b.var("fuel"), Some(22.));
        let mut state = Snapshot::capture(9, "Vehicles/test.bus".into(), &a);
        state.vars.retain(|(n, _)| n != "door");
        assert!(!state.apply(&mut b));
        assert_eq!(b.var("fuel"), Some(22.));
        assert!(decode(&[1, 2, 3]).is_none());
    }
}
