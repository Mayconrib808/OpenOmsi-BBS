//! Dedicated responders use the real road graph, collision checks and normal AI bodies.
use super::*;
use omsi_sim::{ai_traffic::response::Service, traffic::way::lanes_near};

impl Traffic {
    pub(crate) fn dispatch_incident_vehicle(
        &mut self,
        world: &crate::scene::World,
        ty: Arc<VehicleType>,
        bus: DVec3,
        heading: f64,
        service: Service,
    ) -> Result<u64, String> {
        let viewer = self.sim.viewer.ok_or("waiting for camera")?;
        let h = heading.to_radians();
        let forward = DVec3::new(h.sin(), h.cos(), 0.0);
        let desired = bus - forward * 30.0;
        let mut goals = Vec::new();
        for lane in lanes_near(&self.sim.net, desired.truncate(), 150.0) {
            let l = &self.sim.net.lanes[lane];
            if l.kind != LaneKind::Street
                || l.invisible
                || !l.allows(ty.def.ai_veh_type)
                || l.length() < 8.0
            {
                continue;
            }
            let Some((s, distance)) = l.nearest_point(desired) else {
                continue;
            };
            let s = s.clamp(3.0, l.length() - 3.0);
            let (at, lane_heading) = l.at(s);
            let behind = (at - bus).dot(forward);
            let angle = ((lane_heading as f64 - heading + 180.0).rem_euclid(360.0) - 180.0).abs();
            if distance <= 12.0
                && (at.z - bus.z).abs() <= 3.0
                && (-50.0..=-16.0).contains(&behind)
                && angle < 35.0
            {
                goals.push((distance, lane, s, at, lane_heading));
            }
        }
        goals.sort_by(|a, b| a.0.total_cmp(&b.0));
        let goal = goals
            .into_iter()
            .take(16)
            .find(|g| !self.blocked(&ty, g.3, g.4 as f64))
            .ok_or("no clear road stop behind bus")?;
        let radius = ty.half_length().unwrap_or(6.0) as f64 + 2.0;
        let mut starts = Vec::new();
        for lane in lanes_near(&self.sim.net, bus.truncate(), 500.0) {
            let l = &self.sim.net.lanes[lane];
            if l.kind != LaneKind::Street
                || l.invisible
                || !l.allows(ty.def.ai_veh_type)
                || l.length() < 10.0
            {
                continue;
            }
            // A long spline can pass the bus while its start lies kilometres away.
            let near = l.nearest_point(bus).map_or(0.0, |x| x.0);
            let mut samples = vec![
                4.0,
                l.length() - 4.0,
                near - 400.0,
                near - 240.0,
                near + 240.0,
                near + 400.0,
            ];
            samples.retain(|s| *s >= 4.0 && *s <= l.length() - 4.0);
            samples.sort_by(f32::total_cmp);
            samples.dedup_by(|a, b| (*a - *b).abs() < 1.0);
            for s in samples {
                let (at, heading) = l.at(s);
                let distance = (at - bus).length();
                if (160.0..=480.0).contains(&distance) {
                    starts.push((distance, lane, s, at, heading));
                }
            }
        }
        starts.sort_by(|a, b| a.0.total_cmp(&b.0));
        for start in starts.into_iter().take(64) {
            if !viewer.hides(start.3, radius, || {
                self.occluded(world, &viewer, start.3, radius)
            }) || self.blocked(&ty, start.3, start.4 as f64)
            {
                continue;
            }
            let Some(route) = self.sim.net.shortest_path(start.1, goal.1) else {
                continue;
            };
            if route.len() > 512
                || route
                    .iter()
                    .any(|&i| !self.sim.net.lanes[i].allows(ty.def.ai_veh_type))
            {
                continue;
            }
            let length: f32 = route
                .iter()
                .map(|&i| self.sim.net.lanes[i].length())
                .sum::<f32>()
                - start.2
                - (self.sim.net.lanes[goal.1].length() - goal.2);
            if length > 2000.0 || (route.len() == 1 && goal.2 <= start.2) {
                continue;
            }
            let seed = self.rand();
            let id = self.create_car(
                world,
                bus,
                LaneKind::Street,
                start.1,
                start.2,
                ty,
                seed,
                None,
                None,
                Some(0.0),
                None,
            );
            if self.sim.dispatch_response(id, route, goal.2, service) {
                return Ok(id);
            }
            self.remove_car(id);
            return Err("response route was invalidated".into());
        }
        Err("no hidden connected spawn with enough free space".into())
    }

    /// Normal off-screen retirement after driving away; never pop a vehicle beside the bus.
    pub(crate) fn retire_incident_vehicle(&mut self, world: &crate::scene::World, id: u64) -> bool {
        let Some(car) = self.sim.cars.iter().find(|c| c.id == id) else {
            return true;
        };
        if !car
            .response
            .as_ref()
            .is_some_and(|d| d.phase == omsi_sim::ai_traffic::response::Phase::Departing)
        {
            return true;
        }
        let Some(viewer) = self.sim.viewer else {
            return false;
        };
        let at = car.vehicle.position;
        let radius = car.vehicle.ty.half_length().unwrap_or(6.0) as f64 + 2.0;
        if viewer.hides(at, radius, || self.occluded(world, &viewer, at, radius)) {
            self.remove_car(id)
        } else {
            false
        }
    }
}
