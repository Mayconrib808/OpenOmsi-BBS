use super::*;

#[test]
fn disabled_reactions_do_not_change_waits_or_rng_and_enabled_thanks_are_cosmetic() {
    let root = std::env::temp_dir().join(format!("omsi-optional-reactions-{}", std::process::id()));
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
