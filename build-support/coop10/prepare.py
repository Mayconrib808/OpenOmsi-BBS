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

print('Prepared owner-avatar handover fix. Requires Rust compile and two-player validation.')
