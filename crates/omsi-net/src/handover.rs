//! Reliable, ordered, owner-authored handover states. Only names and paths, never audio.
use std::collections::{BTreeMap, HashMap, VecDeque};

// Protocol 15 reserves 0xBA/0xBB for traffic, 0xBC for voice and, since
// upstream 0.3.43, 0xBD for freetex. Coop handover must use its own byte.
pub const MAGIC: u8 = 0xBE;
const HEAD: usize = 36;
const CHUNK: usize = 1024;
const MAX_SIZE: usize = 512 * 1024;
const WINDOW: u64 = 4;

#[derive(Clone, Debug, PartialEq)]
pub struct Event {
    pub data: Vec<u8>,
}
#[derive(Debug)]
pub struct Incoming {
    pub owner: u32,
    pub event: Event,
}
#[derive(Clone, Copy)]
pub(crate) struct Address {
    pub from: u32,
    pub to: u32,
}
struct Packet<'a> {
    addr: Address,
    ack: bool,
    epoch: u64,
    seq: u64,
    part: usize,
    count: usize,
    size: usize,
    data: &'a [u8],
}
fn packet(data: &[u8], proto: u8) -> Option<Packet<'_>> {
    if data.len() < HEAD || data[0] != MAGIC || data[1] != proto || data[2] > 1 || data[3] != 0 {
        return None;
    }
    let p = Packet {
        addr: Address {
            from: u32::from_le_bytes(data[4..8].try_into().ok()?),
            to: u32::from_le_bytes(data[8..12].try_into().ok()?),
        },
        ack: data[2] == 1,
        epoch: u64::from_le_bytes(data[12..20].try_into().ok()?),
        seq: u64::from_le_bytes(data[20..28].try_into().ok()?),
        part: u16::from_le_bytes(data[28..30].try_into().ok()?) as usize,
        count: u16::from_le_bytes(data[30..32].try_into().ok()?) as usize,
        size: u32::from_le_bytes(data[32..36].try_into().ok()?) as usize,
        data: &data[HEAD..],
    };
    if p.seq == 0 || p.epoch == 0 || p.addr.from == 0 || p.addr.to == 0 {
        return None;
    }
    if p.ack {
        if data.len() != HEAD || p.part != 0 || p.count != 0 || p.size != 0 {
            return None;
        }
    } else if p.size == 0
        || p.size > MAX_SIZE
        || p.count != p.size.div_ceil(CHUNK)
        || p.part >= p.count
        || p.data.len() != (p.size - p.part * CHUNK).min(CHUNK)
    {
        return None;
    }
    Some(p)
}
pub(crate) fn address(data: &[u8], proto: u8) -> Option<Address> {
    Some(packet(data, proto)?.addr)
}
fn header(proto: u8, from: u32, to: u32, epoch: u64, seq: u64, ack: bool) -> Vec<u8> {
    let mut v = vec![MAGIC, proto, ack as u8, 0];
    v.extend(from.to_le_bytes());
    v.extend(to.to_le_bytes());
    v.extend(epoch.to_le_bytes());
    v.extend(seq.to_le_bytes());
    v.resize(HEAD, 0);
    v
}
fn encode(e: &Event) -> Option<Vec<u8>> {
    (!e.data.is_empty() && e.data.len() <= MAX_SIZE).then(|| e.data.clone())
}
fn decode(v: &[u8]) -> Option<Event> {
    (!v.is_empty() && v.len() <= MAX_SIZE).then(|| Event { data: v.to_vec() })
}
struct Tx {
    seq: u64,
    data: Vec<u8>,
    wait: f32,
}
#[derive(Default)]
struct Sender {
    next: u64,
    queue: VecDeque<Tx>,
}
struct Assembly {
    size: usize,
    parts: Vec<Option<Vec<u8>>>,
}
struct Receiver {
    epoch: u64,
    next: u64,
    pending: BTreeMap<u64, Assembly>,
}
pub(crate) struct Sync {
    epoch: u64,
    tx: HashMap<u32, Sender>,
    rx: HashMap<u32, Receiver>,
    incoming: Vec<Incoming>,
}
impl Default for Sync {
    fn default() -> Self {
        Self {
            epoch: crate::random_session_id().max(1),
            tx: HashMap::new(),
            rx: HashMap::new(),
            incoming: Vec::new(),
        }
    }
}
impl Sync {
    pub fn publish(&mut self, peers: &[u32], event: Event) {
        let Some(data) = encode(&event) else {
            log::warn!("LAN handover state exceeds protocol limits");
            return;
        };
        for &peer in peers {
            let s = self.tx.entry(peer).or_default();
            if s.queue.len() >= 8 {
                log::warn!("LAN handover states awaiting acknowledgement from {peer}: queue full");
                continue;
            }
            s.next += 1;
            s.queue.push_back(Tx {
                seq: s.next,
                data: data.clone(),
                wait: 0.0,
            });
        }
    }
    pub fn retain_peers(&mut self, peers: &[u32]) {
        self.tx.retain(|id, _| peers.contains(id));
        self.rx.retain(|id, _| peers.contains(id));
        self.incoming.retain(|s| peers.contains(&s.owner));
    }
    pub fn tick(&mut self, proto: u8, me: u32, dt: f32) -> Vec<Vec<u8>> {
        let mut out = Vec::new();
        for (&peer, s) in &mut self.tx {
            let mut budget = 512;
            let first = s.queue.front().map(|t| t.seq).unwrap_or(0);
            for t in &mut s.queue {
                if t.seq >= first + WINDOW || budget < t.data.len().div_ceil(CHUNK) {
                    break;
                }
                t.wait -= dt.max(0.0);
                if t.wait > 0.0 {
                    continue;
                }
                t.wait = 0.25;
                budget -= t.data.len().div_ceil(CHUNK);
                for (part, data) in t.data.chunks(CHUNK).enumerate() {
                    let mut v = header(proto, me, peer, self.epoch, t.seq, false);
                    v[28..30].copy_from_slice(&(part as u16).to_le_bytes());
                    v[30..32].copy_from_slice(&(t.data.len().div_ceil(CHUNK) as u16).to_le_bytes());
                    v[32..36].copy_from_slice(&(t.data.len() as u32).to_le_bytes());
                    v.extend(data);
                    out.push(v);
                }
            }
        }
        out
    }
    pub fn receive(&mut self, proto: u8, me: u32, data: &[u8]) -> Option<Vec<u8>> {
        let p = packet(data, proto)?;
        if p.addr.to != me || p.addr.from == me {
            return None;
        }
        if p.ack {
            if p.epoch == self.epoch {
                if let Some(s) = self.tx.get_mut(&p.addr.from) {
                    s.queue.retain(|t| t.seq != p.seq);
                }
            }
            return None;
        }
        let r = self.rx.entry(p.addr.from).or_insert_with(|| Receiver {
            epoch: p.epoch,
            next: 1,
            pending: BTreeMap::new(),
        });
        // A peer's stream lives for the session. Old epochs cannot reset deduplication.
        if r.epoch != p.epoch || p.seq >= r.next + WINDOW {
            return None;
        }
        if p.seq >= r.next {
            if self.incoming.len() >= 16 {
                return None;
            }
            let a = r.pending.entry(p.seq).or_insert_with(|| Assembly {
                size: p.size,
                parts: vec![None; p.count],
            });
            if a.size != p.size {
                return None;
            }
            a.parts[p.part].get_or_insert_with(|| p.data.to_vec());
            if a.parts.iter().any(Option::is_none) {
                return None;
            }
            let bytes: Vec<u8> = a
                .parts
                .iter()
                .flat_map(|v| v.as_ref().unwrap().iter().copied())
                .collect();
            decode(&bytes)?;
            drain_ready(p.addr.from, r, &mut self.incoming);
        }
        Some(header(proto, me, p.addr.from, p.epoch, p.seq, true))
    }
    pub fn take_incoming(&mut self) -> Vec<Incoming> {
        for (&owner, r) in &mut self.rx {
            drain_ready(owner, r, &mut self.incoming);
        }
        std::mem::take(&mut self.incoming)
    }
}

