//! Reactions use each installed ticket pack's own voice, with bus-wide and rider cooldowns.
use super::*;

// Cosmetic selection must never advance the RNG that controls passenger tasks,
// destinations or ticket choices, even when the optional thanks are enabled.
fn exit_thanks_selected(person: u32, transfer: u64) -> bool {
    let mut value = u64::from(person) ^ transfer.wrapping_mul(0x9e3779b97f4a7c15);
    value = (value ^ (value >> 30)).wrapping_mul(0xbf58476d1ce4e5b9);
    value = (value ^ (value >> 27)).wrapping_mul(0x94d049bb133111eb);
    (value ^ (value >> 31)) % 100 < 22
}
impl PeopleSim {
    pub(super) fn door_reaction(&mut self,i:usize,bn:&BusNow,near_platform:bool,dt:f32) {
        if !self.optional_features.passenger_door_comments { return; }
        let Some(p)=self.pax(i).cloned() else{return;};
        if p.inspector || self.people[i].remote || bn.id!=BusId::Player{return;}
        let exiting=p.task==Task::InBusToExit;
        let waiting_at_stop=p.task==Task::WaitingForBus;
        if waiting_at_stop&&self.visual_hail_bus(i,std::slice::from_ref(bn)).map(|b|b.0)!=Some(BusId::Player){return;}
        let entering=matches!(p.task,Task::WaitingForBus|Task::ToBus|Task::WalkingToBus);
        let side=p.stop.and_then(|stop|self.preferred_entry_side(stop,bn,self.people[i].position));
        // The selected alighting door matters: an open front entrance must not
        // cancel a passenger waiting at the closed rear exit.
        let door=if exiting { p.reaction_exit.or(p.door).or_else(|| bn.cabin.exits.iter().enumerate()
            .min_by(|(_,a),(_,b)| (p.pos-a.inside.as_dvec3()).length_squared()
                .total_cmp(&(p.pos-b.inside.as_dvec3()).length_squared())).map(|(k,_)|k)) }
        else {p.door.or_else(||bn.cabin.entries.iter().enumerate()
            .filter(|(_,d)|super::pax_stops::entry_on_platform(side,d.side)
                &&bn.to_local(self.people[i].position).x*d.side>=0.5)
            .min_by(|(_,a),(_,b)| (self.people[i].position-bn.world(a.outside)).length_squared()
                .total_cmp(&(self.people[i].position-bn.world(b.outside)).length_squared())).map(|(k,_)|k))};
        if exiting && p.reaction_exit.is_none() {if let Some(pax)=self.pax_mut(i) {pax.reaction_exit=door;}}
        let open=door.and_then(|k|if exiting{bn.exit_open.get(k)}else{bn.entry_open.get(k)}).copied().unwrap_or(false);
        let near_door=door.and_then(|k|if exiting{bn.cabin.exits.get(k)}else{bn.cabin.entries.get(k)})
            .is_some_and(|d|if exiting{(p.pos-d.inside.as_dvec3()).length()<1.6}else{(self.people[i].position-bn.world(d.outside)).length()<2.5});
        let has_room=bn.cabin.seats.iter().enumerate().any(|(k,_)|
            !bn.places_off.get(k).copied().unwrap_or(false)
            && !self.seats.get(&bn.id).and_then(|taken|taken.get(k)).copied().unwrap_or(false));
        let capacity=bn.cabin.seats.iter().enumerate().filter(|(k,_)|!bn.places_off.get(*k).copied().unwrap_or(false)).count();
        let aboard=self.people.iter().filter(|person|matches!(person.place,Place::Bus(id,_) if id==bn.id)).take(capacity).count();
        let has_room=has_room && aboard<capacity;
        // Remember the requested stop while approaching it. It need not have
        // stopped or opened any door to detect that this stop was passed.
        if exiting && p.reaction_stop.is_none() && !p.reaction_missed_spoken {
            let target=bn.next_stop.as_ref().filter(|s|p.dest.as_deref().is_none_or(|d|s.is_named(d)))
                .map(|s|s.pos).or_else(||self.stops.values()
                    .filter(|s|p.dest.as_deref().map_or(near_platform,|d|s.is_named(d)))
                    .filter(|s|(s.pos-bn.pos).length()<120.0 && bn.to_local(s.pos).y > -10.0)
                    .min_by(|a,b|(a.pos-bn.pos).length_squared().total_cmp(&(b.pos-bn.pos).length_squared())).map(|s|s.pos));
            if let Some(pax)=self.pax_mut(i) {pax.reaction_stop=target;}
        }
        if exiting && open {
            if let Some(pax)=self.pax_mut(i) {
                pax.door_reaction_wait=0.0;pax.door_reaction_cooldown=0.0;
                pax.reaction_door_was_open=true;
            }
            return;
        }
        let step=dt.max(0.0).min(0.25);
        let shut_on_rider=exiting&&p.reaction_door_was_open&&!open&&near_door;
        // A rider ready to leave sees the front door open, but the exit stays shut.
        let waiting=((entering&&has_room)||(exiting&&bn.entry_open.iter().any(|o|*o)))
            &&near_platform&&near_door&&!open&&bn.speed.abs()<0.28;
        let elapsed=if waiting {p.door_reaction_wait.max(0.0)+step}else{0.0};
        let missed=exiting && !p.reaction_missed_spoken && p.complaint!=3 && !open
            && bn.speed>1.0 && self.pax(i).and_then(|p|p.reaction_stop)
                .is_some_and(|stop|bn.to_local(stop).y < -(bn.half.y as f32+15.0)
                    && (stop-bn.pos).length()<150.0);
        let caption=PassengerDialog::door_reaction(missed,shut_on_rider).id();
        let cooldown=(p.door_reaction_cooldown-dt.max(0.0)).max(0.0);
        if let Some(pax)=self.pax_mut(i) {
            pax.door_reaction_wait=elapsed;
            pax.door_reaction_cooldown=cooldown;
            pax.reaction_door_was_open=open;
        }
        if (!missed && cooldown>0.0) || (!shut_on_rider && !missed && elapsed<2.0){return;}
        if missed {
            if let Some(pax)=self.pax_mut(i) {pax.reaction_missed_spoken=true;}
            let event=Path::new("__bridge_missed_stop_bus__");
            if self.voice_said.get(event).is_some_and(|t|self.time>=*t && self.time-*t<12.0){return;}
            self.voice_said.insert(event.into(),self.time);
        }
        // Bounded bus-wide cooldown: many riders never form an audio/text flood.
        // Missed-stop anger takes precedence over an earlier polite reminder.
        if !missed && self.time-self.last_chat<10.0 && self.time>=self.last_chat{return;}
        if let Some(pax)=self.pax_mut(i) {
            pax.door_reaction_cooldown=13.0;
            if missed {pax.door_reaction_wait=0.0;}
        }
        log::info!("BCS door reaction: person={} mode={} task={:?} near_stop={} door={:?} entry_open={:?} exit_open={:?}",
            self.people[i].id,if missed{"missed_stop"}else if exiting{"exiting"}else{"boarding"},p.task,near_platform,door,bn.entry_open,bn.exit_open);
        if self.say_brazilian_reaction(i, if missed {"MissedStop"}else{"OpenDoor"}) {
            self.dialog=Some(caption.into());return;
        }
        if missed {
            for line in ["Angry_1","Angry_2","Pardon_2","OpenDoor_2"] {
                if self.voice_file(i,line).is_some(){
                    self.say_ex(i,line,true);self.dialog=Some(caption.into());return;
                }
            }
        }
        // Voice packs may provide only one variant. Missing lines are skipped.
        for line in ["OpenDoor_1","OpenDoor_2","OpenDoor_A","Door_1","DoorOpen_1"] {
            if self.voice_file(i,line).is_some(){self.say_ex(i,line,true);self.dialog=Some(caption.into());return;}
        }
        let directory=self.voice_file(i,"Thanks_1").or_else(||self.voice_file(i,"Hello_1")).and_then(|p|p.parent().map(Path::to_path_buf));
        if let Some(directory)=directory {
            for (file,is_dir) in omsi_cfg::vfs::list_dir(&directory).unwrap_or_default().into_iter().take(256) {
                let name=file.to_string_lossy();let lower=name.to_lowercase();
                if !is_dir&&lower.ends_with(".wav")&&["opendoor","dooropen","pleaseopen","tuerauf","türauf","tuer_","tür_","abrirporta","porta_"].iter().any(|s|lower.contains(s)) {
                    self.say_ex(i,&name[..name.len()-4],true);self.dialog=Some(caption.into());return;
                }
            }
        }
        // Stock packs without a door-specific line can still ask for attention
        // in their own voice. The localized hint supplies the actual request.
        if self.voice_file(i,"Pardon_1").is_some(){self.say_ex(i,"Pardon_1",true);}
        else{self.last_chat=self.time;}
        self.dialog=Some(caption.into());
    }
    fn say_brazilian_reaction(&mut self,i:usize,kind:&str)->bool {
        let Some(dir)=self.brazilian_voice_dir.as_ref() else{return false;};
        if self.voices!=0 {return true;}
        let count=if kind=="MissedStop" {2}else{3};
        let variant=(u64::from(self.people[i].id).wrapping_mul(0x9e3779b97f4a7c15)
            ^ (self.time.max(0.0) as u64/10)) % count + 1;
        let path=dir.join(format!("{kind}_{variant}.wav"));
        if !omsi_cfg::vfs::is_file(&path) {return false;}
        self.last_chat=self.time;
        self.voice_lines.push(VoiceLine{position:self.people[i].position+DVec3::Z*1.6,
            speaker:self.people[i].id,path,driver_request:kind!="ExitThanks"});
        true
    }
    pub(super) fn thank_on_exit(&mut self,i:usize){
        if !self.optional_features.passenger_exit_thanks { return; }
        let Some(person) = self.people.get(i) else { return; };
        let transfer = self.pax(i).map_or(0, |p| p.transfer_count as u64);
        if exit_thanks_selected(person.id, transfer) {
            if !self.say_brazilian_reaction(i,"ExitThanks") {self.say_ex(i,"Thanks_1",true);}
            self.dialog=Some(PassengerDialog::ExitThanks.id().into());
        }
    }
}

#[cfg(test)]
#[path = "reaction_options_tests.rs"]
mod optional_tests;
