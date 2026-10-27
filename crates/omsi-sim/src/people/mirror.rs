//! LAN play (see `lan_world`): a host keeps people around every player, tells the clients
//! where they are and hands the waiting ones over to a client's bus; a client draws the
//! host's people instead of its own and simulates only those who board its bus.

use super::*;
use super::pax_stops::entry_on_platform;

/// Where one of the host's people is this frame, as a client draws them.
#[derive(Debug, Clone, Copy)]
pub struct MirrorPose {
    pub pos: DVec3,
    pub heading: f64,
    pub vel: DVec2,
    pub activity: Activity,
    /// Aboard a timetable bus: (its id, the point of its frame, heading in its frame, the
    /// seat or standing place, if known).
    pub aboard: Option<(u64, Vec3, f64, Option<usize>)>,
    /// Waiting at a stop: (the stop object, the waiting place).
    pub waiting: Option<(i64, usize)>,
}

/// The bus id (`BusId::Ai`) another LAN player's bus has among the buses here: far above
/// the traffic's car ids.
pub fn remote_bus_id(player: u32) -> u64 {
    (1 << 40) | player as u64
}

/// The bus id (`BusId::Ai`) of a vehicle the player placed (`Player::uid`).
pub fn placed_bus_id(uid: u64) -> u64 {
    (2 << 40) | uid
}

/// The player whose bus `remote_bus_id` gave this id (None for a traffic bus).
pub fn remote_bus_player(bus: u64) -> Option<u32> {
    (bus >> 40 == 1).then_some((bus & 0xFFFF_FFFF) as u32)
}

/// One of the host's people as it tells the clients.
pub struct LanPerson {
    pub id: u32,
    pub ty: Arc<HumanType>,
    pub pos: DVec3,
    pub heading: f64,
    pub speed: f64,
    pub activity: Activity,
    pub aboard: Option<(u64, Vec3, f64, Option<usize>)>,
    pub waiting: Option<(i64, usize)>,
}

#[cfg(test)]
mod service_intent_2212_tests{
    use super::*;
    #[test] fn line_identity_ignores_loading_order_but_not_destinations(){
        let a:HashSet<String>=["Terminal A","Terminal B"].into_iter().map(str::to_string).collect();
        let b:HashSet<String>=["Terminal B","Terminal A"].into_iter().map(str::to_string).collect();
        let c:HashSet<String>=["Terminal C"].into_iter().map(str::to_string).collect();
        assert_eq!(PeopleSim::hail_line_key(&a),PeopleSim::hail_line_key(&b));
        assert_ne!(PeopleSim::hail_line_key(&a),PeopleSim::hail_line_key(&c));
    }
}
impl PeopleSim {
    /// Exact individual service preference, supplied only by the authoritative host.
    pub fn mirror_bus_fit(&self,id:u32,bn:&BusNow,position:DVec3)->Option<bool>{
        let (stop,key,line,dest)=self.mirror_hail_intent.get(&id)?;
        if key.is_some() && line.is_none(){return Some(false)}
        if self.mirror_wait.get(&id)?.0!=*stop{return None}
        let s=self.stops.get(stop)?;
        if bn.served.is_some_and(|v|v!=*stop) || bn.route.as_ref().is_some_and(|r|r.is_last_stop(s)){return Some(false)}
        let Some(term)=bn.terminus.as_deref() else{return Some(false)};
        if !matches!(at_stop(*stop,s,Some(term),&bn.takes),AtStop::Serves){return Some(false)}
        let side=self.preferred_entry_side(*stop,bn,position);
        if !bn.cabin.entries.iter().any(|d|d.point.is_some() && entry_on_platform(side,d.side)){return Some(false)}
        Some(match line.and_then(|k|s.lines.get(k)){
            Some((_,termini))=>fit(*stop,s,dest.as_deref(),termini,term,&bn.takes)
                .or_else(||bn.route.as_ref().and_then(|r|r.takes_waiting_for(termini))).is_some(),
            None=>line.is_none(),
        })
    }
    /// Stable identity of the termini, independent of HashMap loading/line order.
    pub fn hail_line_key(termini:&HashSet<String>)->u64{
        let mut names:Vec<_>=termini.iter().map(|s|s.trim()).collect();names.sort_unstable();
        let mut h=0xcbf29ce484222325u64;
        for name in names{for byte in name.bytes().chain(std::iter::once(0)){h^=byte as u64;h=h.wrapping_mul(0x100000001b3);}}
        h
    }
    pub fn hail_intent(&self,id:u32)->Option<(i64,Option<u64>,Option<String>)>{
        let p=self.people.iter().find(|p|p.id==id && !p.remote)?;
        let State::Pax(p)=&p.state else{return None};
        let stop=p.stop?;
        let key=match p.line{Some(k)=>Some(Self::hail_line_key(&self.stops.get(&stop)?.lines.get(k)?.1)),None=>None};
        Some((stop,key,p.dest.clone()))
    }
    /// The native client has finished walking to a connecting stop. Only after the
    /// host confirms taking ownership may we retire this client-owned passenger.
    pub fn finish_returned_connection(&mut self, wire_id: u32) -> bool {
        let Some(i) = self.people.iter().position(|person| {
            if person.remote || person.puppet.is_some() { return false; }
            let id = if person.id >= (1 << 30) {
                ((person.id - (1 << 30)) & 0x7F_FFFF) | 0x80_0000
            } else { person.id & 0x7F_FFFF };
            id == wire_id && matches!(&person.state, State::Pax(p)
                if p.task == Task::WaitingForBus && p.transfer_from == Some(BusId::Player)
                    && p.transfer_count > 0)
        }) else { return false; };
        self.release(i);
        let person = self.people.swap_remove(i);
        self.retire(&person);
        true
    }

