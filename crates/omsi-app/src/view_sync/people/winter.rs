//! Original lightweight winter garments for standard OMSI human models.
//! These are shared rigid meshes attached to the existing posed human rig.
//! No commercial passenger meshes or textures are copied or redistributed.
use super::*;
use glam::{Affine3A,Quat,Vec2};
use omsi_geometry::MeshData;
use omsi_sim::human::{BONE_HEAD,BONE_MAIN,BONE_OA_L,BONE_OA_R,BONE_UA_L,BONE_UA_R,hand_slot,slot_of,Rig};
use std::f32::consts::TAU;

const PARTS: usize = 9; // body, four sleeves, scarf, cap, two mittens
const VISIBLE_DISTANCE_SQ: f64 = 48.0 * 48.0;
const MAX_VISIBLE: usize = 54;
const NEW_PER_FRAME: usize = 5;

#[derive(Clone,Copy,Debug,PartialEq,Eq)]
pub(super) enum Cold { None, Jacket, Winter }
pub(super) fn cold_level(temp: f32, snow: bool) -> Cold {
    if !temp.is_finite() { return Cold::None; }
    if temp <= 5.0 || (snow && temp <= 12.0) { Cold::Winter }
    else if temp <= 15.0 { Cold::Jacket }
    else { Cold::None }
}
fn winter_cti(ty: &HumanType, variant: usize) -> bool {
    let Some(name) = variant.checked_sub(1).and_then(|i| ty.variants.get(i)).map(|v|v.name.to_lowercase()) else { return false; };
    name.split(|c:char| !c.is_alphanumeric()).any(|s|
        matches!(s, "winter"|"cold"|"coat"|"inverno"|"casaco"|"snow"|"frost"|"winterjacke"|"neve"))
}

fn tri(m: &mut MeshData, a: Vec3, b: Vec3, c: Vec3) {
    let n = (b-a).cross(c-a).normalize_or(Vec3::Z);
    let i = m.positions.len() as u32;
    m.positions.extend([a,b,c]); m.normals.extend([n;3]);
    m.uvs.extend([Vec2::ZERO,Vec2::X,Vec2::Y]);
    m.indices.extend([i,i+1,i+2]);
}
fn complete(mut m: MeshData) -> MeshData {
    m.ranges = vec![(0,m.indices.len() as u32,0)];
    m
}
fn jacket() -> MeshData {
    // Elliptic tapered winter jacket. Unit length is scaled to each stock human's torso.
    let mut m=MeshData::default();
    for k in 0..12 {
        let a=k as f32*TAU/12.; let b=(k+1) as f32*TAU/12.;
        let p=|z:f32,t:f32| Vec3::new((1.12-0.15*z)*t.cos(),(0.87+0.05*z)*t.sin(),z);
        let (a0,b0,a1,b1)=(p(0.,a),p(0.,b),p(1.,a),p(1.,b));
        tri(&mut m,a0,b0,b1); tri(&mut m,a0,b1,a1);
    }
    complete(m)
}
fn sleeve() -> MeshData {
    let mut m=MeshData::default();
    for k in 0..10 {
        let a=k as f32*TAU/10.;let b=(k+1)as f32*TAU/10.;
        let p=|z:f32,t:f32|Vec3::new((1.04-0.1*z)*t.cos(),(1.04-0.1*z)*t.sin(),z);
        let (a0,b0,a1,b1)=(p(0.,a),p(0.,b),p(1.,a),p(1.,b));
        tri(&mut m,a0,b0,b1);tri(&mut m,a0,b1,a1);
    }
    complete(m)
}
fn ring() -> MeshData {
    let mut m=MeshData::default();
    for k in 0..16 { for j in 0..5 {
        let a=k as f32*TAU/16.;let b=(k+1)as f32*TAU/16.;
        let c=j as f32*TAU/5.;let d=(j+1)as f32*TAU/5.;
        let p=|u:f32,v:f32|Vec3::new((0.85+0.18*v.cos())*u.cos(),
                                      (0.85+0.18*v.cos())*u.sin(),0.12*v.sin());
        tri(&mut m,p(a,c),p(b,c),p(b,d));tri(&mut m,p(a,c),p(b,d),p(a,d));
    }}
    complete(m)
}
fn beanie() -> MeshData {
    let mut m=MeshData::default();
    let radius=|t:f32|(1.0-(t*t)*0.86).sqrt();
    for k in 0..16 {for j in 0..4 {
        let a=k as f32*TAU/16.;let b=(k+1)as f32*TAU/16.;
        let z=j as f32/4.;let zz=(j+1)as f32/4.;
        let p=|t:f32,u:f32| Vec3::new(radius(t)*u.cos(),radius(t)*u.sin(),t);
        tri(&mut m,p(z,a),p(z,b),p(zz,b));tri(&mut m,p(z,a),p(zz,b),p(zz,a));
    }}
    complete(m)
}
fn mitten() -> MeshData {
    let mut m=MeshData::default();
    let radius=|t:f32|(1.0-(2.0*t-1.0).powi(2)).max(0.04).sqrt();
    for k in 0..10 {for j in 0..6 {
        let a=k as f32*TAU/10.;let b=(k+1)as f32*TAU/10.;
        let z=j as f32/6.;let zz=(j+1)as f32/6.;
        let p=|t:f32,u:f32| Vec3::new(radius(t)*u.cos(),radius(t)*u.sin(),t);
        tri(&mut m,p(z,a),p(z,b),p(zz,b));tri(&mut m,p(z,a),p(zz,b),p(zz,a));
    }}
    complete(m)
}

