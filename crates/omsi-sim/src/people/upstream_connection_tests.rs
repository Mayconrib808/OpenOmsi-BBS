use super::*;

#[derive(Default)]
pub(super) struct FlatWorld {
    collision: Arc<crate::collision::CollisionWorld>,
    parked: Arc<Vec<crate::collision::Obb>>,
    objects: crate::world_data::ObjectPlaces,
    ground_samples: std::sync::atomic::AtomicUsize,
}

impl World for FlatWorld {
    fn root(&self) -> &Path {
        Path::new("/nonexistent")
    }
    fn map_dir(&self) -> &Path {
        self.root()
    }
    fn walk_height(&self, _: f64, _: f64) -> Option<f64> {
        Some(0.0)
    }
    fn walk_height_near(&self, x: f64, y: f64, _: f64) -> Option<f64> {
        self.ground_samples.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        self.walk_height(x, y)
    }
    fn has_ground(&self, _: f64, _: f64) -> bool {
        true
    }
    fn stop_exit_weight(&self, _: i64) -> f32 {
        0.5
    }
    fn stop_enter(&self, _: i64) -> (f32, f32) {
        (1.0, 0.0)
    }
    fn stop_side(&self, _: i64) -> f32 {
        1.0
    }
    fn stop_length(&self, _: i64) -> f32 {
        30.0
    }
    fn bus_stops(&self) -> &[(i64, DVec3, f64, String)] {
        &[]
    }
    fn waiting_places(&self) -> &[(i64, DVec3, f64, f32)] {
        &[]
    }
    fn object_positions(&self) -> &crate::world_data::ObjectPlaces {
        &self.objects
    }
    fn collision(&self) -> &Arc<crate::collision::CollisionWorld> {
        &self.collision
    }
    fn parked_boxes(&self) -> &Arc<Vec<crate::collision::Obb>> {
        &self.parked
    }
    fn tiles_generation(&self) -> u64 {
        0
    }
}

fn wheelchair_service(name:&str)->(PeopleSim,BusNow,PlayerBusView,FlatWorld) {
    let (mut h,mut bn)=super::super::pax_stops::brt_tests::setup(name,None);
    bn.id=BusId::Player;
    let p=wheelchair::tests::profile();
    h.wheelchairs.enabled=true;
    h.wheelchairs.profile=Some(p.clone());h.wheelchairs.profiles=vec![p.clone()];
    h.player_next_stop=bn.next_stop.clone();
    h.people[0].position=bn.world(p.outside(0.0));
    let x=h.pax_mut(0).unwrap();x.stop=None;x.pos=p.outside(0.0).as_dvec3();
    let view=PlayerBusView {
        wheelchair_feedback:wheelchair::Feedback{door_open:true,bay_ready:Some(true),..Default::default()},
        position:bn.pos,heading:bn.heading,rot:bn.rot,speed:0.0,speed_kmh:0.0,
        a_trans:Vec3::ZERO,bounding_box:None,interior:1.0,air:CabinAir::default(),
        given_ticket:None,terminus:None,tt_stop:None,act_busstop:String::new(),route:None,cabin:None,
    };
    (h,bn,view,FlatWorld::default())
}