    /// Promote a client-owned connection to a real host passenger. The host chooses
    /// a free waiting place and new world id (never trusts either from the client).
    pub fn adopt_returned_connection(&mut self, world: &dyn World, kind: usize,
        stop: i64, line_key: Option<u64>, destination: Option<String>, peer: u32,
        position: DVec3, heading: f64) -> Option<u32> {
        if !position.is_finite() || !heading.is_finite() { return None; }
        let s = self.stops.get(&stop)?;
        if (s.gather - position).length() > 40.0 { return None; }
        let dest = destination?.trim().to_string();
        if dest.is_empty() || dest.len() > 180 { return None; }
        let key = line_key?;
        let line = s.lines.iter().position(|(name, terms)|
            name.trim() == dest && Self::hail_line_key(terms) == key)?;
        if !s.dests.iter().any(|(name, weight)| name.trim() == dest && *weight > 0.0) {
            return None;
        }
        if kind >= self.types.len() || !self.pool_room() { return None; }
        let spot = self.take_spot(stop)?;
        let sp = self.stops.get(&stop)?.spots.get(spot)?.clone();
        let walk = 1.1 + (self.rand_f() as f32 * 2.0 - 1.0) * 0.2;
        let r = self.rand_f();
        let mut pax = Pax::new(walk, r);
        pax.task = Task::WaitingForBus;
        pax.stop = Some(stop);
        pax.spot = Some(spot);
        pax.dest = Some(dest);
        pax.line = Some(line);
        pax.transfer_from = Some(BusId::Ai(remote_bus_id(peer)));
        pax.transfer_count = 1;
        pax.ride_km = self.rand_f() as f32 * 19.0 + 1.0;
        pax.pos = sp.pos;
        pax.yaw = sp.face.to_radians();
        if sp.height != 0.0 {
            pax.seat_h = sp.height;
            pax.pos -= DVec3::Z * self.types[kind].def.seat_height as f64;
            pax.pax_state = 2.0;
        }
        let at = pax.pos;
        let yaw = sp.face;
        let i = match self.spawn_as(world, at, yaw, State::Pax(Box::new(pax)), Some(kind)) {
            Some(i) => i,
            None => { self.free_spot(stop, spot); return None; }
        };
        let id = self.people[i].id;
        Some(id)
    }

    pub fn mirror_hail_intent(&mut self,id:u32,intent:Option<(i64,Option<u64>,Option<String>)>){
        if let Some((stop,key,dest))=intent{
            if self.mirror_hail_intent.get(&id).is_some_and(|old|old.0==stop && old.1==key && old.3==dest && (key.is_none()||old.2.is_some())){return}
            let line=key.and_then(|key|self.stops.get(&stop)?.lines.iter().position(|(_,terms)|Self::hail_line_key(terms)==key));
            self.mirror_hail_intent.insert(id,(stop,key,line,dest));
        }else{self.mirror_hail_intent.remove(&id);}
    }

