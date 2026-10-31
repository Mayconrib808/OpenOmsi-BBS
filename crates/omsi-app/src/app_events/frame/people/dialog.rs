//! Additional passenger captions use the same compiled catalog and language selection
//! as the rest of the native interface. No voice files or settings are read here.

use omsi_sim::people::PassengerDialog;

fn key(dialog: PassengerDialog) -> &'static str {
    match dialog {
        PassengerDialog::OpenDoor => "Could you open the door, please?",
        PassengerDialog::MissedStop => "Driver, you missed my stop!",
        PassengerDialog::LetMeOff => "Please let me get off!",
        PassengerDialog::ExitThanks => "Thank you, driver!",
    }
}

fn in_language(name: &str, language: &str) -> Option<String> {
    let key = key(PassengerDialog::from_id(name)?);
    let locale = omsi_cfg::lang::language_iso(language);
    Some(if locale.is_empty() {
        key.to_owned()
    } else {
        crate::_rust_i18n_try_translate(locale, key)
            .map(|translated| translated.into_owned())
            .unwrap_or_else(|| key.to_owned())
    })
}

pub(super) fn current(name: &str) -> Option<String> {
    in_language(name, &omsi_ui::i18n::language())
}

#[cfg(test)]
#[path = "dialog_tests.rs"]
mod tests;
