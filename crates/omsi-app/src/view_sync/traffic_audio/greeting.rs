//! The model's full horn set, played by the same runtime as the player's H key.
use super::*;
use std::time::Instant;
pub(crate) struct GreetingHorn {
    set:omsi_audio::SoundSet,cfg:omsi_vehicle::SoundCfg,input:Option<String>,
    sequence:omsi_audio::greeting::PressSequence,last:Instant,pressed:bool,
    attack_events:Vec<String>,release_events:Vec<String>,
}
fn horn(s:&str)->bool {let s=s.to_ascii_lowercase();["horn","hupe","buzina"].iter().any(|k|s.contains(k))}
fn release(s:&str)->bool {let s=s.to_ascii_lowercase();["_off","_aus","release","_end"].iter().any(|k|s.contains(k))}
const LOOP_GATE: &str = "bbs_greeting_loop_gate";
const PRESS: &str = "bbs_horn_press";
const RELEASE: &str = "bbs_horn_release";

fn horn_entry(e:&omsi_vehicle::SoundEntry)->bool {horn(&e.file)||e.triggers.iter().any(|t|horn(t))||e.vol_curves.iter().any(|c|horn(&c.variable))||e.conditions.iter().any(|c|horn(&c.variable))}
impl Traffic {
    pub(super) fn play_greeting_horns(&mut self,audio:&omsi_audio::AudioEngine,frame:&omsi_sim::view_events::SoundFrame,muffled:bool,volume:f32) {
        for &(id,_,_,ref ty) in &frame.greeting_honks {
            if self.greeting_horns.contains_key(&id){continue;}
            let Some(car)=self.sim.cars.iter().find(|c|c.id==id)else{continue};let mut selected=None;
            // Full driver sound first: [sound_ai] often has only the attack file.
            for rel in ty.def.sound.iter().chain(ty.def.sound_ai.iter()) {
                let path=omsi_cfg::resolve_path(ty.def.dir(),rel);
                let Ok(mut cfg)=omsi_vehicle::SoundCfg::load(&path)else{continue};cfg.sounds.retain(horn_entry);
                let dir=path.parent().unwrap_or(ty.def.dir());
                if cfg.sounds.iter().any(|s|audio.load_clip(&omsi_cfg::resolve_path(dir,&s.file)).is_some()){selected=Some((cfg,dir.to_path_buf()));break;}
            }
            let (mut cfg,dir)=selected.unwrap_or_else(||(omsi_vehicle::SoundCfg{sounds:vec![omsi_vehicle::SoundEntry{file:"Sounds/AI_Cars/Hupe_Lkw.wav".into(),volume:1.0,range:30.0,pos:Some([0.0,0.0,0.8]),triggers:vec!["bbs_horn_press".into()],..Default::default()}],..Default::default()},self.sim.root.clone()));
            cfg=cfg.chosen_for(&car.vehicle.number());let mut held=0.10f32;let mut tail=0.0f32;
            for e in &mut cfg.sounds {
                e.pos.get_or_insert([0.0,0.0,0.8]);e.range=e.range.max(24.0);
                let Some(c)=audio.load_clip(&omsi_cfg::resolve_path(&dir,&e.file))else{continue};
                let pitch=if e.is_loop&&e.sample_rate>0.0{(e.sample_rate/c.sample_rate.max(1) as f32).clamp(0.05,8.0)}else{1.0};
                let seconds=c.frames() as f32/c.sample_rate.max(1) as f32/pitch;
                if release(&e.file)||e.triggers.iter().any(|t|release(t)){tail=tail.max(seconds);}
                let is_release = release(&e.file) || e.triggers.iter().any(|t| release(t));
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
                }
            }
            let input=["horn","hupe","buzina","bus_horn"].into_iter().find(|n|car.vehicle.ty.program.trigger(n).is_some()&&car.vehicle.ty.program.trigger(&format!("{n}_off")).is_some()).map(str::to_string);
            log::info!("AI greeting horn: bus={} pattern=2 mechanism=player_soundset input={:?} press_ms=100 one_shot_ms={:.0} release_ms={:.0} complete_samples=true",ty.def.path.display(),input,held*1000.0,tail*1000.0);
            self.greeting_horns.insert(id,GreetingHorn{set:omsi_audio::SoundSet::new_exterior(audio,&cfg,&dir),cfg,input,sequence:omsi_audio::greeting::PressSequence::new(held,tail),last:Instant::now(),pressed:false,attack_events:Vec::new(),release_events:Vec::new()});
        }
        self.greeting_horns.retain(|id,h| {
            let Some(car)=self.sim.cars.iter_mut().find(|c|c.id==*id)else{h.set.stop_all(audio);return false;};
            let now=Instant::now();let dt=now.duration_since(h.last).as_secs_f32().min(0.1);h.last=now;let mut fired=Vec::new();
            for event in h.sequence.advance(dt) {match event {
                omsi_audio::greeting::PressEvent::Button(on)=>{
                    if let Some(input)=&h.input {
                        let n=car.vehicle.host.fired_triggers.len();let capture=car.vehicle.host.capture_sound_triggers;car.vehicle.host.capture_sound_triggers=false;
                        car.vehicle.trigger(&if on{input.clone()}else{format!("{input}_off")});car.vehicle.host.capture_sound_triggers=capture;
                        let pending=if on{&mut h.attack_events}else{&mut h.release_events};pending.extend(car.vehicle.host.fired_triggers.split_off(n));
                    }
                }
                omsi_audio::greeting::PressEvent::Sound(on)=>{
                    h.pressed=on;fired.append(if on{&mut h.attack_events}else{&mut h.release_events});
                    for e in &h.cfg.sounds {for t in &e.triggers {if release(t)!=on&&!fired.contains(t){fired.push(t.clone());}}}
                }
            }}
            h.set.master=volume;h.set.set_muffled(muffled);let pressed=h.pressed;
            h.set.update(audio,&|name|if name == LOOP_GATE || horn(name) {Some(if pressed{1.0}else{0.0})}else{car.vehicle.var(name)},&car.vehicle.world_transform(),&fired);
            if h.sequence.finished(){h.set.stop_all(audio);false}else{true}
        });
    }
}
