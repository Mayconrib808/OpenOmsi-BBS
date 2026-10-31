//! Very occasional, opt-in conversations between nearby *seated* passengers.
//! Uses the passenger's already installed voice pack (no bundled recordings).
//! No scanning on ordinary frames, no audio on a dedicated host or LAN mirror,
//! no changes to tickets, destination planning, or inspector behaviour.
use super::*;

const CONVERSATION_PAUSE: f64 = 42.0;
const RESPONSE_AFTER: f64 = 2.4;
const NEAR: f64 = 3.5;

pub(crate) fn chat_due(now: f64, last: f64, bus_last_chat: f64) -> bool {
    now.is_finite() && now >= 0.0 && now - last >= CONVERSATION_PAUSE
        && now - bus_last_chat >= CHAT_PAUSE
}

impl PeopleSim {
    fn ambient_voice(&mut self, i: usize, response: bool) -> bool {
        // Prefer purpose-recorded clips when supplied by a mod's voice pack.
        // Original OMSI packs often only contain greetings: those are short,
        // harmless exchanges, not invented children's voices.
        let samples: &[&str] = if response {
            &["Chat_2", "SmallTalk_2", "Talk_2", "Hello_2", "GoodMorning_1", "Hello_1"]
        } else {
            &["Chat_1", "SmallTalk_1", "Talk_1", "Hello_1", "GoodEvening_1"]
        };
        for name in samples {
            let Some(path) = self.voice_file(i, name) else { continue };
            if self.voice_said.get(&path).is_some_and(|last| self.time - *last < 12.0 && self.time >= *last) {
                continue;
            }
            let person = &self.people[i];
            self.voice_lines.push(VoiceLine {
                position: person.position + DVec3::new(0.0, 0.0, 1.6),
                speaker: person.id,
                path: path.clone(),
            });
            self.voice_said.insert(path, self.time);
            self.last_chat = self.time;
            return true;
        }
        false
    }

    pub(crate) fn tick_ambient_talk(&mut self) {
        if !self.optional_features.passenger_ambient_talk || self.mirror || self.players_only || self.voices != 0 {
            self.ambient_talk_reply = None;
            return;
        }
        if let Some((id, at)) = self.ambient_talk_reply {
            if self.time < at { return; }
            self.ambient_talk_reply = None;
            if let Some(i) = self.people.iter().position(|p| p.id == id && p.is_local_seated_passenger(BusId::Player)) {
                let _ = self.ambient_voice(i, true);
            }
            return;
        }
        if !chat_due(self.time, self.ambient_talk_at, self.last_chat) { return }
        self.ambient_talk_at = self.time;
        // Run only once per 42 s. Search a bounded number of riders in the
        // player's bus; never scan every AI bus or every pedestrian on the map.
        let seated: Vec<usize> = self.people.iter().enumerate()
            .filter(|(_, p)| p.is_local_seated_passenger(BusId::Player))
            .map(|(i, _)| i).take(20).collect();
        if seated.len() < 2 { return }
        let first = (self.rand() as usize) % seated.len();
        let speaker = seated[first];
        let other = seated.iter().copied().enumerate()
            .filter(|(j,i)| *j != first && (self.people[*i].position - self.people[speaker].position).length() <= NEAR)
            .map(|(_,i)| i).next();
        let Some(other) = other else { return };
        if self.ambient_voice(speaker, false) {
            self.ambient_talk_reply = Some((self.people[other].id, self.time + RESPONSE_AFTER));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn conversations_never_fire_each_frame_or_over_existing_voices() {
        assert!(!chat_due(0.0, 0.0, -1e9));
        assert!(!chat_due(50.0, 10.0, -1e9));
        assert!(!chat_due(50.0, 0.0, 45.0));
        assert!(chat_due(50.0, 0.0, 20.0));
        assert!(!chat_due(f64::NAN, 0.0, 0.0));
    }
}
