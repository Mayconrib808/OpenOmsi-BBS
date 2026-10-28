//! Full local vehicle authority. The original owner keeps a passive BCS/passenger
//! proxy; the visiting driver owns scripts, physics, normal input, picking and cameras.
use crate::{humans::BusId, App, Args, Player, World};
use omsi_lan::vehicle_transfer::{Message, Snapshot};
use std::{
    collections::{HashMap, HashSet},
    sync::{mpsc, Arc},
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};
use winit::keyboard::KeyCode;
pub(crate) const DRIVER_SEAT: u16 = 1023;
struct Owner {
    peer: u32,
    token: u64,
    uid: u64,
    bus: String,
    frozen: bool,
    active: bool,
    seen: Instant,
    sent: Instant,
    triggers: Vec<String>,
    motion: Option<Motion>,
}
struct Motion {
    from: (glam::DVec3, f64, f32, f32),
    to: (glam::DVec3, f64, f32, f32),
    started: Instant,
    duration: f64,
}
impl Motion {
    fn apply(&self, vehicle: &mut omsi_sim::VehicleInstance) {
        self.apply_elapsed(vehicle, self.started.elapsed().as_secs_f64());
    }
    fn apply_elapsed(&self, vehicle: &mut omsi_sim::VehicleInstance, elapsed: f64) {
        let t = (elapsed / self.duration.max(0.001)).clamp(0.0, 1.0);
        vehicle.position = self.from.0.lerp(self.to.0, t);
        vehicle.heading = self.from.1 + ((self.to.1-self.from.1+180.0).rem_euclid(360.0)-180.0)*t;
        vehicle.pitch = self.from.2 + (self.to.2-self.from.2)*t as f32;
        vehicle.bank = self.from.3 + (self.to.3-self.from.3)*t as f32;
    }
}
struct Visitor {
    owner: u32,
    token: u64,
    uid: u64,
    bus: String,
    args: Args,
    world: Arc<World>,
    loading: Option<mpsc::Receiver<anyhow::Result<crate::spawn::PreparedPlayer>>>,
    prepared: Option<crate::spawn::PreparedPlayer>,
    parked: Option<Player>,
    original_bus: Option<String>,
    foot: Option<crate::on_foot::OnFoot>,
    original_duty: Option<crate::schedule::PlayerDuty>,
    active: bool,
    returning: bool,
    actors: HashSet<u32>,
    desk: u32,
    cash: omsi_lan::coop::Desk,
    cash_view: omsi_lan::coop::CashView,
    images: Vec<Option<(u32, u32, bool, Arc<Vec<u8>>)>>,
    seen: Instant,
    sent: Instant,
    started: Instant,
}
#[derive(Clone)]
pub(crate) struct RemoteRoute {
    pub line: String,
    pub tour: String,
    pub trip: String,
    pub stop: usize,
    pub seen: Instant,
}
#[derive(Default)]
pub(crate) struct DriveState {
    requests: HashMap<u32, (u64, Instant)>,
    request: Option<(u32, u64, Instant)>,
    owner: Option<Owner>,
    visitor: Option<Visitor>,
    controllers: HashMap<u32, (u32, u64, Instant)>,
    authority_sent: Option<Instant>,
    route_sent: Option<Instant>,
    pub(crate) routes: HashMap<u32, RemoteRoute>,
    pub(crate) passenger_duty: Option<(u32, crate::schedule::PlayerDuty)>,
}
impl DriveState {
    pub(crate) fn controller(&self, owner: u32) -> u32 {
        self.controllers.get(&owner).filter(|(_,_,t)| t.elapsed() < Duration::from_secs(5))
            .map_or(owner, |(driver,_,_)| *driver)
    }
    pub(crate) fn controllers(&self) -> Vec<(u32,u32)> {
        self.controllers.iter().filter(|(_,(_,_,t))|t.elapsed()<Duration::from_secs(5))
            .map(|(owner,(driver,_,_))|(*owner,*driver)).collect()
    }
    pub(crate) fn owner_active(&self) -> bool {
        self.owner.as_ref().is_some_and(|o| o.frozen)
    }
    pub(crate) fn visitor_active(&self) -> bool {
        self.visitor.as_ref().is_some_and(|v| v.active)
    }
    pub(crate) fn busy(&self) -> bool {
        self.owner.is_some() || self.visitor.is_some()
    }
    pub(crate) fn owner_peer_active(&self) -> Option<u32> {
        self.owner.as_ref().filter(|o| o.active).map(|o| o.peer)
    }
    pub(crate) fn visitor_owner(&self) -> Option<u32> {
        self.visitor.as_ref().filter(|v| v.active).map(|v| v.owner)
    }
}
fn nonce() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(1, |d| d.as_nanos() as u64)
        .max(1)
}
fn bus_name(app: &App, p: &Player) -> String {
    let file = &p.vehicle.ty.def.path;
    omsi_cfg::content_roots()
        .iter()
        .chain(std::iter::once(&app.args.root))
        .find_map(|r| file.strip_prefix(r).ok())
        .unwrap_or(file)
        .to_string_lossy()
        .replace('\\', "/")
}
impl App {
    fn drive_announce_authority(&mut self, token: u64, driver: u32, active: bool) {
        let Some(lan) = self.net.lan.as_ref() else { return; };
        let owner=lan.my_id;
        let peers: Vec<_> = lan.peers().map(|p|p.pose.id).collect();
        if active { self.net.coop.drive.controllers.insert(owner,(driver,token,Instant::now())); }
        else { self.net.coop.drive.controllers.remove(&owner); }
        self.net.coop.drive.authority_sent=Some(Instant::now());
        for peer in peers {
            if let Some(lan)=self.net.lan.as_mut(){
                lan.command(peer,&omsi_lan::vehicle_transfer::authority_notice(token,driver,active));
            }
        }
    }
    pub(crate) fn drive_send(&mut self, to: u32, message: Message) {
        if let (Some(lan), Some(bytes)) = (
            self.net.lan.as_mut(),
            omsi_lan::vehicle_transfer::encode(&message),
        ) {
            lan.publish_handover(to, bytes);
        }
    }
    fn drive_stream_ready(&self, to: u32) -> bool {
        self.net.lan.as_ref().is_some_and(|l| l.handover_pending(to) < 2)
    }
    pub(crate) fn drive_menu_allowed(&self,id:&str)->bool {
        if self.net.coop.drive.busy()&&matches!(id,"place"|"swap"|"switch"|"remove"|"reload"|"clearplaced"|"couple"|"uncouple"){return false;}
        !(self.net.coop.drive.owner_active()&&matches!(id,"reset"|"teleport"|"dest"|"duty"|"repair"|"refuel"|"wash"|"vehicle"|"mirror_hud_edit"|"tobus"))
    }
    pub(crate) fn drive_requested(&self, p: u32) -> bool {
        self.net
            .coop
            .drive
            .requests
            .get(&p)
            .is_some_and(|(_, t)| t.elapsed() < Duration::from_secs(30))
    }
    pub(crate) fn drive_peer(&self) -> Option<u32> {
        self.net.coop.drive.owner.as_ref().map(|o| o.peer)
    }
    pub(crate) fn drive_request(&mut self) {
        if self.net.coop.drive.busy() {
            return;
        }
        let Some(BusId::Ai(id)) = self.foot_bus() else {
            return;
        };
        let Some(owner) = crate::humans::remote_bus_player(id) else {
            return;
        };
        let n = nonce();
        self.net.coop.drive.request = Some((owner, n, Instant::now()));
        self.drive_send(owner, Message::Request { nonce: n });
        self.coop_notice(
            "Pedido de direção enviado. Aguarde a autorização e o carregamento do ônibus.",
            "Driver request sent. Waiting for authorization and the full vehicle.",
            "Fahreranfrage gesendet. Auf Freigabe und vollständiges Fahrzeug warten.",
        );
    }
    pub(crate) fn drive_grant(&mut self, peer: u32) {
        if self.net.coop.drive.busy()
            || !self.drive_requested(peer)
            || !self.coop_peer_inside(peer, omsi_lan::coop::Target::Player)
        {
            return;
        }
        let Some(p) = self.player.as_ref() else {
            return;
        };
        if p.vehicle.physics.velocity_kmh().abs() > 0.5 {
            return;
        }
        let request = self.net.coop.drive.requests[&peer].0;
        let token = nonce();
        let uid = p.uid;
        let now = Instant::now();
        let bus = bus_name(self, p);
        let hof = p
            .vehicle
            .host
            .hof
            .as_ref()
            .and_then(|h| h.path.file_stem())
            .map(|s| s.to_string_lossy().to_string());
        let paint = p
            .vehicle
            .host
            .paint_scheme
            .flatten()
            .and_then(|i| p.vehicle.ty.paint_schemes.get(i))
            .map(|s| s.name.clone());
        self.net.coop.drive.owner = Some(Owner {
            peer,
            token,
            uid,
            bus: bus.clone(),
            frozen: false,
            active: false,
            seen: now,
            sent: now,
            triggers: Vec::new(),
            motion: None,
        });
        self.drive_send(
            peer,
            Message::Offer {
                nonce: request,
                token,
                uid,
                bus,
                hof,
                paint,
            },
        );
    }
    pub(crate) fn drive_return(&mut self) {
        if self.net.coop.drive.owner_active() {
            // The visitor is now the only driver. The original BCS trip remains
            // passive here, but the former owner cannot reclaim the wheel.
            self.coop_notice(
                "Apenas o novo motorista pode devolver a direção.",
                "Only the current driver can return the wheel.",
                "Nur der aktuelle Fahrer kann die Lenkung zurückgeben.",
            );
            return;
        }
        let Some(mut v) = self.net.coop.drive.visitor.take() else {
            return;
        };
        if v.active && !v.returning {
            if let Some(p) = self.player.as_mut() {
                if p.vehicle.physics.velocity_kmh().abs() > 0.5 {
                    self.net.coop.drive.visitor = Some(v);
                    self.coop_notice(
                        "Pare o ônibus antes de devolver a direção.",
                        "Stop before returning the wheel.",
                        "Vor der Rückgabe anhalten.",
                    );
                    return;
                }
                p.release();
                p.authority_remote = true;
                let state = Snapshot::capture(v.uid, v.bus.clone(), &p.vehicle);
                v.returning = true;
                self.drive_send(
                    v.owner,
                    Message::Return {
                        token: v.token,
                        state,
                    },
                );
            }
        }
        self.net.coop.drive.visitor = Some(v);
    }
    pub(crate) fn drive_reset(&mut self, notify: bool) {
        if let Some((token, peer)) = self.net.coop.drive.owner.as_ref().map(|o|(o.token,o.peer)) {
            self.drive_announce_authority(token, peer, false);
        }
        if let Some(o) = self.net.coop.drive.owner.take() {
            if notify {
                self.drive_send(o.peer, Message::Abort { token: o.token });
            }
            if let Some(p) = self.player.as_mut().filter(|p| p.uid == o.uid) {
                p.authority_remote = false;
                p.release();
                p.axes.throttle = 0.;
                p.axes.brake = 1.;
                p.analog = Default::default();
            }
            if o.frozen {
                self.session.on_foot = None;
                if let Some(h) = self.session.humans.as_mut() {
                    h.avatar_remove(crate::on_foot::AVATAR_KEY);
                }
                self.view = "driver".into();
                self.cam.look = (0., 0.);
                self.net.inside_remote = None;
                self.menus.hover_key = None;
                if let (Some(p), Some(cam)) = (self.player.as_ref(), self.camera.as_ref()) {
                    self.camera = Some(p.camera(&self.view, cam));
                }
            }
        }
        if let Some(mut v) = self.net.coop.drive.visitor.take() {
            if notify {
                if v.active {
                    if let Some(p) = self.player.as_mut() {
                        p.authority_remote = true;
                        p.release();
                        let state = Snapshot::capture(v.uid, v.bus.clone(), &p.vehicle);
                        self.drive_send(
                            v.owner,
                            Message::Suspended {
                                token: v.token,
                                state,
                            },
                        );
                    }
                } else {
                    self.drive_send(v.owner, Message::Returned { token: v.token });
                }
            }
            if v.active {
                let restored = self.session.on_foot.is_none();
                let mut foot = self.session.on_foot.take().or_else(|| v.foot.take());
                if let (Some(f), Some(p)) = (foot.as_mut(), self.player.as_ref()) {
                    if restored && f.inside.is_none() {
                        let remote = BusId::Ai(crate::humans::remote_bus_id(v.owner));
                        let local = f.seat.and_then(|(_, k)| self.session.humans.as_ref()
                            .and_then(|h| h.seat_stand(remote, k))).unwrap_or(glam::Vec3::ZERO);
                        f.inside = Some((remote, local));
                    }
                    f.return_from_borrowed(v.owner, &p.vehicle);
                }
                let borrowed_uid = self.player.as_ref().map_or(0, |p| p.uid);
                if let Some(mut p) = self.player.take() {
                    if let (Some(a), Some(mut s)) = (self.sound.audio.as_ref(), p.sounds.take()) {
                        s.stop_all(a);
                    }
                    if let (Some(w), Some(r), Some(scene)) = (
                        self.world.as_ref(),
                        self.renderer.as_ref(),
                        self.scene.as_mut(),
                    ) {
                        if let Some(mut d) = p.driver.take() {
                            d.hide(r, scene);
                        }
                        w.release_vehicle(r, scene, p.render);
                        for t in p.trailer_renders {
                            w.release_vehicle(r, scene, t);
                        }
                    }
                }
                self.args.bus = v.original_bus.take();
                self.player = v.parked.take();
                if let (Some(p), Some(r), Some(scene)) = (self.player.as_mut(), self.renderer.as_ref(), self.scene.as_mut()) {
                    p.sync_transforms(r, scene, true, v.world.settings.rain_quality());
                }
                if let (Some(h), Some(p)) = (self.session.humans.as_mut(), self.player.as_mut()) {
                    h.player_bus_swapped(borrowed_uid, p.uid, &mut p.vehicle);
                }
                self.session.on_foot = foot;
                self.session.duty = v.original_duty.take();
                self.view = if self.session.on_foot.is_some() {
                    "foot"
                } else {
                    "driver"
                }
                .into();
            }
        }
        self.net.coop.camera = None;
        self.input.keys.clear();
        self.input.pad_repeat.clear();
        self.net.coop.drive.request = None;
    }
    /// Send the owner's exact timetable progress without transferring simulation authority.
    /// On-foot passengers use this for their navigator; the BCS duty remains with its owner.
    fn drive_route_broadcast(&mut self) {
        if self.net.coop.drive.route_sent.is_some_and(|t| t.elapsed() < Duration::from_secs(1)) { return; }
        if self.player.as_ref().is_none_or(|p| p.borrowed_from.is_some()) { return; }
        let Some(duty) = self.session.duty.as_ref() else { return; };
        let Ok(json) = serde_json::to_string(&(duty.line.as_str(), duty.tour.as_str(), duty.trip().name.as_str(), duty.next_stop)) else { return };
        if json.len() > 768 { return; }
        let Some(lan) = self.net.lan.as_mut() else { return; };
        let peers: Vec<u32> = lan.peers().map(|p| p.pose.id).collect();
        let msg = format!("COOPROUTE1 {json}");
        for peer in peers { lan.command(peer, &msg); }
        self.net.coop.drive.route_sent = Some(Instant::now());
    }
    fn drive_route_duty(&mut self, route: &RemoteRoute) -> Option<crate::schedule::PlayerDuty> {
        let world = self.world.as_ref()?;
        let sch = self.session.schedule.as_mut()?;
        let mut duty = sch.player_duty(world, &route.line, &route.tour, self.clock.time, None, false).ok()?;
        let trip = duty.trips.iter().position(|t| t.name.eq_ignore_ascii_case(&route.trip))?;
        duty.start_at_here(trip, route.stop);
        Some(duty)
    }
    /// A navigator-only timetable for the actual bus being ridden, not the passenger's
    /// parked bus or their independently selected tour. Never writes career/BCS progress.
    pub(crate) fn drive_route_preview(&mut self) {
        let owner = match self.foot_bus() {
            Some(BusId::Ai(id)) => crate::humans::remote_bus_player(id),
            _ => None,
        };
        let Some(owner) = owner else {
            self.net.coop.drive.passenger_duty = None;
            return;
        };
        let Some(route) = self.net.coop.drive.routes.get(&owner)
            .filter(|r| r.seen.elapsed() < Duration::from_secs(30)).cloned() else { return };
        let matches = self.net.coop.drive.passenger_duty.as_ref().is_some_and(|(id, d)|
            *id == owner && d.line.eq_ignore_ascii_case(&route.line)
                && d.tour.eq_ignore_ascii_case(&route.tour));
        if !matches {
            self.net.coop.drive.passenger_duty = self.drive_route_duty(&route).map(|d| (owner, d));
        }
        if let Some((id, duty)) = self.net.coop.drive.passenger_duty.as_mut() {
            if *id != owner { return; }
            if let Some(trip) = duty.trips.iter().position(|t| t.name.eq_ignore_ascii_case(&route.trip)) {
                if duty.trip_index != trip || duty.next_stop != route.stop {
                    duty.start_at_here(trip, route.stop);
                }
            }
        }
    }
    // Legacy input replay is intentionally never consumed as vehicle authority.
    pub(crate) fn drive_receive(&mut self, from: u32, text: &str) -> bool {
        if let Some(json) = text.strip_prefix("COOPROUTE1 ") {
            if json.len() <= 768 && self.net.lan.as_ref().is_some_and(|l| l.peers().any(|p| p.pose.id == from)) {
                if let Ok((line, tour, trip, stop)) = serde_json::from_str::<(String, String, String, usize)>(json) {
                    if line.len() <= 128 && tour.len() <= 128 && trip.len() <= 256 && stop < 1024
                        && !line.is_empty() && !trip.is_empty() {
                        self.net.coop.drive.routes.insert(from, RemoteRoute { line, tour, trip, stop, seen: Instant::now() });
                    }
                }
            }
            return true;
        }
        if let Some((token, driver, active)) = omsi_lan::vehicle_transfer::parse_authority_notice(text) {
            let valid = from != driver && self.net.lan.as_ref().is_some_and(|l|
                l.peers().any(|p|p.pose.id == from) && (driver == l.my_id || l.peers().any(|p|p.pose.id == driver)));
            if valid {
                if active { self.net.coop.drive.controllers.insert(from, (driver, token, Instant::now())); }
                else if self.net.coop.drive.controllers.get(&from).is_some_and(|(_, t, _)| *t == token) {
                    self.net.coop.drive.controllers.remove(&from);
                }
            }
        }
        false
    }
    fn drive_message(&mut self, from: u32, message: Message) {
        let now = Instant::now();
        match message {
            Message::Authority { token, driver, active } => {
                let valid = token != 0 && from != driver && self.net.lan.as_ref().is_some_and(|l|
                    l.peers().any(|p|p.pose.id == from) && (driver == l.my_id || l.peers().any(|p|p.pose.id == driver)));
                if valid {
                    if active { self.net.coop.drive.controllers.insert(from, (driver,token,now)); }
                    else if self.net.coop.drive.controllers.get(&from).is_some_and(|(_,t,_)|*t==token) {
                        self.net.coop.drive.controllers.remove(&from);
                    }
                }
            }
            Message::CashView { token, cash } => {
                if !cash.valid() { return; }
                if let Some(v) = self.net.coop.drive.visitor.as_mut()
                    .filter(|v| v.active && v.owner == from && v.token == token) {
                    v.cash_view = cash;
                } else if self.net.coop.remote.is_some_and(|(p, e, _)| p == from && e == token && e != 0) {
                    self.net.coop.cash_view = Some((from, token, cash));
                }
            }
            Message::DeskContext { token, desk } => {
                if desk.name.len()>4096 || desk.stage>4 || ![desk.price,desk.paid,desk.due,desk.given,desk.uncollected].iter().all(|x|x.is_finite()) { return; }
                if let Some(v)=self.net.coop.drive.visitor.as_mut().filter(|v|v.owner==from&&v.token==token&&v.active) { v.cash=desk; }
            }
            Message::PassengerWork { token, actor, wire } => {
                if !self.net.coop.drive.owner.as_ref().is_some_and(|o|o.peer==from&&o.token==token&&o.active&&o.frozen) || wire.len()>1024 { return; }
                if let Some(omsi_lan::coop::Message::Action(a))=omsi_lan::coop::Message::parse(&wire) {
                    if a.target != omsi_lan::coop::Target::Player || !matches!(a.kind,2|3|5) { return; }
                    let ok = a.desk == self.coop_desk(a.epoch).id && self.coop_apply_delegated(actor,&a);
                    self.drive_send(from,Message::PassengerResult { token,actor,epoch:a.epoch,seq:a.seq,ok });
                }
            }
            Message::PassengerResult { token, actor, epoch, seq, ok } => {
                if self.net.coop.drive.visitor.as_ref().is_some_and(|v|v.owner==from&&v.token==token&&v.active) {
                    self.coop_delegated_result(actor,epoch,seq,ok);
                }
            }
            Message::Work { token, actor, wire } => {
                let valid = self.net.coop.drive.visitor.as_ref().is_some_and(|v| {
                    v.owner == from && v.token == token && v.active && !v.returning
                });
                if !valid || wire.len() > 1024 {
                    return;
                }
                if let Some(v) = self.net.coop.drive.visitor.as_mut() {
                    v.actors.insert(actor);
                }
                match omsi_lan::coop::Message::parse(&wire) {
                    Some(omsi_lan::coop::Message::Action(a))
                        if a.target == omsi_lan::coop::Target::Player && a.kind <= 1 =>
                    {
                        let ok = self.coop_apply_delegated(actor, &a);
                        self.drive_send(from, Message::WorkResult {
                            token, actor, epoch: a.epoch, seq: a.seq, ok,
                        });
                    }
                    Some(omsi_lan::coop::Message::Keep(epoch)) => {
                        self.coop_keep_delegated(actor, epoch);
                    }
                    _ => {}
                }
            }
            Message::WorkResult { token, actor, epoch, seq, ok } => {
                if self.net.coop.drive.owner.as_ref().is_some_and(|o|
                    o.peer == from && o.token == token && o.active && o.frozen) {
                    self.coop_delegated_result(actor, epoch, seq, ok);
                }
            }
            Message::Request { nonce } => {
                if !self.net.coop.drive.busy()
                    && self.coop_peer_inside(from, omsi_lan::coop::Target::Player)
                {
                    self.net.coop.drive.requests.insert(from, (nonce, now));
                    self.coop_notice("Um passageiro pediu a direção. Autorize no Cooperativo com o ônibus parado.","A passenger requested the wheel. Authorize in Cooperative while stopped.","Ein Fahrgast möchte fahren. Im Stand im Koop-Menü freigeben.");
                }
            }
            Message::Offer {
                nonce,
                token,
                uid,
                bus,
                hof,
                paint,
            } => {
                let expected = self.net.coop.drive.request.is_some_and(|(p, n, t)| {
                    p == from && n == nonce && t.elapsed() < Duration::from_secs(60)
                });
                if !expected || self.net.coop.drive.visitor.is_some() || uid == 0 || token == 0 {
                    return;
                }
                let Some(BusId::Ai(id)) = self.foot_bus() else {
                    return;
                };
                if crate::humans::remote_bus_player(id) != Some(from) {
                    return;
                }
                // Resolve the actual locally mounted copy, including buses downloaded through mods.
                let Some(remote) = self
                    .net
                    .remotes
                    .remotes
                    .get(&from)
                    .filter(|r| !r.stand_in && !r.fallback)
                else {
                    return;
                };
                // Already mounted and validated for this peer; keep its
                // absolute path instead of running the menu resolver again.
                let local = remote.vehicle().ty.def.path.clone();
                let (Some(world), Some(renderer)) = (self.world.clone(), self.renderer.as_ref())
                else {
                    return;
                };
                let args = Args {
                    bus: Some(local.to_string_lossy().into_owned()),
                    paint,
                    hof,
                    autostart: false,
                    keep_fuel: true,
                    situation: None,
                    situation_vars: Vec::new(),
                    situation_strvars: Vec::new(),
                    situation_odometer_km: Some(0.),
                    situation_others: Vec::new(),
                    line: None,
                    tour: None,
                    trip: None,
                    triggers: None,
                    setvar: None,
                    setstr: None,
                    ..self.args.clone()
                };
                let worker_args = args.clone();
                let prefetch = world.placement_prefetch(renderer);
                let (tx, rx) = mpsc::channel();
                if std::thread::Builder::new()
                    .name("full-driver-vehicle".into())
                    .spawn(move || {
                        let result =
                            crate::spawn::PreparedPlayer::load_existing_path(&worker_args, &local)
                                .map(|mut p| {
                                    p.prefetch(prefetch, worker_args.paint.as_deref());
                                    p
                                });
                        let _ = tx.send(result);
                    })
                    .is_err()
                {
                    return;
                }
                self.net.coop.drive.visitor = Some(Visitor {
                    owner: from,
                    token,
                    uid,
                    bus,
                    args,
                    world,
                    loading: Some(rx),
                    prepared: None,
                    parked: None,
                    original_bus: self.args.bus.clone(),
                    foot: None,
                    original_duty: None,
                    active: false,
                    returning: false,
                    actors: HashSet::new(),
                    desk: 0,
                    cash: Default::default(),
                    cash_view: Default::default(),
                    images: Vec::new(),
                    seen: now,
                    sent: now,
                    started: now,
                });
            }
            Message::Prepared { token } => {
                let Some(mut o) = self.net.coop.drive.owner.take() else {
                    return;
                };
                if o.peer == from && o.token == token && !o.frozen {
                    let valid = self.coop_peer_inside(from, omsi_lan::coop::Target::Player)
                        && self.player.as_ref().is_some_and(|p| {
                            p.uid == o.uid && p.vehicle.physics.velocity_kmh().abs() <= 0.5
                        });
                    if !valid {
                        self.net.coop.drive.owner = Some(o);
                        self.drive_reset(true);
                        return;
                    }
                    self.release_vehicle_keys();
                    self.input.cruise = None;
                    if self.session.on_foot.is_none() {
                        let allowed = self.settings.get_up;
                        self.settings.get_up = true;
                        self.get_up();
                        self.settings.get_up = allowed;
                    }
                    let bus = self
                        .player
                        .as_ref()
                        .map(|p| bus_name(self, p))
                        .unwrap_or_default();
                    if let Some(p) = self.player.as_mut() {
                        p.release();
                        p.authority_remote = true;
                        p.startup = None;
                        let state = Snapshot::capture(o.uid, bus, &p.vehicle);
                        o.frozen = true;
                        o.seen = now;
                        log::info!("COOP13 handover frozen owner_uid={} driver_peer={} token={:x}", o.uid, o.peer, o.token);
                        let commit = Message::Commit { token, state };
                        if omsi_lan::vehicle_transfer::encode(&commit).is_none() {
                            p.authority_remote = false;
                            o.frozen = false;
                            self.net.coop.drive.owner = Some(o);
                            self.drive_reset(true);
                            self.coop_notice("O estado deste modelo excede o limite de transferência. O ônibus continua com você.","This model state exceeds the transfer limit. Your bus remains here.","Fahrzeugzustand überschreitet das Übertragungslimit. Dein Bus bleibt hier.");
                            return;
                        }
                        self.drive_send(from, commit);
                    }
                    self.view = if self.session.on_foot.is_some() {
                        "foot"
                    } else {
                        "pax"
                    }
                    .into();
                }
                self.net.coop.drive.owner = Some(o);
            }
            Message::Commit { token, state } => {
                let Some(mut v) = self.net.coop.drive.visitor.take() else {
                    return;
                };
                if v.owner != from
                    || v.token != token
                    || state.uid != v.uid
                    || state.bus != v.bus
                    || !state.valid()
                {
                    self.net.coop.drive.visitor = Some(v);
                    return;
                }
                if !v.active {
                    let route = self.net.coop.drive.routes.get(&v.owner).cloned();
                    let prepared_route = route.as_ref().and_then(|r| self.drive_route_duty(r));
                    let same_world = self
                        .world
                        .as_ref()
                        .is_some_and(|w| Arc::ptr_eq(w, &v.world));
                    let (Some(prepared), Some(r), Some(scene), true) = (
                        v.prepared.take(),
                        self.renderer.as_ref(),
                        self.scene.as_mut(),
                        same_world,
                    ) else {
                        self.net.coop.drive.visitor = Some(v);
                        return;
                    };
                    let result =
                        crate::spawn::spawn_player_prepared(&v.args, &v.world, r, scene, prepared);
                    let Ok(Some(mut p)) = result else {
                        self.net.coop.drive.visitor = Some(v);
                        self.drive_reset(true);
                        return;
                    };
                    if !state.apply(&mut p.vehicle) {
                        v.world.release_vehicle(r, scene, p.render);
                        for t in p.trailer_renders {
                            v.world.release_vehicle(r, scene, t);
                        }
                        self.net.coop.drive.visitor = Some(v);
                        self.drive_reset(true);
                        return;
                    }
                    // Preserve the visitor's own duty/BCS journey. The borrowed navigator
                    // shows the owner's tour, even when the visitor picked another tour.
                    v.original_duty = self.session.duty.take();
                    let mut followed = self.net.coop.drive.passenger_duty.take()
                        .filter(|(id, _)| *id == v.owner).map(|(_, duty)| duty)
                        .or(prepared_route);
                    if let Some(duty) = followed.as_mut() {
                        let ids = &state.timetable.3;
                        let stop = state.timetable.5;
                        if stop >= 0 {
                            if let Some(trip) = duty.trips.iter().position(|t|
                                t.stops.len() == ids.len() &&
                                t.stops.iter().map(|s| s.object_id).eq(ids.iter().copied())) {
                                duty.start_at_here(trip, stop as usize);
                            }
                        }
                    }
                    self.session.duty = followed;
                    p.borrowed_from = Some(v.owner);
                    p.authority_remote = false;
                    p.vehicle.host.auto_clutch = if self.settings.auto_clutch { 1.0 } else { 0.0 };
                    p.vehicle.host.capture_sound_triggers = true;
                    p.vehicle.host.fired_triggers.clear();
                    p.vehicle.host.fired_trigger_vars.clear();
                    p.vehicle.host.fired_file_triggers.clear();
                    p.vehicle.host.network_sound_triggers.clear();
                    p.sync_transforms(r, scene, true, v.world.settings.rain_quality());
                    if let Some(a) = self.sound.audio.as_ref() {
                        p.load_sounds(a);
                    }
                    if let Some(mut old) = self.player.take() {
                        old.release();
                        old.vehicle.host.capture_sound_triggers = false;
                        old.vehicle.host.network_sound_triggers.clear();
                        if let (Some(a), Some(mut sounds)) =
                            (self.sound.audio.as_ref(), old.sounds.take())
                        {
                            sounds.stop_all(a);
                        }
                        if let Some(h) = self.session.humans.as_mut() {
                            h.player_bus_swapped(old.uid, p.uid, &mut p.vehicle);
                        }
                        // Retain the previous bus for return, outside the active/parked
                        // simulation and render lists. It must disappear here too.
                        for render in std::iter::once(&old.render).chain(old.trailer_renders.iter()) {
                            for inst in render.instances.iter().chain(&render.far) {
                                r.set_params(scene, *inst, &[], false, &[]);
                            }
                        }
                        if let Some(driver) = old.driver.as_mut() { driver.hide(r, scene); }
                        v.parked = Some(old);
                    }
                    v.foot = self.session.on_foot.take();
                    if let Some(h) = self.session.humans.as_mut() {
                        h.avatar_remove(crate::on_foot::AVATAR_KEY);
                    }
                    self.args.bus = Some(v.bus.clone());
                    self.player = Some(p);
                    v.active = true;
                    // GPU instantiation can block for seconds on a large vehicle.
                    // Start the running lease after it completes, not before loading.
                    v.seen = Instant::now();
                    log::info!("COOP13 handover active logical_owner={} logical_uid={} local_uid={} token={:x}", v.owner, v.uid, self.player.as_ref().map_or(0, |p|p.uid), v.token);
                    self.net.coop.camera = None;
                    self.net.inside_remote = None;
                    self.view = "driver".into();
                    self.cam.look = (0., 0.);
                    self.menus.hover_key = None;
                    self.input.keys.clear();
                    self.input.pad_repeat.clear();
                    self.net.coop.panel = false;
                    self.net.coop.drive.request = None;
                    if let (Some(p), Some(cam)) = (self.player.as_ref(), self.camera.as_ref()) {
                        self.camera = Some(p.camera(&self.view, cam));
                    }
                    self.coop_notice("Você assumiu o ônibus. Os controles, painel e câmeras usam agora o veículo completo.","You now control the full vehicle, its cockpit and cameras.","Du steuerst jetzt das vollständige Fahrzeug, Cockpit und Kameras.");
                }
                self.drive_send(from, Message::Active { token });
                self.net.coop.drive.visitor = Some(v);
            }
            Message::Active { token } => {
                let confirmed = if let Some(o) = self.net.coop.drive.owner.as_mut()
                    .filter(|o| o.peer == from && o.token == token && o.frozen) {
                    let first = !o.active;
                    o.active = true;
                    o.seen = now;
                    first
                } else { false };
                if confirmed { self.coop_grant(None); self.drive_announce_authority(token, from, true); }
            }
            Message::State {
                token,
                desk,
                state,
                sounds,
                coins,
                ticket,
                change,
                take_change,
            } => {
                let valid = self.net.coop.drive.owner.as_ref().is_some_and(|o| {
                    o.peer == from
                        && o.token == token
                        && o.frozen
                        && o.uid == state.uid
                        && o.bus == state.bus
                });
                if valid {
                    let old_paint = self.player.as_ref().and_then(|p| p.vehicle.host.paint_scheme);
                    let same_desk = desk != 0 && desk == self.coop_desk(0).id;
                    if let Some(p) = self.player.as_mut() {
                        let preserve: Vec<_> = p
                            .vehicle
                            .var_index()
                            .iter()
                            .filter(|(n, _)| {
                                passenger_input(n)
                                    && (!same_desk || !n.eq_ignore_ascii_case("giventicket"))
                            })
                            .map(|(n, i)| (n.clone(), p.vehicle.state.vars[*i as usize]))
                            .collect();
                        let tt = (
                            p.vehicle.host.tt_line.clone(),
                            p.vehicle.host.tt_delay,
                            p.vehicle.host.tt_stops.clone(),
                            p.vehicle.host.tt_stop_ids.clone(),
                            p.vehicle.host.tt_terminus_index,
                            p.vehicle.host.tt_busstop_index,
                        );
                        let old_pose = (p.vehicle.position, p.vehicle.heading, p.vehicle.pitch, p.vehicle.bank);
                        if state.apply_stream(&mut p.vehicle) {
                            let motion = Motion {
                                from: old_pose,
                                to: (p.vehicle.position, p.vehicle.heading, p.vehicle.pitch, p.vehicle.bank),
                                started: now,
                                duration: self.net.coop.drive.owner.as_ref()
                                    .map_or(0.1, |o| o.motion.as_ref().map_or(0.1, |m| now.saturating_duration_since(m.started).as_secs_f64().clamp(0.05, 0.25))),
                            };
                            motion.apply(&mut p.vehicle);
                            if let Some(o) = self.net.coop.drive.owner.as_mut() { o.motion = Some(motion); }
                            let h = &mut p.vehicle.host;
                            h.tt_line = tt.0;
                            h.tt_delay = tt.1;
                            h.tt_stops = tt.2;
                            h.tt_stop_ids = tt.3;
                            h.tt_terminus_index = tt.4;
                            h.tt_busstop_index = tt.5;
                            for (n, value) in preserve {
                                p.vehicle.set_engine_var(&n, value);
                            }
                            for (name, file, values) in &sounds {
                                if let Some(file) = file {
                                    if omsi_net::remote_text::freetex_ok(file) {
                                        p.vehicle
                                            .host
                                            .fired_file_triggers
                                            .push((name.clone(), file.clone()));
                                    }
                                } else {
                                    p.vehicle.host.fired_triggers.push(name.clone());
                                    if let Some(values) = values {
                                        p.vehicle
                                            .host
                                            .fired_trigger_vars
                                            .push((name.clone(), values.clone()));
                                    }
                                }
                            }
                            p.vehicle.host.network_sound_triggers.extend(sounds);
                            if same_desk {
                                p.vehicle.host.change_coins.extend(coins);
                                p.give_ticket |= ticket;
                                p.give_change |= change;
                                p.take_change |= take_change;
                            }
                            if let Some(o) = self.net.coop.drive.owner.as_mut() {
                                o.seen = now;
                                o.active = true;
                            }
                        }
                    }
                    let new_paint = self.player.as_ref().and_then(|p| p.vehicle.host.paint_scheme);
                    if new_paint != old_paint {
                        if let Some(scheme) = new_paint { self.repaint_current_vehicle(scheme); }
                    }
                }
            }
            Message::Context {
                token,
                desk,
                tt,
                schedule,
                vars,
                humans,
                paths,
                seats,
                triggers,
            } => {
                let valid = self.net.coop.drive.visitor.as_ref().is_some_and(|v| {
                    v.owner == from && v.token == token && v.active && !v.returning
                });
                if valid {
                    if let Some(p) = self.player.as_mut() {
                        for (n, value) in vars {
                            if value.is_finite() && passenger_input(&n) {
                                p.vehicle.set_engine_var(&n, value);
                            }
                        }
                        p.vehicle.host.humans_count = humans;
                        p.vehicle.host.humans_on_path_link = paths;
                        p.vehicle.host.humans_on_seat = seats;
                        let h = &mut p.vehicle.host;
                        h.tt_line = tt.0;
                        h.tt_delay = tt.1;
                        h.tt_stops = tt.2;
                        h.tt_stop_ids = tt.3;
                        h.tt_terminus_index = tt.4;
                        h.tt_busstop_index = tt.5;
                        h.schedule_active = schedule;
                        for t in triggers.into_iter().take(128) {
                            p.vehicle.trigger(&t);
                        }
                    }
                    if let Some(v) = self.net.coop.drive.visitor.as_mut() {
                        v.seen = now;
                        v.desk = desk;
                    }
                }
            }
            Message::Return { token, state } => {
                let valid = self.net.coop.drive.owner.as_ref().is_some_and(|o| {
                    o.peer == from
                        && o.token == token
                        && o.frozen
                        && o.uid == state.uid
                        && o.bus == state.bus
                });
                if valid
                    && state.speed.abs() <= 0.5 / 3.6
                    && self
                        .player
                        .as_mut()
                        .is_some_and(|p| state.apply(&mut p.vehicle))
                {
                    self.drive_send(from, Message::Returned { token });
                    self.drive_reset(false);
                }
            }
            Message::Suspended { token, state } => {
                let valid = self.net.coop.drive.owner.as_ref().is_some_and(|o| {
                    o.peer == from
                        && o.token == token
                        && o.frozen
                        && o.uid == state.uid
                        && o.bus == state.bus
                });
                if valid
                    && self
                        .player
                        .as_mut()
                        .is_some_and(|p| state.apply(&mut p.vehicle))
                {
                    self.drive_send(from, Message::Returned { token });
                    self.drive_reset(false);
                }
            }
            Message::Returned { token } => {
                if self
                    .net
                    .coop
                    .drive
                    .owner
                    .as_ref()
                    .is_some_and(|o| o.peer == from && o.token == token && !o.active)
                {
                    self.drive_reset(false);
                }
                if self
                    .net
                    .coop
                    .drive
                    .visitor
                    .as_ref()
                    .is_some_and(|v| v.owner == from && v.token == token && v.returning)
                {
                    self.drive_reset(false);
                }
            }
            Message::Heartbeat { token } => {
                if let Some(o) = self
                    .net
                    .coop
                    .drive
                    .owner
                    .as_mut()
                    .filter(|o| o.peer == from && o.token == token)
                {
                    o.seen = now;
                }
                if let Some(v) = self
                    .net
                    .coop
                    .drive
                    .visitor
                    .as_mut()
                    .filter(|v| v.owner == from && v.token == token)
                {
                    v.seen = now;
                }
            }
            Message::Abort { token } => {
                if self
                    .net
                    .coop
                    .drive
                    .visitor
                    .as_ref()
                    .is_some_and(|v| v.owner == from && v.token == token)
                {
                    if let Some(v) = self.net.coop.drive.visitor.as_ref().filter(|v| v.active) {
                        let (uid, bus, owner) = (v.uid, v.bus.clone(), v.owner);
                        if let Some(p) = self.player.as_mut() {
                            p.authority_remote = true;
                            p.release();
                            let state = Snapshot::capture(uid, bus, &p.vehicle);
                            self.drive_send(owner, Message::Suspended { token, state });
                        }
                    } else {
                        self.drive_send(from, Message::Returned { token });
                    }
                    self.drive_reset(false);
                }
                if self
                    .net
                    .coop
                    .drive
                    .owner
                    .as_ref()
                    .is_some_and(|o| o.peer == from && o.token == token && !o.frozen)
                {
                    self.drive_reset(false);
                }
            }
        }
    }
    pub(crate) fn drive_tick(&mut self) {
        self.drive_route_broadcast();
        if self.net.coop.drive.authority_sent.is_none_or(|t| t.elapsed() >= Duration::from_millis(800)) {
            if let Some((token,peer)) = self.net.coop.drive.owner.as_ref().filter(|o|o.active).map(|o|(o.token,o.peer)) {
                self.drive_announce_authority(token,peer,true);
            }
        }

        let incoming = self
            .net
            .lan
            .as_mut()
            .map(|l| l.take_handovers())
            .unwrap_or_default();
        for m in incoming {
            if let Some(message) = omsi_lan::vehicle_transfer::decode(&m.event.data) {
                self.drive_message(m.owner, message);
            }
        }
        if let (Some(o), Some(p)) = (self.net.coop.drive.owner.as_ref(), self.player.as_mut()) {
            if o.frozen && p.uid == o.uid {
                p.authority_remote = true;
                if let Some(motion) = &o.motion { motion.apply(&mut p.vehicle); }
            }
        }
        if self.session.humans.as_ref().is_some_and(|h| h.stop_request) {
            self.drive_queue_trigger("int_haltewunsch");
        }
        let now = Instant::now();
        self.net
            .coop
            .drive
            .requests
            .retain(|_, (_, t)| t.elapsed() < Duration::from_secs(30));
        if self
            .net
            .coop
            .drive
            .request
            .is_some_and(|(_, _, t)| t.elapsed() > Duration::from_secs(60))
        {
            self.net.coop.drive.request = None;
            self.coop_notice(
                "A troca expirou. Os dois jogadores e o servidor precisam da versão nova.",
                "Driver change expired. Both players and the server need the new version.",
                "Fahrerwechsel abgelaufen. Beide Spieler und Server benötigen die neue Version.",
            );
        }
        if let Some(mut o) = self.net.coop.drive.owner.take() {
            let invalid = self.player.as_ref().is_none_or(|p| p.uid != o.uid)
                || self
                    .net
                    .lan
                    .as_ref()
                    .is_none_or(|l| !l.peers().any(|p| p.pose.id == o.peer));
            // The visitor stops after 2 seconds of silence; the owner requests suspension after 3.
            if !o.frozen && (invalid || o.seen.elapsed() > Duration::from_secs(60)) {
                self.net.coop.drive.owner = Some(o);
                self.drive_reset(true);
                return;
            }
            // A frozen owner never resumes on silence alone: a late Commit must not
            // create a second simulator. Only Return/Suspended confirms the peer stopped.
            if o.frozen && (invalid || o.seen.elapsed() > Duration::from_secs(if o.active { 3 } else { 60 })) {
                if o.sent.elapsed() > Duration::from_millis(250) && self.drive_stream_ready(o.peer) {
                    self.drive_send(o.peer, Message::Abort { token: o.token });
                    o.sent = now;
                }
                self.net.coop.drive.owner = Some(o);
                return;
            }
            if o.sent.elapsed() > Duration::from_millis(150) && self.drive_stream_ready(o.peer) {
                self.drive_send(o.peer, Message::Heartbeat { token: o.token });
                if o.frozen && o.active {
                    if let Some(p) = self.player.as_ref() {
                        let h = &p.vehicle.host;
                        let vars = p
                            .vehicle
                            .var_index()
                            .iter()
                            .filter(|(n, _)| passenger_input(n))
                            .map(|(n, i)| (n.clone(), p.vehicle.state.vars[*i as usize]))
                            .collect();
                        let message = Message::Context {
                            token: o.token,
                            desk: self.coop_desk(0).id,
                            tt: (
                                h.tt_line.clone(),
                                h.tt_delay,
                                h.tt_stops.clone(),
                                h.tt_stop_ids.clone(),
                                h.tt_terminus_index,
                                h.tt_busstop_index,
                            ),
                            schedule: h.schedule_active,
                            vars,
                            humans: h.humans_count,
                            paths: h.humans_on_path_link.clone(),
                            seats: h.humans_on_seat.clone(),
                            triggers: std::mem::take(&mut o.triggers),
                        };
                        self.drive_send(o.peer, message);
                        self.drive_send(o.peer,Message::DeskContext { token:o.token,desk:self.coop_desk(0) });
                    }
                }
                o.sent = now;
            }
            self.net.coop.drive.owner = Some(o);
        }
        if let Some(mut v) = self.net.coop.drive.visitor.take() {
            if !v.active {
                if let Some(rx) = v.loading.as_ref() {
                    match rx.try_recv() {
                        Ok(Ok(p)) => {
                            v.prepared = Some(p);
                            v.loading = None;
                            self.drive_send(v.owner, Message::Prepared { token: v.token });
                        }
                        Ok(Err(e)) => {
                            log::warn!("full vehicle preparation failed: {e:#}");
                            self.net.coop.drive.visitor = Some(v);
                            self.drive_reset(true);
                            return;
                        }
                        Err(mpsc::TryRecvError::Disconnected) => {
                            self.net.coop.drive.visitor = Some(v);
                            self.drive_reset(true);
                            return;
                        }
                        Err(mpsc::TryRecvError::Empty) => {}
                    }
                }
                if v.started.elapsed() > Duration::from_secs(60) {
                    self.net.coop.drive.visitor = Some(v);
                    self.drive_reset(true);
                    return;
                }
            } else if v.seen.elapsed() > Duration::from_secs(2) {
                // No self-reactivation after a network stall: the owner's lease may resume.
                if let Some(p) = self.player.as_mut() {
                    p.authority_remote = true;
                    p.release();
                    let state = Snapshot::capture(v.uid, v.bus.clone(), &p.vehicle);
                    self.drive_send(
                        v.owner,
                        Message::Suspended {
                            token: v.token,
                            state,
                        },
                    );
                }
                self.net.coop.drive.visitor = Some(v);
                self.drive_reset(false);
                return;
            }
            if v.sent.elapsed() > Duration::from_millis(100) && self.drive_stream_ready(v.owner) {
                if v.active && !v.returning {
                    if let Some(p) = self.player.as_mut() {
                        let images: Vec<_> = p.vehicle.host.script_textures.iter_mut()
                            .map(|t| t.released_image()).collect();
                        let changed = images.len() != v.images.len() || images.iter().zip(&v.images).any(|(a, b)| match (a, b) {
                            (Some(a), Some(b)) => (a.0, a.1, a.2) != (b.0, b.1, b.2) || !Arc::ptr_eq(&a.3, &b.3),
                            (None, None) => false,
                            _ => true,
                        });
                        let mut state = Snapshot::capture_stream(v.uid, v.bus.clone(), &p.vehicle, changed);
                        // Prefer released images to a script's unfinished locked buffer.
                        if changed {
                            for (dst, image) in state.images.iter_mut().zip(&images) {
                                if let Some((w, h, mip, rgba)) = image { *dst = (*w, *h, *mip, rgba.as_ref().clone()); }
                            }
                        }
                        v.images = images;
                        let message = Message::State {
                            token: v.token,
                            desk: v.desk,
                            state,
                            sounds: std::mem::take(&mut p.vehicle.host.network_sound_triggers),
                            coins: std::mem::take(&mut p.vehicle.host.change_coins),
                            ticket: std::mem::take(&mut p.give_ticket),
                            change: std::mem::take(&mut p.give_change),
                            take_change: std::mem::take(&mut p.take_change),
                        };
                        self.drive_send(v.owner, message);
                    }
                } else if v.active && v.returning {
                    // Return is a control transition, not a one-shot stream update.
                    // Retry until confirmation even if the initial queue was full.
                    if let Some(p) = self.player.as_ref() {
                        let state = Snapshot::capture(v.uid, v.bus.clone(), &p.vehicle);
                        self.drive_send(v.owner, Message::Return { token: v.token, state });
                    }
                } else {
                    self.drive_send(v.owner, Message::Heartbeat { token: v.token });
                }
                v.sent = now;
            }
            self.net.coop.drive.visitor = Some(v);
        }
    }
    pub(crate) fn coop_riders(&self) -> Option<usize> {
        if let Some(p) = self.player.as_ref().filter(|p| p.borrowed_from.is_some()) {
            return Some(p.vehicle.host.humans_count.max(0.0).round() as usize);
        }
        self.session.humans.as_ref().map(|h| h.riding())
    }
    pub(crate) fn drive_driver_epoch(&self) -> Option<u64> {
        self.net.coop.drive.visitor.as_ref().filter(|v|v.active).map(|v|v.token)
    }
    pub(crate) fn drive_cash_view(&self) -> Option<&omsi_lan::coop::CashView> {
        self.net.coop.drive.visitor.as_ref().filter(|v|v.active).map(|v| &v.cash_view)
    }
    pub(crate) fn drive_cash_recipient(&self) -> Option<(u32, u64)> {
        self.net.coop.drive.owner.as_ref().filter(|o|o.active).map(|o|(o.peer,o.token))
    }
    pub(crate) fn drive_cash_desk(&self) -> Option<&omsi_lan::coop::Desk> {
        self.net.coop.drive.visitor.as_ref().filter(|v|v.active).map(|v|&v.cash)
    }
    pub(crate) fn drive_forward_cash(&mut self, actor: u32, action: &omsi_lan::coop::Action) -> bool {
        let Some(v) = self.net.coop.drive.visitor.as_ref().filter(|v|v.active&&!v.returning) else { return false; };
        let (owner,token)=(v.owner,v.token);
        self.drive_send(owner,Message::PassengerWork { token,actor,wire:omsi_lan::coop::Message::Action(action.clone()).encode() });
        true
    }
    pub(crate) fn drive_delegated_actor(&self, actor: u32) -> bool {
        self.net
            .coop
            .drive
            .visitor
            .as_ref()
            .is_some_and(|v| v.active && v.actors.contains(&actor))
    }
    pub(crate) fn drive_forward_coop(
        &mut self,
        actor: u32,
        message: &omsi_lan::coop::Message,
    ) -> bool {
        let Some(o) = self.net.coop.drive.owner.as_ref().filter(|o| o.frozen) else {
            return false;
        };
        let (peer, token) = (o.peer, o.token);
        self.drive_send(
            peer,
            Message::Work {
                token,
                actor,
                wire: message.encode(),
            },
        );
        true
    }
    pub(crate) fn drive_queue_trigger(&mut self, name: &str) -> bool {
        if let Some(o) = self.net.coop.drive.owner.as_mut().filter(|o| o.frozen) {
            if o.triggers.len() < 128 {
                o.triggers.push(name.to_owned());
            }
            true
        } else {
            false
        }
    }
    pub(crate) fn shared_drive_key(
        &mut self,
        code: KeyCode,
        _pressed: bool,
        _repeat: bool,
    ) -> bool {
        self.net.coop.drive.owner_active()
            && self.session.on_foot.is_none()
            && !matches!(
                code,
                KeyCode::Escape
                    | KeyCode::F1
                    | KeyCode::F2
                    | KeyCode::F3
                    | KeyCode::F4
                    | KeyCode::F12
                    | KeyCode::Slash
                    | KeyCode::KeyV
            )
    }
    pub(crate) fn drive_pad(
        &mut self,
        _a: crate::controllers::Analog,
        _b: &[(String, bool)],
    ) -> bool {
        self.net.coop.drive.owner_active()
    }
    pub(crate) fn drive_lost_focus(&mut self) {}
    pub(crate) fn drive_sign(
        &mut self,
        _form: &crate::game_panels::DestForm,
        _clear: bool,
    ) -> bool {
        false
    }
}
fn passenger_input(name: &str) -> bool {
    let n = name.to_ascii_lowercase();
    (n.starts_with("pax_") && (n.ends_with("_req") || n.ends_with("_busy") || n.contains("place")))
        || matches!(
            n.as_str(),
            "giventicket"
                | "ticket_request"
                | "ticket_type"
                | "ticket_value"
                | "ticket_count"
                | "ticket_paid"
                | "humans_count"
                | "schedule_active"
        )
}

