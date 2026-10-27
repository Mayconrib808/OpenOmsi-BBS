//! Optional local cosmetics. No boarding decisions or network messages are changed.
use super::*;
use glam::{Affine3A,Quat};
use omsi_sim::human::{accessory_arm,accessory_head,accessory_hail_arm,grip_centres,hand_slot};
use std::sync::atomic::{AtomicBool,Ordering};
mod geometry;
mod water;
pub(super) const VISUAL_DISTANCE_SQ:f64=300.*300.;
static RAIN:AtomicBool=AtomicBool::new(false);
pub(super) fn raining() -> bool { RAIN.load(Ordering::Relaxed) }

#[derive(Default)]
struct Cover {
    probe: Option<(DVec3, std::time::Instant)>,
    stable: Option<bool>,
    candidate: Option<(bool, std::time::Instant)>,
}
impl Cover {
    fn due(&self, at: DVec3, now: std::time::Instant) -> bool {
        self.probe.is_none_or(|(p, t)| (p-at).length_squared() > 0.49
            || now.saturating_duration_since(t).as_secs_f32() >= 0.3)
    }
    fn sample(&mut self, at: DVec3, covered: bool, now: std::time::Instant) {
        self.probe = Some((at, now));
        if self.stable.is_none() { self.stable = Some(covered); }
        if self.stable == Some(covered) { self.candidate = None; return; }
        match self.candidate {
            Some((value, since)) if value == covered => {
                if now.saturating_duration_since(since).as_secs_f32() >= 0.45 {
                    self.stable = Some(covered); self.candidate = None;
                }
            }
            _ => self.candidate = Some((covered, now)),
        }
    }
}

