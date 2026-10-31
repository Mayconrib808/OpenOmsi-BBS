//! BCS intent stays with its player's bus. The LAN host alone dispatches shared AI.
//! Compact, repeated snapshots use the authenticated existing command channel.
use super::{assets::Loader,dispatch::{Dispatcher,Input},lifecycle::{Key,Snapshot,Stage}};
use glam::{DVec3,Vec3};
use std::{collections::HashMap,time::{Duration,Instant}};

const REQUEST:&str="bcs222_req ";
const VIEW:&str="bcs222_view ";
const DONE:&str="bcs222_done ";

#[derive(Clone)]
pub(super) struct Visual {
    pub source:Snapshot,pub actor:Option<Vec3>,pub blend:f32,pub phase:f32,pub heard:Instant,
}
impl Visual {
    fn encode(&self)->String {
        let p=self.actor.unwrap_or(Vec3::splat(99.0));
        format!("{} {} {} {} {} {} {} {} {:.2} {:.2} {:.2}",self.source.pid,self.source.instance,
            self.source.event,self.source.service,u8::from(self.source.completed),
            (self.blend.clamp(0.0,1.0)*255.0)as u8,
            (self.phase.rem_euclid(std::f32::consts::TAU)/std::f32::consts::TAU*255.0)as u8,
            self.source.inspector.as_ref().map_or(0,|_|0),p.x,p.y,p.z)
    }
    fn decode(text:&str)->Option<Self>{
        if text.len()>145{return None;}
        let v=text.split_whitespace().collect::<Vec<_>>();if v.len()!=11{return None;}
        let pid=v[0].parse::<u32>().ok()?;let instance=v[1].parse::<u64>().ok()?;
        let event=v[2].parse::<u32>().ok()?;let service=v[3].parse::<u32>().ok()?;
        let done=v[4].parse::<u8>().ok()?;let blend=v[5].parse::<u8>().ok()?as f32/255.0;
        let phase=v[6].parse::<u8>().ok()?as f32/255.0*std::f32::consts::TAU;
        let p=Vec3::new(v[8].parse().ok()?,v[9].parse().ok()?,v[10].parse().ok()?);
        if pid==0||done>1||!matches!(event,0..=4|6|7|30..=34)||!matches!(service,0|2|3|5|6|7)||(event>0&&instance==0)||!p.is_finite(){return None;}
        let actor=(p.abs().max_element()<40.0).then_some(p);
        Some(Self{source:Snapshot{pid,event,instance,service,completed:done==1,delay_elapsed:false,observed_ms:0,inspector:None},actor,blend,phase,heard:Instant::now()})
    }
}

struct Response {
    visual:Visual,patient:Option<u32>,dispatcher:Dispatcher,
    returned:Option<(Key,u32)>,reply:f32,
}
#[derive(Default)]
pub(crate) struct HostResponses {entries:HashMap<u32,Response>,models:Loader,relay:f32}
impl HostResponses {
    pub fn receive(&mut self,lan:&omsi_net::LanSession,from:u32,text:&str)->bool{
        let Some(rest)=text.strip_prefix(REQUEST)else{return false};
        if lan.role!=omsi_net::Role::Host||from==lan.my_id||!lan.peers().any(|p|p.pose.id==from){return true;}
        let Some((patient,rest))=rest.split_once(' ')else{return true};let Ok(patient)=patient.parse::<u32>()else{return true};
        let Some(visual)=Visual::decode(rest)else{return true};
        if self.entries.len()>=64&&!self.entries.contains_key(&from){return true;}
        if self.entries.get(&from).is_none_or(|prior|prior.visual.source.instance!=visual.source.instance||prior.visual.source.event!=visual.source.event||prior.visual.source.service!=visual.source.service) {
            log::info!("BCS host: event request from player={} event={} service={} instance={} completed={}",from,visual.source.event,visual.source.service,visual.source.instance,visual.source.completed);
        }
        let entry=self.entries.entry(from).or_insert_with(||Response{visual:visual.clone(),patient:None,dispatcher:Default::default(),returned:None,reply:0.0});
        entry.visual=visual;entry.patient=(patient>0).then_some(patient);
        true
    }
    pub fn tick(&mut self,lan:&mut omsi_net::LanSession,traffic:&mut crate::traffic::Traffic,
        world:&crate::scene::World,dt:f32){
        if lan.role!=omsi_net::Role::Host{return;}
        if !self.entries.is_empty(){self.models.start(world.root.clone());}
        let peers=lan.peers().map(|p|p.pose.id).collect::<Vec<_>>();
        self.entries.retain(|id,entry|{
            let keep=peers.contains(id)&&entry.visual.heard.elapsed()<Duration::from_secs(8);
            if !keep{entry.dispatcher.cancel(traffic);}
            keep
        });
        self.relay-=dt.max(0.0);let relay=self.relay<=0.0;if relay{self.relay=0.75;}
        for (&owner,entry)in &mut self.entries{
            let Some(pose)=lan.peers().find(|p|p.pose.id==owner&&p.has_pose&&!p.pose.bus.is_empty()).map(|p|p.pose.clone())else{entry.dispatcher.cancel(traffic);continue};
            let at=DVec3::new(pose.x,pose.y,pose.z);if !at.is_finite(){continue;}
            let context=format!("{}:{owner}",lan.session);
            let riders=entry.patient.into_iter().collect::<Vec<_>>();
            let source=&entry.visual.source;
            if let Some(patient)=entry.dispatcher.advance(Input{context:Some(&context),source:Some(source),key:Some(Key{pid:source.pid,instance:source.instance}),
                stage:Stage::AwaitingAmbulance,patient:entry.patient,riders:&riders,blend:entry.visual.blend,
                bus:at,heading:pose.heading as f64,speed:pose.speed_kmh,dt},&self.models,traffic,world){
                entry.returned=Some((Key{pid:source.pid,instance:source.instance},patient));entry.reply=0.0;
            }
            entry.reply-=dt.max(0.0);
            if entry.reply<=0.0{
                entry.reply=0.6;
                if let Some((key,patient))=entry.returned{lan.command(owner,&format!("{DONE}{} {} {patient}",key.pid,key.instance));}
            }
            if relay{
                let message=format!("{VIEW}{owner} {}",entry.visual.encode());
                for &peer in &peers{if peer!=owner{lan.command(peer,&message);}}
            }
        }
    }
    fn visuals(&self)->impl Iterator<Item=(u32,&Visual)>{self.entries.iter().map(|(id,e)|(*id,&e.visual))}
}

