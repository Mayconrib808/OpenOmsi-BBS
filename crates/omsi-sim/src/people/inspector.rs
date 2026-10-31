//! A fictional BCS companion uses the ordinary cabin, seat pool and path walker.
use super::*;

fn inspector_patrol_index(step:u8,count:usize)->usize {
    let len=count.max(1);
    let at=usize::from(step)%(2*len);
    if at<len {at} else {2*len-1-at}
}


impl PeopleSim {
    pub fn spawn_bcs_inspector(&mut self, world: &dyn World, bus: &VehicleInstance,
        identity: u32, female: Option<bool>) -> Option<u32> {
        self.use_map_humans(world);
        // A delayed/repeated BCS snapshot must not create a second inspector
        // while the first is already patrolling or seated in this vehicle.
        if let Some(existing) = self.people.iter().find(|person| !person.remote
            && person.inside(BusId::Player)
            && matches!(&person.state, State::Pax(pax) if pax.inspector)) {
            log::info!("BCS inspector {identity}: duplicate event; reusing person {}", existing.id);
            return Some(existing.id);
        }
        if !self.pool_room() { return None; }
        let cabin = self.cabin_for(bus)?;
        let driver = cabin.data.driver_positions.first().map(|d| Vec3::from(d.pos)).unwrap_or(Vec3::ZERO);
        let off = places_off(bus, &cabin);
        let occupied = self.seats.entry(BusId::Player).or_insert_with(|| vec![false; cabin.seats.len()]);
        occupied.resize(cabin.seats.len(), false);
        let seat = cabin.seats.iter().enumerate()
            .filter(|(k,s)| s.seated && !occupied[*k] && !off.get(*k).copied().unwrap_or(false))
            .min_by(|(_,a),(_,b)| (a.pos-driver).length_squared().total_cmp(&(b.pos-driver).length_squared()))?.0;
        let mut choices: Vec<(i32,u64,usize)> = self.types.iter().enumerate().filter_map(|(k,ty)| {
            if ty.def.age.is_some_and(|age| age < 18) || ty.def.height < 1.3 { return None; }
            let voice = ty.def.voice.to_uppercase();
            let model_female = if voice.starts_with('F') {Some(true)} else if voice.starts_with('M') {Some(false)} else {None};
            if female.is_some() && model_female.is_some() && female != model_female { return None; }
            let name = ty.def.path.to_string_lossy().to_lowercase();
            let professional = ["inspector","kontroll","uniform","business","suit","fiscal"].iter().any(|s| name.contains(s));
            let score = i32::from(professional)*100 + i32::from(female.is_some() && female==model_female)*20;
            let tie = (identity as u64).wrapping_mul(6364136223846793005).rotate_left((k%63) as u32);
            Some((score,tie,k))
        }).collect();
        choices.sort_unstable();
        // Prefer a recognisable professional model; if none is installed, prefer
        // a type not worn by any other passenger currently in this bus.
        // Never alter normal riders' types, models or seat reservations.
        let used: std::collections::HashSet<_> = self.people.iter()
            .filter(|person| person.inside(BusId::Player))
            .map(|person| person.ty.def.path.as_path())
            .collect();
        let ty = choices.iter().rev()
            .find(|(_,_,k)| !used.contains(self.types[*k].def.path.as_path()))
            .or_else(|| choices.last()).map(|c|c.2)?;
        self.seats.get_mut(&BusId::Player)?[seat] = true;
        let s = cabin.seats[seat].clone();
        let mut pax = Pax::new(0.9, 0.5);
        pax.inspector = true; pax.bus=Some(BusId::Player); pax.inside=Some(BusId::Player);
        pax.seat=Some(seat);pax.task=Task::SittingInBus;pax.seat_h=s.height;pax.pax_state=2.0;
        pax.pos=(s.pos-Vec3::Z*self.types[ty].def.seat_height).as_dvec3();pax.yaw=(s.rot as f64).to_radians();
        let at=bus.position+bus.body_rotation().transform_point3(pax.pos.as_vec3()).as_dvec3();
        let Some(i)=self.spawn_as(world,at,bus.heading,State::Pax(Box::new(pax)),Some(ty)) else {
            self.free_seat(BusId::Player,seat);return None;
        };
        self.people[i].place=Place::Bus(BusId::Player,s.pos);
        log::info!("BCS inspector {identity}: character {} in passenger seat {seat}",self.people[i].ty.def.path.display());
        Some(self.people[i].id)
    }

