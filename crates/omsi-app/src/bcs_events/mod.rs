//! Opt-in, local visual companion for BCS events. Never writes to BCS or its counters.
//! The first scene is a seated passenger fainting (event 4). An ambulance request is
//! an intent only: neither the BCS delay nor completion proves a native vehicle arrived.

mod assets;
mod dispatch;
mod feed;
mod lifecycle;
mod posture;
pub(crate) use posture::slump;

use lifecycle::{Controller, Stage};
use std::sync::atomic::{AtomicU64, Ordering};

static PATIENT_POSE: AtomicU64 = AtomicU64::new(0);

/// Read once before parallel skinning; both the target and blend belong to one frame.
pub(crate) fn patient_pose() -> Option<(u32, f32)> {
    let packed = PATIENT_POSE.load(Ordering::Relaxed);
    let blend = f32::from_bits(packed as u32);
    (blend > 0.0 && blend.is_finite()).then_some(((packed >> 32) as u32, blend))
}

fn publish(patient: Option<u32>, blend: f32) {
    let packed = patient.map_or(0, |id| {
        ((id as u64) << 32) | blend.clamp(0.0, 1.0).to_bits() as u64
    });
    PATIENT_POSE.store(packed, Ordering::Relaxed);
}

#[derive(Default)]
pub(crate) struct Incidents {
    configured: bool,
    source: Option<feed::Feed>,
    controller: Controller,
    models: assets::Loader,
    dispatcher: dispatch::Dispatcher,
    last_notice: Option<(Option<lifecycle::Key>, Stage, Option<u32>)>,
}

impl Drop for Incidents {
    fn drop(&mut self) {
        publish(None, 0.0);
    }
}

impl crate::App {
    pub(crate) fn tick_bcs_events(&mut self, dt: f32) {
        let incidents = &mut self.integrations.bcs_events;
        if !incidents.configured {
            incidents.configured = true;
            let enabled = std::env::var("OPENOMSI_BCS_EVENTS_BETA")
                .map(|v| v == "1")
                .unwrap_or(option_env!("OPENOMSI_BCS_EVENTS_BETA_DEFAULT") == Some("1"));
            if enabled {
                incidents.source = feed::Feed::start();
                incidents.models.start(self.args.root.clone());
                log::info!("BCS incident beta: local capture and response controller enabled");
            }
        }
        if incidents.source.is_none() {
            return;
        }

        // Multiplayer needs an authoritative incident protocol before it may change looks.
        let context = if self.net.lan.is_none() && self.args.server.is_none() {
            self.player
                .as_ref()
                .filter(|p| p.borrowed_from.is_none())
                .map(|p| format!("{:?}|{}|{:?}", self.args.map, p.uid, p.vehicle.ty.def.path))
        } else {
            None
        };
        let riders: Vec<u32> = self
            .session
            .humans
            .as_ref()
            .map(|h| {
                h.people
                    .iter()
                    .filter(|p| p.is_local_seated_passenger(omsi_sim::people::BusId::Player))
                    .map(|p| p.id)
                    .collect()
            })
            .unwrap_or_default();
        let latest = incidents.source.as_ref().and_then(|f| f.latest());
        incidents.controller.step(
            context.as_deref(),
            latest.as_ref(),
            &riders,
            if self.paused { 0.0 } else { dt },
        );
        let state = &incidents.controller;
        let mut transfer = None;
        if let (Some(world), Some(traffic), Some(player)) = (
            self.world.as_ref(),
            self.session.traffic.as_mut(),
            self.player.as_ref(),
        ) {
            transfer = incidents.dispatcher.advance(
                dispatch::Input {
                    context: context.as_deref(),
                    source: latest.as_ref(),
                    key: state.key,
                    stage: state.stage,
                    patient: state.patient,
                    riders: &riders,
                    blend: state.blend,
                    bus: player.vehicle.position,
                    heading: player.vehicle.heading,
                    speed: player.vehicle.physics.velocity_kmh(),
                    dt: if self.paused { 0.0 } else { dt },
                },
                incidents
                    .models
                    .model(omsi_sim::ai_traffic::response::Service::Medical),
                traffic,
                world,
            );
            if let (Some(renderer), Some(scene)) = (self.renderer.as_ref(), self.scene.as_mut()) {
                crate::view_sync::apply_traffic(
                    traffic,
                    &mut self.gfx.sim_view.traffic,
                    world,
                    renderer,
                    scene,
                );
            }
        } else if let Some(traffic) = self.session.traffic.as_mut() {
            incidents.dispatcher.cancel(traffic);
        } else {
            incidents.dispatcher.clear_without_world();
        }
        let pose = incidents.dispatcher.pose().or_else(|| state.patient.map(|id| (id,state.blend)));
        publish(pose.map(|p|p.0),pose.map_or(0.0,|p|p.1));
        if let Some(h)=self.session.humans.as_mut(){
            h.sim.hold_for_responder(pose.filter(|p|p.1>0.0).map(|p|p.0));
        }
        if let Some(id) = transfer {
            if let Some(h) = self.session.humans.as_mut() {
                if h.sim.transfer_to_responder(id) {
                    publish(None, 0.0);
                    log::info!(
                        "BCS response: passenger {id} transferred after physical ambulance arrival and BCS completion"
                    );
                    if let (Some(world), Some(renderer), Some(scene)) = (
                        self.world.as_ref(),
                        self.renderer.as_ref(),
                        self.scene.as_mut(),
                    ) {
                        crate::view_sync::apply_people(
                            h,
                            &mut self.gfx.sim_view.people,
                            world,
                            renderer,
                            scene,
                        );
                    }
                }
            }
        }
        let notice = (state.key, state.stage, state.patient);
        if incidents.last_notice != Some(notice) {
            log::info!(
                "BCS incident beta: {:?}, passenger {:?}, source {:?}",
                state.stage,
                state.patient,
                state.key
            );
            if state.stage == Stage::PatientUnwell && state.patient.is_none() {
                log::warn!("BCS incident beta: medical event received but no eligible seated local passenger is aboard this bus");
            }
            use omsi_plugin::InfoValue::{Num, Text};
            crate::plugins::queue_event(
                &mut self.integrations.plugin_events,
                "bcs_incident",
                vec![
                    Num(if state.key.is_some() { 4.0 } else { 0.0 }),
                    Num(state.key.map_or(0, |k| k.instance) as f64),
                    Text(format!("{:?}", state.stage)),
                    Num(state.patient.map_or(-1.0, |p| p as f64)),
                ],
            );
            incidents.last_notice = Some(notice);
        }
    }
}
