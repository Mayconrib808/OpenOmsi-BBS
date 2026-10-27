//! Friendly greetings between oncoming buses. Presentation only; no driving or timetable changes.
use crate::VehicleInstance;
use glam::{DVec2, DVec3};
use std::collections::{HashMap, HashSet};

pub const HORN_EVENT: &str = "bbs_greeting_horn";
pub const FLASH_EVENT: &str = "bbs_greeting_flash";
const HIGH_VARS: &[&str] = &["lights_fern", "lights_fernlicht", "farol_alto", "farolalto", "luz_alta", "luzalta", "high_beam", "highbeam", "fernlicht"];

pub fn horn_active(v: &VehicleInstance) -> bool {
    ["cockpit_hupe", "cockpit_hupe_swheel", "cockpit_hupe_volume", "horn", "hupe", "buzina", "buzina_vol", "buzina_volume", "horn_active", "bbs_input_horn"]
        .iter().any(|name| v.var(name).is_some_and(|n| n > 0.5))
}

pub fn high_active(v: &VehicleInstance) -> bool {
    HIGH_VARS.iter().any(|n| v.var(n).is_some_and(|n| n > 0.5))
        || (v.ty.model.spotlights.len() >= 2 && v.var("Spot_Select").is_some_and(|n| n >= 1.0))
}
fn lamp_names(v: &VehicleInstance) -> Vec<&'static str> {
    HIGH_VARS.iter().copied().chain(std::iter::once("Spot_Select"))
        .filter(|n| v.var(n).is_some()).collect()
}
fn pulse(v: &mut VehicleInstance, saved: &[(&str,f32)], on: bool) {
    for &(name, original) in saved {
        let high = if name == "Spot_Select" && v.ty.model.spotlights.len() < 2 { original } else { 1.0 };
        v.set_var(name, if on { high.max(original) } else { original });
    }
    let names: Vec<_> = saved.iter().map(|(n,_)| *n).collect();
    v.refresh_external_lamp_signals(&names);
}
/// The host's one-shot flash event is independent of whether audio clips are loaded.
pub fn start_remote_flash(v: &mut VehicleInstance) {
    restore_remote_flash(v);
    for name in lamp_names(v) {
        let value = v.var(name).unwrap_or(0.0);
        v.set_engine_var(&format!("bbs_saved_{name}"), value);
    }
    v.set_engine_var("bbs_flash_remaining", 0.7);
}
pub fn restore_remote_flash(v: &mut VehicleInstance) {
    if v.var("bbs_flash_remaining").unwrap_or(0.0) <= 0.0 { return; }
    let saved: Vec<_> = lamp_names(v).into_iter().filter_map(|n| v.var(&format!("bbs_saved_{n}")).map(|x|(n,x))).collect();
    pulse(v,&saved,false);
}
pub fn step_remote_flash(v: &mut VehicleInstance, dt: f32) {
    let remaining = v.var("bbs_flash_remaining").unwrap_or(0.0);
    if remaining <= 0.0 || !dt.is_finite() || dt < 0.0 { return; }
    // Scripts have just run from restored inputs: preserve their current lighting state.
    let saved: Vec<_> = lamp_names(v).into_iter().map(|n| (n,v.var(n).unwrap_or(0.0))).collect();
    for &(n,x) in &saved { v.set_engine_var(&format!("bbs_saved_{n}"),x); }
    let elapsed=0.7-remaining;
    pulse(v,&saved,elapsed < 0.2 || (0.35..0.55).contains(&elapsed));
    let next=(remaining-dt).max(0.0);
    if next == 0.0 { pulse(v,&saved,false); }
    v.set_engine_var("bbs_flash_remaining", next);
}
const RANGE: f64 = 65.0;
const COOLDOWN: f32 = 10.0;
const FLASH_MAX: f32 = 0.8;

#[derive(Clone, Copy, Debug, Eq, PartialEq, Ord, PartialOrd, Hash)]
pub enum BusId { Ai(u64), Player(u32) }

#[derive(Clone)]
pub struct Bus {
    pub id: BusId,
    pub position: DVec3,
    pub heading: f64,
    pub speed: f32,
    pub line: String,
    pub horn: bool,
    pub high_beam: bool,
}

