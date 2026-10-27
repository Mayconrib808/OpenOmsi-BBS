//! Optional driver sharing over protocol 14's authenticated CMD envelope.
//! The original vehicle remains the only simulation and BCS evaluation owner.
use serde::{Deserialize, Serialize};

pub const PREFIX: &str = "DRIVE1 ";
pub const MAX_WIRE: usize = 4096;
pub const INPUT_TIMEOUT_MS: u64 = 750;
pub const LEASE_TIMEOUT_MS: u64 = 3000;
pub const FRAGMENT_PREFIX: &str = "DRV1F ";
pub const COMPACT_INPUT_PREFIX: &str = "DRV1I ";

/// CMD is limited to 160 characters by existing protocol-14 clients and servers.
/// Hex fragments also keep destination text out of the transport's `|` delimiter.
pub fn fragments(message: &Message, id: u64) -> Vec<String> {
    if let Message::Input { token, seq, frame } = message {
        let compact = format!("{COMPACT_INPUT_PREFIX}{token:x} {seq:x} {}",
            serde_json::to_string(frame).expect("serializable input frame"));
        if compact.len() <= 125 && !compact.contains('|') && !compact.contains('\n') {
            return vec![compact];
        }
    }
    let wire = message.encode();
    wire.as_bytes().chunks(48).enumerate().map(|(i, bytes)| {
        let hex: String = bytes.iter().map(|b| format!("{b:02x}")).collect();
        format!("{FRAGMENT_PREFIX}{id} {i} {} {hex}", wire.len().div_ceil(48))
    }).collect()
}
struct Partial {
    peer: u32,
    id: u64,
    at: u64,
    pieces: Vec<Option<String>>,
}
#[derive(Default)]
pub struct Inbox { pending: Vec<Partial> }
impl Inbox {
    pub fn receive(&mut self, peer: u32, wire: &str, now: u64) -> Option<Message> {
        if let Some(data) = wire.strip_prefix(COMPACT_INPUT_PREFIX) {
            if wire.len() > 125 { return None; }
            let mut fields = data.splitn(3, ' ');
            let token = u64::from_str_radix(fields.next()?, 16).ok()?;
            let seq = u64::from_str_radix(fields.next()?, 16).ok()?;
            let frame: Frame = serde_json::from_str(fields.next()?).ok()?;
            let message = Message::Input { token, seq, frame };
            return message.valid().then_some(message);
        }
        self.pending.retain(|p| now.saturating_sub(p.at) < 1500);
        let mut parts = wire.strip_prefix(FRAGMENT_PREFIX)?.split_whitespace();
        let id = parts.next()?.parse::<u64>().ok()?;
        let index = parts.next()?.parse::<usize>().ok()?;
        let total = parts.next()?.parse::<usize>().ok()?;
        let text = parts.next()?;
        if id == 0 || total == 0 || total > MAX_WIRE.div_ceil(48) || index >= total
            || text.is_empty() || text.len() > 96 || text.len() % 2 != 0
            || !text.bytes().all(|b| b.is_ascii_hexdigit()) || parts.next().is_some() { return None; }
        let pos = match self.pending.iter().position(|p| p.peer == peer && p.id == id) {
            Some(i) => i,
            None => {
                if self.pending.len() >= 64 || self.pending.iter().filter(|p| p.peer == peer).count() >= 16 { return None; }
                self.pending.push(Partial {peer,id,at:now,pieces:vec![None;total]});self.pending.len()-1
            }
        };
        let p = &mut self.pending[pos];
        if p.pieces.len() != total { return None; }
        if let Some(old) = &p.pieces[index] { if old != text { return None; } }
        p.pieces[index] = Some(text.into());
        if p.pieces.iter().any(Option::is_none) { return None; }
        let p = self.pending.swap_remove(pos);
        let hex = p.pieces.into_iter().flatten().collect::<String>();
        if hex.len() > MAX_WIRE * 2 { return None; }
        let bytes: Option<Vec<u8>> = hex.as_bytes().chunks_exact(2).map(|b|
            u8::from_str_radix(std::str::from_utf8(b).ok()?,16).ok()).collect();
        Message::parse(std::str::from_utf8(&bytes?).ok()?)
    }
}

