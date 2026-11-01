//! Per-stock-human retarget of the original artist garments.
//!
//! MakeHuman source arms hang diagonally; OMSI rest-arm links differ. A simple
//! translation from shared mannequin anchors produces metre-long triangles.
//! Reconstruct garment-local shoulders/elbows/wrists from the real OBJ surface,
//! then align each segment's axis to the actual stock skeleton before skinning.
//! Keep authored triangles, UVs and fabric textures unchanged.
use super::{Art,Rig};
use glam::{Quat,Vec3};

const MAX_EDGE_M:f32=0.32;
const MAX_STRETCH:f32=3.25;
const EPS:f32=0.0001;

#[derive(Clone,Copy)]
pub(super) struct FitTarget {
    pub waist:Vec3, pub shoulder:[Vec3;2],pub elbow:[Vec3;2],
    pub wrist:[Vec3;2],pub head_pivot:Vec3,pub head_top:f32,pub scale:f32,
}
impl From<&Rig> for FitTarget {
    fn from(r:&Rig)->Self {
        Self{waist:r.waist,shoulder:r.shoulder,elbow:r.elbow,
            wrist:r.wrist,head_pivot:r.head_pivot,head_top:r.head_top,scale:r.scale}
    }
}
fn percentile(mut v:Vec<f32>,q:f32)->f32{
    if v.is_empty(){return 0.0;}
    v.sort_by(|a,b|a.total_cmp(b));
    v[(((v.len()-1) as f32*q).round() as usize).min(v.len()-1)]
}
fn point_band(p:&[Vec3],side:usize,x:f32,r:f32,qz:f32)->Vec3 {
    let sign=if side==0{-1.0}else{1.0};
    let near:Vec<Vec3>=p.iter().copied()
        .filter(|v|(v.x-sign*x).abs()<r).collect();
    let near=if near.len()>=8 {near} else {
        p.iter().copied().filter(|v|v.x*sign>0.0).collect()
    };
    if near.is_empty(){return Vec3::new(sign*x,0.0,1.0);}
    Vec3::new(
        percentile(near.iter().map(|v|v.x).collect(),0.5),
        percentile(near.iter().map(|v|v.y).collect(),0.5),
        percentile(near.iter().map(|v|v.z).collect(),qz),
    )
}
fn align(p:Vec3,src_joint:Vec3,src_next:Vec3,dst_joint:Vec3,dst_next:Vec3)->Vec3{
    let from=src_next-src_joint;
    let to=dst_next-dst_joint;
    if from.length_squared()<EPS*EPS||to.length_squared()<EPS*EPS{return p-src_joint+dst_joint;}
    let rotate=Quat::from_rotation_arc(from.normalize(),to.normalize());
    let scale=(to.length()/from.length()).clamp(0.6,1.5);
    dst_joint+rotate*((p-src_joint)*scale)
}
fn garment_fit(art:&Art,target:&FitTarget)->Result<Vec<Vec3>,String>{
    let p=&art.base.positions;
    if p.is_empty(){return Err("empty artist garment".into())}
    let (min,max)=p.iter().fold((Vec3::splat(f32::MAX),Vec3::splat(f32::MIN)),
        |(lo,hi),v|(lo.min(*v),hi.max(*v)));
    if !min.is_finite()||!max.is_finite()||(max-min).max_element()>2.5 {
        return Err("invalid source bounds".into());
    }
    let width=max.x.abs().max(min.x.abs()).max(0.05);
    let mut out=Vec::with_capacity(p.len());
    if art.kind=="hat" {
        // A cap is skinned to HEAD: retain its own shape and anchor the crown,
        // not its geometric centre, to the stock head top.
        let scale=(target.scale*0.86).clamp(0.65,1.10);
        let src=Vec3::new((min.x+max.x)*0.5,(min.y+max.y)*0.5,max.z);
        let dest=Vec3::new(target.head_pivot.x,target.head_pivot.y,target.head_top+0.025);
        for v in p {out.push(dest+(*v-src)*scale);}
    } else if art.kind=="gloves" {
        // The genuine MakeHuman glove pieces point toward negative Y and
        // lie on either side of the mesh. Turn each into OMSI wrist->hand axis.
        let mut roots=[Vec3::ZERO;2];
        let mut rotations=[Quat::IDENTITY;2];
        for side in 0..2 {
            let sign=if side==0{-1.0}else{1.0};
            let half:Vec<Vec3>=p.iter().copied().filter(|a|a.x*sign>0.0).collect();
            if half.len()<8{return Err("one glove side missing".into());}
            roots[side]=Vec3::new(
                percentile(half.iter().map(|a|a.x).collect(),0.5),
                percentile(half.iter().map(|a|a.y).collect(),0.95),
                percentile(half.iter().map(|a|a.z).collect(),0.5),
            );
            let axis=(target.wrist[side]-target.elbow[side]).normalize_or_zero();
            if axis.length_squared()<0.2{return Err("invalid stock wrist direction".into());}
            rotations[side]=Quat::from_rotation_arc(-Vec3::Y,axis);
        }
        let scale=(target.scale*0.85).clamp(0.60,1.12);
        for v in p {
            let side=if v.x<0.0{0}else{1};
            out.push(target.wrist[side]+rotations[side]*((*v-roots[side])*scale));
        }
    } else {
        // The MakeHuman torso is near x=0, shoulders at about .35 of the half
        // width, elbows .66, cuffs .90. Source arms are diagonally DOWN;
        // OMSI stock joints can be in entirely different rest directions.
        let src_sh=[point_band(p,0,width*0.35,width*0.07,0.82),
                    point_band(p,1,width*0.35,width*0.07,0.82)];
        let src_el=[point_band(p,0,width*0.66,width*0.07,0.50),
                    point_band(p,1,width*0.66,width*0.07,0.50)];
        let src_wr=[point_band(p,0,width*0.91,width*0.07,0.50),
                    point_band(p,1,width*0.91,width*0.07,0.50)];
        let mid:Vec<Vec3>=p.iter().copied().filter(|v|v.x.abs()<width*0.2).collect();
        if mid.len()<10{return Err("cannot locate source torso".into());}
        let src_waist=Vec3::new(0.0,
            percentile(mid.iter().map(|v|v.y).collect(),0.5),
            percentile(mid.iter().map(|v|v.z).collect(),0.5)-0.16);
        let sh_height=(src_sh[0].z+src_sh[1].z)*0.5;
        let target_sh_height=(target.shoulder[0].z+target.shoulder[1].z)*0.5;
        let sz=((target_sh_height-target.waist.z)/(sh_height-src_waist.z).max(0.15)).clamp(0.58,1.4);
        let sx=(target.shoulder[1].x.abs()/src_sh[1].x.abs().max(0.10)).clamp(0.50,1.4);
        let sy=(target.scale*0.9).clamp(0.65,1.18);
        for (i,v) in p.iter().enumerate() {
            let torso=target.waist+(*v-src_waist)*Vec3::new(sx,sy,sz);
            let mut value=Vec3::ZERO;
            let mut sum=0.0;
            // These are source-semantic bindings generated from the real OBJ.
            // All transforms are FIRST matched in the rest pose. OMSI then
            // applies the same slots when the character walks or sits.
            for (slot,weight,_) in &art.influences[i]{
                if *weight<=0.0{continue;}
                let mapped=if *slot==omsi_sim::human::slot_of(omsi_sim::human::BONE_MAIN).unwrap(){
                    torso
                } else if *slot==omsi_sim::human::slot_of(omsi_sim::human::BONE_OA_L).unwrap(){
                    align(*v,src_sh[0],src_el[0],target.shoulder[0],target.elbow[0])
                } else if *slot==omsi_sim::human::slot_of(omsi_sim::human::BONE_OA_R).unwrap(){
                    align(*v,src_sh[1],src_el[1],target.shoulder[1],target.elbow[1])
                } else if *slot==omsi_sim::human::slot_of(omsi_sim::human::BONE_UA_L).unwrap(){
                    align(*v,src_el[0],src_wr[0],target.elbow[0],target.wrist[0])
                } else if *slot==omsi_sim::human::slot_of(omsi_sim::human::BONE_UA_R).unwrap(){
                    align(*v,src_el[1],src_wr[1],target.elbow[1],target.wrist[1])
                } else if *slot==omsi_sim::human::hand_slot(0){
                    align(*v,src_el[0],src_wr[0],target.elbow[0],target.wrist[0])
                } else if *slot==omsi_sim::human::hand_slot(1){
                    align(*v,src_el[1],src_wr[1],target.elbow[1],target.wrist[1])
                } else {torso};
                value+=mapped* *weight;
                sum+=*weight;
            }
            if sum<EPS{return Err("unweighted garment vertex".into());}
            out.push(value/sum);
        }
    }
    if !triangles_safe(&art.base.positions,&out){return Err("fitted garment contains stretched or broken triangles".into());}
    Ok(out)
}
/// No rogue triangles may reach the GPU even when a nonstandard add-on rig is
/// exceptionally small, inverted, malformed, or an animation is nonfinite.
pub(super) fn triangles_safe(rest:&[Vec3],posed:&[Vec3])->bool{
    if rest.len()!=posed.len()||posed.len()%3!=0||posed.iter().any(|v|!v.is_finite()){return false;}
    for (r,p) in rest.chunks_exact(3).zip(posed.chunks_exact(3)) {
        for (i,j) in [(0,1),(1,2),(2,0)]{
            let a=(r[i]-r[j]).length();
            let b=(p[i]-p[j]).length();
            if !b.is_finite()||b>MAX_EDGE_M||(a>0.015&&b/a>MAX_STRETCH){return false;}
        }
    }
    true
}
pub(super) fn fitted(art:&Art,rig:&Rig)->Result<Vec<Vec3>,String>{
    garment_fit(art,&FitTarget::from(rig))
}
#[cfg(test)]
mod tests{
    use super::*;
    #[test] fn arm_segment_rotation_is_not_a_global_translation(){
        let src_sh=Vec3::new(0.30,-0.1,0.91);
        let src_el=Vec3::new(0.56,-0.15,0.56);
        let dst_sh=Vec3::new(0.22,0.0,1.37);
        let dst_el=Vec3::new(0.48,0.0,1.30);
        let v=align(src_el,src_sh,src_el,dst_sh,dst_el);
        assert!((v-dst_el).length()<0.0001);
    }
    #[test] fn giant_disconnected_triangles_are_never_accepted(){
        let rest=[Vec3::ZERO,Vec3::X*0.03,Vec3::Y*0.03];
        let bad=[Vec3::ZERO,Vec3::X*0.9,Vec3::Y*0.03];
        assert!(!triangles_safe(&rest,&bad));
        assert!(triangles_safe(&rest,&rest));
    }
}
