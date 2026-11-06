//! Exercise host intent, client claims, ownership transfer and actual cabin entry together.
use super::*;

fn fixture(name: &str) -> (PeopleSim, BusNow) {
    let (mut people, mut bus) = super::pax_stops::brt_tests::setup(name, None);
    bus.id = BusId::Player;
    bus.served = None;
    bus.entry_open.fill(true);
    bus.terminus = Some("102 DIVISA DISPLAY".into());
    bus.takes = Takes::Duty {
        trip: Arc::new(DutyTrip {
            name: "112 - 102 / A3M / 24".into(), departure: 1143.0,
            terminus: "102 Divisa".into(), public: true,
            stops: vec![(42, "Platform".into(), true), (43, "102 Divisa".into(), true)],
        }), next: 0, done: false,
    };
    // The host supplied a valid destination, but its line key cannot be resolved
    // against this client's local stop table (late loading/different HOF data).
    people.mirror = true;
    people.people[0].remote = true;
    people.people[0].state = State::Idle;
    people.mirror_wait.insert(1, (42, 0));
    people.mirror_hail_intent(1, Some((42, Some(0x102), Some("102 Divisa".into()))));
    people.last_buses = Arc::new(vec![bus.clone()]);
    people.register_buses(std::slice::from_ref(&bus), 0.0);
    (people, bus)
}

#[test]
fn host_destination_can_board_by_the_clients_actual_duty_without_matching_line_keys() {
    let (mut client, bus) = fixture("mirror-duty-boarding");
    assert!(client.listed_at(42, BusId::Player));
    client.claim_waiting();
    let claims = client.take_claims();
    assert_eq!(claims, vec![1], "a valid public duty must not leave all claims at zero");
    let (mut host, _) = super::pax_stops::brt_tests::setup("mirror-duty-host", None);
    host.pax_mut(0).unwrap().task = Task::WaitingForBus;
    assert_eq!(host.hand_over(2, &claims), claims);
    assert!(host.people.is_empty(), "ownership is still decided by the host");
    assert!(client.grant(1));
    assert_eq!(client.pax(0).unwrap().dest.as_deref(), Some("102 Divisa"),
        "grant must retain the host's destination instead of drawing a new one");
    let world = super::upstream_connection_tests::FlatWorld::default();
    let buses = [bus];
    let ix = [(BusId::Player, 0)].into_iter().collect();
    for _ in 0..18000 {
        client.time += 1.0 / 60.0;
        let stops = client.register_buses(&buses, 1.0 / 60.0);
        client.pax_frame(1.0 / 60.0, &world, &buses, &ix, &stops, None,
            &mut false, &mut Vec::new());
        if client.pax(0).is_some_and(|p| p.task == Task::SittingInBus) { break; }
    }
    assert_eq!(client.pax(0).unwrap().task, Task::SittingInBus);
    assert_eq!(client.pax(0).unwrap().inside, Some(BusId::Player));
    assert_eq!(client.people[0].id, 1);
}

#[test]
fn unresolved_host_intent_does_not_claim_wrong_private_or_finished_services() {
    for case in ["wrong-destination", "private", "finished", "no-duty", "previous-bus"] {
        let (mut people, mut bus) = fixture(&format!("mirror-reject-{case}"));
        match case {
            "wrong-destination" => people.mirror_hail_intent(1,
                Some((42, Some(0x102), Some("Unserved destination".into())))),
            "private" => if let Takes::Duty { trip, .. } = &mut bus.takes {
                Arc::make_mut(trip).public = false;
            },
            "finished" => if let Takes::Duty { done, .. } = &mut bus.takes { *done = true; },
            "no-duty" => bus.takes = Takes::Terminus,
            "previous-bus" => { people.mirror_connection_history.insert(1, (1, Some(bus.id))); },
            _ => unreachable!(),
        }
        people.last_buses = Arc::new(vec![bus]);
        people.claim_waiting();
        assert!(people.take_claims().is_empty(), "must reject {case}");
        assert!(people.people[0].remote);
    }
}

#[test]
fn granted_destination_is_not_reassigned_to_an_incompatible_bus() {
    let (mut people, bus) = fixture("mirror-retained-intent");
    assert!(people.grant(1));
    let mut wrong = bus.clone();
    wrong.takes = Takes::Terminus;
    let buses = [wrong];
    let ix = [(BusId::Player, 0)].into_iter().collect();
    assert_eq!(people.bus_for(0, 42, &buses, &ix), None,
        "unresolved host intent cannot become unrestricted no-line boarding after grant");
    let buses = [bus];
    assert_eq!(people.bus_for(0, 42, &buses, &ix), Some((BusId::Player, Fit::Duty)));
}

#[test]
fn mirrored_passengers_with_a_proven_duty_can_also_hail_before_the_stop() {
    let (mut people, mut bus) = fixture("mirror-duty-hailing");
    bus.pos = DVec3::new(-4.0, -70.0, 0.0);
    bus.speed = 5.0;
    people.last_buses = Arc::new(vec![bus.clone()]);
    people.visual_hailing = true;
    let mut frame = crate::view_events::PeopleFrame::default();
    people.snapshot_into(&mut frame);
    assert_eq!(frame.people[0].hail_bus, Some(BusId::Player));
    assert!(frame.people[0].hail_moving);
    assert!(matches!(people.people[0].state, State::Idle));
    assert!(people.people[0].remote, "hailing does not transfer ownership");
}

#[test]
fn a_newly_loaded_stop_has_its_existing_queue_without_waiting_for_future_arrivals() {
    for (name, mean, density, places, expected) in [
        ("ordinary", 2.0, 1.0, 6, 1),
        ("bounded", 50.0, 1.0, 3, 3),
        ("disabled", 2.0, 0.0, 6, 0),
        ("no-demand", 0.0, 1.0, 6, 0),
    ] {
        let (mut people, _) = super::super::pax_stops::brt_tests::setup(
            &format!("initial-stop-{name}"), None);
        people.types = vec![people.people[0].ty.clone()];
        people.people.clear();
        people.density = density;
        people.time = 100.0;
        people.optional_features.independent_passenger_demand = true;
        let stop = people.stops.get_mut(&42).unwrap();
        stop.enter_min = mean;
        stop.enter_max = mean;
        stop.spots = (0..places).map(|i| WaitSpot {
            pos: stop.pos + DVec3::Y * i as f64, face: 0.0, height: 0.0,
        }).collect();
        stop.taken = vec![false; places];
        people.center = stop.pos;
        let world = super::upstream_connection_tests::FlatWorld::default();
        people.stops_tick(0.1, &world);
        let allowed = expected..=if expected == 0 { 0 } else { places.min(4) };
        assert!(allowed.contains(&people.people.len()),
            "{name}: a map opening needs its existing queue, not several empty minutes");
        // Collecting the initial queue does not refill it on another frame.
        while !people.people.is_empty() { people.release(0); people.people.remove(0); }
        people.time += 0.1;
        people.stops_tick(0.1, &world);
        assert!(people.people.is_empty(), "a bus arrival must not refill the queue");
    }
}
