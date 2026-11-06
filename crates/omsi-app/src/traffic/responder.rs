//! Dedicated responders use the real road graph, collision checks and normal AI bodies.
use super::*;
use glam::DVec2;
use omsi_sim::ai_traffic::response::{Service, short_approaches, roadside_stops};
use omsi_sim::people::world::World as _;

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
        let goals = roadside_stops(&self.sim.net, bus, heading, ty.def.ai_veh_type);
        let goal_count = goals.len();
        let mut closest_clear = None;
        let mut blocked_stops = 0;
        let mut approach_count = 0;
        let mut blocked_approaches = 0;
        let radius = ty.half_length().unwrap_or(6.0) as f64 + 2.0;
        for goal in goals.iter().copied().take(128) {
            if self.blocked(&ty, goal.3, goal.4 as f64) { blocked_stops += 1; continue; }
            if closest_clear.is_none() { closest_clear = Some(goal); }
            let mut starts = short_approaches(&self.sim.net, goal.1, goal.2, ty.def.ai_veh_type)
                .into_iter().filter_map(|(route, s, length)| {
                    approach_count += 1;
                    let (at, heading) = self.sim.net.lanes[route[0]].at(s);
                    if (at - bus).length() > 80.0 || self.blocked(&ty, at, heading as f64) { blocked_approaches += 1; return None; }
                    for (k, &lane) in route.iter().enumerate() {
                        let from = if k == 0 { s } else { 0.0 };
                        let end = if k + 1 == route.len() { goal.2 } else { self.sim.net.lanes[lane].length() };
                        let mut check = from + 3.0;
                        while check < end {
                            let (pos, angle) = self.sim.net.lanes[lane].at(check);
                            if self.blocked(&ty, pos, angle as f64) { blocked_approaches += 1; return None; }
                            check += 3.0;
                        }
                    }
                    let hidden = viewer.hides(at, radius, || self.occluded(world, &viewer, at, radius));
                    Some((!hidden, length, route, s))
                }).collect::<Vec<_>>();
            // Prefer an unseen approach, but never send the ambulance hundreds of
            // metres away just to hide its creation during the short BCS call.
            starts.sort_by(|a, b| a.1.total_cmp(&b.1).then(a.0.cmp(&b.0)));
            if let Some((visible, length, route, s)) = starts.into_iter().next() {
                let seed = self.rand();
                let id = self.create_car(world, bus, LaneKind::Street, route[0], s, ty, seed,
                    None, None, Some(6.0_f32.min((2.0 * 3.0 * length).sqrt())), None);
                if self.sim.dispatch_response(id, route, goal.2, service) {
                    log::info!("BCS response: short approach {length:.1} m to a stop {:.1} m from bus, spawn visible {visible}", (goal.3 - bus).length());
                    return Ok(id);
                }
                self.remove_car(id);
                return Err("response route was invalidated".into());
            }
        }
        // A terminal or short cul-de-sac can have a clear stopping place without
        // room for a drive-in. Create the real vehicle stopped on that road;
        // arrival is still confirmed from its actual position and speed.
        if let Some(goal) = closest_clear {
            let seed = self.rand();
            let id = self.create_car(world, bus, LaneKind::Street, goal.1, goal.2, ty, seed,
                None, None, Some(0.0), None);
            if self.sim.dispatch_response(id, vec![goal.1], goal.2, service) {
                log::info!("BCS response: nearby roadside placement {:.1} m from bus; no clear drive-in", (goal.3 - bus).length());
                return Ok(id);
            }
            self.remove_car(id);
            return Err("response roadside placement was invalidated".into());
        }
        // The road is occupied: look for a genuinely wide, clear pavement. This
        // fallback stands still; it never invents a drive across a curb or building.
        if let Some(goal)=goals.first().copied() {
            if let Some((at,angle))=self.incident_pavement(world,&ty,bus) {
                let seed=self.rand();
                let id=self.create_car(world,bus,LaneKind::Street,goal.1,goal.2,ty,seed,None,None,Some(0.0),None);
                if self.sim.dispatch_response(id,vec![goal.1],goal.2,service)&&self.sim.park_response(id,at,angle) {
                    log::info!("BCS response: clear pavement stand {:.1} m from bus",(at-bus).length());
                    return Ok(id);
                }
                self.remove_car(id);
            }
        }
        Err(format!("no clear short connected approach beside the bus (road stops {goal_count}, occupied stops {blocked_stops}, approaches {approach_count}, occupied approaches {blocked_approaches})"))
    }

    fn incident_pavement(&mut self,world:&crate::scene::World,ty:&Arc<VehicleType>,bus:DVec3)->Option<(DVec3,f64)> {
        let data=world.data();
        let bb=ty.def.bounding_box.unwrap_or(DEFAULT_BOX);
        let mut candidates=Vec::new();
        for lane in &self.sim.net.lanes {
            if lane.kind!=LaneKind::Sidewalk || lane.invisible || lane.width<bb[0]+1.0 {continue;}
            let Some((s,_))=lane.nearest_point(bus)else{continue};
            for offset in [-12.0_f32,-8.0,-4.0,0.0,4.0,8.0,12.0] {
                let (at,h)=lane.at((s+offset).clamp(0.0,lane.length()));
                let d=(at-bus).length();
                if (6.0..=32.0).contains(&d)&&(at.z-bus.z).abs()<1.0 {candidates.push((d,at,h as f64));}
            }
        }
        candidates.sort_by(|a,b|a.0.total_cmp(&b.0));
        'candidate:for (_,mut at,h)in candidates.into_iter().take(96) {
            let Some(z)=data.walk_height_near(at.x,at.y,at.z)else{continue};at.z=z;
            if self.blocked(ty,at,h){continue;}
            let mut bounds=omsi_sim::collision::Obb::from_box(bb,at,h);
            bounds.half+=DVec2::splat(0.4);
            // Ignore the supporting floor/curb, but keep posts, shelters and walls.
            bounds.z0=bounds.z0.max(at.z+0.3);
            if data.collision().hit(&bounds).is_some() || data.parked_boxes().iter().any(|b|b.overlaps(&bounds)) {continue;}
            if self.sim.people.iter().any(|(p,_,_)|bounds.overlaps(&omsi_sim::collision::Obb::point(*p+DVec3::Z*0.8,0.55))){continue;}
            for q in bounds.corners().into_iter().chain(std::iter::once(bounds.center)) {
                let Some(floor)=data.walk_height_near(q.x,q.y,z)else{continue 'candidate};
                if (floor-z).abs()>0.22 {continue 'candidate;}
                let Some(foot)=self.sim.net.footing_on_path(q,z,0.35)else{continue 'candidate};
                if self.sim.net.lanes[foot.lane].width<bb[0]+0.5 {continue 'candidate;}
                // A pedestrian crossing is a walking path too: do not call it a curb.
                if self.sim.net.lanes.iter().filter(|l|l.kind==LaneKind::Street).any(|l|{
                    l.nearest_point(q.extend(z)).is_some_and(|(s,_)|{
                        let p=l.at(s).0;(p.z-z).abs()<0.7&&(p.truncate()-q).length()<l.width as f64*0.5+0.2
                    })
                }){continue 'candidate;}
            }
            return Some((at,h));
        }
        None
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
