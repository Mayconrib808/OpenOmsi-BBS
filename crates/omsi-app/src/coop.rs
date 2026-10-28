//! Opt-in cooperative interaction. The simulation owner alone works a real vehicle control.
//! Other players remain ordinary protocol-14 peers. Never simulate a second cash desk.
use crate::coop_view::{self, passenger_event, BodyPose, PresentedView};
use crate::humans::BusId;
use crate::App;
use glam::DVec3;
use omsi_lan::coop::{Action, Desk, Message, Outbox, Phase, Receipt, Target};
use omsi_sim::VehicleInstance;
use std::collections::HashMap;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

#[derive(Clone)]
pub(crate) struct CameraView {
    pub bus: BusId,
    pub local: omsi_vehicle::Camera,
    pub yaw0: f32,
    pub pitch0: f32,
    pub slot: usize,
    /// F1 bank (driver) when true; F2 bank (passenger) otherwise.
    pub driver: bool,
}
struct Grant {
    peer: u32,
    epoch: u64,
    model: std::path::PathBuf,
    uid: u64,
}
#[derive(Clone)]
struct Held {
    from: u32,
    action: Action,
    seen: Instant,
    control: Option<crate::mouse_control::Press>,
}

#[derive(Default)]
pub(crate) struct CoopState {
    pub character: crate::coop_character::CharacterState,
    pub drive: crate::coop_drive::DriveState,
    session: Option<(u64, u32)>,
    grant: Option<Grant>,
    pub remote: Option<(u32, u64, Instant)>,
    receipts: HashMap<(u32, u64), Receipt>,
    pending_delegated: HashMap<(u32, u64, u64), Instant>,
    outbox: Outbox,
    mouse: Option<(u32, Action)>,
    held: HashMap<u32, Held>,
    last_keep: Option<Instant>,
    last_status: Option<Instant>,
    last_grant: Option<Instant>,
    last_sent_desk: Option<(Instant, Desk)>,
    pub desk: Desk,
    bells: Vec<(u32, omsi_sim::people::DoorRequest)>,
    pub camera: Option<CameraView>,
    drawn: Option<(BusId, PresentedView)>,
    auto_drag: Option<(u32, crate::mouse_control::AutoDrag)>,
    last_tick: Option<Instant>,
    ticket_blocks: Option<(u32, crate::money::TicketBlocks)>,
    pub panel: bool,
    pub cash_page: usize,
    pub camera_edit: bool,
    last_close_signal: Option<Instant>,
}

pub(crate) fn text<'a>(pt: &'a str, en: &'a str, de: &'a str) -> &'a str {
    let lang = omsi_ui::i18n::language();
    if lang.starts_with("pt") {
        pt
    } else if lang.starts_with("de") {
        de
    } else {
        en
    }
}

fn hash(s: &str) -> u64 {
    s.bytes().fold(0xcbf29ce484222325, |h, b| {
        (h ^ b as u64).wrapping_mul(0x100000001b3)
    })
}
fn camera_key(v: &VehicleInstance) -> String {
    let path =
        v.ty.def
            .path
            .to_string_lossy()
            .replace('\\', "/")
            .to_lowercase();
    path.rfind("/vehicles/")
        .map(|i| path[i + 1..].to_string())
        .unwrap_or_else(|| {
            format!(
                "{}|{}|{}",
                v.ty.def.manufacturer,
                v.ty.def.type_name,
                v.ty.def
                    .path
                    .file_name()
                    .unwrap_or_default()
                    .to_string_lossy()
            )
        })
}
/// Suppress the added metallic cue only when the actual bus sound configuration
/// names an original closing/cash-register signal. Ordinary pneumatic door sounds
/// are not sufficient evidence: they are not the conductor's "fecho" cue.
pub(crate) fn native_close_cue(v: &VehicleInstance) -> bool {
    let Some(rel) = v.ty.def.sound.as_ref() else { return false };
    let path = omsi_cfg::resolve_path(v.ty.def.dir(), rel);
    let Ok(cfg) = omsi_vehicle::SoundCfg::load(&path) else { return false };
    cfg.sounds.iter().any(|s| {
        let label = std::iter::once(s.file.as_str())
            .chain(s.triggers.iter().map(String::as_str))
            .collect::<Vec<_>>().join(" ");
        native_close_label(&label)
    })
}

fn native_close_label(label: &str) -> bool {
    let label = label.to_ascii_lowercase();
    ["fecho_motor", "fecho-motor", "fechomotor", "fecho motor",
     "sinal_fecho", "sinalfecho", "fecho_porta", "fecha_porta",
     "door_close_signal", "door_closing_signal", "turnstile_coin",
     "moeda_roleta", "coin_turnstile"]
        .iter().any(|word| label.contains(word))
}

#[cfg(test)]
mod close_cue_tests {
    use super::native_close_label;
    #[test]
    fn recognizes_native_lyd_like_close_cue_without_suppressing_every_door_noise() {
        assert!(native_close_label("Sounds/FECHO_MOTOR.wav"));
        assert!(native_close_label("banco.wav sinal_fecho_motor"));
        assert!(!native_close_label("porta_pneumatica_fechando.wav"));
        assert!(!native_close_label("engine_start.wav"));
    }
}

fn mesh_info(v: &VehicleInstance, part: u8, index: u32) -> Option<(String, u64)> {
    let ty = if part == 0 {
        &v.ty
    } else {
        &v.trailers.get(part as usize - 1)?.ty
    };
    let def = ty
        .model
        .meshes
        .get(ty.meshes.get(index as usize)?.def_index)?;
    let event = def.mouse_event.clone()?;
    let signature = hash(&format!(
        "{}:{}",
        def.file.replace('\\', "/").to_lowercase(),
        event
    ));
    Some((event, signature))
}
fn bell(event: &str) -> bool {
    passenger_event(event) && !event.to_ascii_lowercase().contains("kinderwagen")
}

fn press(v: &mut VehicleInstance, event: &str) -> bool {
    crate::mouse_control::Press::begin(v, event).is_some()
}

impl App {
    pub(crate) fn coop_notice(&mut self, pt: &str, en: &str, de: &str) {
        self.service_msg = Some((text(pt, en, de).into(), 4.0));
    }

    fn coop_send(&mut self, to: u32, message: Message) {
        if let Some(lan) = self.net.lan.as_mut() {
            lan.command(to, &message.encode());
        }
    }

    pub(crate) fn coop_vehicle(&self, bus: BusId) -> Option<&VehicleInstance> {
        match bus {
            BusId::Player => self.player.as_ref().map(|p| &p.vehicle),
            BusId::Ai(id) => {
                if let Some(owner) = crate::humans::remote_bus_player(id) {
                    self.net
                        .remotes
                        .remotes
                        .get(&owner)
                        .filter(|r| !r.fallback && !r.stand_in)
                        .map(|r| r.vehicle())
                } else {
                    self.session
                        .traffic
                        .as_ref()?
                        .sim
                        .cars
                        .iter()
                        .find(|c| c.id == id)
                        .map(|c| &c.vehicle)
                }
            }
        }
    }

    fn coop_vehicle_mut(&mut self, target: Target) -> Option<&mut VehicleInstance> {
        match target {
            Target::Player => self.player.as_mut().map(|p| &mut p.vehicle),
            Target::Ai(id) => self
                .session
                .traffic
                .as_mut()?
                .sim
                .cars
                .iter_mut()
                .find(|c| c.id == id)
                .map(|c| &mut c.vehicle),
        }
    }

    fn coop_route(&self, bus: BusId) -> Option<(u32, Target)> {
        match bus {
            // Once delegated, the old owner's own bus is a remote control target.
            // Never execute its clicks against the passive local vehicle.
            BusId::Player => self.net.coop.drive.owner_peer_active()
                .map(|peer| (peer, Target::Player)),
            BusId::Ai(id) => {
                if let Some(owner) = crate::humans::remote_bus_player(id) {
                    Some((self.net.coop.drive.controller(owner), Target::Player))
                } else {
                    Some((
                        if self
                            .net
                            .lan
                            .as_ref()
                            .is_some_and(|l| l.role == omsi_net::Role::Client)
                        {
                            1
                        } else {
                            0
                        },
                        Target::Ai(id),
                    ))
                }
            }
        }
    }

    pub(crate) fn coop_passengers(&self) -> Vec<(u32, String)> {
        let Some(lan) = self.net.lan.as_ref() else {
            return Vec::new();
        };
        lan.peers()
            .filter(|p| {
                (self.net.coop.drive.visitor_owner() == Some(p.pose.id)
                    && self.coop_peer_inside(p.pose.id, Target::Player)) || p.pose.walker.and_then(|w| w.aboard).is_some_and(|a| {
                    // The bus keeps the original logical owner on the wire while
                    // another peer drives its real simulation. He is a passenger.
                    a.owner == lan.my_id
                        || self.net.coop.drive.visitor_owner()
                            .is_some_and(|owner| a.owner == owner)
                })
            })
            .map(|p| (p.pose.id, p.pose.name.clone()))
            .collect()
    }

    pub(crate) fn coop_conductor(&self) -> Option<u32> {
        self.net.coop.grant.as_ref().map(|g| g.peer)
    }

