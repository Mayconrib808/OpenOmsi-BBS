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


# Reduce network command amplification during an active handover. Most input
# snapshots fit one protocol-14 CMD; their previous 48-byte hex fragmentation
# produced three or more commands every 50 ms and starved the 3-second lease
# during Cloudflare relay. Keep the old, safe fragment protocol as fallback.
share_path = game / 'crates/omsi-lan/src/drive_share.rs'
share = share_path.read_text(encoding='utf-8')
src = 'pub const FRAGMENT_PREFIX: &str = "DRV1F ";'
assert share.count(src) == 1
share = share.replace(src, src + '\npub const COMPACT_INPUT_PREFIX: &str = "DRV1I ";')
src = '''    let wire = message.encode();
    wire.as_bytes().chunks(48).enumerate().map(|(i, bytes)| {'''
repl = '''    if let Message::Input { token, seq, frame } = message {
        let compact = format!("{COMPACT_INPUT_PREFIX}{token:x} {seq:x} {}",
            serde_json::to_string(frame).expect("serializable input frame"));
        if compact.len() <= 125 && !compact.contains('|') && !compact.contains('\\n') {
            return vec![compact];
        }
    }
    let wire = message.encode();
    wire.as_bytes().chunks(48).enumerate().map(|(i, bytes)| {'''
assert share.count(src) == 1
share = share.replace(src, repl)
src = '''    pub fn receive(&mut self, peer: u32, wire: &str, now: u64) -> Option<Message> {
        self.pending.retain'''
repl = '''    pub fn receive(&mut self, peer: u32, wire: &str, now: u64) -> Option<Message> {
        if let Some(data) = wire.strip_prefix(COMPACT_INPUT_PREFIX) {
            if wire.len() > 125 { return None; }
            let mut fields = data.splitn(3, ' ');
            let token = u64::from_str_radix(fields.next()?, 16).ok()?;
            let seq = u64::from_str_radix(fields.next()?, 16).ok()?;
            let frame: Frame = serde_json::from_str(fields.next()?).ok()?;
            let message = Message::Input { token, seq, frame };
            return message.valid().then_some(message);
        }
        self.pending.retain'''
assert share.count(src) == 1
share = share.replace(src, repl)
src = 'let m=Message::Input {token:42,seq:1,frame:Frame::safe()};let parts=fragments(&m,7);assert!(parts.len()>1);'
repl = 'let m=Message::Input {token:42,seq:1,frame:Frame {buttons:vec!["large_valid_button_name_one".into(),"large_valid_button_name_two".into(),"large_valid_button_name_three".into()],..Frame::safe()}};let parts=fragments(&m,7);assert!(parts.len()>1);'
assert share.count(src) == 1
share = share.replace(src, repl)
before_test = '    #[test]\n    fn fragments_from_other_peers_and_expired_requests_cannot_be_combined() {'
test = '''    #[test]
    fn compact_input_uses_one_authenticated_command_and_falls_back_for_large_frames() {
        let m = Message::Input { token: 42, seq: 5, frame: Frame::safe() };
        let parts = fragments(&m, 7);
        assert_eq!(parts.len(), 1);
        assert!(parts[0].len() <= 125);
        assert!(parts[0].starts_with(COMPACT_INPUT_PREFIX));
        assert!(!parts[0].contains('|'));
        let mut inbox = Inbox::default();
        assert_eq!(inbox.receive(2, &parts[0], 100), Some(m));
        assert!(inbox.receive(2, "DRV1I 0 2 {}", 200).is_none());
        assert!(inbox.receive(2, "DRV1I 2 nothex {}", 200).is_none());
    }
'''
assert share.count(before_test) == 1
share = share.replace(before_test, test + before_test)
share_path.write_text(share, encoding='utf-8', newline='\n')

drive_path = game / 'crates/omsi-app/src/coop_drive.rs'
drive = drive_path.read_text(encoding='utf-8')
src = '''        if !wire.starts_with(omsi_lan::drive_share::FRAGMENT_PREFIX) {return false;}'''
repl = '''        if !wire.starts_with(omsi_lan::drive_share::FRAGMENT_PREFIX)
            && !wire.starts_with(omsi_lan::drive_share::COMPACT_INPUT_PREFIX) {return false;}'''
