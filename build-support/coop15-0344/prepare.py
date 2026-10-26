"""Port the fully tested cumulative Coop15 layer onto the exact official 0.3.44 changes.

The official 0.3.44 release modifies only seven files relative to v0.3.43.
Replay the audited complete 0.3.43 layer first (125 files), then merge all
seven target files. Never substitute an unmodified official executable for
the cooperative build or replace the existing custom multiplayer code.
"""
from pathlib import Path
import json
import shutil
import subprocess

root = Path("game")
support = Path("support")
official = Path("official44")
spec = json.loads((support / "build-support/coop15-0344/source.json").read_text(encoding="utf-8"))

def revision(dir):
    return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=dir, text=True).strip()

assert revision(root) == spec["official_base_commit"], "The original 0.3.43 source moved"
assert revision(official) == spec["official_target_commit"], "Official 0.3.44 is not pinned"
# This script checks the original SHA-256 and applies the entire prior audited
# patch, including hand/neck, real handover, fuel, conductor and GUI tests.
subprocess.run(["python", str((support / "build-support/coop15-0343/prepare.py").resolve())], check=True)

unchanged_by_coop = [
    "CHANGELOG.md",
    "crates/omsi-app/src/frame_stages.rs",
    "crates/omsi-app/src/watchdog.rs",
    "crates/omsi-navigator/src/build.rs",
    "crates/omsi-render/src/pipelines/errors.rs",
]
assert sorted(spec["official_changed_paths"]) == sorted(unchanged_by_coop + [
    "crates/omsi-app/src/lib.rs", "crates/omsi-app/src/settings.rs"
])
for path in unchanged_by_coop:
    source = official / path
    dest = root / path
    if path != "crates/omsi-app/src/watchdog.rs":
        previous = subprocess.check_output(
            ["git", "show", "HEAD:" + path], cwd=root)
        # The cumulative layer is not allowed to have changed these sources:
        # replacing them must be identical to applying the genuine 0.3.44 diff.
        assert dest.read_text(encoding="utf-8").replace("\r\n","\n") == previous.decode("utf-8").replace("\r\n","\n"), "Coop layer unexpectedly edits " + path
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(source, dest)
    assert dest.read_bytes() == source.read_bytes()

# Preserve ALL additional cooperative modules while enabling official watchdog.
lib_path = root / "crates/omsi-app/src/lib.rs"
lib = lib_path.read_text(encoding="utf-8")
assert lib.count("mod view_sync;\n") == 1
assert "mod watchdog;" not in lib
lib = lib.replace("mod view_sync;\n", "mod view_sync;\nmod watchdog;\n", 1)
lib_path.write_text(lib, encoding="utf-8")

# Preserve passenger/fuel settings while adding the precise 0.3.44 safe-GPU
# defaults. This is the sole overlap in Settings with our cumulative patch.
settings_path = root / "crates/omsi-app/src/settings.rs"
settings = settings_path.read_text(encoding="utf-8")
old = """        self.mirror_size = self.mirror_size.min(256);
        if n >= 2 {
            self.shadows = false;"""
new = """        self.mirror_size = self.mirror_size.min(256);
        // Official 0.3.44: turn off expensive Enhanced effects after driver loss.
        self.ray_tracing_on = false;
        self.cloud_shadows = false;
        self.light_glow = false;
        self.lamp_shadows = false;
        if self.light_shadows == "all" {
            self.light_shadows = "street".into();
        }
        if n >= 2 {
            // Official 0.3.44: a second device loss falls back to Vanilla+.
            if graphics_mode(&self.graphics) == "enhanced" {
                self.graphics = "vanilla_plus".into();
            }
            self.light_shadows = "off".into();
            self.shadows = false;"""
assert settings.count(old) == 1, "0.3.44 safe-GPU merge anchor changed"
settings = settings.replace(old, new, 1)
settings_path.write_text(settings, encoding="utf-8")

