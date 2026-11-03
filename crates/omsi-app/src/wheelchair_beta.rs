//! Explicit beta controls, kept out of the released Bridge updater.
use crate::App;
use omsi_sim::people::wheelchair::{self, Stage};
use std::sync::atomic::{AtomicBool, Ordering};
use winit::keyboard::KeyCode;
static NATIVE_ASSIST: AtomicBool = AtomicBool::new(false);

impl App {
    pub(crate) fn wheelchair_key(&mut self, key: KeyCode) -> bool {
        if !std::env::var("OMSI_WHEELCHAIR_BETA").is_ok_and(|v| v == "1") {
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
