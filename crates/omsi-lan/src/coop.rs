//! Optional cooperative controls, carried in protocol 14's existing CMD envelope.
//! No game without this extension is required to understand these messages.
use std::collections::VecDeque;
use std::time::{Duration, Instant};

pub const PREFIX: &str = "COOP1";
pub const RETRY: Duration = Duration::from_millis(180);
pub const TIMEOUT: Duration = Duration::from_secs(5);

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Target {
    Player,
    Ai(u64),
}
impl Target {
    fn text(self) -> u64 {
        match self {
            Self::Player => 0,
            Self::Ai(id) => id,
        }
    }
    fn parse(id: u64) -> Self {
        if id == 0 {
            Self::Player
        } else {
            Self::Ai(id)
        }
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Phase {
    Down,
    Up,
    Drag,
    Tap,
}
impl Phase {
    fn number(self) -> u8 {
        match self {
            Self::Down => 0,
            Self::Up => 1,
            Self::Drag => 2,
            Self::Tap => 3,
        }
    }
    fn parse(n: u8) -> Option<Self> {
        Some(match n {
            0 => Self::Down,
            1 => Self::Up,
            2 => Self::Drag,
            3 => Self::Tap,
            _ => return None,
        })
    }
}

#[derive(Clone, Debug, PartialEq)]
pub struct Action {
    pub epoch: u64,
    pub seq: u64,
    pub target: Target,
    /// The real passenger at the cash desk. Stale cash commands must not reach the next one.
    pub desk: u32,
    /// 0 mesh, 1 HTML/script texture, 2 native ticket, 3 change coin, 4 conductor closing signal, 5 collect passenger payment.
    pub kind: u8,
    /// 0 front section; 1.. trailer section.
    pub part: u8,
    pub index: u32,
    pub signature: u64,
    pub phase: Phase,
    pub x: f32,
    pub y: f32,
}

/// Does the action affect the current fare transaction? Flaps, windows and other
/// ordinary controls remain usable while the desk changes passengers.
pub fn desk_bound(kind: u8, event: Option<&str>) -> bool {
    if kind != 0 {
        return kind != 4; // Closing cue is not tied to an individual ticket transaction.
    }
    let event = event.unwrap_or_default().to_ascii_lowercase();
    [
        "cashdesk",
        "ticket",
        "fahrschein",
        "fahrkart",
        "kasse",
        "drucker",
        "printer",
        "ibox",
        "almex",
        "atron",
    ]
    .iter()
    .any(|name| event.contains(name))
}

/// A passenger may ring a bell. Only the appointed conductor may operate other
/// controls; cash operations must also belong to the current passenger.
pub fn authorized(
    grant: Option<(u32, u64)>,
    actor: u32,
    a: &Action,
    inside: bool,
    passenger: bool,
    desk_bound: bool,
    desk: u32,
) -> bool {
    if !inside || a.kind > 5 {
        return false;
    }
    if a.kind == 0 && passenger && a.epoch == 0 {
        return true;
    }
    grant == Some((actor, a.epoch))
        && a.epoch != 0
        && a.target == Target::Player
        && (a.kind == 0 || a.part == 0)
        && (!desk_bound || passenger || a.phase == Phase::Up || a.desk == desk)
}

#[derive(Clone, Debug, Default, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct Desk {
    pub epoch: u64,
    pub id: u32,
    pub stage: u8,
    pub price: f32,
    pub paid: f32,
    pub due: f32,
    pub given: f32,
    /// Payment still lying on the desk after the last ticket sale, even with no active passenger.
    #[serde(default)]
    pub uncollected: f32,
    pub name: String,
}

/// Exact local transforms of physical payment and change pieces, without GPU handles.
#[derive(Clone, Debug, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct CashPiece {
    pub coin: u32,
    pub local: [f32; 16],
    pub change: bool,
    pub parent: Option<String>,
}
#[derive(Clone, Debug, Default, PartialEq, serde::Serialize, serde::Deserialize)]
pub struct CashView {
    pub desk: u32,
    pub pieces: Vec<CashPiece>,
}
impl CashView {
    pub fn valid(&self) -> bool {
        self.pieces.len() <= 512 && self.pieces.iter().all(|p| {
            p.coin < 4096 && p.local.iter().all(|x| x.is_finite() && x.abs() < 1000.0)
                && p.parent.as_ref().is_none_or(|s| s.len() <= 256)
        })
    }
}