    pub(crate) fn coop_grant(&mut self, peer: Option<u32>) {
        // The former owner no longer authorizes any controls during handover.
        if peer.is_some() && self.net.coop.drive.owner_active() { return; }
        self.coop_release_held();
        self.coop_cancel_auto();
        if let Some(g) = self.net.coop.grant.take() {
            log::info!("COOP1 conductor revoked: peer={}", g.peer);
            self.coop_send(g.peer, Message::Revoke(g.epoch));
        }
        self.net.coop.receipts.clear();
        self.net.coop.pending_delegated.clear();
        if let Some(peer) = peer.filter(|p| self.coop_passengers().iter().any(|(id, _)| id == p)) {
            let Some(v) = self.player.as_ref() else {
                return;
            };
            let epoch = (SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap_or_default()
                .as_nanos() as u64)
                .max(1);
            self.net.coop.grant = Some(Grant {
                peer,
                epoch,
                model: v.vehicle.ty.def.path.clone(),
                uid: v.uid as u64,
            });
            log::info!("COOP1 conductor assigned: peer={peer}");
            self.coop_send(peer, Message::Grant(epoch));
        }
    }

    pub(crate) fn coop_request(&mut self) {
        if let Some((to, Target::Player)) = self.foot_bus().and_then(|b| self.coop_route(b)) {
            self.coop_send(to, Message::Request);
            self.coop_notice(
                "Pedido enviado ao motorista. Ele pode autorizar pelo menu Cooperativo.",
                "Request sent to the driver. They can allow it in the Cooperative menu.",
                "Anfrage an den Fahrer gesendet. Er kann sie im Koop-Menü erlauben.",
            );
        }
    }

    pub(crate) fn coop_peer_inside(&self, from: u32, target: Target) -> bool {
        let Some(lan) = self.net.lan.as_ref() else {
            return from == 0;
        };
        let Some(w) = lan
            .peers()
            .find(|p| p.pose.id == from)
            .and_then(|p| p.pose.walker)
        else {
            return false;
        };
        match target {
            Target::Player => {
                let local = w.aboard.filter(|a| {
                    a.owner == lan.my_id || self.net.coop.drive.visitor_owner() == Some(a.owner)
                }).map(|a| a.local).or_else(|| {
                    // The former driver's passive proxy can briefly lose its aboard
                    // marker while changing cameras. Its actual position must still
                    // lie inside this exact bus, never somewhere else in the map.
                    (self.net.coop.drive.visitor_owner() == Some(from)).then(|| {
                        let p=self.player.as_ref()?;
                        Some(p.vehicle.body_rotation().inverse().transform_vector3(
                            (DVec3::new(w.x,w.y,w.z)-p.vehicle.position).as_vec3()).to_array())
                    }).flatten()
                });
                self.player.as_ref().zip(local).is_some_and(|(p,local)| {
                    local[2].abs() <= 5.0 && omsi_lan::passenger::inside(
                        &omsi_lan::spawn::footprint_of(&p.vehicle, omsi_lan::spawn::BUS_BOX), local)
                })
            }
            Target::Ai(id) => {
                if lan.role != omsi_net::Role::Host || w.aboard.is_some() {
                    return false;
                }
                self.session
                    .traffic
                    .as_ref()
                    .and_then(|t| t.sim.cars.iter().find(|c| c.id == id))
                    .is_some_and(|c| {
                        let local = c.vehicle.body_rotation().inverse().transform_vector3(
                            (DVec3::new(w.x, w.y, w.z) - c.vehicle.position).as_vec3(),
                        );
                        local.z.abs() <= 5.0
                            && omsi_lan::passenger::inside(
                                &omsi_lan::spawn::footprint_of(
                                    &c.vehicle,
                                    omsi_lan::spawn::BUS_BOX,
                                ),
                                local.to_array(),
                            )
                    })
            }
        }
    }

    pub(crate) fn coop_desk(&self, epoch: u64) -> Desk {
        if let Some(desk)=self.drive_cash_desk() { let mut d=desk.clone();d.epoch=epoch;return d; }
        let Some(h) = self.session.humans.as_ref() else {
            return Desk {
                epoch,
                ..Default::default()
            };
        };
        let (name, price) = h.request.clone().unwrap_or_default();
        let pax = h
            .desk_busy
            .and_then(|id| h.people.iter().find(|p| p.id == id))
            .and_then(|p| {
                if let omsi_sim::people::State::Pax(x) = &p.state {
                    Some(x)
                } else {
                    None
                }
            });
        let paid = h
            .paid
            .map(|p| p.0)
            .or_else(|| pax.map(|p| p.paid))
            .unwrap_or(0.0);
        let due = h.change_due.unwrap_or_else(|| (paid - price).max(0.0));
        let given = h.money.as_ref().map_or(0.0, |m| m.change_value());
        // A finished passenger has left, but their coins may still block the next sale.
        // The owner's simulation is the only authority for this amount.
        let uncollected = h.sim.money.as_ref().map_or(0.0, |m| {
            let pieces: Vec<_> = m.placed.iter()
                .filter(|p| !p.3)
                .map(|p| p.2)
                .collect();
            m.value_of(&pieces)
        });
        let stage = pax.map_or(0, |p| match p.sub {
            3 | 4 => 1,
            5 => 2,
            6 => 4,
            7 => 3,
            _ => 0,
        });
        Desk {
            epoch,
            id: h.desk_busy.unwrap_or(0),
            stage,
            name,
            price,
            paid,
            due,
            given,
            uncollected,
        }
    }

    pub(crate) fn coop_receive(&mut self, from: u32, wire: &str) -> bool {
        if !wire.starts_with(omsi_lan::coop::PREFIX) {
            return false;
        }
        let Some(message) = Message::parse(wire) else {
            return true;
        };
        match message {
            Message::Request => {
                if self.coop_peer_inside(from, Target::Player) {
                    self.coop_send(
                        from,
                        Message::Grant(
                            self.net
                                .coop
                                .grant
                                .as_ref()
                                .filter(|g| g.peer == from)
                                .map_or(0, |g| g.epoch),
                        ),
                    );
                    self.coop_notice("Um passageiro pediu a função de cobrador. Abra Cooperativo para autorizar.",
                    "A passenger requested the conductor role. Open Cooperative to allow it.",
                    "Ein Fahrgast möchte kassieren. Im Koop-Menü erlauben.");
                }
            }
            Message::Grant(epoch) => {
                if self.foot_bus().and_then(|b| self.coop_route(b)) == Some((from, Target::Player))
                {
                    let fresh = self
                        .net
                        .coop
                        .remote
                        .is_none_or(|(id, e, _)| id != from || e != epoch);
                    if fresh {
                        self.net.coop.outbox.clear();
                        self.net.coop.mouse = None;
                        self.net.coop.desk = Desk::default();
                    }
                    self.net.coop.remote = Some((from, epoch, Instant::now()));
                    if fresh && epoch != 0 {
                        self.net.coop.panel = false;
                    }
                }
            }
            Message::Revoke(epoch) => {
                if self
                    .net
                    .coop
                    .remote
                    .is_some_and(|(id, e, _)| id == from && e == epoch)
                {
                    self.net.coop.remote = None;
                    self.net.coop.outbox.clear();
                    self.net.coop.mouse = None;
                    self.net.coop.camera = None;
                    self.coop_notice(
                        "O motorista desativou a função de cobrador.",
                        "The driver disabled the conductor role.",
                        "Der Fahrer hat die Kassiererrolle beendet.",
                    );
                }
            }
            Message::Keep(epoch) => {
                if let Some(h) = self
                    .net
                    .coop
                    .held
                    .get_mut(&from)
                    .filter(|h| h.action.epoch == epoch)
                {
                    h.seen = Instant::now();
                }
                if self.net.coop.held.get(&from).is_some_and(|h|h.action.epoch==epoch) {
                    self.drive_forward_coop(from,&Message::Keep(epoch));
                }
            }
            Message::Ack { epoch, seq, ok } => {
                let cue = self.net.coop.outbox.pending_kind(from, epoch, seq) == Some(4);
                let accepted = self.net.coop.outbox.acknowledge(from, epoch, seq);
                if accepted && ok && cue {
                    // The owner played the sound when it accepted this request. The
                    // conductor plays only on the first matching ACK, never retries.
                    // This button is the conductor's distinct "Fechou" signal,
                    // not a physical door command. Play the chosen cue on ACK
                    // regardless of the bus's own pneumatic door sound set.
                    self.coop_play_close_cue();
                }
                if accepted && !ok {
                    if epoch == 0 {
                        self.coop_notice("O host não conseguiu acionar esta campainha. Consulte o game.log do host.","The host could not operate this bell. See its game.log.","Der Host konnte diesen Haltewunschknopf nicht betätigen. Siehe seine game.log.");
                    } else {
                        self.coop_notice(
                            "Comando recusado: confira a autorização e o passageiro atual.",
                            "Control refused: check your permission and the current passenger.",
                            "Bedienung abgelehnt: Erlaubnis und aktuellen Fahrgast prüfen.",
                        );
                    }
                }
            }
            Message::Desk(d) => {
                if self
                    .net
                    .coop
                    .remote
                    .is_some_and(|(id, e, _)| id == from && e != 0 && e == d.epoch)
                {
                    if self.net.coop.desk != d && (d.stage != 0 || d.uncollected > 0.0) {
                        self.service_msg = Some((
                            format!(
                                "{}: {} | {} {:.2} | {} {:.2} | {} {:.2} / {:.2}",
                                text("Bilhete", "Ticket", "Fahrschein"),
                                d.name,
                                text("Preço", "Price", "Preis"),
                                d.price,
                                text("Pago", "Paid", "Bezahlt"),
                                d.paid,
                                text("Troco", "Change", "Rückgeld"),
                                d.given,
                                d.due
                            ),
                            8.0,
                        ));
                    }
                    self.net.coop.desk = d;
                }
            }
            Message::Action(a) => {
                let key = (from, a.epoch);
                // Refuse rather than execute an operation that cannot be recorded.
                if self.net.coop.receipts.len() >= 128 && !self.net.coop.receipts.contains_key(&key)
                {
                    self.coop_send(
                        from,
                        Message::Ack {
                            epoch: a.epoch,
                            seq: a.seq,
                            ok: false,
                        },
                    );
                    return true;
                }
                let cached = self
                    .net
                    .coop
                    .receipts
                    .get(&key)
                    .and_then(|r| r.cached(a.seq));
                let ok = match cached {
                    Some(ok) => {
                        if ok {
                            if let Some(h) = self
                                .net
                                .coop
                                .held
                                .get_mut(&from)
                                .filter(|h| h.action.epoch == a.epoch && h.action.seq == a.seq)
                            {
                                h.seen = Instant::now();
                            }
                        }
                        ok
                    }
                    None => {
                        let ok = self.coop_apply(from, &a);
                        // Delivery to another authority is not execution. Only its
                        // authenticated WorkResult may acknowledge this operation.
                        if self.net.coop.pending_delegated.contains_key(&(from, a.epoch, a.seq)) {
                            return true;
                        }
                        if matches!(a.phase, Phase::Down | Phase::Tap) {
                            log::info!("COOP1 action peer={from} seq={} target={:?} kind={} desk={} accepted={ok}", a.seq, a.target, a.kind, a.desk);
                        }
                        // Unauthorised epochs must not allocate an unbounded receipt table.
                        if ok || self.net.coop.receipts.contains_key(&key) {
                            self.net
                                .coop
                                .receipts
                                .entry(key)
                                .or_default()
                                .record(a.seq, ok);
                        }
                        ok
                    }
                };
                self.coop_send(
                    from,
                    Message::Ack {
                        epoch: a.epoch,
                        seq: a.seq,
                        ok,
                    },
                );
            }
        }
        true
    }

