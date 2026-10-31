//! Optional #1438: people arrive to a stop over time, independently of a
//! scheduled departure. Passenger objects are still limited to nearby stops;
//! their backlog persists while a stop is outside a player's streaming radius.
//!
//! The host sim owns this accumulator. LAN clients mirror real people only.

#[derive(Clone, Copy, Debug)]
pub struct StopDemand {
    pub waiting: f32,
    pub minute: i64,
    pub last_arrival: f64,
}

impl Default for StopDemand {
    fn default() -> Self {
        Self { waiting: 0.0, minute: i64::MIN, last_arrival: f64::NEG_INFINITY }
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
        if self.minute != minute {
            self.minute = minute;
        }
        let rate = arrivals_per_minute(mean, density, demand_variation(stop, minute));
        self.waiting = (self.waiting + rate * (dt.min(5.0) / 60.0))
            .clamp(0.0, capacity.min(100) as f32);
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
    fn zero_or_nan_density_never_spawns() {
        assert_eq!(arrivals_per_minute(40.0, 0.0, 1.0), 0.0);
        assert_eq!(arrivals_per_minute(40.0, f32::NAN, 1.0), 0.0);
        let mut d = StopDemand::default();
        d.advance(1, 20.0, 1.0, 30.0, 0.0, 20);
        assert_eq!(d.waiting, 0.0);
        assert_eq!(patience_seconds(2), patience_seconds(2));
    }
}
