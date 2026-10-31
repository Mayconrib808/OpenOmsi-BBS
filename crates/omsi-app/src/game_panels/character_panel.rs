//! Character selection is available to every cooperative role.
use super::{button,grow_row,input,title};
use crate::coop::text;
use crate::coop_character::catalog;
use omsi_plugin::ui::{Anchor,Panel};

const PER_PAGE: usize=6;

pub(crate) fn panel(app:&crate::App)->Panel {
    let light = app.settings.is_light_theme();
    let note = |text: &str| super::note(text, light);
    let card = |anchor, x, y, width, alpha, children| super::card(anchor, x, y, width, alpha, light, children);

    let state=&app.net.coop.character;
    let mut children=vec![
        title(text("Personagem / skin","Character / skin","Spielfigur / Skin")),
        note(text("Sua escolha vale como motorista, trocador e passageiro, e fica salva.",
            "Your choice is saved and used as driver, conductor and passenger.",
            "Die Auswahl wird für Fahrer, Kassierer und Fahrgast gespeichert.")),
        input("character_search",&state.query,text("Buscar personagem instalado","Search installed characters","Installierte Figur suchen"),80),
        button("character_default",text("Usar personagem padrão do mapa","Use the map's default character","Standardfigur der Karte verwenden"),None),
    ];
    let choices:Vec<_>=state.choices.iter().enumerate().filter(|(_,c)|catalog::matches(c,&state.query)).collect();
    let pages=choices.len().div_ceil(PER_PAGE).max(1);
    let page=state.page.min(pages-1);
    for (index,choice) in choices.iter().skip(page*PER_PAGE).take(PER_PAGE) {
        let active=choice.file.eq_ignore_ascii_case(&state.chosen);
        children.push(button(&format!("character_pick_{index}"),
            &format!("{}{}",if active {"✓ "}else{""},choice.label),None));
    }
    if choices.is_empty() {
        children.push(note(text("Nenhum .hum encontrado em Humans. Instale personagens no seu OMSI 2 e atualize a lista.",
            "No matching .hum found under Humans. Install characters in OMSI 2 and refresh.",
            "Keine passende .hum-Datei unter Humans. Figuren in OMSI 2 installieren und aktualisieren.")));
    }
    children.push(note(&format!("{} / {} · {}",page+1,pages,choices.len())));
    if pages>1 {children.push(grow_row(vec![
        button("character_prev",text("Anterior","Previous","Zurück"),None),
        button("character_next",text("Próxima","Next","Weiter"),None),
    ]));}
    let current=if state.chosen.is_empty() {text("Padrão do mapa","Map default","Kartenstandard")}else{&state.chosen};
    children.push(note(&format!("{}: {current}",text("Selecionado","Selected","Ausgewählt"))));
    children.push(note(text("Para os amigos verem a mesma skin, ela precisa estar instalada no computador deles.",
        "Friends need the same character installed to see your selected skin.",
        "Freunde benötigen dieselbe installierte Figur, um deinen Skin zu sehen.")));
    if !state.error.is_empty(){children.push(note(&state.error));}
    children.push(grow_row(vec![
        button("character_refresh",text("Atualizar lista","Refresh list","Liste aktualisieren"),None),
        button("character_back",text("Voltar","Back","Zurück"),None),
    ]));
    card(Anchor::TopRight,20.0,20.0,460.0,240,children)
}

pub(crate) fn click(app:&mut crate::App,id:&str)->bool {
    if let Some(index)=id.strip_prefix("character_pick_").and_then(|s|s.parse::<usize>().ok()) {
        let choice=app.net.coop.character.choices.get(index).map(|c|c.file.clone());
        if let Some(choice)=choice{app.character_choose(&choice);}
        return true;
    }
    match id {
        "appearance"|"character_refresh"=>app.character_open(),
        "character_default"=>app.character_choose(""),
        "character_prev"=>app.net.coop.character.page=app.net.coop.character.page.saturating_sub(1),
        "character_next"=>{
            let state=&mut app.net.coop.character;
            let count=state.choices.iter().filter(|c|catalog::matches(c,&state.query)).count();
            state.page=(state.page+1).min(count.div_ceil(PER_PAGE).saturating_sub(1));
        },
        "character_back"=>app.net.coop.character.open=false,
        _=>return false,
    }
    true
}

pub(crate) fn change(app:&mut crate::App,id:&str,value:&omsi_plugin::api::Value) {
    let omsi_plugin::api::Value::Str(value)=value else {return};
    if id=="character_search" {
        app.net.coop.character.query=value.to_string();
        app.net.coop.character.page=0;
    }
}

