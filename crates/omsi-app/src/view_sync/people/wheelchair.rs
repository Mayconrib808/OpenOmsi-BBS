//! Wheelchair and rider share the exact same anchor and orientation. The rider
//! always uses the seated rig, including when original walking animation is selected.
use super::*;
use glam::{Affine3A, DVec2, Quat};
use omsi_sim::human::{Pose, SLOTS};
use omsi_sim::people::wheelchair::{EquipmentPose, geometry};

pub(super) fn bones(
    p: &PersonFrame,
    ty: &HumanType,
    st: &mut PersonView,
    dt: f32,
) -> [Affine3A; SLOTS] {
    let chair = p.wheelchair.unwrap();
    // Contact point sweeps from rear/top to front/top on the handrim, followed
    // by a lower, outward recovery arc. Each side has its own drive phase.
    let grips = omsi_sim::people::wheelchair::pose::hand_targets(chair);
    let grips = match st.wheelchair_grips {
        Some(old) => std::array::from_fn(|i| old[i].lerp(grips[i], 1.0 - (-dt / 0.08).exp())),
        None => grips,
    };
    st.wheelchair_grips = Some(grips);
    let origin = match p.place {
        Place::Ground => p.position,
        Place::Bus(_, l) => l.as_dvec3(),
    };
    let floor = |_: DVec2| Some(origin.z + 0.16);
    let mut input=omsi_sim::people::wheelchair::pose::input(&ty.rig,chair,origin,
        if matches!(p.place,Place::Ground){p.heading}else{p.lheading},
        if matches!(p.place,Place::Ground){0}else{1},grips,&floor);
    st.clip_t += dt;
    // Fade the idle motion under active pushes. Apply it before solving the grips.
    let lean = super::avatars::seated_lean(p.id, ty, st.clip_t);
    input.grip_lean += (1.0 - chair.moving.clamp(0.0, 1.0)) * (3.0 + lean);
    if st.procedural.is_none() {
        let mut pose = Pose::new(p.id);
        for _ in 0..30 {
            pose.advance(&ty.rig, &input, 0.05);
        }
        st.procedural = Some(pose);
    }
    let pose = st.procedural.as_mut().unwrap();
    pose.advance(&ty.rig, &input, dt);
    let posed = pose.bones(&ty.rig);
    st.ankles = posed.ankle;
    st.glide = Vec3::ZERO;
    st.glide_world = DVec3::ZERO;
    posed.bones
}

