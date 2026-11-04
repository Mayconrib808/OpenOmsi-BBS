//! Ordinary demand and destinations share one passenger identity with the
//! accessibility controller. One doorway is served at a time; bays are independent.
use super::*;

pub struct Mobile {
    pub pose: ChairPose,
    position: DVec3,
    heading: f64,
}
pub struct Rider {
    pub profile: Profile,
    pub journey: Journey,
    pub reserved: Vec<usize>,
}

pub(super) fn has_special_signal(v: &VehicleInstance) -> bool {
    let special=|n:&str| {
        let n=n.to_ascii_lowercase();
        ["kinderwagenwunsch", "rollstuhl", "wheelchair", "luzcadeirante"]
            .iter().any(|word|n.contains(word))
    };
    v.ty.model.meshes.iter().any(|m| {
        m.visible.as_ref().is_some_and(|(n,_)|special(n))
            || m.materials.iter().any(|mat|mat.change.as_ref().is_some_and(|(_,_,n)|special(n)))
    })
}

impl Wheelchairs {
    pub fn is_chair(&self,id:u32)->bool {
        self.mobile.contains_key(&id)||self.person==Some(id)||self.riders.contains_key(&id)
    }
    pub fn wants_alight(&self)->bool {
        self.journey.as_ref().is_some_and(|j|j.alight_requested&&j.boarded)
            ||self.riders.values().any(|r|r.journey.alight_requested)
    }
    pub fn request_alight(&mut self,id:u32)->bool {
        if self.person==Some(id) {
            if let Some(j)=self.journey.as_mut() {j.alight_requested=true;return true;}
        }
        if let Some(r)=self.riders.get_mut(&id) {r.journey.alight_requested=true;return true;}
        false
    }
    pub fn free_profiles(&self)->impl Iterator<Item=&Profile> {
        self.profiles.iter().filter(|p| {
            !self.riders.values().any(|r|(r.profile.bay-p.bay).truncate().length()<1.35)
                &&!self.journey.as_ref().zip(self.profile.as_ref()).is_some_and(|(j,other)|
                    j.stage!=Stage::Finished&&(other.bay-p.bay).truncate().length()<1.35)
        })
    }
    pub fn supports_bus(&self,id:BusId,side:Option<f32>)->bool {
        id==BusId::Player&&self.profiles.iter().any(|p|side.is_none_or(|s|s==p.side()))
    }
}

