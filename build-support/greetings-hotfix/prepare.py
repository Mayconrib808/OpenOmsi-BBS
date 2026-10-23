"""Cumulative 2.2.0.4 hotfix: host inputs, proximity responses and remote lamp events."""
from pathlib import Path
import hashlib,json
support=Path(__file__).resolve().parent
root=support.parents[2]
game=root/'game'
evidence={}
def put(name,text):
 p=game/name
 before=p.read_bytes()
 p.write_text(text,encoding='utf-8',newline='\n')
 evidence.setdefault(name,{'before':hashlib.sha256(before).hexdigest()})
 evidence[name]['after']=hashlib.sha256(p.read_bytes()).hexdigest()
def edit(name,old,new):
 p=game/name;s=p.read_text(encoding='utf-8')
 assert s.count(old)==1, (name,old[:80],s.count(old))
 put(name,s.replace(old,new))
put('crates/omsi-sim/src/ai_traffic/greetings.rs',(support/'greetings.rs').read_text())
edit('crates/omsi-app/src/app_events/frame/steps.rs',
 'r.last.flags & omsi_net::FLAG_HORN != 0); b.speed = r.last.speed_kmh / 3.6; b }).collect();',
 'r.last.flags & omsi_net::FLAG_HORN != 0); b.high_beam = r.last.head == 3; b.speed = r.last.speed_kmh / 3.6; b }).collect();')
edit('crates/omsi-lan/src/game.rs','table.horn.iter().any(|id| get(*id) > 0.5),',
 'table.horn.iter().any(|id| get(*id) > 0.5) || omsi_sim::ai_traffic::greetings::horn_active(v),')
edit('crates/omsi-lan/src/game.rs','let head = if on("lights_fern") {',
 'let head = if omsi_sim::ai_traffic::greetings::high_active(v) {')
# Keep local and remote lines identical, including script/display-only duties.
edit('crates/omsi-lan/src/game.rs','''    let line = v
        .var("IBIS_Linie_Complex")
        .filter(|x| *x >= 100.0)
        .map(|x| ((x / 100.0) as i32).to_string())
        .unwrap_or_default();''',
 '''    let line = omsi_sim::ai_traffic::greetings::Bus::from_vehicle(
        omsi_sim::ai_traffic::greetings::BusId::Player(0),v,"",false).line;''')
# Track the actual command, including models whose horn uses a nonstandard variable.
edit('crates/omsi-sim/src/vehicle/vars.rs','''    pub fn trigger(&mut self, name: &str) -> bool {
        let p = self.ty.program.clone();''', '''    pub fn trigger(&mut self, name: &str) -> bool {
        let input=name.to_ascii_lowercase();
        if ["horn","hupe","buzina","bus_horn"].contains(&input.as_str()) {
            self.set_engine_var("bbs_input_horn",1.0);
        } else if ["horn_off","hupe_off","buzina_off","bus_horn_off"].contains(&input.as_str()) {
            self.set_engine_var("bbs_input_horn",0.0);
        }
        let p = self.ty.program.clone();''')
edit('crates/omsi-lan/src/game/sound_sync.rs', '''    /// Called for a mirrored AI bus once its sound set is ready.
    pub fn apply_ai_sounds''', '''    /// Lamp events must not wait for audio clips or an enabled sound device.
    pub fn apply_ai_greetings(&mut self, v: &mut omsi_sim::VehicleInstance, ai: u64, root: &Path) {
        self.pending_sounds.retain(|(s, _)| {
            if s.event.ai != ai || s.event.trigger != omsi_sim::ai_traffic::greetings::FLASH_EVENT { return true; }
            if ai != 0 && s.owner == 1 && same_bus(&s.event.bus, &content_relative(&v.ty.def.path,root)) {
                omsi_sim::ai_traffic::greetings::start_remote_flash(v);
            }
            false
        });
    }
    /// Called for a mirrored AI bus once its sound set is ready.
    pub fn apply_ai_sounds''')
edit('crates/omsi-app/src/app_impl/lan_tick.rs', '''                if ready.contains(&c.id) { self.net.remotes.apply_ai_sounds''',
 '''                self.net.remotes.apply_ai_greetings(&mut c.vehicle,c.id,&config.root);
                if ready.contains(&c.id) { self.net.remotes.apply_ai_sounds''')
