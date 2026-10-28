//! Geometry shared by the passenger camera and the controls in the bus it shows.
use glam::{DVec3, Mat4, Vec3};

#[derive(Clone, Copy, Debug)]
pub(crate) struct BodyPose {
    pub position: DVec3,
    pub rotation: Mat4,
}
impl BodyPose {
    pub(crate) fn of(v: &omsi_sim::VehicleInstance) -> Self {
        Self {
            position: v.position,
            rotation: v.body_rotation(),
        }
    }
    pub(crate) fn remap_ray(self, current: Self, origin: DVec3, direction: Vec3) -> (DVec3, Vec3) {
        let inverse = self.rotation.inverse();
        let local = inverse.transform_point3((origin - self.position).as_vec3());
        (
            current.position + current.rotation.transform_point3(local).as_dvec3(),
            current
                .rotation
                .transform_vector3(inverse.transform_vector3(direction))
                .normalize_or_zero(),
        )
    }
}

/// The view and bus pose used for one picture. The walking tick may overwrite
/// `App::camera` with the avatar's eyes before hover runs, so a control ray must
/// keep the camera together with the body pose it was drawn against.
#[derive(Clone, Copy, Debug)]
pub(crate) struct PresentedView {
    pub camera: omsi_render::Camera,
    pub body: BodyPose,
}
impl PresentedView {
    pub(crate) fn cursor_ray(
        self,
        current: BodyPose,
        ray: impl FnOnce(&omsi_render::Camera) -> (DVec3, Vec3, f32),
    ) -> (DVec3, Vec3, f32) {
        let (origin, direction, spread) = ray(&self.camera);
        let (origin, direction) = self.body.remap_ray(current, origin, direction);
        (origin, direction, spread)
    }
}

/// Carry a view once from the frame that produced it. At high refresh rates there
/// can be several pictures between people's ticks; reusing the simulation frame
/// for each picture would apply the same chassis displacement repeatedly.
pub(crate) fn carry_camera(cam: &mut omsi_render::Camera, simulated: BodyPose,
    current: BodyPose, previous: Option<PresentedView>) {
    let old = previous.filter(|p| p.camera.position == cam.position
        && p.camera.yaw == cam.yaw && p.camera.pitch == cam.pitch && p.camera.roll == cam.roll)
        .map_or(simulated, |p| p.body);
    let (at, forward) = old.remap_ray(current, cam.position, cam.forward());
    let up = current.rotation.transform_vector3(old.rotation.inverse().transform_vector3(cam.up()));
    let right = Vec3::new(forward.y, -forward.x, 0.0).normalize_or_zero();
    let level_up = right.cross(forward);
    cam.position = at;
    cam.yaw = forward.x.atan2(forward.y).to_degrees();
    cam.pitch = forward.z.clamp(-1.0, 1.0).asin().to_degrees();
    cam.roll = up.dot(right).atan2(up.dot(level_up)).to_degrees();
}

pub(crate) fn camera_pose(
    v: &omsi_sim::VehicleInstance,
    camera: &omsi_vehicle::Camera,
    look: (f32, f32),
    initial: (f32, f32),
) -> (DVec3, f32, f32, f32) {
    let mut local = camera.clone();
    local.yaw += (look.0 - initial.0 + 180.0).rem_euclid(360.0) - 180.0;
    local.pitch = (local.pitch + look.1 - initial.1).clamp(-85.0, 85.0);
    v.camera_world_full(&local)
}
/// F1 is the vehicle's driver camera bank. F2 is the passenger camera bank.
/// Do not mix the two: the ticket counter next to the driver can be invisible from F2.
pub(crate) fn camera_choices(def: &omsi_vehicle::Vehicle, driver: bool) -> Vec<omsi_vehicle::Camera> {
    if driver { def.cameras_driver.clone() } else { def.cameras_pax.clone() }
}
pub(crate) fn ai_mesh_visible(script_visible: bool, viewpoint: i32, inside: bool) -> bool {
    script_visible && (viewpoint == 0 || viewpoint & if inside { 2 } else { 4 } != 0)
}
pub(crate) fn passenger_event(event: &str) -> bool {
    let e: String = event
        .chars()
        .filter(char::is_ascii_alphanumeric)
        .map(|c| c.to_ascii_lowercase())
        .collect();
    if [
        "reset", "quitt", "loesch", "cancel", "desliga", "desativ", "limpar", "apagar",
    ]
    .iter()
    .any(|s| e.contains(s))
    {
        return false;
    }
    [
        "haltewunsch",
        "kinderwagenwunsch",
        "stoprequest",
        "stopbell",
        "campainha",
        "solicitaparada",
        "solicitacaoparada",
        "solicitacaodeparada",
        "klingel",
    ]
    .iter()
    .any(|s| e.contains(s))
        || matches!(e.as_str(), "intstop" | "intparada" | "parada")
}