#[test]
fn wheelchair_request_stays_active_until_alt_assistance_completes_alighting() {
    use wheelchair::Stage;
    let (mut h,bn,mut view,world)=wheelchair_service("wheelchair-alt-service");
    let id=h.people[0].id;
    assert!(h.begin_wheelchair_boarding(0,&bn,&world));h.wheelchairs.assisted=true;
    for _ in 0..12000 {
        h.tick_wheelchairs(1.0/60.0,&world,Some(&view),std::slice::from_ref(&bn));
        if h.wheelchairs.riders.contains_key(&id){break;}
    }
    assert!(h.wheelchairs.riders.contains_key(&id));
    assert!(h.wheelchairs.request_alight(id));
    // A request must reach Alt even before the timetable advances or the bus stops.
    h.player_next_stop=None;view.speed=2.0;view.wheelchair_feedback.door_open=false;
    for _ in 0..120 {
        h.tick_wheelchairs(1.0/60.0,&world,Some(&view),std::slice::from_ref(&bn));
        let j=h.wheelchairs.journey.as_ref().expect("pending request was reparked");
        assert_eq!(j.stage,Stage::Riding);assert!(j.alight_requested);assert!(h.wheelchairs.wants_alight());
        assert!(!j.interlocked());
    }
    view.speed=0.0;
    wheelchair::command(wheelchair::ASSIST);
    h.tick_wheelchairs(1.0/60.0,&world,Some(&view),std::slice::from_ref(&bn));
    assert_eq!(h.wheelchairs.journey.as_ref().unwrap().stage,Stage::Riding);
    view.wheelchair_feedback.door_open=true;
    for _ in 0..12000 {
        h.tick_wheelchairs(1.0/60.0,&world,Some(&view),std::slice::from_ref(&bn));
        if h.wheelchairs.journey.is_none(){break;}
    }
    assert!(h.wheelchairs.journey.is_none());assert!(h.wheelchairs.riders.is_empty());
    assert!(!h.wheelchairs.wants_alight());assert_eq!(h.people[0].id,id);
    assert!(h.pax(0).is_none_or(|p|p.inside.is_none()));
}

#[test]
fn wheelchair_boarding_yields_past_an_occupied_bay_with_no_free_standing_place() {
    let (mut h,bn,view,world)=wheelchair_service("wheelchair-full-bay");
    let (mut crowd,_,_,_)=wheelchair_service("wheelchair-crowd");
    let mut other=crowd.people.pop().unwrap();other.id=2;
    let bay=h.wheelchairs.profile.as_ref().unwrap().bay;
    other.position=bn.world(bay);other.place=Place::Bus(BusId::Player,bay);
    if let State::Pax(x)=&mut other.state {x.inside=Some(BusId::Player);x.bus=Some(BusId::Player);
        x.task=Task::SittingInBus;x.pos=bay.as_dvec3();x.seat=None;}
    h.people.push(other);h.seats.insert(BusId::Player,vec![true;bn.cabin.seats.len()]);
    let id=h.people[0].id;
    assert!(h.begin_wheelchair_boarding(0,&bn,&world));h.wheelchairs.assisted=true;
    for _ in 0..12000 {
        h.tick_wheelchairs(1.0/60.0,&world,Some(&view),std::slice::from_ref(&bn));
        if h.wheelchairs.riders.contains_key(&id){break;}
    }
    assert!(h.wheelchairs.riders.contains_key(&id),"occupied bay permanently blocked boarding");
    assert_eq!(h.people.len(),2);
}

#[test]
fn connection_rejects_unloaded_ground_and_large_steps() {
    let from = DVec3::ZERO;
    let to = DVec3::X * 10.0;
    assert!(connection_walkable(from, to, |_, _, _| Some(0.0)));
    assert!(!connection_walkable(from, to, |x, _, _| (x < 4.0
        || x > 6.0)
        .then_some(0.0)));
    assert!(!connection_walkable(from, to, |x, _, _| Some(if x < 5.0 {
        0.0
    } else {
        2.0
    })));
    assert!(!connection_walkable(from, DVec3::NAN, |_, _, _| Some(0.0)));
    assert!(!connection_walkable(from, DVec3::X * 81.0, |_, _, _| Some(
        0.0
    )));
}

