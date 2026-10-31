//! The driver grants one passenger permission; the passenger chooses their own bus view.
use super::{button, grow_row, title};
use crate::coop::text;
use omsi_plugin::ui::{Anchor, Panel};

pub(crate) fn status(app: &crate::App) -> Option<Panel> {
    let light = app.settings.is_light_theme();
    let note = |text: &str| super::note(text, light);
    let card = |anchor, x, y, width, alpha, children| super::card(anchor, x, y, width, alpha, light, children);

    if !app.net.coop.remote.is_some_and(|(_, e, _)| e != 0)
        || (app.net.coop.desk.stage == 0 && app.net.coop.desk.uncollected <= 0.0)
        || app.net.coop.panel
    {
        return None;
    }
    let d = &app.net.coop.desk;
    let mut children = vec![
        title(if d.name.is_empty() {
            text("Dinheiro no caixa", "Money on desk", "Geld in der Kasse")
        } else {
            &d.name
        }),
        note(&format!(
            "{} {:.2}  ·  {} {:.2}",
            text("Preço", "Price", "Preis"),
            d.price,
            text("Pagamento", "Payment", "Zahlung"),
            d.paid
        )),
        note(&format!(
            "{} {:.2} / {:.2}",
            text("Troco entregue", "Change given", "Gegebenes Rückgeld"),
            d.given,
            d.due
        )),
    ];
    if d.uncollected > 0.0 {
        children.push(note(&format!(
            "{} {:.2}",
            text("Dinheiro a recolher:", "Payment to collect:", "Geld einzusammeln:"),
            d.uncollected
        )));
        // Status is intentionally passive: it does not own mouse focus while driving.
        // The actual button stays in the Cooperative panel, and Shift+T works on foot.
        children.push(note(text(
            "Pressione Shift+T para recolher.",
            "Press Shift+T to collect.",
            "Shift+T drücken, um das Geld zu nehmen.",
        )));
    }
    Some(card(Anchor::TopLeft, 20.0, 80.0, 360.0, 250, children))
}