#[derive(Clone, Debug, PartialEq)]
pub enum Message {
    Request,
    Grant(u64),
    Revoke(u64),
    Keep(u64),
    Action(Action),
    Ack { epoch: u64, seq: u64, ok: bool },
    Desk(Desk),
}

fn hex_name(s: &str) -> String {
    let mut bytes = 0;
    s.chars()
        .take_while(|c| {
            bytes += c.len_utf8();
            bytes <= 24
        })
        .flat_map(|c| {
            let mut b = [0; 4];
            c.encode_utf8(&mut b)
                .as_bytes()
                .iter()
                .map(|b| format!("{b:02x}"))
                .collect::<Vec<_>>()
        })
        .collect::<String>()
}
fn unhex(s: &str) -> Option<String> {
    if s == "-" {
        return Some(String::new());
    }
    if s.len() > 48 || s.len() % 2 != 0 || !s.is_ascii() {
        return None;
    }
    let b = (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&s[i..i + 2], 16).ok())
        .collect::<Option<Vec<_>>>()?;
    String::from_utf8(b).ok()
}
fn finite(s: &str) -> Option<f32> {
    s.parse::<f32>()
        .ok()
        .filter(|x| x.is_finite() && x.abs() <= 9_999_999.0)
}

impl Message {
    pub fn encode(&self) -> String {
        match self {
            Self::Request => format!("{PREFIX} q"),
            Self::Grant(e) => format!("{PREFIX} g {e:x}"),
            Self::Revoke(e) => format!("{PREFIX} r {e:x}"),
            Self::Keep(e) => format!("{PREFIX} k {e:x}"),
            Self::Ack { epoch, seq, ok } => {
                format!("{PREFIX} a {epoch:x} {seq:x} {}", u8::from(*ok))
            }
            Self::Action(a) => format!(
                "{PREFIX} c {:x} {:x} {:x} {:x} {} {} {:x} {:x} {} {:.3} {:.3}",
                a.epoch,
                a.seq,
                a.target.text(),
                a.desk,
                a.kind,
                a.part,
                a.index,
                a.signature,
                a.phase.number(),
                a.x,
                a.y
            ),
            Self::Desk(d) => {
                let name = hex_name(&d.name);
                format!(
                    "{PREFIX} s {:x} {:x} {} {:.2} {:.2} {:.2} {:.2} {} {:.2}",
                    d.epoch,
                    d.id,
                    d.stage,
                    d.price.clamp(0.0, 9_999_999.0),
                    d.paid.clamp(0.0, 9_999_999.0),
                    d.due.clamp(0.0, 9_999_999.0),
                    d.given.clamp(0.0, 9_999_999.0),
                    if name.is_empty() { "-" } else { &name },
                    d.uncollected.clamp(0.0, 9_999_999.0)
                )
            }
        }
    }

    pub fn parse(text: &str) -> Option<Self> {
        // CMD's MAX_CHAT is 160 Unicode characters; all of our wire messages are ASCII.
        if text.len() > 160 || !text.is_ascii() {
            return None;
        }
        let v: Vec<_> = text.split_whitespace().collect();
        if v.first().copied() != Some(PREFIX) {
            return None;
        }
        let h = |i: usize| u64::from_str_radix(v.get(i)?, 16).ok();
        let n = |i: usize| v.get(i)?.parse::<u8>().ok();
        Some(match (v.get(1).copied()?, v.len()) {
            ("q", 2) => Self::Request,
            ("g", 3) => Self::Grant(h(2)?),
            ("r", 3) => Self::Revoke(h(2)?),
            ("k", 3) => Self::Keep(h(2)?),
            ("a", 5) => Self::Ack {
                epoch: h(2)?,
                seq: h(3)?,
                ok: match n(4)? {
                    0 => false,
                    1 => true,
                    _ => return None,
                },
            },
            ("c", 13) => {
                let a = Action {
                    epoch: h(2)?,
                    seq: h(3)?,
                    target: Target::parse(h(4)?),
                    desk: h(5)?.try_into().ok()?,
                    kind: n(6)?,
                    part: n(7)?,
                    index: h(8)?.try_into().ok()?,
                    signature: h(9)?,
                    phase: Phase::parse(n(10)?)?,
                    x: finite(v[11])?,
                    y: finite(v[12])?,
                };
                if a.seq == 0
                    || a.kind > 5
                    || a.part > 16
                    || a.x.abs() > 4096.0
                    || a.y.abs() > 4096.0
                {
                    return None;
                }
                Self::Action(a)
            }
            ("s", 10 | 11) => Self::Desk(Desk {
                epoch: h(2)?,
                id: h(3)?.try_into().ok()?,
                stage: n(4)?,
                price: finite(v[5])?,
                paid: finite(v[6])?,
                due: finite(v[7])?,
                given: finite(v[8])?,
                name: unhex(v[9])?,
                uncollected: if v.len() == 11 { finite(v[10])? } else { 0.0 },
            }),
            _ => return None,
        })
    }
}

