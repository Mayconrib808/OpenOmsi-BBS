//! Scenery objects (`.sco`) and splines (`.sli`).

pub mod sco;
pub mod sli;

pub use sco::{SceneryObject, TrafficLight, TrafficLightPhase};
pub use sli::{Spline, SplineProfile, SplineProfilePoint};

/// A traffic path (lane) definition shared by `.sco` (`[path]`/`[path_2]`) and `.sli`.
#[derive(Debug, Clone, PartialEq, Default)]
pub struct PathDef {
    /// 0 = street, 1 = sidewalk (pedestrian), 2 = rail.
    pub kind: i32,
    /// Lateral position (spline) or start point (object).
    pub start: [f32; 3],
    /// Object paths: heading / radius / length description.
    pub end: [f32; 3],
    pub width: f32,
    /// Direction flag: 0 forward, 1 backward, 2 both.
    pub direction: i32,
    pub params: Vec<f32>,
}
