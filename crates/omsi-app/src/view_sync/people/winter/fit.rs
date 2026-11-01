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
const MAX_STRETCH:f32=6.25; // Adjacent sleeve seams on the real hooded jacket reach ~6x locally; absolute 32 cm edge limit still blocks spikes.
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
        // Anchor the authored torso HEM near the actual passenger waist.
        // The old median-Z anchor was around 65 cm ABOVE the hem on the
        // hooded jacket. Mapping it to the stock waist dragged the coat down
        // to shin level (min z ~= 0.11 m on a 1.75 m adult).
        let src_waist=Vec3::new(0.0,
            percentile(mid.iter().map(|v|v.y).collect(),0.5),
            percentile(mid.iter().map(|v|v.z).collect(),0.02)+0.08);
        let sh_height=(src_sh[0].z+src_sh[1].z)*0.5;
        let target_sh_height=(target.shoulder[0].z+target.shoulder[1].z)*0.5;
        // Stock torso is much shorter than the MakeHuman garment's source
        // coordinate range. A 0.58 minimum scale was too large here.
        let sz=((target_sh_height-target.waist.z)/(sh_height-src_waist.z).max(0.15)).clamp(0.30,1.4);
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
    #[test] fn actual_seven_artist_meshes_fit_multiple_stock_like_rigs_without_exploding(){
        let Ok(base)=std::env::var("OPENOMSI_WINTER_TEST_ASSETS") else{return;};
        let root=std::path::Path::new(&base).join("bcs-assets").join("WinterClothes");
        let names=["elvs_hooded_sweat_jacket1","toigo_fisherman_sweater",
            "mindfront_knitted_sweater_01","elvs_slouchy_beanie1",
            "mindfront_knitted_hat_01","toigo_gloves_short","toigo_gloves_medium"];
        for (size,arm_drop) in [(0.80,0.18),(1.00,0.08),(1.15,0.33)]{
            let t=FitTarget{
                waist:Vec3::new(0.0,0.0,1.04*size),
                shoulder:[Vec3::new(-0.21*size,0.0,1.42*size),
                          Vec3::new(0.21*size,0.0,1.42*size)],
                elbow:[Vec3::new(-0.37*size,-0.01,1.38*size-arm_drop),
                       Vec3::new(0.37*size,-0.01,1.38*size-arm_drop)],
                wrist:[Vec3::new(-0.55*size,0.0,1.34*size-arm_drop*2.0),
                       Vec3::new(0.55*size,0.0,1.34*size-arm_drop*2.0)],
                head_pivot:Vec3::new(0.0,-0.02,1.54*size),
                head_top:1.75*size,scale:size,
            };
            for name in names {
                let (art,_)=super::super::load_asset_candidate(&root,name)
                    .unwrap_or_else(|e|panic!("{name}: {e}"));
                let result=garment_fit(&art,&t);
                let posed=result.unwrap_or_else(|e|panic!("size {size} asset {name}: {e}"));
                assert_eq!(posed.len(),art.base.positions.len());
                assert!(triangles_safe(&art.base.positions,&posed),"{name} invalid triangles");
                let min=posed.iter().fold(Vec3::splat(f32::MAX),|a,v|a.min(*v));
                let max=posed.iter().fold(Vec3::splat(f32::MIN),|a,v|a.max(*v));
                assert!((max-min).max_element()<2.5,"{name} exploded extent");
                if art.kind=="torso" {
                    // A coat is not a black column reaching the passenger's
                    // shins. Leave room for low sleeve cuffs on bent arms.
                    assert!(min.z>=t.waist.z-0.45*size,
                        "{name} fell below the human waist into the legs: min={} waist={}",
                        min.z,t.waist.z);
                }
                eprintln!("FIT_OK asset={} scale={} arm_drop={} extent={:?}",name,size,arm_drop,max-min);
            }
        }
    }
    #[test] fn seven_real_garments_remain_bounded_during_joint_poses(){
        // Exercise the actual authored meshes through continuous shoulder,
        // elbow, hand and head movements, not just static fitting. This is a
        // numerical deformation gate, NOT an in-game visual QA approval.
        let Ok(base)=std::env::var("OPENOMSI_WINTER_TEST_ASSETS") else{return;};
        let root=std::path::Path::new(&base).join("bcs-assets").join("WinterClothes");
        let names=["elvs_hooded_sweat_jacket1","toigo_fisherman_sweater",
            "mindfront_knitted_sweater_01","elvs_slouchy_beanie1",
            "mindfront_knitted_hat_01","toigo_gloves_short","toigo_gloves_medium"];
        use glam::Affine3A;
        use omsi_sim::human;
        fn about(p:Vec3,q:Quat)->Affine3A {
            Affine3A::from_translation(p)*Affine3A::from_quat(q)*Affine3A::from_translation(-p)
        }
        for size in [0.80_f32,1.0,1.15] {
            let target=FitTarget{
                waist:Vec3::new(0.0,0.0,1.04*size),
                shoulder:[Vec3::new(-0.21*size,0.0,1.42*size),
                          Vec3::new(0.21*size,0.0,1.42*size)],
                elbow:[Vec3::new(-0.37*size,-0.01,1.30*size),
                       Vec3::new(0.37*size,-0.01,1.30*size)],
                wrist:[Vec3::new(-0.55*size,0.0,1.18*size),
                       Vec3::new(0.55*size,0.0,1.18*size)],
                head_pivot:Vec3::new(0.0,-0.02,1.54*size),
                head_top:1.75*size,scale:size,
            };
            for name in names {
                let (art,_)=super::super::load_asset_candidate(&root,name)
                    .unwrap_or_else(|e|panic!("{name}: {e}"));
                let rest=garment_fit(&art,&target)
                    .unwrap_or_else(|e|panic!("{name} rest: {e}"));
                for angle in [0.0_f32,0.12,0.25] {
                    let mut bones=[Affine3A::IDENTITY;human::SLOTS];
                    for side in 0..2 {
                        let sign=if side==0{-1.0}else{1.0};
                        let upper=about(target.shoulder[side],Quat::from_rotation_y(sign*angle));
                        let fore=upper*about(target.elbow[side],Quat::from_rotation_y(-sign*angle*0.8));
                        let hand=fore*about(target.wrist[side],Quat::from_rotation_z(sign*angle*0.5));
                        bones[human::slot_of(if side==0{human::BONE_OA_L}else{human::BONE_OA_R}).unwrap()]=upper;
                        bones[human::slot_of(if side==0{human::BONE_UA_L}else{human::BONE_UA_R}).unwrap()]=fore;
                        bones[human::hand_slot(side)]=hand;
                    }
                    bones[human::slot_of(human::BONE_HEAD).unwrap()]=
                        about(target.head_pivot,Quat::from_rotation_z(angle*0.6));
                    let posed:Vec<Vec3>=rest.iter().enumerate().map(|(i,p)|{
                        art.influences[i].iter().fold(Vec3::ZERO,|acc,(slot,w,_)|
                            acc+bones[*slot].transform_point3(*p)* *w)
                    }).collect();
                    assert!(triangles_safe(&rest,&posed),
                        "animated mesh unsafe: {name} size={size} angle={angle}");
                    eprintln!("ANIMATED_FIT_OK asset={name} size={size} angle={angle}");
                }
            }
        }
    }
    #[test] fn actual_installed_stock_humans_fit_without_triangle_explosions(){
        // Opt-in integration gate using the *real* licensed OMSI .hum/.o3d
        // files, not synthetic mannequin coordinates. A public CI runner
        // cannot redistribute or invent the user's installed stock humans.
        // This is NUMERICAL validation, never an in-game visual sign-off.
        let Some(files)=std::env::var_os("OPENOMSI_WINTER_STOCK_HUMANS") else {
            eprintln!("STOCK_HUMAN_NUMERIC_QA_SKIPPED: set OPENOMSI_WINTER_STOCK_HUMANS to one or more installed .hum paths");
            return;
        };
        let asset_base=std::env::var("OPENOMSI_WINTER_TEST_ASSETS")
            .expect("stock-human QA needs OPENOMSI_WINTER_TEST_ASSETS");
        let humans:Vec<_>=std::env::split_paths(&files).collect();
        assert!(!humans.is_empty(),"no stock .hum files supplied");
        let root=std::path::Path::new(&asset_base).join("bcs-assets").join("WinterClothes");
        let names=["elvs_hooded_sweat_jacket1","toigo_fisherman_sweater",
            "mindfront_knitted_sweater_01","elvs_slouchy_beanie1",
            "mindfront_knitted_hat_01","toigo_gloves_short","toigo_gloves_medium"];
        let arts:Vec<_>=names.iter().map(|name|
            super::super::load_asset_candidate(&root,name)
                .unwrap_or_else(|e|panic!("asset {name}: {e}")).0
        ).collect();
        use glam::Affine3A;
        use omsi_sim::human;
        let mut checked=0;
        for path in humans {
            assert!(path.is_file(),"missing licensed OMSI human: {}",path.display());
            let ty=human::HumanType::load(&path)
                .unwrap_or_else(|e|panic!("could not load {}: {e:#}",path.display()));
            let rig=&ty.rig;
            for art in &arts {
                let rest=fitted(art,rig).unwrap_or_else(|e|
                    panic!("{} against {}: {e}",art.name,path.display()));
                assert!(triangles_safe(&art.base.positions,&rest),
                    "unsafe real-stock rest pose {} on {}",art.name,path.display());
                // Exercise the same weighted bone slots as the game on the
                // actual measured stock joints. This cannot replace a real
                // screenshot of walking, sitting, looking and arm movement.
                for angle in [0.0_f32,0.12,0.25] {
                    let mut bones=[Affine3A::IDENTITY;human::SLOTS];
                    for side in 0..2 {
                        let sign=if side==0{-1.0}else{1.0};
                        let upper=Affine3A::from_translation(rig.shoulder[side])
                            *Affine3A::from_quat(Quat::from_rotation_y(sign*angle))
                            *Affine3A::from_translation(-rig.shoulder[side]);
                        let fore=upper
                            *Affine3A::from_translation(rig.elbow[side])
                            *Affine3A::from_quat(Quat::from_rotation_y(-sign*angle*0.8))
                            *Affine3A::from_translation(-rig.elbow[side]);
                        let hand=fore
                            *Affine3A::from_translation(rig.wrist[side])
                            *Affine3A::from_quat(Quat::from_rotation_z(sign*angle*0.5))
                            *Affine3A::from_translation(-rig.wrist[side]);
                        bones[human::slot_of(if side==0{human::BONE_OA_L}else{human::BONE_OA_R}).unwrap()]=upper;
                        bones[human::slot_of(if side==0{human::BONE_UA_L}else{human::BONE_UA_R}).unwrap()]=fore;
                        bones[human::hand_slot(side)]=hand;
                    }
                    bones[human::slot_of(human::BONE_HEAD).unwrap()]=
                        Affine3A::from_translation(rig.head_pivot)
                        *Affine3A::from_quat(Quat::from_rotation_z(angle*0.6))
                        *Affine3A::from_translation(-rig.head_pivot);
                    let posed:Vec<Vec3>=rest.iter().enumerate().map(|(i,p)|
                        art.influences[i].iter().fold(Vec3::ZERO,|acc,(slot,w,_)|
                            acc+bones[*slot].transform_point3(*p)* *w)
                    ).collect();
                    assert!(triangles_safe(&rest,&posed),
                        "unsafe real-stock animation {} on {} angle={angle}",
                        art.name,path.display());
                }
                checked+=1;
                eprintln!("STOCK_HUMAN_NUMERIC_FIT_OK human={} garment={} height_m={:.3}",
                    path.display(),art.name,rig.head_top);
            }
        }
        eprintln!("STOCK_HUMAN_NUMERIC_QA_COMPLETE: {checked} actual human/garment combinations; IN_GAME_VISUAL_QA_STILL_REQUIRED");
    }
    #[test] fn giant_disconnected_triangles_are_never_accepted(){
        let rest=[Vec3::ZERO,Vec3::X*0.03,Vec3::Y*0.03];
        let bad=[Vec3::ZERO,Vec3::X*0.9,Vec3::Y*0.03];
        assert!(!triangles_safe(&rest,&bad));
        assert!(triangles_safe(&rest,&rest));
    }
}
