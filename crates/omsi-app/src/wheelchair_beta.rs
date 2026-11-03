//! Explicit beta controls, kept out of the released Bridge updater.
use crate::App;
use omsi_sim::people::wheelchair::{self, Stage};
use std::sync::atomic::{AtomicBool, Ordering};
use winit::keyboard::KeyCode;
static NATIVE_ASSIST: AtomicBool = AtomicBool::new(false);

impl App {
    pub(crate) fn wheelchair_key(&mut self, key: KeyCode) -> bool {
        if !wheelchair::beta_enabled() {
            return false;
        }
        let command = match key {
            KeyCode::KeyW => wheelchair::SPAWN,
            KeyCode::KeyE => wheelchair::ASSIST,
            KeyCode::KeyD => wheelchair::ALIGHT,
            _ => return false,
        };
        wheelchair::command(command);
        if command == wheelchair::ASSIST {
            NATIVE_ASSIST.store(true, Ordering::Release);
        }
        true
    }
    pub(crate) fn tick_wheelchair_beta(&mut self) {
        self.tick_wheelchair_network();
        if !NATIVE_ASSIST.load(Ordering::Acquire) {
            return;
        }
        let Some(h) = self.session.humans.as_ref() else {
            return;
        };
        let Some(profile) = h.wheelchairs.profile.as_ref() else {
            NATIVE_ASSIST.store(false, Ordering::Release);
            return;
        };
        let stowing = h
            .wheelchairs
            .journey
            .as_ref()
            .is_some_and(|j| j.stage == Stage::Riding && !j.alight_requested);
        let trigger = if stowing {
            profile.stow_trigger.clone()
        } else {
            profile.deploy_trigger.clone()
        };
        if let Some(p) = self.player.as_mut() {
            if p.vehicle.physics.speed.abs() > 0.025 {
                return;
            }
            if let Some(trigger) = trigger {
                p.vehicle.trigger(&trigger);
            }
        }
        NATIVE_ASSIST.store(false, Ordering::Release);
    }
}

static LAST_SEND:std::sync::OnceLock<std::sync::Mutex<std::time::Instant>>=std::sync::OnceLock::new();
impl App {
    fn tick_wheelchair_network(&mut self){
        let Some(h)=self.session.humans.as_ref().filter(|h|h.wheelchairs.enabled)else{return;};
        if self.net.lan.is_none(){return;}
        let now=std::time::Instant::now();
        let Ok(mut last)=LAST_SEND.get_or_init(||std::sync::Mutex::new(now-std::time::Duration::from_secs(1))).lock()else{return;};
        if now.duration_since(*last)<std::time::Duration::from_millis(200){return;}
        *last=now;drop(last);let text=h.wheelchair_wire();
        if let Some(lan)=self.net.lan.as_mut(){
            let peers:Vec<_>=lan.peers().filter(|p|p.has_pose).map(|p|p.pose.id).collect();
            for peer in peers{lan.command(peer,&text);}
        }
    }
    pub(crate) fn receive_wheelchair_beta(&mut self,from:u32,text:&str)->bool{
        if !text.starts_with(wheelchair::network::PREFIX){return false;}
        let Some(lan)=self.net.lan.as_ref()else{return true;};
        if from==lan.my_id||!lan.peers().any(|p|p.pose.id==from&&p.has_pose){return true;}
        if let Some(h)=self.session.humans.as_mut(){h.receive_wheelchair(from,text);}true
    }
}
