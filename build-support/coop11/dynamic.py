"""Apply after the cumulative Coop10/greetings hotfix, to either pinned runtime."""
from pathlib import Path
root = Path(__file__).resolve().parents[3]
game = root / 'game'
def edit(name, old, new):
    path = game / name
    s = path.read_text()
    assert s.count(old) == 1, name
    path.write_text(s.replace(old, new), newline='\n')
edit('crates/omsi-app/src/offscreen/setup.rs',
     'let dynamic_server = args.server.is_some() && args.radius == Some(99);',
     '''// A saved radius, --all or a legacy launcher must never disable streaming.
        let dynamic_server = args.server.is_some();''')
edit('crates/omsi-app/src/offscreen/setup.rs',
     'if let Some(s) = schedule.as_mut() {',
     '''// Keep the timetable and map index global, but prepare vehicles only when
        // their region is used. A headless host needs no all-depot texture warmup.
        if let Some(s) = schedule.as_mut().filter(|_| !dynamic_server) {''')
edit('crates/omsi-app/src/schedule/fleet.rs',
     '''    ) {
        let ready = self.fleet_ready.lock().pop();''',
     '''    ) {
        // The dedicated server has no displayed fleet. Its traffic creates the
        // actual vehicles in live interest regions; do not decode every paint.
        if crate::server::SERVER_MODE.load(std::sync::atomic::Ordering::Relaxed) {
            return;
        }
        let ready = self.fleet_ready.lock().pop();''')
print('Dedicated streaming is independent of saved radius; no global fleet texture prewarm.')

with (game / "crates/omsi-app/src/offscreen/setup.rs").open("a") as f:
    f.write((Path(__file__).parent / "streaming-test.rs").read_text())
