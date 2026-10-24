"""Coop12 P15: deliver AI greeting horns even without a loaded sound set.

The horn travels as a reserved network sound event. Unlike the headlamp event,
the previous mirror path waited for AI SoundSet readiness, silently dropping
greetings when a modded bus lacked or delayed its sound configuration.
"""
from pathlib import Path

def replace_one(path: Path, before: str, after: str) -> None:
    original = path.read_text(encoding="utf-8")
    count = original.count(before)
    if count != 1:
        raise RuntimeError(f"Unexpected original code in {path}: found {count} matches")
    path.write_text(original.replace(before, after), encoding="utf-8", newline="\n")

def apply_hotfix(game: Path) -> None:
    network = game / "crates/omsi-lan/src/game/sound_sync.rs"
    before = """    /// Lamp events must not wait for audio clips or an enabled sound device.
    pub fn apply_ai_greetings(&mut self, v: &mut omsi_sim::VehicleInstance, ai: u64, root: &Path) {
        self.pending_sounds.retain(|(s, _)| {
            if s.event.ai != ai || s.event.trigger != omsi_sim::ai_traffic::greetings::FLASH_EVENT { return true; }
            if ai != 0 && s.owner == 1 && same_bus(&s.event.bus, &content_relative(&v.ty.def.path,root)) {
                omsi_sim::ai_traffic::greetings::start_remote_flash(v);
            }
            false
        });
    }
"""
    after = """    /// Reserved AI greeting events must not wait for a loaded SoundSet.
    /// Otherwise the horn is lost on modded buses with missing/slow sound.cfg.
    pub fn apply_ai_greetings(&mut self, v: &mut omsi_sim::VehicleInstance, ai: u64, root: &Path) {
        self.pending_sounds.retain(|(s, _)| {
            let horn = s.event.trigger == omsi_sim::ai_traffic::greetings::HORN_EVENT;
            let flash = s.event.trigger == omsi_sim::ai_traffic::greetings::FLASH_EVENT;
            if s.event.ai != ai || (!horn && !flash) { return true; }
            if ai != 0 && s.owner == 1 && same_bus(&s.event.bus, &content_relative(&v.ty.def.path, root)) {
                if horn {
                    // SoundFrame::snapshot_sounds converts this into one spatial
                    // greeting_honk and the presentation layer makes two short taps.
                    v.host.fired_triggers.push(omsi_sim::ai_traffic::greetings::HORN_EVENT.into());
                } else {
                    omsi_sim::ai_traffic::greetings::start_remote_flash(v);
                }
            }
            false
        });
    }
"""
    replace_one(network, before, after)

    audio = game / "crates/omsi-app/src/view_sync/traffic_audio.rs"
    before_audio = """            let path = model_horn.unwrap_or_else(||omsi_cfg::resolve_path(&self.sim.root,file));
            let Some(clip) = audio.load_clip(&path) else { continue };
"""
    after_audio = """            // A selected model horn may reference an absent clip, especially
            // with community OMSI buses. Try the stock AI horn before silence.
            let fallback = omsi_cfg::resolve_path(&self.sim.root, file);
            let clip = model_horn.as_ref().and_then(|path| audio.load_clip(path))
                .or_else(|| audio.load_clip(&fallback));
            let Some(clip) = clip else {
                log::warn!("AI greeting horn unavailable (model and stock) for {}", ty.def.path.display());
                continue;
            };
"""
    replace_one(audio, before_audio, after_audio)
    print("Coop12 P15 horn hotfix: reserved remote event delivered before SoundSet readiness; fallback guarded.")
