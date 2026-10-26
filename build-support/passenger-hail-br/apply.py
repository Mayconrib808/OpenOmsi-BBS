"""Apply the researched hailing presentation after the complete pinned 0.3.44 port.

Only two cosmetic source files change. Fail closed if the published baseline
drifts; do not modify route matching, passenger counts, boarding or the protocol.
"""
from pathlib import Path
import hashlib
import sys

root = Path(sys.argv[1] if len(sys.argv) > 1 else "game")
here = Path(__file__).resolve().parent
gesture = root / "crates/omsi-sim/src/human/gesture.rs"
extras = root / "crates/omsi-app/src/view_sync/people/extras.rs"

def read_checked(path, expected):
    text = path.read_text(encoding="utf-8").replace("\r\n", "\n").rstrip("\n") + "\n"
    digest = hashlib.sha256(text.encode()).hexdigest()
    assert digest == expected, f"Published hailing baseline changed: {path} ({digest})"
    return text

old_gesture = read_checked(gesture, "11111804d430ea464c06a2dfe70b60585224af275088edeeb0c1a9795bb91d6f")
text = read_checked(extras, "5642089247ca081b408a275100ed79a9b88a40fa2c25d3fc5e28ff6a566eff48")
new_gesture = (here / "gesture.rs").read_text(encoding="utf-8")
# Umbrella arm/grip IK is byte-for-byte preserved.
assert new_gesture.split("/// A Brazilian-style")[0] == old_gesture.split("/// Hail towards")[0]

def replace_once(old, new):
    global text
    assert text.count(old) == 1, f"Hailing integration anchor changed: {old[:90]}"
    text = text.replace(old, new, 1)

replace_once("point:Vec3,original:Vec3,original_nrm:Vec3,index_finger:bool",
             "original:Vec3,original_nrm:Vec3")
replace_once("pub grip:[Vec3;2],knuckle:[Vec3;2],finger_axis:[Vec3;2]",
             "pub grip:[Vec3;2]")
start = text.index("impl HandRest{\n")
end = text.index("impl State{\n", start)
text = text[:start] + """impl HandRest{
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
""" + text[end:]
replace_once("pub grip:Vec3,pub finger:f32,pub hail_side:usize",
             "pub grip:Vec3,pub hail_side:usize")
replace_once("self.hail_side=if self.arm>0.05{0}else{1};self.finger=(self.time*5.6+(p.id%53) as f32).sin()*0.20;",
             "self.hail_side=if self.arm>0.05{0}else{1};")
replace_once(""" // The arm follows the same smoothed bus direction as the head/body. A fixed
 // model +Y offset pointed behind passengers as soon as their body turned.
 accessory_hail_arm(&ty.rig,bones,side,self.look.direction(),ease,self.finger*0.6);""",
""" // Boarding request observed in Brazilian reference photographs: arm extended
 // from the actual torso, around shoulder height. Head and eyes track separately.
 accessory_hail_arm(&ty.rig,bones,side,ease,0.);""")
replace_once("""let (mut p,n)=if point&&v.index_finger{let r=glam::Quat::from_axis_angle(rest.finger_axis[side],self.finger);(rest.knuckle[side]+r*(v.point-rest.knuckle[side]),r*v.original_nrm)}else{(v.pos,v.nrm)};""",
"""// Hailing uses the model's relaxed open hand; only the umbrella hand curls.
 let (mut p,n)=if point{(v.original,v.original_nrm)}else{(v.pos,v.nrm)};""")
gesture.write_text(new_gesture, encoding="utf-8")
extras.write_text(text, encoding="utf-8")
print("Brazilian boarding request applied: extended arm, relaxed open hand, independent bounded gaze.")

