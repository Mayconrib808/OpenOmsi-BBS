//! Original, low-poly umbrella: eight fabric gores, metal ribs and hooked handle.
//! Reference photos are documented in the beta notes; no downloaded meshes/textures.
use glam::{Vec2,Vec3};
use omsi_geometry::MeshData;
use std::f32::consts::TAU;
fn tri(m:&mut MeshData,a:Vec3,b:Vec3,c:Vec3,slot:u32){
 let n=(b-a).cross(c-a).normalize_or(Vec3::Z);let base=m.positions.len() as u32;
 m.positions.extend([a,b,c]);m.normals.extend([n;3]);
 m.uvs.extend([Vec2::ZERO,Vec2::X,Vec2::Y]);m.indices.extend([base,base+1,base+2]);
 let start=m.indices.len() as u32-3;
 if let Some((_,count,s))=m.ranges.last_mut(){if *s==slot{*count+=3;return}}
 m.ranges.push((start,3,slot));
}
fn tube(m:&mut MeshData,a:Vec3,b:Vec3,r:f32,slot:u32){
 let axis=(b-a).normalize_or(Vec3::Z);let u=axis.cross(Vec3::X).normalize_or(Vec3::Y)*r;let v=axis.cross(u).normalize_or(Vec3::X)*r;
 for i in 0..6 {let f=|k:usize|{let t=k as f32*TAU/6.;u*t.cos()+v*t.sin()};let x=f(i);let y=f(i+1);
 tri(m,a+x,b+x,b+y,slot);tri(m,a+x,b+y,a+y,slot);}
}
fn roof(r:f32,angle:f32)->Vec3{
 // A convex sewn panel, slightly scalloped between rib tips.
 let sag=(angle*4.).sin().abs()*0.018*r;
 Vec3::new(0.55*r*angle.cos(),0.55*r*angle.sin(),-0.27*r*r-sag)
}
pub fn canopy()->MeshData{
 let mut m=MeshData::default();
 for gore in 0..8 {for ring in 0..3{for j in 0..4{
 let a=(gore as f32+j as f32/4.)*TAU/8.;let b=a+TAU/32.;let r0=ring as f32/3.;let r1=(ring+1) as f32/3.;
 tri(&mut m,roof(r0,a),roof(r1,a),roof(r1,b),gore);if ring>0{tri(&mut m,roof(r0,a),roof(r1,b),roof(r0,b),gore);}
 }}}
 for rib in 0..8{let a=rib as f32*TAU/8.;for ring in 0..3{let p=roof(ring as f32/3.,a)-Vec3::Z*0.004;let q=roof((ring+1) as f32/3.,a)-Vec3::Z*0.004;tube(&mut m,p,q,0.003,8);}}
 m
}
pub fn shaft()->MeshData{
 let mut m=MeshData::default();tube(&mut m,Vec3::new(0.,0.,-0.055),Vec3::new(0.,0.,0.805),0.006,0);
 tube(&mut m,Vec3::new(0.,0.,-0.055),Vec3::new(0.,0.,-0.13),0.013,1);
 let f=|i:usize|{let a=i as f32*std::f32::consts::PI/12.;Vec3::new(0.045-0.045*a.cos(),0.,-0.13-0.045*a.sin())};
 for i in 0..12{tube(&mut m,f(i),f(i+1),0.013,1);}m
}

/// Shared streaks slide out along the canopy's parabola using radial/vertical scaling.
pub fn runoff()->MeshData{
 let mut m=MeshData::default();
 for i in 0..8{let a=(i as f32+0.43)*TAU/8.;tube(&mut m,roof(0.88,a),roof(1.,a),0.003,0);}
 m
}
/// One shared drop mesh; only instance transforms move each frame.
pub fn drips()->MeshData{
 let mut m=MeshData::default();
 for i in 0..5{let a=i as f32*TAU/5.;let p=Vec3::new(a.cos()*0.018,a.sin()*0.018,-(i as f32)*0.035);tube(&mut m,p,p+Vec3::Z*0.018,0.0035,0);}
 m
}

/// The canopy's own metre UVs let the native window-rain shader draw beads and runners
/// on fabric without triangle-local seams or the old eight moving grey rods.
pub fn rain_canopy()->MeshData{
 let mut m=MeshData::default();
 for gore in 0..8{for ring in 0..3{for j in 0..4{
  let a=(gore as f32+j as f32/4.)*TAU/8.;let b=a+TAU/32.;let r0=ring as f32/3.;let r1=(ring+1)as f32/3.;
  tri(&mut m,roof(r0,a),roof(r1,a),roof(r1,b),0);if ring>0{tri(&mut m,roof(r0,a),roof(r1,b),roof(r0,b),0);}
 }}}
 m.uvs=m.positions.iter().map(|p|p.truncate()).collect();m
}
fn bead(m:&mut MeshData,p:Vec3,r:f32){
 let top=p+Vec3::Z*r*1.7;let bottom=p-Vec3::Z*r*0.9;
 for k in 0..6{let a=k as f32*TAU/6.;let b=(k+1)as f32*TAU/6.;let x=p+Vec3::new(a.cos()*r,a.sin()*r,0.);let y=p+Vec3::new(b.cos()*r,b.sin()*r,0.);tri(m,top,x,y,0);tri(m,bottom,y,x,0);}
}
pub fn edge_drops()->MeshData{
 let mut m=MeshData::default();
 for i in 0..40{let angle=(i as f32+0.31)*TAU/40.;let p=roof(1.,angle)-Vec3::Z*(0.015+(i%5)as f32*0.025);bead(&mut m,p,0.0035+(i%3)as f32*0.001);}
 m
}
pub fn closed_drops()->MeshData{
 let mut m=MeshData::default();
 for i in 0..4{let a=i as f32*TAU/4.;bead(&mut m,Vec3::new(a.cos()*0.006,a.sin()*0.006,i as f32*0.012),0.004);}
 m
}
