//! Script mouse behavior shared by the local cab and an owner-executed cooperative click.
use omsi_sim::VehicleInstance;

#[derive(Debug, Clone)]
pub(crate) struct Press {
    pub plain: bool,
    pub movement: f32,
}

fn has_trigger(v: &VehicleInstance, event: &str) -> bool {
    v.ty.program.trigger(event).is_some()
        || v.trailers
            .iter()
            .any(|t| t.ty.program.trigger(event).is_some())
}

impl Press {
    pub(crate) fn begin(v: &mut VehicleInstance, event: &str) -> Option<Self> {
        v.host.mouse = (0.0, 0.0);
        let plain = v.trigger(event);
        // Doors/flaps can have only _drag, or perform their action on _off. The
        // local mouse accepts both; an absent press trigger is not a refusal.
        if !plain
            && !has_trigger(v, &format!("{event}_drag"))
            && !has_trigger(v, &format!("{event}_off"))
        {
            return None;
        }
        v.update_scripts_only(0.0);
        let low = event.to_ascii_lowercase();
        let drag = format!("{event}_drag");
        if (low.contains("taste") || low.contains("ibox") || low.contains("ticket"))
            && has_trigger(v, &drag)
        {
            v.trigger(&drag);
            v.host.mouse = (0.0, 0.0);
        }
        Some(Self {
            plain,
            movement: 0.0,
        })
    }

    pub(crate) fn drag(&mut self, v: &mut VehicleInstance, event: &str, x: f32, y: f32) {
        self.movement += x.abs() + y.abs();
        v.host.mouse = (x, y);
        v.trigger(&format!("{event}_drag"));
        v.host.mouse = (0.0, 0.0);
    }

    pub(crate) fn release(
        self,
        v: &mut VehicleInstance,
        event: &str,
        animation: Option<String>,
        completed: bool,
    ) -> Option<AutoDrag> {
        v.trigger(&format!("{event}_off"));
        if !completed
            || !auto_drag_click(event, self.plain, self.movement)
            || !has_trigger(v, &format!("{event}_drag"))
        {
            return None;
        }
        let var = animation.filter(|n| !n.trim().is_empty() && n.trim().parse::<f32>().is_err())?;
        let now = v.var(&var)?;
        let target = if now > 0.5 { 0.0 } else { 1.0 };
        Some(AutoDrag {
            ev: event.into(),
            var,
            target,
            axis: 0,
            sign: if target > now { -1.0 } else { 1.0 },
            last: now,
            stalled: 0,
            t: 0.0,
        })
    }
}

pub(crate) fn auto_drag_click(event: &str, plain: bool, movement: f32) -> bool {
    let event = event.to_ascii_lowercase().replace(['_', '-'], "");
    !plain
        && movement < 4.0
        && !["rollo", "sunblind", "sonnenblende"]
            .iter()
            .any(|name| event.contains(*name))
}

/// The same bounded click-to-drag animation used by the local player, independent
/// of their mouse state so the conductor cannot release the driver's own control.
#[derive(Debug, Clone)]
pub(crate) struct AutoDrag {
    pub ev: String,
    pub var: String,
    pub target: f32,
    pub axis: usize,
    pub sign: f32,
    pub last: f32,
    pub stalled: u32,
    pub t: f32,
}
impl AutoDrag {
    pub(crate) fn cancel(&self, v: &mut VehicleInstance) {
        v.trigger(&format!("{}_off", self.ev));
        v.host.mouse = (0.0, 0.0);
    }

