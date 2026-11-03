//! Opt-in accessibility beta. Movement is owned by this state machine, never by the
//! walking passenger's step/climb or stuck-passenger recovery paths.
use super::*;
use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
static INTERLOCK: AtomicBool = AtomicBool::new(false);
pub fn beta_enabled() -> bool {
    std::env::var("OMSI_WHEELCHAIR_BETA").map(|s|s=="1")
        .unwrap_or(option_env!("OPENOMSI_WHEELCHAIR_BETA")==Some("1"))
}
pub fn interlocked() -> bool {
    INTERLOCK.load(Ordering::Acquire)
}

pub mod geometry;
pub mod network;
pub mod pose;

static COMMAND: AtomicU8 = AtomicU8::new(0);
pub const SPAWN: u8 = 1;
pub const ASSIST: u8 = 2;
pub const ALIGHT: u8 = 4;
pub fn command(command: u8) {
    COMMAND.fetch_or(command, Ordering::Release);
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum Equipment {
    AddedLift,
    AddedRamp,
    NativeRamp,
    NativeLift,
}

/// Coordinates are metres, X right / Y forward / Z up, in the lead bus frame.
/// The sidecar is deliberately explicit: a stop button is not evidence of a ramp.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Profile {
    pub schema: u32,
    pub name: String,
    pub equipment: Equipment,
    pub threshold: Vec3,
    pub bay: Vec3,
    pub route: Vec<Vec3>,
    pub bay_heading: f64,
    pub door_var: String,
    pub door_open_value: f32,
    pub exit_index: usize,
    pub clear_width: f32,
    pub platform_length: f32,
    pub native_ground_var: Option<String>,
    pub native_floor_var: Option<String>,
    #[serde(default)]
    pub native_height_var: Option<String>,
    #[serde(default)]
    pub native_stowed_var: Option<String>,
    pub deploy_trigger: Option<String>,
    pub stow_trigger: Option<String>,
    #[serde(default)]
    pub raise_trigger: Option<String>,
    #[serde(default)]
    pub lower_trigger: Option<String>,
    #[serde(default)]
    pub alight_trigger: Option<String>,
}

