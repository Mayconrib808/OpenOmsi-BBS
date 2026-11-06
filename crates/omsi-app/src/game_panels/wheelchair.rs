use super::{button,card,note,title};
use omsi_plugin::ui::{Anchor,Panel};
use omsi_ui::{tr,theme::Palette};
use omsi_sim::people::wheelchair::{self,Equipment};

pub(crate) fn panel(app:&crate::App,pal:Palette)->Panel {
    let mut rows=vec![title(&tr("Wheelchair assistance"))];
    if let Some(h)=app.session.humans.as_ref() {
        let wc=&h.wheelchairs;
        rows.push(note(&format!("{}: {} / {}",tr("Occupied wheelchair bays"),wc.riders.len()+usize::from(wc.journey.as_ref().is_some_and(|j|j.boarded)),wc.profiles.len()),pal));
        if let Some(p)=wc.profile.as_ref() {
            rows.push(note(&format!("{} · {}",p.name,tr(if p.side()>0.0{"Right-side accessible door"}else{"Left-side accessible door"})),pal));
            rows.push(note(&tr(if matches!(p.equipment,Equipment::NativeRamp|Equipment::NativeLift){
                "Original equipment: passengers board when the door and ramp are ready"
            }else{"Auxiliary lift: stop, open the accessible door and start assistance"}),pal));
            if wc.wants_alight() {
                rows.push(note(&tr("A wheelchair passenger wants to alight at the next stop"),pal));
            }
            if let Some(j)=wc.journey.as_ref() {
                if !(j.alight_requested&&j.stage==wheelchair::Stage::Riding) {
                    rows.push(note(&tr(j.stage.ui_label()),pal));
                }
                if !j.hold_reason.is_empty(){rows.push(note(&tr(hold_label(j.hold_reason)),pal));}
                if j.stage==wheelchair::Stage::Waiting {
                    rows.push(button("assist",&tr("Prepare boarding"),Some("accessible")));
                }else if j.stage==wheelchair::Stage::Riding&&j.alight_requested {
                    rows.push(button("assist",&tr("Prepare alighting"),Some("accessible")));
                }
            }else{
                if wc.wants_alight() {
                    rows.push(button("assist",&tr("Prepare alighting"),Some("accessible")));
                }else{
                    rows.push(note(&tr("Waiting for an approaching passenger or an alighting request"),pal));
                }
            }
            rows.push(note(&tr("Keep the bus stopped until the wheelchair is secured and the equipment is stowed"),pal));
        }else{rows.push(note(&tr("No verified accessible door and wheelchair bay were found for this vehicle"),pal));}
    }
    rows.push(button("close",&tr("Close"),Some("close")));
    card(Anchor::BottomRight,32.0,32.0,460.0,245,pal,rows)
}
fn hold_label(reason:&str)->&'static str {
    match reason {
        "Passagem ocupada: aguardando espaço"|"Passagem ocupada: solicitando espaço aos passageiros"|"Área reservada ocupada: aguardando liberação"=>"Waiting for space",
        "Aguardando ônibus completamente parado"=>"Stop the bus completely before starting assistance",
        "Aguardando abertura da porta acessível"|"Pare completamente e abra a porta acessível"=>"Open the accessible door",
        "Aguardando recolhimento do banco retrátil"=>"Waiting for the folding seat",
        "Aguardando recolhimento real do equipamento"|"Aguardando recolhimento do equipamento original"=>"Waiting for the equipment to be stowed",
        "Reposicione a porta acessível perto do passageiro"=>"Move the accessible door closer to the passenger",
        "Aguardando superfície de desembarque carregada e ao alcance"|"Desnível excessivo para esta rampa: reposicione o ônibus"=>"Reposition the bus near a safe pavement",
        _=>"Waiting for the door and equipment",
    }
}
pub(crate) fn click(app:&mut crate::App,id:&str) {
    match id {
        "close"=>app.menus.game_panels.wheelchair=false,
        "assist"=>{
            if app.drive_menu_allowed("wheelchair")&&app.player.as_ref().is_some_and(|p|p.vehicle.physics.speed.abs()<0.025) {
                wheelchair::command(wheelchair::ASSIST);
            }else{app.service_msg=Some((tr("Stop the bus completely before starting assistance").into_owned(),5.0));}
        }
        _=>{}
    }
}
