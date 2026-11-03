//! Original accessibility meshes, metres, +Y forward. Shared meshes; only wheel
//! and lift instance transforms change during playback. No third-party assets.
use glam::{Vec2, Vec3};
use omsi_geometry::MeshData;
use std::f32::consts::TAU;

pub const RUBBER: u32 = 0;
pub const METAL: u32 = 1;
pub const FABRIC: u32 = 2;
pub const FRAME: u32 = 3;
pub const YELLOW: u32 = 4;
pub const DECK: u32 = 5;

#[derive(Default)]
struct Build {
    m: MeshData,
    triangles: [Vec<u32>; 6],
}
impl Build {
    fn tri(&mut self, a: Vec3, b: Vec3, c: Vec3, slot: u32) {
        let n = (b - a).cross(c - a).normalize_or(Vec3::Z);
        let k = self.m.positions.len() as u32;
        self.m.positions.extend([a, b, c]);
        self.m.normals.extend([n; 3]);
        self.m.uvs.extend([Vec2::ZERO, Vec2::X, Vec2::Y]);
        self.triangles[slot as usize].extend([k, k + 1, k + 2]);
    }
    fn quad(&mut self, a: Vec3, b: Vec3, c: Vec3, d: Vec3, slot: u32) {
        self.tri(a, b, c, slot);
        self.tri(a, c, d, slot);
    }
    fn tube(&mut self, a: Vec3, b: Vec3, r: f32, slot: u32) {
        let axis = (b - a).normalize_or(Vec3::Z);
        let u = axis.cross(Vec3::X).normalize_or(Vec3::Y);
        let v = axis.cross(u).normalize_or(Vec3::X);
        for i in 0..12 {
            let radial = |j: usize| {
                let t = j as f32 * TAU / 12.0;
                (u * t.cos() + v * t.sin()) * r
            };
            let (x, y) = (radial(i), radial(i + 1));
            let k = self.m.positions.len() as u32;
            self.m.positions.extend([a + x, b + x, b + y, a + y]);
            self.m
                .normals
                .extend([x.normalize(), x.normalize(), y.normalize(), y.normalize()]);
            self.m.uvs.extend([Vec2::ZERO, Vec2::X, Vec2::ONE, Vec2::Y]);
            self.triangles[slot as usize].extend([k, k + 1, k + 2, k, k + 2, k + 3]);
            self.tri(a, a + y, a + x, slot);
            self.tri(b, b + x, b + y, slot);
        }
    }
    fn box_at(&mut self, c: Vec3, half: Vec3, slot: u32) {
        let p = |x: f32, y: f32, z: f32| c + half * Vec3::new(x, y, z);
        for [a, b, c, d] in [
            [
                p(-1., -1., -1.),
                p(-1., 1., -1.),
                p(1., 1., -1.),
                p(1., -1., -1.),
            ],
            [
                p(-1., -1., 1.),
                p(1., -1., 1.),
                p(1., 1., 1.),
                p(-1., 1., 1.),
            ],
            [
                p(-1., -1., -1.),
                p(1., -1., -1.),
                p(1., -1., 1.),
                p(-1., -1., 1.),
            ],
            [
                p(1., 1., -1.),
                p(-1., 1., -1.),
                p(-1., 1., 1.),
                p(1., 1., 1.),
            ],
            [
                p(-1., 1., -1.),
                p(-1., -1., -1.),
                p(-1., -1., 1.),
                p(-1., 1., 1.),
            ],
            [
                p(1., -1., -1.),
                p(1., 1., -1.),
                p(1., 1., 1.),
                p(1., -1., 1.),
            ],
        ] {
            self.quad(a, b, c, d, slot);
        }
    }
    /// Torus around the wheel axle, X. Rounded rubber and narrow hand rim.
    fn ring(&mut self, x: f32, radius: f32, tube: f32, slot: u32, segments: usize) {
        for i in 0..segments {
            for j in 0..8 {
                let vertex = |i: usize, j: usize| {
                    let a = i as f32 * TAU / segments as f32;
                    let b = j as f32 * TAU / 8.0;
                    let n = Vec3::new(b.sin(), a.cos() * b.cos(), a.sin() * b.cos());
                    (
                        Vec3::new(x, 0., 0.) + Vec3::new(0., a.cos(), a.sin()) * radius + n * tube,
                        n,
                    )
                };
                let vs = [
                    vertex(i, j),
                    vertex(i + 1, j),
                    vertex(i + 1, j + 1),
                    vertex(i, j + 1),
                ];
                let k = self.m.positions.len() as u32;
                for (v, n) in vs {
                    self.m.positions.push(v);
                    self.m.normals.push(n);
                    self.m.uvs.push(Vec2::ZERO);
                }
                self.triangles[slot as usize].extend([k, k + 1, k + 2, k, k + 2, k + 3]);
            }
        }
    }
    fn finish(mut self) -> MeshData {
        for (slot, t) in self.triangles.into_iter().enumerate() {
            if !t.is_empty() {
                let start = self.m.indices.len() as u32;
                self.m.indices.extend(t);
                self.m
                    .ranges
                    .push((start, self.m.indices.len() as u32 - start, slot as u32));
            }
        }
        self.m
    }
}

