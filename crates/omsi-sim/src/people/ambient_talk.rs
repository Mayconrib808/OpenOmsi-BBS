//! Very occasional, opt-in conversations between nearby *seated* passengers.
//! Uses the passenger's already installed voice pack (no bundled recordings).
//! No scanning on ordinary frames, no audio on a dedicated host or LAN mirror,
//! no changes to tickets, destination planning, or inspector behaviour.
use super::*;
use crate::feature_audit::{record,Role,ResultKind};

const CONVERSATION_PAUSE: f64 = 42.0;
const RESPONSE_AFTER: f64 = 2.4;
const NEAR: f64 = 3.5;
/// Max passengers inspected per sparse conversation opportunity, across all loaded buses.
/// This work is never done on the ordinary rendering frame.
const RIDER_SAMPLE_LIMIT: usize = 64;

pub(crate) fn can_converse(a_bus: BusId, b_bus: BusId, a: DVec3, b: DVec3) -> bool {
    a_bus == b_bus && a.is_finite() && b.is_finite()
        && (a - b).length_squared() <= NEAR * NEAR
}

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
            self.demand_audit.conversation_lines += 1;
            record(Role::Host,"passenger_conversations",ResultKind::Completed,"audio_queued",
                &format!("person={} reply={} voice_file={} audio_status=queued_not_yet_played",person.id,response,path.display()));
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
            // A rider might have alighted, changed buses, or been replaced by a
            // LAN mirror after the first line. Never play an orphaned reply.
            if let Some(i) = self.people.iter().position(|p| {
                p.id == id && matches!(p.place, Place::Bus(bus, _) if p.is_local_seated_passenger(bus))
            }) {
                let played=self.ambient_voice(i, true);
                if !played {record(Role::Host,"passenger_conversations",ResultKind::Blocked,"reply_voice_missing",
                    &format!("speaker={} audio_status=not_queued",id));}
            } else {
                record(Role::Host,"passenger_conversations",ResultKind::Blocked,"reply_passenger_left",
                    &format!("speaker={} audio_status=not_queued",id));
            }
            return;
        }
        if !chat_due(self.time, self.ambient_talk_at, self.last_chat) { return }
        self.ambient_talk_at = self.time;
        self.demand_audit.conversation_attempts += 1;
        // Run once every 42 seconds, not per bus or per frame. Include passengers
        // in timetable AI buses as well as the player's. A dedicated host emits no
        // local sound; a LAN mirror doesn't double-play the host's voices.
        let seated: Vec<(usize, BusId)> = self.people.iter().enumerate()
            .filter_map(|(i, p)| match p.place {
                Place::Bus(bus, _) if p.is_local_seated_passenger(bus) => Some((i, bus)),
                _ => None,
            })
            .take(RIDER_SAMPLE_LIMIT)
            .collect();
        if seated.len() < 2 {
            self.demand_audit.conversation_without_pairs += 1;
            record(Role::Host,"passenger_conversations",ResultKind::NoOpportunity,"insufficient_or_distant_passengers",
                &format!("seated={} max_sample={}",seated.len(),RIDER_SAMPLE_LIMIT));
            return
        }
        let first = (self.rand() as usize) % seated.len();
        let (speaker, bus) = seated[first];
        let nearby: Vec<usize> = seated.iter().copied().filter_map(|(i, other_bus)| {
            (i != speaker && can_converse(bus, other_bus,
                self.people[speaker].position, self.people[i].position)).then_some(i)
        }).collect();
        if nearby.is_empty() {
            self.demand_audit.conversation_without_pairs += 1;
            return
        }
        // Pick both participants rather than always taking the lowest person id.
        let other = nearby[(self.rand() as usize) % nearby.len()];
        record(Role::Host,"passenger_conversations",ResultKind::Executed,"pair_selected",
            &format!("speaker={} partner={} bus={:?} distance_m={:.2}",
                self.people[speaker].id,self.people[other].id,bus,
                (self.people[speaker].position-self.people[other].position).length()));
        if self.ambient_voice(speaker, false) {
            self.ambient_talk_reply = Some((self.people[other].id, self.time + RESPONSE_AFTER));
        } else {
            self.demand_audit.conversation_without_clips += 1;
            record(Role::Host,"passenger_conversations",ResultKind::Blocked,"voice_clip_unavailable",
                &format!("speaker={} partner={} audio_status=not_queued",self.people[speaker].id,self.people[other].id));
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
    #[test]
    fn dialogue_requires_same_bus_and_nearby_real_positions() {
        let p = DVec3::new(12.0, 0.0, 1.0);
        assert!(can_converse(BusId::Player, BusId::Player, p,
            DVec3::new(14.0, 0.0, 1.0)));
        assert!(can_converse(BusId::Ai(3), BusId::Ai(3), p, p));
        assert!(!can_converse(BusId::Ai(3), BusId::Ai(4), p, p));
        assert!(!can_converse(BusId::Player, BusId::Ai(3), p, p));
        assert!(!can_converse(BusId::Player, BusId::Player, p,
            DVec3::new(50.0, 0.0, 1.0)));
        assert!(!can_converse(BusId::Player, BusId::Player, p,
            DVec3::new(f64::NAN, 0.0, 1.0)));
    }
}