#[test]
fn connection_keeps_identity_and_stops_after_two_transfers() {
    let (mut h, mut bus) = super::super::pax_stops::brt_tests::setup("connections-identity", None);
    h.transfer_percent = 100.0;
    bus.entry_open.fill(true);
    let mut other = bus.clone();
    other.id = BusId::Ai(4);
    other.entry_open.fill(true);
    let stop = h.stops.get_mut(&42).unwrap();
    stop.dests = vec![("Elsewhere".into(), 1.0)];
    stop.lines = vec![(
        "Elsewhere".into(),
        ["Elsewhere".into()].into_iter().collect(),
    )];
    stop.buses = vec![(other.id, true), (bus.id, true)];
    let id = h.people[0].id;
    let ty = h.people[0].ty.clone();
    let buses = [bus, other];
    let ix = buses.iter().enumerate().map(|(i, b)| (b.id, i)).collect();
    let at = h.people[0].position;
    let world = FlatWorld::default();
    for count in 1..=2 {
        assert!(h.try_connection(
            0,
            at,
            0.0,
            Some(42),
            buses[(count - 1) % 2].id,
            &buses,
            &ix,
            &world
        ));
        assert_eq!(h.people.len(), 1);
        assert_eq!(h.people[0].id, id);
        assert!(Arc::ptr_eq(&h.people[0].ty, &ty));
        assert_eq!(h.pax(0).unwrap().transfer_count, count as u8);
        assert_eq!(h.pax(0).unwrap().task, Task::WalkingToBusstop);
        assert_eq!(
            h.pax(0).unwrap().transfer_from,
            Some(buses[(count - 1) % 2].id)
        );
    }
    assert!(!h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.people[0].id, id);
}

#[test]
fn connections_need_a_service_and_disabled_path_does_not_draw_randomness() {
    let (mut h, bus) = super::super::pax_stops::brt_tests::setup("connections-service", None);
    let buses = [bus];
    let ix = [(buses[0].id, 0)].into_iter().collect();
    let at = h.people[0].position;
    let world = FlatWorld::default();
    let rng = h.rng;
    assert!(!h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.rng, rng);
    h.transfer_percent = 100.0;
    let stop = h.stops.get_mut(&42).unwrap();
    stop.dests = vec![("Elsewhere".into(), 1.0)];
    stop.lines = vec![(
        "Elsewhere".into(),
        ["Elsewhere".into()].into_iter().collect(),
    )];
    assert!(!h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.pax(0).unwrap().transfer_count, 0);
    assert_eq!(world.ground_samples.load(std::sync::atomic::Ordering::Relaxed), 0,
        "stops without valid onward service must not trigger terrain sampling");
}

#[test]
fn chance_zero_twenty_and_hundred_respect_draw_and_service() {
    let (mut h, mut bus) = super::super::pax_stops::brt_tests::setup("connections-chance", None);
    bus.entry_open.fill(true);
    let mut other = bus.clone();
    other.id = BusId::Ai(4);
    other.entry_open.fill(true);
    let stop = h.stops.get_mut(&42).unwrap();
    stop.dests = vec![("Elsewhere".into(), 1.0)];
    stop.lines = vec![("Elsewhere".into(), ["Elsewhere".into()].into_iter().collect())];
    stop.buses = vec![(other.id, true), (bus.id, true)];
    let buses = [bus, other];
    let ix = buses.iter().enumerate().map(|(i, b)| (b.id, i)).collect();
    let at = h.people[0].position;
    let world = FlatWorld::default();

    h.transfer_percent = 0.0;
    let before = h.rng;
    assert!(!h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.rng, before);
    assert_eq!(world.ground_samples.load(std::sync::atomic::Ordering::Relaxed), 0);

    h.transfer_percent = 20.0;
    let mut success_seed = None;
    let mut failure_seed = None;
    for seed in 1..=10000u64 {
        h.rng = seed;
        if h.rand_f() < 0.2 { success_seed.get_or_insert(seed); }
        else { failure_seed.get_or_insert(seed); }
        if success_seed.is_some() && failure_seed.is_some() { break; }
    }
    let fail = failure_seed.expect("20% should have a non-selected deterministic seed");
    h.rng = fail;
    let expected_next = { let _ = h.rand_f(); h.rng };
    h.rng = fail;
    assert!(!h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.rng, expected_next, "rejected draw must consume exactly one RNG sample");

    h.rng = success_seed.expect("20% should have a selected deterministic seed");
    assert!(h.try_connection(0, at, 0.0, Some(42), buses[0].id, &buses, &ix, &world));
    assert_eq!(h.pax(0).unwrap().transfer_count, 1);

    h.transfer_percent = 100.0;
    assert!(h.try_connection(0, at, 0.0, Some(42), buses[1].id, &buses, &ix, &world));
    assert_eq!(h.pax(0).unwrap().transfer_count, 2);
}
