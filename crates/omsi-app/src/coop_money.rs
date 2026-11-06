//! Read-only physical cash mirrors. Every collection goes to the original passenger desk.
use crate::{App, humans::BusId, money};
use glam::{DVec3, Mat4, Vec3};
use omsi_lan::coop::{CashPiece, CashView};
use std::time::{Duration, Instant};

pub(crate) struct CashRender {
    bus: BusId,
    view: CashView,
    money: money::Money,
}

impl App {
    pub(crate) fn coop_cash_view(&self) -> CashView {
        if let Some(cash) = self.drive_cash_view() { return cash.clone(); }
        CashView {
            desk: self.coop_desk(0).id,
            pieces: self.session.humans.as_ref().and_then(|h| h.sim.money.as_ref())
                .map(|m| m.placed.iter().map(|(_, local, coin, change, parent)| CashPiece {
                    coin: *coin as u32, local: local.to_cols_array(), change: *change,
                    parent: parent.clone(),
                }).collect()).unwrap_or_default(),
        }
    }

    pub(crate) fn coop_money_tick(&mut self) {
        let now = Instant::now();
        if self.net.coop.cash_tick.is_some_and(|t| now.duration_since(t) < Duration::from_millis(200)) { return; }
        self.net.coop.cash_tick = Some(now);
        let targets: Vec<_> = self.coop_money_grant().into_iter()
            .chain(self.drive_cash_recipient()).collect();
        let cash = self.coop_cash_view();
        self.net.coop.cash_sent.retain(|peer, _| targets.iter().any(|(p, _)| p == peer));
        for (peer, token) in targets {
            if self.net.lan.as_ref().is_none_or(|l| l.handover_pending(peer) >= 6) { continue; }
            let changed = self.net.coop.cash_sent.get(&peer).is_none_or(|(e, at, last)| {
                *e != token || last != &cash || now.duration_since(*at) > Duration::from_secs(2)
            });
            if changed {
                self.drive_send(peer, omsi_lan::vehicle_transfer::Message::CashView { token, cash: cash.clone() });
                self.net.coop.cash_sent.insert(peer, (token, now, cash.clone()));
            }
        }
    }

    pub(crate) fn coop_drop_money(&mut self) {
        if let Some(cash) = self.net.coop.cash_render.take() {
            if let (Some(r), Some(scene)) = (self.renderer.as_ref(), self.scene.as_mut()) {
                for (instance, ..) in cash.money.placed { r.remove_instance(scene, instance); }
                for instance in cash.money.hidden { r.remove_instance(scene, instance); }
            }
        }
    }

    pub(crate) fn coop_sync_money(&mut self) {
        let borrowed = self.net.coop.drive.visitor_active();
        let bus = self.foot_bus().or_else(|| borrowed.then_some(BusId::Player));
        let cash = if borrowed && bus == Some(BusId::Player) {
            self.drive_cash_view().cloned()
        } else if let (Some((peer, epoch, _)), Some(bus)) = (self.net.coop.remote, bus) {
            // The passive original owner already draws the real cash, never a second copy.
            if bus == BusId::Player { None } else {
                self.net.coop.cash_view.as_ref().filter(|(p, e, _)| *p == peer && *e == epoch)
                    .map(|(_, _, cash)| cash.clone())
            }
        } else { None };
        let (Some(bus), Some(cash)) = (bus, cash) else { self.coop_drop_money(); return; };
        let valid_bus = self.coop_vehicle(bus).is_some();
        if !valid_bus { self.coop_drop_money(); return; }
        let fresh = self.net.coop.cash_render.as_ref().is_none_or(|r| r.bus != bus || r.view != cash);
        if fresh {
            if self.net.coop.cash_render.as_ref().is_some_and(|r| r.bus != bus) {
                self.coop_drop_money();
            }
            let (Some(world), Some(r), Some(scene)) = (self.world.as_ref(), self.renderer.as_ref(), self.scene.as_mut()) else { return; };
            // Keep currency meshes/materials across fares. Only instances change;
            // rebuilding GPU assets on every cash update would accumulate them.
            let mut money = if let Some(mut old) = self.net.coop.cash_render.take() {
                for (instance, ..) in old.money.placed.drain(..) { r.remove_instance(scene, instance); }
                for instance in old.money.hidden.drain(..) { r.remove_instance(scene, instance); }
                old.money
            } else { money::Money::new(&world.root, &world.global.money_system) };
            let count = money.currency.as_ref().map_or(0, |c| c.coins.len() + c.bills.len());
            for piece in &cash.pieces {
                let coin = piece.coin as usize;
                if coin >= count { continue; }
                let before = money.placed.len();
                money::place(&mut money, world, r, scene, &[coin], Vec3::ZERO, [0.0; 2], piece.change, piece.parent.as_deref());
                if money.placed.len() > before {
                    money.placed.last_mut().unwrap().1 = Mat4::from_cols_array(&piece.local);
                }
            }
            self.net.coop.cash_render = Some(CashRender { bus, view: cash, money });
        }
        // Borrow disjoint app fields: the vehicle is read, only its money instances move.
        let vehicle = match bus {
            BusId::Player => self.player.as_ref().map(|p| &p.vehicle),
            BusId::Ai(id) => crate::humans::remote_bus_player(id)
                .and_then(|p| self.net.remotes.remotes.get(&p)).map(|r| r.vehicle()),
        };
        if let (Some(cash), Some(r), Some(scene), Some(v)) = (
            self.net.coop.cash_render.as_mut(), self.renderer.as_ref(), self.scene.as_mut(), vehicle,
        ) { money::sync(&mut cash.money, r, scene, v); }
    }

    pub(crate) fn coop_payment_hit(&self, bus: BusId, origin: DVec3, direction: Vec3) -> bool {
        let Some(v) = self.coop_vehicle(bus) else { return false; };
        let money = if bus == BusId::Player && !self.net.coop.drive.visitor_active() {
            self.session.humans.as_ref().and_then(|h| h.sim.money.as_ref())
        } else {
            self.net.coop.cash_render.as_ref().filter(|r| r.bus == bus).map(|r| &r.money)
        };
        money.is_some_and(|m| money::money_hit(m, origin, direction, v)
            .is_some_and(|k| !m.placed[k].3))
    }

    /// The avatar simulation may run a step behind the rendered remote chassis. Carry
    /// its eyes by the same full body transform before drawing this frame.
    pub(crate) fn coop_follow_live_cabin(&mut self) {
        if self.view != "foot" || self.net.coop.camera.is_some() { return; }
        let Some(f) = self.session.on_foot.as_ref().filter(|f| f.settle <= 0.0 && f.cam == crate::on_foot::FootCam::First) else { return; };
        let bus = f.seat.map(|s| s.0).or(f.inside.map(|s| s.0));
        let Some(bus) = bus else { return; };
        let (Some(h), Some(v), Some(_cam)) = (self.session.humans.as_ref(), self.coop_vehicle(bus), self.camera.as_ref()) else { return; };
        let Some((position, rotation)) = h.cabin_frame(bus) else { return; };
        let old = crate::coop_view::BodyPose { position, rotation };
        let new = crate::coop_view::BodyPose::of(v);
        let previous = self.net.coop.drawn.as_ref().filter(|(b, _)| *b == bus).map(|(_, p)| *p);
        if let Some(cam) = self.camera.as_mut() {
            crate::coop_view::carry_camera(cam, old, new, previous);
        }
    }
}
