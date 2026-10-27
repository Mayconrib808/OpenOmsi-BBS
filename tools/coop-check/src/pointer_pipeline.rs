//! A lightweight window shell around the production App picker and camera maths.
use crate::coop_view::{BodyPose, PresentedView};
use crate::{Camera, DVec3, Vec3};
use omsi_sim::people::BusId;
use omsi_sim::VehicleInstance;

struct Config {
    width: u32,
    height: u32,
}
struct Surface {
    config: Config,
}
struct Gfx {
    surface: Option<Surface>,
}
struct Coop {
    drawn: Option<(BusId, PresentedView)>,
}
struct Net {
    coop: Coop,
}
struct App {
    camera: Option<Camera>,
    selected_camera: Option<Camera>,
    view: String,
    cursor: (f32, f32),
    vehicle: VehicleInstance,
    bus: BusId,
    gfx: Gfx,
    net: Net,
}
impl App {
    fn foot_bus(&self) -> Option<BusId> {
        Some(self.bus)
    }
    fn coop_sync_ticket_blocks(&mut self) {}
    // Renderer and walking stages are represented only by their camera writes.
    // The actual camera pose/projection is computed below by production maths.
    fn coop_camera_frame(&mut self) {
        if let Some(camera) = self.selected_camera {
            self.camera = Some(camera);
        }
    }
    fn coop_vehicle(&self, bus: BusId) -> Option<&VehicleInstance> {
        (self.bus == bus).then_some(&self.vehicle)
    }
    fn cockpit_cursor_ray(&self, camera: &Camera, size: (u32, u32)) -> (DVec3, Vec3, f32) {
        let (o, d) = crate::cursor_ray(
            camera,
            self.cursor.0,
            self.cursor.1,
            size.0 as f32,
            size.1 as f32,
        );
        (o, d, crate::pixel_angle(camera, size.1 as f32) * 6.0)
    }
    // Also supports compiling the previous routing method to reproduce the bug:
    // the walking stage has put the avatar's eyes here, unlike the shown camera.
    fn cursor_ray_now(&self) -> Option<(DVec3, Vec3, f32)> {
        let s = self.gfx.surface.as_ref()?;
        Some(self.cockpit_cursor_ray(self.camera.as_ref()?, (s.config.width, s.config.height)))
    }
}
include!(concat!(env!("OUT_DIR"), "/coop_cursor.rs"));

mod paper {
    use super::*;
    use glam::Mat4;
    use std::path::PathBuf;
    include!(concat!(env!("OUT_DIR"), "/ticket_hit.rs"));
    pub fn blocks(data: omsi_geometry::MeshData, local: Mat4) -> TicketBlocks {
        TicketBlocks {
            made_for: Default::default(),
            blocks: vec![(0, local, 3, data)],
        }
    }
}