struct Shared {
    meshes:[MeshId;5],
    palettes: Vec<[MaterialId;4]>,
}
struct Garment { instances:[usize;PARTS],palette:usize }
#[derive(Default)]
pub(super) struct Renders {
    shared:Option<Shared>,
    live:HashMap<u32,Garment>,
    spare:Vec<Garment>,
    pub new_this_frame:usize,
    last_report:f64,
    created:u64,
}
fn outfit_colour(palette:usize)->[[f32;4];4] {
    let palettes=[
        [[0.09,0.13,0.20,1.],[0.38,0.12,0.15,1.],[0.12,0.14,0.20,1.],[0.13,0.14,0.16,1.]],
        [[0.22,0.21,0.19,1.],[0.16,0.18,0.13,1.],[0.27,0.23,0.20,1.],[0.18,0.18,0.19,1.]],
        [[0.19,0.14,0.22,1.],[0.37,0.17,0.14,1.],[0.19,0.15,0.24,1.],[0.15,0.14,0.17,1.]],
        [[0.08,0.19,0.18,1.],[0.30,0.25,0.11,1.],[0.09,0.21,0.20,1.],[0.19,0.22,0.22,1.]],
        [[0.23,0.15,0.11,1.],[0.12,0.18,0.29,1.],[0.24,0.16,0.13,1.],[0.13,0.18,0.25,1.]],
    ];
    palettes[palette%palettes.len()]
}
fn pose_segment(a:Vec3,b:Vec3,rx:f32,ry:f32)->(Vec3,Mat4) {
    let d=b-a;
    let len=d.length().max(0.01);
    let rot=Quat::from_rotation_arc(Vec3::Z,d.normalize_or(Vec3::Z));
    (a, Mat4::from_quat(rot)*Mat4::from_scale(Vec3::new(rx,ry,len)))
}
fn part_transform(what: usize, rig: &Rig, bones:&[Affine3A;omsi_sim::human::SLOTS]) -> (Vec3,Mat4) {
    let s=rig.scale.clamp(0.6,1.35);
    let main=bones[slot_of(BONE_MAIN).unwrap()];
    let head=bones[slot_of(BONE_HEAD).unwrap()];
    match what {
        0 => pose_segment(main.transform_point3(rig.waist-Vec3::Z*0.16*s),
                          main.transform_point3(rig.neck-Vec3::Z*0.085*s),0.215*s,0.205*s),
        1..=4 => {
            let side=(what-1)/2;
            let upper=bones[slot_of(if side==0 {BONE_OA_L}else{BONE_OA_R}).unwrap()];
            let fore=bones[slot_of(if side==0 {BONE_UA_L}else{BONE_UA_R}).unwrap()];
            let shoulder=upper.transform_point3(rig.shoulder[side]);
            let elbow=upper.transform_point3(rig.elbow[side]);
            let wrist=fore.transform_point3(rig.wrist[side]);
            if (what-1)%2==0 {pose_segment(shoulder,elbow,0.102*s,0.102*s)}
            else {pose_segment(elbow,wrist,0.087*s,0.087*s)}
        }
        5=>{
            let p=main.transform_point3(rig.neck-Vec3::Z*0.025*s);
            (p,Mat4::from_scale(Vec3::new(0.155*s,0.15*s,0.13*s)))
        }
        6=>{
            let p=head.transform_point3(Vec3::new(rig.head_pivot.x,rig.head_pivot.y+0.07*s,rig.head_top-0.17*s));
            let rot=Quat::from_rotation_arc(Vec3::Z,head.transform_vector3(Vec3::Z).normalize_or(Vec3::Z));
            (p,Mat4::from_quat(rot)*Mat4::from_scale(Vec3::new(0.14*s,0.15*s,0.20*s)))
        }
        7|8=>{
            let side=what-7;
            let hand=bones[hand_slot(side)];
            let wrist=hand.transform_point3(rig.wrist[side]);
            let end=hand.transform_point3(rig.wrist[side]+Vec3::Y*0.11*s);
            pose_segment(wrist,end,0.082*s,0.073*s)
        }
        _=>unreachable!()
    }
}
impl Renders {
    pub fn retire(&mut self,id:u32,hidden:&mut Vec<usize>){
        if let Some(garment)=self.live.remove(&id){
            hidden.extend(garment.instances);
            self.spare.push(garment);
        }
    }
    pub fn rename(&mut self,from:u32,to:u32){
        if let Some(garment)=self.live.remove(&from){self.live.insert(to,garment);}
    }
    fn hide(&mut self,id:u32,renderer:&Renderer,scene:&mut Scene){
        if let Some(g)=self.live.remove(&id){
            for i in g.instances{renderer.set_params(scene,i,&[],false,&[]);}
            self.spare.push(g);
        }
    }
    fn assets(&mut self,renderer:&Renderer,scene:&mut Scene){
        if self.shared.is_some(){return;}
        let meshes=[
            renderer.add_mesh(scene,&jacket()),
            renderer.add_mesh(scene,&sleeve()),
            renderer.add_mesh(scene,&ring()),
            renderer.add_mesh(scene,&beanie()),
            renderer.add_mesh(scene,&mitten())
        ];
        let palettes=(0..5).map(|k|{
            let colors=outfit_colour(k);
            colors.map(|col|renderer.add_material_extra(scene,None,AlphaMode::Opaque,col,
                false,None,None,None,None,[0.;3],
                omsi_render::MaterialExtra{solid:true,..Default::default()}))
        }).collect();
        self.shared=Some(Shared{meshes,palettes});
    }
    pub fn draw(&mut self,p:&PersonFrame,st:&PersonView,at:DVec3,xf:Mat4,
                distance_sq:f64,enabled:bool,temp:f32,snow:bool,
                renderer:&Renderer,scene:&mut Scene){
        let level=cold_level(temp,snow);
        if !enabled||level==Cold::None||p.hidden.is_some()||p.puppet||
           distance_sq>VISIBLE_DISTANCE_SQ||winter_cti(&p.ty,st.made_as.1) {
            self.hide(p.id,renderer,scene);return;
        }
        let Some(bones)=st.skin_bones.as_ref() else {self.hide(p.id,renderer,scene);return;};
        if !self.live.contains_key(&p.id){
            if self.new_this_frame>=NEW_PER_FRAME||self.live.len()>=MAX_VISIBLE{return;}
            self.assets(renderer,scene);
            let shared=self.shared.as_ref().unwrap();
            let garment=if let Some(g)=self.spare.pop(){g}else{
                let palette=p.id as usize%shared.palettes.len();
                let meshes=[0,1,1,1,1,2,3,4,4];
                let mats=[0,0,0,0,0,1,2,3,3];
                let instances=std::array::from_fn(|i|
                    renderer.add_instance(scene,shared.meshes[meshes[i]],at,
                                         Mat4::IDENTITY,vec![shared.palettes[palette][mats[i]]]));
                Garment{instances,palette}
            };
            self.live.insert(p.id,garment);
            self.new_this_frame+=1;self.created+=1;
        }
        let ty=st.avatar.as_ref().unwrap_or(&p.ty);
        let rig=&ty.rig;
        let garment=self.live.get(&p.id).unwrap();
        let _palette=garment.palette; // Stable palette for pooled instances.
        for (i,inst) in garment.instances.iter().copied().enumerate(){
            let active = if level==Cold::Winter {true} else {i<=5};
            if !active {renderer.set_params(scene,inst,&[],false,&[]);continue;}
            let (local,pose)=part_transform(i,rig,bones);
            renderer.set_transform(scene,inst,at+xf.transform_vector3(local).as_dvec3(),xf*pose);
            renderer.set_params(scene,inst,&[],true,&[]);
            renderer.set_interior(scene,inst,st.lit*0.5);
        }
    }
    pub fn report(&mut self,time:f64){
        if time-self.last_report>=60.0 {
            log::info!("FEATURE AUDIT default_winter_clothes active_people={} created_since_last={} gpu_parts_per_person={} asset_source=original_geometry",
                self.live.len(),self.created,PARTS);
            self.last_report=time;self.created=0;
        }
    }
}
#[cfg(test)]
mod tests{
    use super::*;
    #[test] fn thresholds_and_weather() {
        assert_eq!(cold_level(22.,false),Cold::None);
        assert_eq!(cold_level(15.,false),Cold::Jacket);
        assert_eq!(cold_level(8.,false),Cold::Jacket);
        assert_eq!(cold_level(5.,false),Cold::Winter);
        assert_eq!(cold_level(10.,true),Cold::Winter);
        assert_eq!(cold_level(f32::NAN,true),Cold::None);
    }
    #[test] fn original_meshes_are_nonempty_and_bounded() {
        for m in [jacket(),sleeve(),ring(),beanie(),mitten()] {
            assert!(!m.positions.is_empty());
            assert!(m.indices.len()<2000);
            assert!(m.positions.iter().all(|v|v.is_finite()));
            assert!(m.indices.iter().all(|&i|(i as usize)<m.positions.len()));
            assert_eq!(m.ranges[0].1 as usize,m.indices.len());
        }
    }
}