#[derive(Clone, Debug, Default, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Frame {
    /// Steering, throttle, brake and clutch. None leaves the keyboard in charge.
    pub axes: [Option<f32>; 4],
    pub engine: u8,
    /// Complete held keyboard snapshot; a lost release is repaired by the next frame.
    pub keys: Vec<(i32, i32)>,
    pub buttons: Vec<String>,
}
impl Frame {
    pub fn safe() -> Self {
        Self { axes: [None, Some(0.0), Some(1.0), Some(1.0)], ..Self::default() }
    }
    pub fn valid(&self) -> bool {
        self.engine < 128 && self.keys.len() <= 32 && self.buttons.len() <= 32
            && self.axes.iter().enumerate().all(|(i, x)| x.is_none_or(|x|
                x.is_finite() && x >= if i == 0 { -1.0 } else { 0.0 } && x <= 1.0))
            && self.keys.iter().all(|(scan, modifier)| (1..=255).contains(scan) && (0..=7).contains(modifier))
            && self.keys.iter().enumerate().all(|(i, key)| !self.keys[..i].iter().any(|k| k.0 == key.0))
            && self.buttons.iter().all(|b| !b.is_empty() && b.len() <= 96
                && b.bytes().all(|c| c.is_ascii_alphanumeric() || c == b'_'))
            && self.buttons.iter().enumerate().all(|(i, b)| !self.buttons[..i].contains(b))
    }
}

#[derive(Clone, Debug, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", deny_unknown_fields)]
pub enum Message {
    Request { nonce: u64 },
    Offer { nonce: u64, token: u64 },
    Ready { token: u64 },
    Status { token: u64 },
    Input { token: u64, seq: u64, frame: Frame },
    Release { token: u64 },
    Sign { token: u64, seq: u64, route: String, lines: [String; 2], clear: bool },
    SignAck { token: u64, seq: u64, ok: bool },
}
impl Message {
    pub fn valid(&self) -> bool {
        match self {
            Self::Request { nonce } => *nonce != 0,
            Self::Offer { nonce, token } => *nonce != 0 && *token != 0,
            Self::Ready { token } | Self::Status { token } | Self::Release { token } => *token != 0,
            Self::Input { token, seq, frame } => *token != 0 && *seq != 0 && frame.valid(),
            Self::Sign { token, seq, route, lines, .. } => *token != 0 && *seq != 0
                && route.chars().count() <= 12 && lines.iter().all(|s| s.chars().count() <= 60)
                && std::iter::once(route).chain(lines.iter()).all(|s| !s.chars().any(char::is_control)),
            Self::SignAck { token, seq, .. } => *token != 0 && *seq != 0,
        }
    }
    pub fn encode(&self) -> String {
        format!("{PREFIX}{}", serde_json::to_string(self).expect("finite validated driver message"))
    }
    pub fn parse(wire: &str) -> Option<Self> {
        if wire.len() > MAX_WIRE { return None; }
        let m: Self = serde_json::from_str(wire.strip_prefix(PREFIX)?).ok()?;
        m.valid().then_some(m)
    }
}

