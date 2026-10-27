"""Complete the validated 0.3.45 layer without changing its preserved baseline."""
from pathlib import Path
import sys
root=Path(sys.argv[1] if len(sys.argv)>1 else "game")
def edit(name,old,new):
 p=root/name;s=p.read_text(encoding="utf-8")
 assert s.count(old)==1,("Follow-up source anchor changed",name,old[:80])
 p.write_text(s.replace(old,new,1),encoding="utf-8")

edit("crates/omsi-sim/src/human/gesture.rs",
"""    if amount <= 0.0001 { return; }
    let side = side.min(1);""",
"""    if amount <= 0.0001 { return; }
    // The IK owns the phase, with zero velocity at either end. This is a rig
    // choice, not a measured duration from the photographic references.
    let amount = amount * amount * (3.0 - 2.0 * amount);
    let side = side.min(1);""")
edit("crates/omsi-sim/src/human/gesture.rs",
"""    let old_pole = old_el - (sh + old_wr) * 0.5;
    let pole = old_pole.lerp(-Vec3::Z + right * (SIDE[side] * 0.15), amount);""",
"""    // Carry the elbow plane with the reach direction. A raw pole blend could
    // pass close to that axis and make the elbow/wrist turn abruptly.
    let axis=(target-sh).normalize_or(-Vec3::Z);
    let old_axis=(old_wr-sh).normalize_or(-Vec3::Z);
    let carried=Quat::from_rotation_arc(old_axis,axis)*(old_el-(sh+old_wr)*0.5);
    let outward=right*SIDE[side];
    let project=|v:Vec3|(v-axis*v.dot(axis)).normalize_or(outward);
    let from_pole=project(carried);
    let to_pole=project(-Vec3::Z+outward*0.15);
    let turn=axis.dot(from_pole.cross(to_pole)).atan2(from_pole.dot(to_pole));
    let pole=Quat::from_axis_angle(axis,turn*amount)*from_pole;""")
edit("crates/omsi-app/src/view_sync/people/extras.rs",
""" let ease=self.hail*self.hail*(3.-2.*self.hail);
 // Boarding request""",
""" let ease=self.hail; // The arm IK owns its phase; do not ease it twice.
 // Boarding request""")

edit("crates/omsi-lan/src/vehicle_transfer.rs","pub fn encode(m: &Message)",
"""// Status metadata only; simulation authority still needs token-bound handover.
// The logical owner is always the transport sender, never a supplied field.
pub fn authority_notice(token:u64,driver:u32,active:bool)->String {
 format!("COOPAUTH1 {token:x} {driver} {}",u8::from(active))
}
pub fn parse_authority_notice(text:&str)->Option<(u64,u32,bool)> {
 let mut p=text.strip_prefix("COOPAUTH1 ")?.split_whitespace();
 let token=u64::from_str_radix(p.next()?,16).ok()?;
 let driver=p.next()?.parse().ok()?;
 let active=match p.next()? {"1"=>true,"0"=>false,_=>return None};
 (p.next().is_none()&&token!=0&&driver!=0).then_some((token,driver,active))
}
pub fn encode(m: &Message)""")
edit("crates/omsi-lan/src/vehicle_transfer.rs","    fn bus() -> VehicleInstance {",
"""    #[test]
    fn authority_notice_has_no_claimed_owner_or_simulation_input() {
        let text=authority_notice(71,3,true);
        assert_eq!(parse_authority_notice(&text),Some((71,3,true)));
        for bad in ["COOPAUTH1 0 3 1","COOPAUTH1 47 0 1","COOPAUTH1 47 3 2",
            "COOPAUTH1 47 3 1 extra","COOPAUTH1 47 3 1 speed 99"] {
            assert!(parse_authority_notice(bad).is_none());
        }
    }
    fn bus() -> VehicleInstance {""")
edit("crates/omsi-lan/src/game.rs","    controllers: hashbrown::HashMap<u32, u32>,",
"""    controllers: hashbrown::HashMap<u32, u32>,
    reported_controllers: hashbrown::HashMap<u32,(u32,u64,Instant)>,""")
