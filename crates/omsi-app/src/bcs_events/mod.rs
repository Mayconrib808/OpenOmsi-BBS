//! Opt-in, local visual companion for BCS events. Never writes to BCS or its counters.
//! The first scene is a seated passenger fainting (event 4). An ambulance request is
//! an intent only: neither the BCS delay nor completion proves a native vehicle arrived.

mod feed;
mod lifecycle;
mod posture;
pub(crate) use posture::slump;

use std::sync::atomic::{AtomicU64, Ordering};
use lifecycle::{Controller, Stage};

static PATIENT_POSE: AtomicU64 = AtomicU64::new(0);

/// Read once before parallel skinning; both the target and blend belong to one frame.
pub(crate) fn patient_pose() -> Option<(u32, f32)> {
    let packed = PATIENT_POSE.load(Ordering::Relaxed);
    let blend = f32::from_bits(packed as u32);
    (blend > 0.0 && blend.is_finite()).then_some(((packed >> 32) as u32, blend))
}

fn publish(patient: Option<u32>, blend: f32) {
    let packed = patient.map_or(0, |id| ((id as u64) << 32) | blend.clamp(0.0, 1.0).to_bits() as u64);
    PATIENT_POSE.store(packed, Ordering::Relaxed);
}

#[derive(Default)]
pub(crate) struct Incidents {
    configured: bool,
    source: Option<feed::Feed>,
    controller: Controller,
    last_notice: Option<(Option<lifecycle::Key>, Stage, Option<u32>)>,
}

impl Drop for Incidents {
    fn drop(&mut self) { publish(None, 0.0); }
}

impl crate::App {
    pub(crate) fn tick_bcs_events(&mut self, dt: f32) {
        let incidents = &mut self.integrations.bcs_events;
        if !incidents.configured {
            incidents.configured = true;
            if std::env::var("OPENOMSI_BCS_EVENTS_BETA").is_ok_and(|v| v == "1") {
                incidents.source = feed::Feed::start();
                log::info!("BCS incident beta: local capture reader enabled; ambulance rendering is not installed yet");
            }
        }
        if incidents.source.is_none() { return; }

        // Multiplayer needs an authoritative incident protocol before it may change looks.
        let context = if self.net.lan.is_none() && self.args.server.is_none() {
            self.player.as_ref().filter(|p| p.borrowed_from.is_none()).map(|p|
                format!("{:?}|{}|{:?}", self.args.map, p.uid, p.vehicle.ty.def.path))
        } else { None };
        let riders: Vec<u32> = self.session.humans.as_ref().map(|h| h.people.iter()
            .filter(|p| !p.remote && p.inside(omsi_sim::people::BusId::Player)
                && matches!(p.activity, omsi_sim::human::Activity::Sit))
            .map(|p| p.id).collect()).unwrap_or_default();
        let latest = incidents.source.as_ref().and_then(|f| f.latest());
        incidents.controller.step(context.as_deref(), latest.as_ref(), &riders, if self.paused { 0.0 } else { dt });
        let state = &incidents.controller;
        publish(state.patient, state.blend);
        let notice = (state.key, state.stage, state.patient);
        if incidents.last_notice != Some(notice) {
            log::info!("BCS incident beta: {:?}, passenger {:?}, source {:?}", state.stage, state.patient, state.key);
            use omsi_plugin::InfoValue::{Num, Text};
            crate::plugins::queue_event(&mut self.integrations.plugin_events, "bcs_incident", vec![
                Num(if state.key.is_some() { 4.0 } else { 0.0 }),
                Num(state.key.map_or(0, |k| k.instance) as f64),
                Text(format!("{:?}", state.stage)),
                Num(state.patient.map_or(-1.0, |p| p as f64)),
            ]);
            incidents.last_notice = Some(notice);
        }
    }
}
