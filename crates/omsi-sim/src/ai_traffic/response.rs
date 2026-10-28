//! A dispatched emergency vehicle's explicit destination and physical stop.
//! Nothing here decides that a BCS event is complete or alters the player's passengers.
use super::*;

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Service {
    Medical,
    Police,
    Fire,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Phase {
    EnRoute,
    OnScene,
    Departing,
}

#[derive(Clone, Debug, Default)]
pub struct Signals {
    pub en_route: Vec<(String, f32)>,
    pub on_scene: Vec<(String, f32)>,
    pub departing: Vec<(String, f32)>,
}

#[derive(Clone, Debug)]
pub struct Drive {
    pub service: Service,
    pub goal_lane: usize,
    pub goal_s: f32,
    pub goal_index: usize,
    pub phase: Phase,
    pub signals: Arc<Signals>,
}

impl TrafficSim {
    /// Assign a route to a dedicated, non-timetable vehicle. Reject mismatched routes.
    pub fn dispatch_response(
        &mut self,
        id: u64,
        route: Vec<usize>,
        goal_s: f32,
        service: Service,
    ) -> bool {
        let Some(i) = self.cars.iter().position(|c| c.id == id) else {
            return false;
        };
        let car = &self.cars[i];
        let Some(&goal_lane) = route.last() else {
            return false;
        };
        if car.is_bus()
            || route.first().copied() != Some(car.state.lane)
            || !goal_s.is_finite()
            || goal_s < 0.0
            || route.iter().any(|&l| {
                self.net
                    .lanes
                    .get(l)
                    .is_none_or(|l| l.kind != LaneKind::Street || !l.allows(car.state.veh_type))
            })
            || route
                .windows(2)
                .any(|pair| !self.net.lanes[pair[0]].next.contains(&pair[1]))
            || goal_s > self.net.lanes[goal_lane].length()
            || (route.len() == 1 && goal_s < car.state.s)
        {
            return false;
        }
        let car = &mut self.cars[i];
        car.response = Some(Drive {
            service,
            goal_lane,
            goal_s,
            goal_index: route.len() - 1,
            phase: Phase::EnRoute,
            signals: Default::default(),
        });
        car.state.route = route;
        car.state.route_index = 0;
        car.state.planned_next = None;
        car.state.ahead.clear();
        car.state.plan_next(&self.net);
        true
    }

    /// An origin stop is converted to the existing front-bumper stop convention.
    /// Collision avoidance, crossings, leads and lights still contribute their own stops.
    pub(super) fn response_stop(&mut self, i: usize) -> Option<f32> {
        let drive = self.cars[i].response.as_ref()?.clone();
        if drive.phase == Phase::Departing {
            return None;
        }
        let car = &mut self.cars[i];
        if car.state.route.get(drive.goal_index).copied() != Some(drive.goal_lane)
            || car.state.route_index > drive.goal_index
        {
            return Some(car.state.front);
        }
        let distance = car
            .state
            .route_distance(&self.net, drive.goal_index, drive.goal_s);
        if car.state.lane == drive.goal_lane {
            let goal = self.net.lanes[drive.goal_lane].at(drive.goal_s).0;
            let arrived = physically_arrived(car.vehicle.position, goal, car.state.speed);
            if let Some(response) = car.response.as_mut() {
                response.phase = if arrived {
                    Phase::OnScene
                } else {
                    Phase::EnRoute
                };
            }
        }
        Some(distance.max(0.0) + car.state.front)
    }

    /// Drive away normally. The instance retains its role so a permanently-active model
    /// cannot keep claiming emergency priority after this dispatch has finished.
    pub fn release_response(&mut self, id: u64) -> bool {
        let Some(car) = self.cars.iter_mut().find(|c| c.id == id) else {
            return false;
        };
        let Some(response) = car.response.as_mut() else {
            return false;
        };
        response.phase = Phase::Departing;
        car.state.route.clear();
        car.state.route_index = 0;
        car.state.ahead.clear();
        car.state.planned_next = None;
        car.state.plan_next(&self.net);
        true
    }

    /// Recheck the body after its step; a collision must not leave a stale arrival flag.
    pub fn response_on_scene(&self, id: u64) -> bool {
        let Some(car) = self.cars.iter().find(|c| c.id == id) else {
            return false;
        };
        let Some(drive) = &car.response else {
            return false;
        };
        drive.phase == Phase::OnScene
            && car.state.lane == drive.goal_lane
            && self.net.lanes.get(drive.goal_lane).is_some_and(|lane| {
                physically_arrived(
                    car.vehicle.position,
                    lane.at(drive.goal_s).0,
                    car.state.speed,
                )
            })
    }

    /// Model-specific signals are applied after its OMSI script, including during departure.
    pub fn set_response_signals(&mut self, id: u64, signals: Arc<Signals>) -> bool {
        let Some(drive) = self
            .cars
            .iter_mut()
            .find(|c| c.id == id)
            .and_then(|c| c.response.as_mut())
        else {
            return false;
        };
        drive.signals = signals;
        true
    }

    pub(super) fn apply_response_signals(&mut self) {
        for car in &mut self.cars {
            let Some(drive) = &car.response else { continue };
            let variables = match drive.phase {
                Phase::EnRoute => &drive.signals.en_route,
                Phase::OnScene => &drive.signals.on_scene,
                Phase::Departing => &drive.signals.departing,
            };
            car.vehicle.set_engine_var(
                "TrafficPriority",
                if drive.phase == Phase::EnRoute {
                    1.0
                } else {
                    0.0
                },
            );
            for (name, value) in variables {
                car.vehicle.set_engine_var(name, *value);
            }
        }
    }
}

fn physically_arrived(at: DVec3, goal: DVec3, speed: f32) -> bool {
    speed.is_finite()
        && speed.abs() < 0.3
        && (at - goal).truncate().length() <= 1.5
        && (at.z - goal.z).abs() <= 2.0
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn arrival_requires_position_height_and_standstill() {
        let goal = DVec3::new(100.0, 200.0, 3.0);
        assert!(!physically_arrived(goal, goal, 10.0));
        assert!(!physically_arrived(goal + DVec3::X * 8.0, goal, 0.0));
        assert!(!physically_arrived(goal + DVec3::Z * 5.0, goal, 0.0));
        assert!(physically_arrived(goal + DVec3::X * 0.5, goal, 0.1));
    }
    #[test]
    fn a_response_does_not_reclassify_the_model_or_other_instances() {
        use crate::ai_traffic::scenario_tests::{Fixture, add_car, street, traffic};
        let f = Fixture::new();
        let mut net = Network::default();
        net.lanes.push(street(DVec3::ZERO, 0.0, 300.0));
        net.link(1.5);
        let mut t = traffic(&f, net);
        let responder = add_car(&mut t, &f, 0, 10.0, 1, Some(0.0));
        let ordinary = add_car(&mut t, &f, 0, 40.0, 2, Some(0.0));
        assert!(t.dispatch_response(responder, vec![0], 100.0, Service::Police));
        assert!(super::super::emergency::active(
            t.cars.iter().find(|c| c.id == responder).unwrap()
        ));
        assert!(!super::super::emergency::active(
            t.cars.iter().find(|c| c.id == ordinary).unwrap()
        ));
        assert!(t.release_response(responder));
        assert!(!super::super::emergency::active(
            t.cars.iter().find(|c| c.id == responder).unwrap()
        ));
    }
    #[test]
    fn drives_to_a_real_stop_and_drives_away_when_released() {
        use crate::ai_traffic::scenario_tests::{Fixture, add_car, street, traffic};
        let f = Fixture::new();
        let mut net = Network::default();
        net.lanes.push(street(DVec3::ZERO, 0.0, 500.0));
        net.link(1.5);
        let mut t = traffic(&f, net);
        let id = add_car(&mut t, &f, 0, 20.0, 1, Some(0.0));
        assert!(t.dispatch_response(id, vec![0], 120.0, Service::Medical));
        for _ in 0..3600 {
            t.tick(super::super::fixed_step::SIM_DT, None);
        }
        let car = t.cars.iter().find(|c| c.id == id).unwrap();
        assert!(car.state.s <= 121.5, "overshot scene: {}", car.state.s);
        assert!(
            t.response_on_scene(id),
            "arrival did not occur at actual vehicle stop"
        );
        assert!(t.release_response(id));
        for _ in 0..900 {
            t.tick(super::super::fixed_step::SIM_DT, None);
        }
        assert!(t.cars.iter().find(|c| c.id == id).unwrap().state.s > 125.0);
    }
}
