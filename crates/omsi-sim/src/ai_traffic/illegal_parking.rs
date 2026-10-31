//! #2372: deterministic state machine for optional temporary roadside obstructions.
//! The real traffic integration must supply collision-cleared placement and a valid
//! hazard-light script before enabling it. No normal parked map objects are replaced.
use super::horn::draw;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum HornReaction { Wait, Nudge, Leave }

#[derive(Debug, Clone, Copy)]
pub struct TemporaryPark {
    pub until: f32,
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
        Some(Self { until: now + duration, response_at: None, honks: 0 })
    }

    /// Honking creates one delayed response (3-10 s), not an immediate teleport.
    pub fn hear_horn(&mut self, now: f32, seed: u64) {
        if !now.is_finite() || now >= self.until { return; }
        self.honks = self.honks.saturating_add(1);
        if self.response_at.is_none() {
            self.response_at = Some(now + 3.0 + 7.0 * draw(seed, 200 + self.honks as u64));
        }
    }

    pub fn reaction(&mut self, now: f32, seed: u64) -> HornReaction {
        if !now.is_finite() { return HornReaction::Wait; }
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
    fn zero_frequency_never_creates_an_event() {
        assert!(!(0..1000).any(|n| eligible_event(17, n, 0.0)));
        assert!(!eligible_event(17, 1, f32::NAN));
    }
}
