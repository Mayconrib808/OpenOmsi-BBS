//! A local medical response may finish visually after the BCS popup closes.
//! Source completion is latched; arrival is read only from the actual AI vehicle.
use super::{
    assets::Model,
    lifecycle::{Key, Snapshot, Stage},
};
use glam::DVec3;
use omsi_sim::ai_traffic::response::Service;

struct Medical {
    key: Key,
    context: String,
    patient: u32,
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
        model: Option<Model>,
        traffic: &mut crate::traffic::Traffic,
        world: &crate::scene::World,
    ) -> Option<u32> {
        self.retiring
            .retain(|&id| !traffic.retire_incident_vehicle(world, id));
        if self.medical.is_none()
            && matches!(
                input.stage,
                Stage::AmbulanceRequested | Stage::AwaitingAmbulance
            )
        {
            if let (Some(key), Some(patient), Some(context)) =
                (input.key, input.patient, input.context)
            {
                if self.seen != Some(key) && input.dt > 0.0 {
                    self.seen = Some(key);
                    self.medical = Some(Medical {
                        key,
                        context: context.into(),
                        patient,
                        vehicle: None,
                        bus_at: None,
                        age: 0.0,
                        retry: 0.0,
                        completed: false,
                        arrived: false,
                        blend: input.blend,
                        stalled_reported: false,
                    });
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
            || !input.riders.contains(&scene.patient)
            || moved
            || scene.age > 30.0
        {
            self.cancel(traffic);
            return None;
        }
        if let Some(s) = input.source {
            if s.pid == scene.key.pid
                && s.instance == scene.key.instance
                && s.event == 4
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
            let result = match model {
                Some(model) => traffic
                    .dispatch_incident_vehicle(
                        world,
                        model.ty,
                        input.bus,
                        input.heading,
                        Service::Medical,
                    )
                    .map(|id| {
                        traffic.sim.set_response_signals(id, model.signals);
                        id
                    }),
                None => Err("ambulance model not loaded or installed".into()),
            };
            match result {
                Ok(id) => {
                    scene.vehicle = Some(id);
                    scene.bus_at = Some(input.bus);
                    self.last_problem = None;
                    log::info!(
                        "BCS response: ambulance {id} dispatched for passenger {}",
                        scene.patient
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
                    log::info!("BCS response: ambulance {id} physically on scene");
                }
                if dt > 0.0 && transfer_ready(scene.completed, input.speed) {
                    let patient = scene.patient;
                    self.cancel(traffic);
                    return Some(patient);
                }
            } else if !scene.stalled_reported {
                if let Some(car) = traffic.sim.cars.iter().find(|c| c.id == id && c.stopped >= 2.0) {
                    scene.stalled_reported = true;
                    log::warn!("BCS response: ambulance {id} stopped before arrival: reason {}, bus distance {:.1} m, lane {}, s {:.1}, response {:?}", car.why.0, (car.vehicle.position - input.bus).length(), car.state.lane, car.state.s, car.response);
                }
            }
        }
        None
    }
    pub fn pose(&self) -> Option<(u32, f32)> {
        self.medical.as_ref().map(|s| (s.patient, s.blend))
    }
    pub fn cancel(&mut self, traffic: &mut crate::traffic::Traffic) {
        if let Some(id) = self.medical.take().and_then(|s| s.vehicle) {
            let released = traffic.sim.release_response(id);
            if released && self.retiring.len() < 16 {
                self.retiring.push(id);
            }
            log::info!("BCS response: ambulance {id} released to drive away");
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
