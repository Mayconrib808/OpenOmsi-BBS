//! Artist-made, licensed MakeHuman garments over standard OMSI passengers.
//! All geometry and diffuse maps must be installed under bcs-assets/WinterClothes.
//! No fallback generated primitives; missing / malformed assets disable this visual only.
use super::{PersonFrame,PersonView};
use crate::scene::World;
use glam::{DVec3,Mat4,Vec2,Vec3};
use hashbrown::HashMap;
use omsi_geometry::MeshData;
use omsi_render::{AlphaMode,MaterialId,MeshId,Renderer,Scene};
use omsi_sim::feature_audit::{record,ResultKind,Role};
use omsi_sim::human::{self,Rig};
use serde::Deserialize;
use std::{fs,path::Path};

const MAX_NPCS:usize=12;
const MAX_NEW_FRAME:usize=2;
const MAX_DISTANCE_SQ:f64=32.0*32.0;
const MAX_OBJ_BYTES:u64=6_000_000;
const MAX_BINDING_BYTES:u64=3_000_000;
const MAX_VERTICES:usize=36_000;
const VERTEX_BUDGET:usize=120_000;

#[derive(Debug,Deserialize)]
struct TextureRef {file:String}
#[derive(Debug,Deserialize)]
struct Binding {
    garment:String,
    garment_kind:String,
    per_vertex_weights:Vec<Vec<(String,f32)>>,
    reference_anchors_m:std::collections::HashMap<String,[f32;3]>,
    copied_real_textures_and_materials:Vec<TextureRef>,
}
struct Art {
    name:String, kind:String, base:MeshData,
    influences:Vec<Vec<(usize,f32,Vec3)>>,
    diffuse:omsi_texture::Image,
}
struct LivePart {which:usize,mesh:MeshId,instance:usize,rest:Vec<Vec3>,positions:Vec<Vec3>,normals:Vec<Vec3>,reported:bool}
#[derive(Default)]
pub(super) struct Renders {
    loaded:bool,
    art:Vec<Art>,
    materials:Vec<MaterialId>,
    live:HashMap<u32,Vec<LivePart>>,
    spare:HashMap<usize,Vec<LivePart>>,
    pub new_this_frame:usize,
    reported_missing:bool,
}
fn target_bone(rig:&Rig,name:&str)->Option<(usize,Vec3)> {
    use human::{BONE_HEAD,BONE_MAIN,BONE_OA_L,BONE_OA_R,BONE_UA_L,BONE_UA_R,slot_of,hand_slot};
    match name {
        "MAIN"=>Some((slot_of(BONE_MAIN)?,rig.waist)),
        "HEAD"=>Some((slot_of(BONE_HEAD)?,Vec3::new(rig.head_pivot.x,rig.head_pivot.y,rig.head_top-0.11*rig.scale))),
        "UPPER_L"=>Some((slot_of(BONE_OA_L)?,rig.shoulder[0])),
        "FORE_L"=>Some((slot_of(BONE_UA_L)?,rig.elbow[0])),
        "HAND_L"=>Some((hand_slot(0),rig.wrist[0])),
        "UPPER_R"=>Some((slot_of(BONE_OA_R)?,rig.shoulder[1])),
        "FORE_R"=>Some((slot_of(BONE_UA_R)?,rig.elbow[1])),
        "HAND_R"=>Some((hand_slot(1),rig.wrist[1])),
        _=>None,
    }
}
fn allowed_texture_name(text:&str)->bool {
    !text.is_empty() && !text.contains('/') && !text.contains('\\')
        && !text.contains("..") && text.len()<96
}
fn size_bounded(path:&Path,max:u64)->Result<Vec<u8>,String> {
    let m=fs::metadata(path).map_err(|e|format!("{}: {e}",path.display()))?;
    if !m.is_file() || m.len()>max{return Err(format!("{}: size or file rejected",path.display()));}
    fs::read(path).map_err(|e|format!("{}: {e}",path.display()))
}
fn parse_obj(buf:&[u8],binding:&Binding)->Result<(MeshData,Vec<Vec<(usize,f32,Vec3)>>),String>{
    let text=std::str::from_utf8(buf).map_err(|e|e.to_string())?;
    let mut vertices=Vec::<Vec3>::new();
    let mut uv=Vec::<Vec2>::new();
    let mut faces=Vec::<Vec<(usize,usize)>>::new();
    for line in text.lines() {
        let mut words=line.split_ascii_whitespace();
        match words.next() {
            Some("v")=>{
                let vals=words.take(3).map(|s|s.parse::<f32>()).collect::<Result<Vec<_>,_>>().map_err(|e|e.to_string())?;
                if vals.len()!=3||vals.iter().any(|x|!x.is_finite()){return Err("nonfinite OBJ vertex".into());}
                vertices.push(Vec3::new(vals[0],vals[1],vals[2]));
                if vertices.len()>MAX_VERTICES{return Err("too many OBJ vertices".into());}
            },
            Some("vt")=>{
                let vals=words.take(2).map(|s|s.parse::<f32>()).collect::<Result<Vec<_>,_>>().map_err(|e|e.to_string())?;
                if vals.len()!=2||vals.iter().any(|x|!x.is_finite()){return Err("bad OBJ UV".into());}
                uv.push(Vec2::new(vals[0],1.0-vals[1]));
            },
            Some("f")=>{
                let mut face=Vec::new();
                for corner in words {
                    let mut pair=corner.split('/');
                    let v=pair.next().unwrap_or("").parse::<usize>().map_err(|e|e.to_string())?;
                    let t=pair.next().unwrap_or("").parse::<usize>().map_err(|e|e.to_string())?;
                    if v==0||t==0||v>vertices.len()||t>uv.len(){return Err("bad OBJ face index".into());}
                    face.push((v-1,t-1));
                }
                if !(3..=12).contains(&face.len()){return Err("invalid OBJ polygon".into());}
                faces.push(face);
                if faces.len()>MAX_VERTICES*2{return Err("too many OBJ polygons".into());}
            },
            _=>{},
        }
    }
    if vertices.len()!=binding.per_vertex_weights.len() || faces.is_empty(){
        return Err("OBJ and skin bindings disagree".into());
    }
    let mut averaged=vec![Vec3::ZERO;vertices.len()];
    for face in &faces {for k in 1..face.len()-1 {
        let a=vertices[face[0].0];let b=vertices[face[k].0];let c=vertices[face[k+1].0];
        let n=(b-a).cross(c-a);
        for &i in &[0,k,k+1] {averaged[face[i].0]+=n;}
    }}
    for n in &mut averaged {*n=n.normalize_or(Vec3::Z);}
    let mut mesh=MeshData::default();
    let mut skin=Vec::new();
    for face in faces {for k in 1..face.len()-1 {
        for &i in &[0,k,k+1] {
            let (vertex,texture)=face[i];
            mesh.positions.push(vertices[vertex]);mesh.normals.push(averaged[vertex]);
            mesh.uvs.push(uv[texture]);mesh.indices.push(mesh.indices.len() as u32);
            let mut inf=Vec::new();
            for (name,weight) in &binding.per_vertex_weights[vertex] {
                if !weight.is_finite()||*weight<0.0 {return Err("invalid skin weights".into());}
                let Some(anchor)=binding.reference_anchors_m.get(name) else {return Err(format!("reference anchor absent: {name}"));};
                let Some(slot)=human::slot_of(match name.as_str() {
                    "HEAD"=>human::BONE_HEAD,"MAIN"=>human::BONE_MAIN,
                    "UPPER_L"=>human::BONE_OA_L,"UPPER_R"=>human::BONE_OA_R,
                    "FORE_L"=>human::BONE_UA_L,"FORE_R"=>human::BONE_UA_R,
                    "HAND_L"=>human::BONE_OA_L,"HAND_R"=>human::BONE_OA_R,_=>return Err("unknown skin joint".into())
                }) else{return Err("invalid joint slot".into());};
                let slot=if name=="HAND_L"{human::hand_slot(0)}else if name=="HAND_R"{human::hand_slot(1)}else{slot};
                inf.push((slot,*weight,Vec3::from_array(*anchor)));
            }
            if inf.is_empty(){return Err("empty skin binding".into());}
            skin.push(inf);
        }
    }}
    mesh.ranges=vec![(0,mesh.indices.len() as u32,0)];
    mesh.one_sided=false;
    Ok((mesh,skin))
}
fn load_asset(root:&Path,name:&str)->Result<Art,String>{
    if !allowed_texture_name(name){return Err("bad asset name".into());}
    let dir=root.join(name);
    let raw=size_bounded(&dir.join("skin_binding.json"),MAX_BINDING_BYTES)?;
    let binding:Binding=serde_json::from_slice(&raw).map_err(|e|e.to_string())?;
    if binding.garment!=name{return Err("skin binding garment mismatch".into());}
    let (base,influences)=parse_obj(&size_bounded(&dir.join("garment.obj"),MAX_OBJ_BYTES)?,&binding)?;
    let materials=binding.copied_real_textures_and_materials.iter()
        .filter(|m|allowed_texture_name(&m.file)&&m.file.ends_with(".mhmat"))
        .find_map(|m|fs::read_to_string(dir.join(&m.file)).ok());
    let Some(materials)=materials else{return Err("original MakeHuman material file missing".into())};
    let Some(texture_name)=materials.lines().find_map(|line|{
        let mut p=line.split_ascii_whitespace();
        (p.next()==Some("diffuseTexture")).then(||p.next()).flatten()
    }) else{return Err("material has no diffuseTexture".into());};
    if !allowed_texture_name(texture_name) || !binding.copied_real_textures_and_materials.iter().any(|r|r.file==texture_name) {
        return Err("diffuse texture absent or not credited".into());
    }
    let diffuse=omsi_texture::decode_file(&dir.join(texture_name)).map_err(|e|e.to_string())?;
    if diffuse.width>2048||diffuse.height>2048{return Err("winter diffuse exceeds 2K limit".into());}
    Ok(Art{name:name.into(),kind:binding.garment_kind,base,influences,diffuse})
}
fn fitted(art:&Art,rig:&Rig)->Vec<Vec3>{
    let mut out=Vec::with_capacity(art.base.positions.len());
    // Source anchors are *not* exact stock OMSI rest joints; conform each authored
    // vertex to the target's real rest joint while preserving author-created detail.
    // Rigidly re-center accessories so a MakeHuman's atypical hand pose cannot
    // put gloves under the passenger's feet.
    let src_min=art.base.positions.iter().copied().reduce(Vec3::min).unwrap_or(Vec3::ZERO);
    let src_max=art.base.positions.iter().copied().reduce(Vec3::max).unwrap_or(Vec3::ZERO);
    let centre=(src_min+src_max)*0.5;
    for (i,p) in art.base.positions.iter().enumerate() {
        let ref_infs=&art.influences[i];
        let mut mapped=Vec3::ZERO;
        for (slot,w,src_ref) in ref_infs {
            let target=match *slot {
                t if t==human::hand_slot(0)=>rig.wrist[0],
                t if t==human::hand_slot(1)=>rig.wrist[1],
                _=>{
                    // For the other bones identify the matching source reference.
                    let name=if *slot==human::slot_of(human::BONE_HEAD).unwrap(){"HEAD"}
                      else if *slot==human::slot_of(human::BONE_MAIN).unwrap(){"MAIN"}
                      else if *slot==human::slot_of(human::BONE_OA_L).unwrap(){"UPPER_L"}
                      else if *slot==human::slot_of(human::BONE_OA_R).unwrap(){"UPPER_R"}
                      else if *slot==human::slot_of(human::BONE_UA_L).unwrap(){"FORE_L"}
                      else {"FORE_R"};
                    target_bone(rig,name).map(|x|x.1).unwrap_or(rig.waist)
                }
            };
            let source=if art.kind=="gloves" {Vec3::new(if target.x<0.0{-src_max.x.abs()*0.87}else{src_max.x.abs()*0.87},centre.y,centre.z)}
                else if art.kind=="hat" {centre} else {*src_ref};
            let scale=if art.kind=="gloves" {0.57} else {rig.scale.clamp(0.75,1.25)};
            mapped+=(target+(*p-source)*scale)* *w;
        }
        out.push(mapped);
    }
    out
}
fn coat_name(id:u32)->&'static str{if id%4==0{"elvs_hooded_sweat_jacket1"}else if id%2==0{"toigo_fisherman_sweater"}else{"mindfront_knitted_sweater_01"}}
fn candidates(id:u32,very_cold:bool)->Vec<&'static str>{
    let mut v=vec![coat_name(id)];
    if very_cold {
        v.push(if id%3==0{"elvs_slouchy_beanie1"}else{"mindfront_knitted_hat_01"});
        v.push("toigo_gloves_short");
    }
    v
}
impl Renders {
    pub fn retire(&mut self,id:u32,hidden:&mut Vec<usize>){
        if let Some(parts)=self.live.remove(&id){
            for part in parts{hidden.push(part.instance);self.spare.entry(part.which).or_default().push(part);}
        }
    }
    pub fn rename(&mut self,from:u32,to:u32){
        if let Some(p)=self.live.remove(&from){self.live.insert(to,p);}
    }
    fn setup(&mut self,root:&Path,renderer:&Renderer,scene:&mut Scene){
        if self.loaded{return;}
        self.loaded=true;
        for name in [
            "elvs_hooded_sweat_jacket1","toigo_fisherman_sweater","mindfront_knitted_sweater_01",
            "elvs_slouchy_beanie1","mindfront_knitted_hat_01","toigo_gloves_short"
        ] {
            match load_asset(&root.join("bcs-assets").join("WinterClothes"),name) {
                Ok(art)=>{
                    let tex=renderer.add_texture(scene,&art.diffuse,true);
                    let mat=renderer.add_material_extra(scene,Some(tex),AlphaMode::Opaque,[1.;4],false,
                        None,None,None,None,[0.;3],
                        omsi_render::MaterialExtra{solid:true,..Default::default()});
                    self.materials.push(mat);self.art.push(art);
                    record(Role::Client,"winter_clothing",ResultKind::Executed,"authored_asset_loaded",
                        &format!("name={} texture_loaded=true",name));
                }
                Err(reason)=>{record(Role::Client,"winter_clothing",ResultKind::Blocked,"authored_asset_unavailable",
                    &format!("name={} reason={reason}",name));}
            }
        }
    }
    fn hide(&mut self,id:u32,renderer:&Renderer,scene:&mut Scene){
        if let Some(parts)=self.live.remove(&id){
            for part in &parts{renderer.set_params(scene,part.instance,&[],false,&[]);}
            for part in parts {self.spare.entry(part.which).or_default().push(part);}
        }
    }
    pub fn draw(&mut self,p:&PersonFrame,st:&PersonView,at:DVec3,xf:Mat4,
                distance_sq:f64,enabled:bool,temp:f32,snow:bool,root:&Path,
                renderer:&Renderer,scene:&mut Scene,pose_due:bool){
        let cold=temp<=15.0&&temp.is_finite();
        let strong=temp<=5.0||(snow&&temp<=12.0);
        // Do not cover custom avatars, inspectors, and already-tagged winter skins.
        let has_winter_cti=st.made_as.1.checked_sub(1)
            .and_then(|v|p.ty.variants.get(v))
            .is_some_and(|v|{
                let n=v.name.to_lowercase();
                ["winter","cold","coat","inverno","casaco","snow"].iter().any(|tag|n.contains(tag))
            });
        if !enabled||!cold||distance_sq>MAX_DISTANCE_SQ||p.hidden.is_some()||p.puppet
            ||st.avatar.is_some()||has_winter_cti {
            self.hide(p.id,renderer,scene);return;
        }
        self.setup(root,renderer,scene);
        if self.art.is_empty(){return;}
        // Clothes must change while a session is running when the weather crosses
        // 15C or 5C; no respawn/teleport of the passenger is allowed.
        if self.live.contains_key(&p.id) {
            let wanted=candidates(p.id,strong).into_iter()
                .filter_map(|name|self.art.iter().position(|a|a.name==name))
                .collect::<Vec<_>>();
            let actual=self.live.get(&p.id).unwrap().iter().map(|part|part.which).collect::<Vec<_>>();
            if actual!=wanted {self.hide(p.id,renderer,scene);}
        }
        if !self.live.contains_key(&p.id) {
            if self.new_this_frame>=MAX_NEW_FRAME||self.live.len()>=MAX_NPCS{return;}
            let names=candidates(p.id,strong);
            let mut parts=Vec::new();
            let mut triangles=0usize;
            for name in names {
                let Some((idx,art))=self.art.iter().enumerate().find(|(_,a)|a.name==name) else {continue;};
                triangles+=art.base.indices.len()/3;
                if triangles>VERTEX_BUDGET/3 {break;}
                let rest=fitted(art,&p.ty.rig);
                let part=if let Some(mut reuse)=self.spare.get_mut(&idx).and_then(|v|v.pop()){
                    // The GPU instance was last skinned to another passenger. Its
                    // cached vertex buffers cannot be reused without an upload.
                    reuse.rest=rest;
                    reuse.positions.clear();
                    reuse.normals.clear();
                    reuse.reported=false;
                    reuse
                }else{
                    let mesh=renderer.add_mesh(scene,&art.base);
                    let instance=renderer.add_instance(scene,mesh,at,xf,vec![self.materials[idx]]);
                    LivePart{which:idx,mesh,instance,rest,
                        positions:Vec::new(),normals:Vec::new(),reported:false}
                };
                parts.push(part);
            }
            if parts.is_empty(){return;}
            record(Role::Client,"winter_clothing",ResultKind::Executed,"garment_instances_created",
                &format!("person={} temperature_c={temp:.1} source_model={} garments={}",
                    p.id,p.ty.def.path.display(),
                    parts.iter().map(|part|self.art[part.which].name.as_str()).collect::<Vec<_>>().join(",")));
            self.live.insert(p.id,parts);
            self.new_this_frame+=1;
        }
        let Some(bones)=st.skin_bones.as_ref() else {return;};
        let Some(parts)=self.live.get_mut(&p.id) else{return;};
        for part in parts {
            let art=&self.art[part.which];
            if pose_due||part.positions.is_empty() {
                part.positions.clear();part.normals.clear();
                for (i,rest) in part.rest.iter().enumerate() {
                    let mut pt=Vec3::ZERO;let mut normal=Vec3::ZERO;
                    for (slot,weight,_) in &art.influences[i] {
                        let bone=&bones[*slot];
                        pt+=bone.transform_point3(*rest)* *weight;
                        normal+=bone.transform_vector3(art.base.normals[i])* *weight;
                    }
                    part.positions.push(pt);
                    part.normals.push(normal.normalize_or(Vec3::Z));
                }
                renderer.update_mesh(scene,part.mesh,&part.positions,&part.normals,&art.base.uvs);
            }
            renderer.set_transform(scene,part.instance,at,xf);
            renderer.set_params(scene,part.instance,&[],true,&[]);
            renderer.set_interior(scene,part.instance,st.lit*0.5);
            if !part.reported && part.positions.len()==art.base.positions.len()
                    && part.positions.iter().all(|v|v.is_finite()) {
                part.reported=true;
                record(Role::Client,"winter_clothing",ResultKind::Completed,"real_mesh_uploaded_and_instance_visible",
                    &format!("person={} temperature_c={:.1} stock_model={} asset_file={} vertices={} original_diffuse_texture=true bone_deformed=true",
                        p.id,temp,p.ty.def.path.display(),
                        root.join("bcs-assets").join("WinterClothes").join(&art.name).join("garment.obj").display(),
                        part.positions.len()));
            }
        }
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test] fn accessory_selection_changes_with_temperature(){
        assert_eq!(candidates(2,false).len(),1);
        assert_eq!(candidates(2,true).len(),3);
    }
    #[test] fn authored_bundle_is_readable_by_the_real_game_loader(){
        let Ok(base)=std::env::var("OPENOMSI_WINTER_TEST_ASSETS") else{return};
        let root=std::path::Path::new(&base).join("bcs-assets").join("WinterClothes");
        for name in [
            "elvs_hooded_sweat_jacket1","toigo_fisherman_sweater","mindfront_knitted_sweater_01",
            "elvs_slouchy_beanie1","mindfront_knitted_hat_01","toigo_gloves_short","toigo_gloves_medium"
        ] {
            let art=load_asset(&root,name).unwrap_or_else(|error|panic!("{name}: {error}"));
            assert!(!art.base.positions.is_empty(),"{name}: no geometry");
            assert_eq!(art.base.positions.len(),art.influences.len(),"{name}: missing bone weights");
            assert_eq!(art.base.positions.len(),art.base.uvs.len(),"{name}: missing UVs");
            assert!(art.base.positions.iter().all(|p|p.is_finite()),"{name}: invalid geometry");
            assert!(!art.diffuse.rgba.is_empty(),"{name}: missing authentic texture");
            eprintln!("WINTER ASSET OK name={} triangles={} texture={}x{}",
                name,art.base.indices.len()/3,art.diffuse.width,art.diffuse.height);
        }
    }
    #[test] fn all_model_names_are_safe(){
        for n in candidates(10,true){assert!(allowed_texture_name(n));}
        assert!(!allowed_texture_name("../evil.png"));
    }
}
