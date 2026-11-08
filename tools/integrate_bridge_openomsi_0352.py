#!/usr/bin/env python3
"""Resolve the verified v0.3.52 merge and apply the Bridge gameplay changes."""
from pathlib import Path
import json
import re

ROOT = Path("crates")
CONFLICT = re.compile(r"^<<<<<<< [^\n]+\n(.*?)^=======\n(.*?)^>>>>>>> [^\n]+\n", re.M | re.S)


def edit(path, before, after):
    p = Path(path)
    s = p.read_text(encoding="utf-8")
    assert s.count(before) == 1, (path, before)
    p.write_text(s.replace(before, after), encoding="utf-8")


def resolve(path, choose):
    p = ROOT / path
    text, count = CONFLICT.subn(lambda m: choose(*m.groups()), p.read_text(encoding="utf-8"))
    assert count == 1, path
    p.write_text(text, encoding="utf-8")


def settings(_bridge, official):
    for key, old, new in [
        ("passenger_animation", '"original".into()', '"enhanced".into()'),
        ("passenger_hailing", "false", "true"),
        ("passenger_full_bus_reaction", "false", "true"),
        ("passenger_connections", "false", "true"),
    ]:
        assert f"{key}: {old}" in official
        official = official.replace(f"{key}: {old}", f"{key}: {new}")
    return official.rstrip().removesuffix(" }") + (
        ", optional_features: Default::default(), wheelchair_spawn_percent: 3.0,"
        " illegal_parking_frequency: 0.8, passenger_umbrellas: true,"
        " ticket_chance_enabled: false, ticket_chance_percent: -1.0,"
        " passenger_hailing_maps: omsi_launcher_lib::passenger_defaults::MAPS.into() }\n"
    )


resolve("omsi-app/src/admin.rs", lambda a, b: a[a.index('    out.push(("Clock +1 hour"'):].replace("out.push", "clock_rows.push"))
resolve("omsi-app/src/game_panels/quick_menu.rs", lambda a, b: a.splitlines(keepends=True)[0] + b)
resolve("omsi-app/src/offscreen/setup.rs", lambda a, b: a.replace("settings.nav_arrows", "settings.nav_map_arrows"))
resolve("omsi-app/src/settings.rs", settings)
resolve("omsi-sim/src/ai_traffic/model.rs", lambda a, b: "".join(a.splitlines(keepends=True)[:2]) + b)
resolve("omsi-sim/src/human/tests.rs", lambda a, b: a + "}\n\n#[test]\n" + b)

engine = Path("ENGINE_BASE.json")
meta = json.loads(engine.read_text(encoding="utf-8"))
meta.update(version="0.3.52", upstream_tag="v0.3.52", upstream_commit="41d69ab19043656b49d6cad0c34f85f3178503b8")
engine.write_text(json.dumps(meta, indent=2) + "\n", encoding="utf-8")
build = Path("crates/omsi-platform/build.rs")
build.write_text(build.read_text(encoding="utf-8").replace("0.3.50", "0.3.52").replace("full v0.3.49 engine", "full v0.3.52 engine"), encoding="utf-8")

quick = "crates/omsi-app/src/game_panels/quick_menu.rs"
edit(quick, '    ("stats", "Statistics",', '''    ("preheat", "Preheat the bus to 19 °C", "thermostat"),
    ("clock_plus_minute", "+1m", "clock_plus_minute"),
    ("stats", "Statistics",''')
edit(quick, "        let mut lit = Vec::new();", '''        if app.player.is_none() || app.session.on_foot.is_some() {
            off.push("preheat");
        }
        if app.net.lan.is_some()
            && !(crate::admin_panel::remote(app) && app.net.admin_panel.may_world())
        {
            off.push("clock_plus_minute");
        }
        let mut lit = Vec::new();''')
edit(quick, "    let kept = app.menus.game_panels.quick.take();", '''    if matches!(id, "preheat" | "clock_plus_minute") && TileState::of(app).off.contains(&id) {
        return;
    }
    let kept = app.menus.game_panels.quick.take();''')
edit(quick, '        "wheelchair" => app.menus.game_panels.wheelchair = true,', '''        "wheelchair" => app.menus.game_panels.wheelchair = true,
        "preheat" => {
            if let Some(p) = app.player.as_mut() {
                p.vehicle.set_cabin_temperature(19.0);
                app.service_msg = Some((tr("Cabin preheated to 19 °C").into_owned(), 3.0));
            }
        }
        "clock_plus_minute" => {
            if app.net.lan.is_some() {
                crate::admin::run(app, "time 60");
            } else {
                app.shift_clock(60.0);
            }
        }''')
