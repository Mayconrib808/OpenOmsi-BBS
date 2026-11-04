//! Shared seated pose and handrim contact geometry.
use super::ChairPose;
use crate::human::{Activity,HumanType,Influence,PoseInput,Rig,SLOTS};
use glam::{Affine3A,DVec2,DVec3,Vec3};

pub struct HandRest(Vec<Vec<(usize,Vec3,Vec3,Influence)>>);
impl HandRest {
    pub fn new(ty:&HumanType)->Self {
        let curled=crate::human::curl_hands(ty,0.009);
        let hands=[crate::human::slot_of(crate::human::BONE_HAND_L).unwrap() as u8,
            crate::human::slot_of(crate::human::BONE_HAND_R).unwrap() as u8];
        Self(ty.meshes.iter().zip(curled).map(|(mesh,(positions,normals))|{
            mesh.skin.iter().enumerate().filter(|(_,inf)|
                (0..inf.n as usize).any(|k|hands.contains(&inf.slot[k])&&inf.weight[k]>0.5))
                .map(|(i,inf)|(i,positions[i],normals[i],*inf)).collect()
        }).collect())
    }
    pub fn apply(&self,bones:&[Affine3A;SLOTS],skins:&mut[(Vec<Vec3>,Vec<Vec3>)]) {
        for(vertices,(positions,normals))in self.0.iter().zip(skins) {
            for &(i,rest,normal,inf)in vertices {
                if i>=positions.len()||i>=normals.len(){continue;}
                let(mut p,mut n)=(Vec3::ZERO,Vec3::ZERO);
                for k in 0..inf.n as usize {
                    let b=bones[inf.slot[k] as usize];let w=inf.weight[k];
                    p+=b.transform_point3(rest)*w;n+=b.transform_vector3(normal)*w;
                }
                positions[i]=p;normals[i]=n.normalize_or(Vec3::Z);
            }
        }
    }
}
pub fn hand_targets(chair:ChairPose)->[Vec3;2] {
    std::array::from_fn(|side|{
        let phase=chair.push_phase[side];
        let(angle,outward)=if chair.moving<0.01{(0.8,0.0)}
            else if phase<0.58{(2.20-(phase/0.58)*2.0,0.0)}
            else{(0.20+((phase-0.58)/0.42)*2.0,
                (((phase-0.58)/0.42)*std::f32::consts::PI).sin()*0.035)};
        let angle=if chair.direction[side]<0.0{std::f32::consts::PI-angle}else{angle};
        Vec3::new(if side==0{-0.366-outward}else{0.366+outward},
            0.240*angle.cos(),0.30+0.240*angle.sin())
    })
}
pub fn foot_targets(rig:&Rig)->[Vec3;2] {
    [-0.14,0.14].map(|x|Vec3::new(x,0.48,0.16+rig.ankle_h))
}
pub fn input<'a>(rig:&Rig,chair:ChairPose,origin:DVec3,heading:f64,frame:u64,
    grips:[Vec3;2],floor:&'a dyn Fn(DVec2)->Option<f64>)->PoseInput<'a> {
    PoseInput{activity:Activity::Sit,origin,heading,frame,velocity:DVec2::ZERO,
        seat:Some(Vec3::new(0.0,rig.seat_front(),0.51)),
        seat_pelvis:Some(Vec3::new(0.0,0.015,0.51+rig.seat_lift)),
        seat_feet:Some(foot_targets(rig)),
        grips:Some(grips),grip_frames:Some(std::array::from_fn(|side|(
            Vec3::new(0.0,grips[side].z-0.30,-grips[side].y).normalize_or(Vec3::Y),
            Vec3::new(if side==0{1.0}else{-1.0},0.0,0.0)))),
        grip_lean:chair.moving*3.0,floor:Some(floor),..Default::default()}
}
#[cfg(test)]mod tests {
    use super::*;
    #[test]fn propulsion_hands_follow_the_actual_rim_radius(){
        for step in 0..58 {
            let chair=ChairPose{moving:1.0,direction:[1.0,-1.0],
                push_phase:[step as f32/100.0;2],..Default::default()};
            for hand in hand_targets(chair){
                assert!((hand.x.abs()-0.366).abs()<1e-6);
                assert!((hand.y.hypot(hand.z-0.30)-0.240).abs()<1e-6);
            }
        }
    }
}