impl Profile {
    pub fn validate(&self) -> Result<(), String> {
        if self.schema != 1 {
            return Err("unsupported wheelchair profile schema".into());
        }
        if !self.threshold.is_finite()
            || !self.bay.is_finite()
            || !self.bay_heading.is_finite()
            || !self.clear_width.is_finite()
            || !self.platform_length.is_finite()
            || !self.door_open_value.is_finite()
        {
            return Err("non-finite accessibility geometry".into());
        }
        if !(0.78..=1.8).contains(&self.clear_width)
            || !(1.25..=2.2).contains(&self.platform_length)
            || !(0.65..=1.6).contains(&self.threshold.x.abs())
            || self.threshold.y.abs() > 20.0
            || self.threshold.z < 0.15
            || self.threshold.z > 2.0
            || self.bay.x.abs() > 1.1
            || (self.bay.z - self.threshold.z).abs() > 0.05
        {
            return Err("door, platform or wheelchair bay is outside supported dimensions".into());
        }
        if self.route.is_empty()
            || self.route.len() > 24
            || self.route.iter().any(|p| {
                !p.is_finite()
                    || p.x.abs() > 1.6
                    || p.y.abs() > 20.0
                    || (p.z - self.threshold.z).abs() > 0.05
            })
        {
            return Err("accessible route must stay on a continuous cabin floor".into());
        }
        let mut previous = self.threshold;
        for next in self.route.iter().chain(std::iter::once(&self.bay)) {
            if (*next - previous).length() > 4.0 {
                return Err("accessible route has an unverified gap".into());
            }
            previous = *next;
        }
        if self.door_var.is_empty() {
            return Err("an actual door animation variable is required".into());
        }
        if matches!(
            self.equipment,
            Equipment::NativeRamp | Equipment::NativeLift
        ) && (self.native_ground_var.is_none()
            || self.native_floor_var.is_none()
            || self.native_stowed_var.is_none()
            || (self.equipment == Equipment::NativeLift && self.native_height_var.is_none()))
        {
            return Err("native equipment requires ground AND floor endpoint feedback".into());
        }
        Ok(())
    }
    pub fn side(&self) -> f32 {
        self.threshold.x.signum()
    }
    pub fn platform(&self, height: f32) -> Vec3 {
        Vec3::new(
            self.threshold.x + self.side() * self.platform_length * 0.5,
            self.threshold.y,
            height,
        )
    }
    pub fn outside(&self, height: f32) -> Vec3 {
        Vec3::new(
            self.threshold.x + self.side() * (self.platform_length + 0.95),
            self.threshold.y,
            height,
        )
    }
    pub fn load(bus: &Path) -> Result<Option<Self>, String> {
        let specific = bus.with_extension("wheelchair.json");
        let general = bus
            .parent()
            .unwrap_or(Path::new("."))
            .join("wheelchair.json");
        let path = if specific.is_file() {
            specific
        } else {
            general
        };
        if !path.is_file() {
            return Ok(None);
        }
        let bytes = std::fs::read(&path).map_err(|e| e.to_string())?;
        if bytes.len() > 65536 {
            return Err("accessibility profile is too large".into());
        }
        let profile: Self =
            serde_json::from_slice(&bytes).map_err(|e| format!("{}: {e}", path.display()))?;
        profile.validate()?;
        Ok(Some(profile))
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Stage {
    Waiting,
    Deploying,
    ToPlatform,
    Raising,
    ToBay,
    Securing,
    Riding,
    Unsecuring,
    ToExit,
    Lowering,
    Leaving,
    Finished,
}
impl Stage {
    pub fn label(self) -> &'static str {
        match self {
            Self::Waiting => "Aguardando assistência na porta acessível",
            Self::Deploying => "Preparando equipamento",
            Self::ToPlatform => "Entrando no equipamento",
            Self::Raising => "Elevando plataforma",
            Self::ToBay => "Seguindo até a área reservada",
            Self::Securing => "Acomodando e fixando a cadeira",
            Self::Riding => "Em viagem na área reservada",
            Self::Unsecuring => "Preparando desembarque",
            Self::ToExit => "Seguindo até a porta acessível",
            Self::Lowering => "Baixando plataforma",
            Self::Leaving => "Desembarcando",
            Self::Finished => "Desembarque concluído",
        }
    }
}

#[derive(Debug, Clone, Copy, Default)]
pub struct ChairPose {
    pub wheels: [f32; 2],
    pub push_phase: [f32; 2],
    pub moving: f32,
    pub secured: bool,
    pub direction: [f32; 2],
}

#[derive(Debug, Clone, Copy)]
pub struct EquipmentPose {
    pub owner: u32,
    pub position: DVec3,
    pub rotation: Mat4,
    pub length: f32,
    pub width: f32,
    pub height: f32,
    pub floor_height: f32,
    pub deployment: f32,
    pub bridge_deployment: f32,
    pub side: f32,
    pub ground_height: f32,
    pub ramp: bool,
}

/// A controller's snapshot; invalid or missing feedback always holds movement.
#[derive(Debug, Clone, Copy, Default)]
pub struct Feedback {
    pub door_open: bool,
    pub ground_ready: bool,
    pub floor_ready: bool,
    pub equipment_height: Option<f32>,
    pub stowed: bool,
}

#[derive(Debug, Clone)]
pub struct Journey {
    pub stage: Stage,
    pub local: Vec3,
    pub heading: f64,
    pub pose: ChairPose,
    pub platform_height: f32,
    pub deployment: f32,
    pub bridge_deployment: f32,
    pub ground_height: f32,
    pub alight_requested: bool,
    pub boarded: bool,
    pub outside_world: Option<DVec3>,
    pub board_stop: Option<i64>,
    pub last_stop: Option<i64>,
    pub stops: usize,
    timer: f32,
    route_at: usize,
    returning: bool,
    pub hold_reason: &'static str,
}

fn approach(value: &mut f32, target: f32, step: f32) -> bool {
    *value += (target - *value).clamp(-step, step);
    (*value - target).abs() < 0.0001
}
fn angle_delta(from: f64, to: f64) -> f64 {
    (to - from + 180.0).rem_euclid(360.0) - 180.0
}

impl Journey {
    pub fn new(profile: &Profile, ground_height: f32, heading: f64) -> Self {
        Self {
            stage: Stage::Waiting,
            local: profile.outside(ground_height),
            heading,
            pose: Default::default(),
            platform_height: profile.threshold.z,
            deployment: 0.0,
            bridge_deployment: 0.0,
            ground_height,
            alight_requested: false,
            boarded: false,
            outside_world: None,
            board_stop: None,
            last_stop: None,
            stops: 0,
            timer: 0.0,
            route_at: 0,
            returning: false,
            hold_reason: "",
        }
    }
    fn transition(&mut self, to: Stage) {
        log::info!(
            "WHEELCHAIR transition {:?} -> {:?} position={:?} equipment_z={:.3}",
            self.stage,
            to,
            self.local,
            self.platform_height
        );
        self.stage = to;
        self.timer = 0.0;
        self.route_at = 0;
        self.hold_reason = "";
    }
    pub fn interlocked(&self) -> bool {
        self.deployment > 0.0001
            || !matches!(self.stage, Stage::Waiting | Stage::Riding | Stage::Finished)
    }
    /// Differential drive: each wheel follows its own travelled arc. The push
    /// cycle is distance based, so stopped hands never continue propelling.
    fn drive(&mut self, target: Vec3, dt: f32, blockers: &[Vec3]) -> bool {
        let delta = target - self.local;
        if delta.length() < 0.002 {
            return true;
        }
        let wanted = (delta.x as f64).atan2(delta.y as f64).to_degrees();
        let yaw = angle_delta(self.heading, wanted).clamp(-55.0 * dt as f64, 55.0 * dt as f64);
        let heading = (self.heading + yaw).rem_euclid(360.0);
        let ds = if angle_delta(heading, wanted).abs() < 6.0 {
            (0.48 * dt).min(delta.truncate().length())
        } else {
            0.0
        };
        let next = self.local + delta.truncate().normalize_or_zero().extend(0.0) * ds;
        if blockers
            .iter()
            .any(|p| (p.z - next.z).abs() < 1.5 && (p.truncate() - next.truncate()).length() < 0.64)
        {
            self.hold_reason = "Passagem ocupada: aguardando espaço";
            self.pose.moving = 0.0;
            return false;
        }
        self.hold_reason = "";
        self.heading = heading;
        self.local = next;
        let turn = yaw.to_radians() as f32 * 0.33;
        for side in 0..2 {
            let distance = ds + if side == 0 { turn } else { -turn };
            self.pose.direction[side] = distance.signum();
            self.pose.wheels[side] =
                (self.pose.wheels[side] + distance / 0.30).rem_euclid(std::f32::consts::TAU);
            self.pose.push_phase[side] =
                (self.pose.push_phase[side] + distance.abs() / 0.85).fract();
        }
        self.pose.moving = if ds > 0.0001 || yaw.abs() > 0.001 {
            1.0
        } else {
            0.0
        };
        (target - self.local).truncate().length() < 0.002
    }
    fn turn_to(&mut self, heading: f64, dt: f32) -> bool {
        let yaw = angle_delta(self.heading, heading).clamp(-45.0 * dt as f64, 45.0 * dt as f64);
        self.heading = (self.heading + yaw).rem_euclid(360.0);
        for side in 0..2 {
            let distance = yaw.to_radians() as f32 * 0.33 * if side == 0 { 1.0 } else { -1.0 };
            self.pose.direction[side] = distance.signum();
            self.pose.wheels[side] =
                (self.pose.wheels[side] + distance / 0.30).rem_euclid(std::f32::consts::TAU);
            self.pose.push_phase[side] =
                (self.pose.push_phase[side] + distance.abs() / 0.85).fract();
        }
        self.pose.moving = if yaw.abs() > 0.001 { 1.0 } else { 0.0 };
        angle_delta(self.heading, heading).abs() < 0.1
    }
    /// Large stalls are substepped. Time paused by a closed door/obstacle is not
    /// accumulated as a future jump, and no timeout moves a passenger through a wall.
    pub fn advance(
        &mut self,
        profile: &Profile,
        dt: f32,
        speed: f32,
        feedback: Feedback,
        assist: bool,
        blockers: &[Vec3],
    ) {
        if !dt.is_finite() || dt <= 0.0 || !speed.is_finite() {
            return;
        }
        let dt = dt.min(0.1);
        let native = matches!(
            profile.equipment,
            Equipment::NativeRamp | Equipment::NativeLift
        );
        let ramp = matches!(
            profile.equipment,
            Equipment::AddedRamp | Equipment::NativeRamp
        );
        if native && feedback.stowed && matches!(self.stage,Stage::Riding|Stage::Finished) {
            self.deployment=0.0;
            self.bridge_deployment=0.0;
        }
        self.pose.moving = 0.0;
        if self.stage == Stage::Riding && self.deployment <= 0.0 && !self.alight_requested {
            return;
        }
        if (!matches!(self.stage, Stage::Waiting | Stage::Riding | Stage::Finished)
            || self.deployment > 0.0001)
            && (speed.abs() > 0.025 || !feedback.door_open)
        {
            self.hold_reason = if speed.abs() > 0.025 {
                "Aguardando ônibus completamente parado"
            } else {
                "Aguardando abertura da porta acessível"
            };
            return;
        }
        if native && !ramp {
            let measured = feedback.equipment_height.filter(|h| h.is_finite()
                && *h >= self.ground_height.min(profile.threshold.z) - 0.05
                && *h <= self.ground_height.max(profile.threshold.z) + 0.05);
            if let Some(height) = measured {
                self.platform_height = height;
                if matches!(self.stage, Stage::Raising | Stage::Lowering) {
                    self.local.z = height;
                }
            } else if !matches!(self.stage, Stage::Waiting | Stage::Riding | Stage::Finished) {
                self.hold_reason = "Aguardando altura real válida do elevador";
                return;
            }
            if self.stage == Stage::ToPlatform
                && (!feedback.ground_ready || (self.platform_height-self.ground_height).abs()>0.005) {
                self.hold_reason = "Elevador ainda não alinhado ao solo";
                return;
            }
            if matches!(self.stage, Stage::ToBay | Stage::ToExit)
                && (!feedback.floor_ready || (self.platform_height-profile.threshold.z).abs()>0.005) {
                self.hold_reason = "Elevador ainda não alinhado ao piso";
                return;
            }
        }
        match self.stage {
            Stage::Waiting => {
                if assist && speed.abs() < 0.025 && feedback.door_open {
                    if ramp
                        && (profile.threshold.z - self.ground_height).abs()
                            / profile.platform_length
                            > 0.22
                    {
                        self.hold_reason =
                            "Desnível excessivo para esta rampa: reposicione o ônibus";
                        return;
                    }
                    self.transition(Stage::Deploying);
                } else if assist {
                    self.hold_reason = "Pare completamente e abra a porta acessível";
                }
            }
            Stage::Deploying => {
                approach(&mut self.deployment, 1.0, dt / 1.8);
                if native {
                    if feedback.ground_ready && feedback.floor_ready && ramp {
                        self.transition(Stage::ToPlatform);
                    } else if feedback.ground_ready && !ramp
                        && (self.platform_height-self.ground_height).abs()<=0.005 {
                        self.transition(Stage::ToPlatform);
                    } else {
                        self.hold_reason = "Aguardando posição real do equipamento do ônibus";
                    }
                } else if self.deployment >= 1.0
                    && approach(&mut self.platform_height, self.ground_height, dt * 0.16)
                {
                    self.transition(Stage::ToPlatform);
                }
            }
            Stage::ToPlatform => {
                let p = profile.platform(self.ground_height);
                if self.drive(p, dt, blockers) {
                    self.outside_world = None;
                    self.transition(if ramp { Stage::ToBay } else { Stage::Raising });
                }
            }
            Stage::Raising => {
                let ready = if native {
                    feedback.floor_ready && (self.platform_height-profile.threshold.z).abs()<=0.005
                } else {
                    approach(&mut self.platform_height, profile.threshold.z, dt * 0.14)
                };
                self.local.z = self.platform_height;
                if ready && (native || approach(&mut self.bridge_deployment, 1.0, dt / 1.2)) {
                    self.transition(Stage::ToBay);
                }
            }
            Stage::ToBay => {
                let target = if self.route_at == 0 {
                    profile.threshold
                } else if self.route_at <= profile.route.len() {
                    profile.route[self.route_at - 1]
                } else {
                    profile.bay
                };
                if self.drive(target, dt, blockers) {
                    if self.route_at > profile.route.len() {
                        if self.turn_to(profile.bay_heading, dt) {
                            self.boarded = true;
                            self.transition(Stage::Securing);
                        }
                    } else {
                        self.route_at += 1;
                    }
                }
            }
            Stage::Securing => {
                self.timer += dt;
                if self.timer >= 2.0 {
                    self.pose.secured = true;
                    self.transition(Stage::Riding);
                }
            }
            Stage::Riding => {
                if self.deployment > 0.0 && (!native || feedback.stowed) {
                    if approach(&mut self.bridge_deployment, 0.0, dt / 1.2) {
                        approach(&mut self.deployment, 0.0, dt / 1.8);
                    }
                }
                if self.alight_requested && assist && speed.abs() < 0.025 && feedback.door_open {
                    self.returning = true;
                    self.pose.secured = false;
                    self.transition(Stage::Unsecuring);
                }
            }
            Stage::Unsecuring => {
                approach(&mut self.deployment, 1.0, dt / 1.8);
                if !native || ramp {self.platform_height = profile.threshold.z;}
                self.timer += dt;
                approach(&mut self.bridge_deployment, 1.0, dt / 1.2);
                if self.timer >= 2.0 && (!native || (feedback.floor_ready
                    && (ramp || (self.platform_height-profile.threshold.z).abs()<=0.005))) {
                    self.transition(Stage::ToExit);
                }
            }
            Stage::ToExit => {
                let n = profile.route.len();
                let target = if self.route_at < n {
                    profile.route[n - self.route_at - 1]
                } else if self.route_at == n {
                    profile.threshold
                } else {
                    profile.platform(profile.threshold.z)
                };
                if self.drive(target, dt, blockers) {
                    if self.route_at > n {
                        self.transition(if ramp {
                            Stage::Leaving
                        } else {
                            Stage::Lowering
                        });
                    } else {
                        self.route_at += 1;
                    }
                }
            }
            Stage::Lowering => {
                if !native && !approach(&mut self.bridge_deployment, 0.0, dt / 1.2) {
                    return;
                }
                let ready = if native {
                    feedback.ground_ready && (self.platform_height-self.ground_height).abs()<=0.005
                } else {
                    approach(&mut self.platform_height, self.ground_height, dt * 0.14)
                };
                self.local.z = self.platform_height;
                if ready {
                    self.transition(Stage::Leaving);
                }
            }
            Stage::Leaving => {
                if self.drive(profile.outside(self.ground_height), dt, blockers) {
                    self.boarded = false;
                    self.transition(Stage::Finished);
                }
            }
            Stage::Finished => {
                if !native || feedback.stowed {
                    approach(&mut self.deployment, 0.0, dt / 1.8);
                } else {
                    self.hold_reason = "Aguardando recolhimento real do equipamento";
                }
            }
        }
        if ramp
            && matches!(
                self.stage,
                Stage::ToPlatform | Stage::ToBay | Stage::ToExit | Stage::Leaving
            )
        {
            let t = ((self.local.x - profile.threshold.x).abs() / profile.platform_length)
                .clamp(0.0, 1.0);
            self.local.z = profile.threshold.z + (self.ground_height - profile.threshold.z) * t;
        }
    }
}

pub struct Wheelchairs {
    pub enabled: bool,
    pub profile: Option<Profile>,
    pub journey: Option<Journey>,
    pub person: Option<u32>,
    pub status: String,
    pub feedback: Feedback,
    pub equipment: Option<EquipmentPose>,
    pub auto_spawn: bool,
    pub served: HashSet<i64>,
    reserved: Vec<usize>,
    remotes: HashMap<u32, network::Remote>,
    remote_poses: HashMap<u32, ChairPose>,
    pub(crate) remote_equipment: Vec<EquipmentPose>,
}
impl Default for Wheelchairs {
    fn default() -> Self {
        let enabled = beta_enabled();
        Self {
            enabled,
            profile: None,
            journey: None,
            person: None,
            status: String::new(),
            feedback: Feedback::default(),
            equipment: None,
            auto_spawn: std::env::var("OMSI_WHEELCHAIR_AUTO").map_or(enabled, |s|s=="1"),
            served: HashSet::new(),
            reserved: Vec::new(),
            remotes: HashMap::new(),
            remote_poses: HashMap::new(),
            remote_equipment: Vec::new(),
        }
    }
}
impl Wheelchairs {
    pub fn pose(&self, id: u32) -> Option<ChairPose> {
        (self.person == Some(id))
            .then(|| self.journey.as_ref().map(|j| j.pose))
            .flatten().or_else(||self.remote_poses.get(&id).copied())
    }
    pub fn feedback_from(&self, v: &VehicleInstance) -> Feedback {
        let Some(p) = self.profile.as_ref() else {
            return Feedback::default();
        };
        let ready = |name: &Option<String>| {
            name.as_ref()
                .and_then(|n| v.var(n))
                .is_some_and(|x| x.is_finite() && x >= 0.95)
        };
        Feedback {
            door_open: v
                .var(&p.door_var)
                .is_some_and(|x| x.is_finite() && x >= p.door_open_value),
            ground_ready: ready(&p.native_ground_var),
            floor_ready: ready(&p.native_floor_var),
            equipment_height: p.native_height_var.as_ref().and_then(|n| v.var(n)),
            stowed: ready(&p.native_stowed_var),
        }
    }
}

impl PeopleSim {
    pub fn configure_wheelchairs(&mut self, v: &VehicleInstance, cabin: &Cabin) {
        if !self.wheelchairs.enabled {
            return;
        }
        INTERLOCK.store(false, Ordering::Release);
        if let Some(seats) = self.seats.get_mut(&BusId::Player) {
            for k in self.wheelchairs.reserved.drain(..) {
                if let Some(s) = seats.get_mut(k) {
                    *s = false;
                }
            }
        }
        if let Some(id) = self.wheelchairs.person.take() {
            if let Some(i) = self.people.iter().position(|p| p.id == id) {
                let p = self.people.swap_remove(i);
                self.retire(&p);
            }
        }
        self.wheelchairs.journey = None;
        self.wheelchairs.equipment = None;
        self.wheelchairs.profile = match Profile::load(&v.ty.def.path) {
            Ok(Some(profile))
                if profile.exit_index < cabin.exits.len() && v.var(&profile.door_var).is_some() =>
            {
                log::info!(
                    "WHEELCHAIR profile={} bus={} door={} threshold={:?} bay={:?}",
                    profile.name,
                    v.ty.def.path.display(),
                    profile.door_var,
                    profile.threshold,
                    profile.bay
                );
                self.wheelchairs.status = format!(
                    "Acessibilidade beta: {} | Ctrl+Alt+W: passageiro | Ctrl+Alt+E: assistência | Ctrl+Alt+D: desembarque",
                    profile.name
                );
                Some(profile)
            }
            Ok(Some(_)) => {
                self.wheelchairs.status =
                    "Perfil incompatível: porta ou variável não encontrada no ônibus".into();
                None
            }
            Ok(None) => {
                self.wheelchairs.status="Este ônibus precisa de um perfil wheelchair.json medido para a porta e área reservada".into();
                None
            }
            Err(e) => {
                log::warn!("WHEELCHAIR profile rejected: {e}");
                self.wheelchairs.status = format!("Perfil rejeitado: {e}");
                None
            }
        };
    }