edit("crates/omsi-lan/src/game.rs","/// What the frame knows that LAN play needs, filled by the game each frame.",
"""// A dedicated server has no App/DriveState, but must show the current driver.
// This authenticated sender-owned assignment grants no vehicle control.
pub fn note_driver_authority(game:&mut LanGame,lan:&LanSession,from:u32,text:&str)->bool {
 if !text.starts_with("COOPAUTH1 ") {return false;}
 let Some((token,driver,active))=crate::vehicle_transfer::parse_authority_notice(text) else{return true;};
 if from==driver||!lan.peers().any(|p|p.pose.id==from)
    ||!(driver==lan.my_id||lan.peers().any(|p|p.pose.id==driver)){return true;}
 if active {game.reported_controllers.insert(from,(driver,token,Instant::now()));}
 else if game.reported_controllers.get(&from).is_some_and(|(_,t,_)|*t==token){
    game.reported_controllers.remove(&from);
 }
 true
}

/// What the frame knows that LAN play needs, filled by the game each frame.""")
edit("crates/omsi-lan/src/game.rs","    game.controllers = frame.controllers.iter().copied().collect();",
"""    game.reported_controllers.retain(|owner,(driver,_,t)|t.elapsed()<Duration::from_secs(5)
        &&lan.peers().any(|p|p.pose.id==*owner)
        &&(*driver==lan.my_id||lan.peers().any(|p|p.pose.id==*driver)));
    game.controllers=game.reported_controllers.iter().map(|(owner,(driver,_,_))|(*owner,*driver))
        .chain(frame.controllers.iter().copied()).collect();""")
edit("crates/omsi-app/src/coop_drive.rs",
"        for peer in peers { self.drive_send(peer,Message::Authority { token,driver,active }); }",
"""        for peer in peers {
            self.drive_send(peer,Message::Authority {token,driver,active});
            if let Some(lan)=self.net.lan.as_mut(){
                lan.command(peer,&omsi_lan::vehicle_transfer::authority_notice(token,driver,active));
            }
        }""")
edit("crates/omsi-app/src/coop_drive.rs","    pub(crate) fn drive_requested(&self, p: u32) -> bool {",
"""    pub(crate) fn drive_menu_allowed(&self,id:&str)->bool {
        if self.net.coop.drive.busy()&&matches!(id,"place"|"swap"|"switch"|"remove"|"reload"|"clearplaced"|"couple"|"uncouple"){return false;}
        !(self.net.coop.drive.owner_active()&&matches!(id,"reset"|"teleport"|"dest"|"duty"|"repair"|"refuel"|"wash"|"vehicle"|"mirror_hud_edit"|"tobus"))
    }
    pub(crate) fn drive_requested(&self, p: u32) -> bool {""")
edit("crates/omsi-app/src/coop_drive.rs","    #[test]\n    fn coop13_passive_motion_smooths_chassis_without_advancing_simulation()",
"""    #[test]
    fn coop13_former_driver_menus_cannot_reset_remove_or_reclaim_the_bus() {
        let args=Args::parse_from(["openomsi","--root","."]);
        let mut app=crate::assemble_app(args,Default::default());
        let player=crate::spawn::coop13_test_player(bus());let uid=player.uid;app.player=Some(player);
        let now=Instant::now();app.net.coop.drive.owner=Some(Owner{
            peer:2,token:11,uid,bus:"Vehicles/test.bus".into(),frozen:true,active:true,
            seen:now,sent:now,triggers:vec![],motion:None,
        });
        for id in ["remove","reset","teleport","refuel","swap","tobus"]{
            assert!(!app.drive_menu_allowed(id),"former driver retained {id}");
            assert!(app.page_action(id));assert!(app.player.is_some());
        }
        for id in ["camera","coop","options","getout","map"]{assert!(app.drive_menu_allowed(id));}
    }
    #[test]
    fn coop13_passive_motion_smooths_chassis_without_advancing_simulation()""")
