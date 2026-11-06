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
    /// A validated pavement stand. It stays parked until out-of-sight retirement.
    pub parked: Option<(DVec3, f64)>,
}

/// Nearby road stops for a bus that may be parked across, beside or away from
/// the AI lane. Bus heading is a preference, never a requirement for access.
pub fn roadside_stops(net: &Network, bus: DVec3, heading: f64, veh_type: i32) -> Vec<(f64, usize, f32, DVec3, f32)> {
    let h = heading.to_radians();
    let forward = DVec3::new(h.sin(), h.cos(), 0.0);
    let mut stops = Vec::new();
    let lanes = if net.grid.is_empty() { (0..net.lanes.len()).collect() }
        else { crate::traffic::way::lanes_near(net, bus.truncate(), 80.0) };
    for lane in lanes {
        let l = &net.lanes[lane];
        if l.kind != LaneKind::Street || l.invisible || !l.allows(veh_type) || l.length() < 1.5 { continue; }
        let Some((nearest, _)) = l.nearest_point(bus) else { continue };
        let mut samples = [-24.0, -18.0, -12.0, -8.0, -4.0, 0.0, 4.0, 8.0, 12.0, 18.0, 24.0]
            .into_iter().map(|offset| (nearest + offset).clamp(1.0, l.length() - 0.1)).collect::<Vec<_>>();
        // Preserve the road point used by the earlier working dispatcher too.
        // It is a candidate, without the former heading/behind restrictions.
        if l.length() >= 8.0 {
            if let Some((s, _)) = l.nearest_point(bus - forward * 30.0) {
                samples.push(s.clamp(3.0, l.length() - 3.0));
            }
        }
        samples.sort_by(f32::total_cmp);
        samples.dedup_by(|a, b| (*a - *b).abs() < 0.1);
        for s in samples {
            let (at, road_heading) = l.at(s);
            let distance = (at - bus).truncate().length();
            if distance > 60.0 || (at.z - bus.z).abs() > 3.0 { continue; }
            let behind = (at - bus).dot(forward);
            let angle = ((road_heading as f64 - heading + 180.0).rem_euclid(360.0) - 180.0).abs();
            let score = distance + behind.max(0.0) * 0.05 + angle * 0.01;
            stops.push((score, lane, s, at, road_heading));
        }
    }
    stops.sort_by(|a, b| a.0.total_cmp(&b.0));
    stops
}

