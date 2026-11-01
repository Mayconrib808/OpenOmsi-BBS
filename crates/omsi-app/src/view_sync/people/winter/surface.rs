//! Bind the authored cloth to the *loaded human surface*, including its real weights.
//!
//! A bone-only mannequin fit can pass edge-length checks and still float 20 cm
//! outside a stock passenger. Each cloth vertex now follows a barycentric point
//! on a nearby body triangle plus a small fabric clearance. Sampling the body's
//! already posed mesh also follows seated poses, IK and finger adjustments exactly.
use super::{Art, fit};
use glam::Vec3;
use omsi_sim::human::{self, HumanType, Influence};

pub(super) struct Fitted {
    pub rest: Vec<Vec3>,
    links: Vec<Link>,
    pub max_correction: f32,
}

#[derive(Clone, Copy)]
struct Link {
    mesh: usize,
    vertices: [usize; 3],
    bary: Vec3,
    clearance: f32,
}

impl Link {
    fn sample(&self, positions: &[Vec3], normals: &[Vec3]) -> Option<Vec3> {
        let [a,b,c] = self.vertices;
        let [wa,wb,wc] = self.bary.to_array();
        let p = *positions.get(a)?*wa + *positions.get(b)?*wb + *positions.get(c)?*wc;
        let n = *normals.get(a)?*wa + *normals.get(b)?*wb + *normals.get(c)?*wc;
        if !p.is_finite() || !n.is_finite() || n.length_squared()<1.0e-8 {return None;}
        Some(p+n.normalize()*self.clearance)
    }
}

impl Fitted {
    pub fn pose(&self, skins: &[(Vec<Vec3>,Vec<Vec3>)], out: &mut Vec<Vec3>) -> bool {
        out.clear();
        out.reserve(self.links.len());
        for link in &self.links {
            let Some((positions,normals))=skins.get(link.mesh) else {out.clear();return false;};
            let Some(p)=link.sample(positions,normals) else {out.clear();return false;};
            out.push(p);
        }
        fit::triangles_safe(&self.rest,out)
    }
}

struct Triangle {
    link: Link,
    p: [Vec3;3],
    min: Vec3,
    max: Vec3,
}

// Bounded AABB tree: fitting is done once per human type/garment, never every
// animation frame. A query visits nearby triangles rather than all body vertices.
struct Tree {
    min: Vec3,
    max: Vec3,
    children: Option<(Box<Tree>,Box<Tree>)>,
    triangles: Vec<Triangle>,
}
impl Tree {
    fn new(mut triangles: Vec<Triangle>) -> Self {
        let min=triangles.iter().fold(Vec3::splat(f32::MAX),|v,t|v.min(t.min));
        let max=triangles.iter().fold(Vec3::splat(f32::MIN),|v,t|v.max(t.max));
        if triangles.len()<=12 {return Self{min,max,children:None,triangles};}
        let size=max-min;
        let axis=if size.x>=size.y && size.x>=size.z {0}else if size.y>=size.z {1}else{2};
        triangles.sort_by(|a,b|(a.min[axis]+a.max[axis]).total_cmp(&(b.min[axis]+b.max[axis])));
        let right=triangles.split_off(triangles.len()/2);
        Self{min,max,children:Some((Box::new(Self::new(triangles)),Box::new(Self::new(right)))),triangles:Vec::new()}
    }
    fn distance_sq(&self,p:Vec3)->f32 {(p-p.clamp(self.min,self.max)).length_squared()}
    fn nearest(&self,p:Vec3,best:&mut f32,link:&mut Option<Link>) {
        if self.distance_sq(p)>*best {return;}
        if let Some((a,b))=&self.children {
            if a.distance_sq(p)<=b.distance_sq(p) {a.nearest(p,best,link);b.nearest(p,best,link);}
            else {b.nearest(p,best,link);a.nearest(p,best,link);}
        } else {
            for t in &self.triangles {
                let bary=closest_barycentric(p,t.p);
                let q=t.p[0]*bary.x+t.p[1]*bary.y+t.p[2]*bary.z;
                let distance=(p-q).length_squared();
                if distance<*best {*best=distance;*link=Some(Link{bary,..t.link});}
            }
        }
    }
}