/// One outstanding operation at a time: packet loss cannot reorder press/release or money.
pub struct Outbox {
    next: u64,
    queue: VecDeque<(u32, Action, Instant, Option<Instant>)>,
    last_emit: Option<Instant>,
}
impl Default for Outbox {
    fn default() -> Self {
        Self {
            next: 1,
            queue: VecDeque::new(),
            last_emit: None,
        }
    }
}
impl Outbox {
    pub fn push(&mut self, to: u32, mut a: Action, now: Instant) -> bool {
        // Coalesce unsent movement, never a packet already in flight or a money action.
        if a.phase == Phase::Drag && self.queue.len() > 1 {
            if let Some((last_to, last, _, sent)) =
                self.queue.back_mut().filter(|(id, last, _, sent)| {
                    *id == to
                        && last.phase == Phase::Drag
                        && last.epoch == a.epoch
                        && last.target == a.target
                        && last.kind == a.kind
                        && last.part == a.part
                        && last.index == a.index
                        && sent.is_none()
                })
            {
                let _ = (last_to, sent);
                if a.kind == 1 {
                    last.x = a.x;
                    last.y = a.y;
                } else {
                    last.x = (last.x + a.x).clamp(-4096.0, 4096.0);
                    last.y = (last.y + a.y).clamp(-4096.0, 4096.0);
                }
                return true;
            }
        }
        if self.queue.len() >= 32 || self.next == u64::MAX {
            return false;
        }
        a.seq = self.next;
        self.next += 1;
        self.queue.push_back((to, a, now, None));
        true
    }
    /// The kind of the in-flight command, checked before acknowledgement removes it.
    /// A late/repeated ACK does not retrigger a sound.
    pub fn pending_kind(&self, from: u32, epoch: u64, seq: u64) -> Option<u8> {
        self.queue.front().and_then(|(to, a, _, _)| {
            (*to == from && a.epoch == epoch && a.seq == seq).then_some(a.kind)
        })
    }
    pub fn acknowledge(&mut self, from: u32, epoch: u64, seq: u64) -> bool {
        if self
            .queue
            .front()
            .is_some_and(|(to, a, _, _)| *to == from && a.epoch == epoch && a.seq == seq)
        {
            self.queue.pop_front();
            true
        } else {
            false
        }
    }
    pub fn poll(&mut self, now: Instant) -> Result<Option<(u32, Message)>, ()> {
        let Some((to, a, born, sent)) = self.queue.front_mut() else {
            return Ok(None);
        };
        if now.saturating_duration_since(*born) > TIMEOUT {
            self.queue.clear();
            return Err(());
        }
        // Protocol 14 allows ten CMD messages/s. Leave room for keep-alives/plugins.
        if self
            .last_emit
            .is_some_and(|t| now.saturating_duration_since(t) < Duration::from_millis(130))
        {
            return Ok(None);
        }
        if sent.is_none_or(|s| now.saturating_duration_since(s) >= RETRY) {
            *sent = Some(now);
            self.last_emit = Some(now);
            return Ok(Some((*to, Message::Action(a.clone()))));
        }
        Ok(None)
    }
    pub fn clear(&mut self) {
        self.queue.clear();
    }
    pub fn is_empty(&self) -> bool {
        self.queue.is_empty()
    }
}