# The logical owner of a delegated bus becomes a passenger. The old owner
# may not recall the wheel; only the active visiting driver can return it.
# Both peers must still be able to exchange authorized conductor messages.
drive_path = root / "crates/omsi-app/src/coop_drive.rs"
drive = drive_path.read_text(encoding="utf-8")
beg = drive.index("    pub(crate) fn drive_return(&mut self) {")
owner_beg = drive.index("        if let Some(o) = self.net.coop.drive.owner.as_ref() {", beg)
owner_end = drive.index("        let Some(mut v) = self.net.coop.drive.visitor.take() else {", owner_beg)
assert drive.count("    pub(crate) fn drive_return(&mut self) {") == 1
drive = (drive[:owner_beg] + """        if self.net.coop.drive.owner_active() {
            // The visitor is now the only driver. The original BCS trip remains
            // passive here, but the former owner cannot reclaim the wheel.
            self.coop_notice(
                "Apenas o novo motorista pode devolver a direção.",
                "Only the current driver can return the wheel.",
                "Nur der aktuelle Fahrer kann die Lenkung zurückgeben.",
            );
            return;
        }
""" + drive[owner_end:])
assert "    pub(crate) fn drive_reset(&mut self" in drive
# The cooperative UI must use public DriveState accessors: the Owner/Visitor
# internals are intentionally private to the transfer state machine.
owner_method = "    pub(crate) fn visitor_owner(&self) -> Option<u32> {"
assert drive.count(owner_method)==1
drive=drive.replace(owner_method, """    pub(crate) fn owner_peer_active(&self) -> Option<u32> {
        self.owner.as_ref().filter(|o| o.active).map(|o| o.peer)
    }
""" + owner_method, 1)
# Revoke any previously issued conductor permission as soon as the new
# driver is confirmed active. The old driver must not keep an old control epoch.
old_active = """            Message::Active { token } => {
                if let Some(o) = self
                    .net
                    .coop
                    .drive
                    .owner
                    .as_mut()
                    .filter(|o| o.peer == from && o.token == token && o.frozen)
                {
                    o.active = true;
                    o.seen = now;
                }
            }"""
new_active = """            Message::Active { token } => {
                let confirmed = if let Some(o) = self.net.coop.drive.owner.as_mut()
                    .filter(|o| o.peer == from && o.token == token && o.frozen) {
                    let first = !o.active;
                    o.active = true;
                    o.seen = now;
                    first
                } else { false };
                if confirmed { self.coop_grant(None); }
            }"""
assert drive.count(old_active) == 1, "Delegated owner activation changed upstream"
drive=drive.replace(old_active,new_active,1)
drive_path.write_text(drive, encoding="utf-8")

coop_path = root / "crates/omsi-app/src/coop.rs"
coop = coop_path.read_text(encoding="utf-8")
old_route = "            BusId::Player => None,"
new_route = """            // Once delegated, the old owner's own bus is a remote control target.
            // Never execute its clicks against the passive local vehicle.
            BusId::Player => self.net.coop.drive.owner_peer_active()
                .map(|peer| (peer, Target::Player)),"""
assert coop.count(old_route) == 1
coop = coop.replace(old_route, new_route, 1)
old_passengers = """                p.pose
                    .walker
                    .and_then(|w| w.aboard)
                    .is_some_and(|a| a.owner == lan.my_id)"""
new_passengers = """                p.pose.walker.and_then(|w| w.aboard).is_some_and(|a| {
                    // The bus keeps the original logical owner on the wire while
                    // another peer drives its real simulation. He is a passenger.
                    a.owner == lan.my_id
                        || self.net.coop.drive.visitor_owner()
                            .is_some_and(|owner| a.owner == owner)
                })"""
assert coop.count(old_passengers) == 1, "Coop passenger selection changed"
coop = coop.replace(old_passengers, new_passengers, 1)
old_inside = "let Some(a) = w.aboard.filter(|a| a.owner == lan.my_id) else {"
new_inside = """let Some(a) = w.aboard.filter(|a| {
                    a.owner == lan.my_id ||
                    self.net.coop.drive.visitor_owner()
                        .is_some_and(|owner| a.owner == owner)
                }) else {"""