pub fn chair_frame() -> MeshData {
    let mut b = Build::default();
    // Powder-coated side frames, cross brace, back uprights and push handles.
    for side in [-1., 1.] {
        let x = side * 0.255;
        for (a, c) in [
            (Vec3::new(x, -0.22, 0.28), Vec3::new(x, -0.22, 0.99)),
            (Vec3::new(x, -0.22, 0.48), Vec3::new(x, 0.30, 0.48)),
            (Vec3::new(x, 0.30, 0.48), Vec3::new(x, 0.40, 0.13)),
            (Vec3::new(x, 0.40, 0.13), Vec3::new(x, -0.22, 0.28)),
            (Vec3::new(x, -0.22, 0.99), Vec3::new(x, -0.36, 0.99)),
        ] {
            b.tube(a, c, 0.015, FRAME);
        }
        b.tube(
            Vec3::new(x, -0.31, 0.99),
            Vec3::new(x, -0.40, 0.99),
            0.021,
            RUBBER,
        );
        // Arm support, removable footrest tube and heel-supporting footplate.
        b.tube(
            Vec3::new(x, -0.15, 0.51),
            Vec3::new(x, -0.15, 0.72),
            0.011,
            METAL,
        );
        b.tube(
            Vec3::new(x, 0.23, 0.51),
            Vec3::new(x, 0.23, 0.72),
            0.011,
            METAL,
        );
        b.box_at(
            Vec3::new(x, 0.04, 0.73),
            Vec3::new(0.034, 0.23, 0.022),
            RUBBER,
        );
        b.tube(
            Vec3::new(x, 0.28, 0.47),
            Vec3::new(side * 0.14, 0.55, 0.16),
            0.012,
            METAL,
        );
        b.box_at(
            Vec3::new(side * 0.135, 0.56, 0.147),
            Vec3::new(0.105, 0.12, 0.012),
            RUBBER,
        );
        // Brake lever near the main tyre and caster fork.
        b.tube(
            Vec3::new(side * 0.28, 0.18, 0.48),
            Vec3::new(side * 0.29, 0.23, 0.60),
            0.009,
            METAL,
        );
        b.tube(
            Vec3::new(side * 0.29, 0.23, 0.60),
            Vec3::new(side * 0.29, 0.27, 0.62),
            0.015,
            RUBBER,
        );
        for dx in [-0.026, 0.026] {
            b.tube(
                Vec3::new(x + dx, 0.40, 0.17),
                Vec3::new(x + dx, 0.46, 0.075),
                0.008,
                METAL,
            );
        }
        // Anti-tip rear rollers.
        b.tube(
            Vec3::new(x, -0.18, 0.25),
            Vec3::new(x, -0.42, 0.08),
            0.011,
            FRAME,
        );
    }
    b.tube(
        Vec3::new(-0.255, -0.20, 0.28),
        Vec3::new(0.255, 0.24, 0.46),
        0.012,
        METAL,
    );
    b.tube(
        Vec3::new(0.255, -0.20, 0.28),
        Vec3::new(-0.255, 0.24, 0.46),
        0.012,
        METAL,
    );
    b.box_at(
        Vec3::new(0.0, 0.03, 0.50),
        Vec3::new(0.24, 0.25, 0.025),
        FABRIC,
    );
    b.box_at(
        Vec3::new(0.0, -0.225, 0.745),
        Vec3::new(0.24, 0.021, 0.215),
        FABRIC,
    );
    // Seat seams and lower calf support.
    for x in [-0.22, 0.22] {
        b.tube(
            Vec3::new(x, -0.19, 0.528),
            Vec3::new(x, 0.25, 0.528),
            0.003,
            RUBBER,
        );
    }
    b.box_at(
        Vec3::new(0., 0.39, 0.27),
        Vec3::new(0.22, 0.012, 0.032),
        FABRIC,
    );
    b.finish()
}