edit(quick, "        assert_eq!(pages(), 2);", "        assert_eq!(pages(), 3);")
edit(quick, "        assert_eq!((q.page, q.focus), (1, 4));", "        assert_eq!((q.page, q.focus), (pages() - 1, 3));")
edit(quick, "        for k in [K::ArrowDown, K::ArrowDown, K::ArrowRight] {",
     "        for k in [K::ArrowDown, K::ArrowDown, K::ArrowRight, K::ArrowRight] {")
edit(quick, '''        click(&mut app, "next");
        assert_eq!(app.menus.game_panels.quick.as_ref().map(|q| q.page), Some(0));''',
     '''        for page in [2, 0] {
            click(&mut app, "next");
            assert_eq!(app.menus.game_panels.quick.as_ref().map(|q| q.page), Some(page));
        }''')
edit(quick, "    #[test]\n    fn a_tile_leaves_the_pause_as_it_was()", '''    #[test]
    fn offline_plus_minute_advances_the_clock_without_admin() {
        let mut app = app();
        let before = app.clock.time;
        assert!(!TileState::of(&app).off.contains(&"clock_plus_minute"));
        click(&mut app, "clock_plus_minute");
        assert!((app.clock.time - (before + 60.0).rem_euclid(86400.0)).abs() < 1e-6);
        assert!(TileState::of(&app).off.contains(&"preheat"));
    }

    #[test]
    fn a_tile_leaves_the_pause_as_it_was()''')

edit("crates/omsi-lan/src/admin/panel.rs", "    fn may_world(&self)", "    pub fn may_world(&self)")
edit("crates/omsi-app/src/coop_drive.rs", '"wash"|"wheelchair"|"vehicle"', '"wash"|"wheelchair"|"preheat"|"vehicle"')
with Path("crates/omsi-app/locales/app.yml").open("a", encoding="utf-8") as f:
    f.write('''
"Preheat the bus to 19 °C":
  pt: "Pré-aquecer o ônibus a 19 °C"
  pt-pt: "Pré-aquecer o autocarro a 19 °C"
  de: "Bus auf 19 °C vorwärmen"
  es: "Precalentar el autobús a 19 °C"
  fr: "Préchauffer le bus à 19 °C"

"Cabin preheated to 19 °C":
  pt: "Interior do ônibus a 19 °C"
  pt-pt: "Interior do autocarro a 19 °C"
  de: "Innenraum auf 19 °C vorgewärmt"
  es: "Interior del autobús a 19 °C"
  fr: "Habitacle préchauffé à 19 °C"
''')
Path("assets/icons/custom/clock_plus_minute.svg").write_text('''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
  <path fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"
    d="M2 13h6m-3-3v6m6-5 2-2v8m4 0v-5m0 1.5c0-2 2.5-2 2.5 0V17m0-3.5c0-2 2.5-2 2.5 0V17"/>
</svg>
''', encoding="utf-8")

edit("crates/omsi-sim/src/people/reactions.rs",
     'if !self.say_brazilian_reaction(i,"ExitThanks") {self.say_ex(i,"Thanks_1",true);}',
     '// The ticket-payment sound can be coins instead of speech.\n            self.say_brazilian_reaction(i, "ExitThanks");')

clips = "crates/omsi-sim/src/human/clips.rs"
edit(clips, "    pub fn seconds(&self)", '''    /// Small change in torso pitch from a seated recording. The rider's IK still
    /// solves contact with the handrims and footrests.
    pub fn seated_lean(&self, seconds: f32) -> f32 {
        let start = self.frames[0][SPINE];
        let now = self.ping_pong(seconds)[SPINE];
        let (pitch, _, _) = model(now * start.inverse()).to_euler(glam::EulerRot::XYZ);
        pitch.to_degrees().clamp(-3.0, 3.0)
    }

    pub fn seconds(&self)''')
avatar = "crates/omsi-app/src/view_sync/people/avatars.rs"
edit(avatar, "fn weight(id: &str)", '''pub(super) fn seated_lean(id: u32, ty: &HumanType, seconds: f32) -> f32 {
    let Some(pool) = ready() else { return 0.0 };
    if ty.biped.is_none() {
        return 0.0;
    }
    let clips = &pool.motions[is_female(ty) as usize][Kind::Sit as usize];
    if clips.is_empty() {
        return 0.0;
    }
    let seed = (u64::from(id).wrapping_mul(0x9E37_79B9_7F4A_7C15) >> 32) as usize;
    clips[seed % clips.len()].seated_lean(seconds + (seed % 120) as f32)
}

fn weight(id: &str)''')
chair = "crates/omsi-app/src/view_sync/people/wheelchair.rs"
edit(chair, "    let input=omsi_sim::people::wheelchair::pose::input", "    let mut input=omsi_sim::people::wheelchair::pose::input")
edit(chair, "    if st.procedural.is_none() {", '''    st.clip_t += dt;
    // Fade the idle motion under active pushes. Apply it before solving the grips.
    let lean = super::avatars::seated_lean(p.id, ty, st.clip_t);
    input.grip_lean += (1.0 - chair.moving.clamp(0.0, 1.0)) * (3.0 + lean);
    if st.procedural.is_none() {''')

