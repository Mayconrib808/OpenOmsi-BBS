//! Accessibility journeys. Movement is owned by this state machine, never by the
//! walking passenger's step/climb or stuck-passenger recovery paths.
use super::*;
use serde::{Deserialize, Serialize};
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
static INTERLOCK: AtomicBool = AtomicBool::new(false);
pub fn beta_enabled() -> bool {
    true
}
pub fn interlocked() -> bool {
    INTERLOCK.load(Ordering::Acquire)
}

pub mod discovery;
pub mod geometry;
pub mod network;
pub mod pose;
mod passengers;

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

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Maneuver {
    pub at: Vec3,
    pub heading: f64,
    #[serde(default)]
    pub reverse: bool,
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
    /// Collision-checked forward/reverse legs and pivot turns for confined cabins.
    #[serde(default)]
    pub maneuvers: Vec<Maneuver>,
    pub bay_heading: f64,
    pub door_var: String,
    #[serde(default)]
    pub door_vars: Vec<String>,
    #[serde(default)]
    pub native: Option<discovery::NativeDeck>,
    #[serde(default)]
    pub fold: Option<discovery::FoldSeat>,
    #[serde(default)]
    pub replacement_meshes: Vec<String>,
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
            || !(if self.equipment==Equipment::NativeRamp{0.55..=2.2}else{1.25..=2.2}).contains(&self.platform_length)
            || !(0.65..=1.6).contains(&self.threshold.x.abs())
            || self.threshold.y.abs() > 20.0
            || self.threshold.z < 0.15
            || self.threshold.z > 2.0
            || self.bay.x.abs() > 1.1
            || (self.bay.z - self.threshold.z).abs() > 0.18
        {
            return Err("door, platform or wheelchair bay is outside supported dimensions".into());
        }
        if self.route.is_empty()
            || self.route.len() > 64
            || self.route.iter().any(|p| {
                !p.is_finite()
                    || p.x.abs() > 1.6
                    || p.y.abs() > 20.0
                    || (p.z - self.threshold.z).abs() > 0.18
            })
        {
            return Err("accessible route must stay on a continuous cabin floor".into());
        }
        if self.maneuvers.len()>64 || self.maneuvers.iter().any(|m|
            !m.at.is_finite()||!m.heading.is_finite()||m.at.x.abs()>1.6
                ||m.at.y.abs()>20.0||(m.at.z-self.threshold.z).abs()>0.18) {
            return Err("invalid accessible maneuver".into());
        }
        if self.maneuvers.last().is_some_and(|m|(m.at-self.bay).length()>0.005
            ||angle_delta(m.heading,self.bay_heading).abs()>0.1) {
            return Err("accessible maneuvers must end secured in the selected bay".into());
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
        ) && self.native.is_none() && (self.native_ground_var.is_none()
            || self.native_floor_var.is_none()
            || self.native_stowed_var.is_none()
            || (self.equipment == Equipment::NativeLift && self.native_height_var.is_none()))
        {
            return Err("native equipment requires ground AND floor endpoint feedback".into());
        }
        if let Some(n)=&self.native {
            if self.equipment!=Equipment::NativeRamp||n.mesh.is_empty()||n.corners.iter().any(|p|!p.is_finite())||!n.thickness.is_finite()||!(0.0..=0.20).contains(&n.thickness) {
                return Err("invalid native ramp binding".into());
            }
        }
        Ok(())
    }
    pub fn side(&self) -> f32 {
        self.threshold.x.signum()
    }
    pub fn platform(&self, height: f32) -> Vec3 {
        let centre_shift=if matches!(self.equipment,Equipment::AddedLift|Equipment::NativeLift){0.13}else{0.0};
        Vec3::new(
            self.threshold.x + self.side() * (self.platform_length * 0.5+centre_shift),
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
    pub fn ui_label(self)->&'static str {
        match self {
            Self::Waiting=>"Waiting for wheelchair assistance",
            Self::Deploying|Self::Raising|Self::Lowering=>"Preparing accessibility equipment",
            Self::ToPlatform|Self::ToBay=>"Boarding",
            Self::Securing=>"Securing the wheelchair",
            Self::Riding=>"Riding in the wheelchair bay",
            Self::Unsecuring=>"Preparing to alight",
            Self::ToExit|Self::Leaving=>"Alighting",
            Self::Finished=>"Alighting complete",
        }
    }
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
    pub surface: Option<[Vec3;3]>,
    pub bay_ready: Option<bool>,
    pub floor_normal:Option<Vec3>,
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
    // Ordinary passengers are asked to yield first. After a short stall the
    // wheelchair gets priority over their cosmetic collision capsules, never
    // over the bus structure or the equipment's physical interlocks.
    crowd_wait: f32,
    crowd_priority: bool,
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
            crowd_wait: 0.0,
            crowd_priority: false,
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
        if matches!(to, Stage::Riding | Stage::Finished) {
            self.crowd_wait = 0.0;
            self.crowd_priority = false;
        }
    }
    pub fn interlocked(&self) -> bool {
        self.deployment > 0.0001
            || !matches!(self.stage, Stage::Waiting | Stage::Riding | Stage::Finished)
    }
    pub fn crowd_priority_active(&self)->bool {self.crowd_priority}
    pub fn doorway_busy(&self)->bool {
        !matches!(self.stage,Stage::Waiting|Stage::Riding|Stage::Finished)
    }
    fn crowd_allows_motion(&mut self, blocked:bool, dt:f32)->bool {
        if !blocked {self.crowd_wait=0.0;return true;}
        if self.crowd_priority{return true;}
        self.crowd_wait+=dt.clamp(0.0,0.1);
        if self.crowd_wait<1.5 {
            self.hold_reason="Passagem ocupada: solicitando espaço aos passageiros";
            self.pose.moving=0.0;
            return false;
        }
        self.crowd_priority=true;
        log::info!("WHEELCHAIR crowd_yield_timeout stage={:?} position={:?}: passenger collision softened; bus safety interlocks retained",self.stage,self.local);
        true
    }
    /// Differential drive: each wheel follows its own travelled arc. The push
    /// cycle is distance based, so stopped hands never continue propelling.
    fn drive(&mut self, target: Vec3, dt: f32, blockers: &[Vec3], reverse:bool) -> bool {
        let delta = target - self.local;
        if delta.length() < 0.002 { return true; }
        // The old drive() tested only XY at completion. On auxiliary lifts or
        // multi-level interiors (e.g. MAN DL09), two route vertices can share XY
        // while their cabin-floor heights differ; that skipped a vertical step
        // and sent the chair to the next corridor waypoint below the floor.
        let planar = delta.truncate().length();
        if planar < 0.002 {
            // Do not teleport a chair through a deck or across a missing floor.
            if delta.z.abs() > 0.55 {
                self.hold_reason = "Aguardando nivelamento da plataforma";
                return false;
            }
            self.local.z += delta.z.clamp(-dt * 1.2, dt * 1.2);
            self.hold_reason = "";
            self.pose.moving = 0.0;
            return (target.z - self.local.z).abs() < 0.002;
        }
        let wanted = (delta.x as f64).atan2(delta.y as f64).to_degrees()+if reverse{180.0}else{0.0};
        let yaw = angle_delta(self.heading, wanted).clamp(-55.0 * dt as f64, 55.0 * dt as f64);
        let heading = (self.heading + yaw).rem_euclid(360.0);
        let ds = if angle_delta(heading, wanted).abs() < 1.0 {
            (0.48 * dt).min(delta.truncate().length())
        } else {
            0.0
        };
        let mut next = self.local + delta.truncate().normalize_or_zero().extend(0.0) * ds;
        if ds>0.0 {
            let rise=delta.z*(ds/planar).min(1.0);
            next.z=self.local.z+rise.clamp(-dt * 1.2,dt * 1.2);
        }
        // These blockers are *people*, not vehicle meshes. Ask the crowd to
        // move first; never strand a whole trip because an inspector or an
        // ordinary passenger is frozen on a lift. The geometry planner still
        // certifies the cabin and the lift/door/brake interlocks still apply.
        // A standing passenger on a different step or the lift's lower
        // level must not make the chair wait indefinitely. The 0.58 m
        // horizontal envelope still keeps same-floor bodies from overlapping;
        // after a short wait the controller prioritises the chair over
        // *cosmetic* pedestrian collisions, never the vehicle geometry.
        let crowd_blocked = blockers.iter().any(|p|
            (p.z-next.z).abs()<0.72 && (p.truncate()-next.truncate()).length()<0.58);
        if !self.crowd_allows_motion(crowd_blocked,dt){return false;}
        self.hold_reason = "";
        self.heading = heading;
        self.local = next;
        let turn = yaw.to_radians() as f32 * 0.33;
        for side in 0..2 {
            let distance = (if reverse{-ds}else{ds}) + if side == 0 { turn } else { -turn };
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
            && (target.z-self.local.z).abs()<0.002
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
    fn align_ramp(&mut self,profile:&Profile,feedback:Feedback) {
        if matches!(profile.equipment,Equipment::AddedRamp|Equipment::NativeRamp)
            &&matches!(self.stage,Stage::ToPlatform|Stage::ToBay|Stage::ToExit|Stage::Leaving) {
            let t=((self.local.x-profile.threshold.x)*profile.side()/profile.platform_length).clamp(0.0,1.0);
            if t>0.0{self.local.z=discovery::ramp_height(profile,self.local.truncate(),self.ground_height,feedback.surface);}
        }
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
        self.hold_reason = "";
        if self.stage == Stage::Riding && self.deployment <= 0.0 && !self.alight_requested {
            return;
        }
        if (!matches!(self.stage, Stage::Waiting | Stage::Riding | Stage::Finished)
            || self.deployment > 0.0001)
            && (speed.abs() > 0.025 || (!feedback.door_open && (self.doorway_busy() || (!native && self.bridge_deployment>0.001))))
        {
            self.hold_reason = if speed.abs() > 0.025 {
                "Aguardando ônibus completamente parado"
            } else {
                "Aguardando abertura da porta acessível"
            };
            return;
        }
        if native && ramp && matches!(self.stage,Stage::ToPlatform|Stage::ToBay|Stage::ToExit|Stage::Leaving)
            && (!feedback.ground_ready||!feedback.floor_ready) {
            self.hold_reason="Aguardando rampa alinhada ao piso e apoiada no solo";return;
        }
        if matches!(self.stage,Stage::ToBay|Stage::Securing)
            && blockers.iter().any(|at| (at.z-profile.bay.z).abs()<0.72
                && (at.truncate()-profile.bay.truncate()).length()<0.58) {
            self.timer=0.0;
            self.hold_reason="Área reservada ocupada: aguardando liberação";
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
                if assist&&(self.local-profile.outside(self.ground_height)).truncate().length()>1.5 {
                    self.hold_reason="Reposicione a porta acessível perto do passageiro";return;
                }
                if assist && speed.abs() < 0.025 && feedback.door_open {
                    if ramp && !native
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
                if feedback.bay_ready==Some(false){self.hold_reason="Aguardando recolhimento do banco retrátil";return;}
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
                let reached=self.drive(p, dt, blockers, false);
                // The middle of a sloped ramp is ABOVE the road by design.
                // The ramp's sampled surface owns Z here; only a lift must
                // actually reach its measured platform height before moving
                // to the cabin. Keeping a 3-D target check on the ramp leaves
                // the chair at ToPlatform forever.
                if reached || (ramp && (self.local-p).truncate().length()<0.002) {
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
                if !profile.maneuvers.is_empty() && self.route_at>0 {
                    let m=&profile.maneuvers[self.route_at-1];
                    if self.drive(m.at,dt,blockers,m.reverse)&&self.turn_to(m.heading,dt) {
                        if self.route_at==profile.maneuvers.len() {
                            self.boarded=true;self.transition(Stage::Securing);
                        }else{self.route_at+=1;}
                    }
                    self.align_ramp(profile,feedback);return;
                }
                let target = if self.route_at == 0 {
                    profile.threshold
                } else if self.route_at <= profile.route.len() {
                    profile.route[self.route_at - 1]
                } else {
                    profile.bay
                };
                if self.drive(target, dt, blockers, false) {
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
                if native&&self.deployment>0.0&&!feedback.stowed {self.hold_reason="Aguardando recolhimento do equipamento original";}
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
                if !profile.maneuvers.is_empty() {
                    let n=profile.maneuvers.len();
                    if self.route_at<n {
                        let source=&profile.maneuvers[n-1-self.route_at];
                        let (at,heading)=if self.route_at+1==n {
                            (profile.threshold,if profile.side()>0.0{270.0}else{90.0})
                        }else{
                            let previous=&profile.maneuvers[n-2-self.route_at];
                            (previous.at,previous.heading)
                        };
                        if self.drive(at,dt,blockers,!source.reverse)&&self.turn_to(heading,dt){self.route_at+=1;}
                    }else if self.drive(profile.platform(profile.threshold.z),dt,blockers,true) {
                        self.transition(if ramp{Stage::Leaving}else{Stage::Lowering});
                    }
                    self.align_ramp(profile,feedback);return;
                }
                let n = profile.route.len();
                let target = if self.route_at < n {
                    profile.route[n - self.route_at - 1]
                } else if self.route_at == n {
                    profile.threshold
                } else {
                    profile.platform(profile.threshold.z)
                };
                if self.drive(target, dt, blockers, true) {
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
                if self.drive(profile.outside(self.ground_height), dt, blockers, true) {
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
        self.align_ramp(profile,feedback);
    }
}

pub struct Wheelchairs {
    pub enabled: bool,
    pub profile: Option<Profile>,
    pub profiles: Vec<Profile>,
    pub access_profiles: Vec<Profile>,
    /// Every wheelchair user keeps the same identity through stops and connections.
    pub mobile: HashMap<u32, passengers::Mobile>,
    pub riders: HashMap<u32, passengers::Rider>,
    pub spawn_percent: f32,
    pub assisted: bool,
    pub special_stop_signal: bool,
    pub stop_signal_variables:Vec<String>,
    // Edge of the wheelchair call button, not one new press per simulation
    // frame. Repeated int_haltewunsch presses kept some native lamps latched.
    pub stop_request_armed: bool,
    // The Bridge owns these bus light variables only from its own wheelchair
    // request until the last requesting chair has finished alighting.
    pub stop_light_written: bool,
    pub journey: Option<Journey>,
    pub person: Option<u32>,
    pub status: String,
    pub feedback: Feedback,
    pub equipment: Option<EquipmentPose>,
    pub auto_spawn: bool,
    pub bay_occupied: bool,
    floor_surface:Option<discovery::FloorSurface>,
    crowd_targets: HashMap<u32,(Vec3,bool)>,
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
            profiles: Vec::new(),
            access_profiles: Vec::new(),
            mobile: HashMap::new(),
            riders: HashMap::new(),
            spawn_percent: 3.0,
            assisted: false,
            special_stop_signal: false,
            stop_signal_variables:Vec::new(),
            stop_request_armed: false,
            stop_light_written: false,
            journey: None,
            person: None,
            status: String::new(),
            feedback: Feedback::default(),
            equipment: None,
            auto_spawn: false,
            bay_occupied: false,
            floor_surface:None,
            crowd_targets: HashMap::new(),
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
            .flatten().or_else(||self.riders.get(&id).map(|r|r.journey.pose))
            .or_else(||self.mobile.get(&id).map(|m|m.pose))
            .or_else(||self.remote_poses.get(&id).copied())
    }
    /// Reflect actual accessibility requests into the model's own dedicated
    /// warning indicator if it has one, and release only the indicator that we
    /// asserted when that request is complete. The HUD alert is independent of
    /// these optional variables and works on every supported vehicle.
    pub fn sync_stop_signal(&mut self, v: &mut VehicleInstance) {
        if self.stop_signal_variables.is_empty() {
            self.stop_light_written = false;
            return;
        }
        if self.wants_alight() {
            for name in &self.stop_signal_variables {
                if v.var(name).is_some() {
                    v.set_var(name, 1.0);
                }
            }
            self.stop_light_written = true;
        } else if self.stop_light_written {
            for name in &self.stop_signal_variables {
                if v.var(name).is_some() {
                    v.set_var(name, 0.0);
                }
            }
            self.stop_light_written = false;
            log::info!("WHEELCHAIR stop_signal_cleared after completed alighting");
        }
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
        let mut feedback=if p.native.is_some(){discovery::feedback(v,p)}else{Feedback {
            door_open: discovery::doors_ready(v,p),
            ground_ready: ready(&p.native_ground_var),
            floor_ready: ready(&p.native_floor_var),
            equipment_height: p.native_height_var.as_ref().and_then(|n| v.var(n)),
            stowed: ready(&p.native_stowed_var),
            surface:None,bay_ready:p.fold.as_ref().map(|_|discovery::fold_ready(v,p)),floor_normal:None,
        }};
        if let (Some(floor),Some(j))=(&self.floor_surface,&self.journey) {
            feedback.floor_normal=Some(floor.normal(j.local,j.heading as f32,p,j.ground_height,feedback.surface,j.deployment>0.0));
        }
        feedback
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
        self.wheelchairs.stop_request_armed = false;
        self.wheelchairs.stop_light_written = false;
        self.wheelchairs.crowd_targets.clear();
        self.wheelchairs.equipment = None;
        self.wheelchairs.profiles.clear();
        self.wheelchairs.access_profiles.clear();
        self.wheelchairs.riders.clear();
        self.wheelchairs.assisted = false;
        self.wheelchairs.stop_signal_variables = passengers::special_signal_variables(v);
        self.wheelchairs.special_stop_signal = !self.wheelchairs.stop_signal_variables.is_empty();
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
                    "Acessibilidade: {}",
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
                let detected=discovery::detect(v,cabin);
                log::info!("WHEELCHAIR DETECTION {}",serde_json::to_string(&detected).unwrap_or_default());
                self.wheelchairs.profiles=detected.profiles;
                self.wheelchairs.access_profiles=detected.access_profiles;
                if let Some(profile)=detected.profile {
                    self.wheelchairs.status=format!("Acessibilidade automática: {}",profile.name);Some(profile)
                }else{self.wheelchairs.status="Acessibilidade: porta, área ou manobra ainda precisam de calibração".into();None}
            }
            Err(e) => {
                log::warn!("WHEELCHAIR profile rejected: {e}");
                self.wheelchairs.status = format!("Perfil rejeitado: {e}");
                None
            }
        };
        if self.wheelchairs.profiles.is_empty() {
            self.wheelchairs.profiles.extend(self.wheelchairs.profile.clone());
        }
        self.wheelchairs.floor_surface=self.wheelchairs.profile.as_ref().map(|_|discovery::FloorSurface::load(v));
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
        self.tick_wheelchair_mobility(dt);
        self.schedule_wheelchair_journeys(world, bus, buses);
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
        let (Some(id), Some(mut journey)) =
            (self.wheelchairs.person, self.wheelchairs.journey.take())
        else {
            return;
        };
        let Some(i) = self.people.iter().position(|p| p.id == id) else {
            self.wheelchairs.person = None;
            return;
        };
        if matches!(journey.stage,Stage::Waiting|Stage::Deploying|Stage::Finished) {
            if let Some(at) = journey.outside_world {
                journey.local = bn.to_local(at);
                if journey.stage!=Stage::Finished {journey.ground_height=journey.local.z;}
                // A person left at the curb keeps their world orientation when
                // the bus turns or drives away.
                journey.heading = angle_delta(bn.heading, self.people[i].heading);
            }
        }
        if commands & ASSIST != 0 { self.wheelchairs.assisted = true; }
        let mut alight_surface_loaded=true;
        if !matches!(journey.stage,Stage::Riding|Stage::Waiting|Stage::Finished) && bus.speed.abs()<0.025 {
            let outside=bn.world(profile.outside(0.0));
            if let Some(height)=world.walk_height_near(outside.x,outside.y,bn.pos.z) {
                journey.ground_height=bn.to_local(DVec3::new(outside.x,outside.y,height)).z;
            }
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
            if journey.stops >= 2 && self.pax(i).is_none_or(|p|p.dest.is_none()) {
                journey.alight_requested = true;
            }
            if journey.alight_requested {
                if bus.speed.abs()<0.025 {
                    let outside=bn.world(profile.outside(0.0));
                    if let Some(height)=world.walk_height_near(outside.x,outside.y,bn.pos.z) {
                        let ground=bn.to_local(DVec3::new(outside.x,outside.y,height)).z;
                        if (profile.threshold.z-ground).abs()<=1.6{journey.ground_height=ground;}else{alight_surface_loaded=false;}
                    }else{alight_surface_loaded=false;}
                    if !alight_surface_loaded{journey.hold_reason="Aguardando superfície de desembarque carregada e ao alcance";}
                }
            }
        }
        self.clear_wheelchair_corridor(id,&profile,&journey,bn,world,dt);
        let blockers: Vec<Vec3> = self
            .people
            .iter()
            .filter(|p| p.id != id)
            .filter(|p| (p.position - bn.world(journey.local)).length_squared() < 16.0)
            .map(|p| bn.to_local(p.position))
            .collect();
        self.wheelchairs.bay_occupied=self.people.iter().any(|p|p.id!=id&&matches!(p.place,Place::Bus(BusId::Player,_))
            &&(bn.to_local(p.position).truncate()-profile.bay.truncate()).length()<0.85);
        let bay_blocked=journey.stage==Stage::Waiting&&self.wheelchairs.bay_occupied;
        if journey.stage==Stage::Waiting && bus.speed.abs()<0.025 {
            let target=profile.outside(journey.ground_height);
            let at=bn.world(journey.local);
            let to=bn.world(target);
            if !world.collision().ray_blocked(at+DVec3::Z*0.45,to+DVec3::Z*0.45,0.30,0.60) {
                let before=journey.local;
                journey.drive(target,dt,&blockers,false);
                let next=bn.world(journey.local);
                if let Some(z)=world.walk_height_near(next.x,next.y,at.z).filter(|z|(*z-at.z).abs()<0.08) {
                    journey.local.z=bn.to_local(DVec3::new(next.x,next.y,z)).z;
                    journey.ground_height=journey.local.z;
                } else { journey.local=before; journey.pose.moving=0.0; }
                journey.outside_world=Some(bn.world(journey.local));
            }
        }
        let native=matches!(profile.equipment,Equipment::NativeRamp|Equipment::NativeLift);
        let ready_to_board=native && self.wheelchairs.feedback.ground_ready
            && self.wheelchairs.feedback.floor_ready;
        // The initial bay check must use the same short crowd yield as motion.
        // Otherwise a full standing area blocks before drive() can resolve it.
        let occupied_before_assist=bay_blocked
            &&(!(self.wheelchairs.assisted||ready_to_board)
                ||!journey.crowd_allows_motion(true,dt));
        journey.advance(
            &profile,
            dt,
            bus.speed,
            self.wheelchairs.feedback,
            (self.wheelchairs.assisted || ready_to_board) && alight_surface_loaded && !occupied_before_assist,
            &blockers,
        );
        if !alight_surface_loaded {journey.hold_reason="Aguardando superfície de desembarque carregada e ao alcance";}
        if occupied_before_assist {journey.hold_reason="Área reservada ocupada: aguardando liberação";}
        // The bay is a volume, not a seat index: reserve all overlapping standing
        // places, and never take a place from somebody who is already there.
        if journey.stage!=Stage::Finished {
            if let Some(seats) = self.seats.get_mut(&BusId::Player) {
                for (k, s) in bn.cabin.seats.iter().enumerate() {
                    if (!s.seated||profile.fold.is_some()) && (s.floor.truncate() - profile.bay.truncate()).length() < 0.85 {
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
        p.tilt = if matches!(journey.stage,Stage::Waiting|Stage::Deploying|Stage::Finished) {
            Mat4::IDENTITY
        } else {
            let outward=(journey.local.x-profile.threshold.x)*profile.side();
            let local_tilt=if let Some(normal)=self.wheelchairs.feedback.floor_normal {
                Mat4::from_quat(glam::Quat::from_rotation_arc(Vec3::Z,normal))
            }else if matches!(profile.equipment,Equipment::NativeRamp|Equipment::AddedRamp)&&outward>0.0&&outward<profile.platform_length {
                let normal=self.wheelchairs.feedback.surface.map(|s|(s[1]-s[0]).cross(s[2]-s[0]).normalize_or_zero()).map(|n|if n.z<0.0{-n}else{n})
                    .unwrap_or_else(||Vec3::new(profile.side()*(profile.threshold.z-journey.ground_height)/profile.platform_length,0.0,1.0).normalize());
                Mat4::from_quat(glam::Quat::from_rotation_arc(Vec3::Z,normal))
            }else{Mat4::IDENTITY};
            bn.rot*local_tilt*Mat4::from_rotation_z(bn.heading.to_radians() as f32)
        };
        p.interior = if journey.boarded { bn.interior } else { 0.0 };
        p.vel = DVec2::ZERO;
        p.activity = Activity::Sit;
        p.why = "wheelchair accessibility controller";
        if let State::Pax(pax)=&mut p.state {
            pax.pos=if journey.boarded{journey.local.as_dvec3()}else{p.position};
            pax.inside=journey.boarded.then_some(BusId::Player);
            pax.yaw=(if journey.boarded{journey.heading}else{p.heading}).to_radians();
        }
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
        self.wheelchairs.journey = Some(journey);
        self.finish_wheelchair_service(i, bn, buses, world);
    }
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    pub fn profile() -> Profile {
        Profile {
            schema: 1,
            name: "test".into(),
            equipment: Equipment::AddedLift,
            threshold: Vec3::new(1.16, 0.0, 0.97),
            bay: Vec3::new(-0.72, 0.0, 0.97),
            route: vec![Vec3::new(0.35, 0.0, 0.97)],
            maneuvers:Vec::new(),
            bay_heading: 180.0,
            door_var: "door_2".into(),
            door_vars:Vec::new(),native:None,fold:None,replacement_meshes:Vec::new(),
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
    fn independent_bays_keep_their_passenger_and_alighting_request() {
        let first=profile();
        let mut second=first.clone();second.bay.y+=1.8;
        let mut wc=Wheelchairs::default();
        wc.profiles=vec![first.clone(),second.clone()];
        let mut j=Journey::new(&first,0.18,270.0);
        j.stage=Stage::Riding;j.boarded=true;
        wc.riders.insert(17,passengers::Rider{profile:first,journey:j,reserved:vec![1,2]});
        assert_eq!(wc.free_profiles().count(),1);
        assert_eq!(wc.free_profiles().next().unwrap().bay,second.bay);
        let mut j=Journey::new(&second,0.18,270.0);
        j.stage=Stage::Riding;j.boarded=true;
        wc.riders.insert(23,passengers::Rider{profile:second,journey:j,reserved:vec![4,5]});
        assert_eq!(wc.free_profiles().count(),0);
        assert!(wc.request_alight(23));
        assert!(wc.wants_alight());
        assert!(!wc.riders[&17].journey.alight_requested);
        assert_eq!(wc.riders[&17].reserved,vec![1,2]);
        assert!(!wc.request_alight(99));
    }
    #[test]
    fn a_confined_maneuver_retraces_reverse_legs_and_pivots_on_alighting() {
        let mut p=profile();p.bay_heading=90.0;
        let centre=p.route[0];
        p.maneuvers=vec![
            Maneuver{at:centre,heading:270.0,reverse:false},
            Maneuver{at:centre,heading:90.0,reverse:false},
            Maneuver{at:p.bay,heading:90.0,reverse:true},
        ];
        p.validate().unwrap();
        let mut j=Journey::new(&p,0.18,270.0);
        run(&mut j,&p,Stage::Riding);
        assert!((j.local-p.bay).length()<0.003);
        assert!(angle_delta(j.heading,90.0).abs()<0.1);
        j.alight_requested=true;
        run(&mut j,&p,Stage::Finished);
        assert!((j.local-p.outside(0.18)).length()<0.003);
        assert!(!j.boarded);
    }
    #[test]
    fn two_routes_to_one_bay_do_not_double_its_capacity() {
        let p=profile();let mut alternative=p.clone();alternative.threshold.y+=0.6;
        alternative.threshold.x=-alternative.threshold.x;
        let mut wc=Wheelchairs::default();wc.profiles=vec![p.clone()];wc.access_profiles=vec![alternative];
        assert_eq!(wc.profiles.len(),1);
        assert_eq!(wc.free_profiles().count(),2);
        assert!(wc.supports_bus(BusId::Player,Some(1.0)));
        assert!(wc.supports_bus(BusId::Player,Some(-1.0)));
        let j=Journey::new(&p,0.18,270.0);
        wc.profile=Some(p);wc.person=Some(1);wc.journey=Some(j);
        assert_eq!(wc.free_profiles().count(),0);
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
                surface:None,bay_ready:None,floor_normal:None,
            },
            false,
            &[],
        );
        assert_eq!(j.stage, Stage::ToPlatform);
    }
    #[test]
    fn stationary_passenger_at_lift_yields_then_chair_completes_boarding() {
        let p = profile();
        let mut j = Journey::new(&p, 0.0, 270.0);
        j.transition(Stage::ToPlatform);
        let before = j.local;
        // The human at the lift must not keep the bus interlocked forever.
        // Start with the stationary person directly in front of the chair.
        let person = before + Vec3::new(-0.1, 0.0, 0.0);
        let ready = Feedback { door_open: true, ..Default::default() };
        for _ in 0..10 {
            j.advance(&p, 0.1, 0.0, ready, true, &[person]);
        }
        assert_eq!(j.local,before,"normal crowd yielding happens first");
        assert_eq!(j.stage,Stage::ToPlatform);
        for _ in 0..12000 {
            j.advance(&p, 0.1, 0.0, ready, true, &[person]);
            if j.stage==Stage::Riding {break;}
        }
        assert_eq!(j.stage,Stage::Riding,"standing passenger must not strand a trip");
        assert!((j.local-p.bay).length()<0.01,"real route to reserved bay is completed");
        assert!(!j.crowd_priority,"soft collision priority ends when the wheelchair is secured");
    }
    #[test]
    fn occupied_reserved_bay_never_times_out_into_a_secured_overlap() {
        let p=profile();let mut j=Journey::new(&p,0.18,270.0);
        j.stage=Stage::ToBay;j.local=p.threshold;j.route_at=1;
        let before=j.local;
        for _ in 0..1200 {
            j.advance(&p,0.1,0.0,Feedback{door_open:true,..Default::default()},true,&[p.bay]);
        }
        assert_eq!(j.stage,Stage::ToBay);assert_eq!(j.local,before);
        assert!(!j.pose.secured);
        run(&mut j,&p,Stage::Riding);
        assert!((j.local-p.bay).length()<0.003);
    }
    #[test]
    fn crowd_priority_does_not_override_brakes_or_closed_doors() {
        let p=profile();
        let mut j=Journey::new(&p,0.0,270.0);
        j.transition(Stage::ToPlatform);
        let person=j.local+Vec3::new(-0.1,0.0,0.0);
        let open=Feedback{door_open:true,..Default::default()};
        for _ in 0..20 {j.advance(&p,0.1,0.0,open,true,&[person]);}
        assert!(j.crowd_priority);
        let before=j.local;
        for _ in 0..25 {
            j.advance(&p,0.1,1.0,open,true,&[person]);
            j.advance(&p,0.1,0.0,Feedback::default(),true,&[person]);
        }
        assert_eq!(j.local,before,"bus motion and shut doors remain hard interlocks");
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
    #[test]
    fn the_ramp_approach_stays_on_ground_and_the_cabin_keeps_its_own_height() {
        let mut p=profile();p.equipment=Equipment::NativeRamp;p.threshold.z=0.30;p.bay.z=0.36;
        for point in &mut p.route{point.z=0.34;}let ground=0.10;let mut j=Journey::new(&p,ground,270.0);
        let surface=[p.threshold,p.threshold+Vec3::Y,p.threshold+Vec3::new(p.platform_length,0.0,ground-p.threshold.z)];
        let f=Feedback{door_open:true,ground_ready:true,floor_ready:true,surface:Some(surface),..Default::default()};let mut biggest=0.0_f32;
        for n in 0..12000{let before=j.local;j.advance(&p,1.0/60.0,0.0,f,n==0,&[]);biggest=biggest.max((j.local-before).length());
            if j.stage==Stage::ToPlatform&&(j.local.x-p.threshold.x)*p.side()>p.platform_length+0.01{assert!((j.local.z-ground).abs()<0.001);}
            if j.stage==Stage::Riding{break;}}
        assert_eq!(j.stage,Stage::Riding);assert!((j.local.z-p.bay.z).abs()<0.001);assert!(biggest<0.012,"vertical jump: {biggest}");
    }
    #[test]
    fn the_platform_waits_for_a_folding_seat() {
        let p=profile();let mut j=Journey::new(&p,0.0,270.0);j.transition(Stage::Deploying);
        for _ in 0..100{j.advance(&p,0.1,0.0,Feedback{door_open:true,bay_ready:Some(false),..Default::default()},false,&[]);}
        assert_eq!(j.stage,Stage::Deploying);assert_eq!(j.deployment,0.0);
    }
    #[test]
    fn a_ramp_can_close_its_original_door_to_stow_after_the_chair_is_clear() {
        let p=profile();let mut j=Journey::new(&p,0.0,270.0);j.deployment=1.0;j.stage=Stage::ToBay;assert!(j.doorway_busy());
        j.stage=Stage::Riding;assert!(!j.doorway_busy());assert!(j.interlocked());
        j.stage=Stage::Finished;assert!(!j.doorway_busy());assert!(j.interlocked());
    }
    #[test]
    fn backing_out_turns_both_wheels_backwards_without_turning_the_chair_around() {
        let p=profile();let mut j=Journey::new(&p,0.0,180.0);j.local=Vec3::ZERO;
        for _ in 0..110{j.drive(Vec3::Y,0.02,&[],true);}
        assert!(j.local.y>0.9);assert!((j.heading-180.0).abs()<0.1);assert_eq!(j.pose.direction,[-1.0,-1.0]);
    }

    #[test]fn native_stowing_with_closed_doors_clears_the_hold_after_retraction() {
        let mut p=profile();p.equipment=Equipment::NativeRamp;let mut j=Journey::new(&p,0.0,270.0);j.stage=Stage::Riding;j.deployment=1.0;
        j.advance(&p,0.02,0.0,Feedback::default(),false,&[]);assert!(j.interlocked());assert!(j.hold_reason.contains("recolhimento"));
        j.advance(&p,0.02,0.0,Feedback{stowed:true,..Default::default()},false,&[]);assert!(!j.interlocked());assert!(j.hold_reason.is_empty());
    }
    #[test]
    fn a_dl09_auxiliary_lift_cannot_skip_a_cabin_height_waypoint() {
        let mut j=Journey::new(&profile(),0.0,0.0);
        j.local=Vec3::new(0.2,0.5,0.16);
        let upper=Vec3::new(0.2,0.5,0.46);
        assert!(!j.drive(upper,0.02,&[],false));
        assert!(j.local.z>0.16 && j.local.z<upper.z);
        let mut reached=false;
        for _ in 0..100 { if j.drive(upper,0.02,&[],false){reached=true;break;} }
        assert!(reached);
        assert!((j.local-upper).length()<0.002);
        let before=j.local;
        assert!(!j.drive(Vec3::new(before.x,before.y,before.z+0.8),0.02,&[],false));
        assert_eq!(j.local,before,"unverified deck transition must not teleport");
    }

    #[test]
    fn wheelchair_stop_call_is_one_press_and_recovers_for_later_requests() {
        let p=profile();
        let mut wc=Wheelchairs::default();
        wc.special_stop_signal=true; // A bus with a physical lamp still needs the HUD.
        wc.stop_signal_variables=vec!["cockpit_light_kinderwagenwunsch".into(),"door_kinderwagenwunsch".into()];
        let mut first=Journey::new(&p,0.0,0.0);
        first.boarded=true;
        first.stage=Stage::Riding;
        wc.riders.insert(1,passengers::Rider{profile:p.clone(),journey:first,reserved:vec![]});
        assert!(!wc.wants_alight());
        assert!(!wc.take_new_stop_request());
        assert!(wc.request_alight(1));
        assert!(wc.wants_alight(),"special signal must not hide the request");
        assert!(wc.take_new_stop_request(),"first stop request was lost");
        for _ in 0..100 {assert!(!wc.take_new_stop_request(),"request was retriggered every tick");}

        let mut second=Journey::new(&p,0.0,0.0);
        second.boarded=true;
        second.stage=Stage::Riding;
        wc.riders.insert(2,passengers::Rider{profile:p.clone(),journey:second,reserved:vec![]});
        assert!(wc.request_alight(2));
        wc.riders.get_mut(&1).unwrap().journey.stage=Stage::Finished;
        assert!(wc.wants_alight(),"first rider must not clear second rider's signal");
        assert!(!wc.take_new_stop_request());
        wc.riders.get_mut(&2).unwrap().journey.stage=Stage::Finished;
        assert!(!wc.wants_alight(),"light must be releasable when the last rider finishes");
        assert!(!wc.take_new_stop_request());
        wc.riders.remove(&1);
        wc.riders.remove(&2);
        let mut third=Journey::new(&p,0.0,0.0);
        third.boarded=true;
        third.stage=Stage::Riding;
        wc.riders.insert(3,passengers::Rider{profile:p,journey:third,reserved:vec![]});
        assert!(wc.request_alight(3));
        assert!(wc.take_new_stop_request(),"future service must ring again exactly once");
    }

    #[test]fn assistance_does_not_move_a_chair_left_far_behind_the_bus() {
        let p=profile();let mut j=Journey::new(&p,0.0,270.0);j.local.y-=8.0;let before=j.local;
        j.advance(&p,0.02,0.0,Feedback{door_open:true,..Default::default()},true,&[]);assert_eq!(j.stage,Stage::Waiting);assert_eq!(j.local,before);assert!(!j.interlocked());
    }

}