/// The authority gate is independent of rendering and vehicle scripts so regressions
/// exercise the very checks used when accepting a remote driver's real inputs.
#[derive(Clone, Debug)]
pub struct Lease {
    pub peer: u32,
    pub token: u64,
    pub uid: u64,
    pub active: bool,
    pub seq: u64,
    pub last_input_ms: u64,
}
impl Lease {
    pub fn activate(&mut self, peer: u32, token: u64, uid: u64, inside: bool, speed: f32, now: u64) -> bool {
        if self.peer != peer || self.token != token || self.uid != uid || !inside
            || (!self.active && (!speed.is_finite() || speed.abs() > 0.5)) { return false; }
        if !self.active { self.last_input_ms = now; }
        self.active = true;
        true
    }
    pub fn accept(&mut self, peer: u32, token: u64, uid: u64, inside: bool, seq: u64, frame: &Frame, now: u64) -> bool {
        if !self.active || peer != self.peer || token != self.token || uid != self.uid || !inside
            || seq <= self.seq || !frame.valid() { return false; }
        self.seq = seq;
        self.last_input_ms = now;
        true
    }
    pub fn stale(&self, now: u64) -> bool { now.saturating_sub(self.last_input_ms) > INPUT_TIMEOUT_MS }
    pub fn expired(&self, now: u64) -> bool { now.saturating_sub(self.last_input_ms) > LEASE_TIMEOUT_MS }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn lease() -> Lease { Lease { peer: 2, token: 42, uid: 7, active: false, seq: 0, last_input_ms: 0 } }
    #[test]
    fn handshake_requires_correct_bus_peer_permission_and_stationary_bus() {
        let mut l = lease();
        for (peer, token, uid, inside, speed) in [(3,42,7,true,0.),(2,41,7,true,0.),(2,42,8,true,0.),(2,42,7,false,0.),(2,42,7,true,1.),(2,42,7,true,f32::NAN)] {
            assert!(!l.activate(peer,token,uid,inside,speed,100));
        }
        assert!(l.activate(2,42,7,true,0.,100));
        // A retransmitted READY while rolling must not cancel the existing driver.
        assert!(l.activate(2,42,7,true,35.,200));
        assert_eq!(l.last_input_ms,100);
    }
    #[test]
    fn replay_wrong_peer_old_bus_and_ungranted_driver_cannot_drive() {
        let mut l=lease();let f=Frame::default();
        assert!(!l.accept(2,42,7,true,1,&f,100));
        l.activate(2,42,7,true,0.,100);
        assert!(l.accept(2,42,7,true,2,&f,120));
        for (peer,token,uid,inside,seq) in [(2,42,7,true,1),(2,42,7,true,2),(3,42,7,true,3),(2,43,7,true,3),(2,42,8,true,3),(2,42,7,false,3)] {
            assert!(!l.accept(peer,token,uid,inside,seq,&f,150));
        }
        assert_eq!(l.seq,2);assert_eq!(l.last_input_ms,120);
    }
    #[test]
    fn lost_input_brakes_before_lease_is_returned() {
        let mut l=lease();l.activate(2,42,7,true,0.,100);
        assert!(!l.stale(850));assert!(l.stale(851));assert!(!l.expired(851));assert!(l.expired(3101));
        let f=Frame::safe();assert!(f.valid());assert_eq!(f.axes[1],Some(0.));assert_eq!(f.axes[2],Some(1.));
    }
    #[test]
    fn snapshots_and_destination_text_roundtrip_and_reject_unbounded_inputs() {
        let messages=[Message::Input {token:42,seq:1,frame:Frame {axes:[Some(-0.5),Some(1.),None,None],engine:1,keys:vec![(30,0)],buttons:vec!["door_1".into()]}},
            Message::Sign{token:42,seq:2,route:"205".into(),lines:["São Paulo".into(),"Terminal".into()],clear:false}];
        for m in messages {assert_eq!(Message::parse(&m.encode()),Some(m));}
        assert!(Message::parse(&format!("{PREFIX}{}","x".repeat(MAX_WIRE))).is_none());
        assert!(Message::parse("DRIVE1 {\"type\":\"Ready\",\"token\":0}").is_none());
        let mut f=Frame::default();f.axes[1]=Some(f32::NAN);assert!(!f.valid());
        f.axes[1]=Some(1.1);assert!(!f.valid());f.axes[1]=None;
        f.keys=vec![(30,0),(30,1)];assert!(!f.valid());
    }
    #[test]
    fn real_protocol14_command_limit_carries_unicode_sign_and_full_input_snapshots() {
        let m=Message::Sign {token:u64::MAX,seq:u64::MAX,route:"205".into(),lines:["São Paulo | Centro".into(),"Terminal João".into()],clear:false};
        let parts=fragments(&m,u64::MAX);let mut inbox=Inbox::default();let mut got=None;
        for part in parts.iter().rev() { assert!(part.len()<=160);assert!(!part.contains('|'));got=inbox.receive(2,part,100).or(got); }
        assert_eq!(got,Some(m));
    }
    #[test]
    fn compact_input_uses_one_authenticated_command_and_falls_back_for_large_frames() {
        let m = Message::Input { token: 42, seq: 5, frame: Frame::safe() };
        let parts = fragments(&m, 7);
        assert_eq!(parts.len(), 1);
        assert!(parts[0].len() <= 125);
        assert!(parts[0].starts_with(COMPACT_INPUT_PREFIX));
        assert!(!parts[0].contains('|'));
        let mut inbox = Inbox::default();
        assert_eq!(inbox.receive(2, &parts[0], 100), Some(m));
        assert!(inbox.receive(2, "DRV1I 0 2 {}", 200).is_none());
        assert!(inbox.receive(2, "DRV1I 2 nothex {}", 200).is_none());
    }
    #[test]
    fn fragments_from_other_peers_and_expired_requests_cannot_be_combined() {
        let m=Message::Input {token:42,seq:1,frame:Frame {buttons:vec!["large_valid_button_name_one".into(),"large_valid_button_name_two".into(),"large_valid_button_name_three".into()],..Frame::safe()}};let parts=fragments(&m,7);assert!(parts.len()>1);
        let mut inbox=Inbox::default();
        assert!(inbox.receive(2,&parts[0],0).is_none());
        for part in parts.iter().skip(1) {assert!(inbox.receive(3,part,100).is_none());}
        for part in parts.iter().skip(1) {assert!(inbox.receive(2,part,2000).is_none());}
        assert!(inbox.receive(2,"DRV1F 9 0 999999 aa",2100).is_none());
    }
}
