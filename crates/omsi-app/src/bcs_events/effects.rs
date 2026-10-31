//! Bounded scene-only effects. Completion comes from BCS, never a guessed countdown.
use super::lifecycle::Snapshot;
use glam::{DVec3,Mat4,Vec2,Vec3};
use omsi_geometry::MeshData;
use omsi_render::{AlphaMode,Renderer,Scene,SmokeParticle};
use omsi_sim::people::world::World as _;

#[derive(Default)]
pub(super) struct Effects {
    key:Option<(String,u32,u64,u32)>, at:DVec3, kind:u32, age:f32,
    geometry:Vec<(usize,usize,usize)>,
    world:Option<std::sync::Arc<crate::scene::World>>,
}
impl Effects {
    fn clear(&mut self,r:&Renderer,scene:&mut Scene){
        for (instance,mesh,material)in self.geometry.drain(..){r.remove_instance(scene,instance);r.free_mesh(scene,mesh);r.free_material(scene,material);}
        self.kind=0;
    }
    pub fn update(&mut self,context:Option<&str>,source:Option<&Snapshot>,bus:DVec3,heading:f64,dt:f32,
        world:&std::sync::Arc<crate::scene::World>,r:&Renderer,scene:&mut Scene){
        if self.world.as_ref().is_some_and(|old|!std::sync::Arc::ptr_eq(old,world)) {
            // Resources belonged to the previous world's discarded Scene.
            self.geometry.clear();self.key=None;self.kind=0;
        }
        self.world=Some(world.clone());
        let next=context.zip(source).filter(|(_,s)|matches!(s.event,30..=32)&&!s.completed)
            .map(|(c,s)|(c.to_string(),s.pid,s.instance,s.event));
        if next!=self.key {
            self.clear(r,scene);self.key=next;self.age=0.0;
            if let Some((_,_,_,event))=self.key.as_ref(){
                self.kind=*event;
                let a=heading.to_radians();let forward=DVec3::new(a.sin(),a.cos(),0.0);let right=DVec3::new(a.cos(),-a.sin(),0.0);
                self.at=bus+match self.kind{30=>right*3.5-forward*3.0,32=>right*1.6-forward*8.0,_=>DVec3::ZERO};
                let ground=world.data().walk_height_near(self.at.x,self.at.y,self.at.z);
                if let Some(z)=ground{self.at.z=z;}else{self.kind=0;return;}
                let mesh=match self.kind{30=>Some(bin()),32=>Some(oil()),_=>None};
                if let Some(mesh)=mesh {
                    let color=if self.kind==32{[0.018,0.022,0.014,0.87]}else{[0.15,0.22,0.12,1.0]};
                    let material=r.add_material(scene,None,if self.kind==32{AlphaMode::Blend}else{AlphaMode::Opaque},color,false);
                    let mesh=r.add_mesh(scene,&mesh);
                    let instance=r.add_instance(scene,mesh,self.at,Mat4::from_rotation_z(-heading.to_radians()as f32),vec![material]);
                    if self.kind==32{scene.instances[instance].decal=true;scene.instances[instance].surface_bias=true;}
                    self.geometry.push((instance,mesh,material));
                }
                log::info!("BCS visual scene {} created near player bus",self.kind);
            }
        }
        self.age+=dt.max(0.0).min(0.25);
        if self.kind==31{
            let a=heading.to_radians();self.at=bus-DVec3::new(a.sin(),a.cos(),0.0)*4.0+DVec3::Z*1.3;
        }
    }
    pub fn sprites(&self,scene:&mut Scene){
        if !matches!(self.kind,30|31){return;}
        let fire=self.kind==30;
        for k in 0..18 {
            let t=(self.age*0.45+k as f32/18.0).fract();let angle=k as f32*2.39996;
            let radius=0.18+t*0.55;
            scene.smoke.push(SmokeParticle{position:self.at+DVec3::new((angle.cos()*radius)as f64,(angle.sin()*radius)as f64,if fire{0.85}else{0.0}+t as f64*2.6),size:0.25+t*0.8,color:if fire{[0.16,0.15,0.14]}else{[0.32,0.34,0.35]},alpha:(1.0-t)*0.44,ground:Some(self.at.z),..Default::default()});
        }
        if fire {for k in 0..9 {
            let t=(self.age*1.9+k as f32/9.0).fract();let a=k as f32*2.39996;
            scene.smoke.push(SmokeParticle{position:self.at+DVec3::new(a.cos()as f64*0.16,a.sin()as f64*0.16,0.8+t as f64*0.9),size:0.15+(1.0-t)*0.2,color:[1.0,0.2+0.35*(1.0-t),0.015],alpha:(1.0-t)*0.85,ground:Some(self.at.z),..Default::default()});
        }}
    }
}
fn oil()->MeshData{
    let mut mesh=MeshData::default();mesh.positions.push(Vec3::new(0.0,0.0,0.022));
    for k in 0..32{let a=k as f32*std::f32::consts::TAU/32.0;let radius=1.0+0.09*(a*5.0).sin()+0.06*(a*9.0).cos();mesh.positions.push(Vec3::new(a.cos()*1.25*radius,a.sin()*2.4*radius,0.022));}
    for k in 0..32{mesh.indices.extend([0,k+1,(k+1)%32+1]);}
    mesh.normals=vec![Vec3::Z;mesh.positions.len()];mesh.uvs=vec![Vec2::ZERO;mesh.positions.len()];mesh.ranges=vec![(0,mesh.indices.len()as u32,0)];mesh
}
fn bin()->MeshData{
    let mut mesh=MeshData::default();
    for k in 0..12{let a=k as f32*std::f32::consts::TAU/12.0;let b=(k+1)as f32*std::f32::consts::TAU/12.0;let n=mesh.positions.len()as u32;
        mesh.positions.extend([Vec3::new(a.cos()*0.3,a.sin()*0.3,0.0),Vec3::new(b.cos()*0.3,b.sin()*0.3,0.0),Vec3::new(b.cos()*0.32,b.sin()*0.32,0.85),Vec3::new(a.cos()*0.32,a.sin()*0.32,0.85)]);
        mesh.normals.extend([Vec3::new(a.cos(),a.sin(),0.0);4]);mesh.uvs.extend([Vec2::ZERO;4]);mesh.indices.extend([n,n+1,n+2,n,n+2,n+3]);
    }
    mesh.ranges=vec![(0,mesh.indices.len()as u32,0)];mesh
}