struct Fixture(std::path::PathBuf);
impl Fixture {
    fn new() -> Self {
        static NEXT: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(0);
        let id = NEXT.fetch_add(1, std::sync::atomic::Ordering::Relaxed);
        let root =
            std::env::temp_dir().join(format!("omsi-shown-camera-{}-{id}", std::process::id()));
        std::fs::create_dir_all(root.join("model")).unwrap();
        // Three actual controls across the cashier's desk: ticket, coin, flap.
        let mut cfg = String::new();
        for (n, x, event) in [
            (0, -0.45, "ticket_key_3"),
            (1, 0.0, "cashdesk_changer_1_00"),
            (2, 0.45, "cp_portinha"),
        ] {
            let mut obj = String::new();
            for (a, b) in [(-0.12, -0.12), (0.12, -0.12), (0.12, 0.12), (-0.12, 0.12)] {
                obj.push_str(&format!("v {} {} -2\n", x + a, 1.25 + b));
            }
            obj.push_str("f 1 2 3 4\n");
            std::fs::write(root.join(format!("model/{n}.obj")), obj).unwrap();
            cfg.push_str(&format!("[mesh]\n{n}.obj\n[mouseevent]\n{event}\n"));
        }
        std::fs::write(root.join("model/model.cfg"), cfg).unwrap();
        std::fs::write(
            root.join("fixture.bus"),
            "[friendlyname]\nTest\nCashier camera\nTest\n[model]\nmodel/model.cfg\n",
        )
        .unwrap();
        Self(root)
    }
    fn vehicle(&self) -> VehicleInstance {
        let ty = omsi_sim::VehicleType::load(&self.0, &self.0.join("fixture.bus")).unwrap();
        VehicleInstance::new(
            std::sync::Arc::new(ty),
            omsi_sim::VehicleHost::new(Default::default()),
        )
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn setup(v: VehicleInstance, shown: Camera) -> App {
    let mut app = App {
        // The avatar is at another place and looks backwards, as in the report.
        camera: Some(Camera {
            position: v.position + DVec3::new(0.8, -4.0, 1.65),
            yaw: 180.0,
            pitch: 0.0,
            ..shown
        }),
        selected_camera: Some(shown),
        view: "foot".into(),
        cursor: (0.0, 0.0),
        vehicle: v,
        bus: BusId::Ai(17),
        gfx: Gfx {
            surface: Some(Surface {
                config: Config {
                    width: 1920,
                    height: 1080,
                },
            }),
        },
        net: Net {
            coop: Coop { drawn: None },
        },
    };
    let avatar = app.camera;
    // Actual production capture: the selected cashier view is applied first.
    app.coop_drawn_view();
    assert_eq!(
        app.net.coop.drawn.unwrap().1.camera.position,
        shown.position
    );
    // Next walking tick overwrites the mutable camera before a hover/press.
    app.camera = avatar;
    app
}
fn camera(v: &VehicleInstance, local: [f32; 3], yaw: f32, pitch: f32, fov: f32) -> Camera {
    let (position, yaw, pitch, roll) = crate::coop_view::camera_pose(
        v,
        &omsi_vehicle::Camera {
            pos: local,
            yaw,
            pitch,
            fov,
            ..Default::default()
        },
        (201.0, 8.0),
        (201.0, 8.0),
    );
    Camera {
        position,
        yaw,
        pitch,
        roll,
        fov_deg: fov,
        near: 0.1,
        far: 6000.0,
    }
}
fn screen(camera: &Camera, world: DVec3) -> (f32, f32) {
    let p = camera
        .view_proj(1920.0 / 1080.0, camera.position)
        .project_point3((world - camera.position).as_vec3());
    assert!(
        p.x.abs() < 1.0 && p.y.abs() < 1.0,
        "fixture target must be visible: {p:?}"
    );
    ((p.x + 1.0) * 960.0, (1.0 - p.y) * 540.0)
}

#[test]
fn cashier_hover_and_press_follow_shown_camera_instead_of_avatar() {
    let f = Fixture::new();
    let mut v = f.vehicle();
    v.position = DVec3::new(5620.3, 1314.5, 0.09);
    v.heading = 169.0;
    let shown = camera(&v, [0.0, 0.5, 1.8], 0.0, -20.0, 65.0);
    let mut app = setup(v, shown);
    for (i, x) in [(0, -0.45), (1, 0.0), (2, 0.45)] {
        let target = app.vehicle.position
            + app
                .vehicle
                .body_rotation()
                .transform_point3(Vec3::new(x, 2.0, 1.25))
                .as_dvec3();
        app.cursor = screen(&shown, target);
        let wrong = app.cursor_ray_now().unwrap();
        assert_eq!(
            crate::picking::pick_in(
                &app.vehicle,
                wrong.0,
                wrong.1,
                wrong.2,
                crate::picking::Reach::ANY
            ),
            None
        );
        // Same production App method is used by coop_hit for hover and press.
        for avatar_yaw in [180.0, 70.0] {
            app.camera.as_mut().unwrap().yaw = avatar_yaw;
            let (o, d, spread) = app.coop_cursor_ray(app.bus).unwrap();
            assert_eq!(
                crate::picking::pick_in(&app.vehicle, o, d, spread, crate::picking::Reach::ANY)
                    .map(|h| h.0),
                Some(i)
            );
        }
    }
}

#[test]
fn shifted_zoomed_cashier_camera_keeps_ticket_and_money_under_cursor_on_moving_bus() {
    let f = Fixture::new();
    for (local, yaw, pitch, fov) in [
        ([0.0, 0.5, 1.8], 0.0, -20.0, 65.0),
        ([0.4, 0.25, 1.95], -10.0, -25.0, 42.0),
    ] {
        let mut v = f.vehicle();
        v.position = DVec3::new(5620.3, 1314.5, 0.09);
        v.heading = 169.0;
        let shown = camera(&v, local, yaw, pitch, fov);
        let mut app = setup(v, shown);
        let drawn = BodyPose::of(&app.vehicle);
        let data = app.vehicle.ty.meshes[0].data.clone();
        let blocks = paper::blocks(data, glam::Mat4::IDENTITY);
        // Physics advanced after the picture; the presented view stays unchanged.
        app.vehicle.position += DVec3::new(0.7, -0.4, 0.08);
        app.vehicle.heading += 6.0;
        for (i, x) in [(0, -0.45), (1, 0.0), (2, 0.45)] {
            let target = drawn.position
                + drawn
                    .rotation
                    .transform_point3(Vec3::new(x, 2.0, 1.25))
                    .as_dvec3();
            app.cursor = screen(&shown, target);
            let (o, d, spread) = app.coop_cursor_ray(app.bus).unwrap();
            assert_eq!(
                blocks.hit(o, d, &app.vehicle),
                (i == 0).then_some(3),
                "native tickets must not steal money or flap clicks"
            );
            assert_eq!(
                crate::picking::pick_in(&app.vehicle, o, d, spread, crate::picking::Reach::ANY)
                    .map(|h| h.0),
                Some(i)
            );
            assert!((spread - crate::pixel_angle(&shown, 1080.0) * 6.0).abs() < 1e-7);
        }
    }
}

#[test]
fn cooperative_pointer_does_not_reuse_another_bus_picture() {
    let f = Fixture::new();
    let v = f.vehicle();
    let shown = camera(&v, [0.0, 0.5, 1.8], 0.0, -20.0, 65.0);
    let mut app = setup(v, shown);
    app.net.coop.drawn = None;
    assert!(app.coop_cursor_ray(app.bus).is_none());
    app.net.coop.drawn = Some((
        BusId::Ai(99),
        PresentedView {
            camera: shown,
            body: BodyPose::of(&app.vehicle),
        },
    ));
    assert!(app.coop_cursor_ray(app.bus).is_none());
}
