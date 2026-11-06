//! Optional P16 visual snapshots. The authenticated sender owns the simulation.
//! No packet layout change to the original LAN protocol.
use super::*;
use base64::{engine::general_purpose::URL_SAFE_NO_PAD,Engine};
use std::time::{Duration,Instant};
pub const PREFIX:&str="wcb1 ";

/// The P16 message lane allows ten commands/second per sender. Spend at most
/// eight on accessibility, send nothing while idle, and clear each observer once.
pub struct SendSchedule {
    last:Instant,cursor:usize,active:bool,clear:std::collections::VecDeque<u32>,
}
impl Default for SendSchedule {
    fn default()->Self{Self{last:Instant::now()-Duration::from_secs(1),
        cursor:0,active:false,clear:Default::default()}}
}
impl SendSchedule {
    pub fn recipient(&mut self,now:Instant,peers:&[u32],active:bool)->Option<u32>{
        if active!=self.active {
            self.active=active;self.cursor=0;self.clear.clear();
            if !active{self.clear.extend(peers.iter().copied());}
        }
        if now.saturating_duration_since(self.last)<Duration::from_millis(125){return None;}
        let peer=if active {
            if peers.is_empty(){return None;}
            let peer=peers[self.cursor%peers.len()];
            self.cursor=self.cursor.wrapping_add(1);Some(peer)
        }else{
            while self.clear.front().is_some_and(|p|!peers.contains(p)){self.clear.pop_front();}
            self.clear.pop_front()
        };
        if peer.is_some(){self.last=now;}peer
    }
}

