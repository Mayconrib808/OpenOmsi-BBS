//! Native raindrop film plus pooled gravity-driven rim and closed-tip droplets.
use super::*;
struct Meshes { film:MeshId,edge:MeshId,closed:MeshId,film_mat:MaterialId,drop_mat:MaterialId }
#[derive(Default)]
pub(super) struct WaterRenders { meshes:Option<Meshes>,live:HashMap<u32,[usize;5]>,spare:Vec<[usize;5]> }
/// Existing water stays on the folded cloth while it dries. Opening is a pose,
/// not a visibility switch: hiding at 97% made all beads pop off when closing.
fn wet_canopy_pose(st: &State, at: DVec3, xf: Mat4) -> Option<(DVec3, Mat4)> {
 if st.wetness<0.05{return None}
 Some(super::canopy_pose(st,at,xf))
}
impl WaterRenders {
 pub fn retire(&mut self,id:u32,hidden:&mut Vec<usize>){if let Some(pair)=self.live.remove(&id){hidden.extend(pair);self.spare.push(pair);}}
 pub fn rename(&mut self,from:u32,to:u32){if let Some(pair)=self.live.remove(&from){self.live.insert(to,pair);}}
 pub fn hide(&mut self,id:u32,renderer:&Renderer,scene:&mut Scene){if let Some(pair)=self.live.remove(&id){for i in pair{renderer.set_params(scene,i,&[],false,&[]);}self.spare.push(pair);}}
 pub fn draw(&mut self,p:&PersonFrame,st:&State,at:DVec3,xf:Mat4,renderer:&Renderer,scene:&mut Scene){
  let Some((cap_at,fabric))=wet_canopy_pose(st,at,xf) else{self.hide(p.id,renderer,scene);return};
  if self.meshes.is_none(){
   // The same renderer/material branch used by the windscreen and mirror rain film.
   let film_mat=renderer.add_material_extra(scene,None,AlphaMode::Blend,[1.;4],false,None,None,None,None,[0.;3],
    omsi_render::MaterialExtra{rain_film:true,glass:true,no_z_write:true,solid:true,..Default::default()});
   let drop_mat=renderer.add_material_extra(scene,None,AlphaMode::Blend,[0.66,0.78,0.86,0.82],false,None,None,None,None,[0.;3],
    omsi_render::MaterialExtra{glass:true,no_z_write:true,solid:true,specular:[0.95,0.98,1.,80.],..Default::default()});
   self.meshes=Some(Meshes{film:renderer.add_mesh(scene,&geometry::rain_canopy()),edge:renderer.add_mesh(scene,&geometry::edge_drops()),
    closed:renderer.add_mesh(scene,&geometry::closed_drops()),film_mat,drop_mat});
  }
  let m=self.meshes.as_ref().unwrap();
  let pair=if let Some(pair)=self.live.get(&p.id){*pair}else{
   let pair=self.spare.pop().unwrap_or_else(||[
    renderer.add_instance(scene,m.film,at,Mat4::IDENTITY,vec![m.film_mat]),
    renderer.add_instance(scene,m.edge,at,Mat4::IDENTITY,vec![m.drop_mat]),renderer.add_instance(scene,m.edge,at,Mat4::IDENTITY,vec![m.drop_mat]),
    renderer.add_instance(scene,m.closed,at,Mat4::IDENTITY,vec![m.drop_mat]),renderer.add_instance(scene,m.closed,at,Mat4::IDENTITY,vec![m.drop_mat])]);
   self.live.insert(p.id,pair);pair
  };
  // Offset in the canopy's own frame to avoid z-fighting even while lowered.
  let film_at=cap_at+fabric.transform_vector3(Vec3::Z*0.0025).as_dvec3();
  renderer.set_transform(scene,pair[0],film_at,fabric);
  renderer.set_params(scene,pair[0],&[st.wetness],true,&[]);
  for k in 0..2{
   // Independent phases and a quadratic fall replace the synchronized straight rods.
   let phase=(st.time*1.25+(p.id%43)as f32*0.117+k as f32*0.5).fract();
   let down=0.62*phase*phase;
   let rim=cap_at-xf.transform_vector3(Vec3::Z*down).as_dvec3();
   renderer.set_transform(scene,pair[1+k],rim,fabric);
   renderer.set_params(scene,pair[1+k],&[st.wetness],st.open>0.001&&phase<st.wetness,&[]);
   let tip=st.cap-Vec3::Z*0.012;
   let fall=(tip.z-0.02).clamp(0.,0.55)*phase*phase;
   let pos=at+xf.transform_vector3(tip-Vec3::Z*fall).as_dvec3();
   renderer.set_transform(scene,pair[3+k],pos,xf);
   renderer.set_params(scene,pair[3+k],&[st.wetness],st.open<0.06&&st.raise<0.12&&tip.z>0.025&&phase<st.wetness,&[]);
  }
 }
}

#[cfg(test)]
mod umbrella_water_tests {
 use super::*;
 #[test]
 fn wet_beads_follow_every_stage_of_folding_and_lowering() {
  let mut st=State{wetness:1.,size:1.12,cap:Vec3::new(0.2,0.3,1.8),..Default::default()};
  let at=DVec3::new(12000.,-34000.,8.);
  let xf=Mat4::from_rotation_z(0.7);
  let mut previous_radius=f32::INFINITY;
  for open in [1.,0.99,0.97,0.95,0.75,0.5,0.25,0.05,0.001,0.] {
   st.open=open;
   for raise in [1.,0.5,0.] {
    st.raise=raise;
    st.cap_rotation=Quat::from_rotation_x((1.-raise)*std::f32::consts::PI);
    let (water_at,water_xf)=wet_canopy_pose(&st,at,xf).expect("wet cloth remains visible during closing and lowering");
    let (cloth_at,cloth_xf)=super::super::canopy_pose(&st,at,xf);
    assert_eq!(water_at,cloth_at);
    for vertex in geometry::rain_canopy().positions {
     assert!((water_xf.transform_point3(vertex)-cloth_xf.transform_point3(vertex)).length()<1e-6);
    }
   }
   let (_,pose)=wet_canopy_pose(&st,at,xf).unwrap();
   let radius=pose.transform_vector3(Vec3::X*0.55).length();
   assert!(radius<=previous_radius+1e-6,"droplet surface folds continuously with the cloth");
   assert!((radius-0.55*(0.04+0.96*open)*st.size).abs()<1e-5);
   previous_radius=radius;
  }
 }
 #[test]
 fn only_dry_cloth_retires_the_water_layer() {
  let mut st=State{open:0.,raise:0.,wetness:0.5,size:1.,..Default::default()};
  assert!(wet_canopy_pose(&st,DVec3::ZERO,Mat4::IDENTITY).is_some());
  st.wetness=0.;
  assert!(wet_canopy_pose(&st,DVec3::ZERO,Mat4::IDENTITY).is_none());
 }
}
