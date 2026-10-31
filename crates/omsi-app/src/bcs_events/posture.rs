//! A reversible seated collapse; legs and seat anchor stay in their original pose.
use glam::{Affine3A, Quat};
use omsi_sim::human::{HumanType, SLOTS, slot_of, BONE_MAIN, BONE_HEAD,
    BONE_OA_L, BONE_OA_R, BONE_UA_L, BONE_UA_R, BONE_HAND_L, BONE_HAND_R};

pub(crate) fn slump(ty: &HumanType, bones: &mut [Affine3A; SLOTS], blend: f32) {
    if !blend.is_finite() || blend <= 0.0 { return; }
    let blend = blend.clamp(0.0, 1.0);
    let main = slot_of(BONE_MAIN).expect("engine torso slot");
    let pivot = bones[main].transform_point3(ty.rig.waist);
    let lean = Affine3A::from_translation(pivot)
        * Affine3A::from_quat(Quat::from_rotation_z(8.0_f32.to_radians() * blend)
            * Quat::from_rotation_x(-28.0_f32.to_radians() * blend))
        * Affine3A::from_translation(-pivot);
    for bone in [BONE_MAIN, BONE_HEAD, BONE_OA_L, BONE_OA_R, BONE_UA_L, BONE_UA_R, BONE_HAND_L, BONE_HAND_R] {
        if let Some(slot) = slot_of(bone) { bones[slot] = lean * bones[slot]; }
    }
    let head = slot_of(BONE_HEAD).expect("engine head slot");
    let neck = bones[head].transform_point3(ty.rig.head_pivot);
    bones[head] = Affine3A::from_translation(neck)
        * Affine3A::from_quat(Quat::from_rotation_x(-15.0_f32.to_radians() * blend))
        * Affine3A::from_translation(-neck) * bones[head];
}

pub(crate) fn reaction(ty:&HumanType,bones:&mut[Affine3A;SLOTS],event:u32,blend:f32,phase:f32){
    if blend<=0.0{return;}
    let main=slot_of(BONE_MAIN).unwrap();
    if event==6 {
        let pivot=bones[main].transform_point3(ty.rig.waist);
        let sway=Affine3A::from_translation(pivot)*Affine3A::from_quat(
            Quat::from_rotation_z(0.09*phase.sin()*blend)*Quat::from_rotation_x(0.05*(phase*0.7).sin()*blend))
            *Affine3A::from_translation(-pivot);
        for bone in [BONE_MAIN,BONE_HEAD,BONE_OA_L,BONE_OA_R,BONE_UA_L,BONE_UA_R,BONE_HAND_L,BONE_HAND_R]{
            if let Some(slot)=slot_of(bone){bones[slot]=sway*bones[slot];}
        }
    }else if event==7 {
        let shoulder=bones[main].transform_point3(ty.rig.shoulder[0]);
        let target=shoulder+glam::Vec3::new(0.14,0.38,0.08+0.08*(phase*2.0).sin());
        omsi_sim::human::accessory_arm(&ty.rig,bones,0,target,blend*0.8,false,phase.sin()*0.15);
    }
}