sound = "crates/omsi-app/src/view_sync/traffic_audio/greeting.rs"
edit(sound, "fn horn_entry(", '''const LOOP_GATE: &str = "bbs_greeting_loop_gate";
const PRESS: &str = "bbs_horn_press";
const RELEASE: &str = "bbs_horn_release";

fn horn_entry(''')
edit(sound, "e.vol_curves.iter().any(|c|horn(&c.variable))}",
     "e.vol_curves.iter().any(|c|horn(&c.variable))||e.conditions.iter().any(|c|horn(&c.variable))}")
edit(sound, "let mut held=0.18f32;", "let mut held=0.10f32;")
edit(sound, '''                else if e.is_loop&&!e.no_loop{held=held.max(seconds.clamp(0.18,0.45));}else{held=held.max(seconds);}''',
'''                let is_release = release(&e.file) || e.triggers.iter().any(|t| release(t));
                if e.triggers.is_empty() && (is_release || e.no_loop) {
                    e.triggers.push(if is_release { RELEASE } else { PRESS }.into());
                }
                if e.triggers.is_empty() {
                    e.conditions.push(omsi_vehicle::sound::Condition {
                        variable: LOOP_GATE.into(), value: 1.0,
                        kind: omsi_vehicle::sound::ConditionKind::Bool, ..Default::default()
                    });
                } else if !is_release {
                    held = held.max(seconds);
                }''')
edit(sound, 'if horn(name){Some(if pressed{1.0}else{0.0})}else{car.vehicle.var(name)}',
     'if name == LOOP_GATE || horn(name) {Some(if pressed{1.0}else{0.0})}else{car.vehicle.var(name)}')
edit(sound, "sample_ms={:.0}", "one_shot_ms={:.0}")
audio = "crates/omsi-audio/src/greeting.rs"
edit(audio, '''        let sample=if sample.is_finite(){sample.clamp(0.18,20.0)}else{0.18};
        let tail=if tail.is_finite(){tail.clamp(0.0,20.0)}else{0.0};let again=sample+tail+0.08;''',
'''        let audible = sample.max(0.10 + tail);
        let again = audible + 0.10;''')
edit(audio, '''(0.10,Button(false)),(sample,Sound(false)),(again,Button(true)),(again,Sound(true)),(again+0.10,Button(false)),(again+sample,Sound(false))],end:again+sample+tail+0.05''',
'''(0.10,Button(false)),(0.10,Sound(false)),(again,Button(true)),(again,Sound(true)),(again+0.10,Button(false)),(again+0.10,Sound(false))],end:again+audible+0.05''')
edit(audio, '''            assert!(sounds[1].0-sounds[0].0>=sample-0.02);assert!(sounds[3].0-sounds[2].0>=sample-0.02);
            assert!(sounds[2].0-sounds[1].0>=tail+0.06);assert!(s.finished());''',
'''            for tap in sounds.chunks_exact(2) {
                assert!((tap[1].0 - tap[0].0 - 0.10).abs() < 0.02);
            }
            assert!(sounds[2].0 - sounds[0].0 >= sample.max(0.10 + tail) + 0.08);
            assert!(s.finished());''')

edit("crates/omsi-sim/src/vehicle/tests/scripts_props.rs",
     '        file: PathBuf::new(),\n        materials: ["film.png",',
     '        file: PathBuf::new(),\n        pick_geometry: Default::default(),\n        materials: ["film.png",')
edit(audio, '''        if dt.is_finite()&&dt>0.0{self.at+=dt;}let mut out=Vec::new();
        while self.next<8&&self.at>=self.edges[self.next].0{out.push(self.edges[self.next].1);self.next+=1;}out''',
'''        if dt.is_finite() && dt > 0.0 {
            self.at += dt;
        }
        let Some(&(at, _)) = self.edges.get(self.next) else { return Vec::new() };
        if self.at < at {
            return Vec::new();
        }
        // Apply one edge at a time, even after a render stall. A press and its
        // release must reach the mixer in different updates.
        let mut events = Vec::new();
        while self.next < self.edges.len() && self.edges[self.next].0 == at {
            events.push(self.edges[self.next].1);
            self.next += 1;
        }
        let late = self.at - at;
        for (time, _) in &mut self.edges[self.next..] {
            *time += late;
        }
        self.end += late;
        events''')