    pub(crate) fn coop_keep_delegated(&mut self,from:u32,epoch:u64){
        if let Some(h)=self.net.coop.held.get_mut(&from).filter(|h|h.action.epoch==epoch){h.seen=Instant::now();}
    }
    pub(crate) fn coop_apply_delegated(&mut self, from: u32, a: &Action) -> bool {
        let key = (from, a.epoch);
        if let Some(ok) = self.net.coop.receipts.get(&key).and_then(|r| r.cached(a.seq)) {
            if ok { self.coop_keep_delegated(from, a.epoch); }
            return ok;
        }
        if self.net.coop.receipts.len() >= 128 && !self.net.coop.receipts.contains_key(&key) {
            return false;
        }
        let ok = self.coop_apply_inner(from, a, true);
        self.net.coop.receipts.entry(key).or_default().record(a.seq, ok);
        log::info!("COOP1 delegated execution peer={from} epoch={} seq={} executed={ok}", a.epoch, a.seq);
        ok
    }
    pub(crate) fn coop_delegated_result(&mut self, actor: u32, epoch: u64, seq: u64, ok: bool) {
        if self.net.coop.pending_delegated.remove(&(actor, epoch, seq)).is_none() { return; }
        self.net.coop.receipts.entry((actor, epoch)).or_default().record(seq, ok);
        self.coop_send(actor, Message::Ack { epoch, seq, ok });
    }
    fn coop_apply(&mut self, from:u32, a:&Action)->bool { self.coop_apply_inner(from,a,false) }
    fn coop_apply_inner(&mut self, from: u32, a: &Action, delegated:bool) -> bool {
        if !delegated && from != 0 && !self.coop_peer_inside(from, a.target) {
            log::info!(
                "COOP1 refused: peer={from} target={:?} reason=passenger_not_inside",
                a.target
            );
            return false;
        }
        let event = if a.kind == 0 {
            let Some(v) = self.coop_vehicle_for_target(a.target) else {
                return false;
            };
            let Some((event, signature)) = mesh_info(v, a.part, a.index) else {
                return false;
            };
            if signature != a.signature {
                log::info!("COOP1 refused: peer={from} target={:?} part={} mesh={} event={event} reason=control_signature_mismatch", a.target, a.part, a.index);
                return false;
            }
            Some(event)
        } else {
            None
        };
        let passenger = event.as_deref().is_some_and(passenger_event);
        if !delegated && !omsi_lan::coop::authorized(
            self.net.coop.grant.as_ref().map(|g| (g.peer, g.epoch)),
            from,
            a,
            true,
            passenger,
            omsi_lan::coop::desk_bound(a.kind, event.as_deref()),
            self.coop_desk(a.epoch).id,
        ) {
            log::info!("COOP1 refused: peer={from} target={:?} kind={} event={event:?} desk={} current_desk={} reason=authorization_or_stale_transaction", a.target, a.kind, a.desk, self.coop_desk(a.epoch).id);
            return false;
        }
        if !delegated && a.target == Target::Player && a.kind <= 1
            && self.net.coop.drive.owner_peer_active().is_some() {
            if self.net.coop.pending_delegated.len() >= 128
                && !self.net.coop.pending_delegated.contains_key(&(from, a.epoch, a.seq)) { return false; }
            if !self.drive_forward_coop(from, &Message::Action(a.clone())) { return false; }
            self.net.coop.pending_delegated.entry((from, a.epoch, a.seq)).or_insert_with(Instant::now);
            if matches!(a.phase,Phase::Up|Phase::Tap) { self.net.coop.held.remove(&from); }
            else { self.net.coop.held.insert(from,Held{from,action:a.clone(),seen:Instant::now(),control:None}); }
            return true;
        }
        if !delegated && a.target==Target::Player && matches!(a.kind,2|3) && self.net.coop.drive.visitor_active() {
            if self.net.coop.pending_delegated.len()>=128 && !self.net.coop.pending_delegated.contains_key(&(from,a.epoch,a.seq)) { return false; }
            if !self.drive_forward_cash(from,a) { return false; }
            self.net.coop.pending_delegated.entry((from,a.epoch,a.seq)).or_insert_with(Instant::now);
            return true;
        }
        if a.kind == 4 {
            // One-shot conductor signal, NOT a door trigger: the driver is in control
            // of the physical doors. No ticket transaction or pressed control is changed.
            if a.phase != Phase::Tap || a.target != Target::Player || a.part != 0
                || a.index != 0 || a.signature != 0 || a.x != 0.0 || a.y != 0.0 {
                return false;
            }
            let now = Instant::now();
            if self.net.coop.last_close_signal.is_some_and(|t| now.saturating_duration_since(t) < Duration::from_millis(850)) {
                return false;
            }
            self.net.coop.last_close_signal = Some(now);
            // The owner hears exactly the same custom cue the conductor hears
            // on first ACK; later retransmissions do not replay either side.
            self.coop_play_close_cue();
            log::info!("COOP1 closing signal by conductor peer={from}");
            return true;
        }
        if a.phase == Phase::Up {
            let matches = self.net.coop.held.get(&from).is_some_and(|h| {
                h.action.epoch == a.epoch
                    && h.action.target == a.target
                    && h.action.kind == a.kind
                    && h.action.part == a.part
                    && h.action.index == a.index
            });
            if matches {
                self.coop_finish_held_from(from, true);
            }
            return matches;
        }
        if a.kind == 5 {
            // Shift+T / Collect payment. Never simulate money on the visiting client:
            // consume the real coins on the authoritative vehicle owner's desk.
            if a.phase != Phase::Tap || a.target != Target::Player || a.part != 0
                || a.index != 0 || a.signature != 0 || a.x != 0.0 || a.y != 0.0
            {
                return false;
            }
            let Some(m) = self.session.humans.as_mut().and_then(|h| h.sim.money.as_mut()) else {
                return false;
            };
            let coin_count = m.currency.as_ref().map_or(0, |c| c.coins.len());
            let payment_count = m.placed.iter().filter(|p| !p.3).count();
            if payment_count == 0 {
                return false;
            }
            let had_coins = m.placed.iter().any(|p| !p.3 && p.2 < coin_count);
            // Do not remove the change lying on the other tray.
            m.clear(false);
            if had_coins {
                if let Some(p) = self.player.as_mut() {
                    p.vehicle.host.fired_triggers.push("ev_ticketsale_takemoney".into());
                }
            }
            log::info!("COOP cashier collected {payment_count} payment pieces for peer={from}");
            return true;
        }
        if a.kind == 2 || a.kind == 3 {
            if a.phase != Phase::Tap {
                return false;
            }
            if a.kind == 2 {
                let valid = self
                    .player
                    .as_ref()
                    .and_then(|p| p.vehicle.host.tickets.as_ref())
                    .is_some_and(|t| (a.index as usize) < t.tickets.len());
                if !valid { return false; }
                self.player
                    .as_mut()
                    .unwrap()
                    .vehicle
                    .set_engine_var("GivenTicket", a.index as f32);
            } else {
                let valid = self
                    .session
                    .humans
                    .as_ref()
                    .and_then(|h| h.money.as_ref())
                    .and_then(|m| m.currency.as_ref())
                    .is_some_and(|c| (a.index as usize) < c.coins.len() + c.bills.len());
                if !valid || self.coop_desk(a.epoch).stage != 3 {
                    return false;
                }
                self.player
                    .as_mut()
                    .unwrap()
                    .vehicle
                    .host
                    .change_coins
                    .push(a.index as usize);
            }
            return true;
        }
        if a.phase == Phase::Drag {
            if !self.net.coop.held.get(&from).is_some_and(|h| {
                h.action.target == a.target && h.action.kind == a.kind && h.action.index == a.index
            }) {
                return false;
            }
        } else {
            self.coop_release_held_from(from);
            if a.target == Target::Player {
                self.coop_cancel_auto();
            }
        }
        let mut pressed_control = None;
        if a.kind == 0 {
            let event = event.unwrap();
            let held = self
                .net
                .coop
                .held
                .get(&from)
                .and_then(|h| h.control.clone());
            let Some(v) = self.coop_vehicle_mut(a.target) else {
                return false;
            };
            let control = if a.phase == Phase::Drag {
                let Some(mut control) = held else {
                    return false;
                };
                control.drag(
                    v,
                    &event,
                    a.x.clamp(-500.0, 500.0),
                    a.y.clamp(-500.0, 500.0),
                );
                control
            } else {
                let Some(control) = crate::mouse_control::Press::begin(v, &event) else {
                    log::info!(
                        "COOP1 control unavailable: event={event} part={} mesh={}",
                        a.part,
                        a.index
                    );
                    return false;
                };
                control
            };
            log::info!(
                "COOP1 control peer={from} phase={:?} event={event} part={} mesh={}",
                a.phase,
                a.part,
                a.index
            );
            pressed_control = Some(control);
            if a.phase != Phase::Drag && bell(&event) {
                self.coop_ring(from, a.target);
            }
        } else if a.kind == 1 {
            use omsi_sim::htmltex::PointerKind;
            let k = match a.phase {
                Phase::Down | Phase::Tap => PointerKind::Down,
                Phase::Drag => PointerKind::Move,
                Phase::Up => PointerKind::Up,
            };
            let (x, y) = (a.x.clamp(0.0, 1.0), a.y.clamp(0.0, 1.0));
            let pointer = match k {
                PointerKind::Down => omsi_plugin::POINTER_DOWN,
                PointerKind::Move => omsi_plugin::POINTER_MOVE,
                _ => omsi_plugin::POINTER_UP,
            };
            let handled = a.target == Target::Player
                && self
                    .integrations
                    .plugin_pointer
                    .push(a.index as usize, pointer, x, y);
            if !handled
                && !self
                    .coop_vehicle_mut(a.target)
                    .is_some_and(|v| v.html_pointer(a.index as usize, x, y, k))
            {
                return false;
            }
        }
        self.net.coop.held.insert(
            from,
            Held {
                from,
                action: a.clone(),
                seen: Instant::now(),
                control: pressed_control,
            },
        );
        if a.phase == Phase::Tap {
            self.coop_finish_held_from(from, true);
        }
        true
    }

