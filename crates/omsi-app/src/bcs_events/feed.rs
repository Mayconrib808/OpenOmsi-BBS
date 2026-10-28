//! Bounded capture reads on a worker, outside the render thread. No history replay.
use super::lifecycle::Snapshot;
use std::{fs, io::Read, path::{Path, PathBuf}, sync::{Arc, Mutex, atomic::{AtomicBool, Ordering}}, time::{Duration, SystemTime, UNIX_EPOCH}};

const FRESH_MS: u64 = 15_000;
const MAX_BYTES: u64 = 65_536;

pub(super) struct Feed {
    value: Arc<Mutex<Option<Snapshot>>>,
    running: Arc<AtomicBool>,
}

impl Feed {
    pub fn start() -> Option<Self> {
        let directory = std::env::var_os("OPENOMSI_BCS_EVENTS_DIR").map(PathBuf::from)
            .or_else(|| std::env::var_os("LOCALAPPDATA").map(|p| PathBuf::from(p).join("OpenOmsi-BBS").join("EventosBeta")))?;
        let value = Arc::new(Mutex::new(None));
        let running = Arc::new(AtomicBool::new(true));
        let worker_value = value.clone();
        let worker_running = running.clone();
        std::thread::Builder::new().name("BCS incident capture reader".into()).spawn(move || {
            while worker_running.load(Ordering::Relaxed) {
                let next = scan(&directory, now_ms());
                if let Ok(mut slot) = worker_value.lock() { *slot = next; }
                std::thread::sleep(Duration::from_millis(250));
            }
        }).ok()?;
        Some(Self { value, running })
    }

    pub fn latest(&self) -> Option<Snapshot> {
        self.value.lock().ok()?.clone().filter(|s| fresh(s.observed_ms, now_ms()))
    }
}

impl Drop for Feed {
    fn drop(&mut self) { self.running.store(false, Ordering::Relaxed); }
}

fn now_ms() -> u64 { SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_millis() as u64 }
fn fresh(observed: u64, now: u64) -> bool { observed <= now.saturating_add(2000) && now.saturating_sub(observed) <= FRESH_MS }

fn scan(directory: &Path, now: u64) -> Option<Snapshot> {
    let mut found = None;
    let entries = fs::read_dir(directory).ok()?;
    for entry in entries.take(256).flatten() {
        let name = entry.file_name();
        let Some(name) = name.to_str() else { continue };
        let Some(pid) = name.strip_prefix("bcs-events-").and_then(|n| n.strip_suffix(".json")).and_then(|n| n.parse::<u32>().ok()) else { continue };
        let Ok(meta) = entry.metadata() else { continue };
        if !meta.is_file() || meta.len() > MAX_BYTES { continue; }
        let mut bytes = Vec::new();
        let Ok(file) = fs::File::open(entry.path()) else { continue };
        if file.take(MAX_BYTES + 1).read_to_end(&mut bytes).is_err() || bytes.len() as u64 > MAX_BYTES { continue; }
        let Some(snapshot) = decode(&bytes, now) else { continue };
        if snapshot.pid != pid { continue; }
        // Two active BCS JVMs cannot be associated with this bus reliably. Do not guess.
        if found.is_some() { return None; }
        found = Some(snapshot);
    }
    found
}

pub(super) fn decode(bytes: &[u8], now: u64) -> Option<Snapshot> {
    let v: serde_json::Value = serde_json::from_slice(bytes).ok()?;
    if v.get("schema")?.as_u64()? != 1 || v.get("status")?.as_str()? != "compatible" { return None; }
    let observed_ms = timestamp(v.get("observed_at")?.as_str()?)?;
    if !fresh(observed_ms, now) { return None; }
    let pid = v.get("pid")?.as_str()?.parse::<u32>().ok().filter(|p| *p > 0)?;
    let event = u32::try_from(v.get("event_id")?.as_u64()?).ok()?;
    let instance = v.get("event_instance")?.as_u64()?;
    let service = u32::try_from(v.get("requested_service_id")?.as_u64()?).ok()?;
    if !matches!(event, 0..=4 | 6 | 7 | 30..=34) || !matches!(service, 0 | 2 | 3 | 5 | 6 | 7) || (event > 0 && instance == 0) { return None; }
    let completed = v.get("popup_completed_flag").and_then(|v| v.as_bool()).unwrap_or(false)
        && v.get("manager_event_id")?.as_u64()? == 0;
    Some(Snapshot { pid, event, instance, service, completed,
        delay_elapsed: v.get("service_delay_elapsed")?.as_bool()?, observed_ms })
}

// Java Instant's UTC format. Validate the timestamp itself, not the file's copy time.
fn timestamp(text: &str) -> Option<u64> {
    let (date, clock) = text.strip_suffix('Z')?.split_once('T')?;
    let d: Vec<i64> = date.split('-').map(str::parse).collect::<Result<_, _>>().ok()?;
    let (clock, fraction) = clock.split_once('.').unwrap_or((clock, ""));
    let t: Vec<i64> = clock.split(':').map(str::parse).collect::<Result<_, _>>().ok()?;
    if d.len() != 3 || t.len() != 3 || !(1970..=9999).contains(&d[0]) || !(1..=12).contains(&d[1]) || !(0..24).contains(&t[0]) || !(0..60).contains(&t[1]) || !(0..60).contains(&t[2]) || fraction.len() > 9 || !fraction.bytes().all(|b| b.is_ascii_digit()) { return None; }
    let leap = d[0] % 4 == 0 && (d[0] % 100 != 0 || d[0] % 400 == 0);
    let days = [31, if leap {29} else {28}, 31,30,31,30,31,31,30,31,30,31];
    if d[2] < 1 || d[2] > days[(d[1] - 1) as usize] { return None; }
    let y = d[0] - i64::from(d[1] <= 2);
    let era = y / 400;
    let year = y - era * 400;
    let month = d[1] + if d[1] > 2 { -3 } else { 9 };
    let day = (153 * month + 2) / 5 + d[2] - 1;
    let unix_days = era * 146097 + year * 365 + year / 4 - year / 100 + day - 719468;
    let millis: u64 = format!("{fraction:0<3}").chars().take(3).collect::<String>().parse().ok()?;
    u64::try_from((unix_days * 86400 + t[0] * 3600 + t[1] * 60 + t[2]) * 1000 + millis as i64).ok()
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test] fn java_instant_and_calendar() {
        assert_eq!(timestamp("1970-01-01T00:00:00Z"), Some(0));
        assert_eq!(timestamp("2026-10-28T15:45:14.650Z"), Some(1793202314650));
        assert_eq!(timestamp("2026-02-29T12:00:00Z"), None);
        assert_eq!(timestamp("2024-02-29T12:00:00.123456789Z"), Some(1709208000123));
    }
    #[test] fn rejects_stale_future_wrong_schema_and_non_bcs_processes() {
        let raw = br#"{"schema":1,"pid":"29780","status":"compatible","event_id":34,"event_instance":1,"requested_service_id":6,"manager_event_id":34,"popup_completed_flag":false,"service_delay_elapsed":false,"observed_at":"2026-10-28T15:45:14.650Z"}"#;
        assert!(decode(raw, 1793202314650).is_some());
        assert!(decode(raw, 1793202330651).is_none());
        assert!(decode(raw, 1793202310000).is_none());
        assert!(decode(&String::from_utf8_lossy(raw).replace("compatible", "waiting_for_bcs").into_bytes(), 1793202314650).is_none());
    }
}
