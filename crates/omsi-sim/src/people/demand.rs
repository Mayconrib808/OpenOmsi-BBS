//! Optional #1438: people arrive to a stop over time, independently of a
//! scheduled departure. Passenger objects are still limited to nearby stops;
//! their backlog persists while a stop is outside a player's streaming radius.
//!
//! The host sim owns this accumulator. LAN clients mirror real people only.

/// Low-cost, once-per-minute diagnostics. These are counters of actual work,
/// not merely option flags; no per-frame or per-passenger INFO log is emitted.
#[derive(Clone, Debug)]
pub struct DemandAudit {
    pub last_report_at: f64,
    pub last_seat_report_at: f64,
    pub last_mirror_claim_report_at: f64,
    pub seeded_stops: u64,
    pub new_queue_units: f64,
    pub materialized: u64,
    pub spawn_refused: u64,
    pub refused_pool_full: u64,
    pub refused_spots_full: u64,
    pub refused_spawn: u64,
    pub retries_suppressed: u64,
    pub last_refused_stop: Option<i64>,
    pub last_refused_reason: Option<&'static str>,
    pub patience_left: u64,
    pub last_stop: Option<i64>,
    pub clothing_evaluated: u64,
    pub clothing_tagged_selected: u64,
    pub conversation_attempts: u64,
    pub conversation_without_pairs: u64,
    pub conversation_without_clips: u64,
    pub conversation_lines: u64,
}
impl Default for DemandAudit {
    fn default() -> Self {
        Self { last_report_at: -60.0, last_seat_report_at: -60.0,
            last_mirror_claim_report_at: -30.0, seeded_stops: 0,
            new_queue_units: 0.0, materialized: 0, spawn_refused: 0,
            refused_pool_full: 0, refused_spots_full: 0, refused_spawn: 0,
            retries_suppressed: 0, last_refused_stop: None, last_refused_reason: None,
            patience_left: 0, last_stop: None, clothing_evaluated: 0,
            clothing_tagged_selected: 0, conversation_attempts: 0,
            conversation_without_pairs: 0, conversation_without_clips: 0,
            conversation_lines: 0 }
    }
}

impl DemandAudit {
    pub fn record_refusal(&mut self, stop: i64, reason: &'static str) {
        self.last_refused_stop = Some(stop);
        self.last_refused_reason = Some(reason);
        match reason {
            "human_pool_full" => self.refused_pool_full += 1,
            "waiting_spots_full" => self.refused_spots_full += 1,
            _ => self.refused_spawn += 1,
        }
    }
}

#[derive(Clone, Copy, Debug)]
pub struct StopDemand {
    pub waiting: f32,
    pub minute: i64,
    pub last_arrival: f64,
    /// Next moment at which a refused spawn can be retried. Prevent frame floods.
    pub retry_after: f64,
}

impl Default for StopDemand {
    fn default() -> Self {
        Self { waiting: 0.0, minute: i64::MIN, last_arrival: f64::NEG_INFINITY,
            retry_after: f64::NEG_INFINITY }
    }
}

/// Stable, deterministic variation per stop and minute: does not consume the
/// global passenger RNG or diverge when someone changes camera viewpoint.
pub fn demand_variation(stop: i64, minute: i64) -> f32 {
    let mut x = (stop as u64).wrapping_mul(0x9E37_79B9_7F4A_7C15)
        ^ (minute as u64).wrapping_mul(0xBF58_476D_1CE4_E5B9);
    x ^= x >> 30;
    x = x.wrapping_mul(0xBF58_476D_1CE4_E5B9);
    x ^= x >> 27;
    x = x.wrapping_mul(0x94D0_49BB_1331_11EB);
    x ^= x >> 31;
    0.70 + ((x & 0xFFFF) as f32 / 65535.0) * 0.60
}

/// The map's mean cube value is a stock of people per roughly six minutes,
/// not a new group to spawn every frame or on every bus arrival.
pub fn arrivals_per_minute(map_mean: f32, density: f32, variation: f32) -> f32 {
    if !map_mean.is_finite() || !density.is_finite() || !variation.is_finite() {
        return 0.0;
    }
    (map_mean.max(0.0) * density.clamp(0.0, 3.0) * variation.clamp(0.0, 2.0) / 6.0)
        .clamp(0.0, 12.0)
}