    fn coop_vehicle_for_target(&self, target: Target) -> Option<&VehicleInstance> {
        match target {
            Target::Player => self.player.as_ref().map(|p| &p.vehicle),
            Target::Ai(id) => self
                .session
                .traffic
                .as_ref()?
                .sim
                .cars
                .iter()
                .find(|c| c.id == id)
                .map(|c| &c.vehicle),
        }
    }

    fn coop_release_held(&mut self) {
        let peers: Vec<_> = self.net.coop.held.keys().copied().collect();
        for from in peers {
            self.coop_release_held_from(from);
        }
    }

    fn coop_release_held_from(&mut self, from: u32) {
        self.coop_finish_held_from(from, false);
    }

    fn coop_cancel_auto(&mut self) {
        if let Some((_, control)) = self.net.coop.auto_drag.take() {
            if let Some(p) = self.player.as_mut() {
                control.cancel(&mut p.vehicle);
            }
        }
    }

    fn coop_finish_held_from(&mut self, from: u32, completed: bool) {
        let Some(h) = self.net.coop.held.remove(&from) else {
            return;
        };
        if h.action.kind == 0 {
            if let Some(v) = self.coop_vehicle_mut(h.action.target) {
                if let Some((event, signature)) = mesh_info(v, h.action.part, h.action.index)
                    .filter(|(_, s)| *s == h.action.signature)
                {
                    let _ = signature;
                    let ty = if h.action.part == 0 {
                        Some(&v.ty)
                    } else {
                        v.trailers.get(h.action.part as usize - 1).map(|t| &t.ty)
                    };
                    let animation = ty
                        .and_then(|ty| {
                            ty.meshes
                                .get(h.action.index as usize)
                                .and_then(|m| ty.model.meshes.get(m.def_index))
                        })
                        .and_then(|m| m.animations.first())
                        .map(|a| a.variable.clone());
                    if let Some(control) = h.control {
                        let auto = control.release(v, &event, animation, completed);
                        if h.action.target == Target::Player {
                            self.coop_cancel_auto();
                            self.net.coop.auto_drag = auto.map(|a| (from, a));
                        }
                    } else {
                        v.trigger(&format!("{event}_off"));
                    }
                }
            }
        } else if h.action.kind == 1 {
            let a = &h.action;
            let handled = a.target == Target::Player
                && self.integrations.plugin_pointer.push(
                    a.index as usize,
                    omsi_plugin::POINTER_UP,
                    a.x.clamp(0.0, 1.0),
                    a.y.clamp(0.0, 1.0),
                );
            if !handled {
                if let Some(v) = self.coop_vehicle_mut(a.target) {
                    v.html_pointer(
                        a.index as usize,
                        a.x.clamp(0.0, 1.0),
                        a.y.clamp(0.0, 1.0),
                        omsi_sim::htmltex::PointerKind::Up,
                    );
                }
            }
        }
    }

    fn coop_ring(&mut self, from: u32, target: Target) {
        let bus = match target {
            Target::Player => BusId::Player,
            Target::Ai(id) => BusId::Ai(id),
        };
        let Some(h) = self.session.humans.as_ref() else {
            return;
        };
        let Some(b) = h.last_buses.iter().find(|b| b.id == bus) else {
            return;
        };
        if b.cabin.exits.is_empty() {
            return;
        }
        let request = omsi_sim::people::DoorRequest {
            bus,
            exit: true,
            index: 0,
        };
        let source = 0x8000_0000 | from;
        self.net.coop.bells.retain(|(s, _)| *s != source);
        self.net.coop.bells.push((source, request));
    }