#[derive(Debug,Clone,Copy)]
pub struct EquipmentLocal {
    pub threshold:Vec3,pub width:f32,pub length:f32,pub height:f32,
    pub deployment:f32,pub bridge:f32,pub ground:f32,pub ramp:bool,
}
#[derive(Debug,Clone,Copy)]
pub struct Snapshot {
    pub kind:u16,pub inside:bool,pub platform_attached:bool,pub local:Vec3,
    pub heading:f32,pub chair:ChairPose,pub equipment:Option<EquipmentLocal>,
}
impl Snapshot {
    pub fn encode(self)->String {
        let flags=u8::from(self.inside)|(u8::from(self.chair.moving>0.1)<<1)
            |(u8::from(self.chair.secured)<<2)|(u8::from(self.chair.direction[0]<0.0)<<3)
            |(u8::from(self.chair.direction[1]<0.0)<<4)|(u8::from(self.equipment.is_some())<<5)
            |(u8::from(self.equipment.is_some_and(|e|e.ramp))<<6)
            |(u8::from(self.platform_attached)<<7);
        let mut b=Vec::with_capacity(72);b.push(1);b.extend(self.kind.to_le_bytes());b.push(flags);
        let e=self.equipment;let t=e.map_or(Vec3::ZERO,|e|e.threshold);
        for x in [self.local.x,self.local.y,self.local.z,self.heading,
            self.chair.wheels[0],self.chair.wheels[1],self.chair.push_phase[0],self.chair.push_phase[1],
            e.map_or(0.0,|e|e.deployment),e.map_or(0.0,|e|e.height),t.x,t.y,t.z,
            e.map_or(0.0,|e|e.width),e.map_or(0.0,|e|e.length),e.map_or(0.0,|e|e.bridge),
            e.map_or(0.0,|e|e.ground)] {b.extend(x.to_le_bytes());}
        format!("{PREFIX}{}",URL_SAFE_NO_PAD.encode(b))
    }
    pub fn decode(text:&str)->Option<Self> {
        let text=text.strip_prefix(PREFIX)?;
        if text.len()!=96{return None;}
        let b=URL_SAFE_NO_PAD.decode(text).ok()?;
        if b.len()!=72||b[0]!=1{return None;}
        let f=b[3];let mut a=[0.0_f32;17];
        for(x,b)in a.iter_mut().zip(b[4..].chunks_exact(4)){*x=f32::from_le_bytes(b.try_into().ok()?);}
        if !a.iter().all(|x|x.is_finite()){return None;}
        let local=Vec3::new(a[0],a[1],a[2]);
        if local.truncate().abs().max_element()>250.0||local.z.abs()>10.0||a[3].abs()>36000.0
            ||a[4..6].iter().any(|x|!(0.0..=6.284).contains(x))
            ||a[6..8].iter().any(|x|!(0.0..=1.0).contains(x)){return None;}
        let equipment=if f&32!=0 {
            if !(0.0..=1.0).contains(&a[8])||!(0.0..=1.0).contains(&a[15])
                ||!(0.65..=1.6).contains(&a[10].abs())||a[11].abs()>20.0
                ||!(0.15..=2.0).contains(&a[12])||!(0.78..=1.8).contains(&a[13])
                ||!(1.25..=2.2).contains(&a[14])||a[16].abs()>2.0
                ||a[9]<a[16]-0.05||a[9]>a[12]+0.05{return None;}
            Some(EquipmentLocal{threshold:Vec3::new(a[10],a[11],a[12]),width:a[13],length:a[14],
                height:a[9],deployment:a[8],bridge:a[15],ground:a[16],ramp:f&64!=0})
        }else{None};
        Some(Self{kind:u16::from_le_bytes([b[1],b[2]]),inside:f&1!=0,platform_attached:f&128!=0,
            local,heading:a[3],chair:ChairPose{wheels:[a[4],a[5]],push_phase:[a[6],a[7]],
                moving:if f&2!=0{1.0}else{0.0},secured:f&4!=0,
                direction:[if f&8!=0{-1.0}else{1.0},if f&16!=0{-1.0}else{1.0}]},equipment})
    }
}
pub(super) struct Remote {target:Snapshot,shown:Snapshot,person:Option<u32>,heard:Instant}
impl PeopleSim {
    pub fn wheelchair_wire(&self)->String {
        let Some(j)=self.wheelchairs.journey.as_ref()else{return format!("{PREFIX}-");};
        let Some(p)=self.wheelchairs.person.and_then(|id|self.people.iter().find(|p|p.id==id))
            else{return format!("{PREFIX}-");};
        let Some(profile)=self.wheelchairs.profile.as_ref()else{return format!("{PREFIX}-");};
        let kind=self.types.iter().position(|t|Arc::ptr_eq(t,&p.ty)).unwrap_or(0).min(u16::MAX as usize)as u16;
        Snapshot{kind,inside:j.boarded,platform_attached:matches!(j.stage,Stage::Raising|Stage::Lowering),
            local:j.local,heading:j.heading as f32,chair:j.pose,
            equipment:self.wheelchairs.equipment.map(|_|EquipmentLocal{threshold:profile.threshold,
                width:profile.clear_width,length:profile.platform_length,height:j.platform_height,
                deployment:j.deployment,bridge:j.bridge_deployment,ground:j.ground_height,
                ramp:profile.equipment==Equipment::AddedRamp})}.encode()
    }
    pub fn receive_wheelchair(&mut self,owner:u32,text:&str){
        if !self.wheelchairs.enabled||owner==0{return;}
        if text==format!("{PREFIX}-"){
            if let Some(r)=self.wheelchairs.remotes.remove(&owner){
                if let Some(id)=r.person{self.remove_wheelchair_body(id);}
            }return;
        }
        let Some(snapshot)=Snapshot::decode(text)else{return;};
        if self.wheelchairs.remotes.len()>=64&&!self.wheelchairs.remotes.contains_key(&owner){return;}
        if !self.wheelchairs.remotes.contains_key(&owner){log::info!("WHEELCHAIR network_peer owner={owner} snapshot_accepted=true");}
        let r=self.wheelchairs.remotes.entry(owner).or_insert_with(||Remote{
            target:snapshot,shown:snapshot,person:None,heard:Instant::now()});
        r.target=snapshot;r.heard=Instant::now();
    }
    fn remove_wheelchair_body(&mut self,id:u32){
        self.wheelchairs.remote_poses.remove(&id);
        if let Some(i)=self.people.iter().position(|p|p.id==id){let p=self.people.swap_remove(i);self.retire(&p);}
    }
    pub(crate) fn tick_remote_wheelchairs(&mut self,dt:f32,world:&dyn World,buses:&[BusNow]){
        if !self.wheelchairs.enabled{return;}
        let mut remotes=std::mem::take(&mut self.wheelchairs.remotes);
        self.wheelchairs.remote_poses.clear();self.wheelchairs.remote_equipment.clear();
        remotes.retain(|&owner,r|{
            let bn=buses.iter().find(|b|b.id==BusId::Ai(super::super::mirror::remote_bus_id(owner)));
            if r.heard.elapsed()>Duration::from_secs(20)||bn.is_none(){
                if let Some(id)=r.person.take(){self.remove_wheelchair_body(id);}
                return r.heard.elapsed()<=Duration::from_secs(20);
            }
            let bn=bn.unwrap();let k=(1.0-(-dt.max(0.0)/0.10).exp()).clamp(0.0,1.0);
            r.shown.local=r.shown.local.lerp(r.target.local,k);
            r.shown.heading+=angle_delta(r.shown.heading as f64,r.target.heading as f64)as f32*k;
            for side in 0..2 {
                let a=r.shown.chair.wheels[side];let b=r.target.chair.wheels[side];
                r.shown.chair.wheels[side]=(a+((b-a+std::f32::consts::PI).rem_euclid(std::f32::consts::TAU)-std::f32::consts::PI)*k).rem_euclid(std::f32::consts::TAU);
            }
            r.shown.chair.push_phase=r.target.chair.push_phase;r.shown.chair.direction=r.target.chair.direction;
            r.shown.chair.moving=r.target.chair.moving;r.shown.chair.secured=r.target.chair.secured;
            let at=bn.world(r.shown.local);
            let known=r.person.and_then(|id|self.people.iter().position(|p|p.id==id));
            let i=match known {Some(i)=>i,None=>{
                let Some(i)=self.spawn_as(world,at,bn.heading+r.shown.heading as f64,State::Idle,Some(r.target.kind as usize))else{return true;};
                let p=&mut self.people[i];p.puppet=Some(Puppet{mode:PuppetMode::Wheelchair});r.person=Some(p.id);
                log::info!("WHEELCHAIR remote_spawn owner={owner} person={} equipment={}",p.id,r.target.equipment.is_some());i
            }};
            let p=&mut self.people[i];p.position=at;p.heading=bn.heading+r.shown.heading as f64;p.lheading=r.shown.heading as f64;
            p.place=if r.target.inside{Place::Bus(bn.id,r.shown.local)}else{Place::Ground};
            p.tilt=bn.tilt_at(r.shown.local);p.vel=DVec2::ZERO;p.activity=Activity::Sit;
            p.interior=if r.target.inside{bn.interior}else{0.0};p.why="remote wheelchair owner snapshot";
            self.wheelchairs.remote_poses.insert(p.id,r.shown.chair);
            if let Some(e)=r.target.equipment {
                let height=if r.target.platform_attached{r.shown.local.z}else{e.height};
                self.wheelchairs.remote_equipment.push(EquipmentPose{owner,
                    position:bn.world(Vec3::new(e.threshold.x,e.threshold.y,if e.ramp{e.threshold.z}else{height})),
                    rotation:bn.rot,length:e.length,width:e.width,height,floor_height:e.threshold.z,
                    deployment:e.deployment,bridge_deployment:e.bridge,side:e.threshold.x.signum(),ground_height:e.ground,ramp:e.ramp});
            }true
        });self.wheelchairs.remotes=remotes;
    }
}
#[cfg(test)]mod tests{
    use super::*;
    fn sample()->Snapshot{Snapshot{kind:42,inside:true,platform_attached:false,local:Vec3::new(-0.62,-4.4,0.97),heading:180.0,
        chair:ChairPose{wheels:[1.0,2.0],push_phase:[0.2,0.7],moving:1.0,secured:true,direction:[-1.0,1.0]},
        equipment:Some(EquipmentLocal{threshold:Vec3::new(1.16,-4.44,0.97),width:1.05,length:1.4,height:0.5,deployment:1.0,bridge:0.0,ground:0.0,ramp:false})}}
    #[test]fn packet_fits_existing_p16_limit_and_roundtrips(){
        let s=sample();let w=s.encode();assert!(w.len()<=160);let r=Snapshot::decode(&w).unwrap();
        assert_eq!(r.kind,s.kind);assert_eq!(r.local,s.local);assert_eq!(r.chair.wheels,s.chair.wheels);
        assert_eq!(r.chair.direction,[-1.0,1.0]);assert_eq!(r.equipment.unwrap().height,0.5);
    }
    #[test]fn malformed_nonfinite_and_out_of_bounds_network_data_is_rejected(){
        for t in ["wcb1 ","wcb1 -","wcb1 !!!"]{assert!(Snapshot::decode(t).is_none());}
        let mut s=sample();s.local.x=f32::NAN;assert!(Snapshot::decode(&s.encode()).is_none());
        s=sample();s.local.x=10000.0;assert!(Snapshot::decode(&s.encode()).is_none());
        s=sample();s.equipment.as_mut().unwrap().width=100.0;assert!(Snapshot::decode(&s.encode()).is_none());
    }
    #[test]fn repeated_updates_do_not_duplicate_actors_and_peer_count_is_bounded(){
        let mut h=PeopleSim::new(Path::new("/nonexistent"),100);h.wheelchairs.enabled=true;let text=sample().encode();
        for _ in 0..100{h.receive_wheelchair(1,&text);}assert_eq!(h.wheelchairs.remotes.len(),1);
        for owner in 2..130{h.receive_wheelchair(owner,&text);}assert_eq!(h.wheelchairs.remotes.len(),64);
        h.receive_wheelchair(1,"wcb1 -");assert_eq!(h.wheelchairs.remotes.len(),63);
    }
    #[test]fn command_budget_is_bounded_for_a_large_room(){
        let peers:Vec<u32>=(1..65).collect();let now=Instant::now();
        let mut schedule=SendSchedule::default();let mut sent=0;
        for millisecond in 0..1000 {
            if schedule.recipient(now+Duration::from_millis(millisecond),&peers,true).is_some(){sent+=1;}
        }
        assert_eq!(sent,8);
    }
    #[test]fn each_peer_is_served_in_order_and_idle_has_no_periodic_traffic(){
        let now=Instant::now();let peers=[2,7,11];let mut schedule=SendSchedule::default();
        assert!(schedule.recipient(now,&peers,false).is_none());
        for n in 0..9 {
            assert_eq!(schedule.recipient(now+Duration::from_millis(n*125),&peers,true),Some(peers[n as usize%3]));
        }
        for n in 0..3 {
            assert_eq!(schedule.recipient(now+Duration::from_millis(1250+n*125),&peers,false),Some(peers[n as usize]));
        }
        assert!(schedule.recipient(now+Duration::from_secs(100),&peers,false).is_none());
    }
    #[test]fn bundled_torino_profile_passes_the_runtime_validator(){
        let json=std::fs::read_to_string(Path::new(env!("CARGO_MANIFEST_DIR")).join("../../accessibility-profiles/torino2007.json")).unwrap();
        let p:Profile=serde_json::from_str(&json).unwrap();p.validate().unwrap();assert_eq!(p.door_var,"door_2");
    }
}