// Closest point on a nondegenerate triangle, expressed as barycentric weights.
fn closest_barycentric(p:Vec3,t:[Vec3;3])->Vec3 {
    let [a,b,c]=t;
    let ab=b-a;let ac=c-a;let ap=p-a;
    let d1=ab.dot(ap);let d2=ac.dot(ap);
    if d1<=0.0&&d2<=0.0{return Vec3::X;}
    let bp=p-b;let d3=ab.dot(bp);let d4=ac.dot(bp);
    if d3>=0.0&&d4<=d3{return Vec3::Y;}
    let vc=d1*d4-d3*d2;
    if vc<=0.0&&d1>=0.0&&d3<=0.0{let v=d1/(d1-d3);return Vec3::new(1.0-v,v,0.0);}
    let cp=p-c;let d5=ab.dot(cp);let d6=ac.dot(cp);
    if d6>=0.0&&d5<=d6{return Vec3::Z;}
    let vb=d5*d2-d1*d6;
    if vb<=0.0&&d2>=0.0&&d6<=0.0{let v=d2/(d2-d6);return Vec3::new(1.0-v,0.0,v);}
    let va=d3*d6-d5*d4;
    if va<=0.0&&d4-d3>=0.0&&d5-d6>=0.0{let v=(d4-d3)/((d4-d3)+(d5-d6));return Vec3::new(0.0,1.0-v,v);}
    let denom=1.0/(va+vb+vc);
    Vec3::new(1.0-vb*denom-vc*denom,vb*denom,vc*denom)
}

fn weight(influence:&Influence,slots:&[usize])->f32 {
    (0..usize::from(influence.n).min(4)).filter(|&i|slots.contains(&(influence.slot[i] as usize)))
        .map(|i|influence.weight[i]).sum()
}

fn surface(ty:&HumanType,kind:&str,side:Option<usize>,clearance:f32)->Result<Tree,String> {
    let slots:Vec<usize>=match kind {
        "hat"=>vec![human::slot_of(human::BONE_HEAD).unwrap()],
        "gloves"=>vec![human::hand_slot(side.unwrap_or(0))],
        "torso"=>[human::BONE_MAIN,human::BONE_HIP,human::BONE_OA_L,human::BONE_OA_R,
                  human::BONE_UA_L,human::BONE_UA_R].into_iter().filter_map(human::slot_of).collect(),
        _=>return Err("unsupported garment kind".into()),
    };
    let mut triangles=Vec::new();
    for (mesh,m) in ty.meshes.iter().enumerate() {
        for tri in m.data.indices.chunks_exact(3) {
            let vertices=[tri[0] as usize,tri[1] as usize,tri[2] as usize];
            if vertices.iter().any(|&i|i>=m.data.positions.len()||i>=m.data.normals.len()||i>=m.skin.len()){continue;}
            if vertices.iter().map(|&i|weight(&m.skin[i],&slots)).sum::<f32>()<1.5 {continue;}
            let p=vertices.map(|i|m.data.positions[i]);
            if p.iter().any(|v|!v.is_finite()) || (p[1]-p[0]).cross(p[2]-p[0]).length_squared()<1.0e-12 {continue;}
            // A hat follows the crown/hair, never the eyes, nose or chin.
            if kind=="hat" && p.iter().any(|v|v.z<ty.rig.head_top-0.17*ty.rig.scale){continue;}
            // Exclude skirt/leg triangles partly sharing the pelvis bone.
            if kind=="torso" && p.iter().any(|v|v.z<ty.rig.waist.z-0.20*ty.rig.scale){continue;}
            let min=p[0].min(p[1]).min(p[2]);let max=p[0].max(p[1]).max(p[2]);
            triangles.push(Triangle{link:Link{mesh,vertices,bary:Vec3::X,clearance},p,min,max});
            if triangles.len()>100_000{return Err("human surface exceeds fit budget".into());}
        }
    }
    if triangles.len()<4{return Err(format!("{kind}: human has no usable weighted surface"));}
    Ok(Tree::new(triangles))
}