    pub(crate) fn coop_tick(&mut self) {
        let now = Instant::now();
        self.net.coop.pending_delegated.retain(|_, since| now.saturating_duration_since(*since) < Duration::from_secs(15));
        let dt = self
            .net
            .coop
            .last_tick
            .replace(now)
            .map_or(0.0, |last| {
                now.saturating_duration_since(last).as_secs_f32()
            })
            .min(0.05);
        if self.net.coop.mouse.is_some()
            && (self.view != "foot"
                || self.game_panels_mouse()
                || self.menus.game_menu.is_some()
                || self.photo.is_some()
                || self.window.as_ref().is_some_and(|w| !w.has_focus())
                || self.net.coop.mouse.as_ref().is_some_and(|(to, a)| {
                    self.foot_bus().and_then(|b| self.coop_route(b)) != Some((*to, a.target))
                }))
        {
            if let Some((to, a)) = self.net.coop.mouse.take() {
                self.coop_queue(
                    to,
                    Action {
                        phase: Phase::Up,
                        ..a
                    },
                );
            }
            self.input.dragging = false;
        }
        let session = self.net.lan.as_ref().map(|l| (l.session, l.my_id));
        if self.net.coop.session != session {
            self.drive_reset(false);
            self.coop_release_held();
            self.coop_cancel_auto();
            self.coop_drop_ticket_blocks();
            self.net.coop = CoopState {
                session,
                ..Default::default()
            };
        }
        self.character_sync();
        self.drive_tick();
        let revoke = self.net.coop.grant.as_ref().is_some_and(|g| {
            !self.coop_peer_inside(g.peer, Target::Player)
                || self
                    .player
                    .as_ref()
                    .is_none_or(|p| p.vehicle.ty.def.path != g.model || p.uid as u64 != g.uid)
        });
        if revoke {
            self.coop_grant(None);
        }
        if let Some((from, mut control)) = self.net.coop.auto_drag.take() {
            let live = self.drive_delegated_actor(from) || (self.net.coop.grant.as_ref().is_some_and(|g| g.peer == from)
                && self.coop_peer_inside(from, Target::Player));
            if let Some(p) = self.player.as_mut() {
                if live && (self.paused || control.step(&mut p.vehicle, dt)) {
                    self.net.coop.auto_drag = Some((from, control));
                } else {
                    control.cancel(&mut p.vehicle);
                }
            }
        }
        let expired: Vec<_> = self
            .net
            .coop
            .held
            .values()
            .filter(|h| {
                now.duration_since(h.seen) > Duration::from_secs(1)
                    || (h.from != 0 && !self.coop_peer_inside(h.from, h.action.target))
            })
            .map(|h| h.from)
            .collect();
        for peer in expired {
            self.coop_release_held_from(peer);
        }
        if self.net.coop.remote.is_some_and(|(owner, _, seen)| {
            now.duration_since(seen) > Duration::from_secs(4)
                || self.foot_bus().and_then(|b| self.coop_route(b)) != Some((owner, Target::Player))
                || self.foot_bus().and_then(|b| self.coop_vehicle(b)).is_none()
        }) {
            self.net.coop.remote = None;
            self.net.coop.outbox.clear();
            self.net.coop.mouse = None;
            self.net.coop.camera = None;
        }
        if self
            .net
            .coop
            .last_keep
            .is_none_or(|t| now.duration_since(t) > Duration::from_millis(200))
        {
            self.net.coop.last_keep = Some(now);
            if let Some((owner, a)) = self.net.coop.mouse.as_ref() {
                let (owner, epoch) = (*owner, a.epoch);
                if owner == 0 {
                    if let Some(h) = self.net.coop.held.get_mut(&0) {
                        h.seen = now;
                    }
                } else if self.net.coop.outbox.is_empty() {
                    self.coop_send(owner, Message::Keep(epoch));
                }
            }
        }
        match self.net.coop.outbox.poll(now) {
            Ok(Some((to, m))) => self.coop_send(to, m),
            Err(()) => {
                self.net.coop.mouse = None;
                self.coop_notice(
                    "O outro jogo não respondeu. Confira se usa o protótipo cooperativo.",
                    "The other game did not answer. Check that it uses the cooperative prototype.",
                    "Das andere Spiel antwortet nicht. Die Koop-Testversion prüfen.",
                );
            }
            _ => {}
        }
        if self
            .net
            .coop
            .last_status
            .is_none_or(|t| now.duration_since(t) > Duration::from_millis(400))
        {
            self.net.coop.last_status = Some(now);
            if let Some(g) = self.net.coop.grant.as_ref() {
                let (peer, epoch) = (g.peer, g.epoch);
                if self
                    .net
                    .coop
                    .last_grant
                    .is_none_or(|t| now.duration_since(t) >= Duration::from_secs(1))
                {
                    self.net.coop.last_grant = Some(now);
                    self.coop_send(peer, Message::Grant(epoch));
                }
                let d = self.coop_desk(epoch);
                if self
                    .net
                    .coop
                    .last_sent_desk
                    .as_ref()
                    .is_none_or(|(at, last)| {
                        *last != d || now.duration_since(*at) > Duration::from_secs(2)
                    })
                {
                    self.net.coop.last_sent_desk = Some((now, d.clone()));
                    self.coop_send(peer, Message::Desk(d));
                }
            }
        }
        if let Some(h) = self.session.humans.as_mut() {
            self.net.coop.bells.retain(|(source, request)| {
                if h.requested_door_open(*request) {
                    h.clear_manual_door_request(*source);
                    false
                } else {
                    h.set_manual_door_request(*source, *request, 1.0);
                    true
                }
            });
        }
        if self
            .net
            .coop
            .camera
            .as_ref()
            .is_some_and(|c| self.foot_bus() != Some(c.bus) || self.coop_vehicle(c.bus).is_none())
        {
            self.net.coop.camera = None;
        }
    }

    fn coop_cursor_ray(&self, bus: BusId) -> Option<(DVec3, glam::Vec3, f32)> {
        let v = self.coop_vehicle(bus)?;
        let (_, shown) = self.net.coop.drawn.filter(|(b, _)| *b == bus)?;
        let surface = self.gfx.surface.as_ref()?;
        let size = (surface.config.width, surface.config.height);
        Some(shown.cursor_ray(BodyPose::of(v), |camera| {
            self.cockpit_cursor_ray(camera, size)
        }))
    }

    fn coop_hit(&self) -> Option<Action> {
        if self.view != "foot" {
            return None;
        }
        let bus = self.foot_bus()?;
        let (to, target) = self.coop_route(bus)?;
        let v = self.coop_vehicle(bus)?;
        let (o, dir, spread) = self.coop_cursor_ray(bus)?;
        let epoch = self
            .net
            .coop
            .remote
            .filter(|(id, _, _)| *id == to)
            .map_or(0, |(_, e, _)| e);
        let base = Action {
            epoch,
            seq: 1,
            target,
            desk: self.net.coop.desk.id,
            kind: 0,
            part: 0,
            index: 0,
            signature: 0,
            phase: Phase::Down,
            x: 0.0,
            y: 0.0,
        };
        if epoch != 0 {
            if let Some(index) = self
                .net
                .coop
                .ticket_blocks
                .as_ref()
                .filter(|(owner, _)| *owner == to)
                .and_then(|(_, blocks)| blocks.hit(o, dir, v))
                .or_else(|| (bus == BusId::Player).then(|| self.gfx.sim_view.people.ticket_blocks.as_ref().and_then(|b|b.hit(o,dir,v))).flatten())
            {
                return Some(Action {
                    kind: 2,
                    index: index as u32,
                    phase: Phase::Tap,
                    ..base
                });
            }
            if let Some((index, x, y)) =
                crate::player::pick_html_in(v, o, dir, &self.integrations.plugin_pointer.pages)
            {
                return Some(Action {
                    kind: 1,
                    index: index as u32,
                    x,
                    y,
                    ..base
                });
            }
        }
        let allowed = |event: &str| epoch != 0 || passenger_event(event);
        let hit =
            crate::player::pick_in_filtered(v, o, dir, spread, crate::player::Reach::ANY, &allowed)
                .map(|(i, _)| (0, i))
                .or_else(|| {
                    crate::player::pick_trailer_in_filtered(
                        v,
                        o,
                        dir,
                        spread,
                        crate::player::Reach::ANY,
                        &allowed,
                    )
                    .map(|((part, i), _)| ((part + 1) as u8, i))
                })?;
        let (event, signature) = mesh_info(v, hit.0, hit.1 as u32)?;
        if epoch == 0 && !passenger_event(&event) {
            return None;
        }
        Some(Action {
            part: hit.0,
            index: hit.1 as u32,
            signature,
            ..base
        })
    }

    pub(crate) fn coop_hover(&mut self) -> bool {
        if self.view != "foot" || self.foot_bus().and_then(|b| self.coop_route(b)).is_none() {
            return false;
        }
        let hit = self.coop_hit();
        self.menus.hover_hand = hit.is_some();
        self.menus.hover_part = None;
        self.menus.hover = hit.and_then(|a| {
            if a.kind == 0 {
                self.foot_bus()
                    .and_then(|b| self.coop_vehicle(b))
                    .and_then(|v| mesh_info(v, a.part, a.index))
                    .map(|x| x.0)
            } else if a.kind == 2 {
                self.session.humans.as_ref().and_then(|h| h.tickets.as_ref())
                    .and_then(|p| p.tickets.get(a.index as usize))
                    .map(|t| format!("{} ({:.2})", t.name, t.value))
                    .or_else(|| Some(format!("{} {}", text("Bilhete", "Ticket", "Fahrschein"), a.index + 1)))
            } else {
                Some(text("Caixa", "Cash desk", "Kasse").into())
            }
        });
        self.set_cursor_kind(if self.input.mouse_look {
            3
        } else if self.menus.hover_hand {
            1
        } else {
            0
        });
        true
    }

    fn coop_queue(&mut self, to: u32, a: Action) -> bool {
        if to == 0 {
            self.coop_apply(0, &a)
        } else {
            self.net.coop.outbox.push(to, a, Instant::now())
        }
    }

