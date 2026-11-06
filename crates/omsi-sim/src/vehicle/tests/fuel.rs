//! The fuel pump (`veh_tank`) as a script runs it: what the "full tank" start and the petrol
//! station both fill the tank with.

use super::*;

/// A bus whose script starts it with 15 % of a 250 l tank and whose pump puts 3 l in a
/// second, up to 250 (the SD202's way); `pump`: whether it has the pump at all.
fn bus(pump: bool) -> VehicleInstance {
    bus_with_frame(pump, "")
}

fn bus_with_frame(pump: bool, frame: &str) -> VehicleInstance {
    static NEXT: std::sync::atomic::AtomicUsize = std::sync::atomic::AtomicUsize::new(0);
    let dir = std::env::temp_dir().join(format!("omsi_fuel_{}_{}", std::process::id(), NEXT.fetch_add(1, std::sync::atomic::Ordering::Relaxed)));
    std::fs::create_dir_all(&dir).unwrap();
    let script = dir.join("engine.osc");
    let trigger = if pump { "{trigger:veh_tank}\n(L.L.engine_tank_content) 3 (L.S.Timegap) * + 250 min (S.L.engine_tank_content)\n{end}\n" } else { "" };
    std::fs::write(&script, format!("{{init}}\n37.5 (S.L.engine_tank_content)\n{{end}}\n{trigger}\n{{frame}}\n{frame}\n{{end}}\n")).unwrap();
    let vars = dir.join("vars.txt");
    std::fs::write(&vars, "engine_tank_content\nengine_fuel_used\nengine_consumption_h\ntank_percent\n").unwrap();
    let constants = dir.join("constants.txt");
    std::fs::write(&constants, "[const]\nengine_tank_capacity\n250\n").unwrap();
    let program = omsi_script::compile(&omsi_script::CompileInput { scripts: vec![script], varlists: vec![vars], constfiles: vec![constants], ..Default::default() });
    assert!(program.errors.is_empty(), "{:?}", program.errors);
    let ty = Arc::new(VehicleType {
        def: Default::default(),
        model: Model::default(),
        model_dir: dir.clone(),
        program: Arc::new(program),
        meshes: Vec::new(),
        lod_meshes: Vec::new(),
        paint_schemes: Vec::new(),
        texchanges: Vec::new(),
        wheel_meshes: Vec::new(),
        suspension_axles: Vec::new(),
        missing_packs: Vec::new(),
        mesh_bounds: Vec::new(),
        mesh_boxes: Vec::new(),
    });
    std::fs::remove_dir_all(dir).unwrap();
    VehicleInstance::new(ty, VehicleHost::new(Default::default()))
}

#[test]
fn the_pump_fills_the_tank_to_what_the_script_holds() {
    let mut v = bus(true);
    assert_eq!(v.var("engine_tank_content"), Some(37.5));
    // filled to the script's own top, and no further
    assert_eq!(v.refuel(), Some(250.0));
    assert_eq!(v.var("engine_tank_content"), Some(250.0));
    // a full tank stays full; the clock the script reads is left as it was
    let gap = v.host.clock.timegap;
    assert_eq!(v.refuel(), Some(250.0));
    assert_eq!(v.host.clock.timegap, gap);
}

#[test]
fn a_bus_without_a_pump_keeps_its_tank() {
    let mut v = bus(false);
    assert_eq!(v.refuel(), None);
    assert_eq!(v.var("engine_tank_content"), Some(37.5));
}

#[test]
fn negative_overrun_consumption_cannot_refill_a_bcs_tank() {
    let mut v = bus_with_frame(true,
        "(L.L.engine_tank_content) 0.01 + (S.L.engine_tank_content)\n\
         (L.L.engine_fuel_used) 0.01 - (S.L.engine_fuel_used)\n\
         -3.6 (S.L.engine_consumption_h)\n\
         (L.L.engine_tank_content) 250 / (S.L.tank_percent)");
    v.set_var("engine_tank_content", 62.5); // BCS: exactly 25 percent.
    for _ in 0..600 { v.update(0.1); }
    assert_eq!(v.var("engine_tank_content"), Some(62.5));
    assert_eq!(v.var("tank_percent"), Some(0.25));
    assert_eq!(v.var("engine_fuel_used"), Some(0.0));
    assert_eq!(v.var("engine_consumption_h"), Some(0.0));
    assert!(v.service("veh_tank", 1.0));
    assert_eq!(v.var("engine_tank_content"), Some(65.5));
    v.update(0.1);
    assert_eq!(v.var("engine_tank_content"), Some(65.5));
    assert_eq!(v.var("tank_percent"), Some(65.5 / 250.0));
}

#[test]
fn driving_still_consumes_fuel_and_external_assignments_remain_authoritative() {
    let mut v = bus_with_frame(true,
        "(L.L.engine_tank_content) 0.01 - 0 max (S.L.engine_tank_content)");
    v.set_var("engine_tank_content", 62.5);
    for _ in 0..600 { v.update(0.1); }
    let remaining = v.var("engine_tank_content").unwrap();
    assert!(remaining < 57.0 && remaining > 56.0, "{remaining}");
    assert_eq!(v.var("tank_percent"), Some(remaining / 250.0));
    v.set_var("engine_tank_content", 150.0);
    v.update_scripts_only(0.0);
    assert!(v.var("engine_tank_content").unwrap() > 149.9);
    assert_eq!(v.refuel(), Some(250.0));
}

#[test]
fn a_paint_scheme_name_with_a_leading_space_is_found() {
    let dir = std::env::temp_dir().join(format!("omsi_cti_{}", std::process::id()));
    std::fs::create_dir_all(&dir).unwrap();
    std::fs::write(dir.join("a.cti"), "[item]\n White | Tector 7 | Manual\nbody\nx.png\n").unwrap();
    let schemes = crate::vehicle::types::load_paint_schemes(&dir);
    assert_eq!(schemes.iter().map(|s| s.name.as_str()).collect::<Vec<_>>(), ["White | Tector 7 | Manual"]);
    let _ = std::fs::remove_dir_all(&dir);
}