impl PeopleSim {
    pub(crate) fn wheelchair_spawn_kind(&mut self,world:&dyn World,state:&State)->Option<usize> {
        if !self.wheelchairs.enabled||!self.optional_features.wheelchair_passengers
            ||!matches!(state,State::Pax(p) if p.inside.is_none()&&p.stop.is_some()) {
            return None;
        }
        let chance=self.wheelchairs.spawn_percent;
        if !chance.is_finite()||chance<=0.0||self.rand_f()*100.0>=chance.clamp(0.0,100.0) as f64 {
            return None;
        }
        self.use_map_humans(world);
        let types:Vec<_>=self.types.iter().enumerate().filter(|(_,t)|
            t.def.age.unwrap_or(40)>=18&&(1.45..=1.95).contains(&t.rig.head_top)
                &&t.rig.upper_arm.is_finite()&&t.rig.forearm.is_finite()
                &&t.rig.upper_arm+t.rig.forearm>0.45).map(|(k,_)|k).collect();
        (!types.is_empty()).then(||types[self.rand() as usize%types.len()])
    }
    pub(crate) fn register_wheelchair(&mut self,i:usize) {
        let p=&self.people[i];
        self.wheelchairs.mobile.insert(p.id,Mobile{pose:Default::default(),position:p.position,heading:p.heading});
        if let Some(pax)=self.pax_mut(i) {pax.walk_speed=0.75;pax.speed_des=0.75;}
        log::info!("WHEELCHAIR demand_spawn person={} chance_percent={:.2} stop={:?}",
            self.people[i].id,self.wheelchairs.spawn_percent,self.pax(i).and_then(|p|p.stop));
    }
    pub(super) fn tick_wheelchair_mobility(&mut self,dt:f32) {
        let ids:HashSet<_>=self.people.iter().map(|p|p.id).collect();
        self.wheelchairs.mobile.retain(|id,_|ids.contains(id));
        self.wheelchairs.riders.retain(|id,_|ids.contains(id));
        for p in &self.people {
            let Some(m)=self.wheelchairs.mobile.get_mut(&p.id) else {continue};
            let ds=(p.position-m.position).truncate().length() as f32;
            let yaw=angle_delta(m.heading,p.heading).to_radians() as f32;
            // A reload/remote relocation must not turn into a propulsion stroke.
            let ds=if ds<dt.max(0.0)*1.5+0.02{ds}else{0.0};
            m.pose.moving=if ds>0.001||yaw.abs()>0.001{1.0}else{0.0};
            for side in 0..2 {
                let distance=ds+yaw*0.33*if side==0{1.0}else{-1.0};
                m.pose.wheels[side]=(m.pose.wheels[side]+distance/0.30).rem_euclid(std::f32::consts::TAU);
                m.pose.push_phase[side]=(m.pose.push_phase[side]+distance.abs()/0.85).fract();
                m.pose.direction[side]=distance.signum();
            }
            m.position=p.position;m.heading=p.heading;
        }
        if self.wheelchairs.wants_alight() {self.stop_request=true;}
    }
    pub(crate) fn begin_wheelchair_boarding(&mut self,i:usize,bn:&BusNow,world:&dyn World)->bool {
        if bn.id!=BusId::Player||self.wheelchairs.journey.is_some()||bn.speed.abs()>0.025 {return false;}
        let Some(pax)=self.pax(i) else {return false};
        let stop=pax.stop;
        let side=stop.and_then(|s|self.preferred_entry_side(s,bn,self.people[i].position));
        let profile=self.wheelchairs.free_profiles().filter(|p|side.is_none_or(|s|s==p.side()))
            .min_by(|a,b|(bn.world(a.outside(0.0))-self.people[i].position).length_squared()
                .total_cmp(&(bn.world(b.outside(0.0))-self.people[i].position).length_squared())).cloned();
        let Some(profile)=profile else {return false};
        let at=self.people[i].position;
        if world.walk_height_near(at.x,at.y,at.z).is_none(){return false;}
        let local=bn.to_local(at);
        if (local.z-profile.threshold.z).abs()>1.6{return false;}
        let mut journey=Journey::new(&profile,local.z,angle_delta(bn.heading,self.people[i].heading));
        journey.local=local;journey.outside_world=Some(at);journey.board_stop=stop;
        self.wheelchairs.profile=Some(profile.clone());
        self.wheelchairs.person=Some(self.people[i].id);
        self.wheelchairs.journey=Some(journey);
        self.wheelchairs.assisted=false;
        let p=&mut self.people[i];
        p.puppet=Some(Puppet{mode:PuppetMode::Wheelchair});
        if let State::Pax(pax)=&mut p.state {
            pax.task=Task::WalkingToBus;pax.st=0;pax.bus=Some(BusId::Player);pax.seat=None;
        }
        log::info!("WHEELCHAIR boarding_selected person={} bay={:?} door={}",self.people[i].id,profile.bay,profile.door_var);
        true
    }
    pub(super) fn schedule_wheelchair_journeys(&mut self,world:&dyn World,bus:Option<&PlayerBusView>,buses:&[BusNow]) {
        if self.wheelchairs.journey.is_some(){return;}
        let Some(bus)=bus.filter(|b|b.speed.abs()<0.025)else{return;};
        let Some(bn)=buses.iter().find(|b|b.id==BusId::Player)else{return;};
        let Some(stop)=self.player_next_stop.as_ref().filter(|s|(s.pos-bn.pos).truncate().length()<35.0)else{return;};
        let next=stop.id;
        let id=self.wheelchairs.riders.iter().filter(|(_,r)|r.journey.alight_requested
            &&r.journey.board_stop!=Some(next)).map(|(id,_)|*id).min();
        if let Some(id)=id {
            let r=self.wheelchairs.riders.remove(&id).unwrap();
            self.wheelchairs.person=Some(id);self.wheelchairs.profile=Some(r.profile);
            self.wheelchairs.journey=Some(r.journey);self.wheelchairs.reserved=r.reserved;
            self.wheelchairs.assisted=false;
            if let Some(p)=self.people.iter_mut().find(|p|p.id==id){p.puppet=Some(Puppet{mode:PuppetMode::Wheelchair});}
        }
        let _=(bus,world);
    }
    pub(super) fn finish_wheelchair_service(&mut self,i:usize,bn:&BusNow,buses:&[BusNow],world:&dyn World) {
        let Some(j)=self.wheelchairs.journey.as_ref()else{return;};
        if j.deployment>0.0001{return;}
        let riding=j.stage==Stage::Riding;
        if !riding&&j.stage!=Stage::Finished{return;}
        let journey=self.wheelchairs.journey.take().unwrap();
        let Some(id)=self.wheelchairs.person.take()else{return;};
        self.wheelchairs.equipment=None;self.wheelchairs.assisted=false;
        self.people[i].puppet=None;
        if let Some(m)=self.wheelchairs.mobile.get_mut(&id){m.pose=journey.pose;m.pose.moving=0.0;}
        if riding {
            let profile=self.wheelchairs.profile.clone().unwrap();
            let reserved=std::mem::take(&mut self.wheelchairs.reserved);
            if let Some(p)=self.pax_mut(i){
                p.task=Task::SittingInBus;p.st=0;p.pax_state=2.0;p.pos=journey.local.as_dvec3();
                p.inside=Some(BusId::Player);p.bus=Some(BusId::Player);p.seat=None;
            }
            self.wheelchairs.riders.insert(id,Rider{profile,journey,reserved});
            log::info!("WHEELCHAIR bay_secured person={id} occupied={} capacity={}",self.wheelchairs.riders.len(),self.wheelchairs.profiles.len());
        }else{
            let at=self.people[i].position;let heading=self.people[i].heading;
            let stop=self.player_next_stop.as_ref().map(|s|s.id);
            let away=at+(at-bn.pos).truncate().normalize_or_zero().extend(0.0)*3.0;
            let bus_ix=buses.iter().enumerate().map(|(k,b)|(b.id,k)).collect();
            if let Some(p)=self.pax_mut(i){p.inside=None;p.pos=at;p.bus=None;p.seat=None;}
            let mut remove=Vec::new();
            if !self.try_connection(i,at,heading,stop,BusId::Player,buses,&bus_ix,world) {
                self.walk_street_from_bus(i,at,heading,stop,away,world,&mut remove);
            }
            log::info!("WHEELCHAIR alighted person={id} onward_state={}",self.people[i].state.name());
        }
        INTERLOCK.store(false,Ordering::Release);
    }
    pub(super) fn clear_wheelchair_corridor(&mut self,id:u32,profile:&Profile,bn:&BusNow,_dt:f32) {
        let corridor=|at:Vec3| {
            (at-profile.bay).truncate().length()<0.85
                ||profile.route.iter().any(|p|(at-*p).truncate().length()<0.64)
        };
        let movers:Vec<_>=self.people.iter().enumerate().filter_map(|(i,p)| {
            let State::Pax(x)=&p.state else{return None};
            if p.id==id||self.wheelchairs.is_chair(p.id)||x.inspector||x.responder_hold
                ||x.inside!=Some(BusId::Player)||x.task!=Task::SittingInBus{return None;}
            let fold=profile.fold.is_some()&&(x.pos.as_vec3()-profile.bay).truncate().length()<1.0;
            (corridor(x.pos.as_vec3())&&(x.seat.is_none_or(|k|bn.cabin.seats.get(k).is_none_or(|s|!s.seated))||fold)).then_some(i)
        }).collect();
        for i in movers {
            let group=self.pax(i).and_then(|p|p.seat).and_then(|k|bn.cabin.seats.get(k)).map(|s|s.group);
            let off:Vec<_>=bn.cabin.seats.iter().enumerate().map(|(k,s)|
                !s.floor.is_finite()||s.floor.abs().max_element()>100.0||corridor(s.floor)
                    ||group.is_some_and(|g|g!=s.group)||bn.places_off.get(k).copied().unwrap_or(false)).collect();
            let Some(k)=self.reserve_place(BusId::Player,&bn.cabin.seats,&off)else{continue};
            if let Some(old)=self.pax(i).and_then(|p|p.seat){self.free_seat(BusId::Player,old);}
            let all=bn.cabin.all_points();
            let at=self.pax(i).unwrap().pos.as_vec3();
            let pt=bn.cabin.omsi_nearest(at,&all,false,false,None,None);
            let p=self.pax_mut(i).unwrap();
            p.seat=Some(k);p.task=Task::InBusToPlace;p.pt=pt;p.ticket=0;p.pax_state=1.0;p.target_bus=true;
            self.route_to_place(i,bn);
            log::info!("WHEELCHAIR make_room person={} new_place={k}",self.people[i].id);
        }
    }
}
