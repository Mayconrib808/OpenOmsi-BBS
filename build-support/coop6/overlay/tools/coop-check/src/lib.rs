#![allow(dead_code)]
use glam::{DVec3,Vec3};
extern crate self as omsi_render;
mod render_camera {
    use glam::{DVec3, Mat4, Vec3};
    include!(concat!(env!("OUT_DIR"), "/camera.rs"));
}
pub use render_camera::Camera;
include!(concat!(env!("OUT_DIR"), "/cursor.rs"));
include!(concat!(env!("OUT_DIR"),"/broad_phase.rs"));
#[path="../../../crates/omsi-app/src/player/picking.rs"]
mod picking;
#[path="../../../crates/omsi-app/src/coop_view.rs"]
mod coop_view;
#[path="../../../crates/omsi-lan/src/coop.rs"]
mod protocol;
#[path="../../../crates/omsi-app/src/mouse_control.rs"]
mod mouse_control;
#[cfg(test)]
mod pointer_pipeline;