assert drive.count(src) == 1
drive = drive.replace(src, repl)
drive_path.write_text(drive, encoding='utf-8', newline='\n')


# A driver takeover is represented as a *seat occupation* in the existing
# protocol-14 walker tail. 1023 cannot be a normal passenger seat. Its owner
# is still enforced by the lease gate; the marker only affects rendering.
drive_path = game / 'crates/omsi-app/src/coop_drive.rs'
drive = drive_path.read_text(encoding='utf-8')
mark = 'use winit::keyboard::KeyCode;'
assert drive.count(mark) == 1
drive = drive.replace(mark, mark + '\n/// A visual driver seat, never a passenger-seat index.\npub(crate) const DRIVER_SEAT: u16 = 1023;')
mark = '''                    self.coop_camera_load_group(true);
                    self.net.coop.panel=false;'''
repl = '''                    self.coop_camera_load_group(true);
                    // Place this actual avatar beside the steering wheel instead of
                    // leaving its body in a passenger seat while the camera teleports.
                    if let Some(BusId::Ai(id)) = self.foot_bus() {
                        let target = self.coop_vehicle(BusId::Ai(id)).and_then(|bus| {
                            let eye = bus.ty.def.cameras_driver.first()?;
                            let mut local = glam::Vec3::from(eye.pos);
                            local.z -= 1.48;
                            let at = bus.world_transform().transform_point3(local).as_dvec3();
                            Some((local, at))
                        });
                        if let (Some((local, at)), Some(foot)) = (target, self.session.on_foot.as_mut()) {
                            foot.seat = None;
                            foot.inside = Some((BusId::Ai(id), local));
                            foot.pos = at;
                            foot.vel = glam::DVec2::ZERO;
                            foot.transit = None;
                            foot.face_seat = false;
                        }
                    }
                    self.net.coop.panel=false;'''
assert drive.count(mark) == 1, 'New driver camera anchor changed'
drive = drive.replace(mark,repl)
drive_path.write_text(drive,encoding='utf-8',newline='\n')

foot_path = game / 'crates/omsi-app/src/on_foot.rs'
foot = foot_path.read_text(encoding='utf-8')
mark = '''        // (the way it goes, which is not where the body faces when it steps sideways)'''
extra = '''        // The approved visiting driver occupies the wheel, not an arbitrary
        // passenger seat; other clients render them with DriverFigure at the wheel.
        let driving = self.net.coop.drive.visitor_active();
        let aboard = if driving {
            aboard.map(|mut a| { a.seat = Some(crate::coop_drive::DRIVER_SEAT); a })
        } else { aboard };
'''
assert foot.count(mark) == 1
foot = foot.replace(mark,extra+mark)
mark = 'seated: f.seat.is_some(), aboard })'
assert foot.count(mark) == 1
foot = foot.replace(mark, 'seated: f.seat.is_some() || driving, aboard })')
mark = '''        let Some(wk) = wk else { continue };
        // (the way it goes: an older game sends none, then its facing)'''
repl = '''        let Some(wk) = wk else { continue };
        if wk.aboard.is_some_and(|a| a.seat == Some(crate::coop_drive::DRIVER_SEAT)) {
            // Rendered by DriverFigure in the owner's bus; not also as a
            // seated/standing passenger at an unrelated seat.
            h.avatar_remove(REMOTE_KEY + id);
            continue;
        }
        // (the way it goes: an older game sends none, then its facing)'''
assert foot.count(mark) == 1
foot = foot.replace(mark,repl)
foot_path.write_text(foot,encoding='utf-8',newline='\n')

# Existing scene DriverFigure draws the person physically seated at the
# correct wheel with that model's driver pose/animations. Switch its character
# to the approved visitor while the owner's real human avatar is on foot.
character_path = game / 'crates/omsi-app/src/coop_character.rs'
character = character_path.read_text(encoding='utf-8')
mark = '''        let key=(p.uid,self.net.coop.character.chosen.clone());'''
repl = '''        let borrowed = if self.net.coop.drive.owner_active() {
            self.drive_peer().and_then(|peer| {
                self.net.lan.as_ref()?.peers()
                    .find(|p| p.pose.id == peer)
                    .and_then(|p| omsi_net::human_path(&p.pose.figure))
            })
        } else { None };
        let key=(p.uid, borrowed.unwrap_or_else(|| self.net.coop.character.chosen.clone()));'''
