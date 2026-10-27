//! Native raindrop film plus pooled gravity-driven rim and closed-tip droplets.
use super::*;
struct Meshes { film:MeshId,edge:MeshId,closed:MeshId,film_mat:MaterialId,drop_mat:MaterialId }
#[derive(Default)]
pub(super) struct WaterRenders { meshes:Option<Meshes>,live:HashMap<u32,[usize;5]>,spare:Vec<[usize;5]> }
impl WaterRenders {
 pub fn retire(&mut self,id:u32,hidden:&mut Vec<usize>){if let Some(pair)=self.live.remove(&id){hidden.extend(pair);self.spare.push(pair);}}
 pub fn rename(&mut self,from:u32,to:u32){if let Some(pair)=self.live.remove(&from){self.live.insert(to,pair);}}
 pub fn hide(&mut self,id:u32,renderer:&Renderer,scene:&mut Scene){if let Some(pair)=self.live.remove(&id){for i in pair{renderer.set_params(scene,i,&[],false,&[]);}self.spare.push(pair);}}
 pub fn draw(&mut self,p:&PersonFrame,st:&State,at:DVec3,xf:Mat4,renderer:&Renderer,scene:&mut Scene){
  if st.wetness<0.05{self.hide(p.id,renderer,scene);return}
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
  let cap_at=at+xf.transform_vector3(st.cap+Vec3::Z*0.0025).as_dvec3();
  let fabric=xf*Mat4::from_quat(st.cap_rotation)*Mat4::from_scale(Vec3::splat(st.size));
  renderer.set_transform(scene,pair[0],cap_at,fabric);
  renderer.set_params(scene,pair[0],&[st.wetness],st.open>0.97&&st.raise>0.97,&[]);
  for k in 0..2{
   // Independent phases and a quadratic fall replace the synchronized straight rods.
   let phase=(st.time*1.25+(p.id%43)as f32*0.117+k as f32*0.5).fract();
   let down=0.62*phase*phase;
   let rim=cap_at-xf.transform_vector3(Vec3::Z*down).as_dvec3();
   renderer.set_transform(scene,pair[1+k],rim,xf*Mat4::from_scale(Vec3::splat(st.size)));
   renderer.set_params(scene,pair[1+k],&[st.wetness],st.open>0.94&&phase<st.wetness,&[]);
   let tip=st.cap-Vec3::Z*0.012;
   let fall=(tip.z-0.02).clamp(0.,0.55)*phase*phase;
   let pos=at+xf.transform_vector3(tip-Vec3::Z*fall).as_dvec3();
   renderer.set_transform(scene,pair[3+k],pos,xf);
   renderer.set_params(scene,pair[3+k],&[st.wetness],st.open<0.06&&st.raise<0.12&&tip.z>0.025&&phase<st.wetness,&[]);
  }
 }
}