    pub(super) fn tick_wheelchairs(
        &mut self,
        dt: f32,
        world: &dyn World,
        bus: Option<&PlayerBusView>,
        buses: &[BusNow],
    ) {
        if !self.wheelchairs.enabled {
            return;
        }
        let commands = COMMAND.swap(0, Ordering::AcqRel);
        let (Some(profile), Some(bus), Some(bn)) = (
            self.wheelchairs.profile.clone(),
            bus,
            buses.iter().find(|b| b.id == BusId::Player),
        ) else {
            return;
        };
        self.wheelchairs.feedback = bus.wheelchair_feedback;
        let stop = self.player_next_stop.as_ref().map(|s| s.id);
        let auto = self.wheelchairs.auto_spawn
            && stop.is_some_and(|s| !self.wheelchairs.served.contains(&s))
            && self
                .player_next_stop
                .as_ref()
                .is_some_and(|s| (s.pos - bn.pos).length() < 35.0)
            && bus.speed.abs() < 0.025;
        if self.wheelchairs.journey.is_none() && (commands & SPAWN != 0 || auto) {
            if bus.speed.abs() > 0.025 {
                self.wheelchairs.status =
                    "Pare o ônibus antes de criar o passageiro de teste".into();
                return;
            }
            let outside = bn.world(profile.outside(0.0));
            let Some(height) = world.walk_height_near(outside.x, outside.y, bn.pos.z) else {
                self.wheelchairs.status =
                    "Não foi encontrada uma superfície carregada para aguardar o ônibus".into();
                return;
            };
            let ground = bn.to_local(DVec3::new(outside.x, outside.y, height)).z;
            if (profile.threshold.z - ground).abs() > 1.6 {
                self.wheelchairs.status = "Desnível fora do alcance do elevador".into();
                return;
            }
            let mut journey = Journey::new(
                &profile,
                ground,
                if profile.side() > 0.0 { 270.0 } else { 90.0 },
            );
            let at = bn.world(journey.local);
            journey.outside_world = Some(at);
            let compatible:Vec<usize>=self.types.iter().enumerate()
                .filter(|(_,t)|t.def.age.unwrap_or(40)>=18)
                .filter(|(_,t)|(1.45..=1.95).contains(&t.rig.head_top)
                    && t.rig.upper_arm.is_finite()&&t.rig.forearm.is_finite()
                    && t.rig.upper_arm+t.rig.forearm>0.45)
                .map(|(i,_)|i).collect();
            if compatible.is_empty(){self.wheelchairs.status="Nenhum personagem adulto com rig compatível foi encontrado".into();return;}
            let selected=Some(compatible[self.rand() as usize%compatible.len()]);
            let Some(i) = self.spawn_as(
                world,
                at,
                bn.heading + journey.heading,
                State::Idle,
                selected,
            ) else {
                self.wheelchairs.status =
                    "Não foi possível carregar um personagem instalado".into();
                return;
            };
            let person = &mut self.people[i];
            person.puppet = Some(Puppet {
                mode: PuppetMode::Wheelchair,
            });
            person.activity = Activity::Sit;
            self.wheelchairs.person = Some(person.id);
            self.wheelchairs.journey = Some(journey);
            if let Some(s) = stop {
                self.wheelchairs.served.insert(s);
            }
            log::info!(
                "WHEELCHAIR spawned person={} profile={} ground_z={ground:.3}",
                person.id,
                profile.name
            );
        }
        let (Some(id), Some(mut journey)) =
            (self.wheelchairs.person, self.wheelchairs.journey.take())
        else {
            return;
        };
        let Some(i) = self.people.iter().position(|p| p.id == id) else {
            self.wheelchairs.person = None;
            return;
        };
        if journey.stage == Stage::Waiting || journey.stage == Stage::Finished {
            if let Some(at) = journey.outside_world {
                journey.local = bn.to_local(at);
                // A person left at the curb keeps their world orientation when
                // the bus turns or drives away.
                journey.heading = angle_delta(bn.heading, self.people[i].heading);
            }
        }
        if commands & ALIGHT != 0 {
            journey.alight_requested = true;
        }
        if journey.stage == Stage::Riding {
            if journey.board_stop.is_none() {
                journey.board_stop = stop;
                journey.last_stop = stop;
            }
            if stop.is_some() && stop != journey.last_stop {
                journey.last_stop = stop;
                journey.stops += 1;
            }
            if journey.stops >= 2 {
                journey.alight_requested = true;
            }
            if journey.alight_requested {
                self.stop_request = true;
            }
        }
        let blockers: Vec<Vec3> = self
            .people
            .iter()
            .filter(|p| p.id != id && p.puppet.is_none())
            .filter(|p| (p.position - bn.world(journey.local)).length_squared() < 16.0)
            .map(|p| bn.to_local(p.position))
            .collect();
        journey.advance(
            &profile,
            dt,
            bus.speed,
            self.wheelchairs.feedback,
            commands & ASSIST != 0,
            &blockers,
        );
        // The bay is a volume, not a seat index: reserve all overlapping standing
        // places, and never take a place from somebody who is already there.
        if !matches!(journey.stage, Stage::Waiting | Stage::Finished) {
            if let Some(seats) = self.seats.get_mut(&BusId::Player) {
                for (k, s) in bn.cabin.seats.iter().enumerate() {
                    if !s.seated && (s.floor.truncate() - profile.bay.truncate()).length() < 0.85 {
                        if let Some(slot) = seats.get_mut(k) {
                            if !*slot {
                                self.wheelchairs.reserved.push(k);
                                *slot = true;
                            }
                        }
                    }
                }
            }
        }
        if journey.stage == Stage::Finished {
            if journey.outside_world.is_none() {
                journey.outside_world = Some(bn.world(journey.local));
            }
            if let Some(seats) = self.seats.get_mut(&BusId::Player) {
                for k in self.wheelchairs.reserved.drain(..) {
                    if let Some(s) = seats.get_mut(k) {
                        *s = false;
                    }
                }
            }
        }
        INTERLOCK.store(journey.interlocked(), Ordering::Release);
        if journey.stage == Stage::ToBay
            && (journey.local.x - profile.threshold.x) * profile.side() < -0.05
        {
            journey.boarded = true;
        }
        let p = &mut self.people[i];
        p.position = bn.world(journey.local);
        p.heading = bn.heading + journey.heading;
        p.lheading = journey.heading;
        p.place = if journey.boarded {
            Place::Bus(BusId::Player, journey.local)
        } else {
            Place::Ground
        };
        p.tilt = if matches!(journey.stage,Stage::Waiting|Stage::Finished) {
            Mat4::IDENTITY
        } else {
            bn.rot * Mat4::from_rotation_z(bn.heading.to_radians() as f32)
        };
        p.interior = if journey.boarded { bn.interior } else { 0.0 };
        p.vel = DVec2::ZERO;
        p.activity = Activity::Sit;
        p.why = "wheelchair accessibility controller";
        let native = matches!(
            profile.equipment,
            Equipment::NativeLift | Equipment::NativeRamp
        );
        self.wheelchairs.equipment =
            (!native && journey.deployment > 0.0001).then(|| EquipmentPose {
                owner: 0,
                position: bn.world(Vec3::new(
                    profile.threshold.x,
                    profile.threshold.y,
                    if profile.equipment == Equipment::AddedRamp {
                        profile.threshold.z
                    } else {
                        journey.platform_height
                    },
                )),
                rotation: bn.rot,
                length: profile.platform_length,
                width: profile.clear_width,
                height: journey.platform_height,
                floor_height: profile.threshold.z,
                deployment: journey.deployment,
                bridge_deployment: journey.bridge_deployment,
                side: profile.side(),
                ground_height: journey.ground_height,
                ramp: profile.equipment == Equipment::AddedRamp,
            });
        self.wheelchairs.status = format!(
            "{}{}",
            journey.stage.label(),
            if journey.hold_reason.is_empty() {
                String::new()
            } else {
                format!(" · {}", journey.hold_reason)
            }
        );
        if journey.stage == Stage::Finished
            && journey.deployment <= 0.0
            && (self.people[i].position - bn.pos).length() > 45.0
        {
            let person = self.people.swap_remove(i);
            self.retire(&person);
            self.wheelchairs.person = None;
            self.wheelchairs.equipment = None;
            INTERLOCK.store(false, Ordering::Release);
        } else {
            self.wheelchairs.journey = Some(journey);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    pub fn profile() -> Profile {
        Profile {
            schema: 1,
            name: "test".into(),
            equipment: Equipment::AddedLift,
            threshold: Vec3::new(1.16, 0.0, 0.97),
            bay: Vec3::new(-0.72, 0.0, 0.97),
            route: vec![Vec3::new(0.35, 0.0, 0.97)],
            bay_heading: 180.0,
            door_var: "door_2".into(),
            door_open_value: 0.9,
            exit_index: 0,
            clear_width: 1.05,
            platform_length: 1.4,
            native_ground_var: None,
            native_floor_var: None,
            native_height_var: None,
            native_stowed_var: None,
            deploy_trigger: None,
            stow_trigger: None,
            raise_trigger: None,
            lower_trigger: None,
            alight_trigger: None,
        }
    }
    fn run(j: &mut Journey, p: &Profile, target: Stage) {
        for n in 0..12000 {
            j.advance(
                p,
                1.0 / 60.0,
                0.0,
                Feedback {
                    door_open: true,
                    ..Default::default()
                },
                n == 0,
                &[],
            );
            if j.stage == target {
                return;
            }
        }
        panic!(
            "never reached {target:?}, at {:?} pos {:?}",
            j.stage, j.local
        );
    }
    #[test]
    fn complete_lift_journey_has_bounded_continuous_movement() {
        let p = profile();
        p.validate().unwrap();
        let mut j = Journey::new(&p, 0.0, 270.0);
        let mut max_step = 0.0f32;
        for n in 0..12000 {
            let before = j.local;
            j.advance(
                &p,
                1.0 / 60.0,
                0.0,
                Feedback {
                    door_open: true,
                    ..Default::default()
                },
                n == 0,
                &[],
            );
            max_step = max_step.max((j.local - before).length());
            if j.stage == Stage::Riding {
                break;
            }
        }
        assert_eq!(j.stage, Stage::Riding);
        assert!(j.pose.secured);
        assert!(max_step < 0.009, "teleport: {max_step}");
        j.alight_requested = true;
        run(&mut j, &p, Stage::Finished);
        assert!(!j.boarded);
        assert!((j.local - p.outside(0.0)).length() < 0.003);
    }
    #[test]
    fn door_and_motion_interlocks_hold_every_boarding_stage() {
        let p = profile();
        let mut j = Journey::new(&p, 0.0, 270.0);
        j.transition(Stage::Raising);
        j.local = p.platform(0.0);
        let before = j.local;
        for _ in 0..600 {
            j.advance(
                &p,
                0.1,
                0.5,
                Feedback {
                    door_open: true,
                    ..Default::default()
                },
                false,
                &[],
            );
        }
        assert_eq!(j.local, before);
        assert_eq!(j.stage, Stage::Raising);
        for _ in 0..600 {
            j.advance(&p, 0.1, 0.0, Feedback::default(), false, &[]);
        }
        assert_eq!(j.local, before);
    }
    #[test]
    fn native_equipment_waits_for_actual_endpoint_feedback() {
        let mut p = profile();
        p.equipment = Equipment::NativeLift;
        let mut j = Journey::new(&p, 0.0, 270.0);
        j.transition(Stage::Deploying);
        for _ in 0..1000 {
            j.advance(
                &p,
                0.1,
                0.0,
                Feedback {
                    door_open: true,
                    ..Default::default()
                },
                false,
                &[],
            );
        }
        assert_eq!(j.stage, Stage::Deploying);
        j.advance(
            &p,
            0.1,
            0.0,
            Feedback {
                door_open: true,
                ground_ready: true,
                floor_ready: false,
                equipment_height: Some(0.0),
                stowed: false,
            },
            false,
            &[],
        );
        assert_eq!(j.stage, Stage::ToPlatform);
    }
    #[test]
    fn obstacles_hold_indefinitely_without_ghosting() {
        let p = profile();
        let mut j = Journey::new(&p, 0.0, 270.0);
        j.transition(Stage::ToPlatform);
        let before = j.local;
        let blocker = before + Vec3::new(-0.1, 0.0, 0.0);
        for _ in 0..6000 {
            j.advance(
                &p,
                0.1,
                0.0,
                Feedback {
                    door_open: true,
                    ..Default::default()
                },
                false,
                &[blocker],
            );
        }
        assert_eq!(j.local, before);
        assert_eq!(j.stage, Stage::ToPlatform);
    }
    #[test]
    fn invalid_profiles_and_nan_are_rejected() {
        let mut p = profile();
        p.route[0].z += 0.3;
        assert!(p.validate().is_err());
        p = profile();
        p.bay.x = f32::NAN;
        assert!(p.validate().is_err());
        let p = profile();
        let mut j = Journey::new(&p, 0.0, 270.0);
        let before = j.local;
        j.advance(&p, f32::NAN, 0.0, Feedback::default(), true, &[]);
        assert_eq!(j.local, before);
    }
    #[test]
    fn native_lift_tracks_measured_height_and_checks_alignment() {
        let mut p=profile();p.equipment=Equipment::NativeLift;
        let mut j=Journey::new(&p,0.0,270.0);j.transition(Stage::Raising);
        j.local=p.platform(0.0);j.platform_height=0.0;
        for n in 1..=90 {
            let before=j.local.z;
            j.advance(&p,0.1,0.0,Feedback{door_open:true,equipment_height:Some(n as f32*0.01),
                floor_ready:true,..Default::default()},false,&[]);
            assert!((j.local.z-before).abs()<0.011);
            assert_eq!(j.stage,Stage::Raising);
        }
        j.advance(&p,0.1,0.0,Feedback{door_open:true,equipment_height:None,
            floor_ready:true,..Default::default()},false,&[]);
        assert_eq!(j.stage,Stage::Raising);assert!((j.local.z-0.9).abs()<0.001);
        j.advance(&p,0.1,0.0,Feedback{door_open:true,equipment_height:Some(p.threshold.z),
            floor_ready:true,..Default::default()},false,&[]);
        assert_eq!(j.stage,Stage::ToBay);assert_eq!(j.local.z,p.threshold.z);
    }
    #[test]
    fn native_equipment_must_be_stowed_before_departure_after_unloading() {
        let mut p=profile();p.equipment=Equipment::NativeLift;
        let mut j=Journey::new(&p,0.0,270.0);j.transition(Stage::Finished);j.deployment=1.0;
        for _ in 0..100 {j.advance(&p,0.1,0.0,Feedback{door_open:true,..Default::default()},false,&[]);}
        assert_eq!(j.deployment,1.0);assert!(j.interlocked());
        for _ in 0..30 {j.advance(&p,0.1,0.0,Feedback{door_open:true,stowed:true,..Default::default()},false,&[]);}
        assert_eq!(j.deployment,0.0);assert!(!j.interlocked());
    }
    #[test]
    fn pivot_turns_have_opposite_wheel_directions() {
        let p = profile();
        let mut j = Journey::new(&p, 0.0, 0.0);
        j.turn_to(30.0, 0.1);
        assert!(j.pose.wheels[0] < 0.2);
        assert!(j.pose.wheels[1] > 6.0);
        assert_eq!(j.local, p.outside(0.0));
    }
}