#[cfg(test)]
mod coop13_regressions {
    use super::*;
    use clap::Parser;

    fn bus() -> omsi_sim::VehicleInstance {
        let program = omsi_script::compile(&omsi_script::CompileInput {
            builtin_vars: vec!["counter".into()], ..Default::default()
        });
        let ty = omsi_sim::VehicleType {
            def: Default::default(), model: Default::default(), model_dir: Default::default(),
            program: Arc::new(program), meshes: vec![], lod_meshes: vec![], paint_schemes: vec![], texchanges: vec![],
            wheel_meshes: vec![], suspension_axles: vec![], missing_packs: vec![],
            mesh_bounds: vec![], mesh_boxes: vec![],
        };
        omsi_sim::VehicleInstance::new(Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
    }

    #[test]
    fn coop13_g_does_not_reclaim_a_vehicle_with_remote_authority() {
        let args = Args::parse_from(["openomsi", "--root", "."]);
        let mut app = crate::assemble_app(args, crate::settings::Settings::default());
        let mut player = crate::spawn::coop13_test_player(bus());
        player.authority_remote = true;
        let uid = player.uid;
        app.player = Some(player);
        app.start_on_foot(glam::DVec3::ZERO, 0.0);
        app.session.on_foot.as_mut().unwrap().inside = Some((BusId::Player, glam::Vec3::ZERO));
        let now = Instant::now();
        app.net.coop.drive.owner = Some(Owner {
            peer: 2, token: 11, uid, bus: "Vehicles/test.bus".into(), frozen: true,
            active: true, seen: now, sent: now, triggers: vec![], motion: None,
        });
        assert!(app.foot_key(KeyCode::KeyG, true, false, false, false));
        assert!(app.session.on_foot.is_some(), "G reclaimed the remote driver's wheel");
        assert!(app.player.as_ref().unwrap().authority_remote);
        assert_eq!(app.view, "foot");
    }

    #[test]
    fn coop13_former_driver_menus_cannot_reset_remove_or_reclaim_the_bus() {
        let args=Args::parse_from(["openomsi","--root","."]);
        let mut app=crate::assemble_app(args,Default::default());
        let player=crate::spawn::coop13_test_player(bus());let uid=player.uid;app.player=Some(player);
        let now=Instant::now();app.net.coop.drive.owner=Some(Owner{
            peer:2,token:11,uid,bus:"Vehicles/test.bus".into(),frozen:true,active:true,
            seen:now,sent:now,triggers:vec![],motion:None,
        });
        for id in ["remove","reset","teleport","refuel","swap","tobus"]{
            assert!(!app.drive_menu_allowed(id),"former driver retained {id}");
            assert!(app.page_action(id));assert!(app.player.is_some());
        }
        for id in ["camera","coop","options","getout","map"]{assert!(app.drive_menu_allowed(id));}
    }
    #[test]
    fn coop_borrowed_bus_hud_uses_original_owner_passenger_count() {
        let mut app = crate::assemble_app(Args::parse_from(["openomsi", "--root", "."]), Default::default());
        let mut p = crate::spawn::coop13_test_player(bus());
        p.borrowed_from = Some(2);
        p.vehicle.host.humans_count = 14.0;
        app.player = Some(p);
        assert_eq!(app.coop_riders(), Some(14));
    }
    #[test]
    fn coop_cash_collect_delegation_keeps_change_and_retries_cannot_take_next_fare() {
        use omsi_lan::coop::{Action, Phase, Target};
        let mut app = crate::assemble_app(Args::parse_from(["openomsi", "--root", "."]), Default::default());
        let mut humans = crate::humans::Humans::new(std::path::Path::new("."), &mut app.gfx.sim_view.people);
        let mut money = omsi_sim::people::Money::new(std::path::Path::new("."), "missing-currency.cfg");
        money.placed.push((11, glam::Mat4::IDENTITY, 0, false, None));
        money.placed.push((12, glam::Mat4::IDENTITY, 0, true, None));
        humans.sim.money = Some(money);
        app.session.humans = Some(humans);
        let action = Action { epoch: 77, seq: 1, target: Target::Player, desk: 0,
            kind: 5, part: 0, index: 0, signature: 0, phase: Phase::Tap, x: 0.0, y: 0.0 };
        assert!(app.coop_apply_delegated(3, &action));
        let m = app.session.humans.as_mut().unwrap().sim.money.as_mut().unwrap();
        assert_eq!(m.placed.len(), 1);
        assert!(m.placed[0].3);
        m.placed.push((13, glam::Mat4::IDENTITY, 0, false, None));
        assert!(app.coop_apply_delegated(3, &action));
        assert_eq!(app.session.humans.as_ref().unwrap().sim.money.as_ref().unwrap().placed.len(), 2,
            "a repeated receipt must not collect a later payment");
    }
    #[test]
    fn coop_returned_walker_uses_current_vehicle_pose_and_forgets_old_eye() {
        let mut app = crate::assemble_app(Args::parse_from(["openomsi", "--root", "."]), Default::default());
        app.start_on_foot(glam::DVec3::new(1.0, 2.0, 3.0), 0.0);
        let foot = app.session.on_foot.as_mut().unwrap();
        let remote = BusId::Ai(crate::humans::remote_bus_id(2));
        let local = glam::Vec3::new(0.5, 2.0, 1.0);
        foot.inside = Some((remote, local));
        foot.eye = Some(foot.pos);
        let mut vehicle = bus();
        vehicle.position = glam::DVec3::new(9000.0, -8000.0, 40.0);
        vehicle.heading = 90.0;
        foot.return_from_borrowed(2, &vehicle);
        let expected = vehicle.position + vehicle.body_rotation().transform_point3(local).as_dvec3();
        assert!((foot.pos - expected).length() < 0.001);
        assert_eq!(foot.inside, Some((remote, local)));
        assert!(foot.eye.is_none());
    }
    #[test]
    fn coop13_passive_motion_smooths_chassis_without_advancing_simulation() {
        let mut player = crate::spawn::coop13_test_player(bus());
        player.authority_remote = true;
        player.vehicle.host.clock.run_time = 123.0;
        let motion = Motion {
            from: (glam::DVec3::ZERO, 359.0, 0.0, 0.0),
            to: (glam::DVec3::new(10.0, 0.0, 0.0), 1.0, 0.1, 0.0),
            started: Instant::now() - Duration::from_millis(50),
            duration: 0.1,
        };
        motion.apply_elapsed(&mut player.vehicle, 0.05);
        assert!((player.vehicle.position.x-5.0).abs() < 1e-6);
        assert!((player.vehicle.heading-360.0).abs() < 0.2);
        let position = player.vehicle.position;
        player.tick(0.1, None, false, false);
        assert_eq!(player.vehicle.position, position);
        assert_eq!(player.vehicle.host.clock.run_time, 123.0);
    }
}