    pub(crate) fn coop_mouse(&mut self, down: bool) -> bool {
        // A delegated original bus is now remote too. Use the authority route
        // instead of treating BusId::Player as proof of driving permission.
        let active = self.view == "foot" && self.foot_bus().and_then(|b| self.coop_route(b)).is_some();
        if !down {
            if let Some((to, a)) = self.net.coop.mouse.take() {
                self.input.dragging = false;
                self.coop_queue(
                    to,
                    Action {
                        phase: Phase::Up,
                        ..a
                    },
                );
                return true;
            }
        }
        if !active {
            return false;
        }
        if down {
            if let Some(a) = self.coop_hit() {
                let to = self.foot_bus().and_then(|b| self.coop_route(b)).unwrap().0;
                log::info!(
                    "COOP1 click bus={:?} recipient={to} kind={} part={} index={}",
                    self.foot_bus(),
                    a.kind,
                    a.part,
                    a.index
                );
                if a.kind == 0 {
                    if let Some((event, _)) = self
                        .foot_bus()
                        .and_then(|b| self.coop_vehicle(b))
                        .and_then(|v| mesh_info(v, a.part, a.index))
                    {
                        log::info!(
                            "COOP1 click event={event} epoch={:x} desk={}",
                            a.epoch,
                            a.desk
                        );
                    }
                }
                if self.coop_queue(to, a.clone()) {
                    if a.phase != Phase::Tap {
                        self.net.coop.mouse = Some((to, a));
                        self.input.dragging = true;
                    }
                } else {
                    self.coop_notice(
                        "O botão não pôde ser executado. Consulte o game.log.",
                        "The control could not be executed. See game.log.",
                        "Das Bedienelement konnte nicht ausgeführt werden. Siehe game.log.",
                    );
                }
            } else {
                let model = self.foot_bus().and_then(|b| self.coop_vehicle(b));
                if model.is_none() {
                    log::info!("COOP1 target unavailable: bus={:?}", self.foot_bus());
                    self.net.coop.remote = None;
                    self.net.coop.outbox.clear();
                    self.net.coop.mouse = None;
                    self.net.coop.camera = None;
                    self.net.coop.drawn = None;
                    self.coop_notice(
                        "O ônibus está indisponível. Aguarde o carregamento ou confira se o motorista continua conectado.",
                        "The bus is unavailable. Wait for it to load or check that the driver is still connected.",
                        "Der Bus ist nicht verfügbar. Laden abwarten oder die Verbindung des Fahrers prüfen.");
                    return true;
                }
                let bells = model.map_or(0, coop_view::recognized_bells);
                let pointed = self
                    .foot_bus()
                    .and_then(|b| self.coop_cursor_ray(b))
                    .zip(model)
                    .and_then(|((o, d, s), v)| {
                        crate::player::pick_in(v, o, d, s, crate::player::Reach::ANY)
                            .and_then(|(i, _)| mesh_info(v, 0, i as u32))
                            .map(|m| m.0)
                    });
                let granted = self.net.coop.remote.is_some_and(|(_, epoch, _)| epoch != 0);
                let cached = model.map_or(0, |v| {
                    v.ty.meshes
                        .iter()
                        .filter(|m| m.pick_geometry.get().is_some_and(Option::is_some))
                        .count()
                });
                log::info!("COOP1 no passenger control: bus={:?} model={} recognized_bells={bells} pointed={pointed:?} conductor_authorized={granted} restored_pick_meshes={cached}",self.foot_bus(),model.map_or_else(String::new,|v|v.ty.def.path.display().to_string()));
                if !granted
                    && pointed
                        .as_ref()
                        .is_some_and(|event| !passenger_event(event))
                {
                    if matches!(self.foot_bus(), Some(BusId::Ai(id))
                        if crate::humans::remote_bus_player(id).is_some())
                    {
                        self.coop_notice(
                            "Esse controle exige autorização do motorista para a função de cobrador.",
                            "This control requires the driver's conductor permission.",
                            "Dieses Bedienelement benötigt die Kassiererfreigabe des Fahrers.",
                        );
                    } else {
                        self.coop_notice(
                            "Na IA, a caixa é automática. Como passageiro, use a campainha.",
                            "AI ticket service is automatic. As a passenger, use the bell.",
                            "Der KI-Fahrscheinverkauf ist automatisch. Als Fahrgast kannst du klingeln.",
                        );
                    }
                } else if bells == 0 && !granted {
                    self.coop_notice("Este modelo não possui campainha clicável reconhecida. O caminho do ônibus está no game.log.","This model has no recognized clickable bell. Its path is in game.log.","Dieses Modell hat keinen erkannten klickbaren Haltewunschknopf. Der Pfad steht in game.log.");
                }
            }
        }
        true
    }

    pub(crate) fn coop_drag(&mut self) {
        let Some((to, a)) = self.net.coop.mouse.clone() else {
            return;
        };
        let (dx, dy) = std::mem::take(&mut self.input.drag_delta);
        if dx.abs() + dy.abs() < 0.001 {
            return;
        }
        let (x, y) = if a.kind == 1 {
            let Some(bus) = self.foot_bus() else { return };
            self.coop_vehicle(bus)
                .zip(self.coop_cursor_ray(bus))
                .and_then(|(v, (o, d, _))| {
                    crate::player::pick_html_in(v, o, d, &self.integrations.plugin_pointer.pages)
                })
                .filter(|(i, _, _)| *i == a.index as usize)
                .map_or((a.x, a.y), |(_, x, y)| (x, y))
        } else {
            (dx.clamp(-500.0, 500.0), dy.clamp(-500.0, 500.0))
        };
        self.coop_queue(
            to,
            Action {
                phase: Phase::Drag,
                x,
                y,
                ..a
            },
        );
    }

    pub(crate) fn coop_camera_slot(&mut self, delta: i32) {
        let driver = self.net.coop.camera.as_ref().is_some_and(|c| c.driver);
        self.coop_camera_choose(driver, delta);
    }

    fn coop_camera_choose(&mut self, driver: bool, delta: i32) {
        let Some(bus) = self.foot_bus().filter(|b| *b != BusId::Player) else { return };
        let friend = matches!(bus, BusId::Ai(id) if crate::humans::remote_bus_player(id).is_some());
        if driver && (!friend || (!self.net.coop.remote.is_some_and(|(_, e, _)| e != 0) && !self.net.coop.drive.visitor_active())) {
            self.coop_notice("Somente cobradores autorizados usam as câmeras F1.",
                "Only authorized conductors may use F1 cameras.",
                "Nur autorisierte Kassierer dürfen F1-Kameras verwenden.");
            return;
        }
        let Some(v) = self.coop_vehicle(bus) else { return };
        let cams = coop_view::camera_choices(&v.ty.def, driver);
        if cams.is_empty() {
            self.coop_notice("Este ônibus não tem câmeras desse tipo.",
                "This bus has no cameras of that type.",
                "Dieser Bus hat keine Kameras dieses Typs.");
            return;
        }
        let current = self.net.coop.camera.as_ref()
            .filter(|c| c.bus == bus && c.driver == driver).map(|c| c.slot);
        let first = if driver && !self.net.coop.drive.visitor_active() { v.ty.def.view_ticketselling.unwrap_or(0).min(cams.len()-1) } else { 0 };
        let slot = current.map_or(first, |i| (i as i32 + delta).rem_euclid(cams.len() as i32) as usize);
        let Some(f) = self.session.on_foot.as_ref() else { return };
        self.net.coop.camera = Some(CameraView {
            bus, local: cams[slot].clone(), yaw0: f.yaw, pitch0: f.pitch, slot, driver,
        });
        if let Some(f) = self.session.on_foot.as_mut() {
            f.face_seat = false;
            f.vel = glam::DVec2::ZERO;
        }
    }

    pub(crate) fn coop_camera_load_group(&mut self, driver: bool) {
        self.coop_camera_choose(driver, 0);
        if self.net.coop.camera.as_ref().is_none_or(|c| c.driver != driver) { return; }
        // A driver takeover starts at the actual front driving view, never at a
        // saved cashier camera from the earlier conductor role.
        if driver && self.net.coop.drive.visitor_active() {
            self.net.coop.camera_edit = false;
        } else {
            self.coop_camera_load_saved();
            self.net.coop.camera_edit = true;
        }
    }

    pub(crate) fn coop_camera_adjust(&mut self, axis: usize, delta: f32) {
        if let Some(c) = self.net.coop.camera.as_mut() {
            c.local.pos[axis] += delta;
        }
    }

    pub(crate) fn coop_camera_save(&mut self) {
        let Some(mut c) = self.net.coop.camera.clone() else {
            return;
        };
        if let Some(f) = self.session.on_foot.as_ref() {
            c.local.yaw = (c.local.yaw + f.yaw - c.yaw0).rem_euclid(360.0);
            c.local.pitch = (c.local.pitch + f.pitch - c.pitch0).clamp(-85.0, 85.0);
            c.yaw0 = f.yaw;
            c.pitch0 = f.pitch;
        }
        let Some(v) = self.coop_vehicle(c.bus) else {
            return;
        };
        let key = self.coop_camera_key(c.bus, v, c.driver);
        let Some(path) = crate::lan::data_dir().map(|p| p.join("coop-cameras.json")) else {
            return;
        };
        let mut entries: std::collections::BTreeMap<String, serde_json::Value> =
            std::fs::read(&path)
                .ok()
                .and_then(|b| serde_json::from_slice(&b).ok())
                .unwrap_or_default();
        entries.insert(key, serde_json::json!({"pos":c.local.pos,"yaw":c.local.yaw,"pitch":c.local.pitch,"fov":c.local.fov,"dist":c.local.dist,"slot":c.slot}));
        let result = (|| -> std::io::Result<()> {
            if let Some(p) = path.parent() {
                std::fs::create_dir_all(p)?;
            }
            let b = serde_json::to_vec_pretty(&entries)?;
            std::fs::write(&path, b)
        })();
        if result.is_ok() {
            self.net.coop.camera = Some(c);
            self.net.coop.camera_edit = false;
            self.coop_notice(
                "Câmera salva para este ônibus.",
                "Camera saved for this bus.",
                "Kamera für diesen Bus gespeichert.",
            );
        } else {
            self.coop_notice(
                "Não foi possível salvar a câmera.",
                "Could not save the camera.",
                "Kamera konnte nicht gespeichert werden.",
            );
        }
    }