assert coop.count(old_inside) == 1, "Delegated-cabin permission check changed"
coop = coop.replace(old_inside, new_inside, 1)
grant_fn = "    pub(crate) fn coop_grant(&mut self, peer: Option<u32>) {"
assert coop.count(grant_fn) == 1
coop = coop.replace(grant_fn, grant_fn + """
        // The former owner no longer authorizes any controls during handover.
        if peer.is_some() && self.net.coop.drive.owner_active() { return; }""", 1)
coop_path.write_text(coop, encoding="utf-8")

panel_path = root / "crates/omsi-app/src/game_panels/coop_panel.rs"
panel = panel_path.read_text(encoding="utf-8")
old_button = """        children.push(button("drive_return", text("Retomar direção (ônibus parado)", "Take back wheel (stopped)", "Lenkung zurücknehmen (im Stand)"), None));"""
new_button = """        children.push(note(text("Você agora é passageiro. Peça ao motorista atual a função de cobrador.",
            "You are now a passenger. Ask the current driver for conductor permission.",
            "Du bist jetzt Fahrgast. Bitte den Fahrer um die Kassiererrolle.")));
        children.push(button("request", text("Solicitar função de cobrador", "Request conductor role", "Kassiererrolle anfragen"), None));"""
assert panel.count(old_button)==1
panel = panel.replace(old_button,new_button,1)
panel_path.write_text(panel, encoding="utf-8")

assert "BusId::Player => self.net.coop.drive.owner_peer_active()" in coop
assert "Only the current driver can return the wheel." in drive
assert "Request conductor role" in panel

# Give native authenticated administrators fine-grained clock steps; regular
# players still cannot issue native admin commands. Dedicated-server BCS
# supervision has a separate, default-off AllowAdminTime setting.
admin_path = root / "crates/omsi-app/src/admin.rs"
admin = admin_path.read_text(encoding="utf-8")
after_hour = '    out.push(("Clock +1 hour".into(), "time 3600".into()));'
assert admin.count(after_hour) == 1
admin = admin.replace(after_hour, after_hour + '''
    for minutes in [1, 5, 10, 15, 17, 30] {
        out.push((format!("Relógio +{minutes} min"), format!("time {}", minutes * 60)));
    }
    for minutes in [1, 5, 10, 15] {
        out.push((format!("Relógio -{minutes} min"), format!("time -{}", minutes * 60)));
    }''', 1)
admin_path.write_text(admin, encoding="utf-8")

# Exact clock setting for authenticated native administrators. The upstream
# web panel already has a server-authorized "Set" button, but only accepts a
# time rounded to the minute. Keep its existing role/auth checks and permit
# precise HH:MM:SS without introducing a privileged client-side shortcut.
web_html_path = root / "crates/omsi-admin/panel/index.html"
web_html = web_html_path.read_text(encoding="utf-8")
input_old = '<input id="clock" type="time" required>'
input_new = '<input id="clock" type="time" step="1" required aria-label="Horário exato (HH:MM:SS)">'
assert web_html.count(input_old) == 1
web_html = web_html.replace(input_old, input_new, 1)
web_html_path.write_text(web_html, encoding="utf-8")
panel_js_path = root / "crates/omsi-admin/panel/panel.js"
panel_js = panel_js_path.read_text(encoding="utf-8")
js_old = '''$("clock-go").addEventListener("click", () => { const v = $("clock").value; if (/^\\d\\d:\\d\\d$/.test(v)) { const [h, m] = v.split(":").map(Number); op({ op: "clock", seconds: h * 3600 + m * 60 }); } });'''
js_new = '''$("clock-go").addEventListener("click", () => {
    const v = $("clock").value.trim();
    // HTML time controls can emit HH:MM or HH:MM:SS; require valid 24-hour
    // civil time. Send an absolute time, never an incremental clock shift.
    if (!/^\\d{2}:\\d{2}(?::\\d{2})?$/.test(v)) return;
    const [h, m, s = 0] = v.split(":").map(Number);
    if (h > 23 || m > 59 || s > 59) return;
    op({ op: "clock", seconds: h * 3600 + m * 60 + s });
  });'''
