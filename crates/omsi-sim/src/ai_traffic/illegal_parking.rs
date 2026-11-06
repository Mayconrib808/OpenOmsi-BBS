//! #2372: deterministic state machine for optional temporary roadside obstructions.
//! The real traffic integration must supply collision-cleared placement and a valid
//! hazard-light script before enabling it. No normal parked map objects are replaced.
use super::random::draw;
use super::{LaneKind, TrafficSim};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HornReaction { Wait, Nudge, Leave }

#[derive(Debug, Clone, Copy)]
pub struct TemporaryPark {
    pub until: f32,
    /// Duration counts after the physical stop, not while approaching it.
    pub duration: f32,
    pub parked: bool,
    pub response_at: Option<f32>,
    pub honks: u8,
}

/// Only accept temporary stops on wide, unobstructed street stretches far from
/// junctions and crossings. This is a necessary condition, not full clearance.
pub fn may_consider_stop(street: bool, scheduled: bool, near_junction: f32,
    clear_ahead: f32, speed: f32) -> bool {
    street && !scheduled && near_junction.is_finite() && clear_ahead.is_finite()
        && speed.is_finite() && speed >= 1.0
        && near_junction >= 35.0 && clear_ahead >= 25.0
}

/// Frequency is a percentage of *eligible events*, not a random draw each frame.
pub fn eligible_event(seed: u64, event_number: u64, frequency_percent: f32) -> bool {
    frequency_percent.is_finite() && frequency_percent > 0.0
        && draw(seed, event_number) < frequency_percent.clamp(0.0, 100.0) / 100.0
}

impl TemporaryPark {
    pub fn new(now: f32, seed: u64, event_number: u64, single_lane: bool) -> Option<Self> {
        if !now.is_finite() || now < 0.0 { return None; }
        // A blockage of a single-lane street is strictly capped at 30 seconds.
        let duration = if single_lane { 10.0 + 20.0 * draw(seed, event_number + 1) }
                       else { 15.0 + 30.0 * draw(seed, event_number + 1) };
        Some(Self { until: now + duration, duration, parked: true, response_at: None, honks: 0 })
    }

    /// Start the deadline only when the body is actually stationary at the target.
    pub fn activate(&mut self, now: f32) {
        if self.parked || !now.is_finite() { return; }
        self.parked = true;
        self.until = now + self.duration;
    }

    /// Honking creates one delayed response (3-10 s), not an immediate teleport.
    pub fn hear_horn(&mut self, now: f32, seed: u64) {
        if !self.parked || !now.is_finite() || now >= self.until { return; }
        self.honks = self.honks.saturating_add(1);
        if self.response_at.is_none() {
            self.response_at = Some(now + 3.0 + 7.0 * draw(seed, 200 + self.honks as u64));
        }
    }

    pub fn reaction(&mut self, now: f32, seed: u64) -> HornReaction {
        if !now.is_finite() || !self.parked { return HornReaction::Wait; }
        if now >= self.until { return HornReaction::Leave; }
        if self.response_at.is_none_or(|at| now < at) { return HornReaction::Wait; }
        self.response_at = None;
        // Repeated honks eventually clear the obstruction; the first can nudge.
        if self.honks >= 3 || draw(seed, 900 + self.honks as u64) > 0.45 {
            HornReaction::Leave
        } else {
            HornReaction::Nudge
        }
    }
}