#[derive(Default)]
pub(super) struct Renders {
    meshes: Option<[MeshId; 7]>,
    materials: Vec<MaterialId>,
    chairs: HashMap<u32, [usize; 5]>,
    spare: Vec<[usize; 5]>,
    equipment: HashMap<u32, [usize;4]>,
    equipment_spare: Vec<[usize;4]>,
}
impl Renders {
    fn ensure(&mut self, r: &Renderer, s: &mut Scene) {
        if self.meshes.is_some() {
            return;
        }
        self.materials = [
            [0.025, 0.027, 0.032, 1.0],
            [0.56, 0.59, 0.62, 1.0],
            [0.045, 0.075, 0.115, 1.0],
            [0.025, 0.12, 0.19, 1.0],
            [0.96, 0.68, 0.06, 1.0],
            [0.20, 0.23, 0.25, 1.0],
        ]
        .into_iter()
        .map(|c| {
            r.add_material_extra(
                s,
                None,
                AlphaMode::Opaque,
                c,
                false,
                None,
                None,
                None,
                None,
                [0.0; 3],
                omsi_render::MaterialExtra {
                    solid: true,
                    ..Default::default()
                },
            )
        })
        .collect();
        self.meshes = Some(
            [
                geometry::chair_frame(),
                geometry::wheel(-1.0),
                geometry::wheel(1.0),
                geometry::caster(),
                geometry::platform(),
                geometry::lift_arm(),
                geometry::lift_arm(),
            ]
            .map(|m| r.add_mesh(s, &m)),
        );
    }
    pub fn draw_chair(
        &mut self,
        p: &PersonFrame,
        at: DVec3,
        xf: Mat4,
        r: &Renderer,
        s: &mut Scene,
    ) {
        if p.wheelchair.is_none() {
            return;
        }
        self.ensure(r, s);
        let meshes = self.meshes.unwrap();
        let _instances = if let Some(i) = self.chairs.get(&p.id) {
            *i
        } else {
            let i = self.spare.pop().unwrap_or_else(|| {
                [0, 1, 2, 3, 3].map(|k| {
                    r.add_instance(s, meshes[k], at, Mat4::IDENTITY, self.materials.clone())
                })
            });
            self.chairs.insert(p.id, i);
            i
        };
        self.follow_chair(p, at, xf, r, s);
    }
    pub fn follow_chair(&self, p: &PersonFrame, at: DVec3, xf: Mat4, r: &Renderer, s: &mut Scene) {
        let (Some(chair), Some(instances)) = (p.wheelchair, self.chairs.get(&p.id)) else {
            return;
        };
        r.set_transform(s, instances[0], at, xf);
        for side in 0..2 {
            let x = if side == 0 { -0.33 } else { 0.33 };
            let pivot = Vec3::new(x, 0.0, 0.30);
            r.set_transform(
                s,
                instances[side + 1],
                at + xf.transform_vector3(pivot).as_dvec3(),
                xf * Mat4::from_rotation_x(-chair.wheels[side]),
            );
            let pivot = Vec3::new(if side == 0 { -0.255 } else { 0.255 }, 0.46, 0.075);
            r.set_transform(
                s,
                instances[side + 3],
                at + xf.transform_vector3(pivot).as_dvec3(),
                xf * Mat4::from_rotation_x(-chair.wheels[side] * 4.0),
            );
        }
        for &i in instances {
            r.set_params(s, i, &[], p.hidden != Some(true), &[]);
            r.set_interior(s, i, p.interior * 0.5);
        }
    }
    pub fn draw_equipments(&mut self,list:&[EquipmentPose],r:&Renderer,s:&mut Scene){
        let gone:Vec<_>=self.equipment.keys().copied().filter(|owner|!list.iter().any(|e|e.owner==*owner)).collect();
        for owner in gone {if let Some(instances)=self.equipment.remove(&owner){
            for &i in &instances{r.set_params(s,i,&[],false,&[]);}self.equipment_spare.push(instances);
        }}
        for &e in list{self.draw_equipment(e,r,s);}
    }
    fn draw_equipment(&mut self,e:EquipmentPose,r:&Renderer,s:&mut Scene){
        self.ensure(r, s);
        let meshes = self.meshes.unwrap();
        let instances = *self.equipment.entry(e.owner).or_insert_with(|| self.equipment_spare.pop().unwrap_or_else(|| {
            [4, 5, 6, 4].map(|k| {
                r.add_instance(
                    s,
                    meshes[k],
                    e.position,
                    Mat4::IDENTITY,
                    self.materials.clone(),
                )
            })
        }));
        let hinge = e.position;
        let slope = if e.ramp {
            e.side * ((e.floor_height - e.ground_height) / e.length).atan()
        } else {
            0.0
        };
        let fold = Mat4::from_rotation_y(
            -e.side * (1.0 - e.deployment) * std::f32::consts::FRAC_PI_2 + slope * e.deployment,
        );
        let length = if e.ramp {
            e.length.hypot(e.floor_height - e.ground_height)
        } else {
            e.length
        };
        let center = hinge
            + (e.rotation * fold)
                .transform_vector3(Vec3::X * (e.side * length * 0.5))
                .as_dvec3();
        let xf = e.rotation * fold * Mat4::from_scale(Vec3::new(length, e.width, 1.0));
        r.set_transform(s, instances[0], center, xf);
        r.set_params(s, instances[0], &[], true, &[]);
        let bridge_rotation = e.rotation
            * Mat4::from_rotation_y(
                e.side * (1.0 - e.bridge_deployment) * std::f32::consts::FRAC_PI_2,
            );
        let bridge_origin = hinge
            + e.rotation
                .transform_vector3(Vec3::Z * (e.floor_height - e.height + 0.005))
                .as_dvec3();
        let bridge_center = bridge_origin
            + bridge_rotation
                .transform_vector3(Vec3::X * (-e.side * 0.5))
                .as_dvec3();
        r.set_transform(
            s,
            instances[3],
            bridge_center,
            bridge_rotation * Mat4::from_scale(Vec3::new(1.0, e.width, 0.5)),
        );
        r.set_params(
            s,
            instances[3],
            &[],
            !e.ramp && e.bridge_deployment > 0.001,
            &[],
        );
        for side in 0..2 {
            let y = if side == 0 {
                -e.width * 0.42
            } else {
                e.width * 0.42
            };
            let local = Vec3::new(-e.side * e.length * 0.45, y, 0.03);
            let a = center + e.rotation.transform_vector3(local).as_dvec3();
            let b = hinge
                + e.rotation
                    .transform_vector3(Vec3::new(0.0, y, e.floor_height - e.height + 0.10))
                    .as_dvec3();
            let delta = (b - a).as_vec3();
            let arm = Mat4::from_quat(Quat::from_rotation_arc(
                Vec3::Z,
                delta.normalize_or(Vec3::Z),
            )) * Mat4::from_scale(Vec3::new(1.0, 1.0, delta.length().max(0.01)));
            r.set_transform(s, instances[side + 1], a, arm);
            r.set_params(s, instances[side + 1], &[], !e.ramp, &[]);
        }
    }
    pub fn retain(&mut self, people: &[PersonFrame], r: &Renderer, s: &mut Scene) {
        let gone: Vec<_> = self
            .chairs
            .keys()
            .copied()
            .filter(|id| !people.iter().any(|p| p.id == *id && p.wheelchair.is_some()))
            .collect();
        for id in gone {
            if let Some(i) = self.chairs.remove(&id) {
                for k in i {
                    r.set_params(s, k, &[], false, &[]);
                }
                self.spare.push(i);
            }
        }
    }
}