#[derive(Default)]
pub(super) struct Network {
    pub host:HostResponses,pub remote:HashMap<u32,Visual>,
    session:Option<(u64,u32)>,send:f32,last:Option<Visual>,last_patient:Option<u32>,
    pub returned:Option<(Key,u32)>,applied:Vec<Key>,
}
impl Network {
    pub fn receive(&mut self,lan:&omsi_net::LanSession,from:u32,text:&str)->bool{
        if self.host.receive(lan,from,text){return true;}
        if let Some(rest)=text.strip_prefix(VIEW){
            if from==1&&lan.role==omsi_net::Role::Client{
                if let Some((owner,rest))=rest.split_once(' '){if let (Ok(owner),Some(v))=(owner.parse::<u32>(),Visual::decode(rest)){
                    if owner!=lan.my_id&&self.remote.len()<64{self.remote.insert(owner,v);}
                }}
            }return true;
        }
        if let Some(rest)=text.strip_prefix(DONE){
            if from==1&&lan.role==omsi_net::Role::Client{
                let v=rest.split_whitespace().collect::<Vec<_>>();
                if v.len()==3{if let (Ok(pid),Ok(instance),Ok(patient))=(v[0].parse::<u32>(),v[1].parse::<u64>(),v[2].parse::<u32>()){
                    let key=Key{pid,instance};
                    if !self.applied.contains(&key)&&self.last.as_ref().is_some_and(|s|s.source.pid==pid&&s.source.instance==instance&&s.source.completed&&self.last_patient==Some(patient)){
                        self.returned=Some((key,patient));self.applied.push(key);if self.applied.len()>64{self.applied.remove(0);}
                    }
                }}
            }return true;
        }
        false
    }
    pub fn local(&mut self,lan:&mut omsi_net::LanSession,source:Option<&Snapshot>,patient:Option<u32>,
        actor:Option<Vec3>,blend:f32,phase:f32,dt:f32){
        let session=Some((lan.session,lan.my_id));
        if self.session!=session{self.remote.clear();self.applied.clear();self.last=None;self.last_patient=None;self.returned=None;self.session=session;self.send=0.0;}
        self.remote.retain(|owner,v|v.heard.elapsed()<Duration::from_secs(4)&&lan.peers().any(|p|p.pose.id==*owner));
        self.send-=dt.max(0.0);
        if let Some(source)=source{
            if self.last.as_ref().is_none_or(|old| old.source.instance!=source.instance||old.source.event!=source.event||old.source.service!=source.service) {
                log::info!("BCS client: sending event={} service={} instance={} completed={} to role {:?}",source.event,source.service,source.instance,source.completed,lan.role);
            }
            // Keep the observed completion alive until the host has had time to
            // finish the real response; a popup disappearing is not arrival.
            let keep=self.last.as_ref().is_some_and(|v|v.source.completed&&source.event==0&&v.heard.elapsed()<Duration::from_secs(8));
            if !keep{self.last_patient=patient;self.last=Some(Visual{source:source.clone(),actor,blend,phase,heard:Instant::now()});}
        }else{self.last=None;}
        if self.send<=0.0{
            self.send=0.5;
            if let Some(v)=&self.last{
                let text=format!("{REQUEST}{} {}",self.last_patient.unwrap_or(0),v.encode());
                if lan.role==omsi_net::Role::Client{lan.command(1,&text);}
                else{let peers=lan.peers().map(|p|p.pose.id).collect::<Vec<_>>();for peer in peers{lan.command(peer,&format!("{VIEW}{} {}",lan.my_id,v.encode()));}}
            }
        }
    }
    pub fn visuals(&self)->impl Iterator<Item=(u32,&Visual)>{self.remote.iter().map(|(id,v)|(*id,v)).chain(self.host.visuals())}
}

#[cfg(test)]mod tests{
    use super::*;
    #[test]fn compact_snapshot_fits_command_and_rejects_bad_values(){
        let text="12345 99 4 3 1 255 23 0 0.80 -5.40 1.10";
        let v=Visual::decode(text).unwrap();assert!(v.source.completed);assert_eq!(v.source.event,4);
        assert!(format!("{VIEW}4294967295 {}",v.encode()).chars().count()<160);
        assert!(Visual::decode(&text.replace("0.80","NaN")).is_none());
        assert!(Visual::decode(&text.replace("99 4","0 4")).is_none());
        assert!(Visual::decode(&text.replace("4 3 1","400 3 1")).is_none());
    }
}
