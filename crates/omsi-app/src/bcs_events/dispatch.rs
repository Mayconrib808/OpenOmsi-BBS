//! A local medical response may finish visually after the BCS popup closes.
//! Source completion is latched; arrival is read only from the actual AI vehicle.
use super::{
    assets::Loader,
    lifecycle::{Key, Snapshot, Stage},
};
use glam::DVec3;
use omsi_sim::ai_traffic::response::Service;

struct Medical {
    key: Key,
    context: String,
    patient: Option<u32>,
    event: u32,
    service: Service,
    vehicle: Option<u64>,
    bus_at: Option<DVec3>,
    age: f32,
    retry: f32,
    completed: bool,
    arrived: bool,
    blend: f32,
    stalled_reported: bool,
}
#[derive(Default)]
pub(super) struct Dispatcher {
    medical: Option<Medical>,
    seen: Option<Key>,
    retiring: Vec<u64>,
    last_problem: Option<String>,
}
pub(super) struct Input<'a> {
    pub context: Option<&'a str>,
    pub source: Option<&'a Snapshot>,
    pub key: Option<Key>,
    pub stage: Stage,
    pub patient: Option<u32>,
    pub riders: &'a [u32],
    pub blend: f32,
    pub bus: DVec3,
    pub heading: f64,
    pub speed: f32,
    pub dt: f32,
}

impl Dispatcher {
    pub fn advance(
        &mut self,
        input: Input<'_>,
        models: &Loader,
        traffic: &mut crate::traffic::Traffic,
        world: &crate::scene::World,
    ) -> Option<u32> {
        self.retiring
            .retain(|&id| !traffic.retire_incident_vehicle(world, id));
        let intent = input.source.filter(|s| !s.completed).and_then(|s| {
            let service = service_for(s.event)?;
            let requested = match service {Service::Medical=>3,Service::Police=>2,Service::Fire=>5};
            if s.service != requested {return None;}
            // Police and fire scenes belong to the event, not to whichever rider
            // happens to occupy the first seat: that rider can move or alight.
            let patient = if service == Service::Medical {Some(input.patient?)} else {None};
            Some((Key{pid:s.pid,instance:s.instance},s.event,service,patient))
        });
        if self.medical.is_none() {
            if let (Some((key,event,service,patient)),Some(context))=(intent,input.context) {
                if self.seen != Some(key) && input.dt > 0.0 {
                    self.seen = Some(key);
                    log::info!("BCS response: accepted {:?} incident event={} instance={} patient={:?}",service,event,key.instance,patient);
                    self.medical = Some(Medical { key,context:context.into(),patient,event,service,
                        vehicle:None,bus_at:None,age:0.0,retry:0.0,completed:false,arrived:false,
                        blend:input.blend,stalled_reported:false });
                }
            }
        }
        let scene = self.medical.as_mut()?;
        let source_ok = input.source.is_some_and(|s| s.pid == scene.key.pid);
        let moved = scene
            .bus_at
            .is_some_and(|at| (at - input.bus).length() > 8.0);
        if input.context != Some(scene.context.as_str())
            || !source_ok
            || scene.patient.is_some_and(|p| !input.riders.contains(&p))
            || moved
            || scene.age > 180.0
        {
            log::warn!("BCS response: {:?} event={} cancelled context_ok={} source_ok={} rider_ok={} moved={} age={:.1}s",scene.service,scene.event,input.context==Some(scene.context.as_str()),source_ok,scene.patient.is_none_or(|p|input.riders.contains(&p)),moved,scene.age);
            self.cancel(traffic);
            return None;
        }
        if let Some(s) = input.source {
            if s.pid == scene.key.pid
                && s.instance == scene.key.instance
                && s.event == scene.event
                && s.completed
            {
                scene.completed = true;
            }
        }
        let dt = input.dt.max(0.0).min(0.25);
        scene.age += dt;
        scene.retry -= dt;
        scene.blend = (scene.blend + dt).min(1.0);
        if scene.vehicle.is_none() && scene.retry <= 0.0 && input.speed.abs() < 1.0 && dt > 0.0 {
            scene.retry = 0.5;
            let result = match models.model(scene.service) {
                Some(model) => traffic
                    .dispatch_incident_vehicle(
                        world,
                        model.ty,
                        input.bus,
                        input.heading,
                        scene.service,
                    )
                    .map(|id| {
                        traffic.sim.set_response_signals(id, model.signals);
                        id
                    }),
                None => Err(format!("{:?} responder model not loaded or installed",scene.service)),
            };
            match result {
                Ok(id) => {
                    scene.vehicle = Some(id);
                    scene.bus_at = Some(input.bus);
                    self.last_problem = None;
                    log::info!(
                        "BCS response: {:?} vehicle {id} dispatched for passenger {:?}",
                        scene.service, scene.patient
                    );
                }
                Err(problem) => {
                    if self.last_problem.as_ref() != Some(&problem) {
                        log::warn!("BCS response pending: {problem}");
                        self.last_problem = Some(problem);
                    }
                }
            }
        }
        if let Some(id) = scene.vehicle {
            if !traffic.sim.cars.iter().any(|c| c.id == id) {
                self.cancel(traffic);
                return None;
            }
            if traffic.sim.response_on_scene(id) {
                if !scene.arrived {
                    scene.arrived = true;
                    log::info!("BCS response: {:?} vehicle {id} physically on scene", scene.service);
                }
                if dt > 0.0 && transfer_ready(scene.completed, input.speed) {
                    let patient = scene.patient;
                    self.cancel(traffic);
                    return patient;
                }
            } else if !scene.stalled_reported {
                if let Some(car) = traffic.sim.cars.iter().find(|c| c.id == id && c.stopped >= 2.0) {
                    scene.stalled_reported = true;
                    log::warn!("BCS response: {:?} vehicle {id} stopped before arrival: reason {}, bus distance {:.1} m, lane {}, s {:.1}, response {:?}", scene.service,car.why.0, (car.vehicle.position - input.bus).length(), car.state.lane, car.state.s, car.response);
                }
            }
        }
        None
    }
    pub fn pose(&self) -> Option<(u32, f32)> {
        self.medical.as_ref().filter(|s|s.service==Service::Medical).and_then(|s|s.patient.map(|p|(p,s.blend)))
    }
    pub fn cancel(&mut self, traffic: &mut crate::traffic::Traffic) {
        if let Some(id) = self.medical.take().and_then(|s| s.vehicle) {
            let released = traffic.sim.release_response(id);
            if released && self.retiring.len() < 16 {
                self.retiring.push(id);
            }
            log::info!("BCS response: vehicle {id} released to drive away");
        }
    }
    pub fn clear_without_world(&mut self) {
        self.medical = None;
        self.retiring.clear();
    }
}

fn transfer_ready(bcs_complete: bool, bus_speed: f32) -> bool {
    bcs_complete && bus_speed.is_finite() && bus_speed.abs() < 1.0
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn bcs_delay_alone_does_not_transfer_a_passenger() {
        assert!(!transfer_ready(false, 0.0));
        assert!(!transfer_ready(true, 30.0));
        assert!(transfer_ready(true, 0.0));
    }
}

/// Visual services only. Radio contest and passenger count have no native vehicle.
fn service_for(event:u32)->Option<Service>{
    match event {1..=4=>Some(Service::Medical),6|7=>Some(Service::Police),30..=32=>Some(Service::Fire),_=>None}
}