edit("crates/omsi-app/src/app_impl/menus.rs","    pub(crate) fn page_action(&mut self, id: &str) -> bool {",
"""    pub(crate) fn page_action(&mut self, id: &str) -> bool {
        if !self.drive_menu_allowed(id){
            self.coop_notice("Este comando depende da direção. Para trocar ou remover o veículo, devolva a direção primeiro.",
                "This command requires vehicle authority. Return the wheel before replacing or removing the vehicle.",
                "Dieser Befehl benötigt Fahrzeugkontrolle. Vor Fahrzeugwechsel oder Entfernen die Lenkung zurückgeben.");
            return true;
        }""")
edit("crates/omsi-app/src/app_impl/menus.rs",
"""        v.splice(before..before, lines.into_iter().map(|m| ("plugin", Cow::Owned(m.label.clone()), Some(m))));
        v""",
"""        v.splice(before..before, lines.into_iter().map(|m| ("plugin", Cow::Owned(m.label.clone()), Some(m))));
        v.retain(|line|self.drive_menu_allowed(line.0));
        v""")
edit("crates/omsi-app/src/game_panels/quick_menu.rs",
"        let off = MENU_LINES.iter().copied().filter(|id| !offered.contains(id)).collect();",
"""        let mut off:Vec<_>=MENU_LINES.iter().copied().filter(|id|!offered.contains(id)).collect();
        off.extend(TILES.iter().filter(|(id,_,_)|!app.drive_menu_allowed(id)).map(|(id,_,_)|*id));""")
edit("crates/omsi-app/src/game_panels/coop_panel.rs",
"""        app.session.on_foot.is_none()
        || app.coop_passengers()""",
"""        app.session.on_foot.is_none()
        || app.net.coop.drive.visitor_active()
        || app.coop_passengers()""")
edit("crates/omsi-app/src/app_impl/lan_tick.rs",
"""        use omsi_lan::commands::Command;
        if self.drive_receive""",
"""        use omsi_lan::commands::Command;
        if let Some(lan)=self.net.lan.as_ref(){
            if lan::note_driver_authority(&mut self.net.remotes,lan,from,text){return;}
        }
        if self.drive_receive""")
edit("crates/omsi-app/src/offscreen/step.rs","            ref remotes_off,\n","            ref mut remotes_off,\n")
edit("crates/omsi-app/src/offscreen/step.rs",
"            let positions = |id: u32| remotes_off.remotes.get(&id).map(|r| (r.vehicle().position, r.vehicle().heading));",
"""            let mut commands=l.take_commands();
            commands.retain(|(from,text)|!lan::note_driver_authority(remotes_off,l,*from,text));
            let positions = |id: u32| remotes_off.remotes.get(&id).map(|r| (r.vehicle().position, r.vehicle().heading));""")
edit("crates/omsi-app/src/offscreen/step.rs","            for (from, text) in l.take_commands() {",
"            for (from, text) in commands {")
edit("crates/omsi-plugin/src/lib.rs",
"""            .map(PathBuf::from)
            .or_else(|| exe_dir.map(|d| d.join("omsi-plugin-host32.exe")))
            .filter(|p| p.is_file());""",
"""            .map(PathBuf::from)
            .filter(|p| p.is_file())
            .or_else(|| exe_dir.map(|d| d.join("omsi-plugin-host32.exe")).filter(|p| p.is_file()));""")
edit("launcher/ui/js/pages/settings-schema.js",
"key: 'passenger_transfer_percent', label: 'Chance de fazer conexão', min: 0, max: 100, step: 1, unit: '%',",
"key: 'passenger_transfer_percent', label: 'Chance de fazer conexão', min: 0, max: 100, step: 1, def: 20, store: round, format: (v) => Math.round(v)+'%',")
print("Smooth hailing transition, current-driver menus and dedicated status assignment applied.")
