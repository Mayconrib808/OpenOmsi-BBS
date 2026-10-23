"""Targeted, fail-closed first correction for driver handover (Coop 10 candidate).

Run only after coop6, driver-share, character and greetings overlays have been
applied to the pinned upstream openOMSI source. This does not publish a build.
"""
from pathlib import Path

support = Path(__file__).resolve().parent
root = support.parents[2]
game = root / 'game'
path = game / 'crates/omsi-app/src/coop_drive.rs'
source = path.read_text(encoding='utf-8')

# The owner has an actual on-foot avatar; switching merely to the pax camera
# leaves the driver seated on the clients and can put the camera in the saloon.
old = '''                        self.coop_grant(Some(from));
                        self.view="pax".into();'''
new = '''                        self.coop_grant(Some(from));
                        // Release the actual driver seat before changing the view.
                        // Keep ownership and BCS on this client; do not respawn a bus.
                        if self.session.on_foot.is_none() {
                            let allowed_get_up = self.settings.get_up;
                            self.settings.get_up = true;
                            self.get_up();
                            self.settings.get_up = allowed_get_up;
                        }
                        self.view = if self.session.on_foot.is_some() { "foot" } else { "pax" }.into();'''
assert source.count(old) == 1, 'Unexpected driver handover source: refuses unsafe patch'
source = source.replace(old, new)

old_message = 'O pedido de direção expirou. Confira se os dois usam a Coop 7.'
if old_message not in source:
    old_message = 'O pedido de direÃ§Ã£o expirou. Confira se os dois usam a Coop 7.'
if source.count(old_message) == 1:
    source = source.replace(old_message, 'O pedido de direcao expirou. Confira se os dois usam a mesma versao do Coop.')

path.write_text(source, encoding='utf-8', newline='\n')

# Allow the owner to grant pending requests even when standing/walking outside
# the bus. This only exposes the button when an actual passenger is requesting
# control of the owner's bus; the authority check stays in drive_grant().
panel_path = game / 'crates/omsi-app/src/game_panels/coop_panel.rs'
panel = panel_path.read_text(encoding='utf-8')
seated_only = 'if app.session.on_foot.is_none() && app.player.is_some() {'
all_owner_modes = '''if app.player.is_some() && (
        app.session.on_foot.is_none()
        || app.coop_passengers().iter().any(|(peer, _)| app.drive_requested(*peer))
    ) {'''
assert panel.count(seated_only) == 1, 'Unexpected Coop panel source: refuses unsafe patch'
panel = panel.replace(seated_only, all_owner_modes)
panel_path.write_text(panel, encoding='utf-8', newline='\n')


# First-person Cooperative camera: hide only the remote driver's *render*
# for the viewer while looking through the driver's eyes, not network avatar
# state. Other players, F2 and exterior views retain the chosen figure.
lan_path = game / 'crates/omsi-lan/src/game.rs'
lan = lan_path.read_text(encoding='utf-8')
where = '    pub listener: Option<DVec3>,'
assert lan.count(where) == 1, 'LanInput structure changed unexpectedly'
lan = lan.replace(where,
    where + '\n    /// Local rendering preference; never transmitted to other clients.\n    pub hide_driver_for: Option<u32>,')
lan_path.write_text(lan, encoding='utf-8', newline='\n')

tick_path = game / 'crates/omsi-app/src/app_impl/lan_tick.rs'
tick = tick_path.read_text(encoding='utf-8')
where = '        let figure = self.character_figure();'
extra = '''        let hide_driver_for = if self.net.coop.camera.as_ref().is_some_and(|c| c.driver) {
            match self.foot_bus() {
                Some(crate::humans::BusId::Ai(id)) => crate::humans::remote_bus_player(id),
                _ => None,
            }
        } else {
            None
        };'''
assert tick.count(where) == 1, 'Character tick changed unexpectedly'
tick = tick.replace(where, where + '\n' + extra)
where = '            listener: self.camera.as_ref().map(|c| c.position),'
assert tick.count(where) == 1, 'LAN frame creation changed unexpectedly'
tick = tick.replace(where, where + '\n            hide_driver_for,')
tick_path.write_text(tick, encoding='utf-8', newline='\n')

off_path = game / 'crates/omsi-app/src/offscreen/step.rs'
off = off_path.read_text(encoding='utf-8')
where = '            let frame = lan::LanInput {\n                listener,'
assert off.count(where) == 1, 'Offscreen LAN frame changed unexpectedly'
off = off.replace(where, where + '\n                hide_driver_for: None,')
off_path.write_text(off, encoding='utf-8', newline='\n')

view_path = game / 'crates/omsi-app/src/view_sync/lan.rs'
view = view_path.read_text(encoding='utf-8')
where = 'rv.last.walker.is_none(), false);'
assert view.count(where) == 1, 'Remote driver renderer changed unexpectedly'
view = view.replace(where,
    'rv.last.walker.is_none() && frame.hide_driver_for != Some(*id), false);')
# The body model is allowed to refresh even while the remote vehicle is being
# represented by a temporary stand-in. Otherwise the selected .hum can remain
# frozen until the bus is removed or the player reconnects.
stale_skin = 'if v.driver_for.as_deref() != Some(rv.last.figure.as_str()) && !rv.stand_in {'
assert view.count(stale_skin) == 1, 'Remote character lifecycle changed unexpectedly'
view = view.replace(stale_skin,
    'if v.driver_for.as_deref() != Some(rv.last.figure.as_str()) {')
view_path.write_text(view, encoding='utf-8', newline='\n')


# The player-hosted LAN world must stream traffic, waiting passengers and
# pedestrians around EVERY player, including those who removed their bus or
# work as a conductor on foot. The host remains the only AI authority.
world_path = game / 'crates/omsi-app/src/lan_world.rs'
world = world_path.read_text(encoding='utf-8')
filter_vehicle = '.filter(|p| p.has_pose && p.pose.has_vehicle())'
assert world.count(filter_vehicle) == 2, 'LAN host player-center filters changed unexpectedly'
world = world.replace(filter_vehicle,
    '.filter(|p| p.has_pose && (p.pose.has_vehicle() || p.pose.walker.is_some()))')
vehicle_center = '.map(|p| (p.pose.id, DVec3::new(p.pose.x, p.pose.y, p.pose.z)))'
assert world.count(vehicle_center) == 1, 'LAN host center mapper changed unexpectedly'
world = world.replace(vehicle_center,
    '''.map(|p| {
                let at = p.pose.walker.map(|w| DVec3::new(w.x, w.y, w.z))
                    .unwrap_or_else(|| DVec3::new(p.pose.x, p.pose.y, p.pose.z));
                (p.pose.id, at)
            })''')
vehicle_eye = '''                let at = DVec3::new(p.pose.x, p.pose.y, p.pose.z);
                crate::traffic::Viewer::lan_player(at, p.pose.heading as f64)'''
assert world.count(vehicle_eye) == 1, 'LAN remote eye mapper changed unexpectedly'
world = world.replace(vehicle_eye,
    '''                let at = p.pose.walker.map(|w| DVec3::new(w.x, w.y, w.z))
                    .unwrap_or_else(|| DVec3::new(p.pose.x, p.pose.y, p.pose.z));
                let heading = p.pose.walker.map(|w| w.heading).unwrap_or(p.pose.heading);
                crate::traffic::Viewer::lan_player(at, heading as f64)''')
world_path.write_text(world, encoding='utf-8', newline='\n')

print('Prepared owner-avatar, on-foot authorization, F1 skin, and player-hosted LAN foot traffic source fixes; validation required.')
