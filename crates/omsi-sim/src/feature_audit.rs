//! Evidence of executed features, not a dump of enabled settings.
//!
//! Host/simulation events and rendered-client events are kept in separate ledgers.
//! A limited sample of concrete decisions is emitted, alongside periodic totals.
//! Each evaluation is counted; no-opportunity and blocked are distinguished.
use std::collections::BTreeMap;
use std::sync::{Mutex, OnceLock};
use std::time::{Duration, Instant};

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Role { Host, Client }
impl Role {
    fn name(self) -> &'static str { match self { Self::Host => "host", Self::Client => "client" } }
}
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum ResultKind { NoOpportunity, Evaluated, Executed, Completed, Blocked, Failed }
impl ResultKind {
    fn name(self) -> &'static str {
        match self {
            Self::NoOpportunity => "no_opportunity",
            Self::Evaluated => "evaluated",
            Self::Executed => "executed",
            Self::Completed => "completed",
            Self::Blocked => "blocked",
            Self::Failed => "failed",
        }
    }
}
#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct Totals {
    pub evaluated: u64,
    pub executed: u64,
    pub completed: u64,
    pub blocked: u64,
    pub failed: u64,
    pub no_opportunity: u64,
}
impl Totals {
    fn add(&mut self, kind: ResultKind) {
        self.evaluated += 1;
        match kind {
            ResultKind::NoOpportunity => self.no_opportunity += 1,
            ResultKind::Evaluated => (),
            ResultKind::Executed => self.executed += 1,
            ResultKind::Completed => self.completed += 1,
            ResultKind::Blocked => self.blocked += 1,
            ResultKind::Failed => { self.failed += 1; self.blocked += 1; },
        }
    }
}
#[derive(Default)]
struct Row {
    totals: Totals,
    reasons: BTreeMap<String,u64>,
    samples: u8,
}
#[derive(Default)]
struct Ledger {
    at: Option<Instant>,
    rows: BTreeMap<(String,String),Row>,
}
static LEDGER: OnceLock<Mutex<Ledger>> = OnceLock::new();
fn global() -> &'static Mutex<Ledger> { LEDGER.get_or_init(|| Mutex::new(Ledger::default())) }
const PERIOD: Duration = Duration::from_secs(60);
const DETAIL_LIMIT: u8 = 4;
const MAX_REASONS: usize = 12;
/// One actual decision. The caller supplies real IDs, positions or file paths in detail.
/// Detail is bounded and logged at most four times per feature/role in each minute.
/// A failed or blocked attempt still increments totals when its detail is suppressed.
pub fn record(role: Role, feature: &'static str, kind: ResultKind, reason: &'static str, detail: &str) {
    let Ok(mut ledger) = global().lock() else { return; };
    let now = Instant::now();
    ledger.at.get_or_insert(now);
    if ledger.at.is_some_and(|t| now.saturating_duration_since(t) >= PERIOD) {
        write_summary(&mut ledger);
        ledger.at = Some(now);
    }
    let row = ledger.rows.entry((role.name().into(),feature.into())).or_default();
    row.totals.add(kind);
    if let Some(n) = row.reasons.get_mut(reason) { *n += 1; }
    else if row.reasons.len() < MAX_REASONS { row.reasons.insert(reason.into(),1); }
    else { *row.reasons.entry("other".into()).or_default() += 1; }
    if row.samples < DETAIL_LIMIT {
        row.samples += 1;
        log::info!("FEATURE EVENT role={} feature={} result={} reason={} {}",role.name(),feature,kind.name(),reason,detail);
    }
}
/// Mark an enabled feature when no actual decision was reached in the interval.
/// Do not misreport a client mirror's lack of host decisions as failure.
pub fn heartbeat(role: Role, feature: &'static str, enabled: bool) {
    let Ok(mut ledger) = global().lock() else { return; };
    let now=Instant::now();
    ledger.at.get_or_insert(now);
    if ledger.at.is_some_and(|t|now.saturating_duration_since(t)>=PERIOD){
        write_summary(&mut ledger);
        ledger.at=Some(now);
    }
    let row=ledger.rows.entry((role.name().into(),feature.into())).or_default();
    if !enabled {row.reasons.entry("disabled".into()).or_insert(1);}
}
fn write_summary(ledger: &mut Ledger) {
    for ((role,feature),row) in &ledger.rows {
        let outcome=if row.totals.evaluated==0 {
            if row.reasons.contains_key("disabled"){"disabled"} else {"no_opportunity_observed"}
        } else if row.totals.failed>0 {"some_failures"}
        else if row.totals.blocked>0 {"some_blocked"}
        else if row.totals.completed>0 {"completed"}
        else if row.totals.executed>0 {"executed"}
        else if row.totals.no_opportunity==row.totals.evaluated {"no_opportunity"}
        else {"evaluated_only"};
        log::info!(
            "FEATURE SUMMARY role={} feature={} evaluated={} executed={} completed={} blocked={} failed={} no_opportunity={} outcome={} reasons={:?}",
            role,feature,row.totals.evaluated,row.totals.executed,row.totals.completed,
            row.totals.blocked,row.totals.failed,row.totals.no_opportunity,outcome,row.reasons,
        );
    }
    ledger.rows.clear();
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn counters_do_not_confuse_absence_with_failure() {
        let mut t=Totals::default();
        t.add(ResultKind::NoOpportunity);
        assert_eq!(t.no_opportunity,1);assert_eq!(t.failed,0);
        t.add(ResultKind::Executed);t.add(ResultKind::Completed);
        t.add(ResultKind::Blocked);t.add(ResultKind::Failed);
        assert_eq!(t.evaluated,5);assert_eq!(t.executed,1);
        assert_eq!(t.completed,1);assert_eq!(t.blocked,2);assert_eq!(t.failed,1);
    }
    #[test]
    fn roles_are_kept_separate() {
        assert_ne!(Role::Host.name(),Role::Client.name());
    }
}