impl StopDemand {
    /// Keep an off-screen queue, but never allocate more people than the stop has
    /// real waiting places. This function is O(1), independently of trip count.
    pub fn advance(&mut self, stop: i64, now: f64, dt: f32, mean: f32, density: f32, capacity: usize) {
        if !dt.is_finite() || dt <= 0.0 || !now.is_finite() { return; }
        let minute = (now / 60.0).floor() as i64;
        if self.minute == i64::MIN {
            // When a map first loads, some people were already waiting. This
            // one-time seed is not tied to any bus departure or camera reload.
            self.waiting = (mean.max(0.0) * density.clamp(0.0, 3.0)
                * demand_variation(stop, minute))
                .clamp(0.0, capacity.min(100) as f32);
        }
        self.minute = minute;
        let rate = arrivals_per_minute(mean, density, demand_variation(stop, minute));
        self.waiting = (self.waiting + rate * (dt.min(5.0) / 60.0))
            .clamp(0.0, capacity.min(100) as f32);
    }

    pub fn can_retry(&self, now: f64) -> bool { now.is_finite() && now >= self.retry_after }

    pub fn retry_later(&mut self, now: f64, succeeded: bool) {
        if now.is_finite() {
            self.retry_after = now + if succeeded { 2.5 } else { 10.0 };
        }
    }

    pub fn admitted(&mut self) {
        self.waiting = (self.waiting - 1.0).max(0.0);
    }
}

/// People who have waited much longer than the headway may leave. Deterministic
/// patience by person ID, and no removal during boarding or onboard travel.
pub fn patience_seconds(person: u32) -> f64 {
    480.0 + (person.wrapping_mul(101) % 481) as f64
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn arrivals_are_independent_of_trip_timing_and_bounded() {
        let mut d = StopDemand::default();
        for minute in 0..600 {
            d.advance(43, minute as f64, 1.0, 18.0, 1.0, 12);
        }
        assert!(d.waiting > 0.0 && d.waiting <= 12.0);
        let x = d.waiting;
        d.admitted();
        assert!(d.waiting < x);
        assert_eq!(demand_variation(43, 10), demand_variation(43, 10));
        assert_ne!(demand_variation(43, 10), demand_variation(43, 11));
    }
    #[test]
    fn initial_queue_is_seeded_once_not_refilled_by_bus_arrivals() {
        let mut d=StopDemand::default();
        d.advance(5, 60.0, 1.0, 8.0, 1.0, 15);
        let initial=d.waiting;
        assert!(initial > 0.0);
        d.admitted();
        let after=d.waiting;
        d.advance(5, 60.1, 0.1, 8.0, 1.0, 15);
        assert!(d.waiting < initial);
        assert!(d.waiting >= after);
    }
    #[test]
    fn zero_or_nan_density_never_spawns() {
        assert_eq!(arrivals_per_minute(40.0, 0.0, 1.0), 0.0);
        assert_eq!(arrivals_per_minute(40.0, f32::NAN, 1.0), 0.0);
        let mut d = StopDemand::default();
        d.advance(1, 20.0, 1.0, 30.0, 0.0, 20);
        assert_eq!(d.waiting, 0.0);
        assert_eq!(patience_seconds(2), patience_seconds(2));
    }
}

#[cfg(test)]
mod retry_tests {
    use super::*;

    #[test]
    fn failed_spawn_is_not_retried_every_frame() {
        let mut d = StopDemand::default();
        assert!(d.can_retry(1.0));
        d.retry_later(1.0, false);
        assert!(!d.can_retry(1.001));
        assert!(!d.can_retry(10.999));
        assert!(d.can_retry(11.0));
        d.retry_later(11.0, true);
        assert!(!d.can_retry(13.0));
        assert!(d.can_retry(13.5));
    }
    #[test]
    fn backlog_never_exceeds_free_waiting_positions() {
        let mut d = StopDemand::default();
        d.advance(77, 100.0, 1.0, 1000.0, 3.0, 4);
        assert!(d.waiting <= 4.0);
        // No free positions remaining: do not accumulate more people.
        d.advance(77, 101.0, 1.0, 1000.0, 3.0, 0);
        assert_eq!(d.waiting, 0.0);
    }
}