edit('crates/omsi-app/src/offscreen/step.rs', '''                    if ready.contains(&c.id) { remotes_off.apply_ai_sounds''',
 '''                    remotes_off.apply_ai_greetings(&mut c.vehicle,c.id,&config.root);
                    if ready.contains(&c.id) { remotes_off.apply_ai_sounds''')
edit('crates/omsi-app/src/lan_world.rs', '''            run.par_iter_mut().for_each(|(v, f)| v.update_ai(dt, f));''',
 '''            run.par_iter_mut().for_each(|(v, f)| {
                omsi_sim::ai_traffic::greetings::restore_remote_flash(v);
                v.update_ai(dt,f);
                omsi_sim::ai_traffic::greetings::step_remote_flash(v,dt);
            });''')
# Separate friendly horns from ordinary traffic horns to avoid replaying native AI audio twice.
edit('crates/omsi-sim/src/view_events/sounds.rs',
 '    pub honks: Vec<(bool, DVec3, Arc<VehicleType>)>,',
 '    pub honks: Vec<(bool, DVec3, Arc<VehicleType>)>,\n    pub greeting_honks: Vec<(bool, DVec3, Arc<VehicleType>)>,')
edit('crates/omsi-sim/src/view_events/sounds.rs','        out.honks.clear();','        out.honks.clear();\n        out.greeting_honks.clear();')
edit('crates/omsi-sim/src/view_events/sounds.rs','if greeting && heard { out.honks.push','if greeting && heard { out.greeting_honks.push')
edit('crates/omsi-app/src/view_sync/traffic_audio.rs','        for (heavy, position, ty) in &frame.honks {',
 '''        for ((heavy, position, ty), greeting) in frame.honks.iter().map(|h|(h,false))
            .chain(frame.greeting_honks.iter().map(|h|(h,true))) {''')
# Use each model's sound file when it has a horn, including Brazilian scripts.
edit('crates/omsi-app/src/view_sync/traffic_audio.rs', '''            let own = sound_plan(&mut self.sound_plans, &mut self.sound_cfgs, &ty.def)
                .is_some_and(|p| p.cfg.sounds.iter().any(|s| s.triggers.iter().any(|t| t.trim().eq_ignore_ascii_case("ev_AI_Horn"))));
            if own {
                continue;
            }
            let file = if *heavy { "Sounds/AI_Cars/Hupe_Lkw.wav" } else { "Sounds/AI_Cars/hupe.wav" };
            let path = omsi_cfg::resolve_path(&self.sim.root, file);''',
 '''            if !greeting {
                let own=sound_plan(&mut self.sound_plans,&mut self.sound_cfgs,&ty.def)
                    .is_some_and(|p| p.cfg.sounds.iter().any(|s|s.triggers.iter().any(|t|t.trim().eq_ignore_ascii_case("ev_AI_Horn"))));
                if own { continue; }
            }
            let mut model_horn = None;
            for rel in ty.def.sound_ai.iter().chain(ty.def.sound.iter()) {
                let path=omsi_cfg::resolve_path(ty.def.dir(),rel);
                let cfg=self.sound_cfgs.entry(path.clone()).or_insert_with(||
                    omsi_vehicle::SoundCfg::load(&path).ok().map(Arc::new));
                if let Some(cfg)=cfg {
                    if let Some(entry)=cfg.sounds.iter().find(|e| {
                        e.triggers.iter().chain(e.vol_curves.iter().map(|c| &c.variable))
                            .chain(std::iter::once(&e.file)).any(|n| {
                                let n=n.to_ascii_lowercase();
                                ["horn","hupe","buzina"].iter().any(|k| n.contains(k))
                                    && !["_off","release","end","button"].iter().any(|k|n.contains(k))
                            })
                    }) { model_horn=Some(omsi_cfg::resolve_path(path.parent().unwrap_or(ty.def.dir()),&entry.file));break; }
                }
            }
            let file = if *heavy { "Sounds/AI_Cars/Hupe_Lkw.wav" } else { "Sounds/AI_Cars/hupe.wav" };
            let path = model_horn.unwrap_or_else(||omsi_cfg::resolve_path(&self.sim.root,file));''')
# Keep network lamp refresh tests independent of the local sound mixer.
(root/'out').mkdir(exist_ok=True)
(root/'out/greetings-hotfix-source-sha256.json').write_text(json.dumps(evidence,indent=2)+'\n')
print('Prepared hotfix greetings, model inputs, reliable remote flash events and model-specific horn samples.')
