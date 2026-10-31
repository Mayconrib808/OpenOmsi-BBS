//! Opt-in, local visual companion for BCS events. Never writes to BCS or its counters.
//! The first scene is a seated passenger fainting (event 4). An ambulance request is
//! an intent only: neither the BCS delay nor completion proves a native vehicle arrived.

mod assets;
mod network;
pub(crate) use network::HostResponses;
mod names;
mod inspector;
mod effects;
mod dispatch;
mod feed;
mod lifecycle;
mod posture;
pub(crate) use posture::{slump,reaction};

use lifecycle::{Controller, Stage};
use std::sync::atomic::{AtomicU64, Ordering};

static PATIENT_POSE: AtomicU64 = AtomicU64::new(0);
static ACTOR_POSE: AtomicU64 = AtomicU64::new(0);
static REMOTE_POSES:std::sync::Mutex<Vec<(u32,u32,f32,f32)>>=std::sync::Mutex::new(Vec::new());
pub(crate) fn remote_poses()->Vec<(u32,u32,f32,f32)>{REMOTE_POSES.lock().map(|p|p.clone()).unwrap_or_default()}

pub(crate) fn actor_pose()->Option<(u32,u32,f32,f32)>{
    let p=ACTOR_POSE.load(Ordering::Relaxed);
    (p&255>0).then_some(((p>>32)as u32,((p>>8)&255)as u32,(p&255)as f32/255.0,((p>>16)&65535)as f32/65535.0*std::f32::consts::TAU))
}

/// Only the standalone visual harness may keep its seeded test passengers aboard
/// before a scene. A real BCS session needs its plugins; LAN and unseeded runs are
/// excluded even if the environment switch is inherited accidentally.
pub(crate) fn local_visual_test(args: &crate::cli::Args) -> bool {
    args.riders > 0
        && args.server.is_none()
        && args.lan_host.is_none()
        && args.lan_join.is_none()
        && omsi_cfg::flags::OMSI_NO_PLUGINS.is_set()
        && std::env::var("OPENOMSI_BCS_EVENTS_VISUAL_TEST").as_deref() == Ok("1")
}

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
    options: omsi_cfg::optional_features::OptionalFeatures,
    actor_key: Option<(String,lifecycle::Key)>,
    actor_person: Option<u32>, actor_blend:f32, actor_time:f32,

    effects: effects::Effects,
    network:network::Network,
    remote_effects:std::collections::HashMap<u32,effects::Effects>,
    inspector: inspector::InspectorActor,
    source: Option<feed::Feed>,
    controller: Controller,
    models: assets::Loader,
    dispatcher: dispatch::Dispatcher,
    last_notice: Option<(Option<lifecycle::Key>, Stage, Option<u32>)>,
}

impl Drop for Incidents {
    fn drop(&mut self) {
        publish(None, 0.0);
        ACTOR_POSE.store(0,Ordering::Relaxed);
        if let Ok(mut p)=REMOTE_POSES.lock(){p.clear();}
    }
}

