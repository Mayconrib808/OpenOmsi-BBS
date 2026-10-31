use super::{in_language, key};
use omsi_sim::people::PassengerDialog;

#[test]
fn every_passenger_caption_uses_a_real_catalog_translation_for_every_supported_language() {
    let mut translations = 0;
    for &(code, _, locale, aliases, _) in omsi_cfg::lang::LANGUAGES {
        let table = omsi_launcher_lib::i18n::table(crate::_LOCALES, locale);
        for dialog in PassengerDialog::ALL {
            let english = key(dialog);
            let expected = if locale == "en" {
                english.to_owned()
            } else {
                let translated = table
                    .get(english)
                    .unwrap_or_else(|| panic!("missing {locale}: {english}"));
                assert!(!translated.trim().is_empty(), "empty {locale}: {english}");
                assert_ne!(
                    translated, english,
                    "English placeholder for {locale}: {english}"
                );
                translations += 1;
                translated.clone()
            };
            assert_eq!(
                in_language(dialog.id(), code).as_deref(),
                Some(expected.as_str()),
                "{code}: {dialog:?}"
            );
            for alias in aliases {
                assert_eq!(
                    in_language(dialog.id(), alias).as_deref(),
                    Some(expected.as_str()),
                    "{alias}: {dialog:?}"
                );
            }
        }
    }
    assert_eq!(translations, 29 * PassengerDialog::ALL.len());
}

#[test]
fn french_japanese_brazilian_and_european_portuguese_do_not_use_the_three_language_helper() {
    assert_eq!(
        in_language("BridgeOpenDoor", "FRA").as_deref(),
        Some("Pouvez-vous ouvrir la porte, s’il vous plaît ?")
    );
    assert_eq!(
        in_language("BridgeOpenDoor", "JPN").as_deref(),
        Some("ドアを開けてもらえますか？")
    );
    assert_eq!(
        in_language("BridgeMissedStop", "PTB").as_deref(),
        Some("Motorista, você passou do meu ponto!")
    );
    assert_eq!(
        in_language("BridgeMissedStop", "PTP").as_deref(),
        Some("Motorista, passou a minha paragem!")
    );
}

#[test]
fn different_reactions_remain_distinct_and_stock_voice_captions_keep_their_existing_path() {
    for &(_, _, locale, _, _) in omsi_cfg::lang::LANGUAGES {
        let values: Vec<_> = PassengerDialog::ALL
            .iter()
            .map(|d| in_language(d.id(), locale).unwrap())
            .collect();
        for (index, value) in values.iter().enumerate() {
            assert!(
                !values[..index].contains(value),
                "two reactions share the same caption in {locale}"
            );
        }
    }
    for stock in ["TooBad_A", "Thanks_1", "Pardon_1", "Hello_1", ""] {
        assert_eq!(in_language(stock, "FRA"), None);
    }
}