    /// Is person `id` (one drawn for another game) here?
    pub fn has_mirror(&self, id: u32) -> bool {
        self.people.iter().any(|p| p.id == id && p.remote)
    }

    /// The riders of our own bus, for the other LAN players to see: (id, type, place in the
    /// bus frame, heading there, seat, activity).
    pub fn lan_riders(&self) -> Vec<LanPerson> {
        self.people
            .iter()
            .filter(|p| p.puppet.is_none() && !p.remote)
            .filter_map(|p| match p.place {
                Place::Bus(BusId::Player, l) => Some(LanPerson {
                    id: p.id,
                    ty: p.ty.clone(),
                    pos: p.position,
                    heading: p.heading,
                    speed: 0.0,
                    activity: p.activity,
                    aboard: Some((
                        0,
                        l,
                        p.lheading,
                        match &p.state {
                            State::Pax(x) if x.task == Task::SittingInBus => x.seat,
                            _ => None,
                        },
                    )),
                    waiting: None,
                }),
                _ => None,
            })
            .collect()
    }

    /// The places people are kept around: ours (the player's bus, else the camera) - not on
    /// a dedicated server, where nobody plays there (`players_only`) - and every other LAN
    /// player's.
    pub fn anchors(&self) -> impl Iterator<Item = DVec3> + '_ {
        (!self.players_only)
            .then_some(self.center)
            .into_iter()
            .chain(self.lan_centers.iter().copied())
    }

    /// Is `p` further than `r` from us and from every other LAN player?
    pub fn far_from_players(&self, p: DVec3, r: f64) -> bool {
        self.anchors().all(|c| (p - c).length() > r)
    }

    /// The stops and pavements around the other players of a LAN session (host).
    pub fn populate_lan_centers(
        &mut self,
        world: &dyn World,
        net: &Network,
    ) {
        if self.lan_centers.is_empty() {
            return;
        }
        let mine = self.center;
        for c in self.lan_centers.clone() {
            // (near us they are there already - unless nobody plays here)
            if !self.players_only && (c - mine).length() < 150.0 {
                continue;
            }
            self.populate_with(world, Some(net), c);
            self.populate_on_foot(world, net, 1.0);
        }
        self.center = mine;
    }

    /// Everybody within `radius` of `near` the clients may see (host): on foot, waiting at
    /// a stop, or aboard a timetable bus - not the riders of our own bus, which the others
    /// see from outside only.
    pub fn lan_people(&self, near: DVec3, radius: f64) -> Vec<LanPerson> {
        let r2 = radius * radius;
        self.people
            .iter()
            .filter(|p| p.puppet.is_none() && !p.remote)
            .filter(|p| (p.position - near).length_squared() < r2)
            .filter_map(|p| {
                let aboard = match p.place {
                    Place::Bus(BusId::Player, _) => return None,
                    Place::Bus(BusId::Ai(bus), l) => Some((
                        bus,
                        l,
                        p.lheading,
                        match &p.state {
                            State::Pax(x) if x.task == Task::SittingInBus => x.seat,
                            _ => None,
                        },
                    )),
                    Place::Ground => None,
                };
                let waiting = match &p.state {
                    State::Pax(x) if aboard.is_none() && x.task == Task::WaitingForBus => {
                        if self.mirror {
                            if x.transfer_from == Some(BusId::Player) && x.transfer_count > 0 {
                                x.stop.map(|stop| (stop, x.spot.unwrap_or(255)))
                            } else { None }
                        } else { x.stop.zip(x.spot) }
                    },
                    _ => None,
                };
                Some(LanPerson {
                    id: p.id,
                    ty: p.ty.clone(),
                    pos: p.position,
                    heading: p.heading,
                    speed: if aboard.is_some() { 0.0 } else { p.vel.length() },
                    activity: p.activity,
                    aboard,
                    waiting,
                })
            })
            .collect()
    }

    /// A client's bus takes these waiting people (host): those still waiting leave our
    /// world (they are the client's now); returns them. Somebody who has meanwhile walked
    /// up to another bus stays ours.
    pub fn hand_over(&mut self, player: u32, ids: &[u32]) -> Vec<u32> {
        let mut out = Vec::new();
        for id in ids {
            let Some(i) = self.people.iter().position(|p| p.id == *id) else {
                continue;
            };
            if !matches!(&self.people[i].state, State::Pax(x) if x.task == Task::WaitingForBus) || self.people[i].remote {
                continue;
            }
            // (still counted at their stop while that bus stands there, as the people
            // boarding a bus of ours are: see `handed`)
            if let Some(stop) = self.pax(i).and_then(|x| x.stop) {
                self.handed.push((stop, remote_bus_id(player)));
            }
            self.release(i);
            let p = self.people.swap_remove(i);
            self.retire(&p);
            out.push(*id);
        }
        out
    }

    /// Draw the host's people from now on (`on`), or simulate our own again. Everybody who
    /// is not getting on, riding or getting off our bus goes (the host's come instead; the
    /// host's copies cannot walk on by themselves).
    pub fn set_mirror(&mut self, on: bool) {
        if self.mirror == on {
            return;
        }
        self.mirror = on;
        let keep = |p: &Person| !p.remote && p.state.bus() == Some(BusId::Player);
        let mut i = 0;
        while i < self.people.len() {
            if keep(&self.people[i]) || self.people[i].puppet.is_some() {
                i += 1;
                continue;
            }
            self.release(i);
            let p = self.people.swap_remove(i);
            self.retire(&p);
        }
        // our own people from now on are numbered far above the host's (those riding with
        // us already too)
        if on {
            self.next_id = self.next_id.max(1 << 30);
            for p in self.people.iter_mut().filter(|p| p.puppet.is_none()) {
                self.bodies.ops.push(BodyOp::Renamed { from: p.id, to: self.next_id });
                p.id = self.next_id;
                self.next_id += 1;
            }
        }
        for s in self.stops.values_mut() {
            for t in s.taken.iter_mut() {
                *t = false;
            }
        }
        self.claims_out.clear();
        self.claimed.clear();
        self.mirror_wait.clear();
        self.mirror_hail_intent.clear();
    }

    /// The human type of a file relative to a content root (`Humans/…/x.hum`).
    pub fn type_by_file(&self, file: &str) -> Option<usize> {
        let want = file.replace('\\', "/").to_ascii_lowercase();
        self.types.iter().position(|t| {
            t.def
                .path
                .to_string_lossy()
                .replace('\\', "/")
                .to_ascii_lowercase()
                .ends_with(&want)
        })
    }

    /// The file of a human type relative to its content root (`Humans/…/x.hum`).
    pub fn type_file(ty: &HumanType) -> String {
        let p = ty.def.path.to_string_lossy().replace('\\', "/");
        match p.to_ascii_lowercase().rfind("/humans/") {
            Some(k) => p[k + 1..].to_string(),
            None => p,
        }
    }

    /// One of the host's people appears here (client).
    pub fn mirror_add(
        &mut self,
        world: &dyn World,
        id: u32,
        ty: usize,
        pose: &MirrorPose,
    ) -> bool {
        if self.people.iter().any(|p| p.id == id) {
            return false;
        }
        let state = State::Idle;
        let Some(i) =
            self.spawn_as(world, pose.pos, pose.heading, state, Some(ty))
        else {
            return false;
        };
        self.next_id -= 1;
        let p = &mut self.people[i];
        self.bodies.renamed(p.id, id);
        p.id = id;
        p.anim = OmsiAnim::default();
        p.remote = true;
        self.mirror_set(id, pose);
        true
    }

    /// Where one of the host's people is this frame (client).
    pub fn mirror_set(&mut self, id: u32, pose: &MirrorPose) {
        let Some(p) = self.people.iter_mut().find(|p| p.id == id && p.remote) else {
            return;
        };
        p.position = pose.pos;
        p.heading = pose.heading;
        p.vel = pose.vel;
        p.activity = pose.activity;
        match pose.waiting {
            Some(w) => {
                self.mirror_wait.insert(id, w);
            }
            None => {
                self.mirror_wait.remove(&id);
        self.mirror_hail_intent.remove(&id);
            }
        }
        match pose.aboard {
            Some((bus, local, lheading, _)) => {
                p.place = Place::Bus(BusId::Ai(bus), local);
                p.lheading = lheading;
                p.vel = DVec2::ZERO;
            }
            None => p.place = Place::Ground,
        }
    }

    /// One of the host's people has gone (client).
    pub fn mirror_remove(&mut self, id: u32) {
        if let Some(i) = self.people.iter().position(|p| p.id == id && p.remote) {
            let p = self.people.swap_remove(i);
            self.retire(&p);
        }
        self.claimed.remove(&id);
        self.mirror_wait.remove(&id);
        self.mirror_hail_intent.remove(&id);
    }

    /// A remote person this frame: they stand where the host put them.
    pub fn mirror_want(&mut self, i: usize, _buses: &[BusNow]) -> Want {
        Want::stand(None, self.people[i].activity)
    }

    /// The type of one of our people (host).
    pub fn lan_people_by_id(&self, id: u32) -> Option<Arc<HumanType>> {
        self.people
            .iter()
            .find(|p| p.id == id && !p.remote)
            .map(|p| p.ty.clone())
    }

    /// Where the host's people are drawn (client; `OMSI_LAN_TRACE`).
    pub fn mirror_positions(&self) -> Vec<(u32, DVec3)> {
        self.people
            .iter()
            .filter(|p| p.remote && p.place == Place::Ground)
            .map(|p| (p.id, p.position))
            .collect()
    }

    /// Waiting people to ask the host for (client).
    pub fn take_claims(&mut self) -> Vec<u32> {
        std::mem::take(&mut self.claims_out)
    }

    /// The host handed this waiting person over to our bus (client). (The people of a LAN
    /// game stay the host's: nothing is taken over.)
    pub fn grant(&mut self, id: u32) -> bool {
        self.claimed.remove(&id);
        let Some((stop, spot)) = self.mirror_wait.remove(&id) else { return false };
        let Some(i) = self.people.iter().position(|p| p.id == id && p.remote) else { return false };
        if !self.stops.contains_key(&stop) {
            return false;
        }
        // ours from now on: waiting at that place, for the bus that stands there (the first
        // listed, as Omsi.exe takes it without a line record)
        let sp = self.stops[&stop].spots.get(spot).cloned();
        let seatheight = self.people[i].ty.def.seat_height;
        let walk = 1.1 + (self.rand_f() as f32 * 2.0 - 1.0) * 0.2;
        let mut pax = Pax::new(walk, self.rand_f());
        pax.task = Task::WaitingForBus;
        pax.stop = Some(stop);
        // what a waiting person of ours has (sub_626044 and task 6): a destination drawn
        // from the stop (both sides load the same map) and a distance to ride without one;
        // with neither they would get off again at once (#813)
        let (dest,line)=self.mirror_hail_intent.remove(&id)
            .filter(|(wanted,_,_,_)|*wanted==stop).and_then(|(_,key,line,dest)|if key.is_none()||line.is_some(){Some((dest,line))}else{None})
            .unwrap_or_else(||self.draw_dest(stop));
        pax.dest = dest;
        pax.line = line;
        pax.ride_km = self.rand_f() as f32 * 19.0 + 1.0;
        pax.pos = self.people[i].position;
        pax.yaw = self.people[i].heading.to_radians();
        if let Some(sp) = sp {
            pax.spot = Some(spot);
            if let Some(t) = self.stops.get_mut(&stop).unwrap().taken.get_mut(spot) {
                *t = true;
            }
            if sp.height != 0.0 {
                pax.seat_h = sp.height;
                pax.pos = sp.pos - DVec3::Z * seatheight as f64;
                pax.pax_state = 2.0;
            }
            pax.yaw = sp.face.to_radians();
        }
        let p = &mut self.people[i];
        p.remote = false;
        p.state = State::Pax(Box::new(pax));
        true
    }

    /// The host's people waiting at the stop our bus is listed at (client): ask for them.
    pub fn claim_waiting(&mut self) {
        if !self.mirror {
            return;
        }
        let now = self.time;
        self.claimed.retain(|_, t| now - *t < 10.0);
        let at: Vec<i64> = self
            .stops
            .iter()
            .filter(|(_, s)| s.buses.iter().any(|b| b.0 == BusId::Player))
            .map(|(id, _)| *id)
            .collect();
        if at.is_empty() {
            return;
        }
        for (id, (stop, _)) in &self.mirror_wait {
            let fits=self.people.iter().find(|p|p.id==*id).and_then(|person|
                self.last_buses.iter().find(|b|b.id==BusId::Player)
                .and_then(|b|self.mirror_bus_fit(*id,b,person.position))).unwrap_or(true);
            if at.contains(stop) && fits && !self.claimed.contains_key(id) {
                self.claimed.insert(*id, now);
                self.claims_out.push(*id);
            }
        }
    }
}