impl crate::App {
    pub(crate) fn tick_bcs_events(&mut self, dt: f32) {
        let incidents = &mut self.integrations.bcs_events;
        if !incidents.configured {
            incidents.configured = true;
            // These options are captured for this session. Both settings interfaces
            // explicitly say a game restart is required to change them.
            incidents.options = self.settings.optional_features;
            incidents.network.host = HostResponses::new(incidents.options.bcs_event_scenes);
            if incidents.options.needs_bcs_capture() {
                incidents.source = feed::Feed::start();
                if incidents.options.bcs_event_scenes && incidents.source.is_some() {
                    incidents.models.start(self.args.root.clone());
                }
            }
        }
        if incidents.source.is_none() {
            return;
        }

        // Each capture belongs to its original bus; shared responders are host-authoritative.
        let context = if self.args.server.is_none() {
            self.player
                .as_ref()
                .filter(|p| p.borrowed_from.is_none())
                .map(|p| format!("{:?}|{}|{:?}", self.args.map, p.uid, p.vehicle.ty.def.path))
        } else {
            None
        };
        let latest = incidents.source.as_ref().and_then(|feed| feed.latest());
        if incidents.options.bcs_inspectors && !self.paused {
            if let (Some(humans), Some(player), Some(world)) =
                (self.session.humans.as_mut(), self.player.as_ref(), self.world.as_ref()) {
                incidents.inspector.update(context.as_deref(), latest.as_ref().and_then(|s| s.inspector.as_ref()),
                    humans, &player.vehicle, world);
            }
        }
        // The ordinary people synchronization draws inspector changes on the next
        // frame; do not perform a second whole-population synchronization here.
        if !incidents.options.bcs_event_scenes { return; }
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
        let actor_source=latest.as_ref().filter(|s|matches!(s.event,6|7)&&!s.completed);
        let actor_key=context.as_ref().zip(actor_source).map(|(c,s)|(c.clone(),lifecycle::Key{pid:s.pid,instance:s.instance}));
        if incidents.actor_key!=actor_key {
            incidents.actor_key=actor_key;
            incidents.actor_person=actor_source.and_then(|_|riders.iter().copied().min());
        }
        if incidents.actor_person.is_some_and(|id|!riders.contains(&id)){incidents.actor_person=None;}
        let delta=if self.paused{0.0}else{dt.max(0.0).min(0.25)};
        let want=if incidents.actor_key.is_some()&&incidents.actor_person.is_some(){1.0}else{0.0};
        incidents.actor_blend+=(want-incidents.actor_blend).clamp(-delta,delta);
        incidents.actor_time+=delta;
        let actor=incidents.actor_person.zip(actor_source).map_or(0,|(id,s)|{
            ((id as u64)<<32)|(((incidents.actor_time.rem_euclid(std::f32::consts::TAU)/std::f32::consts::TAU*65535.0)as u64)<<16)
                |((s.event as u64)<<8)|(incidents.actor_blend.clamp(0.0,1.0)*255.0)as u64
        });
        ACTOR_POSE.store(actor,Ordering::Relaxed);

        if let (Some(world),Some(renderer),Some(scene),Some(player))=(self.world.as_ref(),self.renderer.as_ref(),self.scene.as_mut(),self.player.as_ref()) {
            incidents.effects.update(context.as_deref(),latest.as_ref(),player.vehicle.position,player.vehicle.heading,
                if self.paused{0.0}else{dt},world,renderer,scene);
        }
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
                    context: if self.net.lan.as_ref().is_some_and(|l|l.role==omsi_net::Role::Client){None}else{context.as_deref()},
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
                &incidents.models,
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
            if context.is_some() && state.stage == Stage::Idle && local_visual_test(&self.args) {
                h.sim.hold_seated_test_passengers();
            }
        }
        if let Some(id) = transfer {
            if let Some(h) = self.session.humans.as_mut() {
                if h.sim.transfer_to_responder(id) {
                    publish(None, 0.0);
        ACTOR_POSE.store(0,Ordering::Relaxed);
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
                "BCS incident beta: {:?}, passenger {:?}, source {:?}, capture {:?}, bus {:?}",
                state.stage,
                state.patient,
                state.key,
                latest.as_ref().map(|s| (s.pid, s.instance)),
                context.as_deref()
            );
            if state.stage == Stage::PatientUnwell && state.patient.is_none() {
                log::warn!("BCS incident beta: medical event received but no eligible seated local passenger is aboard this bus");
            }
            use omsi_plugin::InfoValue::{Num, Text};
            crate::plugins::queue_event(
                &mut self.integrations.plugin_events,
                "bcs_incident",
                vec![
                    Num(if state.key.is_some() { latest.as_ref().map_or(0.0,|s|s.event as f64) } else { 0.0 }),
                    Num(state.key.map_or(0, |k| k.instance) as f64),
                    Text(format!("{:?}", state.stage)),
                    Num(state.patient.map_or(-1.0, |p| p as f64)),
                ],
            );
            incidents.last_notice = Some(notice);
        }
        if let Some(lan)=self.net.lan.as_mut(){
            let selected=pose.map(|p|p.0).or(incidents.actor_person);
            let actor=self.session.humans.as_ref().and_then(|h|h.people.iter().find(|p|Some(p.id)==selected)).and_then(|p|{
                if let omsi_sim::people::Place::Bus(omsi_sim::people::BusId::Player,local)=p.place{Some(local)}else{None}
            });
            incidents.network.local(lan,context.as_ref().and(latest.as_ref()),selected,actor,
                pose.map_or(incidents.actor_blend,|p|p.1),incidents.actor_time,delta);
            if let (Some(traffic),Some(world))=(self.session.traffic.as_mut(),self.world.as_ref()){
                incidents.network.host.tick(lan,traffic,world,delta);
            }
            if let Some((_,id))=incidents.network.returned.take(){
                if let Some(h)=self.session.humans.as_mut(){
                    if h.sim.transfer_to_responder(id){publish(None,0.0);}
                }
            }
        }
        let mut remote_poses=Vec::new();
        let visuals=incidents.network.visuals().map(|(id,v)|(id,v.clone())).collect::<Vec<_>>();
        let mut active=std::collections::HashSet::new();
        if let (Some(world),Some(renderer),Some(scene))=(self.world.as_ref(),self.renderer.as_ref(),self.scene.as_mut()){
            for (owner,visual)in visuals{
                if visual.heard.elapsed()>std::time::Duration::from_secs(8){continue;}
                let Some(remote)=self.net.remotes.remotes.get(&owner)else{continue};
                let vehicle=remote.vehicle();active.insert(owner);
                incidents.remote_effects.entry(owner).or_default().update(Some(&format!("{}:{owner}",self.net.lan.as_ref().map_or(0,|l|l.session))),
                    Some(&visual.source),vehicle.position,vehicle.heading,delta,world,renderer,scene);
                if let (Some(local),Some(h))=(visual.actor,self.session.humans.as_ref()){
                    if let Some(person)=h.people.iter().filter(|p|p.remote).find(|p|matches!(p.place,
                        omsi_sim::people::Place::Bus(omsi_sim::people::BusId::Ai(bus),at)
                        if bus==crate::humans::remote_bus_id(owner)&&(at-local).length()<0.2)){
                        remote_poses.push((person.id,visual.source.event,visual.blend,visual.phase));
                    }
                }
            }
            incidents.remote_effects.retain(|owner,e|{
                if active.contains(owner){true}else{e.update(None,None,glam::DVec3::ZERO,0.0,0.0,world,renderer,scene);false}
            });
        }
        if let Ok(mut p)=REMOTE_POSES.lock(){*p=remote_poses;}
    }
    pub(crate) fn receive_bcs_event(&mut self,from:u32,text:&str)->bool{
        if !self.integrations.bcs_events.options.bcs_event_scenes {
            return text.starts_with("bcs222_req ") || text.starts_with("bcs222_view ") || text.starts_with("bcs222_done ");
        }
        let Some(lan)=self.net.lan.as_ref()else{return false};
        self.integrations.bcs_events.network.receive(lan,from,text)
    }
}

pub(crate) fn append_visuals(incidents:&Incidents,scene:&mut omsi_render::Scene){incidents.effects.sprites(scene);for e in incidents.remote_effects.values(){e.sprites(scene);}}
