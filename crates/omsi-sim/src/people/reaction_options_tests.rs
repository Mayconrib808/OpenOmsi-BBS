use super::*;

#[test]
fn disabled_reactions_do_not_change_waits_or_rng_and_enabled_thanks_are_cosmetic() {
    let (root,mut people,bus,i)=fixture("options");
    let rng = people.rng;

    people.door_reaction(i, &bus, true, 0.25);
    people.thank_on_exit(usize::MAX);
    assert_eq!(people.pax(i).unwrap().door_reaction_wait, 4.9);
    assert_eq!(people.rng, rng);
    assert!(people.dialog.is_none());

    let selected = (1..1000).find(|&id| exit_thanks_selected(id, 0)).unwrap();
    people.people[i].id = selected;
    people.optional_features.passenger_exit_thanks = true;
    people.thank_on_exit(i);
    assert_eq!(
        people.dialog.as_deref(),
        Some(PassengerDialog::ExitThanks.id())
    );
    assert_eq!(
        people.rng, rng,
        "cosmetic thanks must not alter boarding/ticket RNG"
    );

    people.dialog = None;
    people.last_chat = -100.0;
    people.optional_features.passenger_door_comments = true;
    people.door_reaction(i, &bus, true, 0.25);
    assert!(people.pax(i).unwrap().door_reaction_wait > 4.9);
    assert_eq!(
        people.dialog.as_deref(),
        Some(PassengerDialog::OpenDoor.id())
    );
    assert_eq!(people.rng, rng);
    std::fs::remove_dir_all(root).unwrap();
}