pub(crate) fn panel(app: &crate::App) -> Panel {
    let light = app.settings.is_light_theme();
    let note = |text: &str| super::note(text, light);
    let card = |anchor, x, y, width, alpha, children| super::card(anchor, x, y, width, alpha, light, children);

    if app.net.coop.character.open { return super::character_panel::panel(app); }
    let mut children = vec![
        title(text("Cooperativo", "Cooperative", "Kooperativ")),
        button("appearance", text("Escolher personagem / skin", "Choose character / skin", "Spielfigur / Skin wählen"), None),
    ];
    if app.net.coop.drive.owner_active() {
        children.push(note(text("Seu amigo está dirigindo. Esta viagem continua vinculada ao seu BCS; mantenha o jogo aberto.",
            "Your friend is driving. This trip stays on your BCS; keep the game open.",
            "Dein Freund fährt. Die Fahrt bleibt in deinem BCS; Spiel geöffnet lassen.")));
        children.push(note(text("Você agora é passageiro. Peça ao motorista atual a função de cobrador.",
            "You are now a passenger. Ask the current driver for conductor permission.",
            "Du bist jetzt Fahrgast. Bitte den Fahrer um die Kassiererrolle.")));
        children.push(button("request", text("Solicitar função de cobrador", "Request conductor role", "Kassiererrolle anfragen"), None));
    } else if app.net.coop.drive.visitor_active() {
        children.push(note(text("Você é o motorista autorizado deste ônibus. A viagem do BCS pertence a quem a iniciou.",
            "You are this bus's authorized driver. The BCS trip belongs to its original driver.",
            "Du bist der berechtigte Fahrer. Die BCS-Fahrt gehört dem ursprünglichen Fahrer.")));
        children.push(button("drive_sign", text("Letreiro / linha / destino", "Destination display / line", "Zielschild / Linie"), None));
        children.push(button("drive_return", text("Devolver direção (ônibus parado)", "Return wheel (stopped)", "Lenkung zurückgeben (im Stand)"), None));
    }
    if !app.net.coop.drive.owner_active() && app.player.is_some() && (
        app.session.on_foot.is_none()
        || app.net.coop.drive.visitor_active()
        || app.coop_passengers().iter().any(|(peer, _)| app.drive_requested(*peer))
    ) {
        children.push(note(text(
            "Autorize um passageiro para operar os botões e a caixa do seu ônibus.",
            "Allow a passenger to operate your bus's controls and cash desk.",
            "Erlaube einem Fahrgast, die Bedienelemente und Kasse zu bedienen.",
        )));
        let passengers = app.coop_passengers();
        if passengers.is_empty() {
            children.push(note(text(
                "O amigo precisa estar dentro do seu ônibus.",
                "Your friend must be aboard your bus.",
                "Dein Freund muss in deinem Bus sein.",
            )));
        }
        for (id, name) in passengers {
            if app.drive_requested(id) && app.drive_peer().is_none() {
                children.push(button(&format!("drive_grant_{id}"),
                    &format!("{}: {}", text("Entregar direção", "Hand over wheel", "Lenkung übergeben"), name), None));
            }
            let active = app.coop_conductor() == Some(id);
            children.push(button(
                &format!("grant_{id}"),
                &format!(
                    "{}: {}",
                    if active {
                        text("Cobrador atual", "Current conductor", "Aktueller Kassierer")
                    } else {
                        text("Autorizar", "Allow", "Erlauben")
                    },
                    name
                ),
                None,
            ));
        }
        if app.coop_conductor().is_some() {
            children.push(button(
                "revoke",
                text(
                    "Desativar cobrador",
                    "Disable conductor",
                    "Kassierer deaktivieren",
                ),
                None,
            ));
        }
    } else if app.foot_bus().is_some() {
        let friend=app.net.coop.drive.owner_active()
            || matches!(app.foot_bus(),Some(crate::humans::BusId::Ai(id)) if crate::humans::remote_bus_player(id).is_some());
        let enabled = app.net.coop.remote.is_some_and(|(_, e, _)| e != 0);
        if enabled {
            children.push(note(text(
                "Autorizado. Você pode clicar nos controles reais fora deste painel.",
                "Allowed. You can click the real controls outside this panel.",
                "Erlaubt. Bediene die echten Tasten außerhalb dieses Fensters.",
            )));
        } else if friend {
            children.push(note(text("Campainhas funcionam como passageiro. Para operar a caixa do amigo, peça autorização.",
                "Bells work as a passenger. Request permission to use your friend's cash desk.",
                "Als Fahrgast kannst du klingeln. Für die Kasse benötigst du die Erlaubnis des Fahrers.")));
            children.push(button(
                "request",
                text(
                    "Solicitar função de cobrador",
                    "Request conductor role",
                    "Kassiererrolle anfragen",
                ),
                None,
            ));
        } else {
            children.push(note(text("Ônibus da IA: feche o painel e clique na campainha. Em rede, o host também precisa desta versão.",
                "AI bus: close the panel and click the bell. In multiplayer the host needs this version too.",
                "KI-Bus: Schließe das Fenster und klicke auf den Haltewunschknopf. Im Mehrspieler braucht auch der Host diese Version.")));
        }
        if friend && !app.net.coop.drive.visitor_active() {
            children.push(button("drive_request", text("Solicitar direção", "Request wheel", "Lenkung anfragen"), None));
        }
        if friend && enabled {
            children.push(button("camera_f1",
                text("F1 — Câmera do motorista / caixa", "F1 — Driver / cash desk cameras", "F1 — Fahrer- / Kassenkamera"), None));
        }
        children.push(grow_row(vec![
            button("camera_f2", text("F2 — Câmeras de passageiro", "F2 — Passenger cameras", "F2 — Fahrgastkameras"), None),
            button("normal", text("Visão normal", "Normal view", "Normale Sicht"), None),
        ]));
        if app.net.coop.camera.is_some() && app.net.coop.camera_edit {
            children.push(grow_row(vec![
                button(
                    "prev",
                    text("Câmera anterior", "Previous camera", "Vorherige Kamera"),
                    None,
                ),
                button(
                    "next",
                    text("Próxima câmera", "Next camera", "Nächste Kamera"),
                    None,
                ),
            ]));
            children.push(note(if app.net.coop.camera.as_ref().is_some_and(|c| c.driver) {
                text("Banco F1: ajuste a visão da caixa junto ao motorista.",
                    "F1 bank: adjust the cash desk beside the driver.",
                    "F1-Kameras: Passe die Kassensicht neben dem Fahrer an.")
            } else {
                text("Banco F2: ajuste as câmeras do salão.",
                    "F2 bank: adjust passenger cameras.",
                    "F2-Kameras: Passe die Fahrgastkameras an.")
            }));
            children.push(grow_row(vec![
                button("x-", text("Esquerda", "Left", "Links"), None),
                button("x+", text("Direita", "Right", "Rechts"), None),
            ]));
            children.push(grow_row(vec![
                button("y+", text("Frente", "Forward", "Vor"), None),
                button("y-", text("Trás", "Back", "Zurück"), None),
            ]));
            children.push(grow_row(vec![
                button("z+", text("Subir", "Up", "Höher"), None),
                button("z-", text("Descer", "Down", "Tiefer"), None),
            ]));
            children.push(button(
                "save",
                text(
                    "Salvar câmera para este ônibus",
                    "Save camera for this bus",
                    "Kamera für diesen Bus speichern",
                ),
                None,
            ));
        }
        if enabled {
            children.push(button("close_signal", text("Sinal de fechar porta — bater na roleta",
                "Close-door signal — tap turnstile", "Türschließsignal — Münzklang"), None));
            let d = &app.net.coop.desk;
            children.push(note(&format!(
                "{} | {} {:.2} | {} {:.2} | {} {:.2}/{:.2}",
                d.name,
                text("Preço", "Price", "Preis"),
                d.price,
                text("Pago", "Paid", "Bezahlt"),
                d.paid,
                text("Troco", "Change", "Rückgeld"),
                d.given,
                d.due
            )));
            if d.uncollected > 0.0 {
                children.push(note(&format!(
                    "{} {:.2}",
                    text("Dinheiro a recolher:", "Payment to collect:", "Geld einzusammeln:"),
                    d.uncollected
                )));
                children.push(button(
                    "collect_payment",
                    text("Recolher dinheiro (Shift+T)", "Collect payment (Shift+T)", "Geld nehmen (Shift+T)"),
                    None,
                ));
            }
            // Fallback for buses using the engine's tear-off tickets/change, rather than
            // modelled printer keys. The owner's native cash desk still processes the sale.
            let start = app.net.coop.cash_page * 6;
            if d.stage == 2 {
                if let Some(tickets) = app.session.humans.as_ref().and_then(|h| h.tickets.as_ref())
                {
                    for (i, t) in tickets.tickets.iter().enumerate().skip(start).take(6) {
                        children.push(button(
                            &format!("ticket_{i}"),
                            &format!("{} — {:.2}", t.name.trim(), t.value),
                            None,
                        ));
                    }
                }
            } else if d.stage == 3 {
                if let Some(c) = app
                    .session
                    .humans
                    .as_ref()
                    .and_then(|h| h.money.as_ref())
                    .and_then(|m| m.currency.as_ref())
                {
                    let denominations: Vec<_> = c
                        .coins
                        .iter()
                        .chain(c.bills.iter())
                        .enumerate()
                        .skip(start)
                        .take(6)
                        .map(|(i, (_, value))| {
                            button(
                                &format!("change_{i}"),
                                &format!("{} {:.2}", text("Dar", "Give", "Geben"), value),
                                None,
                            )
                        })
                        .collect();
                    for row in denominations.chunks(3) {
                        children.push(grow_row(row.to_vec()));
                    }
                }
            }
            if d.stage == 2 || d.stage == 3 {
                children.push(grow_row(vec![
                    button("page_prev", "‹", None),
                    button("page_next", "›", None),
                ]));
            }
        }
    } else {
        children.push(note(text(
            "Entre no ônibus do amigo ou da IA para interagir.",
            "Board your friend's bus or an AI bus to interact.",
            "Steige in den Bus deines Freundes oder einen KI-Bus ein.",
        )));
    }
    children.push(button(
        "close",
        text("Continuar", "Continue", "Weiter"),
        None,
    ));
    card(Anchor::TopRight, 20.0, 20.0, 430.0, 235, children)
}