assert panel_js.count(js_old) == 1, "Upstream admin clock HTML handler changed"
panel_js = panel_js.replace(js_old, js_new, 1)
panel_js_path.write_text(panel_js, encoding="utf-8")
assert 'step="1"' in web_html and 'h * 3600 + m * 60 + s' in panel_js

# An x32 time speed is an opt-in native ADMIN-only command; regular players
# cannot select it because the server's world permission is still enforced.
# Preserve the upstream speed values and extend the existing approved range.
op_path = root / "crates/omsi-admin/src/op.rs"
op = op_path.read_text(encoding="utf-8")
assert op.count('"speed" => Op::Speed(in_range("factor", 1.0, 30.0)?),') == 1
op = op.replace('"speed" => Op::Speed(in_range("factor", 1.0, 30.0)?),', '"speed" => Op::Speed(in_range("factor", 1.0, 32.0)?),',1)
assert op.count('json!({"op": "speed", "factor": 31}),') == 1
op = op.replace('json!({"op": "speed", "factor": 31}),', 'json!({"op": "speed", "factor": 33}),', 1)
op_path.write_text(op, encoding="utf-8")
cfg_path = root / "crates/omsi-lan/src/server_cfg.rs"
cfg_text = cfg_path.read_text(encoding="utf-8")
assert cfg_text.count('.clamp(1.0, 30.0)') == 1
cfg_text=cfg_text.replace('.clamp(1.0, 30.0)', '.clamp(1.0, 32.0)', 1)
cfg_path.write_text(cfg_text, encoding="utf-8")
web_html = web_html.replace('<option>30</option></select>', '<option>30</option><option>32</option></select>',1)
web_html_path.write_text(web_html, encoding="utf-8")
app_admin_path=root / "crates/omsi-app/src/admin.rs"
app_admin=app_admin_path.read_text(encoding="utf-8")
assert app_admin.count('pub(crate) const SPEEDS: [f64; 6] = [1.0, 2.0, 4.0, 8.0, 15.0, 30.0];')==1
app_admin=app_admin.replace('pub(crate) const SPEEDS: [f64; 6] = [1.0, 2.0, 4.0, 8.0, 15.0, 30.0];','pub(crate) const SPEEDS: [f64; 7] = [1.0, 2.0, 4.0, 8.0, 15.0, 30.0, 32.0];',1)
app_admin_path.write_text(app_admin,encoding="utf-8")
lan_panel_path=root / "crates/omsi-lan/src/admin/panel.rs"
lan_panel=lan_panel_path.read_text(encoding="utf-8")
assert lan_panel.count('for s in [1.0, 2.0, 4.0, 8.0, 15.0, 30.0]') ==1
lan_panel=lan_panel.replace('for s in [1.0, 2.0, 4.0, 8.0, 15.0, 30.0]', 'for s in [1.0, 2.0, 4.0, 8.0, 15.0, 30.0, 32.0]',1)
lan_panel_path.write_text(lan_panel,encoding="utf-8")

# Verify that all official 0.3.44 additions were transferred, not just the
# version string. Source identity and patch proof stay independently pinned.
target_lib = (official / "crates/omsi-app/src/lib.rs").read_text(encoding="utf-8")
assert "mod watchdog;" in target_lib and lib.count("mod watchdog;") == 1
target_settings = (official / "crates/omsi-app/src/settings.rs").read_text(encoding="utf-8")
for expression in [
    "self.ray_tracing_on = false;", "self.cloud_shadows = false;",
    "self.light_glow = false;", "self.lamp_shadows = false;",
    'self.graphics = "vanilla_plus".into();',
    'self.light_shadows = "off".into();',
]:
    assert expression in target_settings and expression in settings, expression
for required in ["mod coop;", "mod coop_drive;", "mod coop_view;", "mod mouse_control;"]:
    assert required in lib, "Coop module regressed: " + required
for required in ["passenger_umbrellas", "passenger_hailing_maps", "ticket_chance_percent"]:
    assert required in settings, "Coop setting regressed: " + required
assert not subprocess.check_output(["git", "diff", "--check"], cwd=root, text=True)
print("Authentic upstream 0.3.44 changes merged with all cumulative Coop15, passenger and fuel fixes.")
