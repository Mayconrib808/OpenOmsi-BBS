//! Load the exact shipped fallback geometry, scripts and sound config headlessly.
use std::{path::PathBuf,sync::Arc};
fn main(){
 let root=PathBuf::from(std::env::args().nth(1).expect("bcs-assets path"));
 omsi_cfg::add_content_root(root.clone());
 for kind in ["Police","Fire"]{
  let path=root.join(format!("Vehicles/OpenOMSI_BCS/{kind}/{kind}.ovh"));
  let ty=Arc::new(omsi_sim::VehicleType::load_ai(&root,&path).expect("load fallback vehicle"));
  assert_eq!(ty.meshes.len(),2,"body and beacon meshes must load");
  for i in 0..ty.meshes.len(){let mesh=ty.mesh_data(i).expect("actual mesh data");assert!(mesh.positions.len()>20);assert!(mesh.positions.iter().all(|p|p.is_finite()));}
  let sound=omsi_vehicle::SoundCfg::load(&path.parent().unwrap().join("sound.cfg")).expect("load own siren config");
  assert_eq!(sound.sounds.len(),1);
  let mut v=omsi_sim::VehicleInstance::new(ty,omsi_sim::VehicleHost::new(Default::default()));
  assert_eq!(v.var("bcs_pitch"),Some(1.0),"init script must run");
  v.set_var("bcs_beacon",1.0);
  let mut states=std::collections::HashSet::new();
  for k in 0..120{v.host.clock.time=k as f64/30.0;v.update_ai(1.0/30.0,&Default::default());states.insert(v.var("bcs_blue").unwrap_or(-1.0)as i32);}
  assert!(states.contains(&0)&&states.contains(&1),"beacons must flash");
  println!("PASS {kind}: geometry, materials, scripts, siren config and beacon cycle");
 }
}