pub fn line_key(line: &str) -> String {
    let value: String = line.chars().filter(|c| !c.is_whitespace()).flat_map(char::to_uppercase).collect();
    if value.is_empty() || value.chars().all(|c| c == '0' || c == '-') { return String::new(); }
    // Numeric leading zeros are display padding; preserve every letter and branch suffix.
    if value.bytes().all(|b| b.is_ascii_digit()) { value.trim_start_matches('0').to_string() } else { value }
}

impl Bus {
    pub fn from_vehicle(id: BusId, v: &VehicleInstance, line: &str, horn: bool) -> Self {
        let line = if !line.trim().is_empty() { line.to_string() }
            else if !v.host.tt_line.trim().is_empty() { v.host.tt_line.clone() }
            else {
                let shown = [v.str_var("Matrix_Nr"), v.str_var("SetLineTo"), v.str_var("IBIS_Linie")]
                    .into_iter().find(|s| !s.trim().is_empty());
                shown.unwrap_or_else(|| {
                    // Do not silently merge 92E and 92 when only a numeric IBIS is available.
                    if v.var("IBIS_Linie_Suffix").is_some_and(|s| s > 0.5) { return String::new(); }
                    v.var("IBIS_Linie_Complex").filter(|n| n.is_finite() && *n >= 100.0)
                        .map(|n| format!("{:.0}", (n / 100.0).floor()))
                        .or_else(|| v.var("IBIS_LinieKurs").filter(|n| n.is_finite() && *n > 0.0)
                        .map(|n| format!("{:.0}", n))).unwrap_or_default()
                })
            };
        Self { id, position: v.position, heading: v.heading, speed: v.physics.speed,
            line: line_key(&line), horn: horn || horn_active(v), high_beam: high_active(v) }
    }
}

/// Both drivers see the other's front, on the same road level, in neighbouring lanes.
pub fn oncoming(a: &Bus, b: &Bus) -> bool {
    if a.id == b.id || !a.position.is_finite() || !b.position.is_finite()
        || !a.heading.is_finite() || !b.heading.is_finite() { return false; }
    let d = b.position - a.position;
    let distance = d.truncate().length();
    if !(3.0..=RANGE).contains(&distance) || d.z.abs() > 2.5 { return false; }
    let forward = |h: f64| { let h = h.to_radians(); DVec2::new(h.sin(), h.cos()) };
    let (f, g) = (forward(a.heading), forward(b.heading));
    f.dot(g) < -0.75 && d.truncate().dot(f) > 1.0 && (-d.truncate()).dot(g) > 1.0
        && d.truncate().perp_dot(f).abs() <= 12.0 && d.truncate().perp_dot(g).abs() <= 12.0
}