fn fixture(name:&str)->(PathBuf,PeopleSim,BusNow,usize) {
    let root = std::env::temp_dir().join(format!("omsi-optional-reactions-{}-{name}", std::process::id()));
    let humans = root.join("Humans/Test");
    std::fs::create_dir_all(&humans).unwrap();
    std::fs::write(humans.join("person.hum"), "[model]\nmodel.cfg\n").unwrap();
    std::fs::write(humans.join("model.cfg"), "").unwrap();
    std::fs::write(
        root.join("test.bus"),
        "[boundingbox]\n2.5\n12\n3\n0\n0\n1.5\n[passengercabin]\ncabin.cfg\n[paths]\npaths.cfg\n",
    )
    .unwrap();
    std::fs::write(
        root.join("paths.cfg"),
        "[pathpnt]\n0\n0\n0.4\n[pathpnt]\n0\n1\n0.4\n[pathlink]\n0\n1\n",
    )
    .unwrap();
    std::fs::write(
        root.join("cabin.cfg"),
        "[entry]\n0\n[exit]\n0\n[passpos]\n-0.5\n0\n0.9\n0.45\n0\n",
    )
    .unwrap();
    let def = omsi_vehicle::Vehicle::load(&root.join("test.bus")).unwrap();
    let cabin = Arc::new(Cabin::load_train(&[(&def, Vec3::ZERO, f32::INFINITY)]).unwrap());
    let bus = BusNow {
        id: BusId::Player,
        cabin,
        pos: DVec3::ZERO,
        rot: Mat4::IDENTITY,
        heading: 0.0,
        speed: 0.0,
        entry_open: vec![true],
        exit_open: vec![false],
        walk_open: None,
        interior: 0.0,
        air: CabinAir::default(),
        half: DVec2::new(1.25, 6.0),
        centre: DVec2::ZERO,
        accel: DVec2::ZERO,
        trailers: vec![],
        terminus: Some("Town".into()),
        takes: Takes::Terminus,
        next_stop: None,
        places_off: vec![],
        served: None,
        route: None,
    };
    let world = crate::world_data::WorldData::default();
    let mut people = PeopleSim::new(&root, 20);
    let i = people
        .spawn(
            &world,
            DVec3::ZERO,
            0.0,
            State::Pax(Box::new(Pax::new(1.0, 0.5))),
        )
        .unwrap();
    let p = people.pax_mut(i).unwrap();
    p.bus = Some(BusId::Player);
    p.inside = Some(BusId::Player);
    p.task = Task::InBusToExit;
    p.door = Some(0);
    p.pos = DVec3::ZERO;
    p.door_reaction_wait = 4.9;
    people.time = 100.0;
    (root,people,bus,i)
}
fn enable_brazilian(people:&mut PeopleSim,i:usize) {
    people.optional_features.passenger_door_comments=true;
    people.brazilian_voice_dir=Some(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../coop-assets/PassengerVoices/pt-BR"));
    people.last_chat=-100.0;
    people.pax_mut(i).unwrap().door_reaction_wait=0.0;
}
#[test]
fn closed_rear_door_requests_brazilian_audio_after_two_seconds_and_cancels_when_open() {
    let (root,mut h,mut bus,i)=fixture("rear");enable_brazilian(&mut h,i);
    // An unrelated open exit must not silence this passenger's closed rear exit.
    bus.exit_open.push(true);
    for _ in 0..7 {h.door_reaction(i,&bus,true,0.25);h.time+=0.25;}
    assert!(h.voice_lines.is_empty());
    h.door_reaction(i,&bus,true,0.25);
    assert_eq!(h.voice_lines.len(),1);
    assert!(h.voice_lines[0].path.file_name().unwrap().to_str().unwrap().starts_with("OpenDoor_"));
    assert!(h.voice_lines[0].driver_request);
    for _ in 0..16 {h.time+=0.25;h.door_reaction(i,&bus,true,0.25);}
    assert_eq!(h.voice_lines.len(),1,"no per-frame repetition");
    bus.exit_open[0]=true;h.door_reaction(i,&bus,true,0.25);
    assert_eq!(h.pax(i).unwrap().door_reaction_wait,0.0);
    std::fs::remove_dir_all(root).unwrap();
}
#[test]
fn passed_stop_speaks_once_without_prior_stopping_or_opening_the_front_door() {
    let (root,mut h,mut bus,i)=fixture("missed");enable_brazilian(&mut h,i);
    h.pax_mut(i).unwrap().reaction_stop=Some(DVec3::ZERO);
    bus.entry_open[0]=false;bus.speed=5.0;bus.pos=DVec3::new(0.0,-40.0,0.0);
    h.door_reaction(i,&bus,false,0.25);assert!(h.voice_lines.is_empty(),"still approaching");
    bus.pos.y=40.0;h.door_reaction(i,&bus,false,0.25);
    assert_eq!(h.voice_lines.len(),1);
    assert!(h.voice_lines[0].path.file_name().unwrap().to_str().unwrap().starts_with("MissedStop_"));
    for _ in 0..200 {h.time+=0.25;h.door_reaction(i,&bus,false,0.25);}
    assert_eq!(h.voice_lines.len(),1,"one complaint for this missed alighting");
    std::fs::remove_dir_all(root).unwrap();
}
#[test]
fn boarding_requests_require_a_closed_door_and_available_capacity() {
    let (root,mut h,mut bus,i)=fixture("boarding");enable_brazilian(&mut h,i);
    let p=h.pax_mut(i).unwrap();p.task=Task::ToBus;p.inside=None;p.pos=DVec3::ZERO;
    bus.entry_open[0]=false;
    h.seats.insert(bus.id,vec![true;bus.cabin.seats.len()]);
    for _ in 0..16 {h.door_reaction(i,&bus,true,0.25);h.time+=0.25;}
    assert!(h.voice_lines.is_empty(),"full bus");
    h.seats.insert(bus.id,vec![false;bus.cabin.seats.len()]);
    for _ in 0..8 {h.door_reaction(i,&bus,true,0.25);h.time+=0.25;}
    assert_eq!(h.voice_lines.len(),1,"closed entry with a free place");
    h.voice_lines.clear();h.voices=2;h.last_chat=-100.0;h.pax_mut(i).unwrap().door_reaction_cooldown=0.0;
    for _ in 0..12 {h.door_reaction(i,&bus,true,0.25);h.time+=0.25;}
    assert!(h.voice_lines.is_empty(),"passenger mute is respected");
    std::fs::remove_dir_all(root).unwrap();
}