assert character.count(mark) == 1
character = character.replace(mark,repl)
character_path.write_text(character,encoding='utf-8',newline='\n')

# Keep the friend's animated driver visible even though the *owner* is walking;
# do not redraw the original owner's body over the new one.
render_path = game / 'crates/omsi-app/src/app_events/frame/driving.rs'
render = render_path.read_text(encoding='utf-8')
mark = 'cx.common.settings.driver && cx.people.on_foot.is_none(), at_eyes,'
assert render.count(mark) == 1
render = render.replace(mark,
    'cx.common.settings.driver && (cx.people.on_foot.is_none() || cx.net.coop.drive.owner_active()), at_eyes,')
render_path.write_text(render,encoding='utf-8',newline='\n')

# Pass temporary occupants through the existing LAN tick for *every* observer.
# The owner's own driver seat is rendered by the player_to_scene rule above;
# remote owners are rendered by view_sync/lan.rs with the visitor's .hum.
lan_path = game / 'crates/omsi-lan/src/game.rs'
lan = lan_path.read_text(encoding='utf-8')
mark = '    pub hide_driver_for: Option<u32>,'
assert lan.count(mark) == 1
lan = lan.replace(mark, mark + '\n    /// Visual driver occupants of remote buses; does not grant control.\n    pub borrowed_drivers: Vec<(u32, String)>,')
lan_path.write_text(lan,encoding='utf-8',newline='\n')

tick_path = game / 'crates/omsi-app/src/app_impl/lan_tick.rs'
tick = tick_path.read_text(encoding='utf-8')
mark = '''        let radio_keyed = self.voice_radio_held();'''
repl = '''        let borrowed_drivers: Vec<(u32, String)> = self.net.lan.as_ref().map(|lan|
            lan.peers().filter_map(|peer| {
                let a = peer.pose.walker?.aboard?;
                (a.seat == Some(crate::coop_drive::DRIVER_SEAT))
                    .then_some((a.owner, peer.pose.figure.clone()))
            }).collect()
        ).unwrap_or_default();
        let radio_keyed = self.voice_radio_held();'''
assert tick.count(mark) == 1
tick = tick.replace(mark, repl)
mark = '''            hide_driver_for,'''
assert tick.count(mark) == 1
tick = tick.replace(mark, mark+'\n            borrowed_drivers,')
tick_path.write_text(tick,encoding='utf-8',newline='\n')
off_path = game / 'crates/omsi-app/src/offscreen/step.rs'
off = off_path.read_text(encoding='utf-8')
mark = '''                hide_driver_for: None,'''
assert off.count(mark)==1
off=off.replace(mark,mark+'\n                borrowed_drivers: Vec::new(),')
off_path.write_text(off,encoding='utf-8',newline='\n')

view_path = game / 'crates/omsi-app/src/view_sync/lan.rs'
view = view_path.read_text(encoding='utf-8')
mark = '''        if v.driver_for.as_deref() != Some(rv.last.figure.as_str()) {'''
repl = '''        let borrowed = frame.borrowed_drivers.iter()
            .find(|(owner, _)| *owner == *id)
            .and_then(|(_, figure)| omsi_net::human_path(figure));
        let actual_figure = borrowed.as_deref().unwrap_or(rv.last.figure.as_str());
        if v.driver_for.as_deref() != Some(actual_figure) {'''
assert view.count(mark) == 1
view = view.replace(mark,repl)
mark = '''            v.driver_for = Some(rv.last.figure.clone());'''
assert view.count(mark)==1
view=view.replace(mark,'            v.driver_for = Some(actual_figure.to_string());')
mark = '''omsi_net::human_path(&rv.last.figure).as_deref(), 1000 + *id as u64);'''
assert view.count(mark)==1
view=view.replace(mark,'''omsi_net::human_path(actual_figure).as_deref(), 1000 + *id as u64);''')
mark='rv.last.walker.is_none() && frame.hide_driver_for != Some(*id), false);'
assert view.count(mark)==1
view=view.replace(mark,
    '(borrowed.is_some() || rv.last.walker.is_none()) && frame.hide_driver_for != Some(*id), false);')
view_path.write_text(view,encoding='utf-8',newline='\n')

print('Prepared Coop 10 real visitor-driver figure, compact CMD, F1, walking AI, human sync; requires two-player validation.')
