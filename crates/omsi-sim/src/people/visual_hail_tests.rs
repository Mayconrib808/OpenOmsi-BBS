//! Regression coverage for optional visual-only hailing eligibility.
use super::*;

#[test]
fn request_only_for_close_approaching_buses() {
    let dir = DVec2::Y;
    assert!(PeopleSim::visual_hail_range(DVec3::new(3.0,-50.0,0.0),dir));
    // Register/boarding begins at 60 m. The hailing window must begin
    // earlier, otherwise the waiting task changes before the arm is drawn.
    assert!(PeopleSim::visual_hail_range(DVec3::new(3.0,-80.0,0.0),dir));
    assert!(!PeopleSim::visual_hail_range(DVec3::new(3.0,-100.0,0.0),dir));
    assert!(!PeopleSim::visual_hail_range(DVec3::new(35.0,-20.0,0.0),dir));
    assert!(!PeopleSim::visual_hail_range(DVec3::new(0.0,30.0,0.0),dir));
    assert!(!PeopleSim::visual_hail_range(DVec3::new(0.0,-20.0,12.0),dir));
    assert!(!PeopleSim::visual_hail_range(DVec3::new(f64::NAN,0.0,0.0),dir));
}

#[test]
fn approaching_service_outranks_closer_parked_service() {
    use std::cmp::Ordering;
    // An idle compatible bus at 10 m must not suppress an approaching
    // compatible bus at 75 m (still inside the pre-boarding hail window).
    assert_eq!(PeopleSim::visual_hail_target_order(0.0, 100.0, 6.0, 5625.0), Ordering::Greater);
    assert_eq!(PeopleSim::visual_hail_target_order(6.0, 5625.0, 0.0, 100.0), Ordering::Less);
    // Two approaching buses retain nearest-bus preference.
    assert_eq!(PeopleSim::visual_hail_target_order(6.0, 1600.0, 4.0, 2500.0), Ordering::Less);
    // Two parked buses still retain nearest-bus preference, without raising an arm.
    assert_eq!(PeopleSim::visual_hail_target_order(0.0, 2500.0, 0.0, 1600.0), Ordering::Greater);
    // Reversing and invalid speeds must not count as an approaching bus.
    assert_eq!(PeopleSim::visual_hail_target_order(-1.0, 100.0, 5.0, 2500.0), Ordering::Greater);
    assert_eq!(PeopleSim::visual_hail_target_order(f64::NAN, 100.0, 5.0, 2500.0), Ordering::Greater);
}