    fn coop_camera_load_saved(&mut self) {
        let Some(c) = self.net.coop.camera.as_ref() else { return };
        let (bus, driver) = (c.bus, c.driver);
        let Some(v) = self.coop_vehicle(bus) else { return };
        let key = self.coop_camera_key(bus, v, driver);
        // coop.5 had one combined conductor camera bank; preserve its old F2
        // adjustment as a fallback, without ever letting a F1 view load it.
        let legacy = format!("v2|{}|{}", if matches!(bus,BusId::Ai(id) if crate::humans::remote_bus_player(id).is_some()) {
            "conductor"
        } else { "passenger" }, camera_key(v));
        let entry = crate::lan::data_dir()
            .and_then(|p| std::fs::read(p.join("coop-cameras.json")).ok())
            .and_then(|b| serde_json::from_slice::<serde_json::Value>(&b).ok())
            .and_then(|v| v.get(&key).cloned().or_else(|| {
                // Legacy conductor slots included all driver cameras first. A
                // legacy F1 seat must never overwrite a new F2 passenger view.
                if driver { return None; }
                let old = v.get(&legacy)?.clone();
                let driver_count = self.coop_vehicle(bus)?.ty.def.cameras_driver.len() as u64;
                let slot = old.get("slot")?.as_u64()?;
                if matches!(bus, BusId::Ai(id) if crate::humans::remote_bus_player(id).is_some()) {
                    if slot < driver_count { return None; }
                    let mut old = old;
                    old["slot"] = serde_json::json!(slot - driver_count);
                    Some(old)
                } else { Some(old) }
            }));
        if let (Some(entry), Some(c)) = (entry, self.net.coop.camera.as_mut()) {
            if let Some(pos) = entry.get("pos")
                .and_then(|v| serde_json::from_value::<[f32;3]>(v.clone()).ok())
                .filter(|p| p.iter().all(|v| v.is_finite() && v.abs() < 100.0)) { c.local.pos = pos; }
            for (key, value) in [
                ("yaw", &mut c.local.yaw), ("pitch", &mut c.local.pitch),
                ("fov", &mut c.local.fov), ("dist", &mut c.local.dist),
            ] {
                if let Some(v) = entry.get(key).and_then(|v| v.as_f64()).filter(|v| v.is_finite()) {
                    *value = v as f32;
                }
            }
            c.local.fov = c.local.fov.clamp(15.0, 110.0);
            c.local.pitch = c.local.pitch.clamp(-85.0, 85.0);
            c.local.yaw = c.local.yaw.rem_euclid(360.0);
            c.local.dist = c.local.dist.clamp(0.0, 100.0);
            // A saved camera belongs to a specific bank; don't use a stale slot
            // to select a camera from the wrong F1/F2 list later.
            if let Some(slot) = entry.get("slot").and_then(|v| v.as_u64()) {
                c.slot = (slot as usize).min(4096);
            }
        }
    }

    pub(crate) fn coop_camera_frame(&mut self) {
        if self.view != "foot" {
            return;
        }
        let Some(c) = self
            .net
            .coop
            .camera
            .clone()
            .filter(|c| self.foot_bus() == Some(c.bus))
        else {
            return;
        };
        let Some(f) = self.session.on_foot.as_ref() else {
            return;
        };
        let Some(v) = self.coop_vehicle(c.bus) else {
            return;
        };
        let (position, yaw, pitch, roll) =
            coop_view::camera_pose(v, &c.local, (f.yaw, f.pitch), (c.yaw0, c.pitch0));
        let fov = if self.settings.fov >= 20.0 {
            self.settings.fov.min(120.0)
        } else {
            c.local.fov.clamp(15.0, 110.0)
        };
        let zoom = self.cam.view_zoom.get("foot").copied().unwrap_or(1.0);
        if let Some(cam) = self.camera.as_mut() {
            cam.position = position;
            cam.yaw = yaw;
            cam.pitch = pitch;
            cam.roll = roll;
            cam.fov_deg = (fov * zoom).clamp(8.0, 120.0);
        }
    }

    fn coop_camera_key(&self, bus: BusId, v: &VehicleInstance, driver: bool) -> String {
        let role = if matches!(bus,BusId::Ai(id) if crate::humans::remote_bus_player(id).is_some())
        {
            "conductor"
        } else {
            "passenger"
        };
        format!("v3|{role}|{}|{}", if driver { "f1" } else { "f2" }, camera_key(v))
    }

    pub(crate) fn coop_drawn_view(&mut self) {
        self.coop_sync_ticket_blocks();
        self.coop_camera_frame();
        // Snapshot after the camera override, while the bus still has its drawn
        // pose. Hover, press and HTML drag all use this same view, even if an
        // intervening walking tick puts the avatar's eyes in `self.camera`.
        self.net.coop.drawn = self
            .foot_bus()
            .filter(|_| self.view == "foot")
            .and_then(|bus| {
                self.coop_vehicle(bus).zip(self.camera.as_ref()).map(|(v, camera)| {
                    (bus, PresentedView { camera: *camera, body: BodyPose::of(v) })
                })
            });
    }

    fn coop_drop_ticket_blocks(&mut self) {
        if let Some((_, blocks)) = self.net.coop.ticket_blocks.take() {
            if let (Some(renderer), Some(scene)) = (self.renderer.as_ref(), self.scene.as_mut()) {
                blocks.remove(renderer, scene);
            }
        }
    }

    /// Native paper tickets belong to the driver's simulation. Show their existing
    /// attachment layout to the authorized conductor and send taps to that owner.
    fn coop_sync_ticket_blocks(&mut self) {
        let owner = self
            .foot_bus()
            .filter(|_| self.view == "foot")
            .and_then(|bus| self.coop_route(bus))
            .filter(|(_, target)| *target == Target::Player)
            .map(|(owner, _)| owner)
            .filter(|owner| {
                self.net
                    .coop
                    .remote
                    .is_some_and(|(id, epoch, _)| id == *owner && epoch != 0)
            });
        let Some(owner) = owner else {
            self.coop_drop_ticket_blocks();
            return;
        };
        let Some(BusId::Ai(bus))=self.foot_bus() else { self.coop_drop_ticket_blocks();return; };
        let Some(logical_owner)=crate::humans::remote_bus_player(bus) else { self.coop_drop_ticket_blocks();return; };
        let Some(remote) = self
            .net
            .remotes
            .remotes
            .get(&logical_owner)
            .filter(|r| !r.fallback && !r.stand_in)
        else {
            self.coop_drop_ticket_blocks();
            return;
        };
        let v = remote.vehicle();
        let (Some(world), Some(renderer), Some(scene)) = (
            self.world.as_ref(),
            self.renderer.as_ref(),
            self.scene.as_mut(),
        ) else {
            return;
        };
        if world.ticket_pack.trim().is_empty() || v.ty.def.attachments.is_empty() {
            self.coop_drop_ticket_blocks();
            return;
        }
        let pack = omsi_cfg::resolve_path(&world.root, &world.ticket_pack);
        let fresh = self
            .net
            .coop
            .ticket_blocks
            .as_ref()
            .is_none_or(|(id, blocks)| {
                *id != owner || blocks.made_for.0 != v.ty.def.path || blocks.made_for.1 != pack
            });
        if fresh {
            if let Some((_, blocks)) = self.net.coop.ticket_blocks.take() {
                blocks.remove(renderer, scene);
            }
            self.net.coop.ticket_blocks = Some((
                owner,
                crate::money::TicketBlocks::new(world, renderer, scene, v, &pack),
            ));
        }
        if let Some((_, blocks)) = self.net.coop.ticket_blocks.as_ref() {
            blocks.sync(renderer, scene, v);
        }
    }

    /// The conductor's "fecho" is a safe signal to the driver, not a remote
    /// activation of door mechanisms. It is sent through the owner/ACK pipeline.
    pub(crate) fn coop_close_signal(&mut self) {
        let Some((to, epoch, _)) = self.net.coop.remote.filter(|(_, e, _)| *e != 0) else {
            self.coop_notice("Peça autorização de cobrador primeiro.",
                "Ask the driver to authorize you first.",
                "Bitte zunächst als Kassierer autorisieren lassen.");
            return;
        };
        if self.foot_bus().and_then(|b| self.coop_route(b)) != Some((to, Target::Player)) {
            return;
        }
        let a = Action {
            epoch, seq: 1, target: Target::Player, desk: 0, kind: 4,
            part: 0, index: 0, signature: 0, phase: Phase::Tap, x: 0.0, y: 0.0,
        };
        if !self.coop_queue(to, a) {
            self.coop_notice("Não foi possível enviar o sinal.",
                "Could not send the closing signal.",
                "Schließsignal konnte nicht gesendet werden.");
        }
    }

    fn coop_play_close_cue(&self) {
        let Some(audio) = self.sound.audio.as_ref() else { return };
        let path = std::env::current_exe().ok()
            .and_then(|p| p.parent().map(|p| p.join("assets/sounds/coop_turnstile_coin.wav")))
            .filter(|p| p.is_file())
            .unwrap_or_else(|| std::path::PathBuf::from("assets/sounds/coop_turnstile_coin.wav"));
        if let Some(clip) = audio.load_clip(&path) {
            // The user-provided real turnstile coin cue is dynamics-compressed,
            // has extra midrange presence, and is non-spatial so both participants
            // can hear it over their own local engine mix. Priority prevents it
            // being stolen when the bus has many active audio voices.
            audio.play(clip, omsi_audio::VoiceParams {
                gain: 1.08, pitch: 1.0, looping: false, position: None,
                range: 14.0, important: true, ..Default::default()
            });
        } else {
            log::warn!("COOP6 turnstile cue not found: {}", path.display());
        }
    }