pub(crate) fn recognized_bells(v: &omsi_sim::VehicleInstance) -> usize {
    std::iter::once(&v.ty)
        .chain(v.trailers.iter().map(|t| &t.ty))
        .map(|ty| {
            ty.model
                .meshes
                .iter()
                .filter(|m| m.mouse_event.as_deref().is_some_and(passenger_event))
                .count()
        })
        .sum()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    fn vehicle() -> omsi_sim::VehicleInstance {
        let ty = omsi_sim::VehicleType {
            def: Default::default(),
            model: Default::default(),
            model_dir: Default::default(),
            program: Default::default(),
            meshes: Vec::new(),
            lod_meshes: Vec::new(),
            paint_schemes: Vec::new(),
            texchanges: Vec::new(),
            wheel_meshes: Vec::new(),
            suspension_axles: Vec::new(),
            missing_packs: Vec::new(),
            mesh_bounds: Vec::new(),
            mesh_boxes: Vec::new(),
        };
        omsi_sim::VehicleInstance::new(Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
    }
    #[test]
    fn selected_view_keeps_its_authored_eye_when_bus_moves_and_turns() {
        let mut v = vehicle();
        let cam = omsi_vehicle::Camera {
            pos: [-0.7, 4.0, 1.8],
            fov: 60.0,
            yaw: 35.0,
            ..Default::default()
        };
        for (position, heading) in [
            (DVec3::new(25.0, 10.0, 1.0), 0.0),
            (DVec3::new(29.0, 12.0, 1.0), 90.0),
        ] {
            v.position = position;
            v.heading = heading;
            let actual = camera_pose(&v, &cam, (179.0, 0.0), (179.0, 0.0));
            let expected = v.camera_world_full(&cam);
            assert!((actual.0 - expected.0).length() < 1e-6);
            assert!((actual.1 - expected.1).abs() < 1e-5);
            let rear = position
                + v.body_rotation()
                    .transform_point3(Vec3::new(0.0, -4.0, 1.2))
                    .as_dvec3();
            assert!((actual.0 - rear).length() > 7.0);
        }
    }
    #[test]
    fn look_across_north_adds_two_degrees() {
        let v = vehicle();
        let cam = omsi_vehicle::Camera {
            fov: 60.0,
            yaw: 20.0,
            ..Default::default()
        };
        let a = camera_pose(&v, &cam, (1.0, 12.0), (359.0, 10.0));
        let e = v.camera_world_full(&omsi_vehicle::Camera {
            yaw: 22.0,
            pitch: 2.0,
            ..cam
        });
        assert!((a.1 - e.1).abs() < 0.001 && (a.2 - e.2).abs() < 0.001);
    }
    #[test]
    fn coop_passenger_camera_is_not_carried_twice_between_simulation_ticks() {
        let mut v = vehicle();
        let simulated = BodyPose::of(&v);
        let local_eye = DVec3::new(0.5, 2.0, 1.7);
        let mut cam = omsi_render::Camera { position: local_eye, yaw: 0.0,
            pitch: 0.0, roll: 0.0, fov_deg: 60.0, near: 0.1, far: 1000.0 };
        v.position.x = 10.0;
        let first = BodyPose::of(&v);
        carry_camera(&mut cam, simulated, first, None);
        assert!((cam.position - (v.position + local_eye)).length() < 1e-5);
        let previous = PresentedView { camera: cam, body: first };
        v.position.x = 12.0;
        let second = BodyPose::of(&v);
        carry_camera(&mut cam, simulated, second, Some(previous));
        assert!((cam.position - (v.position + local_eye)).length() < 1e-5);
        let previous = PresentedView { camera: cam, body: second };
        carry_camera(&mut cam, simulated, second, Some(previous));
        assert!((cam.position - (v.position + local_eye)).length() < 1e-5);
    }
    #[test]
    fn visible_ray_follows_the_same_button_in_the_simulation() {
        let mut v = vehicle();
        v.position = DVec3::new(100.0, 20.0, 2.0);
        v.heading = 7.0;
        let drawn = BodyPose::of(&v);
        let eye = Vec3::new(0.0, 0.0, 1.5);
        let button = Vec3::new(0.6, 2.0, 1.5);
        let origin = drawn.position + drawn.rotation.transform_point3(eye).as_dvec3();
        let dir = drawn.rotation.transform_vector3((button - eye).normalize());
        v.position += DVec3::new(1.5, 0.1, 0.1);
        v.heading = 11.0;
        let (o, d) = drawn.remap_ray(BodyPose::of(&v), origin, dir);
        let target = v.position + v.body_rotation().transform_point3(button).as_dvec3();
        assert!(d.as_dvec3().cross((target - o).normalize()).length() < 1e-5);
        assert!(dir.as_dvec3().cross((target - origin).normalize()).length() > 0.1);
    }
    #[test]
    fn ai_passenger_choices_exclude_driver_cameras() {
        let driver = omsi_vehicle::Camera {
            pos: [-0.8, 4.5, 1.7],
            ..Default::default()
        };
        let pax = omsi_vehicle::Camera {
            pos: [0.8, -2.0, 1.2],
            ..Default::default()
        };
        let mut d = omsi_vehicle::Vehicle {
            cameras_driver: vec![driver.clone()],
            cameras_pax: vec![pax.clone()],
            ..Default::default()
        };
        assert_eq!(camera_choices(&d, false), vec![pax.clone()]);
        assert_eq!(camera_choices(&d, true), vec![driver]);
        d.cameras_pax.clear();
        assert!(camera_choices(&d, false).is_empty());
    }
    #[test]
    fn entering_and_leaving_ai_changes_the_viewpoint_mask() {
        assert!(ai_mesh_visible(true, 2, true));
        assert!(!ai_mesh_visible(true, 4, true));
        assert!(!ai_mesh_visible(true, 2, false));
        assert!(ai_mesh_visible(true, 4, false));
        for inside in [false, true] {
            assert!(ai_mesh_visible(true, 0, inside));
            assert!(ai_mesh_visible(true, 6, inside));
            assert!(!ai_mesh_visible(false, 6, inside));
        }
    }
    #[test]
    fn only_common_passenger_bells_are_permitted() {
        for e in [
            "int_haltewunsch",
            "door_haltewunsch",
            "int_solicita_parada",
            "int_solicitaparada",
            "int_Parada",
            "stop_request",
            "stop_bell",
            "campainha",
            "klingel",
        ] {
            assert!(passenger_event(e), "{e}");
        }
        for e in [
            "int_light",
            "engine_start",
            "door_driver",
            "haltewunsch_reset",
            "campainha_desliga",
        ] {
            assert!(!passenger_event(e), "{e}");
        }
    }

    #[test]
    fn bells_in_the_rear_section_count_as_available_passenger_controls() {
        let mut v = vehicle();
        let mut rear = omsi_sim::vehicle::TrailerPart::new(v.ty.clone(), &v.ty, &v.ty.program, 0);
        std::sync::Arc::make_mut(&mut rear.ty)
            .model
            .meshes
            .push(omsi_model::MeshDef {
                mouse_event: Some("int_haltewunsch".into()),
                ..Default::default()
            });
        v.trailers.push(rear);
        assert_eq!(recognized_bells(&v), 1);
    }
}
