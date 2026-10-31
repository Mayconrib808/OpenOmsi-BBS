//! Scene intent only. The companion never completes an event or calls a BCS service.

#[derive(Clone, Debug)]
pub(super) struct Snapshot {
    pub pid: u32, pub event: u32, pub instance: u64, pub service: u32,
    pub completed: bool, pub delay_elapsed: bool, pub observed_ms: u64,
    pub inspector: Option<InspectorSnapshot>,
}

#[derive(Clone, Debug)]
pub(super) struct InspectorSnapshot {
    pub id: u32, pub name: String, pub instance: u64,
    pub phase: String, pub checked: u32,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(super) struct Key { pub pid: u32, pub instance: u64 }

#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub(super) enum Stage {
    #[default] Idle,
    PatientUnwell,
    AmbulanceRequested,
    AwaitingAmbulance,
    ResolvedByBcs,
}

#[derive(Default)]
pub(super) struct Controller {
    context: Option<String>,
    blocked: Option<Key>,
    pub key: Option<Key>,
    pub patient: Option<u32>,
    selected: bool,
    pub blend: f32,
    pub stage: Stage,
}

impl Controller {
    pub fn step(&mut self, context: Option<&str>, snapshot: Option<&Snapshot>, riders: &[u32], dt: f32) {
        let candidate = snapshot.filter(|s| matches!(s.event, 1..=4)).map(|s| Key { pid: s.pid, instance: s.instance });
        if self.context.as_deref() != context {
            // An incident already assigned to another bus/map may never follow a switch.
            self.blocked = self.key.or(self.blocked);
            self.context = context.map(str::to_owned);
            self.key = None; self.patient = None; self.selected = false; self.blend = 0.0;
        }
        if context.is_none() || snapshot.is_none() {
            self.blocked = self.key.or(self.blocked);
            self.key = None; self.patient = None; self.blend = 0.0; self.stage = Stage::Idle;
            return;
        }
        let snapshot = snapshot.unwrap();
        let valid = candidate.filter(|k| Some(*k) != self.blocked);
        if valid != self.key {
            self.key = valid; self.patient = None; self.selected = false; self.blend = 0.0;
        }
        if valid.is_some() && !snapshot.completed {
            if !self.selected {
                self.patient = riders.iter().copied().min();
                self.selected = self.patient.is_some();
            }
            if self.patient.is_some_and(|p| !riders.contains(&p)) {
                self.patient = None; // never transfer the illness to somebody else
            }
            self.stage = if snapshot.service != 3 { Stage::PatientUnwell }
                else if snapshot.delay_elapsed { Stage::AwaitingAmbulance }
                else { Stage::AmbulanceRequested };
        } else {
            self.stage = if valid.is_some() { Stage::ResolvedByBcs } else { Stage::Idle };
        }
        let want = if valid.is_some() && !snapshot.completed && self.patient.is_some() { 1.0 } else { 0.0 };
        self.blend += (want - self.blend).clamp(-dt.max(0.0).min(0.25), dt.max(0.0).min(0.25));
        if self.blend <= 0.0 && (valid.is_none() || snapshot.completed) { self.patient = None; }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn faint() -> Snapshot { Snapshot { pid: 10, event: 4, instance: 1, service: 0, completed: false, delay_elapsed: false, observed_ms: 0, inspector: None } }
    #[test] fn wrong_service_deadline_and_actual_completion_are_distinct() {
        let mut c = Controller::default(); let mut s = faint();
        c.step(Some("bus A"), Some(&s), &[8,2,7], 0.25);
        assert_eq!(c.patient, Some(2));
        s.service=2; c.step(Some("bus A"),Some(&s),&[2,7,8],0.25);
        assert_eq!(c.stage,Stage::PatientUnwell);
        s.service=3; c.step(Some("bus A"),Some(&s),&[2,7,8],0.25);
        assert_eq!(c.stage,Stage::AmbulanceRequested);
        s.delay_elapsed=true; c.step(Some("bus A"),Some(&s),&[2,7,8],0.25);
        assert_eq!(c.stage,Stage::AwaitingAmbulance);
        s.completed=true; c.step(Some("bus A"),Some(&s),&[2,7,8],0.25);
        assert_eq!(c.stage,Stage::ResolvedByBcs);
    }
    #[test] fn lotacao_missing_source_and_switching_bus_do_not_create_patients() {
        let mut c=Controller::default();let mut s=faint();s.event=34;
        c.step(Some("A"),Some(&s),&[1],0.25);assert!(c.patient.is_none());
        s.event=4;c.step(Some("A"),Some(&s),&[1],0.25);assert_eq!(c.patient,Some(1));
        c.step(Some("B"),Some(&s),&[2],0.25);assert!(c.patient.is_none());
        s.instance=2;c.step(Some("B"),Some(&s),&[2],0.25);assert_eq!(c.patient,Some(2));
        c.step(Some("B"),None,&[2],0.25);assert_eq!(c.blend,0.0);
    }
    #[test] fn a_passenger_leaving_does_not_make_another_passenger_faint() {
        let mut c=Controller::default();let s=faint();
        c.step(Some("A"),Some(&s),&[1,2],0.25);
        c.step(Some("A"),Some(&s),&[2],0.25);
        c.step(Some("A"),Some(&s),&[2],0.25);assert!(c.patient.is_none());
    }
    #[test] fn paused_selection_and_stale_source_do_not_reassign_the_patient() {
        let mut c=Controller::default();let s=faint();
        c.step(Some("A"),Some(&s),&[1],0.0);assert_eq!(c.patient,Some(1));
        c.step(Some("A"),Some(&s),&[1],0.25);assert!(c.blend>0.0);
        c.step(Some("A"),None,&[1],0.25);assert!(c.patient.is_none());
        c.step(Some("A"),Some(&s),&[2],0.25);assert!(c.patient.is_none());
    }
}