pub(super) fn fitted(art:&Art,ty:&HumanType)->Result<Fitted,String> {
    let mut rough=fit::fitted(art,&ty.rig)?;
    let clearance=match art.kind.as_str(){"torso"=>0.018,"hat"=>0.012,"gloves"=>0.007,_=>return Err("unknown garment kind".into())}*ty.rig.scale;
    let first=surface(ty,&art.kind,Some(0),clearance)?;
    let second=if art.kind=="gloves"{Some(surface(ty,&art.kind,Some(1),clearance)?)}else{None};
    if art.kind=="hat" {
        let min=rough.iter().copied().reduce(Vec3::min).ok_or("empty hat")?;
        let max=rough.iter().copied().reduce(Vec3::max).ok_or("empty hat")?;
        let centre=(min+max)*0.5;
        let target=(first.min+first.max)*0.5;
        let scale=(first.max-first.min)/(max-min).max(Vec3::splat(0.001));
        for v in &mut rough {*v=target+(*v-centre)*scale;}
    }
    let mut out=Fitted{rest:Vec::with_capacity(rough.len()),links:Vec::with_capacity(rough.len()),max_correction:0.0};
    // Duplicate OBJ face corners/UV seams must receive exactly the same
    // attachment, otherwise equal source vertices can rip apart in a pose.
    let mut welded=std::collections::HashMap::<[u32;3],(Link,Vec3)>::new();
    for (i,p) in rough.into_iter().enumerate() {
        let source=art.base.positions[i];
        let key=source.to_array().map(|v|if v==0.0{0}else{v.to_bits()});
        if let Some(&(link,rest))=welded.get(&key){out.links.push(link);out.rest.push(rest);continue;}
        let tree=if source.x>=0.0{second.as_ref().unwrap_or(&first)}else{&first};
        let mut best=(0.45*ty.rig.scale).powi(2);let mut link=None;
        tree.nearest(p,&mut best,&mut link);
        let link=link.ok_or("garment vertex is too far from the actual human surface")?;
        let mesh=&ty.meshes[link.mesh];
        let rest=link.sample(&mesh.data.positions,&mesh.data.normals).ok_or("invalid human surface normal")?;
        out.max_correction=out.max_correction.max(best.sqrt());
        welded.insert(key,(link,rest));out.links.push(link);out.rest.push(rest);
    }
    if !fit::triangles_safe(&art.base.positions,&out.rest){return Err("surface-conformed garment has stretched triangles".into());}
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test] fn nearest_surface_preserves_barycentric_coordinates_and_fabric_clearance(){
        let body=[Vec3::ZERO,Vec3::X,Vec3::Y];
        let bary=closest_barycentric(Vec3::new(0.25,0.25,0.4),body);
        let link=Link{mesh:0,vertices:[0,1,2],bary,clearance:0.018};
        let p=link.sample(&body,&[Vec3::Z;3]).unwrap();
        assert!((p-Vec3::new(0.25,0.25,0.018)).length()<1.0e-6);
        // The earlier fit retained the 40 cm mannequin offset. Attachment to
        // the true human surface must remove it before the first GPU upload.
        assert!((p-Vec3::new(0.25,0.25,0.4)).length()>0.35);
    }
    #[test] fn sleeves_follow_the_final_skinned_body_during_arm_and_seated_motion(){
        let body=[Vec3::ZERO,Vec3::X*0.03,Vec3::Y*0.03];
        let links=body.iter().map(|&p|Link{mesh:0,vertices:[0,1,2],bary:closest_barycentric(p,body),clearance:0.018}).collect::<Vec<_>>();
        let rest=links.iter().map(|l|l.sample(&body,&[Vec3::Z;3]).unwrap()).collect();
        let fit=Fitted{rest,links,max_correction:0.0};
        for angle in [-1.5_f32,-0.4,0.0,0.8,1.5] {
            let rotation=glam::Quat::from_rotation_x(angle);
            let translation=Vec3::new(0.1,0.25,-0.5);
            let positions=body.map(|p|rotation*p+translation).to_vec();
            let normals=vec![rotation*Vec3::Z;3];
            let mut posed=Vec::new();
            assert!(fit.pose(&[(positions,normals)],&mut posed));
            for (a,b) in posed.iter().zip(&fit.rest){assert!((*a-(rotation* *b+translation)).length()<1.0e-6);}
        }
        assert!(!fit.pose(&[],&mut Vec::new()),"missing body data must hide the garment");
    }
    #[test] fn invalid_normals_or_nonfinite_body_never_reach_gpu(){
        let link=Link{mesh:0,vertices:[0,1,2],bary:Vec3::X,clearance:0.018};
        assert!(link.sample(&[Vec3::ZERO;3],&[Vec3::ZERO;3]).is_none());
        assert!(link.sample(&[Vec3::splat(f32::NAN);3],&[Vec3::Z;3]).is_none());
    }
}
