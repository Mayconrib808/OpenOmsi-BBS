//! CPU-only add-on inspection. Optional Y and floor-Z arguments export plan slices.
use glam::Vec3;
use omsi_sim::{VehicleHost, VehicleInstance, VehicleType, people::cabin::Cabin};
use serde_json::json;
use std::{path::Path, sync::Arc};

fn main() -> anyhow::Result<()> {
    let args: Vec<_> = std::env::args().collect();
    let root = Path::new(args.get(1).expect("game root"));
    let file = Path::new(args.get(2).expect(".bus file"));
    let slice = args
        .get(3)
        .zip(args.get(4))
        .map(|(y, z)| (y.parse::<f32>().unwrap(), z.parse::<f32>().unwrap()));
    let v = VehicleInstance::new(
        Arc::new(VehicleType::load(root, file)?),
        VehicleHost::new(Default::default()),
    );
    let ty = &v.ty;
    let cabin = Cabin::load_train(&[(&ty.def, Vec3::ZERO, f32::INFINITY)]);
    let meshes:Vec<_>=ty.meshes.iter().enumerate().map(|(i,m)|{
        let d=&ty.model.meshes[m.def_index];
        json!({"index":i,"file":d.file,"bounds":ty.mesh_boxes[i],"visible":v.mesh_props[i].visible,
            "mouse":d.mouse_event,"animations":d.animations.iter().map(|a|json!({"var":a.variable,"factor":a.factor,
                "written":ty.program.var(&a.variable).is_some_and(|id|ty.program.stores(id))})).collect::<Vec<_>>()})
    }).collect();
    let mut sections = Vec::new();
    if let Some((y, floor)) = slice {
        for offset in [0.10, 0.40, 0.75, 1.10, 1.30] {
            let z = floor + offset;
            let mut lines = Vec::new();
            for (i, m) in ty.meshes.iter().enumerate() {
                let d = &ty.model.meshes[m.def_index];
                if !v.mesh_props[i].visible || !d.animations.is_empty() || d.anim_parent.is_some() {
                    continue;
                }
                let name = d.file.to_lowercase();
                let replaced = ["elevador", "ramp", "hublift", "retratil", "cadeirante"]
                    .iter()
                    .any(|k| name.contains(k));
                for t in m.data.indices.chunks_exact(3) {
                    let p = [
                        m.data.positions[t[0] as usize],
                        m.data.positions[t[1] as usize],
                        m.data.positions[t[2] as usize],
                    ];
                    if p.iter().all(|p| p.y < y - 2.5) || p.iter().all(|p| p.y > y + 2.5) {
                        continue;
                    }
                    let mut cut = Vec::new();
                    for k in 0..3 {
                        let (a, b) = (p[k], p[(k + 1) % 3]);
                        if (a.z - z) * (b.z - z) <= 0.0 && (a.z - b.z).abs() > 1e-6 {
                            let point = a.lerp(b, (z - a.z) / (b.z - a.z));
                            if !cut.iter().any(|p: &Vec3| (*p - point).length() < 1e-5) {
                                cut.push(point);
                            }
                        }
                    }
                    if cut.len() == 2 {
                        lines.push(json!({"mesh":i,"replaced":replaced,"a":cut[0],"b":cut[1]}));
                    }
                }
            }
            sections.push(json!({"offset":offset,"lines":lines}));
        }
    }
    let began=std::time::Instant::now();
    let detection=cabin.as_ref().map(|c|omsi_sim::people::wheelchair::discovery::detect(&v,c));
    let discovery_ms=began.elapsed().as_secs_f64()*1000.0;
    eprintln!("Discovery {}: {:.1} ms",file.display(),discovery_ms);
    println!(
        "{}",
        serde_json::to_string(&json!({"bus":file,"meshes":meshes,"sections":sections,"detection":detection,"discovery_ms":discovery_ms,
            "script_errors":ty.program.errors.len(),"errors":ty.program.errors.iter().take(5).map(|e|e.to_string()).collect::<Vec<_>>(),
            "entries":cabin.as_ref().map(|c|c.entries.iter().map(|d|d.inside).collect::<Vec<_>>()),
            "exits":cabin.as_ref().map(|c|c.exits.iter().map(|d|d.inside).collect::<Vec<_>>()),
            "seats":cabin.as_ref().map(|c|c.seats.iter().map(|s|json!({"floor":s.floor,"seated":s.seated,"switch":s.switch_var})).collect::<Vec<_>>())
        }))?
    );
    Ok(())
}