fn drain_ready(owner: u32, r: &mut Receiver, out: &mut Vec<Incoming>) {
    while out.len() < 16 {
        let Some(a) = r.pending.get(&r.next) else {
            break;
        };
        if a.parts.iter().any(Option::is_none) {
            break;
        }
        let bytes: Vec<u8> = a
            .parts
            .iter()
            .flat_map(|v| v.as_ref().unwrap().iter().copied())
            .collect();
        let Some(event) = decode(&bytes) else {
            break;
        };
        out.push(Incoming { owner, event });
        r.pending.remove(&r.next);
        r.next += 1;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn loss_reorder_and_lost_ack_never_activate_partial_state() {
        let mut a = Sync::default();
        let mut b = Sync::default();
        let e = Event {
            data: (0..200000).map(|i| (i % 251) as u8).collect(),
        };
        a.publish(&[2], e.clone());
        let packets = a.tick(14, 1, 0.0);
        for p in packets.iter().skip(1).rev() {
            b.receive(14, 2, p);
        }
        assert!(b.take_incoming().is_empty());
        for p in a.tick(14, 1, 0.3).iter().rev() {
            b.receive(14, 2, p);
        }
        assert_eq!(b.take_incoming().remove(0).event, e);
        for p in a.tick(14, 1, 0.3) {
            let ack = b.receive(14, 2, &p).unwrap();
            a.receive(14, 1, &ack);
        }
        assert!(b.take_incoming().is_empty());
        assert!(a.tick(14, 1, 1.0).is_empty());
    }
    #[test]
    fn recipient_protocol_and_bounds_are_checked() {
        let mut a = Sync::default();
        let mut b = Sync::default();
        a.publish(
            &[2],
            Event {
                data: vec![1, 2, 3],
            },
        );
        let p = a.tick(14, 1, 0.0).remove(0);
        assert!(b.receive(15, 2, &p).is_none());
        assert!(b.receive(14, 3, &p).is_none());
        for n in 0..p.len() {
            assert!(b.receive(14, 2, &p[..n]).is_none());
        }
        assert!(b.take_incoming().is_empty());
        assert!(b.receive(14, 2, &p).is_some());
    }
}