pub fn wheel(side: f32) -> MeshData {
    let mut b = Build::default();
    b.ring(0.0, 0.282, 0.018, RUBBER, 48);
    b.ring(0.0, 0.255, 0.008, METAL, 48);
    b.ring(side * 0.036, 0.240, 0.009, METAL, 48);
    b.tube(
        Vec3::new(-0.026, 0., 0.),
        Vec3::new(0.026, 0., 0.),
        0.030,
        METAL,
    );
    for i in 0..24 {
        let a = i as f32 * TAU / 24.0;
        b.tube(
            Vec3::new(if i % 2 == 0 { 0.018 } else { -0.018 }, 0., 0.),
            Vec3::new(0., a.cos() * 0.252, a.sin() * 0.252),
            0.0018,
            METAL,
        );
    }
    // Valve and reflector make the rolling phase visible.
    b.tube(
        Vec3::new(0., 0.252, 0.),
        Vec3::new(0., 0.276, 0.),
        0.003,
        RUBBER,
    );
    b.box_at(
        Vec3::new(0., 0.19, 0.),
        Vec3::new(0.013, 0.035, 0.010),
        YELLOW,
    );
    b.finish()
}
pub fn caster() -> MeshData {
    let mut b = Build::default();
    b.ring(0., 0.056, 0.019, RUBBER, 24);
    b.tube(
        Vec3::new(-0.021, 0., 0.),
        Vec3::new(0.021, 0., 0.),
        0.025,
        METAL,
    );
    b.finish()
}

/// Unit platform along X. Sized by the measured door's usable width/length.
pub fn platform() -> MeshData {
    let mut b = Build::default();
    b.box_at(Vec3::new(0., 0., -0.025), Vec3::new(0.5, 0.5, 0.025), DECK);
    for y in [-0.49, 0.49] {
        b.box_at(Vec3::new(0., y, 0.025), Vec3::new(0.5, 0.01, 0.025), YELLOW);
    }
    for x in [-0.49, 0.49] {
        b.box_at(
            Vec3::new(x, 0., 0.003),
            Vec3::new(0.01, 0.48, 0.003),
            YELLOW,
        );
    }
    for i in -10..=10 {
        b.box_at(
            Vec3::new(i as f32 * 0.042, 0., 0.001),
            Vec3::new(0.002, 0.45, 0.001),
            RUBBER,
        );
    }
    b.finish()
}
pub fn lift_arm() -> MeshData {
    let mut b = Build::default();
    b.tube(Vec3::ZERO, Vec3::Z, 0.025, FRAME);
    b.tube(
        Vec3::new(0., 0., 0.1),
        Vec3::new(0., 0., 0.55),
        0.037,
        METAL,
    );
    b.finish()
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn wheelchair_meshes_have_finite_geometry_and_valid_indices() {
        for m in [
            chair_frame(),
            wheel(-1.),
            wheel(1.),
            caster(),
            platform(),
            lift_arm(),
        ] {
            assert!(!m.positions.is_empty());
            assert_eq!(m.positions.len(), m.normals.len());
            assert_eq!(m.positions.len(), m.uvs.len());
            assert!(m.positions.iter().all(|v| v.is_finite()));
            assert!(
                m.normals
                    .iter()
                    .all(|v| v.is_finite() && (v.length() - 1.).abs() < 0.001)
            );
            assert!(m.indices.iter().all(|i| (*i as usize) < m.positions.len()));
            assert_eq!(
                m.ranges.iter().map(|r| r.1).sum::<u32>() as usize,
                m.indices.len()
            );
            assert!(m.indices.len() < 100000);
        }
    }
}