pub(crate) fn click(app: &mut crate::App, id: &str) {
    if super::character_panel::click(app, id) { return; }
    if let Some(peer) = id.strip_prefix("drive_grant_").and_then(|id| id.parse::<u32>().ok()) {
        app.drive_grant(peer); return;
    }
    if let Some(peer) = id.strip_prefix("grant_").and_then(|s| s.parse().ok()) {
        app.coop_grant(Some(peer));
        return;
    }
    if let Some(index) = id.strip_prefix("ticket_").and_then(|s| s.parse().ok()) {
        app.coop_cash_action(2, index);
        return;
    }
    if let Some(index) = id.strip_prefix("change_").and_then(|s| s.parse().ok()) {
        app.coop_cash_action(3, index);
        return;
    }
    match id {
        "drive_request" => app.drive_request(),
        "drive_return" => app.drive_return(),
        "drive_sign" => { app.net.coop.panel = false; crate::game_panels::open_shared_destination(app); },
        "request" => app.coop_request(),
        "revoke" => app.coop_grant(None),
        "camera_f1" => app.coop_camera_load_group(true),
        "camera_f2" => app.coop_camera_load_group(false),
        "close_signal" => app.coop_close_signal(),
        "collect_payment" => { app.coop_collect_payment(); },
        "normal" => app.net.coop.camera = None,
        "prev" => app.coop_camera_slot(-1),
        "next" => app.coop_camera_slot(1),
        "x-" => app.coop_camera_adjust(0, -0.1),
        "x+" => app.coop_camera_adjust(0, 0.1),
        "y-" => app.coop_camera_adjust(1, -0.1),
        "y+" => app.coop_camera_adjust(1, 0.1),
        "z-" => app.coop_camera_adjust(2, -0.1),
        "z+" => app.coop_camera_adjust(2, 0.1),
        "save" => app.coop_camera_save(),
        "page_prev" => app.net.coop.cash_page = app.net.coop.cash_page.saturating_sub(1),
        "page_next" => app.net.coop.cash_page = (app.net.coop.cash_page + 1).min(31),
        "close" => app.net.coop.panel = false,
        _ => {}
    }
}
