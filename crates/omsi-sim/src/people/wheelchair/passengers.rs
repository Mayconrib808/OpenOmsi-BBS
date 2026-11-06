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

fn strong_signal_name(name:&str)->bool {
    let n=name.to_ascii_lowercase().replace('_',"");
    n.contains("kinderwagenwunsch")||n.contains("luzcadeirante")
        ||((n.contains("wheelchair")||n.contains("rollstuhl")||n.contains("cadeirante"))
            &&["request","wunsch","sinal","signal","light","bell","desembarque"].iter().any(|k|n.contains(k)))
}
pub(super) fn special_signal_variables(v:&VehicleInstance)->Vec<String> {
    let mut variables=Vec::new();
    for m in &v.ty.model.meshes {
        let file=m.file.to_ascii_lowercase();
        let indicator=["painel","panel","light","luz","indicator","anzeige","cockpit"].iter().any(|k|file.contains(k));
        for name in m.visible.iter().map(|(name,_)|name)
            .chain(m.materials.iter().filter_map(|mat|mat.change.as_ref().map(|(_,_,name)|name))) {
            let n=name.to_ascii_lowercase();
            if (strong_signal_name(name)||(indicator&&(n.contains("wheelchair")||n.contains("rollstuhl")||n.contains("cadeirante"))))
                &&v.var(name).is_some()&&!variables.contains(name) {variables.push(name.clone());}
        }
    }
    if !variables.is_empty()&&v.var("door_kinderwagenwunsch").is_some(){variables.push("door_kinderwagenwunsch".into());}
    variables.sort();variables.dedup();variables
}

fn spawn_roll(seed:u64,id:u32)->f64 {
    let mut x=seed^(u64::from(id).wrapping_mul(0x9E3779B97F4A7C15));
    x=(x^(x>>30)).wrapping_mul(0xBF58476D1CE4E5B9);
    x=(x^(x>>27)).wrapping_mul(0x94D049BB133111EB);
    ((x^(x>>31))>>11) as f64*(100.0/(1u64<<53) as f64)
}

