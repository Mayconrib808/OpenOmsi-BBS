"""Keep both the bus and the human in every multiplayer population region."""
from pathlib import Path

game = Path(__file__).resolve().parents[3] / 'game'

def edit(relative, old, new, count=1):
    path = game / relative
    text = path.read_text(encoding='utf-8')
    assert text.count(old) == count, f'Unexpected population source: {relative}'
    path.write_text(text.replace(old, new), encoding='utf-8', newline='\n')

edit('crates/omsi-net/src/lib.rs', 'impl Pose {', '''impl Pose {
    /// Live world interest includes the parked bus AND its walking owner.
    /// A heartbeat without either must not populate the map origin.
    pub fn world_interest_centers(&self) -> impl Iterator<Item = [f64; 3]> {
        self.has_vehicle().then_some([self.x, self.y, self.z]).into_iter()
            .chain(self.walker.map(|w| [w.x, w.y, w.z]))
            .filter(|p| p.iter().all(|v| v.is_finite()))
    }
''')

tests = game / 'crates/omsi-net/src/tests.rs'
with tests.open('a', encoding='utf-8') as f:
    f.write('''
#[test]
fn world_interest_keeps_walker_and_bus_for_dedicated_and_player_hosts() {
    let mut p = pose(1000.0);
    assert_eq!(p.world_interest_centers().collect::<Vec<_>>(), vec![[1000.0,2.0,3.0]]);
    p.walker = Some(Walker { x:8000.0, y:9000.0, z:4.0, heading:0.0,
        speed:0.0, course:0.0, seated:false, aboard:None });
    assert_eq!(p.world_interest_centers().collect::<Vec<_>>(),
        vec![[1000.0,2.0,3.0], [8000.0,9000.0,4.0]]);
    p.bus.clear(); p.flags &= !FLAG_VEHICLE;
    assert_eq!(p.world_interest_centers().collect::<Vec<_>>(), vec![[8000.0,9000.0,4.0]]);
    p.walker.as_mut().unwrap().x = 12000.0;
    assert_eq!(p.world_interest_centers().next(), Some([12000.0,9000.0,4.0]));
    p.walker.as_mut().unwrap().x = f64::NAN;
    assert_eq!(p.world_interest_centers().count(), 0);
    p.walker = None;
    assert_eq!(p.world_interest_centers().count(), 0);
}
''')

# The dedicated 0.3.39 source has not received the Coop 10 LAN filter yet.
world = game / 'crates/omsi-app/src/lan_world.rs'
text = world.read_text(encoding='utf-8')
text = text.replace('.filter(|p| p.has_pose && p.pose.has_vehicle())',
    '.filter(|p| p.has_pose && (p.pose.has_vehicle() || p.pose.walker.is_some()))')
text = text.replace('let at = DVec3::new(p.pose.x, p.pose.y, p.pose.z);',
    'let at = p.pose.walker.map(|w| DVec3::new(w.x,w.y,w.z)).unwrap_or_else(|| DVec3::new(p.pose.x,p.pose.y,p.pose.z));')
world.write_text(text, encoding='utf-8', newline='\n')

edit('crates/omsi-app/src/lan_world.rs',
    'let centers: Vec<DVec3> = players.iter().map(|p| p.1).collect();',
    '''let centers: Vec<DVec3> = lan.peers().filter(|p| p.has_pose)
            .flat_map(|p| p.pose.world_interest_centers().map(DVec3::from_array)).collect();''')

edit('crates/omsi-app/src/app.rs',
    'centers.extend(cx.net.remotes.remotes.values().map(|r| r.vehicle().position));',
    '''centers.extend(cx.net.remotes.remotes.values().map(|r| r.vehicle().position));
        if let Some(lan) = cx.net.lan.as_ref() {
            centers.extend(lan.peers().filter(|p| p.has_pose)
                .flat_map(|p| p.pose.world_interest_centers().map(DVec3::from_array)));
        }''')

edit('crates/omsi-app/src/offscreen/step.rs',
    '''            ref run_clock,
            ref remotes_off,
            ref mut player,''',
    '''            ref run_clock,
            ref remotes_off,
            ref lan_off,
            ref mut player,''')

edit('crates/omsi-app/src/offscreen/step.rs',
    '''                    .map(|r| r.vehicle().position)
                    .chain(player.as_ref().map(|p| p.vehicle.position))''',
    '''                    .map(|r| r.vehicle().position)
                    .chain(lan_off.iter().flat_map(|lan| lan.peers())
                        .filter(|p| p.has_pose)
                        .flat_map(|p| p.pose.world_interest_centers().map(DVec3::from_array)))
                    .chain(player.as_ref().map(|p| p.vehicle.position))''')

edit('crates/omsi-app/src/offscreen/step.rs',
    '''.map(|p| DVec3::new(p.pose.x, p.pose.y, p.pose.z)));''',
    '''.flat_map(|p| p.pose.world_interest_centers().map(DVec3::from_array)));''')

print('World streaming and population follow all buses and walkers on both host modes.')
