#!/usr/bin/env python3
"""Layer the 2.2.3 multiplayer/boarding fixes on the *materialized* 2.2.2 source.

The public source branch stores the exact 2.2.2 changes as cumulative patches.
Running before native.patch would silently remove or conflict with those changes.
Fail closed on any changed upstream anchor, so CI never publishes a mixed build.
"""
from pathlib import Path

def apply(path, before, after):
    p = Path(path)
    data = p.read_text(encoding="utf-8")
    count = data.count(before)
    if count != 1:
        raise RuntimeError(f"{p}: expected 1 anchor, got {count}: {before[:90]!r}")
    p.write_text(data.replace(before, after, 1), encoding="utf-8")

# Route-sharing messages are handled by the driving subsystem; stop parsing
# them as generic LAN admin messages and logging a false reject every second.
apply("crates/omsi-app/src/app_impl/lan_tick.rs",
    "        self.drive_receive(from, text);",
    "        if self.drive_receive(from, text) { return; }")

# Keep boarding physically on the same carriageway/platform, but allow an
# eight-metre pole offset in either direction. Never cross a road or floor.
apply("crates/omsi-sim/src/people/pax_stops.rs",
    """pub(super) fn entry_on_platform(side: Option<f32>, door_side: f32) -> bool {
    side.is_none_or(|wanted| wanted == door_side)
}
""",
    """pub(super) fn entry_on_platform(side: Option<f32>, door_side: f32) -> bool {
    side.is_none_or(|wanted| wanted == door_side)
}

/// Tolerate stop-pole placement without treating the opposite road as a stop.
fn tolerant_stop_box(lateral: f64, along: f64, height: f64, length: f32) -> bool {
    let reach = (f64::from(length.max(0.0)) + 8.0).clamp(10.0, 34.0);
    lateral.is_finite() && along.is_finite() && height.is_finite()
        && lateral.abs() <= 4.0 && along.abs() <= reach && height.abs() <= 3.0
}
#[cfg(test)]
mod tolerant_stop_box_tests {
    use super::tolerant_stop_box;
    #[test]
    fn same_platform_before_and_after_pole_can_board() {
        assert!(tolerant_stop_box(1.5, 9.0, 0.0, 5.0));
        assert!(tolerant_stop_box(-2.0, -9.0, 0.0, 5.0));
    }
    #[test]
    fn rejects_opposite_road_other_floor_and_far_stop() {
        assert!(!tolerant_stop_box(6.5, 2.0, 0.0, 5.0));
        assert!(!tolerant_stop_box(1.0, 2.0, 4.0, 5.0));
        assert!(!tolerant_stop_box(1.0, 40.0, 0.0, 5.0));
    }
}
""")
apply("crates/omsi-sim/src/people/pax_stops.rs",
    """                    let in_box = scheduled_here
                        || (lateral.abs() < 2.0 && along.abs() < (s.length as f64 - 5.0).max(0.0));""",
    """                    let in_box = scheduled_here
                        || (same_way && tolerant_stop_box(lateral, along, d.z, s.length));""")

# If the pathgraph refuses a nearest entry, choose another real entry point
# on the same platform and let the original walker move there. Never teleport.
apply("crates/omsi-sim/src/people/pax_task.rs",
    """        let pt = bn.cabin.omsi_nearest(here, &list, p.ticket == TICKET_BUY, false, Some(&flags), Some(&open))
            .or_else(|| bn.cabin.omsi_nearest(here, &list, p.ticket == TICKET_BUY, false, Some(&flags), None));""",
    """        let pt = bn.cabin.omsi_nearest(here, &list, p.ticket == TICKET_BUY, false, Some(&flags), Some(&open))
            .or_else(|| bn.cabin.omsi_nearest(here, &list, p.ticket == TICKET_BUY, false, Some(&flags), None))
            .or_else(|| {
                let nearest = list.iter().flatten().copied()
                    .filter_map(|id| bn.cabin.graph.points.get(id)
                        .map(|pos| (id, (*pos - here).length_squared())))
                    .min_by(|a, b| a.1.total_cmp(&b.1))
                    .map(|(id, _)| id);
                if nearest.is_some() && debug_pax() {
                    log::info!("pax {} reselecting same-side entry for bus {:?}",
                        self.people[i].id, bn.id);
                }
                nearest
            });""")