/// A physical random-traffic stop. The car stays in the normal traffic list,
/// so following vehicles and LAN clients see the obstruction and its hazards.
/// Chance is sampled once on entry to an eligible lane, never every frame.
impl TrafficSim {
    pub fn plan_temporary_illegal_stop(&mut self, i: usize, ts: &super::tick::TickScene) -> Option<f32> {
        if !self.illegal_parking_percent.is_finite() || self.illegal_parking_percent <= 0.0 {
            self.cars[i].temporary_park = None;
            return None;
        }
        let now = self.time;
        if let Some((mut parking, mut at)) = self.cars[i].temporary_park {
            if self.cars[i].state.s >= at - 1.7 && self.cars[i].state.speed < 0.45 {
                parking.activate(now);
                crate::feature_audit::record(crate::feature_audit::Role::Host,"illegal_parking",crate::feature_audit::ResultKind::Completed,"physically_parked",
                    &format!("vehicle={} lane={} at_s={:.1} elapsed={:.1}",self.cars[i].id,self.cars[i].state.lane,at,now));
            }
            match parking.reaction(now, self.cars[i].seed) {
                HornReaction::Leave => {
                    crate::feature_audit::record(crate::feature_audit::Role::Host,"illegal_parking",crate::feature_audit::ResultKind::Completed,"parking_ended",
                        &format!("vehicle={} lane={} at_s={:.1}",self.cars[i].id,self.cars[i].state.lane,at));
                    self.cars[i].temporary_park = None;
                    return None;
                }
                HornReaction::Nudge => { at += 1.4; }
                HornReaction::Wait => {}
            }
            let car = &mut self.cars[i];
            car.temporary_park = Some((parking, at));
            // The parked car is held with normal brakes; no teleportation or
            // removed collision body, even while it nudges forward after a horn.
            return Some((at - car.state.s + car.state.front + 0.4).max(car.state.front));
        }
        let car = &self.cars[i];
        let st = &car.state;
        let lane_id = st.lane;
        if car.parking_sampled_lane == Some(lane_id) || car.is_bus() || car.is_rail()
            || car.response.is_some() || car.park.is_some() || car.passing.is_some()
            || car.bus_lane_pass.is_some() || st.change.is_some() || !st.route.is_empty()
            || st.s < 3.0 || st.s > 12.0 || st.speed < 2.0
        { return None; }
        let lane = &self.net.lanes[lane_id];
        if lane.kind != LaneKind::Street || !self.net.crossings[lane_id].is_empty() {
            return None;
        }
        // Draw only once on this lane even when a prior test was ineligible.
        let (seed, target_s, speed, single_lane) = (car.seed, st.s + 25.0, st.speed,
            lane.left.is_none() && lane.right.is_none());
        self.cars[i].parking_sampled_lane = Some(lane_id);
        let reason = if !may_consider_stop(true,false,lane.length()-target_s,30.0,speed) {
            Some("unsafe_road_segment")
        } else if !eligible_event(seed,lane_id as u64,self.illegal_parking_percent) {
            Some("frequency_draw_not_selected")
        } else if self.cars.iter().filter(|c|c.temporary_park.is_some()).count()>=3 {
            Some("max_three_obstructions")
        } else if !self.lane_clear(i,lane_id,target_s,10.0,15.0,&ts.by_lane) {
            Some("collision_clearance_failed")
        } else if self.cars.iter().any(|c|c.bus.as_ref().is_some_and(|b|
            b.stops.iter().any(|stop|c.state.route.get(stop.ri)==Some(&lane_id)
                && (stop.s-target_s).abs()<35.0))) {
            Some("bus_stop_nearby")
        } else {None};
        if let Some(reason)=reason {
            crate::feature_audit::record(crate::feature_audit::Role::Host,"illegal_parking",
                if reason=="frequency_draw_not_selected" {crate::feature_audit::ResultKind::NoOpportunity}
                else {crate::feature_audit::ResultKind::Blocked},
                reason,&format!("vehicle={} lane={} planned_s={:.1} frequency_percent={:.1}",
                    self.cars[i].id,lane_id,target_s,self.illegal_parking_percent));
            return None;
        }
        let mut state = TemporaryPark::new(now, seed, lane_id as u64, single_lane)?;
        state.parked = false;
        self.cars[i].temporary_park = Some((state, target_s));
        crate::feature_audit::record(crate::feature_audit::Role::Host,"illegal_parking",
            crate::feature_audit::ResultKind::Executed,"parking_scheduled",
            &format!("vehicle={} lane={} target_s={:.1} frequency_percent={:.1} sim_t={now:.1}",
                self.cars[i].id,lane_id,target_s,self.illegal_parking_percent));
        log::info!("t={now:.1}: traffic AI car {} schedules short temporary obstruction on lane {} at {:.1} m", self.cars[i].id, lane_id, target_s);
        Some(25.0 + self.cars[i].state.front + 0.4)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn candidate_stops_do_not_block_intersections_or_scheduled_buses() {
        assert!(may_consider_stop(true, false, 60.0, 30.0, 8.0));
        assert!(!may_consider_stop(true, false, 9.0, 30.0, 8.0));
        assert!(!may_consider_stop(true, true, 60.0, 30.0, 8.0));
        assert!(!may_consider_stop(false, false, 60.0, 30.0, 8.0));
        assert!(!may_consider_stop(true, false, f32::NAN, 30.0, 8.0));
    }
    #[test]
    fn single_lane_clears_in_thirty_seconds_even_without_horn() {
        let mut p = TemporaryPark::new(100.0, 22, 4, true).unwrap();
        assert!(p.until <= 130.0);
        assert_eq!(p.reaction(100.0, 22), HornReaction::Wait);
        assert_eq!(p.reaction(130.0, 22), HornReaction::Leave);
    }
    #[test]
    fn horn_is_delayed_and_repeated_requests_cannot_extend_deadline() {
        let mut p = TemporaryPark::new(100.0, 22, 4, false).unwrap();
        let until = p.until;
        p.hear_horn(101.0, 22);
        let response = p.response_at.unwrap();
        assert!((104.0..=111.0).contains(&response));
        p.hear_horn(102.0, 22);
        assert_eq!(p.response_at, Some(response));
        assert_eq!(p.until, until);
        assert_eq!(p.reaction(102.0, 22), HornReaction::Wait);
        p.hear_horn(103.0, 22);
        assert_eq!(p.reaction(response, 22), HornReaction::Leave);
    }
    #[test]
    fn deadline_starts_when_car_really_stops_not_when_it_plans_to_stop() {
        let mut p = TemporaryPark::new(100.0, 22, 4, true).unwrap();
        p.parked = false;
        p.hear_horn(110.0, 22);
        assert_eq!(p.honks, 0);
        assert_eq!(p.reaction(300.0, 22), HornReaction::Wait);
        p.activate(300.0);
        assert!(p.until <= 330.0);
        assert_eq!(p.reaction(300.0, 22), HornReaction::Wait);
        assert_eq!(p.reaction(330.0, 22), HornReaction::Leave);
    }
    #[test]
    fn zero_frequency_never_creates_an_event() {
        assert!(!(0..1000).any(|n| eligible_event(17, n, 0.0)));
        assert!(!eligible_event(17, 1, f32::NAN));
    }
}