    pub fn remove_bcs_inspector(&mut self,id:u32)->bool{
        if !self.people.iter().any(|p|p.id==id&&matches!(&p.state,State::Pax(x)if x.inspector)){return false;}
        self.transfer_to_responder(id)
    }

    /// `checked`: actual BCS progress; None walks back and sits in the reserved chair.
    /// A disconnected cabin path leaves the character standing rather than teleporting.
    pub fn step_bcs_inspector(&mut self, id:u32, checked:Option<u32>, paused:bool) -> bool {
        let Some(i)=self.people.iter().position(|p|p.id==id&&!p.remote) else{return false;};
        let Some(bn)=self.last_buses.iter().find(|b|b.id==BusId::Player) else{return true;};
        let Some(p)=self.pax(i).filter(|p|p.inspector) else{return false;};
        let Some(seat)=p.seat.and_then(|k|bn.cabin.seats.get(k)) else{return false;};
        // A walking patrol already has its route. Keep the original validity checks,
        // then return before cloning the bus/Pax or rebuilding all cabin waypoints.
        if !paused && checked.is_some() && p.st==5 { return true; }
        let seat=seat.clone();
        let p=p.clone();
        let bn=bn.clone();
        if checked.is_none() && p.task==Task::SittingInBus {return true;}
        if paused {
            if let Some(p)=self.pax_mut(i){p.st=0;p.speed=0.0;p.pax_state=0.0;}
            return true;
        }
        let all=bn.cabin.all_points();
        let Some(home)=bn.cabin.omsi_nearest(seat.floor,&all,false,true,None,None) else{return true;};
        let start=if p.task==Task::SittingInBus {Some(home)} else {
            p.pt.or_else(||bn.cabin.omsi_nearest(p.pos.as_vec3(),&all,false,true,None,None))
        }.unwrap_or(home);
        // A BCS checked-ticket count is not a navigation command. It often stays
        // zero while the ticket checker is visibly in progress. Advance patrol
        // checkpoints when the *character physically reaches* each one instead.
        if checked.is_some() {
            if p.st==5 {return true;}
            if p.st==7 {
                let p=self.pax_mut(i).unwrap();
                p.sub=p.sub.wrapping_add(1);
                p.st=0;p.timer=1.25;p.speed=0.0;p.speed_des=0.0;
                return true;
            }
            if p.timer>0.0{return true;}
        }
        // The seat stays reserved while the inspector is out working.
        if checked.is_none() && matches!(p.st,0|6|7)
            && (p.pos-seat.floor.as_dvec3()).length()<1.2 {
            let height=self.people[i].ty.def.seat_height;
            let p=self.pax_mut(i).unwrap();
            p.task=Task::SittingInBus;p.st=0;p.pax_state=2.0;
            p.seat_h=seat.height;p.pos=(seat.pos-Vec3::Z*height).as_dvec3();
            p.yaw=(seat.rot as f64).to_radians();
            p.pt=None;p.pt_target=None;p.sub=0;p.timer=0.0;
            log::info!("BCS inspector {id}: returned to reserved seat after inspection");
            return true;
        }
        let goal=if checked.is_none(){home}else{
            // Visit seated customers in aisle order. When BCS begins inspection
            // before riders are seated, use spare seats to patrol the corridor.
            let group=bn.cabin.group_at(Some(home));
            let door=|k:usize|bn.cabin.entries.iter().chain(bn.cabin.exits.iter())
                .any(|d|(d.inside-bn.cabin.graph.points[k]).length()<0.9);
            let reachable=|k:usize|k!=home && !door(k)
                && (bn.cabin.groups<=1||bn.cabin.group_at(Some(k))==group)
                && bn.cabin.route_next(home,k).is_some() && bn.cabin.route_next(k,home).is_some();
            let mut targets:Vec<usize>=self.people.iter()
                .filter(|person|person.id!=id && person.inside(BusId::Player))
                .filter_map(|person|match &person.state{
                    State::Pax(x) if x.task==Task::SittingInBus && !x.inspector =>
                        x.seat.and_then(|k|bn.cabin.seats.get(k)).map(|s|s.floor),
                    _=>None
                })
                .filter_map(|pos|bn.cabin.omsi_nearest(pos,&all,false,true,None,None))
                .filter(|&k|reachable(k)).collect();
            // Include the rear aisle even when the currently checked count has
            // not advanced, so the inspector cannot just stand near the door.
            let rear=bn.cabin.seats.iter().filter(|s|s.seated)
                .filter_map(|s|bn.cabin.omsi_nearest(s.floor,&all,false,true,None,None))
                .filter(|&k|reachable(k))
                .min_by(|&a,&b|bn.cabin.graph.points[a].y.total_cmp(&bn.cabin.graph.points[b].y))
                .or_else(||(0..bn.cabin.graph.points.len()).filter(|&k|reachable(k))
                    .min_by(|&a,&b|bn.cabin.graph.points[a].y.total_cmp(&bn.cabin.graph.points[b].y)));
            if let Some(rear)=rear{targets.push(rear);}
            targets.sort_unstable_by(|&a,&b|bn.cabin.graph.points[b].y.total_cmp(&bn.cabin.graph.points[a].y));
            targets.dedup();
            if targets.is_empty(){return true;}
            targets[inspector_patrol_index(p.sub,targets.len())]
        };
        // If the cabin topology is disconnected, wait before retrying.
        // Without this guard the completed inspection logged a warning EVERY
        // simulation frame (over 11,000 in one captured session).
        if checked.is_none() && p.task==Task::Nothing && p.st==0 && p.timer>0.0 {return true;}
        if checked.is_none() && p.st==5 && p.pt_target==Some(home){return true;}
        if start==goal {
            if checked.is_some() {
                let p=self.pax_mut(i).unwrap();p.sub=p.sub.wrapping_add(1);p.timer=0.75;
            }
            return true;
        }
        if bn.cabin.route_next(start,goal).is_none(){
            // The last path index may be stale after the character moved into
            // a different cabin section. Find a physically NEAR waypoint that
            // really connects to the target, and walk to it -- no teleport.
            let here=p.pos.as_vec3();
            let rescue=bn.cabin.graph.points.iter().enumerate()
                .filter(|(k,point)| *k!=start && (**point-here).length()<=1.6
                    && bn.cabin.route_next(*k,goal).is_some())
                .min_by(|a,b| (*a.1-here).length_squared()
                    .total_cmp(&(*b.1-here).length_squared()))
                .map(|(k,point)|(k,*point));
            if let Some((k,point))=rescue {
                let pp=self.pax_mut(i).unwrap();
                pp.task=Task::Nothing;pp.pax_state=1.0;pp.target_bus=true;
                pp.pt=Some(k);pp.pt_target=Some(goal);pp.target=point.as_dvec3();
                pp.st=5;pp.timer=0.0;
                log::info!("BCS inspector {id}: recovered cabin route {start} -> {k} -> {goal} without teleport");
                return true;
            }
            // A truly disconnected cabin must never trigger a hot-loop,
            // repeatedly recalculate hundreds of times per second or block
            // passengers by pretending this idle inspector is still walking.
            log::warn!("BCS inspector {id}: no cabin route between {start} and {goal}; retry after cooldown");
            let pp=self.pax_mut(i).unwrap();
            pp.st=0;pp.speed=0.0;pp.speed_des=0.0;
            if checked.is_some(){pp.sub=pp.sub.wrapping_add(1);pp.timer=0.75;}
            else{pp.timer=5.0;}
            return true;
        }
        let p=self.pax_mut(i).unwrap();
        if p.task==Task::SittingInBus {p.pos=seat.floor.as_dvec3();}
        p.task=Task::Nothing;p.pax_state=1.0;p.target_bus=true;p.short=false;
        p.pt=Some(start);p.pt_target=Some(goal);
        p.target=bn.cabin.graph.points[start].as_dvec3();p.st=5;
        log::info!("BCS inspector {id}: walking cabin from path {start} to {goal}, visit {}",p.sub);
        true
    }
}

#[cfg(test)]
mod patrol_tests {
    use super::inspector_patrol_index;
    #[test]
    fn patrol_goes_aisle_front_to_back_and_back() {
        assert_eq!((0..8).map(|i|inspector_patrol_index(i,4)).collect::<Vec<_>>(),vec![0,1,2,3,3,2,1,0]);
    }
}
