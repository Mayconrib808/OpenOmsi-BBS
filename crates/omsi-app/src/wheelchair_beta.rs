//! Native accessibility equipment and authenticated cooperative snapshots.
use crate::App;
use omsi_sim::people::wheelchair::{self, Stage};
use std::sync::atomic::{AtomicU8, Ordering};
static NATIVE_STAGE: AtomicU8 = AtomicU8::new(0);

impl App {
    pub(crate) fn tick_wheelchair_beta(&mut self) {
        self.tick_wheelchair_network();
        let Some(h)=self.session.humans.as_ref()else{NATIVE_STAGE.store(0,Ordering::Release);return;};
        let Some(profile)=h.wheelchairs.profile.as_ref()else{NATIVE_STAGE.store(0,Ordering::Release);return;};
        let Some(player)=self.player.as_mut()else{return;};
        if h.wheelchairs.journey.as_ref().is_some_and(|j|!matches!(j.stage,Stage::Finished|Stage::Riding))
            &&player.vehicle.physics.speed.abs()<0.025&&h.wheelchairs.feedback.door_open&&!h.wheelchairs.bay_occupied {
            if let Some(f)=&profile.fold {if player.vehicle.var(&f.variable).is_some_and(|x|(x-f.folded_value).abs()>0.02){player.vehicle.trigger(&f.trigger);}}
        }
        let native=matches!(profile.equipment,wheelchair::Equipment::NativeRamp|wheelchair::Equipment::NativeLift);
        let action=h.wheelchairs.journey.as_ref().map(|j|match j.stage {
            Stage::Deploying=>(1,profile.deploy_trigger.clone()),Stage::Raising=>(2,profile.raise_trigger.clone()),
            Stage::Riding if j.deployment>0.0=>(3,profile.stow_trigger.clone()),Stage::Unsecuring=>(4,profile.alight_trigger.clone()),
            Stage::Lowering=>(5,profile.lower_trigger.clone()),Stage::Finished if j.deployment>0.0=>(6,profile.stow_trigger.clone()),_ =>(0,None)
        }).unwrap_or((0,None));
        if !native||action.0==0{NATIVE_STAGE.store(0,Ordering::Release);return;}
        if NATIVE_STAGE.load(Ordering::Acquire)==action.0{return;}
        if player.vehicle.physics.speed.abs()>0.025||!h.wheelchairs.feedback.door_open{return;}
        if let Some(trigger)=action.1 {
            let desired=if matches!(action.0,1|4){1.0}else{0.0};let control=profile.native.as_ref().and_then(|n|n.control_var.as_ref());
            if !control.is_some_and(|n|player.vehicle.var(n).is_some_and(|x|(x-desired).abs()<0.1)) {
                static LAST_TRIGGER:std::sync::OnceLock<std::sync::Mutex<std::time::Instant>>=std::sync::OnceLock::new();
                let Ok(mut last)=LAST_TRIGGER.get_or_init(||std::sync::Mutex::new(std::time::Instant::now()-std::time::Duration::from_secs(2))).lock()else{return;};
                if last.elapsed()<std::time::Duration::from_secs(1){return;}
                player.vehicle.trigger(&trigger);*last=std::time::Instant::now();
                if control.is_some_and(|n|!player.vehicle.var(n).is_some_and(|x|(x-desired).abs()<0.1)){return;}
            }
        }
        NATIVE_STAGE.store(action.0,Ordering::Release);
    }
    pub(crate) fn wheelchair_replacement_visibility(&mut self) {
        let Some(h)=self.session.humans.as_ref()else{return;};let Some(profile)=h.wheelchairs.profile.as_ref()else{return;};
        if !h.wheelchairs.journey.as_ref().is_some_and(|j|j.deployment>0.001){return;}
        let Some(player)=self.player.as_mut()else{return;};
        for (i,m) in player.vehicle.ty.meshes.iter().enumerate() {
            let name=&player.vehicle.ty.model.meshes[m.def_index].file;
            if profile.replacement_meshes.iter().any(|n|n.eq_ignore_ascii_case(name)) {if let Some(p)=player.vehicle.mesh_props.get_mut(i){p.visible=false;}}
        }
    }
}

static LAST_SEND:std::sync::OnceLock<std::sync::Mutex<wheelchair::network::SendSchedule>>=std::sync::OnceLock::new();
impl App {
    fn tick_wheelchair_network(&mut self){
        let Some(h)=self.session.humans.as_ref().filter(|h|h.wheelchairs.enabled)else{return;};
        let Some(lan)=self.net.lan.as_ref()else{return;};
        let mut peers:Vec<_>=lan.peers().filter(|p|p.has_pose).map(|p|p.pose.id).collect();
        peers.sort_unstable();
        let active=h.wheelchairs.journey.is_some()&&h.wheelchairs.person.is_some();
        let Ok(mut schedule)=LAST_SEND.get_or_init(||std::sync::Mutex::new(Default::default())).lock()else{return;};
        let to=schedule.recipient(std::time::Instant::now(),&peers,active);drop(schedule);
        if let Some(to)=to {let text=h.wheelchair_wire();if let Some(lan)=self.net.lan.as_mut(){lan.command(to,&text);}}
    }
    pub(crate) fn receive_wheelchair_beta(&mut self,from:u32,text:&str)->bool{
        if !text.starts_with(wheelchair::network::PREFIX){return false;}
        let Some(lan)=self.net.lan.as_ref()else{return true;};
        if from==lan.my_id||!lan.peers().any(|p|p.pose.id==from&&p.has_pose){return true;}
        if let Some(h)=self.session.humans.as_mut(){h.receive_wheelchair(from,text);}true
    }
}