apply("crates/omsi-sim/src/people/pax_task.rs",
    "            // in the doorway, but the plugins have not had their say about the ticket yet",
    """            if self.pax(i).is_some_and(|p| p.door.is_none()) {
                self.choose_entry(i, buses, bus_ix);
                if self.pax(i).is_some_and(|p| p.door.is_none()) {
                    if debug_pax() {
                        log::info!("pax {} not boarding {:?}: no same-side entry point",
                            self.people[i].id, bn.id);
                    }
                    return;
                }
            }
            // in the doorway, but the plugins have not had their say about the ticket yet""")

# The host already has per-player bubbles. Do not alter the host's own
# population equation (the cumulative patch may have tuned it already).
# Fix only the remote bubble's target using roads local to that player.
apply("crates/omsi-app/src/traffic/population.rs",
    """    #[allow(clippy::too_many_arguments)]
    pub(super) fn populate_kind(""",
    """    /// Road population near the receiving LAN player, not near the host.
    fn street_target_near(&self, center: DVec3) -> usize {
        let near_density: Vec<f32> = self.sim.net.lanes_starting_near(center, self.sim.spawn_radius)
            .into_iter()
            .map(|i| &self.sim.net.lanes[i])
            .filter(|l| l.kind == LaneKind::Street
                && l.points.first().is_some_and(|p| (*p - center).length() < self.sim.spawn_radius))
            .map(|l| if l.no_cars { 0.0 } else { l.density.clamp(0.0, 4.0) })
            .collect();
        (self.sim.target as f32 * self.street_density() * road_scale(&near_density)).round() as usize
    }

    #[allow(clippy::too_many_arguments)]
    pub(super) fn populate_kind(""")
apply("crates/omsi-app/src/traffic/population.rs",
    """            self.sim.count_near = Some((c, self.sim.spawn_radius));
            self.populate_kind(world, c, LaneKind::Street, target);""",
    """            let local_target = self.street_target_near(c);
            if self.sim.debug_population {
                log::info!("population LAN center ({:.0},{:.0}): host target {}, player target {}, admission refused {:?} retried {:?}",
                    c.x, c.y, target, local_target, self.sim.admission.denied, self.sim.admission.retried);
            }
            self.sim.count_near = Some((c, self.sim.spawn_radius));
            self.populate_kind(world, c, LaneKind::Street, local_target);""")
apply("crates/omsi-app/src/traffic/population.rs",
    """        if candidates.is_empty() {
            return;
        }
        let mut acc = 0.0f32;""",
    """        if candidates.is_empty() {
            if self.sim.debug_population {
                let forbidden = nearby.iter().filter(|&&i| self.sim.net.lanes[i].no_cars).count();
                let zero = nearby.iter().filter(|&&i| self.sim.net.lanes[i].density <= 0.0).count();
                log::info!("population no eligible lane at ({:.0},{:.0}) kind={kind:?}: {} nearby, {} forbidden, {} zero density",
                    center.x, center.y, nearby.len(), forbidden, zero);
            }
            return;
        }
        let mut acc = 0.0f32;""")

# Make an ownership grant trace useful: whether a granted passenger became
# local and which stop / bus / task it was trying to board.
apply("crates/omsi-app/src/lan_world.rs",
    """                        log::info!("LAN: the host hands waiting passenger {id} over to our bus");""",
    """                        let passenger = h.people.iter().find(|person| person.id == id)
                            .and_then(|person| match &person.state {
                                omsi_sim::people::State::Pax(p) => Some(format!(
                                    "stop={:?} bus={:?} task={:?} position=({:.1},{:.1})",
                                    p.stop, p.bus, p.task, person.position.x, person.position.y)),
                                _ => None,
                            })
                            .unwrap_or_else(|| "no local waiting passenger".into());
                        log::info!("LAN: the host hands waiting passenger {id} over to our bus; {passenger}");""")
print("OpenOMSI 2.2.3 post-patch fixes layered on the fully materialized native source")