/// Keep the result as well as the sequence: a duplicate gets the original acknowledgement.
#[derive(Default)]
pub struct Receipt {
    last: u64,
    ok: bool,
}
impl Receipt {
    pub fn cached(&self, seq: u64) -> Option<bool> {
        (seq <= self.last).then_some(seq == self.last && self.ok)
    }
    pub fn record(&mut self, seq: u64, ok: bool) {
        if seq > self.last {
            self.last = seq;
            self.ok = ok;
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    fn action() -> Action {
        Action {
            epoch: 25,
            seq: 1,
            target: Target::Player,
            desk: 4,
            kind: 0,
            part: 0,
            index: 7,
            signature: u64::MAX,
            phase: Phase::Down,
            x: 0.0,
            y: 0.0,
        }
    }

    #[test]
    fn ack_for_close_signal_cannot_be_replayed() {
        let now = Instant::now();
        let mut box_ = Outbox::default();
        let mut a = action();
        a.kind = 4;
        a.phase = Phase::Tap;
        assert!(box_.push(7, a, now));
        assert_eq!(box_.pending_kind(7, 25, 1), Some(4));
        assert!(box_.acknowledge(7, 25, 1));
        assert_eq!(box_.pending_kind(7, 25, 1), None);
        assert!(!box_.acknowledge(7, 25, 1));
    }
    #[test]
    fn close_signal_is_conductor_only_and_not_desk_bound() {
        let mut a = action();
        a.kind = 4;
        a.phase = Phase::Tap;
        assert!(!desk_bound(a.kind, None));
        assert!(authorized(Some((2, 25)), 2, &a, true, false, false, 999));
        assert!(!authorized(None, 2, &a, true, false, false, 999));
        assert!(!authorized(Some((2, 25)), 3, &a, true, false, false, 999));
        assert!(!authorized(Some((2, 25)), 2, &a, false, false, false, 999));
        a.target = Target::Ai(1);
        assert!(!authorized(Some((2, 25)), 2, &a, true, false, false, 999));
        let encoded = Message::Action(a.clone()).encode();
        assert_eq!(Message::parse(&encoded), Some(Message::Action(a)));
    }
    #[test]
    fn money_requires_the_right_conductor_bus_epoch_and_passenger() {
        let mut a = action();
        a.epoch = 9;
        a.kind = 2;
        assert!(authorized(Some((2, 9)), 2, &a, true, false, true, 4));
        assert!(!authorized(Some((2, 9)), 3, &a, true, false, true, 4));
        assert!(!authorized(Some((2, 10)), 2, &a, true, false, true, 4));
        assert!(!authorized(Some((2, 9)), 2, &a, false, false, true, 4));
        assert!(!authorized(Some((2, 9)), 2, &a, true, false, true, 5));
        assert!(!authorized(None, 2, &a, true, false, true, 4));
        a.target = Target::Ai(42);
        assert!(!authorized(Some((2, 9)), 2, &a, true, false, true, 4));
        a.target = Target::Player;
        a.phase = Phase::Up;
        assert!(authorized(Some((2, 9)), 2, &a, true, false, true, 5));
        a.kind = 0;
        a.epoch = 0;
        a.target = Target::Ai(42);
        assert!(authorized(None, 2, &a, true, true, false, 0));
        assert!(!authorized(None, 2, &a, true, false, true, 0));
    }

    #[test]
    fn authorized_flaps_are_independent_of_cash_passenger_changes() {
        let a = action();
        let grant = Some((2, a.epoch));
        assert!(!desk_bound(0, Some("cp_portinha")));
        assert!(authorized(grant, 2, &a, true, false, false, a.desk + 1));
        assert!(!authorized(grant, 3, &a, true, false, false, a.desk));
        assert!(!authorized(grant, 2, &a, false, false, false, a.desk));
        for event in ["cashdesk_changer_1_00", "ticket_taste_1", "ibox_taste_2"] {
            assert!(desk_bound(0, Some(event)));
            assert!(!authorized(
                grant,
                2,
                &a,
                true,
                false,
                desk_bound(0, Some(event)),
                a.desk + 1
            ));
        }
        for kind in [1, 2, 3, 5] {
            assert!(desk_bound(kind, None));
        }
    }
    #[test]
    fn conductor_collect_payment_requires_current_authority_and_desk() {
        let mut a = action();
        a.kind = 5;
        a.phase = Phase::Tap;
        assert!(authorized(Some((2, a.epoch)), 2, &a, true, false, true, a.desk));
        assert!(!authorized(Some((2, a.epoch)), 3, &a, true, false, true, a.desk));
        assert!(!authorized(Some((2, a.epoch)), 2, &a, false, false, true, a.desk));
        assert!(!authorized(Some((2, a.epoch)), 2, &a, true, false, true, a.desk + 1));
    }

    #[test]
    fn wire_is_small_and_round_trips() {
        let mut d = Desk {
            epoch: u64::MAX,
            id: u32::MAX,
            stage: 7,
            price: 9999999.0,
            paid: 9999999.0,
            due: 9999999.0,
            given: 9999999.0,
            uncollected: 9999999.0,
            name: "Integração São Paulo — estudante".into(),
        };
        let parsed = Message::parse(&Message::Desk(d.clone()).encode()).unwrap();
        if let Message::Desk(x) = parsed {
            assert!(d.name.starts_with(&x.name));
            d.name = x.name;
        } else {
            panic!()
        }
        for m in [
            Message::Request,
            Message::Grant(9),
            Message::Revoke(9),
            Message::Keep(9),
            Message::Action(action()),
            Message::Ack {
                epoch: 2,
                seq: 4,
                ok: false,
            },
            Message::Desk(d),
        ] {
            let s = m.encode();
            assert!(s.len() <= 160, "{s}");
            assert_eq!(Message::parse(&s), Some(m));
        }
    }
    #[test]
    fn malformed_packets_do_not_become_actions() {
        for s in [
            "COOP1 c 0 0 0 0 0 0 0 0 0 0 0",
            "COOP1 c 0 1 0 0 0 0 0 0 0 NaN 0",
            "COOP1 c 0 1 0 0 6 0 0 0 0 0 0",
            "COOP1 a 0 1 2",
            "COOP1 s 0 0 1 1 1 1 1 ff",
            "COOP1 q unexpected",
        ] {
            assert!(Message::parse(s).is_none(), "{s}");
        }
    }
    #[test]
    fn lost_ack_does_not_repeat_an_operation_and_release_waits() {
        let now = Instant::now();
        let mut q = Outbox::default();
        let mut receipt = Receipt::default();
        q.push(8, action(), now);
        let mut up = action();
        up.phase = Phase::Up;
        q.push(8, up, now);
        let (to, Message::Action(a)) = q.poll(now).unwrap().unwrap() else {
            panic!()
        };
        assert_eq!(to, 8);
        assert_eq!(a.seq, 1);
        assert_eq!(receipt.cached(a.seq), None);
        receipt.record(a.seq, true);
        let (_, Message::Action(retry)) = q.poll(now + RETRY).unwrap().unwrap() else {
            panic!()
        };
        assert_eq!(retry.seq, 1);
        assert_eq!(receipt.cached(retry.seq), Some(true));
        assert!(!q.acknowledge(7, a.epoch, 1));
        assert!(!q.acknowledge(8, a.epoch + 1, 1));
        assert!(q.acknowledge(8, a.epoch, 1));
        let (_, Message::Action(up)) = q.poll(now + RETRY + RETRY).unwrap().unwrap() else {
            panic!()
        };
        assert_eq!(up.seq, 2);
        assert_eq!(up.phase, Phase::Up);
        assert_eq!(receipt.cached(1), Some(true));
        assert_eq!(receipt.cached(0), Some(false));
    }
    #[test]
    fn an_unresponsive_peer_does_not_build_an_infinite_queue() {
        let now = Instant::now();
        let mut q = Outbox::default();
        for _ in 0..32 {
            assert!(q.push(8, action(), now));
        }
        assert!(!q.push(8, action(), now));
        assert!(q.poll(now + TIMEOUT + RETRY).is_err());
        assert!(q.poll(now + TIMEOUT + RETRY).unwrap().is_none());
    }
    #[test]
    fn controls_leave_room_in_the_official_protocols_message_budget() {
        let now = Instant::now();
        let mut q = Outbox::default();
        q.push(8, action(), now);
        q.push(8, action(), now);
        let (_, Message::Action(a)) = q.poll(now).unwrap().unwrap() else {
            panic!()
        };
        assert!(q.acknowledge(8, a.epoch, a.seq));
        assert!(q.poll(now + Duration::from_millis(20)).unwrap().is_none());
        assert!(q.poll(now + Duration::from_millis(129)).unwrap().is_none());
        assert!(q.poll(now + Duration::from_millis(130)).unwrap().is_some());
    }
}