pub(crate) fn weather(w:&omsi_content::weather::Weather){RAIN.store(w.precip.first().copied().unwrap_or(0.).round() as i32==1&&w.precip.get(1).copied().unwrap_or(0.)>0.,Ordering::Relaxed);}
pub(crate) fn map_key(s:&str)->String{omsi_launcher_lib::passenger_defaults::map_key(s)}
pub(crate) fn map_enabled(list:&str,map:&str)->bool{let key=map_key(map);list.split(';').any(|s|!s.trim().is_empty()&&map_key(s)==key)}
#[derive(Clone,Default)]
pub(crate) struct Config{pub umbrellas:bool,pub hail:bool,pub maps:String}
impl Config{pub fn of(s:&crate::settings::Settings)->Self{Self{umbrellas:s.passenger_umbrellas,hail:s.passenger_hailing,maps:s.passenger_hailing_maps.clone()}}}
#[derive(Default)]
pub(super) struct State{
 cover: Cover,
 look:omsi_sim::human::HailLook,
 wrist:omsi_sim::human::HailWrist,
 full_gate:omsi_sim::human::FullBusHail<omsi_sim::people::BusId>,
 pub open:f32,pub arm:f32,pub hail:f32,pub carried:bool,pub grip:Vec3,pub hail_side:usize,
 wet:f32,pub wetness:f32,pub time:f32,pub cap:Vec3,pub size:f32,pub raise:f32,pub cap_rotation:Quat,initialized:bool,pub rest:Option<Arc<HandRest>>,
}
pub(super) struct HandVertex{mesh:usize,index:usize,pos:Vec3,nrm:Vec3,original:Vec3,original_nrm:Vec3}
pub(super) struct HandRest{vertices:[Vec<HandVertex>;2],pub grip:[Vec3;2]}
impl HandRest{
 pub fn new(ty:&HumanType)->Self{
 let curled=omsi_sim::human::curl_hands(ty,0.008);let grip=grip_centres(ty,0.008);
 let mut out=Self{vertices:[Vec::new(),Vec::new()],grip:[Vec3::ZERO;2]};
 for side in 0..2{
 let w=ty.rig.wrist[side];let u=(w-ty.rig.elbow[side]).normalize_or(Vec3::X);
 let len=(ty.joints.finger-ty.joints.hand).length().clamp(0.12,0.3);
 out.grip[side]=grip[side].unwrap_or(w+u*len*0.5);
 for (mi,m) in ty.meshes.iter().enumerate(){for(i,inf)in m.skin.iter().enumerate(){
 if (0..inf.n as usize).any(|k|inf.slot[k] as usize==hand_slot(side)&&inf.weight[k]>0.5){
 out.vertices[side].push(HandVertex{mesh:mi,index:i,pos:curled[mi].0[i],nrm:curled[mi].1[i],original:m.data.positions[i],original_nrm:m.data.normals[i]});
 }}}
 }out
 }
}
impl State{
 pub fn probe_due(&self, at: DVec3, now: std::time::Instant) -> bool { self.cover.due(at, now) }
 pub fn sample_cover(&mut self, at: DVec3, covered: bool, now: std::time::Instant) { self.cover.sample(at, covered, now); }
 pub fn advance(&mut self,p:&PersonFrame,ty:&HumanType,dt:f32,time:f64,c:&Config,enabled:bool,near:bool,hail_map:bool,bones:&mut[Affine3A;omsi_sim::human::SLOTS]){
 let outside=matches!(p.place,Place::Ground);let ordinary=p.hidden.is_none()&&!p.puppet;
 let eligible=enabled&&ordinary&&near;
 let raining=RAIN.load(Ordering::Relaxed);
 self.time=time.rem_euclid(3600.) as f32;
 if raining{self.wet+=dt;}else{self.wet=0.;}
 // Every nearby outdoor passenger carries one; stagger only the opening, not ownership.
 if eligible&&c.umbrellas&&outside&&raining{self.carried=true;}
 let want_open=eligible&&c.umbrellas&&self.carried&&outside&&raining&&self.cover.stable==Some(false);
 if !self.initialized{
  // A person first seen during existing rain already has an open umbrella.
  // Entering the distance budget is not the beginning of the rain.
  if want_open{self.open=1.;self.raise=1.;self.arm=1.;}
  if !outside || !raining || self.cover.stable.is_some() { self.initialized=true; }
 }
 let raise_to=want_open||self.open>0.001;
 self.raise+=(if raise_to{1.}else{0.}-self.raise).clamp(-dt/0.75,dt/0.65);
 let unfurl=want_open&&self.raise>0.995;
 self.open+=(if unfurl{1.}else{0.}-self.open).clamp(-dt/0.55,dt/0.90);
 if raining&&outside&&self.open>0.5{self.wetness=1.;}else{self.wetness=(self.wetness-dt/12.).max(0.);}
 let carry=eligible&&c.umbrellas&&self.carried&&(outside||!matches!(p.facts.activity,omsi_sim::human::Activity::Sit|omsi_sim::human::Activity::Pay));
 self.arm+=(if carry{1.}else{0.}-self.arm).clamp(-dt*3.,dt*3.);
 if !enabled||!c.umbrellas {self.open=0.;self.arm=0.;}
 // Recognition comes from the simulation's cosmetic early-approach check.
 // Keep the arm raised continuously while the bus moves; never hail a parked bus.
 let ask=eligible&&c.hail&&hail_map&&outside&&p.hail_moving&&p.hail_target.is_some();
 let full_ok=p.hail_bus.map_or(true,|bus|self.full_gate.allow(bus,p.hail_full,ask,dt));
 let hail=ask&&full_ok;
 let starting_hail=self.hail<=0.001&&hail;
 self.hail+=(if hail{1.}else{0.}-self.hail).clamp(-dt*2.8,dt*2.8);
 if !enabled||!c.hail||!hail_map {self.hail=0.;}
 let wrist_wave=self.wrist.advance(self.hail>0.001,dt,p.id);
 let torso_slot=omsi_sim::human::slot_of(omsi_sim::human::BONE_MAIN).unwrap();
 let waiting_forward=bones[torso_slot].transform_vector3(Vec3::Y);
 let gaze=if outside && eligible && c.hail && hail_map {
    p.hail_target.and_then(|bus|omsi_sim::human::hail_look_target(p.position,p.heading,bus))
 }else{None};
 let gaze_amount=self.hail*self.hail*(3.-2.*self.hail);
 self.look.advance(&ty.rig,bones,gaze,gaze_amount,dt);
 // Choose the free hand once per request; do not swap arms as the bus passes.
 if starting_hail{self.hail_side=if self.arm>0.05||gaze.is_some_and(|v|v.x>0.){0}else{1};}else if self.arm>0.05{self.hail_side=0;}
 let scale=ty.rig.scale.clamp(0.55,1.5);self.size=(0.96+(p.id%7)as f32*0.012)*scale.sqrt();
 let head=accessory_head(&ty.rig,bones);
 if self.arm>0.001{
 let opening=self.raise*self.raise*(3.-2.*self.raise);
 let lowered=Vec3::new(head.x+0.24*scale,head.y+0.08*scale,head.z-0.60*scale);
 let raised=Vec3::new(head.x+0.20*scale,head.y+0.22*scale,head.z-0.43*scale);
 let wrist=lowered.lerp(raised,opening);
 accessory_arm(&ty.rig,bones,1,wrist,self.arm,true,0.);
 let rest=self.rest.as_ref().map_or(ty.rig.wrist[1],|r|r.grip[1]);self.grip=bones[hand_slot(1)].transform_point3(rest);
 // Lowest fabric edge clears the model's posed head, with the canopy centred over it.
 let above=head+Vec3::new(0.,0.035*scale,0.29*self.size+0.12*scale);
 // A single rigid transform drives the shaft and fabric. Close fully before
 // lowering the wrist; the canopy can never hinge independently of the shaft.
 let up=Quat::from_rotation_arc(Vec3::Z,(above-self.grip).normalize_or(Vec3::Z));
 let down=Quat::from_rotation_x(std::f32::consts::PI);
 let rotation=down.slerp(up,opening);
 self.cap=self.grip+rotation*Vec3::Z*(0.84*scale);
 self.cap_rotation=rotation;
 }
 if self.hail>0.001{
 let side=self.hail_side;
 let ease=self.hail; // The arm IK owns its phase; do not ease it twice.
 // Boarding request observed in Brazilian reference photographs: arm extended
 // in the waiting pose's streetward direction, around shoulder height.
 accessory_hail_arm(&ty.rig,bones,side,waiting_forward,ease,wrist_wave);
 }
 }
 pub fn skin(&self,bones:&[Affine3A;omsi_sim::human::SLOTS],skins:&mut[(Vec<Vec3>,Vec<Vec3>)]){
 let Some(rest)=&self.rest else{return};
 for side in 0..2{let carry=side==1&&self.arm>0.05;let point=side==self.hail_side&&self.hail>0.05;if !carry&&!point{continue}
 let b=bones[hand_slot(side)];
 for v in &rest.vertices[side]{let Some((pos,nrm))=skins.get_mut(v.mesh)else{continue};
 // Hailing uses the model's relaxed open hand; only the umbrella hand curls.
 let (mut p,n)=if point{(v.original,v.original_nrm)}else{(v.pos,v.nrm)};
 let blend=if point{self.hail}else{self.arm};p=v.original.lerp(p,blend.clamp(0.,1.));
 pos[v.index]=b.transform_point3(p);nrm[v.index]=b.transform_vector3(n).normalize_or(Vec3::Z);
 }
 }
 }
}
#[derive(Default)]
pub(super) struct Renders{meshes:Option<(MeshId,MeshId,Vec<MaterialId>,Vec<Vec<MaterialId>>)>,live:HashMap<u32,(usize,usize)>,spare:Vec<(usize,usize)>,water:water::WaterRenders,pub new_this_frame:usize}
impl Renders{
 pub fn retire(&mut self,id:u32,hidden:&mut Vec<usize>){self.water.retire(id,hidden);if let Some((a,b))=self.live.remove(&id){hidden.extend([a,b]);self.spare.push((a,b));}}
 pub fn rename(&mut self,from:u32,to:u32){self.water.rename(from,to);if let Some(pair)=self.live.remove(&from){self.live.insert(to,pair);}}
 pub fn draw(&mut self,p:&PersonFrame,st:&State,at:DVec3,xf:Mat4,allowed:bool,renderer:&Renderer,scene:&mut Scene){
 let show=allowed&&st.arm>0.02&&p.hidden.is_none()&&st.carried;
 if !show{self.water.hide(p.id,renderer,scene);if let Some((a,b))=self.live.remove(&p.id){renderer.set_params(scene,a,&[],false,&[]);renderer.set_params(scene,b,&[],false,&[]);self.spare.push((a,b));}return}
 if self.meshes.is_none(){
 let material=|scene:&mut Scene,color:[f32;4]|renderer.add_material_extra(scene,None,AlphaMode::Opaque,color,false,None,None,None,None,[0.;3],omsi_render::MaterialExtra{solid:true,..Default::default()});
 let metal=material(scene,[0.32,0.34,0.37,1.]);let handle=material(scene,[0.035,0.025,0.02,1.]);
 let colors=[[0.025,0.028,0.04,1.],[0.025,0.06,0.16,1.],[0.35,0.045,0.15,1.],[0.15,0.055,0.28,1.],[0.65,0.08,0.04,1.],[0.06,0.3,0.12,1.],[0.65,0.4,0.025,1.],[0.025,0.22,0.4,1.]];
 let mats:Vec<_>=colors.into_iter().map(|c|material(scene,c)).collect();let palettes=(0..5).map(|i|{let mut v=(0..8).map(|j|mats[if i==4{j}else{i}]).collect::<Vec<_>>();v.push(metal);v}).collect();
 let cap=renderer.add_mesh(scene,&geometry::canopy());let shaft=renderer.add_mesh(scene,&geometry::shaft());self.meshes=Some((cap,shaft,vec![metal,handle],palettes));
 }
 let (cap,shaft,body_mats,palettes)=self.meshes.as_ref().unwrap();let palette=(p.id%5)as usize;
 let pair=if let Some(pair)=self.live.get(&p.id){*pair}else{
 // Bound allocation per frame; every nearby eligible passenger can eventually receive one.
 if self.new_this_frame>=12{return}
 self.new_this_frame+=1;
 let pair=self.spare.pop().unwrap_or_else(||(renderer.add_instance(scene,*shaft,at,Mat4::IDENTITY,body_mats.clone()),renderer.add_instance(scene,*cap,at,Mat4::IDENTITY,palettes[palette].clone())));
 // A pooled instance keeps its original palette; variation is cosmetic only.
 self.live.insert(p.id,pair);pair};
 let base=at+xf.transform_vector3(st.grip).as_dvec3();
 let axis=st.cap-st.grip;let rotation=Quat::from_rotation_arc(Vec3::Z,axis.normalize_or(Vec3::Z));
 renderer.set_transform(scene,pair.0,base,xf*Mat4::from_quat(rotation)*Mat4::from_scale(Vec3::new(st.size,st.size,axis.length().max(0.05)/0.78)));renderer.set_params(scene,pair.0,&[],true,&[]);
 let open=st.open.clamp(0.,1.);let diameter=0.04+0.96*open;
 let cap_xf=xf*Mat4::from_quat(st.cap_rotation)*Mat4::from_scale(Vec3::new(diameter*st.size,diameter*st.size,(1.9-0.9*open)*st.size));
 let cap_at=at+xf.transform_vector3(st.cap).as_dvec3();
 renderer.set_transform(scene,pair.1,cap_at,cap_xf);renderer.set_params(scene,pair.1,&[],true,&[]);
 self.water.draw(p,st,at,xf,renderer,scene);
 }
}

#[cfg(test)]
mod cover_tests {
    use super::*;
    #[test]
    fn roof_edges_need_sustained_exposure_and_use_real_time() {
        use std::time::{Instant, Duration};
        let t = Instant::now(); let at = DVec3::ZERO; let mut c = Cover::default();
        c.sample(at, true, t); assert_eq!(c.stable, Some(true));
        for i in 1..10 {
            c.sample(at, i%2 == 0, t + Duration::from_millis(i*300));
            assert_eq!(c.stable, Some(true));
        }
        c.sample(at, false, t + Duration::from_secs(4));
        c.sample(at, false, t + Duration::from_millis(4600));
        assert_eq!(c.stable, Some(false));
        c.sample(at, true, t + Duration::from_secs(5));
        c.sample(at, true, t + Duration::from_millis(5600));
        assert_eq!(c.stable, Some(true));
        assert!(!c.due(at, t + Duration::from_millis(5700)));
        assert!(c.due(at + DVec3::X, t + Duration::from_millis(5700)));
    }
}