    /// True while more script drag steps are needed. No variable is written directly.
    pub(crate) fn step(&mut self, v: &mut VehicleInstance, dt: f32) -> bool {
        let now = v.var(&self.var).unwrap_or(self.last);
        self.t += dt;
        if (now - self.target).abs() < 0.03 || self.t > 2.5 {
            self.cancel(v);
            return false;
        }
        let towards = (self.target - self.last).signum();
        let moved = now - self.last;
        if self.t > 0.05 {
            if moved * towards < -1e-4 {
                self.sign = -self.sign;
            } else if moved.abs() < 1e-4 {
                self.stalled += 1;
                if self.stalled == 3 {
                    self.sign = -self.sign;
                } else if self.stalled == 6 {
                    self.axis = 1 - self.axis;
                } else if self.stalled == 9 {
                    self.sign = -self.sign;
                } else if self.stalled > 12 {
                    self.cancel(v);
                    return false;
                }
            } else {
                self.stalled = 0;
            }
        }
        self.last = now;
        let step = 900.0 * dt.min(0.05) * self.sign;
        v.host.mouse = if self.axis == 0 {
            (step, 0.0)
        } else {
            (0.0, step)
        };
        let exists = v.trigger(&format!("{}_drag", self.ev));
        v.host.mouse = (0.0, 0.0);
        exists
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::{
        path::PathBuf,
        sync::{
            atomic::{AtomicU64, Ordering},
            Arc,
        },
    };

    struct Fixture(PathBuf);
    impl Fixture {
        fn new(script: &str) -> Self {
            static NEXT: AtomicU64 = AtomicU64::new(0);
            let p = std::env::temp_dir().join(format!(
                "coop-pointer-{}-{}",
                std::process::id(),
                NEXT.fetch_add(1, Ordering::Relaxed)
            ));
            std::fs::create_dir_all(&p).unwrap();
            std::fs::write(p.join("model.cfg"), "").unwrap();
            std::fs::write(p.join("vars.txt"), "flap_pos\nreleased\npressed\ntickets\n").unwrap();
            std::fs::write(p.join("control.osc"), script).unwrap();
            std::fs::write(p.join("test.bus"), "[friendlyname]\nTest\nConductor control\nTest\n[model]\nmodel.cfg\n[varnamelist]\n1\nvars.txt\n[script]\n1\ncontrol.osc\n").unwrap();
            Self(p)
        }
        fn vehicle(&self) -> VehicleInstance {
            let ty = omsi_sim::VehicleType::load(&self.0, &self.0.join("test.bus")).unwrap();
            VehicleInstance::new(Arc::new(ty), omsi_sim::VehicleHost::new(Default::default()))
        }
    }
    impl Drop for Fixture {
        fn drop(&mut self) {
            let _ = std::fs::remove_dir_all(&self.0);
        }
    }
    const FLAP: &str = "{trigger:cp_portinha_drag}\n(L.L.flap_pos) (L.S.mouse_x) 100 / - 0 max 1 min (S.L.flap_pos)\n{end}\n{trigger:cp_portinha_off}\n1 (S.L.released)\n{end}\n";

    #[test]
    fn drag_only_conductor_flap_opens_and_closes_with_real_script() {
        let f = Fixture::new(FLAP);
        let mut v = f.vehicle();
        assert!(v.ty.program.trigger("cp_portinha").is_none());
        for expected in [1.0, 0.0] {
            let p = Press::begin(&mut v, "cp_portinha")
                .expect("a drag-only flap must accept its press");
            assert!(!p.plain);
            let mut drag = p
                .release(&mut v, "cp_portinha", Some("flap_pos".into()), true)
                .unwrap();
            for _ in 0..120 {
                if !drag.step(&mut v, 1.0 / 60.0) {
                    break;
                }
            }
            assert!((v.var("flap_pos").unwrap() - expected).abs() < 0.03);
            assert_eq!(v.host.mouse, (0.0, 0.0));
        }
    }

    #[test]
    fn a_manual_flap_drag_stays_where_the_conductor_left_it() {
        let f = Fixture::new(FLAP);
        let mut v = f.vehicle();
        let mut p = Press::begin(&mut v, "cp_portinha").unwrap();
        p.drag(&mut v, "cp_portinha", -30.0, 0.0);
        assert!(p
            .release(&mut v, "cp_portinha", Some("flap_pos".into()), true)
            .is_none());
        assert!((v.var("flap_pos").unwrap() - 0.3).abs() < 1e-5);
    }

    #[test]
    fn revoking_or_timing_out_a_press_does_not_turn_it_into_a_click() {
        let f = Fixture::new(FLAP);
        let mut v = f.vehicle();
        let p = Press::begin(&mut v, "cp_portinha").unwrap();
        assert!(p
            .release(&mut v, "cp_portinha", Some("flap_pos".into()), false)
            .is_none());
        assert_eq!(v.var("flap_pos"), Some(0.0));
        assert_eq!(v.var("released"), Some(1.0));
    }

    #[test]
    fn release_only_controls_are_accepted_until_mouse_up() {
        let f = Fixture::new("{trigger:portinha_off}\n(L.L.released) 1 + (S.L.released)\n{end}\n");
        let mut v = f.vehicle();
        let p = Press::begin(&mut v, "portinha").unwrap();
        assert_eq!(v.var("released"), Some(0.0));
        assert!(p.release(&mut v, "portinha", None, true).is_none());
        assert_eq!(v.var("released"), Some(1.0));
        assert!(Press::begin(&mut v, "unknown_control").is_none());
    }

    #[test]
    fn momentary_ticket_key_is_consumed_once_before_release() {
        let f = Fixture::new("{trigger:ticket_taste_1}\n1 (S.L.pressed)\n{end}\n{trigger:ticket_taste_1_drag}\n0 (S.L.pressed)\n{end}\n{trigger:ticket_taste_1_off}\n0 (S.L.pressed)\n{end}\n{frame}\n(L.L.pressed) {if}\n(L.L.tickets) 1 + (S.L.tickets)\n{endif}\n{end}\n");
        let mut v = f.vehicle();
        let p = Press::begin(&mut v, "ticket_taste_1").unwrap();
        assert_eq!(v.var("tickets"), Some(1.0));
        p.release(&mut v, "ticket_taste_1", None, true);
        for _ in 0..5 {
            v.update_scripts_only(0.01);
        }
        assert_eq!(v.var("tickets"), Some(1.0));
    }
}