impl Wheelchairs {
    pub fn is_chair(&self,id:u32)->bool {
        self.mobile.contains_key(&id)||self.person==Some(id)||self.riders.contains_key(&id)
    }
    pub fn wants_alight(&self)->bool {
        // A completed trip may linger while the equipment is stowed. It no
        // longer requests a stop: clear the cab lamp and HUD immediately.
        self.journey.as_ref().is_some_and(|j|
            j.alight_requested&&j.boarded&&j.stage!=Stage::Finished)
            ||self.riders.values().any(|r|
                r.journey.alight_requested&&r.journey.stage!=Stage::Finished)
    }
    pub fn request_alight(&mut self,id:u32)->bool {
        let journey=if self.person==Some(id){self.journey.as_mut()}
            else{self.riders.get_mut(&id).map(|r|&mut r.journey)};
        if let Some(j)=journey {
            if !j.alight_requested {
                log::info!("WHEELCHAIR alighting_requested person={id} signal={} variables={:?}",
                    if self.special_stop_signal{"special"}else{"normal_with_driver_notice"},self.stop_signal_variables);
            }
            j.alight_requested=true;return true;
        }
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
    /// Mobility metadata uses the normal P16 person identity and position stream.
    pub fn mirror_wheelchair(&mut self,id:u32,on:bool) {
        let Some(p)=self.people.iter().find(|p|p.id==id&&p.remote)else{return;};
        if on {
            self.wheelchairs.mobile.entry(id).or_insert(Mobile{pose:Default::default(),position:p.position,heading:p.heading});
        }else{self.wheelchairs.mobile.remove(&id);}
    }
    /// A returned connection gets a new authoritative id while keeping its mobility.
    pub fn adopt_wheelchair_mobility(&mut self,id:u32) {
        if let Some(i)=self.people.iter().position(|p|p.id==id&&!p.remote) {
            if !self.wheelchairs.is_chair(id){self.register_wheelchair(i);}
        }
    }
    pub(crate) fn wheelchair_spawn_kind(&mut self,world:&dyn World,state:&State)->Option<usize> {
        if !self.wheelchairs.enabled||!self.optional_features.wheelchair_passengers
            ||!matches!(state,State::Pax(p) if p.inside.is_none()&&p.stop.is_some()) {
            return None;
        }
        let chance=self.wheelchairs.spawn_percent;
        if !chance.is_finite()||chance<=0.0||spawn_roll(self.rng,self.next_id)>=chance.clamp(0.0,100.0) as f64 {
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
        let spot=self.pax(i).filter(|p|p.inside.is_none())
            .and_then(|p|p.stop.zip(p.spot))
            .and_then(|(s,k)|self.stops.get(&s)?.spots.get(k)).cloned();
        if let Some(pax)=self.pax_mut(i) {
            if let Some(sp)=spot {
                pax.pos=sp.pos-DVec3::Z*sp.height as f64;
                pax.pax_state=0.0;pax.seat_h=0.0;
            }
            pax.walk_speed=0.75;pax.speed_des=0.75;
            let at=pax.pos;
            self.people[i].position=at;
        }
        let p=&self.people[i];
        self.wheelchairs.mobile.insert(p.id,Mobile{pose:Default::default(),position:p.position,heading:p.heading});
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
        // A call is an edge, not a button held down on every simulation tick.
        // A level-triggered request repeatedly fired int_haltewunsch, latching
        // native stop lights and hiding the distinction from an ordinary call.
        let wheelchair_call = self.wheelchairs.wants_alight();
        if wheelchair_call && !self.wheelchairs.stop_request_armed {
            self.stop_request = true;
        }
        self.wheelchairs.stop_request_armed = wheelchair_call;
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
        self.wheelchairs.crowd_targets.clear();
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
        if bus.is_none()||!buses.iter().any(|b|b.id==BusId::Player){return;}
        // A request must be visible in Alt as soon as it is made. Timetable
        // stop advancement and the driver's distance to a stop do not own it.
        // The controller still waits for assistance, an open door and zero speed.
        let id=self.wheelchairs.riders.iter().filter(|(_,r)|r.journey.alight_requested)
            .map(|(id,_)|*id).min();
        if let Some(id)=id {
            let r=self.wheelchairs.riders.remove(&id).unwrap();
            self.wheelchairs.person=Some(id);self.wheelchairs.profile=Some(r.profile);
            self.wheelchairs.journey=Some(r.journey);self.wheelchairs.reserved=r.reserved;
            self.wheelchairs.assisted=false;
            if let Some(p)=self.people.iter_mut().find(|p|p.id==id){p.puppet=Some(Puppet{mode:PuppetMode::Wheelchair});}
            log::info!("WHEELCHAIR alighting_activated person={id}");
        }
        let _=world;
    }
    pub(crate) fn finish_wheelchair_service(&mut self,i:usize,bn:&BusNow,buses:&[BusNow],world:&dyn World) {
        let Some(j)=self.wheelchairs.journey.as_ref()else{return;};
        if j.deployment>0.0001{return;}
        let riding=j.stage==Stage::Riding;
        if !riding&&j.stage!=Stage::Finished{return;}
        // Keep the selected rider active until the driver starts the alighting
        // service. Reparking it here erased the Alt action on every frame.
        if riding&&j.alight_requested{return;}
        let journey=self.wheelchairs.journey.take().unwrap();
        let Some(id)=self.wheelchairs.person.take()else{return;};
        self.wheelchairs.equipment=None;self.wheelchairs.assisted=false;self.wheelchairs.crowd_targets.clear();
        self.people[i].puppet=None;
        if let Some(m)=self.wheelchairs.mobile.get_mut(&id){m.pose=journey.pose;m.pose.moving=0.0;}
        if riding {
            let profile=self.wheelchairs.profile.clone().unwrap();
            let reserved=std::mem::take(&mut self.wheelchairs.reserved);
            let boarding_km=self.odometer.get(&BusId::Player).copied().unwrap_or(0.0);
            if let Some(p)=self.pax_mut(i){
                p.task=Task::SittingInBus;p.st=0;p.pax_state=2.0;p.pos=journey.local.as_dvec3();
                p.inside=Some(BusId::Player);p.bus=Some(BusId::Player);p.seat=None;
                p.km_start=boarding_km;
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
    pub(super) fn clear_wheelchair_corridor(&mut self,id:u32,profile:&Profile,journey:&Journey,bn:&BusNow,world:&dyn World,dt:f32) {
        if !dt.is_finite()||dt<=0.0||bn.speed.abs()>0.025{return;}
        let mut path=vec![journey.local,profile.outside(journey.ground_height),profile.threshold];path.extend_from_slice(&profile.route);path.push(profile.bay);
        let corridor=|at:Vec3|corridor_contains(&path,at,0.8);
        let movers:Vec<_>=self.people.iter().enumerate().filter_map(|(i,p)| {
            let State::Pax(x)=&p.state else{return None};if p.id==id||self.wheelchairs.is_chair(p.id){return None;}
            let inside=x.inside==Some(BusId::Player);let at=bn.to_local(p.position);
            let seated=inside&&x.seat.and_then(|k|bn.cabin.seats.get(k)).is_some_and(|s|s.seated);
            let fold=profile.fold.is_some()&&(at-profile.bay).truncate().length()<1.0;
            let ground=x.inside.is_none()&&(p.position-bn.world(journey.local)).length_squared()<25.0;
            ((inside&&(!seated||fold)||ground)&&(corridor(at)||self.wheelchairs.crowd_targets.contains_key(&p.id))).then_some((i,at,!inside))
        }).collect();
        for (i,at,ground) in movers {
            let pid=self.people[i].id;
            if !self.wheelchairs.crowd_targets.contains_key(&pid) {
                let target=if ground {
                    [1.05f32,-1.05,1.5,-1.5,2.0,-2.0].into_iter().filter_map(|dy| {
                        let mut q=at+Vec3::Y*dy;let from=bn.world(at);let to=bn.world(q);let z=world.walk_height_near(to.x,to.y,from.z)?;
                        if (z-from.z).abs()>0.2||corridor(q)||world.collision().ray_blocked(from+DVec3::Z*0.45,to+DVec3::Z*0.45,0.25,0.6){return None;}
                        q.z=bn.to_local(DVec3::new(to.x,to.y,z)).z;Some(q)
                    }).next()
                }else{
                    // Compress a full standing crowd into existing standing places;
                    // preserve the other wheelchair bays and the model's floor.
                    let taken=self.seats.get(&BusId::Player);
                    bn.cabin.seats.iter().enumerate().filter(|(k,s)|!s.seated&&s.floor.is_finite()&&!corridor(s.floor)&&!bn.places_off.get(*k).copied().unwrap_or(false)&&(s.floor.z-at.z).abs()<0.35&&!self.wheelchairs.riders.values().any(|r|(s.floor-r.profile.bay).truncate().length()<0.9))
                    .min_by(|(ka,a),(kb,b)| {
                        let score=|k:usize,q:Vec3|(q-at).length_squared()+if taken.and_then(|t|t.get(k)).copied().unwrap_or(false){100.0}else{0.0}+if self.wheelchairs.crowd_targets.values().any(|(t,g)|!*g&&(*t-q).length()<0.35){20.0}else{0.0};
                        score(*ka,a.floor).total_cmp(&score(*kb,b.floor))
                    }).map(|(_,s)|s.floor)
                };
                let Some(target)=target else{continue};self.wheelchairs.crowd_targets.insert(pid,(target,ground));
                log::info!("WHEELCHAIR make_room person={pid} ground={ground} from={at:?} target={target:?}");
            }
            let (target,was_ground)=self.wheelchairs.crowd_targets[&pid];if was_ground!=ground{self.wheelchairs.crowd_targets.remove(&pid);continue;}
            let next=at+(target-at).clamp_length_max(dt.clamp(0.0,0.1)*1.2);let new_world=bn.world(next);
            let old=self.pax(i).and_then(|x|x.seat);if !ground{if let Some(k)=old{self.free_seat(BusId::Player,k);}}
            if let Some(x)=self.pax_mut(i){x.pos=if ground{new_world}else{next.as_dvec3()};if !ground{x.seat=None;x.seat_h=0.0;x.pax_state=1.0;}}
            self.people[i].position=new_world;if !ground{self.people[i].place=Place::Bus(BusId::Player,next);}
        }
    }
}
fn corridor_contains(path:&[Vec3],at:Vec3,radius:f32)->bool {
    path.windows(2).any(|p|{let d=p[1]-p[0];let t=((at-p[0]).truncate().dot(d.truncate())/d.truncate().length_squared().max(1e-6)).clamp(0.0,1.0);let near=p[0]+d*t;(at.z-near.z).abs()<1.5&&(at-near).truncate().length()<radius})
}
#[cfg(test)]
mod corridor_tests {
    use super::*;
    #[test] fn people_between_route_vertices_and_at_the_curb_also_yield() {
        let path=[Vec3::new(2.0,0.0,0.0),Vec3::ZERO,Vec3::new(0.0,4.0,0.3)];
        assert!(corridor_contains(&path,Vec3::new(0.3,2.0,0.3),0.8));assert!(corridor_contains(&path,Vec3::new(1.7,0.2,0.0),0.8));
        assert!(!corridor_contains(&path,Vec3::new(1.5,2.0,0.3),0.8));assert!(!corridor_contains(&path,Vec3::new(0.0,2.0,3.0),0.8));
    }
}