struct Input { horn: bool, high: bool, began: Option<f32> }
impl Input {
    fn new(bus: &Bus) -> Self { Self { horn: bus.horn, high: bus.high_beam, began: None } }
    fn sample(&mut self, now: f32, bus: &Bus) -> bool {
        let horn = bus.horn && !self.horn;
        if bus.high_beam && !self.high { self.began = Some(now); }
        let flash = !bus.high_beam && self.high && self.began.take()
            .is_some_and(|start| (0.035..=FLASH_MAX).contains(&(now - start)));
        self.horn = bus.horn; self.high = bus.high_beam;
        horn || flash
    }
}
fn nearby(a: &Bus, b: &Bus) -> bool {
    a.id != b.id && a.position.is_finite() && b.position.is_finite()
        && (a.position-b.position).z.abs() <= 2.5
        && (a.position-b.position).truncate().length() <= RANGE
}
struct Pending { ai: u64, other: BusId, at: f32, response: bool }
struct Flash { began: f32, saved: Vec<(&'static str, f32)> }

#[derive(Default)]
pub struct Greetings {
    inputs: HashMap<BusId, Input>,
    encounters: HashMap<(BusId, BusId), f32>,
    last: HashMap<u64, f32>,
    pending: Vec<Pending>,
    flashes: HashMap<u64, Flash>,
    clock: f32,
}

impl Greetings {
    /// Restore before the next script frame, so a pulse cannot latch a switch or affect the driving code.
    pub fn restore(&self, sim: &mut super::TrafficSim) {
        for car in &mut sim.cars {
            if let Some(flash) = self.flashes.get(&car.id) {
                for &(name, value) in &flash.saved { car.vehicle.set_var(name, value); }
            }
        }
    }

    pub fn update(&mut self, sim: &mut super::TrafficSim, mut buses: Vec<Bus>, dt: f32) {
        if sim.mirror || !dt.is_finite() || dt <= 0.0 { return; }
        self.clock += dt;
        let now = self.clock;
        for car in &sim.cars {
            if car.is_bus() && !car.is_rail() {
                let mut bus = Bus::from_vehicle(BusId::Ai(car.id), &car.vehicle, &car.vehicle.host.tt_line, false);
                bus.speed = car.state.speed;
                buses.push(bus);
            }
        }
        buses.sort_by_key(|bus| bus.id);
        let present: HashSet<_> = buses.iter().map(|bus| bus.id).collect();
        self.inputs.retain(|id, _| present.contains(id));
        self.last.retain(|id, _| present.contains(&BusId::Ai(*id)));
        self.encounters.retain(|(a,b), time| present.contains(a) && present.contains(b) && now - *time < 75.0);
        let signals: HashSet<_> = buses.iter().filter(|b| matches!(b.id, BusId::Player(_))).filter_map(|bus| {
            let input = self.inputs.entry(bus.id).or_insert_with(|| Input::new(bus));
            input.sample(now, bus).then_some(bus.id)
        }).collect();
        // Spatial bins bound the search to neighbouring roads, including on large maps.
        let cell = |p: DVec3| ((p.x / RANGE).floor() as i64, (p.y / RANGE).floor() as i64);
        let mut grid: HashMap<(i64,i64),Vec<usize>> = HashMap::new();
        for (i, bus) in buses.iter().enumerate() { grid.entry(cell(bus.position)).or_default().push(i); }
        for (i, a) in buses.iter().enumerate() {
            let (x,y) = cell(a.position);
            for dx in -1..=1 { for dy in -1..=1 {
                for &j in grid.get(&(x+dx,y+dy)).into_iter().flatten() {
                    if j <= i { continue; }
                    let b = &buses[j];
                    if !nearby(a,b) { continue; }
                    let pair = (a.id,b.id);
                    let spontaneous = oncoming(a,b) && (a.speed.abs() > 0.5 || b.speed.abs() > 0.5) && !a.line.is_empty() && a.line == b.line && !self.encounters.contains_key(&pair);
                    let response = signals.contains(&a.id) || signals.contains(&b.id);
                    if !spontaneous && !response { continue; }
                    self.encounters.insert(pair, now);
                    for (k,(bus,other)) in [(a,b),(b,a)].into_iter().enumerate() {
                        let BusId::Ai(id) = bus.id else { continue; };
                        if now - self.last.get(&id).copied().unwrap_or(f32::MIN) < COOLDOWN
                            || self.pending.iter().any(|p| p.ai == id) { continue; }
                        self.pending.push(Pending { ai:id, other:other.id, at:now + 0.15 + k as f32 * 0.25, response });
                    }
                }
            }}
        }
        let mut due = Vec::new();
        self.pending.retain(|p| {
            if !present.contains(&BusId::Ai(p.ai)) || !present.contains(&p.other) { return false; }
            if now >= p.at { due.push((p.ai,p.other,p.response)); false } else { true }
        });
        for (id,other,response) in due {
            let a = buses.iter().find(|b| b.id == BusId::Ai(id));
            let b = buses.iter().find(|b| b.id == other);
            if !a.zip(b).is_some_and(|(a,b)| if response { nearby(a,b) } else { oncoming(a,b) }) { continue; }
            let Some(car) = sim.cars.iter_mut().find(|c| c.id == id) else { continue; };
            self.last.insert(id,now);
            // Audio is emitted once by the model-aware presentation path below;
            // do not also fire a script trigger that could double the same horn.
            // A reserved sound event uses the existing protocol-14 event channel; clients
            // with a model lacking ev_AI_Horn use the ordinary spatial AI horn fallback.
            car.vehicle.host.fired_triggers.push(HORN_EVENT.into());
            if car.vehicle.host.capture_sound_triggers {
                car.vehicle.host.network_sound_triggers.push((HORN_EVENT.into(), None, None));
                car.vehicle.host.network_sound_triggers.push((FLASH_EVENT.into(), None, None));
            }
            let saved = lamp_names(&car.vehicle).into_iter()
                .filter_map(|name| car.vehicle.var(name).map(|v| (name,v))).collect();
            self.flashes.insert(id,Flash { began:now,saved });
        }
        self.flashes.retain(|id,f| present.contains(&BusId::Ai(*id)) && now - f.began <= 0.7);
        for car in &mut sim.cars {
            if let Some(f) = self.flashes.get(&car.id) {
                let elapsed = now - f.began;
                let on = elapsed < 0.2 || (0.35..0.55).contains(&elapsed);
                pulse(&mut car.vehicle, &f.saved, on);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn bus(id: BusId, y:f64, heading:f64, line:&str) -> Bus {
        Bus {id,position:DVec3::new(0.0,y,0.0),heading,speed:8.0,line:line_key(line),horn:false,high_beam:false}
    }
    #[test] fn opposite_direction_and_same_road_only() {
        let a=bus(BusId::Ai(1),0.0,0.0,"205"); let mut b=bus(BusId::Player(1),40.0,180.0,"205");
        assert!(oncoming(&a,&b)); b.heading=0.0; assert!(!oncoming(&a,&b));
        b.heading=90.0; assert!(!oncoming(&a,&b)); b.heading=180.0; b.position.z=5.0; assert!(!oncoming(&a,&b));
        b.position.z=0.0; b.position.x=25.0; assert!(!oncoming(&a,&b)); b.position.x=0.0; b.position.y=-20.0; assert!(!oncoming(&a,&b));
    }
    #[test] fn low_beams_and_steady_high_beams_are_not_a_flash() {
        let mut b=bus(BusId::Player(1),0.0,0.0,""); let mut input=Input::new(&b);
        for n in 0..30 { assert!(!input.sample(n as f32 * 0.1,&b)); }
        b.high_beam=true; assert!(!input.sample(3.0,&b)); assert!(!input.sample(6.0,&b));
        b.high_beam=false; assert!(!input.sample(6.1,&b));
        b.high_beam=true; assert!(!input.sample(7.0,&b)); b.high_beam=false; assert!(input.sample(7.2,&b));
        assert!(!input.sample(7.3,&b));
    }
    #[test] fn initial_high_beam_and_held_horn_do_not_repeat() {
        let mut b=bus(BusId::Player(1),0.0,0.0,""); b.high_beam=true; let mut input=Input::new(&b);
        b.high_beam=false; assert!(!input.sample(0.1,&b)); b.horn=true; assert!(input.sample(0.2,&b));
        assert!(!input.sample(10.0,&b)); b.horn=false; assert!(!input.sample(10.1,&b));
        b.horn=true; assert!(input.sample(10.2,&b));
    }
    #[test] fn line_letters_and_branch_suffixes_stay_distinct() {
        assert_eq!(line_key(" 0205 "),line_key("205")); assert_ne!(line_key("92E"),line_key("92"));
        assert_ne!(line_key("7053-22"),line_key("7053-10")); assert_eq!(line_key("000"),"");
    }

    fn scene() -> (super::super::scenario_tests::Fixture, super::super::TrafficSim) {
        let mut f=super::super::scenario_tests::Fixture::new();
        std::sync::Arc::make_mut(&mut f.ty).model.meshes.push(omsi_model::MeshDef {
            light_enh_2: vec![omsi_model::LightEnh2 { variable:"lights_fern".into(),time_const:10.0,..Default::default() },
                omsi_model::LightEnh2 { variable:"lights_abbl".into(),..Default::default() }], ..Default::default()
        });
        let net=crate::traffic::Network { lanes:vec![super::super::scenario_tests::street(DVec3::ZERO,0.0,500.0)],..Default::default() };
        let mut sim=super::super::scenario_tests::traffic(&f,net);
        let setup=super::super::BusSetup {route:vec![0],stops:vec![],number:None,hof:None,
            timetable:super::super::bus_service::AiTimetable {line:"205".into(),terminus:"A".into(),stops:vec![]} };
        let mut v=VehicleInstance::new(f.ty.clone(),crate::VehicleHost::new(crate::SimClock::default()));
        setup.timetable.install(&mut v.host,None);
        v.set_engine_var("lights_fern",0.0); v.set_engine_var("lights_abbl",1.0); v.light_fade=vec![0.0,0.75];
        sim.place_car(v,crate::traffic::LaneKind::Street,0,0.0,f.ty.clone(),1,None,Some(1),Some(8.0),Some(setup));
        sim.cars[0].vehicle.position=DVec3::ZERO; sim.cars[0].vehicle.heading=0.0;
        sim.cars[0].vehicle.host.capture_sound_triggers=true;
        (f,sim)
    }
    #[test] fn same_line_player_gets_one_greeting_and_lamps_restore() {
        let (_f,mut sim)=scene(); let p=bus(BusId::Player(0),40.0,180.0,"205"); let mut g=Greetings::default();
        g.update(&mut sim,vec![p.clone()],0.1); g.restore(&mut sim); g.update(&mut sim,vec![p.clone()],0.2);
        assert_eq!(sim.cars[0].vehicle.var("lights_fern"),Some(1.0));
        assert_eq!(sim.cars[0].vehicle.var("lights_abbl"),Some(1.0));
        assert_eq!(sim.cars[0].vehicle.light_fade,vec![1.0,0.75]);
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e|e.0==HORN_EVENT).count(),1);
        for _ in 0..200 { g.restore(&mut sim); g.update(&mut sim,vec![p.clone()],0.1); }
        assert_eq!(sim.cars[0].vehicle.var("lights_fern"),Some(0.0));
        assert_eq!(sim.cars[0].vehicle.light_fade,vec![0.0,0.75]);
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e|e.0==HORN_EVENT).count(),1);
    }
    #[test] fn different_line_replies_to_horn_and_brief_flash_but_not_steady_lights() {
        let (_f,mut sim)=scene(); let mut p=bus(BusId::Player(4),40.0,180.0,"2072"); let mut g=Greetings::default();
        g.update(&mut sim,vec![p.clone()],0.1); p.high_beam=true;
        for _ in 0..30 { g.update(&mut sim,vec![p.clone()],0.1); }
        p.high_beam=false; g.update(&mut sim,vec![p.clone()],0.1); g.update(&mut sim,vec![p.clone()],0.2);
        assert!(sim.cars[0].vehicle.host.network_sound_triggers.is_empty());
        p.high_beam=true; g.update(&mut sim,vec![p.clone()],0.1); p.high_beam=false;
        g.update(&mut sim,vec![p.clone()],0.2); g.update(&mut sim,vec![p.clone()],0.2);
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e| e.0==HORN_EVENT).count(),1);
        for _ in 0..110 {g.restore(&mut sim);g.update(&mut sim,vec![p.clone()],0.1);}
        p.horn=true;g.update(&mut sim,vec![p.clone()],0.1);g.update(&mut sim,vec![p.clone()],0.2);
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e| e.0==HORN_EVENT).count(),2);
        for _ in 0..110 {g.restore(&mut sim);g.update(&mut sim,vec![p.clone()],0.1);}
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e| e.0==HORN_EVENT).count(),2);
    }
    #[test] fn mirrored_traffic_never_generates_a_second_greeting() {
        let (_f,mut sim)=scene(); sim.mirror=true; let mut g=Greetings::default();
        for _ in 0..20 {g.update(&mut sim,vec![bus(BusId::Player(0),40.0,180.0,"205")],0.1);}
        assert!(sim.cars[0].vehicle.host.network_sound_triggers.is_empty());
    }
    #[test] fn horn_event_reaches_the_real_spatial_sound_snapshot_once() {
        let (_f,mut sim)=scene(); let mut frame=crate::view_events::SoundFrame::default();
        sim.cars[0].vehicle.host.fired_triggers.push(HORN_EVENT.into());
        sim.snapshot_sounds(&mut frame,DVec3::ZERO,250.0,0.0,None); assert_eq!(frame.greeting_honks.len(),1);
        sim.snapshot_sounds(&mut frame,DVec3::ZERO,250.0,0.0,None); assert!(frame.greeting_honks.is_empty());
    }
    #[test] fn two_ai_buses_on_the_same_line_greet_each_other_once() {
        let (f,mut sim)=scene();
        let setup=super::super::BusSetup {route:vec![0],stops:vec![],number:None,hof:None,
            timetable:super::super::bus_service::AiTimetable {line:"205".into(),terminus:"B".into(),stops:vec![]} };
        let mut v=VehicleInstance::new(f.ty.clone(),crate::VehicleHost::new(crate::SimClock::default()));
        setup.timetable.install(&mut v.host,None);
        v.set_engine_var("lights_fern",0.0);
        sim.place_car(v,crate::traffic::LaneKind::Street,0,40.0,f.ty.clone(),2,None,Some(2),Some(8.0),Some(setup));
        sim.cars[1].vehicle.position=DVec3::new(-3.5,40.0,0.0);sim.cars[1].vehicle.heading=180.0;
        sim.cars[1].vehicle.host.capture_sound_triggers=true;
        let mut g=Greetings::default();g.update(&mut sim,vec![],0.1);g.update(&mut sim,vec![],0.5);
        for car in &sim.cars { assert_eq!(car.vehicle.var("lights_fern"),Some(1.0));
            assert_eq!(car.vehicle.host.network_sound_triggers.iter().filter(|e|e.0==HORN_EVENT).count(),1); }
        for _ in 0..100 {g.restore(&mut sim);g.update(&mut sim,vec![],0.1);}
        for car in &sim.cars {assert_eq!(car.vehicle.host.network_sound_triggers.iter().filter(|e|e.0==HORN_EVENT).count(),1);}
    }

    #[test] fn stationary_parallel_bus_answers_once_without_a_matching_line() {
        let (_f,mut sim)=scene(); let mut p=bus(BusId::Player(4),15.0,0.0,"2072"); p.speed=0.0;
        sim.cars[0].state.speed=0.0; let mut g=Greetings::default();
        g.update(&mut sim,vec![p.clone()],0.1); p.horn=true;
        g.update(&mut sim,vec![p.clone()],0.1);g.update(&mut sim,vec![p.clone()],0.5);
        for _ in 0..200 {g.restore(&mut sim);g.update(&mut sim,vec![p.clone()],0.1);}
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e|e.0==HORN_EVENT).count(),1);
        assert_eq!(sim.cars[0].vehicle.host.network_sound_triggers.iter().filter(|e|e.0==FLASH_EVENT).count(),1);
    }
    #[test] fn remote_flash_restores_model_lamps_even_without_an_audio_set() {
        let (_f,mut sim)=scene();let v=&mut sim.cars[0].vehicle;
        start_remote_flash(v);step_remote_flash(v,0.1);
        assert_eq!(v.var("lights_fern"),Some(1.0));assert_eq!(v.var("lights_abbl"),Some(1.0));
        for _ in 0..10 {restore_remote_flash(v);step_remote_flash(v,0.1);}
        assert_eq!(v.var("lights_fern"),Some(0.0));assert_eq!(v.light_fade,vec![0.0,0.75]);
    }
    #[test] fn brazilian_horn_beam_and_standard_complex_ibis_are_read() {
        let (_f,mut sim)=scene();let v=&mut sim.cars[0].vehicle;
        v.host.tt_line.clear();v.set_engine_var("IBIS_Linie_Complex",20500.0);
        v.set_engine_var("buzina",1.0);v.set_engine_var("farol_alto",1.0);
        let b=Bus::from_vehicle(BusId::Player(0),v,"",false);
        assert_eq!(b.line,"205");assert!(b.horn && b.high_beam);
    }

}