/// A short, connected approach to the scene. Follow predecessors so a nearby
/// spawn never needs a long loop around the map to reach the selected lane.
pub fn short_approaches(net: &Network, goal: usize, goal_s: f32, veh_type: i32) -> Vec<(Vec<usize>, f32, f32)> {
    let mut found = Vec::new();
    for distance in [10.0_f32, 14.0, 18.0] {
        let mut pending = vec![(goal, goal_s, distance, vec![goal])];
        let mut visited = 0;
        while let Some((lane, s, remaining, route)) = pending.pop() {
            visited += 1;
            if visited > 128 { break; }
            let l = &net.lanes[lane];
            if l.kind != LaneKind::Street || l.invisible || !l.allows(veh_type) { continue; }
            let start = s - remaining;
            if start >= 0.0 && start <= l.length() {
                found.push((route, start, distance));
            } else if remaining > s && route.len() < 64 {
                for &prev in &net.prev[lane] {
                    if route.contains(&prev) || !net.lanes[prev].next.contains(&lane) { continue; }
                    let mut before = vec![prev];
                    before.extend_from_slice(&route);
                    pending.push((prev, net.lanes[prev].length(), remaining - s, before));
                }
            }
        }
    }
    found
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
            parked: None,
        });
        car.state.route = route;
        car.state.route_index = 0;
        car.state.planned_next = None;
        car.state.ahead.clear();
        car.state.plan_next(&self.net);
        true
    }

    /// Park a responder only after the app checked the whole footprint against
    /// scenery, pedestrians, vehicles and the pavement's height/width.
    pub fn park_response(&mut self, id:u64, at:DVec3, heading:f64)->bool {
        if !at.is_finite() || !heading.is_finite() { return false; }
        let Some(car)=self.cars.iter_mut().find(|c|c.id==id) else{return false};
        let Some(drive)=car.response.as_mut()else{return false};
        drive.parked=Some((at,heading));drive.phase=Phase::OnScene;
        car.state.speed=0.0;
        let a=heading.to_radians();let f=DVec3::new(a.sin(),a.cos(),0.0);
        car.body.place(&|d|at+f*d as f64,car.vehicle.ground.as_ref().map(|g|g.as_ref()as &dyn Fn(f64,f64)->Option<f64>),car.vehicle.contact.as_deref(),0.0);
        car.body.apply(&mut car.vehicle);
        true
    }

    /// An origin stop is converted to the existing front-bumper stop convention.
    /// Collision avoidance, crossings, leads and lights still contribute their own stops.
    pub(super) fn response_stop(&mut self, i: usize) -> Option<f32> {
        let drive = self.cars[i].response.as_ref()?.clone();
        if drive.parked.is_some() { return Some(self.cars[i].state.front); }
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
        if response.parked.is_some() { return true; }
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
        if let Some((at, _)) = drive.parked {
            return drive.phase == Phase::OnScene && physically_arrived(car.vehicle.position, at, car.state.speed);
        }
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
    fn unindexed_long_road_keeps_the_previously_reachable_stop() {
        use crate::ai_traffic::scenario_tests::{Fixture, street};
        let f = Fixture::new();
        let mut net = Network::default();
        net.lanes.push(street(DVec3::new(0.0, -200.0, 0.0), 0.0, 400.0));
        net.link(1.5);
        let stops = roadside_stops(&net, DVec3::ZERO, 0.0, f.ty.def.ai_veh_type);
        assert!(stops.iter().any(|s| (s.3.y + 30.0).abs() < 0.1));
    }
    #[test]
    fn parked_bus_can_be_served_from_a_perpendicular_nearby_road() {
        use crate::ai_traffic::scenario_tests::{Fixture, street};
        let f = Fixture::new();
        let bus = DVec3::new(502.4, 735.7, 8.9);
        let mut net = Network::default();
        net.lanes.push(street(bus + DVec3::new(-30.0, 10.0, 0.0), 90.0, 60.0));
        net.link(1.5);
        net.build_grid();
        let stops = roadside_stops(&net, bus, 20.0, f.ty.def.ai_veh_type);
        assert!(!stops.is_empty(), "a road beside the parked bus was rejected by bus heading");
        assert!(stops.iter().all(|s| (s.3 - bus).truncate().length() <= 60.0));
        assert!(stops.iter().any(|s| !short_approaches(&net, s.1, s.2, f.ty.def.ai_veh_type).is_empty()));
    }
    #[test]
    fn short_approach_can_cross_many_small_road_segments() {
        use crate::ai_traffic::scenario_tests::{Fixture, street};
        let f = Fixture::new();
        let mut net = Network::default();
        for i in 0..16 { net.lanes.push(street(DVec3::new(0.0, i as f64 * 2.0, 0.0), 0.0, 2.0)); }
        net.link(1.5);
        let approaches = short_approaches(&net, 15, 1.0, f.ty.def.ai_veh_type);
        assert!(!approaches.is_empty(), "short connected splines cannot require a 2 m interior margin");
        for (route, s, length) in approaches {
            let distance = route.iter().map(|&l| net.lanes[l].length()).sum::<f32>() - s - 1.0;
            assert!((distance - length).abs() < 0.01);
        }
    }
    #[test]
    fn short_connected_approach_arrives_in_seconds_without_a_map_detour() {
        use crate::ai_traffic::scenario_tests::{Fixture, add_car, street, traffic};
        let f = Fixture::new();
        let mut net = Network::default();
        net.lanes.push(street(DVec3::ZERO, 0.0, 40.0));
        net.lanes.push(street(DVec3::new(0.0, 40.0, 0.0), 0.0, 60.0));
        net.link(1.5);
        let (route, start, length) = short_approaches(&net, 1, 6.0, f.ty.def.ai_veh_type).remove(0);
        assert_eq!(route, vec![0, 1]);
        assert_eq!(length, 10.0);
        let mut t = traffic(&f, net);
        let id = add_car(&mut t, &f, route[0], start, 1, Some(6.0));
        assert!(t.dispatch_response(id, route, 6.0, Service::Medical));
        for _ in 0..(6.0 / super::super::fixed_step::SIM_DT) as usize {
            t.tick(super::super::fixed_step::SIM_DT, None);
            if t.response_on_scene(id) { return; }
        }
        let car = t.cars.iter().find(|c| c.id == id).unwrap();
        panic!("short response failed to arrive: position {:?}, speed {}, reason {:?}", car.vehicle.position, car.state.speed, car.why);
    }
    #[test]
    fn pavement_stand_keeps_its_real_pose_and_does_not_jump_to_road_on_release() {
        use crate::ai_traffic::scenario_tests::{Fixture,add_car,street,traffic};
        let f=Fixture::new();let mut net=Network::default();
        net.lanes.push(street(DVec3::ZERO,0.0,300.0));net.link(1.5);
        let mut t=traffic(&f,net);let id=add_car(&mut t,&f,0,20.0,1,Some(0.0));
        assert!(t.dispatch_response(id,vec![0],20.0,Service::Fire));
        let at=DVec3::new(8.0,20.0,0.0);
        assert!(t.park_response(id,at,0.0));
        for _ in 0..300 {t.tick(super::super::fixed_step::SIM_DT,None);}
        assert!(t.response_on_scene(id));
        assert!((t.cars.iter().find(|c|c.id==id).unwrap().vehicle.position-at).truncate().length()<0.1);
        assert!(t.release_response(id));
        for _ in 0..300 {t.tick(super::super::fixed_step::SIM_DT,None);}
        assert!(!t.response_on_scene(id));
        assert!((t.cars.iter().find(|c|c.id==id).unwrap().vehicle.position-at).truncate().length()<0.1);
    }
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
