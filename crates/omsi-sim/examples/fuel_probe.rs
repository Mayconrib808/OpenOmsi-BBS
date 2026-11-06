//! Replay the add-on's actual fuel macro, including overrun, without rendering.
use omsi_sim::{VehicleHost, VehicleInstance, VehicleType};
use serde_json::json;
use std::{path::Path, sync::Arc};

fn main() -> anyhow::Result<()> {
    let args: Vec<_> = std::env::args().collect();
    let root = Path::new(args.get(1).expect("game root"));
    let file = Path::new(args.get(2).expect(".bus file"));
    let ty = Arc::new(VehicleType::load(root, file)?);
    let block = ty.program.macro_block("engine_consumption").expect("fuel macro");
    let mut program = (*ty.program).clone();
    program.frame = vec![block];
    let mut records = Vec::new();
    let mut guarded = Vec::new();
    for apply_guard in [false, true] {
    for rpm in [700.0, 1800.0, -700.0] {
        for torque in [-500.0, 0.0, 500.0] {
            let mut v = VehicleInstance::new(ty.clone(), VehicleHost::new(Default::default()));
            if apply_guard {
                Arc::make_mut(&mut v.ty).program = Arc::new(program.clone());
            }
            for (name, value) in [
                ("engine_tank_content", 75.0), ("engine_n", rpm),
                ("engine_resultant_M", torque), ("engine_throttle_injection", 0.5),
                ("engine_altload", 0.0), ("engineNA1_M", 0.0), ("engine_on", 1.0),
                ("engine_fuel_used", 0.0), ("engine_energy_used", 0.0),
            ] { v.set_var(name, value); }
            for _ in 0..600 {
                if apply_guard {
                    v.update(0.1);
                } else {
                    v.host.clock.advance(0.1);
                    v.vm.run_frame(&program, &mut v.state, &mut v.host);
                }
            }
            let record = json!({"rpm":rpm,"torque":torque,"seconds":60,
                "litres_before":75,"litres_after":v.var("engine_tank_content"),
                "consumption_lph":v.var("engine_consumption_h"),"fuel_used":v.var("engine_fuel_used"),
                "tank_percent":v.var("tank_percent")});
            if apply_guard { guarded.push(record); } else { records.push(record); }
        }
    }
    }
    println!("{}", serde_json::to_string(&json!({"bus":file,"raw_script_replay":records,"guarded_replay":guarded}))?);
    Ok(())
}