    /// Shift+T and the cash panel both use the same acknowledged, owner-validated
    /// cooperative action. The conductor never changes a local duplicate desk.
    pub(crate) fn coop_collect_payment(&mut self) -> bool {
        let Some((to, epoch, _)) = self.net.coop.remote.filter(|(_, e, _)| *e != 0) else {
            return false;
        };
        if self.view != "foot"
            || self.foot_bus().and_then(|b| self.coop_route(b)) != Some((to, Target::Player))
            || epoch == 0
        {
            return false;
        }
        self.coop_cash_action(5, 0);
        true
    }

    pub(crate) fn coop_cash_action(&mut self, kind: u8, index: u32) {
        let Some((to, epoch, _)) = self.net.coop.remote.filter(|(_, e, _)| *e != 0) else {
            return;
        };
        let a = Action {
            epoch,
            seq: 1,
            target: Target::Player,
            desk: self.net.coop.desk.id,
            kind,
            part: 0,
            index,
            signature: 0,
            phase: Phase::Tap,
            x: 0.0,
            y: 0.0,
        };
        self.coop_queue(to, a);
    }
}

/// The same physical bell interaction on a dedicated, headless server. No cashier role
/// is granted on AI buses: their passenger service stays under the native AI simulation.
#[derive(Default)]
pub(crate) struct AiServer {
    receipts: HashMap<u32, Receipt>,
    held: HashMap<u32, Held>,
    bells: Vec<(u32, omsi_sim::people::DoorRequest)>,
}

impl AiServer {
    fn release(&mut self, from: u32, traffic: Option<&mut crate::traffic::Traffic>) {
        let Some(h) = self.held.remove(&from) else {
            return;
        };
        let Target::Ai(id) = h.action.target else {
            return;
        };
        if let Some(c) = traffic.and_then(|t| t.sim.cars.iter_mut().find(|c| c.id == id)) {
            if let Some((event, _)) = mesh_info(&c.vehicle, h.action.part, h.action.index)
                .filter(|(_, s)| *s == h.action.signature)
            {
                c.vehicle.trigger(&format!("{event}_off"));
            }
        }
    }

    pub(crate) fn receive(
        &mut self,
        lan: &mut omsi_net::LanSession,
        traffic: Option<&mut crate::traffic::Traffic>,
        humans: Option<&mut crate::humans::Humans>,
        from: u32,
        wire: &str,
    ) -> bool {
        if !wire.starts_with(omsi_lan::coop::PREFIX) {
            return false;
        }
        let Some(message) = Message::parse(wire) else {
            return true;
        };
        match message {
            Message::Keep(0) => {
                if let Some(h) = self.held.get_mut(&from) {
                    h.seen = Instant::now();
                }
            }
            Message::Action(a) => {
                let cached = self.receipts.get(&from).and_then(|r| r.cached(a.seq));
                if cached == Some(true) {
                    if let Some(h) = self.held.get_mut(&from).filter(|h| h.action.seq == a.seq) {
                        h.seen = Instant::now();
                    }
                }
                let ok = cached.unwrap_or_else(|| self.apply(lan, traffic, humans, from, &a));
                if ok || self.receipts.contains_key(&from) {
                    self.receipts.entry(from).or_default().record(a.seq, ok);
                }
                lan.command(
                    from,
                    &Message::Ack {
                        epoch: a.epoch,
                        seq: a.seq,
                        ok,
                    }
                    .encode(),
                );
            }
            _ => {}
        }
        true
    }

    fn apply(
        &mut self,
        lan: &omsi_net::LanSession,
        mut traffic: Option<&mut crate::traffic::Traffic>,
        humans: Option<&mut crate::humans::Humans>,
        from: u32,
        a: &Action,
    ) -> bool {
        if a.epoch != 0 || a.kind != 0 || lan.role != omsi_net::Role::Host {
            return false;
        }
        let Target::Ai(id) = a.target else {
            return false;
        };
        let Some(w) = lan
            .peers()
            .find(|p| p.pose.id == from)
            .and_then(|p| p.pose.walker)
            .filter(|w| w.aboard.is_none())
        else {
            return false;
        };
        let Some(c) = traffic
            .as_ref()
            .and_then(|t| t.sim.cars.iter().find(|c| c.id == id))
        else {
            return false;
        };
        let v = &c.vehicle;
        let local = v
            .body_rotation()
            .inverse()
            .transform_vector3((DVec3::new(w.x, w.y, w.z) - v.position).as_vec3());
        if local.z.abs() > 5.0
            || !omsi_lan::passenger::inside(
                &omsi_lan::spawn::footprint_of(v, omsi_lan::spawn::BUS_BOX),
                local.to_array(),
            )
        {
            return false;
        }
        let Some((event, _)) =
            mesh_info(v, a.part, a.index).filter(|(e, s)| *s == a.signature && passenger_event(e))
        else {
            return false;
        };
        if a.phase == Phase::Up || a.phase == Phase::Drag {
            if !self.held.get(&from).is_some_and(|h| {
                h.action.target == a.target
                    && h.action.part == a.part
                    && h.action.index == a.index
                    && h.action.signature == a.signature
            }) {
                return false;
            }
        }
        if a.phase == Phase::Up {
            self.release(from, traffic);
            return true;
        }
        if a.phase != Phase::Drag {
            self.release(from, traffic.as_deref_mut());
        }
        let Some(c) = traffic.and_then(|t| t.sim.cars.iter_mut().find(|c| c.id == id)) else {
            return false;
        };
        if a.phase == Phase::Drag {
            c.vehicle.host.mouse = (a.x.clamp(-500.0, 500.0), a.y.clamp(-500.0, 500.0));
            c.vehicle.trigger(&format!("{event}_drag"));
            c.vehicle.host.mouse = (0.0, 0.0);
        } else {
            if !press(&mut c.vehicle, &event) {
                return false;
            }
            if bell(&event) {
                if let Some(h) = humans {
                    let bus = BusId::Ai(id);
                    if h.last_buses
                        .iter()
                        .any(|b| b.id == bus && !b.cabin.exits.is_empty())
                    {
                        let source = 0x8000_0000 | from;
                        self.bells.retain(|(s, _)| *s != source);
                        self.bells.push((
                            source,
                            omsi_sim::people::DoorRequest {
                                bus,
                                exit: true,
                                index: 0,
                            },
                        ));
                    }
                }
            }
        }
        self.held.insert(
            from,
            Held {
                from,
                action: a.clone(),
                seen: Instant::now(),
                control: None,
            },
        );
        true
    }

    pub(crate) fn tick(
        &mut self,
        lan: &omsi_net::LanSession,
        mut traffic: Option<&mut crate::traffic::Traffic>,
        humans: Option<&mut crate::humans::Humans>,
    ) {
        let now = Instant::now();
        let expired: Vec<_> = self
            .held
            .iter()
            .filter(|(id, h)| {
                now.duration_since(h.seen) > Duration::from_secs(1)
                    || !lan.peers().any(|p| p.pose.id == **id)
            })
            .map(|(id, _)| *id)
            .collect();
        for id in expired {
            self.release(id, traffic.as_deref_mut());
        }
        self.receipts
            .retain(|id, _| lan.peers().any(|p| p.pose.id == *id));
        if let Some(h) = humans {
            self.bells.retain(|(source, request)| {
                if h.requested_door_open(*request) {
                    h.clear_manual_door_request(*source);
                    false
                } else {
                    h.set_manual_door_request(*source, *request, 1.0);
                    true
                }
            });
        }
    }
}

#[cfg(test)]
mod tests {
    #[test]
    fn delegated_delivery_is_not_a_success_receipt_and_duplicate_results_are_ignored() {
        use clap::Parser;
        let args=crate::Args::parse_from(["openomsi","--root","."]);
        let mut app=crate::assemble_app(args,Default::default());
        app.net.coop.pending_delegated.insert((2,77,9),std::time::Instant::now());
        assert!(app.net.coop.receipts.get(&(2,77)).is_none());
        app.coop_delegated_result(2,77,9,false);
        assert_eq!(app.net.coop.receipts.get(&(2,77)).unwrap().cached(9),Some(false));
        app.coop_delegated_result(2,77,9,true);
        assert_eq!(app.net.coop.receipts.get(&(2,77)).unwrap().cached(9),Some(false));
    }
    use super::*;
    #[test]
    fn a_visitor_cannot_work_driver_controls_just_because_they_start_with_int() {
        assert!(passenger_event("int_haltewunsch"));
        assert!(bell("campainha"));
        for e in [
            "int_engine_start",
            "int_handbrake",
            "ticket_print",
            "door_front",
            "int_light",
        ] {
            assert!(!passenger_event(e));
        }
        assert!(!bell("int_kinderwagenwunsch"));
    }
}
